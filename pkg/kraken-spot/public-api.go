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
//
// # Example Usage:
//
//	status, err := krakenspot.GetSystemStatus()
//	log.Println(status.Status)
//	log.Println(status.Timestamp)
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
//
// # Example Usage:
//
//	online, status := krakenspot.SystemIsOnline()
//	if !online {
//		log.Println(status)
//	}
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
	allAssetInfo := make(map[string]data.AssetInfo, assetsMapSize)
	err := callPublicApi("Assets", &allAssetInfo)
	if err != nil {
		return nil, err
	}
	// Add ticker to each AssetInfo
	for ticker, info := range allAssetInfo {
		info.Ticker = ticker
		allAssetInfo[ticker] = info
	}
	return &allAssetInfo, nil
}

// Calls Kraken API public market data "Assets" endpoint. Returns a slice of
// strings of all tradeable asset names sorted alphabetically.
func ListAssets() ([]string, error) {
	allAssetInfo, err := GetAllAssetInfo()
	if err != nil {
		err = fmt.Errorf("error calling GetAllAssetInfo() | %w", err)
		return nil, err
	}
	allAssets := make([]string, len((*allAssetInfo)))
	i := 0
	for asset := range *allAssetInfo {
		allAssets[i] = asset
		i++
	}
	slices.Sort(allAssets)
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
	// Add ticker to each AssetInfo
	for ticker, info := range assetInfo {
		info.Ticker = ticker
		assetInfo[ticker] = info
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
		initialCapacity = pairsMapSize
	}
	pairInfo := make(map[string]data.AssetPairInfo, initialCapacity)
	err := callPublicApi(endpoint, &pairInfo)
	if err != nil {
		return nil, err
	}
	// Add ticker to each AssetPairInfo
	for ticker, info := range pairInfo {
		info.Ticker = ticker
		pairInfo[ticker] = info
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
		initialCapacity = pairsMapSize
	}
	pairInfo := make(map[string]data.AssetPairMargin, initialCapacity)
	err := callPublicApi(endpoint, &pairInfo)
	if err != nil {
		return nil, err
	}
	// Add ticker to each AssetPairMargin
	for ticker, info := range pairInfo {
		info.Ticker = ticker
		pairInfo[ticker] = info
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
		initialCapacity = pairsMapSize
	}
	pairInfo := make(map[string]data.AssetPairFees, initialCapacity)
	err := callPublicApi(endpoint, &pairInfo)
	if err != nil {
		return nil, err
	}
	// Add ticker to each AssetPairFees
	for ticker, info := range pairInfo {
		info.Ticker = ticker
		pairInfo[ticker] = info
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
		initialCapacity = pairsMapSize
	}
	pairInfo := make(map[string]data.AssetPairLeverage, initialCapacity)
	err := callPublicApi(endpoint, &pairInfo)
	if err != nil {
		return nil, err
	}
	// Add ticker to each AssetPairLeverage
	for ticker, info := range pairInfo {
		info.Ticker = ticker
		pairInfo[ticker] = info
	}
	return &pairInfo, nil
}

// Calls Kraken API public market data "AssetPairs" endpoint and returns slice
// of all tradeable pair names. Sorted alphabetically.
func ListTradeablePairs() ([]string, error) {
	pairInfo, err := GetTradeablePairsInfo()
	tradeablePairs := make([]string, len(*pairInfo))
	if err != nil {
		return nil, err
	}
	i := 0
	for pair := range *pairInfo {
		tradeablePairs[i] = pair
		i++
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
	websocketNames := make([]string, len(*pairInfo))
	i := 0
	for _, pair := range *pairInfo {
		websocketNames[i] = pair.Wsname
		i++
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
	altNames := make([]string, len(*pairInfo))
	i := 0
	for _, pair := range *pairInfo {
		altNames[i] = pair.Altname
		i++
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
	websocketNames := make(map[string]bool, pairsMapSize)
	for _, pair := range *pairInfo {
		websocketNames[pair.Wsname] = true
	}
	return websocketNames, nil
}

// Calls Kraken API public market data "Ticker" endpoint. Accepts one argument
// for the "pair" query parameter. If multiple pairs are desired, pass them as
// one comma delimited string into the pair argument.
//
// Note: Today's prices start at midnight UTC
func GetTickerInfo(pair string) (*map[string]data.TickerInfo, error) {
	initialCapacity := 1
	endpoint := "Ticker?pair=" + pair
	tickers := make(map[string]data.TickerInfo, initialCapacity)
	err := callPublicApi(endpoint, &tickers)
	if err != nil {
		return nil, err
	}
	// Add ticker to each TickerInfo
	for ticker, info := range tickers {
		info.Ticker = ticker
		tickers[ticker] = info
	}
	return &tickers, nil
}

// Calls Kraken API public market data "Ticker" endpoint. Gets ticker info for
// all tradeable pairs.
//
// Note: Today's prices start at midnight UTC
func GetAllTickerInfo() (*map[string]data.TickerInfo, error) {
	initialCapacity := pairsMapSize
	endpoint := "Ticker"
	tickers := make(map[string]data.TickerInfo, initialCapacity)
	err := callPublicApi(endpoint, &tickers)
	if err != nil {
		return nil, err
	}
	// Add ticker to each TickerInfo
	for ticker, info := range tickers {
		info.Ticker = ticker
		tickers[ticker] = info
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
	topVolumeTickers := make([]data.TickerVolume, 0, tickersMapSize)
	tickers, err := GetAllTickerInfo()
	if err != nil {
		return nil, err
	}
	allPairs, err := GetTradeablePairsInfo()
	if err != nil {
		return nil, err
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
	topTradesTickers := make([]data.TickerTrades, 0, tickersMapSize)
	tickers, err := GetAllTickerInfo()
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

// Calls Kraken API public market data "OHLC" endpoint. Gets OHLC data for
// specified pair of the required interval (in minutes).
//
// Accepts optional arg since as a start time in Unix for the data. However,
// per the Kraken API docs "Note: the last entry in the OHLC array is for the
// current, not-yet-committed frame and will always be present, regardless of
// the value of since.
//
// Enum - 'interval': 1, 5, 15, 30, 60, 240, 1440, 10080, 21600
func GetOHLC(pair string, interval uint16, since ...uint64) (*data.OHLCResp, error) {
	endpoint := fmt.Sprintf("OHLC?pair=%s&interval=%v", pair, interval)
	if len(since) > 0 {
		if len(since) > 1 {
			err := fmt.Errorf("too many arguments passed for func: GetOHLC(). excpected 2 or 3 including args 'pair' and 'interval'")
			return nil, err
		}
		endpoint += fmt.Sprintf("&since=%v", since)
	}
	var OHLC data.OHLCResp
	err := callPublicApi(endpoint, &OHLC)
	if err != nil {
		return nil, err
	}
	return &OHLC, nil
}

// Calls Kraken API public market data "Depth" endpoint. Gets arrays of bids and
// asks for arg 'pair'. Optional arg 'count' will return count number of each bids
// and asks. Not passing an arg to 'count' will default to 100.
//
// Enum - 'count': [1..500]
func GetOrderBook(pair string, count ...uint16) (*data.OrderBook, error) {
	var initialCapacity uint16
	endpoint := fmt.Sprintf("Depth?pair=%s", pair)
	if len(count) > 0 {
		if len(count) > 1 {
			err := fmt.Errorf("too many arguments passed for func: GetOrderBook(). expected 1 or 2 including the 'pair' argument")
			return nil, err
		}
		initialCapacity = count[0]
		if initialCapacity > 500 || count[0] < 1 {
			err := fmt.Errorf("invalid number passed to 'count'. check enum")
			return nil, err
		}
		endpoint += fmt.Sprintf("&count=%v", count[0])
	}
	orderBook := make(map[string]data.OrderBook, initialCapacity)
	callPublicApi(endpoint, &orderBook)
	var assetInfo data.OrderBook
	for key := range orderBook {
		assetInfo = orderBook[key]
		assetInfo.Ticker = key
		break
	}
	return &assetInfo, nil
}

// Calls Kraken API public market data "Trades" endpoint. Gets the most
// recent trades for arg 'pair'. Accepts optional arg 'count' to get 'count'
// number of recent trades. Not passing an arg to 'count' will default to 1000
//
// Enum - 'count': [1..1000]
func GetTrades(pair string, count ...uint16) (*data.TradesResp, error) {
	var initialCapacity uint16
	endpoint := "Trades?pair=" + pair
	if len(count) > 0 {
		if len(count) > 1 {
			err := fmt.Errorf("too many arguments passed for func: GetTrades(). excpected 1, or 2 including the 'pair' argument")
			return nil, err
		}
		initialCapacity = count[0]
		if initialCapacity > 1000 || initialCapacity < 1 {
			err := fmt.Errorf("invalid number passed to 'count'. check enum")
			return nil, err
		}
		endpoint += fmt.Sprintf("&count=%v", initialCapacity)
	} else {
		initialCapacity = 1000
	}
	trades := data.TradesResp{}
	err := callPublicApi(endpoint, &trades)
	if err != nil {
		return nil, err
	}
	return &trades, nil
}

// Calls Kraken API public market data "Trades" endpoint. Gets the trades after
// unix time arg 'since' for arg 'pair'. Accepts optional arg 'count' to get
// 'count' number of trades after 'since'. Not passing an arg to 'count' will
// default to 1000
//
// Enum - 'count': [1..1000]
func GetTradesSince(pair string, since uint64, count ...uint16) (*data.TradesResp, error) {
	var initialCapacity uint16
	endpoint := fmt.Sprintf("Trades?pair=%s&since=%v", pair, since)
	if len(count) > 0 {
		if len(count) > 1 {
			err := fmt.Errorf("too many arguments passed for func: GetTrades(). excpected 1, or 2 including the 'pair' argument")
			return nil, err
		}
		initialCapacity = count[0]
		if initialCapacity > 1000 || initialCapacity < 1 {
			err := fmt.Errorf("invalid number passed to 'count'. check enum")
			return nil, err
		}
		endpoint += fmt.Sprintf("&count=%v", initialCapacity)
	} else {
		initialCapacity = 1000
	}
	trades := data.TradesResp{}
	err := callPublicApi(endpoint, &trades)
	if err != nil {
		return nil, err
	}
	return &trades, nil
}

// Calls Kraken API public market data "Spread" endpoint. Returns the last ~200
// top-of-book spreads for given arg 'pair'.
//
// Note: arg 'since' intended for incremental updates within available dataset
// (does not contain all historical spreads)
func GetSpread(pair string, since ...uint64) (*data.SpreadResp, error) {
	endpoint := "Spread?pair=" + pair
	if len(since) > 0 {
		if len(since) > 1 {
			err := fmt.Errorf("too many arguments passed for func: GetSpread(). excpected 1, or 2 including the 'pair' argument")
			return nil, err
		}
		endpoint += fmt.Sprintf("&since=%v", since[0])
	}
	var resp data.SpreadResp
	err := callPublicApi(endpoint, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// Calls Kraken's public api endpoint. Arg 'endpoint' should match the url
// endpoint from the api docs. Arg 'target' interface{} should be a pointer to
// an empty struct of the matching endpoint data type
func callPublicApi(endpoint string, target interface{}) error {
	url := baseUrl + publicPrefix + endpoint
	res, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("error getting response from url %v | %v", url, err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		resp := data.ApiResp{Result: target}
		msg, err := io.ReadAll(res.Body)
		if err != nil {
			return fmt.Errorf("error calling io.readall | %v", err)
		}
		err = json.Unmarshal(msg, &resp)
		if err != nil {
			return fmt.Errorf("error unmarshaling msg to ApiResp | %v", err)
		}
		if len(resp.Error) != 0 {
			return fmt.Errorf("api error(s) | %v", resp.Error)
		}
	} else {
		return fmt.Errorf("http status code not OK status code | %v", res.StatusCode)
	}
	return nil
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
