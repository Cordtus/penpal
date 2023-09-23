package scan

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/cordtus/penpal/internal/alert"
	"github.com/cordtus/penpal/internal/rpc"
	"github.com/cordtus/penpal/internal/settings"
)

func Monitor(cfg settings.Config) {
	alertChan := make(chan alert.Alert)
	exit := make(chan bool)
	client := &http.Client{
		Timeout: time.Second * 5,
	}

	network := cfg.Network[0]
	for _, validator := range cfg.Validators {
		go scanValidator(network, client, validator, alertChan)
	}

	alert.Watch(alertChan, settings.Config{Notifiers: cfg.Notifiers}, client)
	<-exit
}

func scanValidator(network settings.Network, client *http.Client, validator settings.Validator, alertChan chan<- alert.Alert) {
	height, err := rpc.GetLatestHeight(network.Rpcs[0], client)
	if err != nil {
		alertChan <- alert.NoRpc(network.ChainId)
		return
	}

	backCheckAlerted := false
	for {
		blockData, err := rpc.GetLatestBlockData(network.Rpcs[0], client)
		if err != nil {
			alertChan <- alert.RpcDown(network.Rpcs[0])
			continue
		}

		if time.Since(blockData.Block.Header.Time) > network.StallDuration {
			alertChan <- alert.Stalled(blockData.Block.Header.Time, network.ChainId)
		}

		signed, err := rpc.GetSignedCount(network.Rpcs[0], client, validator.Address, height, network.BlocksToCheck)
		if err != nil {
			alertChan <- alert.RpcDown(network.Rpcs[0])
			continue
		}

		if signed < network.BlocksToCheck-network.Threshold {
			if !backCheckAlerted {
				backCheckAlerted = true
				alertChan <- alert.Missed(network.BlocksToCheck-signed, network.BlocksToCheck, validator.Moniker)
			}
		} else if backCheckAlerted {
			backCheckAlerted = false
			alertChan <- alert.Cleared(signed, network.BlocksToCheck, validator.Moniker)
		} else if signed == network.BlocksToCheck {
			alertChan <- alert.Signed(signed, network.BlocksToCheck, validator.Moniker)
		}

		backCheckAlert := backCheck(network, client, height, validator.Address, &backCheckAlerted)
		alertChan <- backCheckAlert

		time.Sleep(network.Interval)
	}
}

func backCheck(network settings.Network, client *http.Client, height int64, address string, alerted *bool) alert.Alert {
	var (
		missing int
		total   int
	)

	for i := 0; i < network.BlocksToCheck; i++ {
		block, err := rpc.GetBlockFromHeight(network.Rpcs[0], client, strconv.FormatInt(height-int64(i), 10))
		if err != nil {
			log.Println("Failed to fetch block from height:", err)
			continue
		}

		if !checkSig(address, block) {
			missing++
		}
		total++
	}

	if missing >= network.Threshold {
		if !*alerted {
			*alerted = true
			return alert.MissingBlocks(missing, total, address)
		}
	} else if *alerted {
		*alerted = false
		return alert.Recovered(address)
	}

	return alert.Alert{}
}

func checkSig(address string, block rpc.Block) bool {
	for _, sig := range block.Result.Block.LastCommit.Signatures {
		if sig.ValidatorAddress == address {
			return true
		}
	}
	return false
}
