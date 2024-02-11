// Package krakenspot is a comprehensive toolkit for interfacing with the Kraken
// Spot Exchange API. It enables WebSocket and REST API interactions, including
// subscription to both public and private channels. The package provides a
// client for initiating these interactions and a state manager for handling
// them.
//
// The constants.go file includes variables and constants user for managing
// connections, subscriptions, and interactions with the Kraken API. They
// include base URLs, WebSocket URLs, public and private channel names,
// order channel events, general message events, valid directions, and
// map sizes for pairs, assets, and tickers.
//
// The API rate limiter constants are used to manage the rate of API calls
// to the Kraken server, ensuring that the application adheres to Kraken's
// rate limits. These include decay rates, maximum counters, and trading
// decay rates for different tiers.
//
// Error definitions are also included to handle specific error scenarios
// that may occur during the operation of the application.
package krakenspot

import "errors"

// URL constants
const (
	baseUrl       = "https://api.kraken.com"
	publicPrefix  = "/0/public/"
	privatePrefix = "/0/private/"
	wsPublicURL   = "wss://ws.kraken.com"
	wsPrivateURL  = "wss://ws-auth.kraken.com"
)

// #region WebSocket constants

var heartbeat = []byte{123, 34, 101, 118, 101, 110, 116, 34, 58, 34, 104, 101, 97, 114, 116, 98, 101, 97, 116, 34, 125}

const timeoutDelay = 10

var publicChannelNames = map[string]bool{
	"ticker":     true,
	"ohlc-1":     true,
	"ohlc-5":     true,
	"ohlc-15":    true,
	"ohlc-30":    true,
	"ohlc-60":    true,
	"ohlc-240":   true,
	"ohlc-1440":  true,
	"ohlc-10080": true,
	"ohlc-21600": true,
	"trade":      true,
	"spread":     true,
	"book-10":    true,
	"book-25":    true,
	"book-100":   true,
	"book-500":   true,
	"book-1000":  true,
}

var privateChannelNames = map[string]bool{
	"ownTrades":  true,
	"openOrders": true,
}

var orderChannelEvents = map[string]bool{
	"addOrderStatus":             true,
	"editOrderStatus":            true,
	"cancelOrderStatus":          true,
	"cancelAllStatus":            true,
	"cancelAllOrdersAfterStatus": true,
}

var generalMessageEvents = map[string]bool{
	"systemStatus":       true,
	"subscriptionStatus": true,
	"pong":               true,
	"error":              true,
}

var validDirection = map[string]int8{
	"buy":  1,
	"sell": -1,
}

// #endregion

const (
	pairsMapSize   = 745 // As of 12/29/2023 there were 677 tradeable pairs. 10% added for buffer
	assetsMapSize  = 321 // As of 12/29/2023 there were 292 listed assets. 10% added for buffer
	tickersMapSize = 813 // As of 12/29/2023 there were 739 listed tickers. 10% added for buffer
)

// #region API rate limiter constants

const (
	tier1DecayRate         uint8  = 3 // seconds per 1 counter decay
	tier2DecayRate         uint8  = 2 // seconds per 1 counter decay
	tier3DecayRate         uint8  = 1 // seconds per 1 counter decay
	tier1MaxCounter        uint8  = 15
	tier2MaxCounter        uint8  = 20
	tier3MaxCounter        uint8  = 20
	tier1TradingDecayRate  uint16 = 1000 // milliseconds per 1 counter decay
	tier2TradingDecayRate  uint16 = 428  // milliseconds per 1 counter decay
	tier3TradingDecayRate  uint16 = 267  // milliseconds per 1 counter decay
	tier1TradingMaxCounter uint8  = 60
	tier2TradingMaxCounter uint8  = 125
	tier3TradingMaxCounter uint8  = 180
)

var decayRateMap = map[uint8]uint8{
	1: tier1DecayRate,
	2: tier2DecayRate,
	3: tier3DecayRate,
}

var maxCounterMap = map[uint8]uint8{
	1: tier1MaxCounter,
	2: tier2MaxCounter,
	3: tier3MaxCounter,
}

var decayTradingRateMap = map[uint8]uint16{
	1: tier1TradingDecayRate,
	2: tier2TradingDecayRate,
	3: tier3TradingDecayRate,
}

var maxTradingCounterMap = map[uint8]uint8{
	1: tier1TradingMaxCounter,
	2: tier2TradingMaxCounter,
	3: tier3TradingMaxCounter,
}

// #endregion

// Formatting for number of decimals for USD (ZUSD) asset on Kraken
const usdDecimalsFormat = "%.4f"

var errNotABookUpdateMsg = errors.New("not a book update message")
var errNoInternetConnection = errors.New("no internet connection")
var err403Forbidden = errors.New("forbidden error encountered; check Kraken's server status and/or your IP's geolocation")
var ErrTooManyArgs = errors.New("too many arguments passed to function/method")
