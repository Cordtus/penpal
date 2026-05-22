package scan

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cordtus/penpal/internal/alert"
	"github.com/cordtus/penpal/internal/rpc"
	"github.com/cordtus/penpal/internal/settings"
)

func Monitor(cfg settings.Config) {
	alertChan := make(chan alert.Alert)
	client := &http.Client{Timeout: time.Second * 10}
	go alert.Watch(alertChan, cfg, client)

	for _, network := range cfg.Networks {
		go monitorNetwork(network, alertChan, client)
		if network.SignerMetrics != "" {
			go monitorSigner(network, alertChan, client)
		}
	}

	select {}
}

// getWorkingRpc tries each RPC in the list and returns the first one that responds.
func getWorkingRpc(rpcs []string, client *http.Client) (string, error) {
	var lastErr error
	for _, url := range rpcs {
		_, _, err := rpc.GetLatestHeight(url, client)
		if err == nil {
			return url, nil
		}
		lastErr = err
		log.Println("RPC unreachable:", url, err)
	}
	return "", lastErr
}

func monitorNetwork(network settings.Network, alertChan chan<- alert.Alert, client *http.Client) {
	backCheckAlerted := false
	rpcAlerted := false

	for {
		// Find a working RPC with failover
		activeRpc, err := getWorkingRpc(network.Rpcs, client)
		if err != nil {
			if !rpcAlerted {
				rpcAlerted = true
				alertChan <- alert.NoRpc(network.ChainId)
			}
			time.Sleep(time.Duration(network.Interval) * time.Second)
			continue
		}
		if rpcAlerted {
			rpcAlerted = false
			log.Println("RPC recovered for", network.ChainId, "using", activeRpc)
		}

		// Get latest block time to check for stalls
		_, blockTime, err := rpc.GetLatestBlockTime(activeRpc, client)
		if err != nil {
			time.Sleep(time.Duration(network.Interval) * time.Second)
			continue
		}

		if network.StallTime > 0 && time.Since(blockTime) > time.Duration(network.StallTime)*time.Minute {
			alertChan <- alert.Stalled(blockTime, network.ChainId)
		}

		// Get latest height
		_, heightStr, err := rpc.GetLatestHeight(activeRpc, client)
		if err != nil {
			time.Sleep(time.Duration(network.Interval) * time.Second)
			continue
		}

		height, err := strconv.ParseInt(heightStr, 10, 64)
		if err != nil {
			alertChan <- alert.InvalidHeight(network.ChainId)
			time.Sleep(time.Duration(network.Interval) * time.Second)
			continue
		}

		// Count signed blocks in the backcheck window
		var missing, total int
		for i := 0; i < network.BackCheck; i++ {
			h := strconv.FormatInt(height-int64(i), 10)
			block, err := rpc.GetBlockFromHeight(h, activeRpc, client)
			if err != nil {
				log.Println("Failed to fetch block at height", h, ":", err)
				continue
			}
			if !checkSig(network.Address, block) {
				missing++
			}
			total++
		}

		signed := total - missing

		if missing >= network.AlertThreshold {
			if !backCheckAlerted {
				backCheckAlerted = true
				alertChan <- alert.Missed(missing, total, network.Name)
			}
		} else if backCheckAlerted {
			backCheckAlerted = false
			alertChan <- alert.Cleared(signed, total, network.Name)
		}

		time.Sleep(time.Duration(network.Interval) * time.Second)
	}
}

type signerMetrics struct {
	errorsCounter      int64
	checkpointIndex    int64
	checkpointUnixTime int64
}

func monitorSigner(network settings.Network, alertChan chan<- alert.Alert, client *http.Client) {
	downAlerted := false
	stalledAlerted := false
	var lastErrorCount int64 = -1
	var lastCheckpointIndex int64 = -1

	for {
		metrics, err := getSignerMetrics(network.SignerMetrics, client)
		if err != nil {
			if !downAlerted {
				downAlerted = true
				alertChan <- alert.SignerDown(network.Name)
			}
			time.Sleep(time.Duration(network.Interval) * time.Second)
			continue
		}

		if downAlerted {
			downAlerted = false
			alertChan <- alert.SignerRecovered(network.Name)
		}

		if lastErrorCount >= 0 && metrics.errorsCounter > lastErrorCount {
			alertChan <- alert.SignerError(network.Name, metrics.errorsCounter)
		}
		lastErrorCount = metrics.errorsCounter

		if metrics.checkpointIndex >= 0 && metrics.checkpointIndex > lastCheckpointIndex {
			lastCheckpointIndex = metrics.checkpointIndex
			if stalledAlerted {
				stalledAlerted = false
				alertChan <- alert.SignerRecovered(network.Name)
			}
		}

		if network.SignerStallMins > 0 && metrics.checkpointUnixTime > 0 {
			lastCheckpoint := time.Unix(metrics.checkpointUnixTime, 0)
			if time.Since(lastCheckpoint) > time.Duration(network.SignerStallMins)*time.Minute {
				if !stalledAlerted {
					stalledAlerted = true
					alertChan <- alert.SignerStalled(lastCheckpoint, network.Name)
				}
			} else if stalledAlerted {
				stalledAlerted = false
				alertChan <- alert.SignerRecovered(network.Name)
			}
		}
		time.Sleep(time.Duration(network.Interval) * time.Second)
	}
}

func getSignerMetrics(url string, client *http.Client) (signerMetrics, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return signerMetrics{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return signerMetrics{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return signerMetrics{}, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return signerMetrics{}, err
	}

	var metrics = signerMetrics{checkpointIndex: -1, checkpointUnixTime: -1}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "nomic_signer_errors "):
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				val, err := strconv.ParseFloat(fields[1], 64)
				if err == nil {
					metrics.errorsCounter = int64(val)
				}
			}
		case strings.HasPrefix(line, "nomic_signer_checkpoint_index "):
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				val, err := strconv.ParseFloat(fields[1], 64)
				if err == nil {
					metrics.checkpointIndex = int64(val)
				}
			}
		case strings.HasPrefix(line, "nomic_signer_checkpoint_timestamp "):
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				val, err := strconv.ParseFloat(fields[1], 64)
				if err == nil {
					metrics.checkpointUnixTime = int64(val)
				}
			}
		}
	}

	return metrics, nil
}

func checkSig(address string, block rpc.Block) bool {
	for _, sig := range block.Result.Block.LastCommit.Signatures {
		if sig.ValidatorAddress == address {
			return true
		}
	}
	return false
}
