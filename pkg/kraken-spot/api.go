package krakenspot

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/readysetliqd/crypto-exchange-library-go/pkg/kraken-spot/internal/data"
)

// Calls Kraken API public market data "Time" endpoint. Gets the server's time.
// data.ServerTime struct
func GetServerTime() (*data.ServerTime, error) {
	serverTime := &data.ServerTime{}
	err := callPublicApi("Time", serverTime)
	if err != nil {
		return nil, err
	}
	return serverTime, nil
}

// Calls Kraken API public market data "SystemStatus" endpoint. Gets the current
// system status or trading mode
func GetSystemStatus() (*data.SystemStatus, error) {
	systemStatus := &data.SystemStatus{}
	err := callPublicApi("SystemStatus", systemStatus)
	if err != nil {
		return nil, err
	}
	return systemStatus, nil
}

// Calls Kraken API public market data "SystemStatus" endpoint and returns true
// if system is online. Returns false and current status as a string if not
// online. Returns false and error if error produced from calling API.
func SystemIsOnline() (bool, string) {
	systemStatus, err := GetSystemStatus()
	if err != nil {
		log.Println("error calling GetSystemStatus() | ", err)
		return false, "error"
	}
	if systemStatus.Status == "online" {
		return true, systemStatus.Status
	}
	return false, systemStatus.Status
}

// Calls Kraken API public market data "Assets" endpoint. Gets information about
// all assets that are available for deposit, withdrawal, trading and staking.
// Returns them as *map[string]data.AssetInfo where the string is the asset name.
func GetAllAssetInfo() (*map[string]data.AssetInfo, error) {
	allAssetInfo := &map[string]data.AssetInfo{}
	err := callPublicApi("Assets", allAssetInfo)
	if err != nil {
		return nil, err
	}
	return allAssetInfo, nil
}

// Calls Kraken API public market data "Assets" endpoint. Returns a slice of
// strings of all tradeable asset names
func AllAssets() ([]string, error) {
	allAssetInfo := &map[string]data.AssetInfo{}
	err := callPublicApi("Assets", allAssetInfo)
	if err != nil {
		return nil, err
	}
	allAssets := []string{}
	for asset := range *allAssetInfo {
		allAssets = append(allAssets, asset)
	}
	return allAssets, nil
}

// Calls Kraken API public market data "Assets" endpoint. Gets information about
// specific asset passed to arg.
func GetAssetInfo(asset string) (*data.AssetInfo, error) {
	assetInfo := &map[string]data.AssetInfo{}
	endpoint := "Assets?asset=" + asset
	err := callPublicApi(endpoint, assetInfo)
	if err != nil {
		return nil, err
	}
	info := (*assetInfo)[asset]
	return &info, nil
}

// Calls Kraken API public market data "AssetPairs" endpoint. Gets info for all
// tradable asset pairs
func GetAllTradeablePairs() (*map[string]data.AssetPair, error) {
	allPairInfo := &map[string]data.AssetPair{}
	err := callPublicApi("AssetPairs", allPairInfo)
	if err != nil {
		return nil, err
	}
	return allPairInfo, nil
}

// Calls Kraken's public api endpoint. Args endpoint string should match the url
// endpoint from the api docs. Args target interface{} should be a pointer to
// an empty struct of the matching endpoint data type
func callPublicApi(endpoint string, target interface{}) error {
	url := data.PublicApiUrl + endpoint
	res, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("error getting response from url %v | %v", url, err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		resp := data.ApiResp{Result: target}
		respMap := map[string]interface{}{}
		msg, err := io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("error calling io.readall | %v", err)
		}
		err = json.Unmarshal(msg, &resp)
		if err != nil {
			return fmt.Errorf("error unmarshaling msg to ApiResp | %v", err)
		}
		err = json.Unmarshal(msg, &respMap)
		if err != nil {
			return fmt.Errorf("error unmarshaling msg to map | %v", err)
		}
		if _, ok := respMap["result"]; !ok {
			return fmt.Errorf("api error | no \"result\" field")
		}
		if len(resp.Error) != 0 {
			return fmt.Errorf("api error(s) | %v", resp.Error)
		}
	} else {
		return fmt.Errorf("http status code not OK status code | %v", res.StatusCode)
	}
	return nil
}
