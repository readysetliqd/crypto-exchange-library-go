package krakenspot

import (
	"fmt"
	"net/url"
)

// For *KrakenClient method GetOpenOrders()
type GetOpenOrdersOption func(payload url.Values)

func WithTrades(trades bool) GetOpenOrdersOption {
	return func(payload url.Values) {
		payload.Add("trades", fmt.Sprintf("%v", trades))
	}
}

func WithUserRef(userRef int) GetOpenOrdersOption {
	return func(payload url.Values) {
		payload.Add("userref", fmt.Sprintf("%v", userRef))
	}
}
