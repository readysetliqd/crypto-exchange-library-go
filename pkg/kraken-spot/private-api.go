package krakenspot

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// #region Public Market Data endpoints

// Calls Kraken API public market data "Time" endpoint. Gets the server's time.
// ServerTime struct
func (kc *KrakenClient) GetServerTime() (*ServerTime, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))

	// Send request
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(publicPrefix+"Time", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var systemStatus ServerTime
	err = processAPIResponse(res, &systemStatus)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}

	return &systemStatus, nil
}

// Calls Kraken API public market data "SystemStatus" endpoint. Gets the current
// system status or trading mode
//
// # Example Usage:
//
//	status, err := krakenspot.GetSystemStatus()
//	log.Println(status.Status)
//	log.Println(status.Timestamp)
func (kc *KrakenClient) GetSystemStatus() (*SystemStatus, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))

	// Send request
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(publicPrefix+"SystemStatus", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var systemStatus SystemStatus
	err = processAPIResponse(res, &systemStatus)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}

	return &systemStatus, nil
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
func (kc *KrakenClient) SystemIsOnline() (bool, string, error) {
	systemStatus, err := kc.GetSystemStatus()
	if err != nil {
		return false, "error", fmt.Errorf("error calling GetSystemStatus() | %w", err)
	}
	if systemStatus.Status == "online" {
		return true, systemStatus.Status, nil
	}
	return false, systemStatus.Status, nil
}

// Calls Kraken API public market data "Assets" endpoint. Gets information about
// all assets that are available for deposit, withdrawal, trading and staking.
// Returns them as *map[string]AssetInfo where the string is the asset name.
func (kc *KrakenClient) GetAllAssetInfo() (*map[string]AssetInfo, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))

	// Send request
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(publicPrefix+"Assets", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	allAssetInfo := make(map[string]AssetInfo, assetsMapSize)
	err = processAPIResponse(res, &allAssetInfo)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
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
// strings of all tradeable asset names
func (kc *KrakenClient) ListAssets() ([]string, error) {
	allAssetInfo, err := kc.GetAllAssetInfo()
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
func (kc *KrakenClient) GetAssetInfo(asset string) (*AssetInfo, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))

	// Send request
	kc.rateLimitAndIncrement(1)
	endpoint := "Assets?asset=" + asset
	res, err := kc.doRequest(publicPrefix+endpoint, payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	assetInfo := make(map[string]AssetInfo)
	err = processAPIResponse(res, &assetInfo)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
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
func (kc *KrakenClient) GetTradeablePairsInfo(pair ...string) (*map[string]AssetPairInfo, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))

	// Build endpoint
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

	// Send request
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(publicPrefix+endpoint, payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	pairInfo := make(map[string]AssetPairInfo, initialCapacity)
	err = processAPIResponse(res, &pairInfo)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
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
func (kc *KrakenClient) GetTradeablePairsMargin(pair ...string) (*map[string]AssetPairMargin, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))

	// Build endpoint
	var initialCapacity int
	endpoint := "AssetPairs?info=margin"
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

	// Send request
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(publicPrefix+endpoint, payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	pairInfo := make(map[string]AssetPairMargin, initialCapacity)
	err = processAPIResponse(res, &pairInfo)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}

	// Add ticker to each AssetPairInfo
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
func (kc *KrakenClient) GetTradeablePairsFees(pair ...string) (*map[string]AssetPairFees, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))

	// Build endpoint
	var initialCapacity int
	endpoint := "AssetPairs?info=fees"
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

	// Send request
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(publicPrefix+endpoint, payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	pairInfo := make(map[string]AssetPairFees, initialCapacity)
	err = processAPIResponse(res, &pairInfo)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}

	// Add ticker to each AssetPairInfo
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
func (kc *KrakenClient) GetTradeablePairsLeverage(pair ...string) (*map[string]AssetPairLeverage, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))

	// Build endpoint
	var initialCapacity int
	endpoint := "AssetPairs?info=leverage"
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

	// Send request
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(publicPrefix+endpoint, payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	pairInfo := make(map[string]AssetPairLeverage, initialCapacity)
	err = processAPIResponse(res, &pairInfo)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}

	// Add ticker to each AssetPairInfo
	for ticker, info := range pairInfo {
		info.Ticker = ticker
		pairInfo[ticker] = info
	}
	return &pairInfo, nil
}

// Calls Kraken API public market data "AssetPairs" endpoint and returns slice
// of all tradeable pair names. Sorted alphabetically.
func (kc *KrakenClient) ListTradeablePairs() ([]string, error) {
	pairInfo, err := kc.GetTradeablePairsInfo()
	tradeablePairs := make([]string, len((*pairInfo)))
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
func (kc *KrakenClient) ListWebsocketNames() ([]string, error) {
	pairInfo, err := kc.GetTradeablePairsInfo()
	if err != nil {
		return nil, err
	}
	websocketNames := make([]string, len((*pairInfo)))
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
func (kc *KrakenClient) ListAltNames() ([]string, error) {
	pairInfo, err := kc.GetTradeablePairsInfo()
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
func (kc *KrakenClient) MapWebsocketNames() (map[string]bool, error) {
	pairInfo, err := kc.GetTradeablePairsInfo()
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
func (kc *KrakenClient) GetTickerInfo(pair string) (*map[string]TickerInfo, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))

	// Send request
	kc.rateLimitAndIncrement(1)
	endpoint := "Ticker?pair=" + pair
	res, err := kc.doRequest(publicPrefix+endpoint, payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	initialCapacity := 1
	tickers := make(map[string]TickerInfo, initialCapacity)
	err = processAPIResponse(res, &tickers)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
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
func (kc *KrakenClient) ListTopVolumeLast24Hours(num ...uint16) ([]TickerVolume, error) {
	if len(num) > 1 {
		err := fmt.Errorf("too many arguments passed into getalltradeablepairs(). excpected 0 or 1")
		return nil, err
	}
	topVolumeTickers := make([]TickerVolume, 0, tickersMapSize)
	tickers, err := kc.GetAllTickerInfo()
	if err != nil {
		return nil, err
	}
	allPairs, err := kc.GetTradeablePairsInfo()
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
				topVolumeTickers = append(topVolumeTickers, TickerVolume{Ticker: ticker, Volume: vwap * volume})
				// Handle cases where USD is base currency
			} else if (*allPairs)[ticker].Base == "ZUSD" {
				volume, _, err := parseVolumeVwap(ticker, ticker, tickers)
				if err != nil {
					return nil, err
				}
				topVolumeTickers = append(topVolumeTickers, TickerVolume{Ticker: ticker, Volume: volume})
			} else {
				// Find matching pair with base and quote USD equivalent to normalize to USD volume
				if _, ok := (*allPairs)[(*allPairs)[ticker].Base+"ZUSD"]; ok {
					volume, vwap, err := parseVolumeVwap(ticker, (*allPairs)[ticker].Base+"ZUSD", tickers)
					if err != nil {
						return nil, err
					}
					topVolumeTickers = append(topVolumeTickers, TickerVolume{Ticker: ticker, Volume: vwap * volume})
				} else if _, ok := (*allPairs)[(*allPairs)[ticker].Base+"USD"]; ok {
					volume, vwap, err := parseVolumeVwap(ticker, (*allPairs)[ticker].Base+"USD", tickers)
					if err != nil {
						return nil, err
					}
					topVolumeTickers = append(topVolumeTickers, TickerVolume{Ticker: ticker, Volume: vwap * volume})
					// Handle edge cases specific to Kraken API base not matching data in tickers
				} else if (*allPairs)[ticker].Base == "XXDG" {
					volume, vwap, err := parseVolumeVwap(ticker, "XDGUSD", tickers)
					if err != nil {
						return nil, err
					}
					topVolumeTickers = append(topVolumeTickers, TickerVolume{Ticker: ticker, Volume: vwap * volume})
				} else if (*allPairs)[ticker].Base == "ZAUD" {
					volume, vwap, err := parseVolumeVwap(ticker, "AUDUSD", tickers)
					if err != nil {
						return nil, err
					}
					topVolumeTickers = append(topVolumeTickers, TickerVolume{Ticker: ticker, Volume: vwap * volume})
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

// Calls Kraken API public market data "Ticker" endpoint. Gets ticker info for
// all tradeable pairs.
//
// Note: Today's prices start at midnight UTC
func (kc *KrakenClient) GetAllTickerInfo() (*map[string]TickerInfo, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))

	// Send request
	kc.rateLimitAndIncrement(1)
	endpoint := "Ticker"
	res, err := kc.doRequest(publicPrefix+endpoint, payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	initialCapacity := pairsMapSize
	tickers := make(map[string]TickerInfo, initialCapacity)
	err = processAPIResponse(res, &tickers)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
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
// tickers sorted descending by their last 24 hour number of trades. Calling
// this function without passing a value to arg num will return the entire list
// of sorted pairs. Passing a value to num will return a slice of the top num
// sorted pairs.
func (kc *KrakenClient) ListTopNumberTradesLast24Hours(num ...uint16) ([]TickerTrades, error) {
	if len(num) > 1 {
		err := fmt.Errorf("too many arguments passed into ListTopNumberTradesLast24Hours(). excpected 0 or 1")
		return nil, err
	}
	topTradesTickers := make([]TickerTrades, 0, tickersMapSize)
	tickers, err := kc.GetAllTickerInfo()
	if err != nil {
		return nil, err
	}
	for ticker := range *tickers {
		numTrades := (*tickers)[ticker].NumberOfTrades.Last24Hours
		topTradesTickers = append(topTradesTickers, TickerTrades{Ticker: ticker, NumTrades: numTrades})
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
// Accepts optional arg since as a start time in Unix for the  However,
// per the Kraken API docs "Note: the last entry in the OHLC array is for the
// current, not-yet-committed frame and will always be present, regardless of
// the value of since.
//
// Enum - 'interval': 1, 5, 15, 30, 60, 240, 1440, 10080, 21600
func (kc *KrakenClient) GetOHLC(pair string, interval uint16, since ...uint64) (*OHLCResp, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))

	// Build endpoint
	endpoint := fmt.Sprintf("OHLC?pair=%s&interval=%v", pair, interval)
	if len(since) > 0 {
		if len(since) > 1 {
			err := fmt.Errorf("too many arguments passed for func: GetOHLC(). excpected 2 or 3 including args 'pair' and 'interval'")
			return nil, err
		}
		endpoint += fmt.Sprintf("&since=%v", since)
	}

	// Send request
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(publicPrefix+endpoint, payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var OHLC OHLCResp
	err = processAPIResponse(res, &OHLC)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &OHLC, nil
}

// Calls Kraken API public market data "Depth" endpoint. Gets arrays of bids and
// asks for arg 'pair'. Optional arg 'count' will return count number of each bids
// and asks. Not passing an arg to 'count' will default to 100.
//
// Enum - 'count': [1..500]
func (kc *KrakenClient) GetOrderBook(pair string, count ...uint16) (*OrderBook, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))

	// Build endpoint
	var initialCapacity uint16
	endpoint := fmt.Sprintf("Depth?pair=%s", pair)
	if len(count) > 0 {
		if len(count) > 1 {
			err := fmt.Errorf("too many arguments passed for func: GetOrderBook(). expected 1 or 2 including the 'pair' argument")
			return nil, err
		}
		initialCapacity = count[0]
		if initialCapacity > 500 || initialCapacity < 1 {
			err := fmt.Errorf("invalid number passed to 'count'. check enum")
			return nil, err
		}
		endpoint += fmt.Sprintf("&count=%v", count[0])
	}

	// Send request
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(publicPrefix+endpoint, payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	orderBook := make(map[string]OrderBook, initialCapacity)
	err = processAPIResponse(res, &orderBook)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}

	// Add pair name (key) as value
	var assetInfo OrderBook
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
func (kc *KrakenClient) GetTrades(pair string, count ...uint16) (*TradesResp, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))

	// Build endpoint
	endpoint := "Trades?pair=" + pair
	if len(count) > 0 {
		if len(count) > 1 {
			err := fmt.Errorf("too many arguments passed for func: GetTrades(). excpected 1, or 2 including the 'pair' argument")
			return nil, err
		}
		if count[0] > 1000 || count[0] < 1 {
			err := fmt.Errorf("invalid number passed to 'count'. check enum")
			return nil, err
		}
		endpoint += fmt.Sprintf("&count=%v", count[0])
	}
	// Send request
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(publicPrefix+endpoint, payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var trades TradesResp
	err = processAPIResponse(res, &trades)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
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
func (kc *KrakenClient) GetTradesSince(pair string, since uint64, count ...uint16) (*TradesResp, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))

	// Build endpoint
	endpoint := fmt.Sprintf("Trades?pair=%s&since=%v", pair, since)
	if len(count) > 0 {
		if len(count) > 1 {
			err := fmt.Errorf("too many arguments passed for func: GetTrades(). excpected 1, or 2 including the 'pair' argument")
			return nil, err
		}
		if count[0] > 1000 || count[0] < 1 {
			err := fmt.Errorf("invalid number passed to 'count'. check enum")
			return nil, err
		}
		endpoint += fmt.Sprintf("&count=%v", count[0])
	}
	// Send request
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(publicPrefix+endpoint, payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var trades TradesResp
	err = processAPIResponse(res, &trades)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &trades, nil
}

// Calls Kraken API public market data "Spread" endpoint. Returns the last ~200
// top-of-book spreads for given arg 'pair'.
//
// Note: arg 'since' intended for incremental updates within available dataset
// (does not contain all historical spreads)
func (kc *KrakenClient) GetSpread(pair string, since ...uint64) (*SpreadResp, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))

	// Build endpoint
	endpoint := "Spread?pair=" + pair
	if len(since) > 0 {
		if len(since) > 1 {
			err := fmt.Errorf("too many arguments passed for func: GetSpread(). excpected 1, or 2 including the 'pair' argument")
			return nil, err
		}
		endpoint += fmt.Sprintf("&since=%v", since[0])
	}

	// Send request
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(publicPrefix+endpoint, payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var spreads SpreadResp
	err = processAPIResponse(res, &spreads)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &spreads, nil
}

// #endregion

// #region Authenticated Account Data endpoints

// Calls Kraken API private Account Data "Balance" endpoint. Returns map of all
// "cash" (including coins) balances, net of pending withdrawals as strings
//
// # Required Permissions:
//
// Funding Permissions - Query;
func (kc *KrakenClient) GetAccountBalances() (*map[string]string, error) {
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"Balance", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()
	var balances map[string]string
	err = processAPIResponse(res, &balances)
	if err != nil {
		return nil, err
	}
	return &balances, nil
}

// Calls Kraken API private Account Data "Balance" endpoint. Returns float of
// total USD balance "ZUSD"
//
// # Required Permissions:
//
// Funding Permissions - Query;
func (kc *KrakenClient) TotalUSDBalance() (float64, error) {
	balances, err := kc.GetAccountBalances()
	if err != nil {
		return 0.0, nil
	}
	usdBal, err := strconv.ParseFloat((*balances)["ZUSD"], 64)
	if err != nil {
		return 0.0, err
	}
	return usdBal, nil
}

// Calls Kraken API private Account Data "BalanceEx" endpoint. Returns map of all
// extended account balances, including credits and held amounts.
//
// # Required Permissions:
//
// Funding Permissions - Query;
func (kc *KrakenClient) GetExtendedBalances() (*map[string]ExtendedBalance, error) {
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))

	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"BalanceEx", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()
	var balances map[string]ExtendedBalance
	err = processAPIResponse(res, &balances)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &balances, nil
}

// Calls Kraken API private Account Data "BalanceEx" endpoint. Returns map of all
// available account balances as float64. Balance available for trading is
// calculated as: available balance = balance + credit - credit_used - hold_trade
//
// # Required Permissions:
//
// Funding Permissions - Query;
func (kc *KrakenClient) GetAvailableBalances() (*map[string]float64, error) {
	balances, err := kc.GetExtendedBalances()
	if err != nil {
		err = fmt.Errorf("error calling GetExtendedBalances() | %w", err)
		return nil, err
	}
	availableBalances := make(map[string]float64, len(*balances))
	assets, err := GetAllAssetInfo()
	if err != nil {
		err = fmt.Errorf("error calling GetAllAssetInfo | %w", err)
		return nil, err
	}
	for coin, balance := range *balances {
		total, err := parseFloat64(balance.Balance)
		if err != nil {
			return nil, err
		}
		credit, err := parseFloat64(balance.Credit)
		if err != nil {
			return nil, err
		}
		creditUsed, err := parseFloat64(balance.CreditUsed)
		if err != nil {
			return nil, err
		}
		holdTrade, err := parseFloat64(balance.HoldTrade)
		if err != nil {
			return nil, err
		}
		asset, ok := (*assets)[coin]
		if !ok {
			availableBalances[coin] = total + credit - creditUsed - holdTrade
		} else {
			format := fmt.Sprintf("%%.%df", asset.Decimals)
			str := fmt.Sprintf(format, total+credit-creditUsed-holdTrade)
			availableBalances[coin], err = strconv.ParseFloat(str, 64)
			if err != nil {
				err = fmt.Errorf("ParseFloat error | %w", err)
				return nil, err
			}
		}

	}
	return &availableBalances, nil
}

// Calls Kraken API private Account Data "BalanceEx" endpoint. Returns available
// USD (ZUSD) account balance as float64. Balance available for trading is
// calculated as: available balance = balance + credit - credit_used - hold_trade
//
// # Required Permissions:
//
// Funding Permissions - Query;
func (kc *KrakenClient) AvailableUSDBalance() (float64, error) {
	balances, err := kc.GetExtendedBalances()
	if err != nil {
		err = fmt.Errorf("error calling GetExtendedBalances() | %w", err)
		return 0.0, err
	}
	usdExtBalance := (*balances)["ZUSD"]
	total, err := parseFloat64(usdExtBalance.Balance)
	if err != nil {
		return 0.0, err
	}
	credit, err := parseFloat64(usdExtBalance.Credit)
	if err != nil {
		return 0.0, err
	}
	creditUsed, err := parseFloat64(usdExtBalance.CreditUsed)
	if err != nil {
		return 0.0, err
	}
	holdTrade, err := parseFloat64(usdExtBalance.HoldTrade)
	if err != nil {
		return 0.0, err
	}
	usdAvailableBalance, err := parseFloat64(fmt.Sprintf(usdDecimalsFormat, total+credit-creditUsed-holdTrade))
	if err != nil {
		return 0.0, err
	}
	return usdAvailableBalance, nil
}

// Calls Kraken API private Account Data "TradeBalance" endpoint. Returns a summary
// of collateral balances, margin position valuations, equity and margin level
// denominated in arg 'asset'. Passing no arg to 'asset' defaults to USD (ZUSD)
// denomination.
//
// # Required Permissions:
//
// Funding Permissions - Query;
//
// Order and Trades - Query open orders & trades
func (kc *KrakenClient) GetTradeBalance(asset ...string) (*TradeBalance, error) {
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	if len(asset) > 0 {
		if len(asset) > 1 {
			err := fmt.Errorf("invalid number of args passed to 'asset'; expected 0 or 1")
			return nil, err
		}
		payload.Add("asset", asset[0])
	}
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"TradeBalance", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()
	var balance TradeBalance
	err = processAPIResponse(res, &balance)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &balance, nil
}

// Calls Kraken API private Account Data "OpenOrders" endpoint. Retrieves
// information for all currently open orders. Accepts functional options args
// 'options'.
//
// # Required Permissions:
//
// Order and Trades - Query open orders & trades;
//
// # Functional Options:
//
//	// Whether or not to include trades related to position in output. Defaults
//	// to false if not called
//	func OOWithTrades(trades bool) GetOpenOrdersOption
//
//	// Restrict results to given user reference id. Defaults to no restrictions
//	// if not called
//	func OOWithUserRef(userRef int) GetOpenOrdersOption
//
// # Example Usage:
//
//	orders, err := kc.GetOpenOrders(krakenspot.OOWithTrades(true), krakenspot.OOWithUserRef(123))
func (kc *KrakenClient) GetOpenOrders(options ...GetOpenOrdersOption) (*OpenOrdersResp, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	for _, option := range options {
		option(payload)
	}

	// Send Request to Kraken API
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"OpenOrders", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var openOrders OpenOrdersResp
	err = processAPIResponse(res, &openOrders)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &openOrders, nil
}

// Calls Kraken API private Account Data "OpenOrders" endpoint with default
// parameters filtered by orders for arg 'pair'. Returns a map of (key) transaction
// IDs and (value) its order information of all open orders for the pair.
//
// # Required Permissions:
//
// Order and Trades - Query open orders & trades;
//
// # Example Usage:
//
//	orders, err := kc.GetOpenOrdersForPair("SOLUSD")
func (kc *KrakenClient) GetOpenOrdersForPair(pair string) (*map[string]Order, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))

	// Send Request to Kraken API
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"OpenOrders", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var openOrders OpenOrdersResp
	err = processAPIResponse(res, &openOrders)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}

	pairOpenOrders := make(map[string]Order)
	for txid, order := range openOrders.OpenOrders {
		if order.Description.Pair == pair {
			pairOpenOrders[txid] = order
		}
	}

	return &pairOpenOrders, nil
}

// Calls Kraken API private Account Data "OpenOrders" endpoint with default
// parameters filtered by orders for arg 'pair'. Returns a slice of transaction
// IDs of all open orders for the pair.
//
// # Required Permissions:
//
// Order and Trades - Query open orders & trades;
//
// # Example Usage:
//
//	orders, err := kc.ListOpenTxIDsForPair("SOLUSD")
func (kc *KrakenClient) ListOpenTxIDsForPair(pair string) ([]string, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))

	// Send Request to Kraken API
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"OpenOrders", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var openOrders OpenOrdersResp
	err = processAPIResponse(res, &openOrders)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}

	// Build slice
	var pairOpenTxIDs []string
	for txID, order := range openOrders.OpenOrders {
		if order.Description.Pair == pair {
			pairOpenTxIDs = append(pairOpenTxIDs, txID)
		}
	}

	return pairOpenTxIDs, nil
}

// Calls Kraken API private Account Data "ClosedOrders" endpoint. Retrieves
// information for most recent closed orders. Accepts functional options args
// 'options'.
//
// # Required Permissions:
//
// Order and Trades - Query closed orders & trades;
//
// # Functional Options:
//
//	// Whether or not to include trades related to position in output. Defaults
//	// to false if not called
//	func COWithTrades(trades bool) GetClosedOrdersOption
//
//	// Restrict results to given user reference id. Defaults to no restrictions
//	// if not called
//	func COWithUserRef(userRef int) GetClosedOrdersOption
//
//	// Starting unix timestamp or order tx ID of results (exclusive). If an order's
//	// tx ID is given for start or end time, the order's opening time (opentm) is used.
//	// Defaults to show most recent orders if not called
//	func COWithStart(start int) GetClosedOrdersOption
//
//	// Ending unix timestamp or order tx ID of results (exclusive). If an order's
//	// tx ID is given for start or end time, the order's opening time (opentm) is used
//	// Defaults to show most recent orders if not called
//	func COWithEnd(end int) GetClosedOrdersOption
//
//	// Result offset for pagination. Defaults to no offset if not called
//	func COWithOffset(offset int) GetClosedOrdersOption
//
//	// Which time to use to search and filter results for COWithStart() and COWithEnd()
//	// Defaults to "both" if not called or invalid arg 'closeTime' passed
//	//
//	// Enum: "open", "close", "both"
//	func COWithCloseTime(closeTime string) GetClosedOrdersOption
//
//	// Whether or not to consolidate trades by individual taker trades. Defaults to
//	// true if not called
//	func COWithConsolidateTaker(consolidateTaker bool) GetClosedOrdersOption
//
// # Example Usage:
//
//	orders, err := kc.GetClosedOrders(krakenspot.COWithConsolidateTaker(true), krakenspot.COWithCloseTime("open"))
func (kc *KrakenClient) GetClosedOrders(options ...GetClosedOrdersOption) (*ClosedOrdersResp, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	for _, option := range options {
		option(payload)
	}

	// Send Request to Kraken API
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"ClosedOrders", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var closedOrders ClosedOrdersResp
	err = processAPIResponse(res, &closedOrders)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &closedOrders, nil
}

// Calls Kraken API private Account Data "OrdersInfo" endpoint. Retrieves order
// information for specific orders with transaction id passed to arg 'txID'.
// Accepts multiple orders with transaction IDs passed as a single comma delimited
// string with no white-space (50 maximum). Accepts functional options args 'options'.
//
// # Required Permissions:
//
// Order and Trades - Query closed orders & trades;
//
// # Functional Options:
//
//	// Whether or not to include trades related to position in output. Defaults
//	// to false if not called
//	func OIWithTrades(trades bool) GetOrdersInfoOptions
//
//	// Restrict results to given user reference id. Defaults to no restrictions
//	// if not called
//	func OIWithUserRef(userRef int) GetOrdersInfoOptions
//
//	// Whether or not to consolidate trades by individual taker trades. Defaults to
//	// true if not called
//	func OIWithConsolidateTaker(consolidateTaker bool)
//
// Example usage:
//
//	orders, err := kc.GetOrdersInfo("OYR15S-VHRBC-VY5NA2,OYBGFG-LQHXB-RJHY4C", krakenspot.OIWithConsolidateTaker(true))
func (kc *KrakenClient) GetOrdersInfo(txID string, options ...GetOrdersInfoOption) (*map[string]Order, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("txid", txID)
	for _, option := range options {
		option(payload)
	}

	// Send request
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"QueryOrders", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var queriedOrders map[string]Order
	err = processAPIResponse(res, &queriedOrders)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &queriedOrders, nil
}

// Calls Kraken API private Account Data "TradesHistory" endpoint. Retrieves
// information about trades/fills. 50 results are returned at a time, the most
// recent by default.
//
// # Required Permissions:
//
// Order and Trades - Query closed orders & trades;
//
// # Functional Options:
//
//	// Type of trade. Defaults to "all" if not called or invalid 'tradeType' passed.
//	//
//	// Enum: "all", "any position", "closed position", "closing position", "no position"
//	func THWithType(tradeType string) GetTradesHistoryOptions
//
//	// Whether or not to include trades related to position in output. Defaults
//	// to false if not called
//	func THWithTrades(trades bool) GetTradesHistoryOptions
//
//	// Starting unix timestamp or order tx ID of results (exclusive). If an order's
//	// tx ID is given for start or end time, the order's opening time (opentm) is used.
//	// Defaults to show most recent orders if not called
//	func THWithStart(start int) GetTradesHistoryOptions
//
//	// Ending unix timestamp or order tx ID of results (exclusive). If an order's
//	// tx ID is given for start or end time, the order's opening time (opentm) is used
//	// Defaults to show most recent orders if not called
//	func THWithEnd(end int) GetTradesHistoryOptions
//
//	// Result offset for pagination. Defaults to no offset if not called
//	func THWithOffset(offset int) GetTradesHistoryOptions
//
//	// Whether or not to consolidate trades by individual taker trades. Defaults to
//	// true if not called
//	func THWithConsolidateTaker(consolidateTaker bool) GetTradesHistoryOptions
//
// # Example Usage:
//
//	trades, err := kc.GetTradesHistory(krakenspot.THWithType("closed position"), krakenspot.THWithOffset(2))
func (kc *KrakenClient) GetTradesHistory(options ...GetTradesHistoryOption) (*TradesHistoryResp, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	for _, option := range options {
		option(payload)
	}

	// Send request
	kc.rateLimitAndIncrement(2)
	res, err := kc.doRequest(privatePrefix+"TradesHistory", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var tradesHistory TradesHistoryResp
	err = processAPIResponse(res, &tradesHistory)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &tradesHistory, nil
}

// Calls Kraken API private Account Data "QueryTrades" endpoint. Retrieves
// information specific trades/fills with transaction id passed to arg 'txID'.
// Accepts multiple trades with transaction IDs passed as a single comma delimited
// string with no white-space (20 maximum). Accepts functional options args 'options'.
//
// # Required Permissions:
//
// Order and Trades - Query closed orders & trades;
//
// # Functional Options:
//
//	// Whether or not to include trades related to position in output. Defaults
//	// to false if not called
//	func TIWithTrades(trades bool) GetTradeInfoOption
//
// # Example Usage:
//
//	trades, err := kc.GetTradeInfo("TRWCIF-3MJWU-5DYJG5,TNGJFU-5CD67-ZV3AEO")
func (kc *KrakenClient) GetTradeInfo(txID string, options ...GetTradeInfoOption) (*map[string]TradeInfo, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("txid", txID)
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"QueryTrades", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var tradeInfo map[string]TradeInfo
	err = processAPIResponse(res, &tradeInfo)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &tradeInfo, nil
}

// Calls Kraken API private Account Data "OpenPositions" endpoint. Gets information
// about open margin positions. Accepts functional options args 'options'.
//
// # Required Permissions:
//
// Order and Trades - Query open orders & trades;
//
// # Functional Options:
//
//	// Comma delimited list of txids to limit output to. Defaults to show all open
//	// positions if not called
//	func OPWithTxID(txID string) GetOpenPositionsOption
//
//	// Whether to include P&L calculations. Defaults to false if not called
//	func OPWithDoCalcs(doCalcs bool) GetOpenPositionsOption
//
// # Example Usage:
//
//	positions, err := kc.GetOpenPositions(krakenspot.OPWithDoCalcs(true))
func (kc *KrakenClient) GetOpenPositions(options ...GetOpenPositionsOption) (*map[string]OpenPosition, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"OpenPositions", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var openPositions map[string]OpenPosition
	err = processAPIResponse(res, &openPositions)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &openPositions, nil
}

// Calls Kraken API private Account Data "OpenPositions" endpoint. Gets information
// about open margin positions consolidated by market/pair. Accepts functional
// options args 'options'.
//
// # Required Permissions:
//
// Order and Trades - Query open orders & trades;
//
// # Functional Options:
//
//	// Comma delimited list of txids to limit output to. Defaults to show all open
//	// positions if not called
//	func OPCWithTxID(txID string) GetOpenPositionsOption
//
//	// Whether to include P&L calculations. Defaults to false if not called
//	func OPCWithDoCalcs(doCalcs bool) GetOpenPositionsOption
//
// # Example Usage:
//
//	positions, err := kc.GetOpenPositionsConsolidated(krakenspot.OPWithDoCalcs(true))
func (kc *KrakenClient) GetOpenPositionsConsolidated(options ...GetOpenPositionsConsolidatedOption) (*[]OpenPositionConsolidated, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("consolidation", "market")
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"OpenPositions", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var openPositions []OpenPositionConsolidated
	err = processAPIResponse(res, &openPositions)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &openPositions, nil
}

// Calls Kraken API private Account Data "Ledgers" endpoint. Retrieves information
// about ledger entries. 50 results are returned at a time, the most recent by
// default. Accepts functional options args 'options'.
//
// # Required Permissions:
//
// Data - Query ledger entries;
//
// # Functional Options:
//
//	// Filter output by asset or comma delimited list of assets. Defaults to "all"
//	// if not called.
//	func LIWithAsset(asset string) GetLedgersInfoOption
//
//	// Filter output by asset class. Defaults to "currency" if not called
//	//
//	// Enum: "currency", ...?
//	func LIWithAclass(aclass string) GetLedgersInfoOption
//
//	// Type of ledger to retrieve. Defaults to "all" if not called or invalid
//	// 'ledgerType' passed.
//	//
//	// Enum: "all", "trade", "deposit", "withdrawal", "transfer", "margin", "adjustment",
//	// "rollover", "credit", "settled", "staking", "dividend", "sale", "nft_rebate"
//	func LIWithType(ledgerType string) GetLedgersInfoOption
//
//	// Starting unix timestamp or ledger ID of results (exclusive). Defaults to most
//	// recent ledgers if not called.
//	func LIWithStart(start int) GetLedgersInfoOption
//
//	// Ending unix timestamp or ledger ID of results (inclusive). Defaults to most
//	// recent ledgers if not called.
//	func LIWithEnd(end int) GetLedgersInfoOption
//
//	// Result offset for pagination. Defaults to no offset if not called.
//	func LIWithOffset(offset int) GetLedgersInfoOption
//
//	// If true, does not retrieve count of ledger entries. Request can be noticeably
//	// faster for users with many ledger entries as this avoids an extra database query.
//	// Defaults to false if not called.
//	func LIWithoutCount(withoutCount bool) GetLedgersInfoOption
//
// # Example Usage:
//
//	ledgers, err := kc.GetLedgersInfo(krakenspot.LIWithAsset("ZUSD,XXBT"), krakenspot.LIWithoutCount(true), krakenspot.LIWithOffset(5))
func (kc *KrakenClient) GetLedgersInfo(options ...GetLedgersInfoOption) (*LedgersInfoResp, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	kc.rateLimitAndIncrement(2)
	res, err := kc.doRequest(privatePrefix+"Ledgers", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var ledgersInfo LedgersInfoResp
	err = processAPIResponse(res, &ledgersInfo)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &ledgersInfo, nil
}

// Calls Kraken API private Account Data "QueryLedgers" endpoint. Retrieves
// information about specific ledger entries passed to arg 'ledgerID'. Accepts
// multiple ledgers with ledger IDs passed as a single comma delimited string
// with no white-space (20 maximum). Accepts functional options args 'options'.
//
// # Required Permissions:
//
// Data - Query ledger entries;
//
// # Functional Options:
//
//	// Whether or not to include trades related to position in output. Defaults to
//	// false if not called.
//	func GLWithTrades(trades bool) GetLedgerOption
//
// # Example Usage:
//
//	ledger, err := kc.GetLedger("LGBRJU-SQZ4L-5HLS3C,L3S26P-BHIOV-TTWYYI", krakenspot.GLWithTrades(true))
func (kc *KrakenClient) GetLedger(ledgerID string, options ...GetLedgerOption) (*map[string]Ledger, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("id", ledgerID)
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"QueryLedgers", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var ledgersInfo map[string]Ledger
	err = processAPIResponse(res, &ledgersInfo)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &ledgersInfo, nil
}

// Calls Kraken API private Account Data "TradeVolume" endpoint. Returns 30 day
// USD trading volume and resulting fee schedule for any asset pair(s) provided.
// Note: If an asset pair is on a maker/taker fee schedule, the taker side is
// given in fees and maker side in fees_maker. For pairs not on maker/taker, they
// will only be given in fees. Accepts functional options args 'options'.
//
// # Required Permissions:
//
// Funds permissions - Query;
//
// # Functional Options:
//
//	// Comma delimited list of asset pairs to get fee info on. Defaults to show none
//	// if not called.
//	func TVWithPair(pair string) GetTradeVolumeOption
//
// # Example Usage:
//
//	kc.GetTradeVolume(krakenspot.TVWithPair("XXBTZUSD,XETHZUSD"))
func (kc *KrakenClient) GetTradeVolume(options ...GetTradeVolumeOption) (*TradeVolume, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"TradeVolume", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var tradeVolume TradeVolume
	err = processAPIResponse(res, &tradeVolume)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &tradeVolume, nil
}

// Calls Kraken API private Account Data "AddExport" endpoint. Requests export
// of trades data to file, defaults to CSV file type. Returns string containing
// report ID or empty string if encountered error. Accepts functional options
// args 'options'.
//
// # Required Permissions:
//
// Orders and trades - Query open orders and trades;
// Orders and trades - Query closed orders and trades;
// Data - Export data;
//
// # Functional Options:
//
//	// File format to export. Defaults to "CSV" if not called or invalid value
//	// passed to arg 'format'
//	//
//	// Enum: "CSV", "TSV"
//	func RTWithFormat(format string) RequestTradesExportReportOption
//
//	// Accepts comma-delimited list of fields passed as a single string to include
//	// in report. Defaults to "all" if not called. API will return error:
//	// [EGeneral:Internal error] if invalid value passed to arg 'fields'. Function has
//	// no validation checks for passed value to 'fields'
//	//
//	// Enum: "ordertxid", "time", "ordertype", "price", "cost", "fee", "vol",
//	// "margin", "misc", "ledgers"
//	func RTWithFields(fields string) RequestTradesExportReportOption
//
//	// UNIX timestamp for report start time. Defaults to 1st of the current month
//	// if not called
//	func RTWithStart(start int) RequestTradesExportReportOption
//
//	// UNIX timestamp for report end time. Defaults to current time if not called
//	func RTWithEnd(end int) RequestTradesExportReportOption
//
// # Example Usage:
//
//	reportID, err := kc.RequestTradesExportReport("January 2021 Trades", krakenspot.RTWithStart(1609459200), krakenspot.RTWithEnd(1612137600), krakenspot.RTWithFields("time,type,asset,amount,balance"))
func (kc *KrakenClient) RequestTradesExportReport(description string, options ...RequestTradesExportReportOption) (string, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("report", "trades")
	payload.Add("description", description)
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"AddExport", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return "", err
	}
	defer res.Body.Close()

	// Process API response
	var exportResp RequestExportReportResp
	err = processAPIResponse(res, &exportResp)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "EGeneral:Internal error") {
			err = fmt.Errorf("if RLWithFields() was passed to 'options', check 'fields' format is correct and 'fields' values matche enum | error calling processAPIResponse() | %w", err)
			return "", err
		} else {
			err = fmt.Errorf("error calling processAPIResponse() | %w", err)
			return "", err
		}
	}
	reportID := exportResp.ID
	return reportID, nil
}

// Calls Kraken API private Account Data "AddExport" endpoint. Requests export
// of ledgers data to file, defaults to CSV file type. Returns string containing
// report ID or empty string if encountered error. Accepts functional options
// args 'options'.
//
// # Required Permissions:
//
// Data - Query ledger entries;
// Data - Export data;
//
// # Functional Options:
//
//	// File format to export. Defaults to "CSV" if not called or invalid value
//	// passed to arg 'format'
//	//
//	// Enum: "CSV", "TSV"
//	func RLWithFormat(format string) RequestLedgersExportReportOption
//
//	// Accepts comma-delimited list of fields passed as a single string to include
//	// in report. Defaults to "all" if not called. API will return error:
//	// [EGeneral:Internal error] if invalid value passed to arg 'fields'. Function has
//	// no validation checks for passed value to 'fields'
//	//
//	// Enum: "refid", "time", "type", "aclass", "asset",
//	// "amount", "fee", "balance"
//	func RLWithFields(fields string) RequestLedgersExportReportOption
//
//	// UNIX timestamp for report start time. Defaults to 1st of the current month
//	// if not called
//	func RLWithStart(start int) RequestLedgersExportReportOption
//
//	// UNIX timestamp for report end time. Defaults to current time if not called
//	func RLWithEnd(end int) RequestLedgersExportReportOption
//
// # Example Usage:
//
//	reportID, err := kc.RequestLedgersExportReport("January 2021 Ledgers", krakenspot.RLWithStart(1609459200), krakenspot.RLWithEnd(1612137600), krakenspot.RLWithFields("time,type,asset,amount,balance"))
func (kc *KrakenClient) RequestLedgersExportReport(description string, options ...RequestLedgersExportReportOption) (string, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("report", "ledgers")
	payload.Add("description", description)
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"AddExport", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return "", err
	}
	defer res.Body.Close()

	// Process API response
	var exportResp RequestExportReportResp
	err = processAPIResponse(res, &exportResp)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "EGeneral:Internal error") {
			err = fmt.Errorf("if RLWithFields() was passed to 'options', check 'fields' format is correct and 'fields' values matche enum | error calling processAPIResponse() | %w", err)
			return "", err
		} else {
			err = fmt.Errorf("error calling processAPIResponse() | %w", err)
			return "", err
		}
	}
	reportID := exportResp.ID
	return reportID, nil
}

// Calls Kraken API private Account Data "ExportStatus" endpoint. Gets status of
// requested data exports. Requires arg 'reportType' of either "trades" or
// "ledgers".
//
// Note: Kraken API requires valid 'reportType' to be passed. According to Kraken
// API docs, this will filter results and only get status for reports of type
// 'reportType'. As of 1/5/2024, this parameter does nothing and the output is
// identical, yet a valid value is still required.
//
// Enum - 'reportType': "trades", "ledgers"
//
// # Required Permissions:
//
// Data - Export data;
//
// # Example Usage:
//
//	reportStatus, err := kc.GetExportReportStatus("ledgers")
func (kc *KrakenClient) GetExportReportStatus(reportType string) (*[]ExportReportStatus, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	validReportType := map[string]bool{
		"trades":  true,
		"ledgers": true,
	}
	if !validReportType[reportType] {
		err := fmt.Errorf("invalid value passed to arg 'reportType'")
		return nil, err
	}
	payload.Add("report", reportType)

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"ExportStatus", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var exportReports []ExportReportStatus
	err = processAPIResponse(res, &exportReports)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &exportReports, nil
}

// Calls Kraken API private Account Data "RetrieveExport" endpoint. Retrieves a
// specified processed data export with the ID passed to arg 'reportID' and
// creates a  file with the output. Accepts optional arg 'path' as the full
// desired path name. Defaults to creating .zip file in current directory if no
// path is entered.
//
// # Required Permissions:
//
// Data - Export data;
//
// # Example Usage:
//
//	err := kc.RetrieveDataExport("TCJA", "C:/Users/User/Downloads/Reports/my-report.zip")
func (kc *KrakenClient) RetrieveDataExport(reportID string, path ...string) error {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("id", reportID)

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"RetrieveExport", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return err
	}
	defer res.Body.Close()

	// Determine filePath from 'path' arg or default
	var filePath string
	if len(path) > 0 {
		if len(path) > 1 {
			err = fmt.Errorf("too many arguments passed, expected 1 or 2 including 'reportID' and optional 'path'")
			return err
		}
		filePath = path[0]
	} else {
		filePath = "report_" + reportID + ".zip"
	}
	// Create .zip file and copy report to it
	out, err := os.Create(filePath)
	if err != nil {
		err = fmt.Errorf("error creating .zip file for report | %w", err)
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, res.Body)
	if err != nil {
		err = fmt.Errorf("error copying output to .zip file | %w", err)
		return err
	}
	return nil
}

// Calls Kraken API private Account Data "RemoveExport" endpoint. Deletes/cancels
// exported trades/ledgers report with specific ID passed to arg 'reportID'.
// Passing "delete" to arg 'requestType' can only be used for reports that have
// already been processed; pass "cancel" for queued or processing reports.
//
// Enum - 'requestType': "delete", "cancel"
//
// # Required Permissions:
//
// Data - Export data;
//
// # Example Usage:
//
//	err := kc.RetrieveDataExport("TCJA", "cancel")
func (kc *KrakenClient) DeleteExportReport(reportID string, requestType string) error {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("id", reportID)
	validRequestType := map[string]bool{
		"cancel": true,
		"delete": true,
	}
	if !validRequestType[requestType] {
		err := fmt.Errorf("invalid value passed to arg 'requestType'")
		return err
	}
	payload.Add("type", requestType)

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"RemoveExport", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return err
	}
	defer res.Body.Close()

	// Process API response
	var deleteResp DeleteReportResp
	err = processAPIResponse(res, &deleteResp)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return err
	}
	if !deleteResp.Delete && !deleteResp.Cancel {
		err = fmt.Errorf("something went wrong completing request, verify status of report and try again")
		return err
	}
	return nil
}

// #endregion

// #region Authenticated Trading endpoints

// Calls Kraken API private Trading "AddOrder" endpoint. Creates a new order
// of type arg 'orderType' with direction or side passed to arg 'direction' and
// size/quantity passed to arg 'volume' for the specified market passed to arg
// 'pair'. Accepts any number of functional options passed to arg 'options' to
// modify behavior or add additional constraints to orders.
//
// # Enums:
//
// 'orderType': See list "OrderType functions" below for possible functions to
// pass to this arg. Must use only one.
//
// 'direction': "buy", "sell"
//
// 'volume': >= 0
//
// 'pair': Call public market data endpoint function ListTradeablePairs() for a
// current list of possible values to pass to this arg
//
// 'options': See list "Functional Options" below for possible functions to pass
// to this arg. Can pass none or many, though some may conflict with eachother
// and/or conflict with the 'orderType' passed.
//
// CAUTION: Conflicting 'options' args passed may not always result in an invalid
// order (resulting in an error from Kraken's API). Sometimes, they will overwrite
// eachother's values depending on the order passed to AddOrder() method and the
// order will be sent. See the docs marked with "CAUTION" in options.go for further
// information on specific conflicts.
//
// # Required Permissions:
//
// Orders and trades - Create & modify orders;
//
// # OrderType functions:
//
// Note: See options.go for complete docstrings with applicable general notes
// for each order type function.
//
//	// Instantly market orders in at best current prices
//	func Market() OrderType
//
//	// Order type of "limit" where arg 'price' is the level at which the limit order
//	// will be placed...
//	func Limit(price string) OrderType
//
//	// Order type of "stop-loss" order type where arg 'price' is the stop loss
//	// trigger price...
//	func StopLoss(price string) OrderType
//
//	// Order type of "take-profit" where arg 'price' is the take profit trigger price...
//	func TakeProfit(price string) OrderType
//
//	// Order type of "stop-loss-limit" where arg 'price' is the stop loss trigger
//	// price and arg 'price2' is the limit order that will be placed...
//	func StopLossLimit(price, price2 string) OrderType
//
//	// Order type of "take-profit-limit" where arg 'price' is the take profit trigger
//	// price and arg 'price2' is the limit order that will be placed...
//	func TakeProfitLimit(price, price2 string) OrderType
//
//	// Order type of "trailing-stop" where arg 'price' is the relative stop trigger
//	// price...
//	func TrailingStop(price string) OrderType
//
//	// Order type of "trailing-stop-limit" where arg 'price' is the relative stop
//	// trigger price and arg 'price2' is the limit order that will be placed...
//	func TrailingStopLimit(price, price2 string) OrderType
//
//	// Order type of "settle-position". Settles any open margin position of same
//	// 'direction' and 'pair' by amount 'volume'...
//	func SettlePosition(leverage string)
//
// # Functional Options:
//
// Note: See options.go for complete docstrings with further notes on some functions
// and applicable general notes for each close order type function.
//
//	// User reference id 'userref' is an optional user-specified integer id that
//	// can be associated with any number of orders. Many clients choose a userref
//	// corresponding to a unique integer id generated by their systems (e.g. a
//	// timestamp). However, because we don't enforce uniqueness on our side, it
//	// can also be used to easily group orders by pair, side, strategy, etc. This
//	// allows clients to more readily cancel or query information about orders in
//	// a particular group, with fewer API calls by using userref instead of our
//	// txid, where supported.
//	func UserRef(userRef string) AddOrderOption
//
//	// Used to create an iceberg order, this is the visible order quantity in terms
//	// of the base asset. The rest of the order will be hidden, although the full
//	// volume can be filled at any time by any order of that size or larger that
//	// matches in the order book. DisplayVolume() can only be used with the Limit()
//	// order type. Must be greater than 0, and less than AddOrder() arg 'volume'.
//	func DisplayVolume(displayVol string) AddOrderOption
//
//	// Price signal used to trigger stop-loss, stop-loss-limit, take-profit,
//	// take-profit-limit, trailing-stop and trailing-stop-limit orders. Defaults to
//	// "last" trigger type if not called. Calling this function overrides default to
//	// "index".
//	//
//	// Note: This trigger type will also be used for any associated conditional
//	// close orders.
//	//
//	// Note: To keep triggers serviceable, the last price will be used as fallback
//	// reference price during connectivity issues with external index feeds.
//	func IndexTrigger() AddOrderOption
//
//	// Amount of leverage desired. Defaults to no leverage if function is not called.
//	// API accepts string of any number; in practice, must be some integer >= 2...
//	func Leverage(leverage string) AddOrderOption
//
//	// If true, order will only reduce a currently open position, not increase it
//	// or open a new position. Defaults to false if not passed.
//	//
//	// Note: ReduceOnly() is only usable with leveraged orders. This includes orders
//	// of 'orderType' SettlePosition() and orders with Leverage() passed to 'options'
//	func ReduceOnly() AddOrderOption
//
//	// Sets self trade behavior to "cancel-oldest". Overrides default value when called.
//	// Default "cancel-newest"...
//	//
//	// CAUTION: Mutually exclusive with STPCancelBoth()
//	func STPCancelOldest() AddOrderOption
//
//	// Sets self trade behavior to "cancel-both". Overrides default value when called.
//	// Default "cancel-newest"...
//	//
//	// CAUTION: Mutually exclusive with STPCancelOldest()
//	func STPCancelBoth() AddOrderOption
//
//	// Post-only order (available when ordertype = limit)
//	func PostOnly() AddOrderOption
//
//	// Prefer fee in base currency (default if selling)
//	//
//	// CAUTION: Mutually exclusive with FCIQ().
//	func FCIB() AddOrderOption
//
//	// Prefer fee in quote currency (default if buying)
//	//
//	// CAUTION: Mutually exclusive with FCIB().
//	func FCIQ() AddOrderOption
//
//	// Disables market price protection for market orders
//	func NOMPP() AddOrderOption
//
//	// Order volume expressed in quote currency. This is supported only for market orders.
//	func VIQC() AddOrderOption
//
//	// Time-in-force of the order to specify how long it should remain in the order
//	// book before being cancelled. Overrides default value with "IOC" (Immediate Or
//	// Cancel). IOC will immediately execute the amount possible and cancel any
//	// remaining balance rather than resting in the book. Defaults to "GTC" (Good
//	// 'Til Canceled) if function is not called.
//	//
//	// CAUTION: Mutually exclusive with GoodTilDate().
//	func ImmediateOrCancel() AddOrderOption
//
//	// Time-in-force of the order to specify how long it should remain in the order
//	// book before being cancelled. Overrides default value with "GTD" (Good Til Date).
//	// GTD, if called, will cause the order to expire at specified unix time passed
//	// to arg 'expireTime'. Expiration time, can be specified as an absolute timestamp
//	// or as a number of seconds in the future...
//	//
//	// CAUTION: Mutually exclusive with ImmediateOrCancel().
//	func GoodTilDate(expireTime string) AddOrderOption
//
// // Conditional close of "limit" order type where arg 'price' is the level at which
// the limit order will be placed...
//
//	// CAUTION: Mutually exclusive with all conditional close orders with format
//	// Close<orderType>()
//	func CloseLimit(price string) AddOrderOption
//
//	// Conditional close of "stop-loss" order type where arg 'price' is the stop
//	// loss trigger price...
//	//
//	// CAUTION: Mutually exclusive with all conditional close orders with format
//	// Close<orderType>()
//	func CloseStopLoss(price string) AddOrderOption
//
//	// Conditional close of "take-profit" order type where arg 'price' is the take
//	// profit trigger price...
//	//
//	// CAUTION: Mutually exclusive with all conditional close orders with format
//	// Close<orderType>()
//	func CloseTakeProfit(price string) AddOrderOption
//
//	// Conditional close of "stop-loss-limit" order type where arg 'price' is the
//	// stop loss trigger price and arg 'price2' is the limit order that will be placed...
//	//
//	// CAUTION: Mutually exclusive with all conditional close orders with format
//	// Close<orderType>()
//	func CloseStopLossLimit(price, price2 string) AddOrderOption
//
//	// Conditional close of "take-profit-limit" order type where arg 'price' is the
//	// take profit trigger price and arg 'price2' is the limit order that will be
//	// placed...
//	//
//	// CAUTION: Mutually exclusive with all conditional close orders with format
//	// Close<orderType>()
//	func CloseTakeProfitLimit(price, price2 string) AddOrderOption
//
//	// Conditional close of "trailing-stop" order type where arg 'price' is the relative
//	// stop trigger price...
//	//
//	// CAUTION: Mutually exclusive with all conditional close orders with format
//	// Close<orderType>()
//	func CloseTrailingStop(price string) AddOrderOption
//
//	// Conditional close of "trailing-stop-limit" order type where arg 'price' is the
//	// relative stop trigger price and arg 'price2' is the limit order that will be
//	// placed...
//	//
//	// CAUTION: Mutually exclusive with all conditional close orders with format
//	// Close<orderType>()
//	func CloseTrailingStopLimit(price, price2 string) AddOrderOption
//
//	// Pass RFC3339 timestamp (e.g. 2021-04-01T00:18:45Z) after which the matching
//	// engine should reject the new order request to arg 'deadline'...
//	func AddWithDeadline(deadline string) AddOrderOption
//
//	// Validates inputs only. Do not submit order. Defaults to "false" if not called.
//	func ValidateAddOrder() AddOrderOption
//
// # Example Usage:
//
//	newOrder, err := kc.AddOrder(krakenspot.Limit("45000"), "buy", "1.0", "XXBTZUSD", krakenspot.PostOnly(), krakenspot.CloseLimit("49000"))
func (kc *KrakenClient) AddOrder(orderType OrderType, direction, volume, pair string, options ...AddOrderOption) (*AddOrderResp, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	orderType(payload)
	payload.Add("type", direction)
	payload.Add("volume", volume)
	payload.Add("pair", pair)
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	res, err := kc.doRequest(privatePrefix+"AddOrder", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var newOrder AddOrderResp
	err = processAPIResponse(res, &newOrder)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &newOrder, nil
}

// Calls Kraken API private Trading "AddOrderBatch" endpoint. Accepts slice of
// BatchOrder types passed to arg 'orders'. Send an array of orders (max: 15).
// Any orders rejected due to order validations, will be dropped and the rest
// of the batch is processed. All orders in batch should be limited to a single
// pair passed to arg 'pair. The order of returned txid's in the response array
// is the same as the order of the order list sent in request. Recommended to
// use constructor function NewBatchOrder() to build new BatchOrder types with
// the three required fields passed to its args.
//
// Note: It is up to the user to ensure any other required fields (ie. Price)
// are set with the *BatchOrder methods depending on orderType and desired order
// behavior, and ensure that no conflicting methods are passed.
//
// # Required Permissions:
//
// Orders and trades - Create & modify orders;
//
// # Functional Options:
//
//	// Pass RFC3339 timestamp (e.g. 2021-04-01T00:18:45Z) after which the matching
//	// engine should reject the new order request to arg 'deadline'.
//	//
//	// In presence of latency or order queueing: min now() + 2 seconds, max now() +
//	// 60 seconds.
//	func AddBatchWithDeadline(deadline string) AddOrderBatchOption
//
//	// Validates inputs only. Do not submit order. Defaults to "false" if not called.
//	func ValidateAddOrderBatch() AddOrderBatchOption
//
// # Example Usage:
//
//	// Layers 15 bids on Bitcoin/USD every 1% down from startingPrice
//	startingPrice := 46000
//	orders := make([]krakenspot.BatchOrder, 15)
//	for i := 0; i < 15; i++ {
//		newOrder := krakenspot.NewBatchOrder("limit", "buy", "0.1").SetPrice(fmt.Sprintf("%.1f", float64(startingPrice)-(math.Pow(1.01, float64(i))-1)*float64(startingPrice)))
//		orders[i] = *newOrder
//	}
//	batchOrderResp, err := kc.AddOrderBatch(orders, "XXBTZUSD", krakenspot.ValidateAddOrderBatch())
//	if err != nil {
//		log.Println("error sending batch | ", err)
//	}
//	log.Println(batchOrderResp)
func (kc *KrakenClient) AddOrderBatch(orders []BatchOrder, pair string, options ...AddOrderBatchOption) (*AddOrderBatchResp, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	for _, order := range orders {
		orderJSON, err := json.Marshal(order)
		if err != nil {
			err := fmt.Errorf("error marshalling order to json | %w", err)
			return nil, err
		}
		payload.Add("orders[]", string(orderJSON))
	}
	payload.Add("pair", pair)
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	res, err := kc.doRequest(privatePrefix+"AddOrderBatch", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var newOrders AddOrderBatchResp
	err = processAPIResponse(res, &newOrders)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &newOrders, nil
}

// Calls Kraken API private Trading "EditOrder" endpoint. Edit volume and price
// on open orders with transaction ID or user reference ID passed to arg 'refID'.
// If reference ID is used, it must be unique to only one order. Uneditable
// orders include triggered stop/profit orders, orders with conditional close
// terms attached, those already cancelled or filled, and those where the
// executed volume is greater than the newly supplied volume. Accepts any
// number of functional options passed to arg 'options'; see list below and/or
// options.go for further docs notes for options.
//
// Note: Edit orders will be rejected by Kraken's API if at least one of NewVolume()
// or NewPrice() is not passed to arg 'options'
//
// Note: Field "userref" from parent order will not be retained on the new
// order after edit. NewUserRef() must be passed to arg 'options' if a reference
// ID associated with this order is still necessary.
//
// Note: Post-only flag is not retained from original order after
// successful edit. Post-only needs to be explicitly set with NewPostOnly()
// passed to arg 'options' on edit request.
//
// # Required Permissions:
//
// Orders and trades - Create & modify orders;
//
// # Functional Options:
//
//	// Field "userref" is an optional user-specified integer id associated with
//	// edit request.
//	//
//	// Note: userref from parent order will not be retained on the new order after
//	// edit.
//	func NewUserRef(userRef string) EditOrderOption
//
//	// Updates order quantity in terms of the base asset.
//	func NewVolume(volume string) EditOrderOption
//
//	// Used to edit an iceberg order, this is the visible order quantity in terms
//	// of the base asset. The rest of the order will be hidden, although the full
//	// volume can be filled at any time by any order of that size or larger that
//	// matches in the order book. displayvol can only be used with the limit order
//	// type, must be greater than 0, and less than volume.
//	func NewDisplayVolume(displayVol string) EditOrderOption
//
//	// Updates limit price for "limit" orders. Updates trigger price for "stop-loss",
//	// "stop-loss-limit", "take-profit", "take-profit-limit", "trailing-stop" and
//	// "trailing-stop-limit" orders...
//	func NewPrice(price string) EditOrderOption
//
//	// Updates limit price for "stop-loss-limit", "take-profit-limit" and
//	// "trailing-stop-limit" orders...
//	func NewPrice2(price2 string) EditOrderOption
//
//	// Post-only order (available when ordertype = limit). All the flags from the
//	// parent order are retained except post-only. Post-only needs to be explicitly
//	// mentioned on every edit request.
//	func NewPostOnly() EditOrderOption
//
//	// RFC3339 timestamp (e.g. 2021-04-01T00:18:45Z) after which the matching
//	// engine should reject the new order request, in presence of latency or order
//	// queueing. min now() + 2 seconds, max now() + 60 seconds.
//	func NewDeadline(deadline string) EditOrderOption
//
//	// Used to interpret if client wants to receive pending replace, before the
//	// order is completely replaced. Defaults to "false" if not called.
//	func NewCancelResponse() EditOrderOption
//
//	// Validate inputs only. Do not submit order. Defaults to false if not called.
//	func ValidateEditOrder() EditOrderOption
//
// # Example Usage:
//
//	editOrder, err := kc.EditOrder("OHYO67-6LP66-HMQ437", "XXBTZUSD", krakenspot.NewVolume("2.1234"), krakenspot.NewPostOnly(), krakenspot.NewPrice("45000.1"), krakenspot.ValidateEditOrder())
func (kc *KrakenClient) EditOrder(txID, pair string, options ...EditOrderOption) (*EditOrderResp, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("txid", txID)
	payload.Add("pair", pair)
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	res, err := kc.doRequest(privatePrefix+"EditOrder", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var editOrder EditOrderResp
	err = processAPIResponse(res, &editOrder)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &editOrder, nil
}

// Calls Kraken API private Trading "CancelOrder" endpoint. Cancels either a
// particular open order by transaction ID passed to arg 'txID' or set of open
// orders with specified user reference ID passed to arg 'txID'.
//
// # Required Permissions:
//
// Orders and trades - Create & modify orders; OR
//
// Orders and trades - Cancel & close orders
//
// # Example Usage:
//
//	cancelResp, err := kc.CancelOrder("1234") // cancels multiple w/ user ref "1234"
func (kc *KrakenClient) CancelOrder(txID string) (*CancelOrderResp, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("txid", txID)

	// Send request to server
	res, err := kc.doRequest(privatePrefix+"CancelOrder", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var cancelOrder CancelOrderResp
	err = processAPIResponse(res, &cancelOrder)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &cancelOrder, nil
}

// Calls Kraken API private Trading "CancelAll" endpoint. Cancels all open orders.
//
// # Required Permissions:
//
// Orders and trades - Create & modify orders; OR
//
// Orders and trades - Cancel & close orders
//
// # Example Usage:
//
//	cancelResp, err := kc.CancelAllOrders()
func (kc *KrakenClient) CancelAllOrders() (*CancelOrderResp, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))

	// Send request to server
	res, err := kc.doRequest(privatePrefix+"CancelAll", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var cancelOrder CancelOrderResp
	err = processAPIResponse(res, &cancelOrder)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &cancelOrder, nil
}

// Calls Kraken API private Trading "CancelAllOrdersAfter" endpoint.
// CancelAllOrdersAfter provides a "Dead Man's Switch" mechanism to protect the
// client from network malfunction, extreme latency or unexpected matching
// engine downtime. The client can send a request with a timeout (in seconds),
// that will start a countdown timer which will cancel all client orders when
// the timer expires. The client has to keep sending new requests to push back
// the trigger time, or deactivate the mechanism by specifying a timeout of 0.
// If the timer expires, all orders are cancelled and then the timer remains
// disabled until the client provides a new (non-zero) timeout.
//
// The recommended use is to make a call every 15 to 30 seconds, providing a
// timeout of 60 seconds. This allows the client to keep the orders in place
// in case of a brief disconnection or transient delay, while keeping them safe
// in case of a network breakdown. It is also recommended to disable the timer
// ahead of regularly scheduled trading engine maintenance (if the timer is
// enabled, all orders will be cancelled when the trading engine comes back
// from downtime - planned or otherwise).
//
// # Required Permissions:
//
// Orders and trades - Create & modify orders; OR
//
// Orders and trades - Cancel & close orders
//
// # Example Usage:
//
//	cancelAfterResp, err := kc.CancelAllOrdersAfter("60")
func (kc *KrakenClient) CancelAllOrdersAfter(timeout string) (*CancelAllAfter, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("timeout", timeout)

	// Send request to server
	res, err := kc.doRequest(privatePrefix+"CancelAllOrdersAfter", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var cancelAfter CancelAllAfter
	err = processAPIResponse(res, &cancelAfter)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &cancelAfter, nil
}

// Calls Kraken API private Trading "CancelOrderBatch" endpoint. Cancels
// multiple open orders by txid or userref passed as a slice to arg 'txIDs'
// (maximum 50 total unique IDs/references)
//
// # Required Permissions:
//
// Orders and trades - Create & modify orders; OR
//
// Orders and trades - Cancel & close orders
//
// # Example Usage:
//
//	ordersToCancel := []string{"OG5V2Y-RYKVL-DT3V3B", "OP5V2Y-RYKVL-ET3V3B"}
//	cancelOrderResp, err := kc.CancelOrderBatch(ordersToCancel)
func (kc *KrakenClient) CancelOrderBatch(txIDs []string) (*CancelOrderResp, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	for _, txID := range txIDs {
		payload.Add("orders[]", txID)
	}

	// Send request to server
	res, err := kc.doRequest(privatePrefix+"CancelOrderBatch", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var cancelOrders CancelOrderResp
	err = processAPIResponse(res, &cancelOrders)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &cancelOrders, nil
}

// Calls Kraken API private Trading "CancelOrderBatch" endpoint and private
// Account Data "OpenOrders" to find and cancel all open orders for market passed
// to arg 'pair'.
//
// # Required Permissions:
//
// Order and Trades - Query open orders & trades;
//
// Orders and trades - Create & modify orders; OR
//
// Orders and trades - Cancel & close orders
//
// # Example Usage:
//
//	cancelResp, err := kc.CancelAllOrdersForPair("XXBTZUSD")
func (kc *KrakenClient) CancelAllOrdersForPair(pair string) (*CancelOrderResp, error) {
	orders, err := kc.ListOpenTxIDsForPair(pair)
	if err != nil {
		err = fmt.Errorf("error calling ListOpenTxIDsForPair() | %w", err)
		return nil, err
	}
	cancelResp, err := kc.CancelOrderBatch(orders)
	if err != nil {
		err = fmt.Errorf("error calling CancelOrderBatch() | %w", err)
		return nil, err
	}
	return cancelResp, nil
}

// #endregion

// #region Authenticated Funding endpoints

// Calls Kraken API private Funding "DepositMethods" endpoint. Retrieve methods
// available for depositing a specified asset passed to arg 'asset'. ~Accepts
// functional options args 'options'.~
//
// # Required Permissions:
//
// Funds permissions - Query;
// Funds permissions - Deposit;
//
// # ~Functional Options:~
//
// ~Asset class being deposited (optional). Defaults to "currency" if function is
// not called. As of 1/7/24, only known valid asset class is "currency".~
//
// ~func DMWithAssetClass(aclass string) GetDepositMethodsOption~
//
// # Example Usage:
//
//	depositMethods, err := kc.GetDepositMethods("XBT")
func (kc *KrakenClient) GetDepositMethods(asset string, options ...GetDepositMethodsOption) (*[]DepositMethod, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("asset", asset)
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"DepositMethods", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var depositMethods []DepositMethod
	err = processAPIResponse(res, &depositMethods)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &depositMethods, nil
}

// Calls Kraken API private Funding "DepositAddresses" endpoint. Retrieve (or
// generate new with DAWithNew() passed to arg 'options') deposit addresses
// for a particular asset and method. Accepts functional options args 'options'.
//
// # Required Permissions:
//
// Funds permissions - Query;
//
// # Functional Options:
//
//	// Whether or not to generate a new address. Defaults to false if function is
//	// not called.
//	func DAWithNew() GetDepositAddressesOption
//
//	// Amount you wish to deposit (only required for method=Bitcoin Lightning)
//	func DAWithAmount(amount string) GetDepositAddressesOption
//
// # Example Usage:
//
//	depositAddresses, err := kc.GetDepositAddresses("XBT", "Bitcoin", krakenspot.DAWithNew())
func (kc *KrakenClient) GetDepositAddresses(asset string, method string, options ...GetDepositAddressesOption) (*[]DepositAddress, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("asset", asset)
	payload.Add("method", method)
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"DepositAddresses", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var depositAddresses []DepositAddress
	err = processAPIResponse(res, &depositAddresses)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &depositAddresses, nil
}

// Calls Kraken API private Funding "DepositStatus" endpoint. Retrieves
// information about recent deposits. Results are sorted by recency, call
// method GetDepositsStatusPaginated() instead to begin an iterated list of
// deposits. Accepts functional options args 'options'.
//
// # Required Permissions:
//
// Funds permissions - Query;
//
// # Functional Options:
//
//	// Filter for specific asset being deposited
//	func DSWithAsset(asset string) GetDepositsStatusOption
//
//	// Filter for specific name of deposit method
//	func DSWithMethod(method string) GetDepositsStatusOption
//
//	// Start timestamp, deposits created strictly before will not be included in
//	// the response
//	func DSWithStart(start string) GetDepositsStatusOption
//
//	// End timestamp, deposits created strictly after will be not be included in
//	// the response
//	func DSWithEnd(end string) GetDepositsStatusOption
//
//	// Number of results to include per page
//	func DSWithLimit(limit uint) GetDepositsStatusOption
//
// ~Filter asset class being deposited. Defaults to "currency" if function is
// not called. As of 1/7/24, only known valid asset class is "currency"~
//
// ~func DSWithAssetClass(aclass string) GetDepositsStatusOption~
//
// # Example Usage:
//
//	deposits, err := kc.GetDepositsStatus()
func (kc *KrakenClient) GetDepositsStatus(options ...GetDepositsStatusOption) (*[]DepositStatus, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"DepositStatus", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var depositsStatus []DepositStatus
	err = processAPIResponse(res, &depositsStatus)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &depositsStatus, nil
}

// Calls Kraken API private Funding "DepositStatus" endpoint. Begins a paginated
// list with information about recent deposits. Results are sorted by recency
// and filtered by functional options args 'options'. After list is initiated via
// this method, use with (kc *KrakenClient) GetDepositsStatusWithCursor().
//
// # Required Permissions:
//
// Funds permissions - Query;
//
// # Functional Options:
//
//	// Filter for specific asset being deposited
//	func DPWithAsset(asset string) GetDepositsStatusPaginatedOption
//
//	// Filter for specific name of deposit method
//	func DPWithMethod(method string) GetDepositsStatusPaginatedOption
//
//	// Start timestamp, deposits created strictly before will not be included in
//	// the response
//	func DPWithStart(start string) GetDepositsStatusPaginatedOption
//
//	// End timestamp, deposits created strictly after will be not be included in
//	// the response
//	func DPWithEnd(end string) GetDepositsStatusPaginatedOption
//
//	// Number of results to include per page
//	func DPWithLimit(limit uint) GetDepositsStatusPaginatedOption
//
// ~Filter asset class being deposited. Defaults to "currency" if function is
// not called. As of 1/7/24, only known valid asset class is "currency"~
//
// ~func DPWithAssetClass(aclass string) GetDepositsStatusPaginatedOption~
//
// # Example Usage:
//
//	depositsResp, err := kc.GetDepositsStatusPaginated(krakenspot.DPWithAsset("XBT"), krakenspot.DPWithLimit(5))
//	deposits := (*depositsResp).Deposits
//	// do something with deposits
//	cursor := (*depositsResp).NextCursor
//	for cursor != "" {
//		depositsResp, err = kc.GetDepositsStatusCursor(cursor)
//		// error handling
//		deposits = (*depositsResp).Deposits
//		// do something with deposits
//		cursor = (*depositsResp).NextCursor
//	}
func (kc *KrakenClient) GetDepositsStatusPaginated(options ...GetDepositsStatusPaginatedOption) (*DepositStatusPaginated, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("cursor", "true")
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"DepositStatus", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var depositsStatus DepositStatusPaginated
	err = processAPIResponse(res, &depositsStatus)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &depositsStatus, nil
}

// Calls Kraken API private Funding "DepositStatus" endpoint. Requires arg 'cursor'
// which has been retrieved from a previous call to GetDepositsStatusPaginated()
// method. Continues paginated list with information about recent deposits. Results
// are sorted by recency and filtered by 'options' passed to GetDepositsStatusPaginated()
// previously.
//
// # Required Permissions:
//
// Funds permissions - Query;
//
// # Example Usage:
//
//	depositsResp, err := kc.GetDepositsStatusPaginated(krakenspot.DPWithAsset("XBT"), krakenspot.DPWithLimit(5))
//	deposits := (*depositsResp).Deposits
//	// do something with deposits
//	cursor := (*depositsResp).NextCursor
//	for cursor != "" {
//		depositsResp, err = kc.GetDepositsStatusCursor(cursor)
//		// error handling
//		deposits = (*depositsResp).Deposits
//		// do something with deposits
//		cursor = (*depositsResp).NextCursor
//	}
func (kc *KrakenClient) GetDepositsStatusCursor(cursor string) (*DepositStatusPaginated, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("cursor", cursor)

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"DepositStatus", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var depositsStatus DepositStatusPaginated
	err = processAPIResponse(res, &depositsStatus)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &depositsStatus, nil
}

// Calls Kraken API private Funding "WithdrawMethods" endpoint. Retrieve a list
// of withdrawal methods available for the user. Accepts functional options args
// 'options'.
//
// # Required Permissions:
//
// Funds permissions - Query;
// Funds permissions - Withdraw;
//
// # Functional Options:
//
//	// Filter methods for specific asset. Defaults to no filter if function is not
//	// called
//	func WMWithAsset(asset string) GetWithdrawalMethodsOption
//
//	// Filter methods for specific network. Defaults to no filter if function is not
//	// called
//	func WMWithNetwork(network string) GetWithdrawalMethodsOption
//
// ~// Filter asset class being withdrawn. Defaults to "currency" if function is
// not called. As of 1/7/24, only known valid asset class is "currency"~
//
// ~func WMWithAssetClass(aclass string) GetWithdrawalMethodsOption~
//
// # Example Usage:
//
// withdrawalMethods, err := kc.GetWithdrawalMethods(krakenspot.WMWithNetwork("Ethereum"))
func (kc *KrakenClient) GetWithdrawalMethods(options ...GetWithdrawalMethodsOption) (*[]WithdrawalMethod, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"WithdrawMethods", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var withdrawalMethods []WithdrawalMethod
	err = processAPIResponse(res, &withdrawalMethods)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &withdrawalMethods, nil
}

// Calls Kraken API private Funding "WithdrawAddresses" endpoint. Retrieves a
// list of withdrawal addresses available for the user. Accepts functional
// options passed to arg 'options'
//
// # Required Permissions:
//
// Funds permissions - Query;
// Funds permissions - Withdraw;
//
// # Functional Options:
//
//	// Filter addresses for specific asset
//	func WAWithAsset(asset string) GetWithdrawalAddressesOption
//
//	// Filter addresses for specific method
//	func WAWithMethod(method string) GetWithdrawalAddressesOption
//
//	// Find address for by withdrawal key name, as set up on your account
//	func WAWithKey(key string) GetWithdrawalAddressesOption
//
//	// Filter by verification status of the withdrawal address. Withdrawal addresses
//	// successfully completing email confirmation will have a verification status of
//	// true.
//	func WAWithVerified(verified bool) GetWithdrawalAddressesOption
//
// ~// Filter asset class being withdrawn. Defaults to "currency" if function is
// not called. As of 1/7/24, only known valid asset class is "currency"~
// ~func WAWithAssetClass(aclass string) GetWithdrawalAddressesOption~
//
// # Example Usage:
//
//	withdrawalAddresses, err := kc.GetWithdrawalAddresses(krakenspot.WAWithAsset("XBT"), krakenspot.WAWithVerified(true))
func (kc *KrakenClient) GetWithdrawalAddresses(options ...GetWithdrawalAddressesOption) (*[]WithdrawalAddress, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"WithdrawAddresses", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var withdrawalAddresses []WithdrawalAddress
	err = processAPIResponse(res, &withdrawalAddresses)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &withdrawalAddresses, nil
}

// Calls Kraken API private Funding "WithdrawInfo" endpoint. Retrieves fee
// information about potential withdrawals for a specified args 'asset',
// withdrawal key name 'key', and 'amount'.
//
// # Required Permissions:
//
// Funds permissions - Query;
// Funds permissions - Withdraw;
//
// # Example Usage:
//
//	withdrawalinfo, err := kc.GetWithdrawalInfo("XBT", "btc_testnet_with1", "0.725")
func (kc *KrakenClient) GetWithdrawalInfo(asset string, key string, amount string) (*WithdrawalInfo, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("asset", asset)
	payload.Add("key", key)
	payload.Add("amount", amount)

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"WithdrawInfo", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var withdrawalInfo WithdrawalInfo
	err = processAPIResponse(res, &withdrawalInfo)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &withdrawalInfo, nil
}

// TODO test if this works
// Calls Kraken API private Funding "Withdraw" endpoint. Makes a withdrawal
// request for specified args 'asset', withdrawal key name 'key', and 'amount'.
// If successful, returns resulting reference ID as a string.
//
// # Required Permissions:
//
// Funds permissions - Withdraw;
//
// # Functional Options:
//
//	// Optional, crypto address that can be used to confirm address matches key
//	// (will return Invalid withdrawal address error if different)
//	func WFWithAddress(address string) WithdrawFundsOption
//
//	// Optional, if the processed withdrawal fee is higher than max_fee, withdrawal
//	// will fail with EFunding:Max fee exceeded
//	func WFWithMaxFee(maxFee string) WithdrawFundsOption
//
// # Example Usage:
//
//	refID, err := kc.WithdrawFunds("XBT", "btc_testnet_with1", "0.725")
func (kc *KrakenClient) WithdrawFunds(asset string, key string, amount string, options ...WithdrawFundsOption) (string, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("asset", asset)
	payload.Add("key", key)
	payload.Add("amount", amount)
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"Withdraw", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return "", err
	}
	defer res.Body.Close()

	// Process API response
	var withdrawFundsResp WithdrawFundsResponse
	err = processAPIResponse(res, &withdrawFundsResp)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return "", err
	}
	return withdrawFundsResp.RefID, nil
}

// Calls Kraken API private Funding "WithdrawStatus" endpoint. Retrieves
// information about recent withdrawals. Results are sorted by recency, call
// method GetWithdrawalsStatusPaginated() instead to begin an iterated list of
// withdrawals. Accepts functional options args 'options'.
//
// # Required Permissions:
//
// Funds permissions - Withdraw; OR
// Data - Query ledger entries;
//
// # Functional Options:
//
//	// Filter for specific asset being withdrawn
//	func WSWithAsset(asset string) GetWithdrawalsStatusOption
//
//	// Filter for specific name of withdrawal method
//	func WSWithMethod(method string) GetWithdrawalsStatusOption
//
//	// Start timestamp, withdrawals created strictly before will not be included in
//	// the response
//	func WSWithStart(start string) GetWithdrawalsStatusOption
//
//	// End timestamp, withdrawals created strictly after will be not be included in
//	// the response
//	func WSWithEnd(end string) GetWithdrawalsStatusOption
//
// ~// Filter asset class being withdrawn. Defaults to "currency" if function is
// not called. As of 1/7/24, only known valid asset class is "currency"~
//
// ~func WSWithAssetClass(aclass string) GetWithdrawalsStatusOption~
//
// # Example Usage:
//
//	withdrawalStatus, err := kc.GetWithdrawalsStatus(krakenspot.WSWithMethod("Bank Frick (SWIFT)"))
func (kc *KrakenClient) GetWithdrawalsStatus(options ...GetWithdrawalsStatusOption) (*[]WithdrawalStatus, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"WithdrawStatus", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var withdrawalsStatus []WithdrawalStatus
	err = processAPIResponse(res, &withdrawalsStatus)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &withdrawalsStatus, nil
}

// Calls Kraken API private Funding "WithdrawStatus" endpoint. Begins a paginated
// list with information about recent withdrawals. Results are sorted by recency
// and filtered by functional options args 'options'. After list is initiated via
// this method, use with (kc *KrakenClient) GetWithdrawalsStatusWithCursor().
//
// # Required Permissions:
//
// Funds permissions - Withdraw; OR
// Data - Query ledger entries;
//
// # Functional Options:
//
//	// Filter for specific asset being withdrawn
//	func WPWithAsset(asset string) GetWithdrawalsStatusPaginatedOption
//
//	// Filter for specific name of withdrawal method
//	func WPWithMethod(method string) GetWithdrawalsStatusPaginatedOption
//
//	// Start timestamp, withdrawals created strictly before will not be included in
//	// the response
//	func WPWithStart(start string) GetWithdrawalsStatusPaginatedOption
//
//	// End timestamp, withdrawals created strictly after will be not be included in
//	// the response
//	func WPWithEnd(end string) GetWithdrawalsStatusPaginatedOption
//
//	// Number of results to include per page. Defaults to 500 if function is not called
//	func WPWithLimit(limit int) GetWithdrawalsStatusPaginatedOption
//
// ~// Filter asset class being withdrawn. Defaults to "currency" if function is
// not called. As of 1/7/24, only known valid asset class is "currency"~
//
// ~func WPWithAssetClass(aclass string) GetWithdrawalsStatusPaginatedOption~
//
// # Example Usage:
//
//	withdrawalsResp, err := kc.GetWithdrawalsStatusPaginated(krakenspot.WPWithAsset("XBT"), krakenspot.WPWithLimit(5))
//	withdrawals := (*withdrawalsResp).Withdrawals
//	// do something with withdrawals
//	cursor := (*withdrawalsResp).NextCursor
//	for cursor != "" {
//		withdrawalsResp, err = kc.GetWithdrawalsStatusWithCursor(cursor)
//		// error handling
//		withdrawals = (*withdrawalsResp).Withdrawals
//		// do something withwithdrawals
//		cursor = (*withdrawalsResp).NextCursor
//	}
func (kc *KrakenClient) GetWithdrawalsStatusPaginated(options ...GetWithdrawalsStatusPaginatedOption) (*WithdrawalStatusPaginated, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("cursor", "true")
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"WithdrawStatus", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var withdrawalsStatus WithdrawalStatusPaginated
	err = processAPIResponse(res, &withdrawalsStatus)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &withdrawalsStatus, nil
}

// Calls Kraken API private Funding "WithdrawStatus" endpoint. Requires arg 'cursor'
// which has been retrieved from a previous call to GetWithdrawalsStatusPaginated()
// method. Continues paginated list with information about recent withdrawals. Results
// are sorted by recency and filtered by 'options' passed to GetWithdrawalsStatusPaginated()
// previously.
//
// # Required Permissions:
//
// Funds permissions - Withdraw; OR
// Data - Query ledger entries;
//
// # Example Usage:
//
//	withdrawalsResp, err := kc.GetWithdrawalsStatusPaginated(krakenspot.WPWithAsset("XBT"), krakenspot.WPWithLimit(5))
//	withdrawals := (*withdrawalsResp).Withdrawals
//	// do something with withdrawals
//	cursor := (*withdrawalsResp).NextCursor
//	for cursor != "" {
//		withdrawalsResp, err = kc.GetWithdrawalsStatusWithCursor(cursor)
//		// error handling
//		withdrawals = (*withdrawalsResp).Withdrawals
//		// do something withwithdrawals
//		cursor = (*withdrawalsResp).NextCursor
//	}
func (kc *KrakenClient) GetWithdrawalsStatusWithCursor(cursor string) (*WithdrawalStatusPaginated, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("cursor", cursor)

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"WithdrawStatus", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var withdrawalsStatus WithdrawalStatusPaginated
	err = processAPIResponse(res, &withdrawalsStatus)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &withdrawalsStatus, nil
}

// Calls Kraken API private Funding "WithdrawCancel" endpoint. Cancels a recently
// requested withdrawal of specified arg 'asset' with 'refID', if it has not
// already been successfully processed.
//
// # Required Permissions:
//
// Funds permissions - Withdraw;
//
// # Example Usage:
//
//	err := kc.CancelWithdrawal("XBT", "FTQcuak-V6Za8qrWnhzTx67yYHz8Tg")
func (kc *KrakenClient) CancelWithdrawal(asset string, refID string) error {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("asset", asset)
	payload.Add("refid", refID)

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"WithdrawCancel", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return err
	}
	defer res.Body.Close()

	// Process API response
	var cancelSuccessful bool
	err = processAPIResponse(res, &cancelSuccessful)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return err
	}
	if !cancelSuccessful {
		err = fmt.Errorf("withdrawal cancellation was unsuccessful but no errors returned. check inputs and try again if necessary")
		return err
	}
	return nil
}

// Calls Kraken API private Funding "WalletTransfer" endpoint. Transfers specified
// arg 'amount' of 'asset' from Kraken spot wallet to Kraken Futures wallet. Note
// that a transfer in the other direction must be requested via the Kraken Futures
// API endpoint for withdrawals to Spot wallets
//
// # Required Permissions:
//
// Funds permissions - Query;
//
// # Example Usage:
//
//	refID, err := kc.TransferToFutures("ZUSD", "10000")
func (kc *KrakenClient) TransferToFutures(asset string, amount string) (string, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("asset", asset)
	payload.Add("amount", amount)
	payload.Add("to", "Futures Wallet")
	payload.Add("from", "Spot Wallet")

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"WalletTransfer", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return "", err
	}
	defer res.Body.Close()

	// Process API response
	var refID WalletTransferResponse
	err = processAPIResponse(res, &refID)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return "", err
	}

	return refID.RefID, err
}

// #endregion

// #region Authenticated Subaccounts endpoints

// Calls Kraken API private Subaccounts "CreateSubaccount" endpoint. Creates a
// trading subaccount with details passed to args 'username' and 'email'
//
// Note: Subaccounts are currently only available to institutional clients.
// Please contact your Account Manager for more details.
//
// # Required Permissions:
//
// Institutional verification;
//
// # Example Usage:
//
//	err := kc.CreateSubaccount("kraken-sub-1", "bryptotrader123@aol.com")
func (kc *KrakenClient) CreateSubaccount(username string, email string) error {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("username", username)
	payload.Add("email", email)

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"CreateSubaccount", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return err
	}
	defer res.Body.Close()

	// Process API response
	var result bool
	err = processAPIResponse(res, &result)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return err
	}
	if !result {
		err = fmt.Errorf("something went wrong. check inputs and try again if necessary")
		return err
	}
	return nil
}

// Calls Kraken API private Subaccounts "AccountTransfer" endpoint. Transfer
// funds to and from master and subaccounts.
//
// Note: AccountTransfer must be called by the master account.
//
// Note: Subaccounts are currently only available to institutional clients.
// Please contact your Account Manager for more details.
//
// # Required Permissions:
//
// Institutional verification;
//
// # Example Usage:
//
//	transfer, err := kc.AccountTransfer("XBT", "1.0", "ABCD 1234 EFGH 5678", "IJKL 0987 MNOP 6543")
func (kc *KrakenClient) AccountTransfer(asset string, amount string, fromAccount string, toAccount string) (*AccountTransfer, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("asset", asset)
	payload.Add("amount", amount)
	payload.Add("fromAccount", fromAccount)
	payload.Add("toAccount", toAccount)

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"AccountTransfer", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var transfer AccountTransfer
	err = processAPIResponse(res, &transfer)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &transfer, nil
}

// #endregion

// #region Authenticated Earn endpoints

// Calls Kraken API private Earn "Allocate" endpoint. Allocate funds to the strategy
// with specified ID passed to arg 'strategyID'. Pass desired amount of base
// currency to allocate in string format to arg 'amount'.
//
// Note: This method is asynchronous. A couple of preflight checks are performed
// synchronously on behalf of the method before it is dispatched further. The
// client is required to poll the result using the (kc *KrakenClient) AllocationStatus()
// method.
//
// Note: There can be only one (de)allocation request in progress for given user
// and strategy.
//
// # Required permissions:
//
// Funds permissions - Earn;
//
// # Example Usage:
//
//	err := kc.AllocateEarnFunds("ESXUM7H-SJHQ6-KOQNNI", "5")
func (kc *KrakenClient) AllocateEarnFunds(strategyID string, amount string) error {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("strategy_id", strategyID)
	payload.Add("amount", amount)

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"Earn/Allocate", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return err
	}
	defer res.Body.Close()

	// Process API response
	var result bool
	err = processAPIResponse(res, &result)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return err
	}
	if !result {
		err = fmt.Errorf("something went wrong. check inputs and allocation status and try again if necessary")
		return err
	}
	return nil
}

// Calls Kraken API private Earn "Deallocate" endpoint. Deallocate funds to the
// strategy with specified ID passed to arg 'strategyID'. Pass desired amount of
// base currency to deallocate in string format to arg 'amount'.
//
// Note: This method is asynchronous. A couple of preflight checks are performed
// synchronously on behalf of the method before it is dispatched further. The
// client is required to poll the result using the (kc *KrakenClient) DeallocationStatus()
// method.
//
// Note: There can be only one (de)allocation request in progress for given user
// and strategy.
//
// # Required permissions:
//
// Funds permissions - Earn;
//
// # Example Usage:
//
//	err := kc.AllocateEarnFunds("ESXUM7H-SJHQ6-KOQNNI", "5")
func (kc *KrakenClient) DeallocateEarnFunds(strategyID string, amount string) error {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("strategy_id", strategyID)
	payload.Add("amount", amount)

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"Earn/Deallocate", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return err
	}
	defer res.Body.Close()

	// Process API response
	var result bool
	err = processAPIResponse(res, &result)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return err
	}
	if !result {
		err = fmt.Errorf("something went wrong. check inputs and deallocation status and try again if necessary")
		return err
	}
	return nil
}

// Calls Kraken API private Earn "AllocateStatus" endpoint. Gets the status of
// the last allocation request for specific strategy ID passed to arg 'strategyID'.
// Returns true if the request is still pending, false if it is completed, and
// an api error if there was an issue with the request. API will also return false
// with no errors for strategies on which the account has never made a request.
//
// # Required Permissions:
//
// Funds permissions - Query; OR
// Funds permissions - Earn;
//
// # Example Usage:
//
//	pending, err := kc.AllocationStatus("ESSR5EH-CKYSY-NUQNZI")
func (kc *KrakenClient) AllocationStatus(strategyID string) (bool, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("strategy_id", strategyID)

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"Earn/AllocateStatus", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return false, err
	}
	defer res.Body.Close()

	// Process API response
	var status AllocationStatus
	err = processAPIResponse(res, &status)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return false, err
	}
	return status.Pending, nil
}

// Calls Kraken API private Earn "DeallocateStatus" endpoint. Gets the status of
// the last deallocation request for specific strategy ID passed to arg 'strategyID'.
// Returns true if the request is still pending, false if it is completed, and
// an api error if there was an issue with the request. API will also return false
// with no errors for strategies on which the account has never made a request.
//
// # Required Permissions:
//
// Funds permissions - Query; OR
// Funds permissions - Earn;
//
// # Example Usage:
//
//	pending, err := kc.DeallocationStatus("ESSR5EH-CKYSY-NUQNZI")
func (kc *KrakenClient) DeallocationStatus(strategyID string) (bool, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("strategy_id", strategyID)

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"Earn/DeallocateStatus", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return false, err
	}
	defer res.Body.Close()

	// Process API response
	var status AllocationStatus
	err = processAPIResponse(res, &status)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return false, err
	}
	return status.Pending, nil
}

// Calls Kraken API private Earn "Strategies" endpoint. Returns earn strategies
// along with their parameters. Returns only strategies that are available to
// the user based on geographic region.
//
// Note: In practice, allocation_restriction_info will always be empty even when
// can_allocate is false despite Kraken API docs note otherwise.
//
// Kraken API docs note: When the user does not meet the tier restriction,
// can_allocate will be false and allocation_restriction_info indicates Tier as
// the restriction reason. Earn products generally require Intermediate tier.
// Get your account verified to access earn.
//
// Note: Paging isn't yet implemented, so the endpoint always returns all data
// in the first page. This results in some functional options having no effect
// on output though the query parameters are still valid.
//
// # Required Permissions:
//
// None;
//
// # Functional Options:
//
//	// Filter strategies by asset name. Defaults to no filter if function not called
//	func ESWithAsset(asset string) GetEarnStrategiesOption
//
//	// Filters displayed strategies by lock type. Accepts array of strings for arg
//	// 'lockTypes' and ignores invalid values passed. Defaults to no filter if
//	// function not called or only invalid values passed.
//	//
//	// Enum - 'lockTypes': "flex", "bonded", "timed", "instant"
//	func ESWithLockType(lockTypes []string) GetEarnStrategiesOption
//
// ~// Pass with arg 'ascending' set to true to sort strategies ascending. Defaults
// to false (descending) if function is not called~
//
// ~func ESWithAscending(ascending bool) GetEarnStrategiesOption~
//
// ~// Sets page ID to display results. Defaults to beginning/end (depending on
// sorting set by ESWithAscending()) if function not called.~
//
// ~func ESWithCursor(cursor string) GetEarnStrategiesOption~
//
// ~// Sets number of items to return per page. Note that the limit may be cap'd to
// lower value in the application code.~
//
// ~func ESWithLimit(limit uint16) GetEarnStrategiesOption~
//
// # Example Usage:
//
//	strategies, err := kc.GetEarnStrategies(krakenspot.ESWithLockType([]string{"flex", "instant"}))
func (kc *KrakenClient) GetEarnStrategies(options ...GetEarnStrategiesOption) (*EarnStrategiesResp, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"Earn/Strategies", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var strategies EarnStrategiesResp
	err = processAPIResponse(res, &strategies)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &strategies, nil
}

// Calls Kraken API private Earn "Allocations" endpoint. Gets all earn allocations
// for the user. By default all allocations are returned, even for strategies
// that have been used in the past and have zero balance now.
//
// Note: Paging hasn't been implemented for this method
//
// # Required Permissions:
//
// Funds permissions - Query;
//
// # Functional Options:
//
//	// Pass with arg 'ascending' set to true to sort strategies ascending. Defaults
//	// to false (descending) if function is not called
//	func EAWithAscending(ascending bool) GetEarnAllocationsOption
//
//	// A secondary currency to express the value of your allocations. Defaults
//	// to express value in USD if function is not called
//	func EAWithConvertedAsset(asset string) GetEarnAllocationsOption
//
//	// Omit entries for strategies that were used in the past but now they don't
//	// hold any allocation. Defaults to false (don't omit) if function is not called
//	func EAWithHideZeroAllocations(hide bool) GetEarnAllocationsOption
//
// # Example Usage:
//
//	allocations, err := kc.GetEarnAllocations(krakenspot.EAWithConvertedAsset("XBT"), krakenspot.EAWithHideZeroAllocations())
func (kc *KrakenClient) GetEarnAllocations(options ...GetEarnAllocationsOption) (*EarnAllocationsResp, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"Earn/Allocations", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var allocations EarnAllocationsResp
	err = processAPIResponse(res, &allocations)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &allocations, nil
}

// #endregion

// #region Websockets Authentication endpoint

// Calls Kraken API private Account Data "GetWebSocketsToken" endpoint. This
// method only returns the API's response as a struct including the token and
// the number of seconds until expiration. It does not add the token to *KrakenClient.
// Use *KrakenClient method AuthenticateWebsocket() instead.
//
// Note: An authentication token must be requested via this REST API endpoint
// in order to connect to and authenticate with our Websockets API. The token
// should be used within 15 minutes of creation, but it does not expire once
// a successful WebSockets connection and private subscription has been made
// and is maintained.
//
// # Required Permissions:
//
// WebSockets interface - On;
//
// # Example Usage:
//
//	tokenResp, err := kc.GetWebSocketsToken()
//	token := (*tokenResp).Token
func (kc *KrakenClient) GetWebSocketsToken() (*WebSocketsToken, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))

	// Send request to server
	kc.rateLimitAndIncrement(1)
	res, err := kc.doRequest(privatePrefix+"GetWebSocketsToken", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var token WebSocketsToken
	err = processAPIResponse(res, &token)
	if err != nil {
		err = fmt.Errorf("error calling processAPIResponse() | %w", err)
		return nil, err
	}
	return &token, nil
}

// Adds WebSockets token to *KrakenClient via Kraken API private Account Data
// "GetWebSocketsToken" endpoint. This method must be called before subscribing
// to any authenticated WebSockets.
//
// Note: An authentication token must be requested via this REST API endpoint
// in order to connect to and authenticate with our Websockets API. The token
// should be used within 15 minutes of creation, but it does not expire once
// a successful WebSockets connection and private subscription has been made
// and is maintained.
//
// # Required Permissions:
//
// WebSockets interface - On;
//
// # Example Usage:
//
//	err := kc.GetWebSocketsToken()
func (kc *KrakenClient) AuthenticateWebSockets() error {
	tokenResp, err := kc.GetWebSocketsToken()
	if err != nil {
		return fmt.Errorf("error calling getwebsocketstoken() | %w", err)
	}
	kc.WebSocketManager.Mutex.Lock()
	kc.WebSocketManager.WebSocketToken = tokenResp.Token
	kc.WebSocketManager.Mutex.Unlock()
	return nil
}

// StartRESTRateLimiter starts self rate-limiting for general (everything except
// "order" type) REST API call methods. For this feature to work correctly,
// StartRESTRateLimiter must be called before any general REST API calls are
// made.
//
// Note: This does not rate limit "trading" endpoint calls (such as AddOrder(),
// EditOrder(), or CancelOrder()).
//
// Note: This feature adds processing and wait overhead so it should not be
// used if many consecutive general API calls won't be made, or if the
// application importing this package is performance critical and/or handling
// rate limiting itself.
func (kc *KrakenClient) StartRESTRateLimiter() error {
	if !kc.APIManager.HandleRateLimit.CompareAndSwap(false, true) {
		return fmt.Errorf("trading rate-limiter was already started")
	}
	return nil
}

// StopRESTRateLimiter stops self rate-limiting for general (everything except
// "order" type) REST API call methods.
func (kc *KrakenClient) StopRESTRateLimiter() error {
	if !kc.APIManager.HandleRateLimit.CompareAndSwap(true, false) {
		return fmt.Errorf("trading rate-limiter was already stopped or never initialized")
	}
	return nil
}

// #endregion

// #region Unexported KrakenClient helper methods

// getSignature generates a signature for a request to the Kraken API
func (kc *KrakenClient) getSignature(urlPath string, values url.Values) string {
	sha := sha256.New()
	sha.Write([]byte(values.Get("nonce") + values.Encode()))
	shasum := sha.Sum(nil)

	mac := hmac.New(sha512.New, kc.APISecret)
	mac.Write(append([]byte(urlPath), shasum...))
	macsum := mac.Sum(nil)
	return base64.StdEncoding.EncodeToString(macsum)
}

// doRequest sends a request to the Kraken API and returns the response
func (kc *KrakenClient) doRequest(urlPath string, values url.Values) (*http.Response, error) {
	signature := kc.getSignature(urlPath, values)

	req, err := http.NewRequest("POST", baseUrl+urlPath, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, fmt.Errorf("error calling http.NewRequest() | %w", err)
	}
	req.Header.Add("API-Key", kc.APIKey)
	req.Header.Add("API-Sign", signature)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	httpResp, err := kc.Client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "no such host") && strings.Contains(err.Error(), "lookup") {
			return nil, fmt.Errorf("%w | %w", errNoInternetConnection, err)
		} else if strings.Contains(err.Error(), "403") && strings.Contains(err.Error(), "forbidden") {
			return nil, fmt.Errorf("%w | %w", err403Forbidden, err)
		} else {
			return nil, fmt.Errorf("unknown http.Client.Do() error | %w", err)
		}
	}
	return httpResp, nil
}

// rateLimitAndIncrement checks rate limit increment won't exceed max counter
// cap when HandleRateLimit is true; if increment will exceed counter cap,
// waits for counter to decrement before proceeding.
func (kc *KrakenClient) rateLimitAndIncrement(incrementAmount uint8) {
	if kc.APIManager.HandleRateLimit.Load() {
		kc.APIManager.Mutex.Lock()
		for kc.APIManager.APICounter+incrementAmount >= kc.APIManager.MaxAPICounter {
			kc.ErrorLogger.Println("Counter will exceed rate limit. Waiting")
			kc.APIManager.CounterDecayCond.Wait()
		}
		if kc.APICounter == 0 {
			go kc.startAPIRateLimiter()
		}
		kc.APIManager.APICounter += incrementAmount
		kc.APIManager.Mutex.Unlock()
	}
}

// startAPIRateLimiter is a go routine method with a timer that decays the
// kc.APIManager.APICounter every second by the amount in kc.APICounterDecay;
// stops timer and returns when counter hits 0.
func (kc *KrakenClient) startAPIRateLimiter() {
	ticker := time.NewTicker(time.Second * time.Duration(kc.APIManager.APICounterDecay))
	defer ticker.Stop()
	for range ticker.C {
		kc.APIManager.Mutex.Lock()
		if kc.APIManager.APICounter > 0 {
			kc.APIManager.APICounter -= 1
			if kc.APIManager.APICounter == 0 {
				ticker.Stop()
				kc.APIManager.Mutex.Unlock()
				return
			}
		}
		kc.APIManager.Mutex.Unlock()
		kc.APIManager.CounterDecayCond.Broadcast()
	}
}

// #endregion

// #region Helper functions

// Processes Kraken API response and unmarshals it into ApiResp data struct.
// Passed arg 'target' is unmarshalled into ApiResp 'Result' field.
func processAPIResponse(res *http.Response, target interface{}) error {
	var err error
	if res.StatusCode == http.StatusOK {
		msg, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}
		resp := ApiResp{Result: target}
		err = json.Unmarshal(msg, &resp)
		if err != nil {
			err = fmt.Errorf("error unmarshalling msg to resp | %w", err)
			return err
		}
		if len(resp.Error) != 0 {
			err = fmt.Errorf("api error(s) | %v", resp.Error)
			return err
		}
		return nil
	} else {
		err = fmt.Errorf("http status code not OK; status code | %v", res.StatusCode)
		return err
	}
}

// Helper function to parse string 's' to float64 using strconv.ParseFloat() method,
// returns 0.0 if string is empty and wraps error message if ParseFloat returns err
func parseFloat64(s string) (float64, error) {
	if s == "" {
		return 0.0, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("ParseFloat error | %w", err)
	}
	return f, nil
}

// #endregion
