package krakenspot

import (
	"fmt"
	"net/url"
)

// For *KrakenClient method GetOpenOrders()
type GetOpenOrdersOption func(payload url.Values)

// Whether or not to include trades related to position in output. Defaults
// to false if not called
func OOWithTrades(trades bool) GetOpenOrdersOption {
	return func(payload url.Values) {
		payload.Add("trades", fmt.Sprintf("%v", trades))
	}
}

// Restrict results to given user reference id. Defaults to no restrictions
// if not called
func OOWithUserRef(userRef int) GetOpenOrdersOption {
	return func(payload url.Values) {
		payload.Add("userref", fmt.Sprintf("%v", userRef))
	}
}

// For *KrakenClient method GetOpenOrders()
type GetClosedOrdersOption func(payload url.Values)

// Whether or not to include trades related to position in output. Defaults
// to false if not called
func COWithTrades(trades bool) GetClosedOrdersOption {
	return func(payload url.Values) {
		payload.Add("trades", fmt.Sprintf("%v", trades))
	}
}

// Restrict results to given user reference id. Defaults to no restrictions
// if not called
func COWithUserRef(userRef int) GetClosedOrdersOption {
	return func(payload url.Values) {
		payload.Add("userref", fmt.Sprintf("%v", userRef))
	}
}

// Starting unix timestamp or order tx ID of results (exclusive). If an order's
// tx ID is given for start or end time, the order's opening time (opentm) is used.
// Defaults to show most recent orders if not called
func COWithStart(start int) GetClosedOrdersOption {
	return func(payload url.Values) {
		payload.Add("start", fmt.Sprintf("%v", start))
	}
}

// Ending unix timestamp or order tx ID of results (exclusive). If an order's
// tx ID is given for start or end time, the order's opening time (opentm) is used
// Defaults to show most recent orders if not called
func COWithEnd(end int) GetClosedOrdersOption {
	return func(payload url.Values) {
		payload.Add("end", fmt.Sprintf("%v", end))
	}
}

// Result offset for pagination. Defaults to no offset if not called
func COWithOffset(offset int) GetClosedOrdersOption {
	return func(payload url.Values) {
		payload.Add("ofs", fmt.Sprintf("%v", offset))
	}
}

// Which time to use to search and filter results for COWithStart() and COWithEnd()
// Defaults to "both" if not called or invalid arg 'closeTime' passed
//
// Enum: "open", "close", "both"
func COWithCloseTime(closeTime string) GetClosedOrdersOption {
	return func(payload url.Values) {
		validArgs := map[string]bool{
			"open":  true,
			"close": true,
			"both":  true,
		}
		if validArgs[closeTime] {
			payload.Add("closetime", fmt.Sprintf("%v", closeTime))
		}
	}
}

// Whether or not to consolidate trades by individual taker trades. Defaults to
// true if not called
func COWithConsolidateTaker(consolidateTaker bool) GetClosedOrdersOption {
	return func(payload url.Values) {
		payload.Add("consolidate_taker", fmt.Sprintf("%v", consolidateTaker))
	}
}

type GetOrdersInfoOptions func(payload url.Values)

// Whether or not to include trades related to position in output. Defaults
// to false if not called
func OIWithTrades(trades bool) GetOrdersInfoOptions {
	return func(payload url.Values) {
		payload.Add("trades", fmt.Sprintf("%v", trades))
	}
}

// Restrict results to given user reference id. Defaults to no restrictions
// if not called
func OIWithUserRef(userRef int) GetOrdersInfoOptions {
	return func(payload url.Values) {
		payload.Add("userref", fmt.Sprintf("%v", userRef))
	}
}

// Whether or not to consolidate trades by individual taker trades. Defaults to
// true if not called
func OIWithConsolidateTaker(consolidateTaker bool) GetOrdersInfoOptions {
	return func(payload url.Values) {
		payload.Add("consolidate_taker", fmt.Sprintf("%v", consolidateTaker))
	}
}

type GetTradesHistoryOptions func(payload url.Values)

// Type of trade. Defaults to "all" if not called or invalid 'tradeType' passed.
//
// Enum: "all", "any position", "closed position", "closing position", "no position"
func THWithType(tradeType string) GetTradesHistoryOptions {
	return func(payload url.Values) {
		validTradeTypes := map[string]bool{
			"all":              true,
			"any position":     true,
			"closed position":  true,
			"closing position": true,
			"no position":      true,
		}
		if validTradeTypes[tradeType] {
			payload.Add("type", tradeType)
		}
	}
}

// Whether or not to include trades related to position in output. Defaults
// to false if not called
func THWithTrades(trades bool) GetTradesHistoryOptions {
	return func(payload url.Values) {
		payload.Add("trades", fmt.Sprintf("%v", trades))
	}
}

// Starting unix timestamp or order tx ID of results (exclusive). If an order's
// tx ID is given for start or end time, the order's opening time (opentm) is used.
// Defaults to show most recent orders if not called
func THWithStart(start int) GetTradesHistoryOptions {
	return func(payload url.Values) {
		payload.Add("start", fmt.Sprintf("%v", start))
	}
}

// Ending unix timestamp or order tx ID of results (exclusive). If an order's
// tx ID is given for start or end time, the order's opening time (opentm) is used
// Defaults to show most recent orders if not called
func THWithEnd(end int) GetTradesHistoryOptions {
	return func(payload url.Values) {
		payload.Add("end", fmt.Sprintf("%v", end))
	}
}

// Result offset for pagination. Defaults to no offset if not called
func THWithOffset(offset int) GetTradesHistoryOptions {
	return func(payload url.Values) {
		payload.Add("ofs", fmt.Sprintf("%v", offset))
	}
}

// Whether or not to consolidate trades by individual taker trades. Defaults to
// true if not called
func THWithConsolidateTaker(consolidateTaker bool) GetTradesHistoryOptions {
	return func(payload url.Values) {
		payload.Add("consolidate_taker", fmt.Sprintf("%v", consolidateTaker))
	}
}
