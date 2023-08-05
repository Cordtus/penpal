package rpc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

func GetLatestHeight(url string, client *http.Client) (chainID string, height string, err error) {
	block, err := GetLatestBlock(url, client)
	return block.Result.Block.Header.ChainID, block.Result.Block.Header.Height, err
}

func GetLatestBlockTime(url string, client *http.Client) (string, time.Time, error) {
	block, err := GetLatestBlock(url, client)
	return block.Result.Block.Header.ChainID, block.Result.Block.Header.Time, err
}

func GetLatestBlock(url string, client *http.Client) (responseData Block, err error) {
	err = GetByUrlAndUnmarshall(&responseData, url+"/block", client)
	return
}

func GetBlockFromHeight(height string, url string, client *http.Client) (responseData Block, err error) {
	err = getByUrlAndUnmarshall(&responseData, url+"/block?height="+height, client)
	return
}

func GetByUrlAndUnmarshall(data interface{}, url string, client *http.Client) (err error) {
	r := &strings.Reader{}
	req, err := http.NewRequestWithContext(context.Background(), "GET", url, r)
	if err != nil {
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			return
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	err = json.Unmarshal(body, &data)
	return
}
