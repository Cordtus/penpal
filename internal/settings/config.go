package settings

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func New(file string) (err error) {
	_, err = os.Stat(file)
	if err == nil {
		err = errors.New("config file already exists at " + file)
		return
	} else if !os.IsNotExist(err) {
		return
	}
	configFile, err := os.Create(filepath.Clean(file))
	if err != nil {
		return
	}
	encoder := json.NewEncoder(configFile)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(Config{
		Networks: []Network{
			{
				Name:           "Network1",
				ChainId:        "network-1",
				Address:        "VALIDATOR_HEX_ADDRESS",
				Rpcs:           []string{"http://localhost:26657"},
				RpcAlert:       true,
				SignerMetrics:  "",
				SignerStallMins: 60,
				BackCheck:      20,
				AlertThreshold: 5,
				Interval:       15,
				StallTime:      30,
			},
		},
		Notifiers: Notifiers{
			Telegram: struct {
				Key  string `json:"key"`
				Chat string `json:"chat_id"`
			}{
				Key:  "api_key",
				Chat: "chat_id",
			},
			Discord: struct {
				Webhook string `json:"webhook"`
			}{
				Webhook: "",
			},
		},
		Health: Health{
			Interval: 1,
			Port:     "8080",
			Nodes:    []string{},
		},
	})
	if err == nil {
		err = errors.New("generated a new config at " + file)
	}
	return
}
