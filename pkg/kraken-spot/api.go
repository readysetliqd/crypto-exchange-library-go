package krakenspot

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"
	"sort"
	"strconv"

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
	allAssetInfo := make(map[string]data.AssetInfo, data.AssetsMapSize)
	err := callPublicApi("Assets", &allAssetInfo)
	if err != nil {
		return nil, err
	}
	return &allAssetInfo, nil
}

// Calls Kraken API public market data "Assets" endpoint. Returns a slice of
// strings of all tradeable asset names
func ListAssets() ([]string, error) {
	allAssetInfo := make(map[string]data.AssetInfo, data.AssetsMapSize)
	err := callPublicApi("Assets", &allAssetInfo)
	if err != nil {
		return nil, err
	}
	allAssets := []string{}
	for asset := range allAssetInfo {
		allAssets = append(allAssets, asset)
	}
	return allAssets, nil
}

// Calls Kraken API public market data "Assets" endpoint. Gets information about
// specific asset passed to arg.
func GetAssetInfo(asset string) (*data.AssetInfo, error) {
	assetInfo := make(map[string]data.AssetInfo)
	endpoint := "Assets?asset=" + asset
	err := callPublicApi(endpoint, &assetInfo)
	if err != nil {
		return nil, err
	}
	info := assetInfo[asset]
	return &info, nil
}

// Calls Kraken API public market data "AssetPairs" endpoint with default info
// query parameter. Calling function without arguments gets info for all tradable
// asset pairs. Accepts one optional argument for the "pair" query parameter. If
// multiple pairs are desired, pass them as one comma delimited string into the
// pair argument.
func GetTradeablePairsInfo(pair ...string) (*map[string]data.AssetPairInfo, error) {
	var initialCapacity int
	endpoint := "AssetPairs"
	if len(pair) > 0 {
		initialCapacity = 1
		if len(pair) > 1 {
			err := fmt.Errorf("too many arguments passed into getalltradeablepairs(). excpected 0 or 1")
			return nil, err
		}
		endpoint += "?pair=" + pair[0]
	} else {
		initialCapacity = data.PairsMapSize
	}
	pairInfo := make(map[string]data.AssetPairInfo, initialCapacity)
	err := callPublicApi(endpoint, &pairInfo)
	if err != nil {
		return nil, err
	}
	return &pairInfo, nil
}

// Calls Kraken API public market data "AssetPairs" endpoint with "margin" info
// query parameter. Calling function without arguments gets info for all tradable
// asset pairs. Accepts one optional argument for the "pair" query parameter. If
// multiple pairs are desired, pass them as one comma delimited string into the
// pair argument.
func GetTradeablePairsMargin(pair ...string) (*map[string]data.AssetPairMargin, error) {
	var initialCapacity int
	endpoint := "AssetPairs?info=margin"
	if len(pair) > 0 {
		initialCapacity = 1
		if len(pair) > 1 {
			err := fmt.Errorf("too many arguments passed into getalltradeablepairs(). excpected 0 or 1")
			return nil, err
		}
		endpoint += "&pair=" + pair[0]
	} else {
		initialCapacity = data.PairsMapSize
	}
	pairInfo := make(map[string]data.AssetPairMargin, initialCapacity)
	err := callPublicApi(endpoint, &pairInfo)
	if err != nil {
		return nil, err
	}
	return &pairInfo, nil
}

// Calls Kraken API public market data "AssetPairs" endpoint with "fees" info
// query parameter. Calling function without arguments gets info for all tradable
// asset pairs. Accepts one optional argument for the "pair" query parameter. If
// multiple pairs are desired, pass them as one comma delimited string into the
// pair argument.
func GetTradeablePairsFees(pair ...string) (*map[string]data.AssetPairFees, error) {
	var initialCapacity int
	endpoint := "AssetPairs?info=fees"
	if len(pair) > 0 {
		initialCapacity = 1
		if len(pair) > 1 {
			err := fmt.Errorf("too many arguments passed into getalltradeablepairs(). excpected 0 or 1")
			return nil, err
		}
		endpoint += "&pair=" + pair[0]
	} else {
		initialCapacity = data.PairsMapSize
	}
	pairInfo := make(map[string]data.AssetPairFees, initialCapacity)
	err := callPublicApi(endpoint, &pairInfo)
	if err != nil {
		return nil, err
	}
	return &pairInfo, nil
}

// Calls Kraken API public market data "AssetPairs" endpoint with "leverage" info
// query parameter. Calling function without arguments gets info for all tradable
// asset pairs. Accepts one optional argument for the "pair" query parameter. If
// multiple pairs are desired, pass them as one comma delimited string into the
// pair argument.
func GetTradeablePairsLeverage(pair ...string) (*map[string]data.AssetPairLeverage, error) {
	var initialCapacity int
	endpoint := "AssetPairs?info=leverage"
	if len(pair) > 0 {
		initialCapacity = 1
		if len(pair) > 1 {
			err := fmt.Errorf("too many arguments passed into getalltradeablepairs(). excpected 0 or 1")
			return nil, err
		}
		endpoint += "&pair=" + pair[0]
	} else {
		initialCapacity = data.PairsMapSize
	}
	pairInfo := make(map[string]data.AssetPairLeverage, initialCapacity)
	err := callPublicApi(endpoint, &pairInfo)
	if err != nil {
		return nil, err
	}
	return &pairInfo, nil
}

// Calls Kraken API public market data "AssetPairs" endpoint and returns slice
// of all tradeable pair names. Sorted alphabetically.
func ListTradeablePairs() ([]string, error) {
	tradeablePairs := []string{}
	pairInfo, err := GetTradeablePairsInfo()
	if err != nil {
		return nil, err
	}
	for pair := range *pairInfo {
		tradeablePairs = append(tradeablePairs, pair)
	}
	slices.Sort(tradeablePairs)
	return tradeablePairs, nil
}

// Calls Kraken API public market data "AssetPairs" endpoint and returns slice
// of all tradeable pairs' websocket names. Sorted alphabetically.
func ListWebsocketNames() ([]string, error) {
	pairInfo, err := GetTradeablePairsInfo()
	if err != nil {
		return nil, err
	}
	websocketNames := []string{}
	for pair := range *pairInfo {
		websocketNames = append(websocketNames, (*pairInfo)[pair].Wsname)
	}
	slices.Sort(websocketNames)
	return websocketNames, nil
}

// Calls Kraken API public market data "AssetPairs" endpoint and returns slice
// of all tradeable pairs' altnames. Sorted alphabetically.
func ListAltNames() ([]string, error) {
	pairInfo, err := GetTradeablePairsInfo()
	if err != nil {
		return nil, err
	}
	altNames := []string{}
	for pair := range *pairInfo {
		altNames = append(altNames, (*pairInfo)[pair].Altname)
	}
	slices.Sort(altNames)
	return altNames, nil
}

// Calls Kraken API public market data "AssetPairs" endpoint and returns map of
// all tradeable websocket names for fast lookup.
func MapWebsocketNames() (map[string]bool, error) {
	pairInfo, err := GetTradeablePairsInfo()
	if err != nil {
		return nil, err
	}
	websocketNames := make(map[string]bool, data.PairsMapSize)
	for pair := range *pairInfo {
		websocketNames[(*pairInfo)[pair].Wsname] = true
	}
	return websocketNames, nil
}

// Calls Kraken API public market data "Ticker" endpoint. Calling function
// without arguments gets tickers for all tradable asset pairs. Accepts one
// optional argument for the "pair" query parameter. If multiple pairs are
// desired, pass them as one comma delimited string into the pair argument.
//
// Note: Today's prices start at midnight UTC
func GetTickerInfo(pair ...string) (*map[string]data.TickerInfo, error) {
	var initialCapacity int
	endpoint := "Ticker"
	if len(pair) > 0 {
		initialCapacity = 1
		if len(pair) > 1 {
			err := fmt.Errorf("too many arguments passed into getalltradeablepairs(). excpected 0 or 1")
			return nil, err
		}
		endpoint += "?pair=" + pair[0]
	} else {
		initialCapacity = data.PairsMapSize
	}
	tickers := make(map[string]data.TickerInfo, initialCapacity)
	err := callPublicApi(endpoint, &tickers)
	if err != nil {
		return nil, err
	}
	return &tickers, nil
}

// Calls Kraken API public market data "Ticker" endpoint. Returns a slice of
// tickers sorted descending by their last 24 hour USD volume. Calling this
// function without passing a value to arg num will return the entire list
// of sorted pairs. Passing a value to num will return a slice of the top num
// sorted pairs.
func ListTopVolumeLast24Hours(num ...uint16) ([]data.TickerVolume, error) {
	if len(num) > 1 {
		err := fmt.Errorf("too many arguments passed into getalltradeablepairs(). excpected 0 or 1")
		return nil, err
	}
	topVolumeTickers := make([]data.TickerVolume, 0, data.TickersMapSize)
	tickers, err := GetTickerInfo()
	if err != nil {
		return nil, err
	}
	allPairs, err := GetTradeablePairsInfo()
	if err != nil {
		log.Println(err)
	}

	for ticker := range *tickers {
		if _, ok := (*allPairs)[ticker]; ok {
			usdEquivalents := map[string]bool{
				"DAI":   true,
				"PYUSD": true,
				"USDC":  true,
				"USDT":  true,
				"ZUSD":  true,
			}
			if usdEquivalents[(*allPairs)[ticker].Quote] {
				volume, vwap, err := parseVolumeVwap(ticker, ticker, tickers)
				if err != nil {
					return nil, err
				}
				topVolumeTickers = append(topVolumeTickers, data.TickerVolume{Ticker: ticker, Volume: vwap * volume})
				// Handle cases where USD is base currency
			} else if (*allPairs)[ticker].Base == "ZUSD" {
				volume, _, err := parseVolumeVwap(ticker, ticker, tickers)
				if err != nil {
					return nil, err
				}
				topVolumeTickers = append(topVolumeTickers, data.TickerVolume{Ticker: ticker, Volume: volume})
			} else {
				// Find matching pair with base and quote USD equivalent to normalize to USD volume
				if _, ok := (*allPairs)[(*allPairs)[ticker].Base+"ZUSD"]; ok {
					volume, vwap, err := parseVolumeVwap(ticker, (*allPairs)[ticker].Base+"ZUSD", tickers)
					if err != nil {
						return nil, err
					}
					topVolumeTickers = append(topVolumeTickers, data.TickerVolume{Ticker: ticker, Volume: vwap * volume})
				} else if _, ok := (*allPairs)[(*allPairs)[ticker].Base+"USD"]; ok {
					volume, vwap, err := parseVolumeVwap(ticker, (*allPairs)[ticker].Base+"USD", tickers)
					if err != nil {
						return nil, err
					}
					topVolumeTickers = append(topVolumeTickers, data.TickerVolume{Ticker: ticker, Volume: vwap * volume})
					// Handle edge cases specific to Kraken API base not matching data in tickers
				} else if (*allPairs)[ticker].Base == "XXDG" {
					volume, vwap, err := parseVolumeVwap(ticker, "XDGUSD", tickers)
					if err != nil {
						return nil, err
					}
					topVolumeTickers = append(topVolumeTickers, data.TickerVolume{Ticker: ticker, Volume: vwap * volume})
				} else if (*allPairs)[ticker].Base == "ZAUD" {
					volume, vwap, err := parseVolumeVwap(ticker, "AUDUSD", tickers)
					if err != nil {
						return nil, err
					}
					topVolumeTickers = append(topVolumeTickers, data.TickerVolume{Ticker: ticker, Volume: vwap * volume})
				}
			}
		}
	}
	// Sort by descending volume and cut slice to num length
	sort.Slice(topVolumeTickers, func(i, j int) bool {
		return topVolumeTickers[i].Volume > topVolumeTickers[j].Volume
	})
	if len(num) > 0 {
		if num[0] < uint16(len(topVolumeTickers)) {
			topVolumeTickers = topVolumeTickers[:num[0]]
		}
	}
	return topVolumeTickers, nil
}

// Calls Kraken API public market data "Ticker" endpoint. Returns a slice of
// tickers sorted descending by their last 24 hour number of trades. Calling
// this function without passing a value to arg num will return the entire list
// of sorted pairs. Passing a value to num will return a slice of the top num
// sorted pairs.
func ListTopNumberTradesLast24Hours(num ...uint16) ([]data.TickerTrades, error) {
	if len(num) > 1 {
		err := fmt.Errorf("too many arguments passed into ListTopNumberTradesLast24Hours(). excpected 0 or 1")
		return nil, err
	}
	topTradesTickers := make([]data.TickerTrades, 0, data.TickersMapSize)
	tickers, err := GetTickerInfo()
	if err != nil {
		return nil, err
	}
	for ticker := range *tickers {
		numTrades := (*tickers)[ticker].NumberOfTrades.Last24Hours
		topTradesTickers = append(topTradesTickers, data.TickerTrades{Ticker: ticker, NumTrades: numTrades})
	}
	sort.Slice(topTradesTickers, func(i, j int) bool {
		return topTradesTickers[i].NumTrades > topTradesTickers[j].NumTrades
	})
	if len(num) > 0 {
		if num[0] < uint16(len(topTradesTickers)) {
			topTradesTickers = topTradesTickers[:num[0]]
		}
	}
	return topTradesTickers, nil
}

// Parses volume and VWAP from tickers using ticker for volume and pair for VWAP
func parseVolumeVwap(ticker string, pair string, tickers *map[string]data.TickerInfo) (float64, float64, error) {
	volume, err := strconv.ParseFloat((*tickers)[ticker].Volume.Last24Hours, 64)
	if err != nil {
		return 0, 0, err
	}
	vwap, err := strconv.ParseFloat((*tickers)[pair].VWAP.Last24Hours, 64)
	if err != nil {
		return 0, 0, err
	}
	return volume, vwap, nil
}

// Calls Kraken API public market data "OHLC" endpoint. Gets OHLC data for
// specified pair of the required interval (in minutes).
//
// Accepts optional arg since as a start time in Unix for the data. However,
// per the Kraken API docs "Note: the last entry in the OHLC array is for the
// current, not-yet-committed frame and will always be present, regardless of
// the value of since.
//
// interval enum: 1, 5, 15, 30, 60, 240, 1440, 10080, 21600
func GetOHLC(pair string, interval uint16, since ...uint64) (*data.OHLCResp, error) {
	endpoint := fmt.Sprintf("OHLC?pair=%s&interval=%v", pair, interval)
	OHLC := &data.OHLCResp{}
	if len(since) > 0 {
		if len(since) > 1 {
			err := fmt.Errorf("too many arguments passed for arg: since func: GetOHLC(). excpected 0 or 1")
			return nil, err
		}
		endpoint += fmt.Sprintf("&since=%v", since)
	}
	callPublicApi(endpoint, OHLC)
	return OHLC, nil
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
