package scan

import (
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/cordtus/penpal/internal/alert"
	"github.com/cordtus/penpal/internal/rpc"
	"github.com/cordtus/penpal/internal/settings"
)

type BlockCheckResult struct {
	Signed bool
	Error  error
}

func Monitor(cfg settings.Config) {
	alertChan := make(chan alert.Alert)
	network := cfg.Network[0]
	exit := make(chan bool)
	client := &http.Client{
		Timeout: time.Second * 5,
	}

	go alert.Watch(alertChan, settings.Config{Notifiers: cfg.Notifiers}, client)

	for _, validator := range cfg.Validators {
		go scanValidator(network, client, validator, alertChan)
	}

	<-exit
}

func scanValidator(network settings.Network, client *http.Client, validator settings.Validators, alertChan chan<- alert.Alert) {
	alerted := new(bool)
	for {
		block, err := rpc.GetLatestBlock(network.Rpcs[0], client)
		if err != nil {
			log.Println("Failed to fetch the latest block data:", err)
		}

		checkValidator(network, block, client, validator, alertChan, alerted)

		if network.Interval < 2 {
			time.Sleep(time.Minute * 2)
		} else {
			time.Sleep(time.Minute * time.Duration(network.Interval))
		}
	}
}

func checkValidator(network settings.Network, block rpc.Block, client *http.Client, validator settings.Validators, alertChan chan<- alert.Alert, alerted *bool) {
	var (
		chainId   string
		height    string
		blocktime time.Time
	)

	height = block.Result.Block.Header.Height
	chainId = block.Result.Block.Header.ChainID
	blocktime = block.Result.Block.Header.Time

	if chainId != network.ChainId && *alerted {
		log.Println("err - chain id validation failed for rpc", network.Rpcs[0], "on", network.ChainId)
		alerted = new(bool)
		*alerted = true
		alertChan <- alert.NoRpc(network.ChainId)
		return
	}

	if network.StallTime != 0 && time.Since(blocktime) > time.Minute*time.Duration(network.StallTime) {
		log.Println("latest block time is", blocktime, "- sending alert")
		alerted = new(bool)
		*alerted = true
		alertChan <- alert.Stalled(blocktime)
	}

	alert := backCheck(network, height, validator, client, alerted) // Pass the alerted parameter here
	alertChan <- alert
}

func backCheck(network settings.Network, height string, validator settings.Validators, client *http.Client, alerted *bool) alert.Alert {
	signedBlocks := 0
	missedBlocks := 0
	heightInt, err := strconv.Atoi(height)
	if err != nil {
		log.Println("Error converting height to integer:", err)
		return alert.Nil("Error converting height to integer")
	}

	var wg sync.WaitGroup
	resultChan := make(chan BlockCheckResult, network.BackCheck)

	for checkHeight := heightInt - network.BackCheck + 1; checkHeight <= heightInt; checkHeight++ {
		wg.Add(1)
		go func(height int) {
			defer wg.Done()

			block, err := rpc.GetBlockFromHeight(strconv.Itoa(height), network.Rpcs[0], client)
			if err != nil {
				log.Println("Failed to fetch block data for height", height, ":", err)
				resultChan <- BlockCheckResult{Signed: false, Error: err}
				return
			}

			if checkSig(validator.Address, block) {
				resultChan <- BlockCheckResult{Signed: true, Error: nil}
			} else {
				resultChan <- BlockCheckResult{Signed: false, Error: nil}
			}
		}(checkHeight)
	}

	wg.Wait()
	close(resultChan)

	for result := range resultChan {
		if result.Error != nil {
			continue
		}
		if result.Signed {
			signedBlocks++
		} else {
			missedBlocks++
		}
	}

	if missedBlocks > network.AlertThreshold {
		if !*alerted {
			*alerted = true
			return alert.Missed(missedBlocks, network.BackCheck, validator.Moniker)
		} else {
			return alert.Nil("repeat alert suppressed - Missed blocks for " + validator.Moniker)
		}
	} else if signedBlocks == network.BackCheck {
		if *alerted {
			*alerted = false
			return alert.Cleared(signedBlocks, network.BackCheck, validator.Moniker)
		} else {
			return alert.Signed(signedBlocks, network.BackCheck, validator.Moniker)
		}
	} else {
		return alert.Nil("found " + strconv.Itoa(signedBlocks) + " of " + strconv.Itoa(network.BackCheck) + " signed for " + validator.Moniker)
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
