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
	client := &http.Client{Timeout: time.Second * 10}

	network := cfg.Networks[0]
	validator := cfg.Validators[0]

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

			if !checkSig(validator.Address, block) {
				missing++
			}
			total++
		}

		if missing >= network.Threshold {
			if !backCheckAlerted {
				backCheckAlerted = true
				alertChan <- alert.MissingBlocks(missing, total, validator.Address)
			}
		} else if backCheckAlerted {
			backCheckAlerted = false
			alertChan <- alert.Recovered(validator.Address)
		}

		time.Sleep(network.Interval)
	}
}

func checkSig(address string, block rpc.Block) bool {
	for _, sig := range block.Result.Block.LastCommit.Signatures {
		if sig.ValidatorAddress == address {
			return true
		}
	}
	return false
}
