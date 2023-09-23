package rpc

import "time"

type Block struct {
	Result struct {
		Block struct {
			Header struct {
				ChainID string    `json:"chain_id"`
				Height  string    `json:"height"`
				Time    time.Time `json:"time"`
			} `json:"header"`
			Data struct {
				Txs []string `json:"txs"`
			} `json:"data"`
			Evidence struct {
				Evidence []string `json:"evidence"`
			} `json:"evidence"`
		} `json:"block"`
	} `json:"result"`
}

type LatestBlockData struct {
	Block     Block
	Height    string
	ChainID   string
	BlockTime time.Time
}
