package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

func GetLatestBlock(url string, client *http.Client) (responseData Block, err error) {
	log.Println("Debug: Final URL is ", url+"/block")
	err = getByURLAndUnmarshal(&responseData, url+"/block", client)
	return responseData, err
}

func GetLatestHeight(url string, client *http.Client) (height string, err error) {
	block, err := GetLatestBlock(url, client)
	if err != nil {
		return "", err
	}
	return block.Result.Block.Header.Height, nil
}

func GetLatestBlockTime(url string, client *http.Client) (blockTime time.Time, err error) {
	block, err := GetLatestBlock(url, client)
	if err != nil {
		return time.Time{}, err
	}
	return block.Result.Block.Header.Time, nil
}

func GetBlockFromHeight(height string, url string, client *http.Client) (responseData Block, err error) {
	log.Println("Debug: Final URL is ", url+"/block?height="+height)
	err = getByURLAndUnmarshal(&responseData, url+"/block?height="+height, client)
	return responseData, err
}

func getByURLAndUnmarshal(data interface{}, url string, client *http.Client) (err error) {
	log.Println("Debug: Final URL is ", url)
	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("Received non-200 response code: %d", resp.StatusCode)
	}
	if resp.Body == nil {
		return fmt.Errorf("Response body is nil")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, &data)
}
