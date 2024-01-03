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

// Which time to use to search. Defaults to "both" if not called or invalid
// closeTime passed
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
