package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/cordtus/penpal/internal/settings"
)

const (
	maxRepeatAlerts    = 5
	maxRetries         = 5
	stallAlertInterval = 60 * time.Minute // Time interval for repeating 'Stall' alerts
)

func Watch(alertChan <-chan Alert, cfg settings.Config, client *http.Client) {
	backoffAttempts := make(map[string]int)
	lastSignedTime := make(map[string]time.Time)
	lastStallTime := make(map[string]time.Time) // Track the last time a 'Stall' alert was sent for each message.

	for {
		a := <-alertChan
		if a.AlertType == None {
			log.Println(a.Message)
			continue
		}

		if a.AlertType == Clear {
			lastTime, exists := lastSignedTime[a.Message]
			if exists {
				if time.Since(lastTime) < 24*time.Hour {
					log.Printf("Skipping 'Clear' alert for message '%s' as it was sent within the last 24 hours.", a.Message)
					continue
				}
			}

			lastSignedTime[a.Message] = time.Now()
		}

		// Check if the alert type is 'Stall'
		if a.AlertType == Stall {
			lastTime, exists := lastStallTime[a.Message]
			if exists {
				if time.Since(lastTime) < 1*time.Hour {
					log.Printf("Skipping 'Stall' alert for message '%s' as it was sent within the hour.", a.Message)
					continue
				}
			}

			lastStallTime[a.Message] = time.Now()
		}

		var notifications []notification
		if cfg.Notifiers.Telegram.Key != "" {
			notifications = append(notifications, telegramNoti(cfg.Notifiers.Telegram.Key, cfg.Notifiers.Telegram.Chat, a.Message))
		}
		if cfg.Notifiers.Discord.Webhook != "" {
			notifications = append(notifications, discordNoti(cfg.Notifiers.Discord.Webhook, a.Message))
		}

		for _, n := range notifications {
			go func(b notification, alertMsg string) {
				for i := 0; i < maxRetries; i++ {

					time.Sleep(1 * time.Second)

					err := b.send(client)
					if err == nil {
						log.Println("Sent alert to", b.Type, alertMsg)
						delete(backoffAttempts, alertMsg)
						return
					}
					log.Printf("Error sending message %s to %s. Retrying...", alertMsg, b.Type)
				}

				backoffAttempts[alertMsg]++
				log.Printf("Error sending message %s to %s after maximum retries. Skipping further notifications.", alertMsg, b.Type)
			}(n, a.Message)
		}

		time.Sleep(1 * time.Second)
	}
}

func (n notification) send(client *http.Client) error {
	json, err := json.Marshal(n.Content)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	req, err := http.NewRequestWithContext(context.Background(), "POST", n.Auth, bytes.NewBuffer(json))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode > 299 || resp.StatusCode < 200 {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}
	return nil
}

func telegramNoti(key, chat, message string) notification {
	return notification{Type: "telegram", Auth: "https://api.telegram.org/bot" + key + "/sendMessage", Content: telegramMessage{Chat: chat, Text: message}}
}

func discordNoti(url, message string) notification {
	return notification{Type: "discord", Auth: url, Content: discordMessage{Username: "Alert-", Content: message}}
}

func Nil(message string) Alert {
	return Alert{AlertType: None, Message: message}
}

func Missed(missed int, check int, validatorMoniker string) Alert {
	return Alert{AlertType: Miss, Message: " ❌ " + validatorMoniker + " missed " + strconv.Itoa(missed) + " of " + strconv.Itoa(check) + " recent blocks "}
}

func Cleared(signed int, check int, validatorMoniker string) Alert {
	return Alert{AlertType: Clear, Message: " ♿️ " + validatorMoniker + " is recovering, " + strconv.Itoa(signed) + " of " + strconv.Itoa(check) + " recent blocks signed "}
}

func Signed(signed int, check int, validatorMoniker string) Alert {
	return Alert{AlertType: Clear, Message: " ✅ " + validatorMoniker + " signed " + strconv.Itoa(signed) + " of " + strconv.Itoa(check) + " recent blocks "}
}

func NoRpc(ChainId string) Alert {
	return Alert{AlertType: RpcError, Message: "📡 no rpcs available for " + ChainId}
}

func RpcDown(url string) Alert {
	return Alert{AlertType: RpcError, Message: "📡 rpc " + url + " is down or malfunctioning "}
}

func InvalidHeight(ChainId string) Alert {
	return Alert{AlertType: Error, Message: "❓ Invalid height for " + ChainId}
}

func Stalled(blocktime time.Time, ChainId string) Alert {
	return Alert{AlertType: Stall, Message: "⏰ warning - last block " + ChainId + " produced at " + blocktime.Format(time.RFC1123)}
}
