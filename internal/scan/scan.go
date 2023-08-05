package scan

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/Cordtus/penpal/internal/alert"
	"github.com/Cordtus/penpal/internal/config"
	"github.com/Cordtus/penpal/internal/rpc"
)

func Monitor(cfg config.Config) {
	alertChan := make(chan alert.Alert)
	exit := make(chan bool)
	client := &http.Client{
		Timeout: time.Second * 5,
	}
	for _, network := range cfg.Networks {
		go scanNetwork(network, alertChan, client)
		go alert.Watch(alertChan, cfg.Notifiers, client)
	}
	if cfg.Health.Interval != 0 {
		go healthServer(cfg.Health.Port)
		go healthCheck(cfg.Health, alertChan, client)
	}
	<-exit
}

func scanNetwork(network config.Network, alertChan chan<- alert.Alert, client *http.Client) {
	var (
		interval int
		alerted  bool
	)
	for {
		checkNetwork(network, client, &alerted, alertChan)
		if alerted && network.Interval > 2 {
			interval = 2
		} else {
			interval = network.Interval
		}
		time.Sleep(time.Duration(interval) * time.Minute)
	}
}

func checkNetwork(network config.Network, client *http.Client, alerted *bool, alertChan chan<- alert.Alert) {
	var (
		chainID string
		height  string
		url     string
		err     error
	)
	rpcs := network.Rpcs
	if len(rpcs) > 1 {
		// ... existing code
	} else if len(rpcs) == 1 {
		url = network.Rpcs[0]
		chainID, height, err = rpc.GetLatestHeight(url, client)
		if err != nil && !*alerted {
			*alerted = true
			alertChan <- alert.NoRpc(network.Name)
			return
		}
		if chainID != network.ChainID && !*alerted {
			*alerted = true
			alertChan <- alert.NoRpc(network.Name)
			return
		}

		block, err := rpc.GetLatestBlock(url, client)
		if err != nil || block.Error != nil {
			log.Println("err - failed to check latest block for", network.Name)
		} else if checkSig(network.Address, block) {
			*alerted = true
			alertChan <- alert.Confirmed(len(block.Result.Block.LastCommit.Signatures), network.BackCheck, network.Name)
		}
	}
}


func backCheck(cfg config.Network, height int, alerted *bool, url string, client *http.Client) alert.Alert {
	var (
		signed    int
		rpcErrors int
	)
	for checkHeight := height - cfg.BackCheck + 1; checkHeight <= height; checkHeight++ {
		block, err := rpc.GetBlockFromHeight(strconv.Itoa(checkHeight), url, client)
		if err != nil || block.Error != nil {
			rpcErrors++
			cfg.BackCheck--
			continue
		}
		if checkSig(cfg.Address, block) {
			signed++
		}
	}
	if rpcErrors > cfg.BackCheck || cfg.BackCheck == 0 {
		if !*alerted {
			*alerted = true
			return alert.RpcDown(url)
		} else {
			return alert.Nil("repeat alert suppressed - RpcDown on " + cfg.Name)
		}
	} else if cfg.BackCheck-signed > cfg.AlertThreshold {
		*alerted = true
		return alert.Missed((cfg.BackCheck - signed), cfg.BackCheck, cfg.Name)
	} else if *alerted {
		*alerted = false
		return alert.Cleared(signed, cfg.BackCheck, cfg.Name)
	} else {
		return alert.Nil("found " + strconv.Itoa(signed) + " of " + strconv.Itoa(cfg.BackCheck) + " signed on " + cfg.Name)
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
