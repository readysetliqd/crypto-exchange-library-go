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

// const (
// 	wsTimeoutDuration = time.Second * 10
// )

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

// #endregion

const (
	pairsMapSize   = 745 // As of 12/29/2023 there were 677 tradeable pairs. 10% added for buffer
	assetsMapSize  = 321 // As of 12/29/2023 there were 292 listed assets. 10% added for buffer
	tickersMapSize = 813 // As of 12/29/2023 there were 739 listed tickers. 10% added for buffer
)

// #region API rate limiter constants

const (
	tier1DecayRate  uint8 = 3 // seconds per 1 counter decay
	tier2DecayRate  uint8 = 2 // seconds per 1 counter decay
	tier3DecayRate  uint8 = 1 // seconds per 1 counter decay
	tier1MaxCounter uint8 = 15
	tier2MaxCounter uint8 = 20
	tier3MaxCounter uint8 = 20
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

// #endregion

// Formatting for number of decimals for USD (ZUSD) asset on Kraken
const usdDecimalsFormat = "%.4f"

var errNotABookUpdateMsg = errors.New("not a book update message")
