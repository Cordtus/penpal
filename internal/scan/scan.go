package scan

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/cordtus/penpal/internal/alert"
	"github.com/cordtus/penpal/internal/config"
	"github.com/cordtus/penpal/internal/rpc"
)

func Monitor(cfg config.Config) {
	alertChan := make(chan alert.Alert)
	exit := make(chan bool)
	client := &http.Client{
		Timeout: time.Second * 5,
	}

	network := cfg.Network[0]
	rpcs := network.Rpcs
	block, err := rpc.GetLatestBlock(rpcs[0], client) // Fetch block data once

	if err != nil {
		log.Fatal("Failed to fetch the latest block data:", err)
		return
	}

	for _, validator := range cfg.Validators {
		go scanValidator(network, block, validator, alertChan)
	}

	go alert.Watch(alertChan, cfg.Notifiers, client)

	if cfg.Health.Interval != 0 {
		go healthServer(cfg.Health.Port)
		go healthCheck(cfg.Health, alertChan, client)
	}

	<-exit
}

func scanValidator(network config.Network, block rpc.Block, validator config.Validators, alertChan chan<- alert.Alert) {
	var (
		interval int
		alerted  bool
	)
	for {
		checkValidator(network, block, validator, &alerted, alertChan)
		if alerted && network.Interval > 2 {
			interval = 2
		} else {
			interval = network.Interval
		}
		time.Sleep(time.Duration(interval) * time.Minute)
	}
}

func checkValidator(network config.Network, block rpc.Block, validator config.Validators, alerted *bool, alertChan chan<- alert.Alert) {
	var (
		chainId   string
		height    string
		blocktime time.Time
	)

	height = block.Result.Block.Header.Height
	chainId = block.Result.Block.Header.ChainID
	blocktime = block.Result.Block.Header.Time

	if chainId != network.ChainId && !*alerted && network.RpcAlert {
		log.Println("err - chain id validation failed for rpc", network.Rpcs[0], "on", network.ChainId)
		*alerted = true
		alertChan <- alert.NoRpc(network.ChainId)
		return
	}

	if network.StallTime != 0 && time.Since(blocktime) > time.Minute*time.Duration(network.StallTime) {
		log.Println("last block time on", network.ChainId, "is", blocktime, "- sending alert")

		*alerted = true
		alertChan <- alert.Stalled(blocktime, network.ChainId)
	}

	alert := alert.Signed(backCheck(network, height, validator, block), network.BackCheck, validator.Moniker)
	alertChan <- alert
}

func backCheck(network config.Network, height string, validator config.Validators, block rpc.Block) int {
	signedBlocks := 0

	heightInt, _ := strconv.Atoi(height)

	for checkHeight := heightInt; checkHeight <= heightInt+network.BackCheck-1; checkHeight++ {
		if checkSig(validator.Address, block, checkHeight) {
			signedBlocks++
		}
	}

	return signedBlocks
}

func checkSig(address string, block rpc.Block, checkHeight int) bool {
	for _, sig := range block.Result.Block.LastCommit.Signatures {
		// You should check the structure of sig to access the correct field, e.g., sig.ValidatorAddress
		// if sig.ValidatorAddress == address && sig.BlockHeight == uint64(checkHeight) {
		if sig.ValidatorAddress == address {
			return true
		}
	}
	return false
}
