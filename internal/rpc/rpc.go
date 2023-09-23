package rpc

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

func GetLatestBlockData(url string, client *http.Client) (data LatestBlockData, err error) {
	resp, err := client.Get(url + "/block")
	if err != nil {
		return LatestBlockData{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return LatestBlockData{}, err
	}

	var block Block
	err = json.Unmarshal(body, &block)
	if err != nil {
		return LatestBlockData{}, err
	}

	return LatestBlockData{
		Block:     block,
		Height:    block.Result.Block.Header.Height,
		ChainID:   block.Result.Block.Header.ChainID,
		BlockTime: block.Result.Block.Header.Time,
	}, nil
}

func GetLatestHeight(url string, client *http.Client) (height string, err error) {
	data, err := GetLatestBlockData(url, client)
	if err != nil {
		return "", err
	}
	return data.Height, nil
}

func GetLatestBlockTime(url string, client *http.Client) (chainID string, blockTime time.Time, err error) {
	data, err := GetLatestBlockData(url, client)
	if err != nil {
		return "", time.Time{}, err
	}
	return data.ChainID, data.BlockTime, nil
}

func GetBlockFromHeight(height string, url string, client *http.Client) (responseData Block, err error) {
	err = getByURLAndUnmarshal(&responseData, url+"/block?height="+height, client)
	return responseData, err
}

func getByURLAndUnmarshal(data interface{}, url string, client *http.Client) (err error) {
	r := &strings.Reader{}
	req, err := http.NewRequestWithContext(context.Background(), "GET", url, r)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			return
		}
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(body, &data)
	return err
}
