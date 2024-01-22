package krakenspot

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
)

// #region Private Account Data endpoints functional options

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

type GetOrdersInfoOption func(payload url.Values)

// Whether or not to include trades related to position in output. Defaults
// to false if not called
func OIWithTrades(trades bool) GetOrdersInfoOption {
	return func(payload url.Values) {
		payload.Add("trades", fmt.Sprintf("%v", trades))
	}
}

// Restrict results to given user reference id. Defaults to no restrictions
// if not called
func OIWithUserRef(userRef int) GetOrdersInfoOption {
	return func(payload url.Values) {
		payload.Add("userref", fmt.Sprintf("%v", userRef))
	}
}

// Whether or not to consolidate trades by individual taker trades. Defaults to
// true if not called
func OIWithConsolidateTaker(consolidateTaker bool) GetOrdersInfoOption {
	return func(payload url.Values) {
		payload.Add("consolidate_taker", fmt.Sprintf("%v", consolidateTaker))
	}
}

type GetTradesHistoryOption func(payload url.Values)

// Type of trade. Defaults to "all" if not called or invalid 'tradeType' passed.
//
// Enum: "all", "any position", "closed position", "closing position", "no position"
func THWithType(tradeType string) GetTradesHistoryOption {
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
func THWithTrades(trades bool) GetTradesHistoryOption {
	return func(payload url.Values) {
		payload.Add("trades", fmt.Sprintf("%v", trades))
	}
}

// Starting unix timestamp or order tx ID of results (exclusive). If an order's
// tx ID is given for start or end time, the order's opening time (opentm) is used.
// Defaults to show most recent orders if not called
func THWithStart(start int) GetTradesHistoryOption {
	return func(payload url.Values) {
		payload.Add("start", fmt.Sprintf("%v", start))
	}
}

// Ending unix timestamp or order tx ID of results (exclusive). If an order's
// tx ID is given for start or end time, the order's opening time (opentm) is used
// Defaults to show most recent orders if not called
func THWithEnd(end int) GetTradesHistoryOption {
	return func(payload url.Values) {
		payload.Add("end", fmt.Sprintf("%v", end))
	}
}

// Result offset for pagination. Defaults to no offset if not called
func THWithOffset(offset int) GetTradesHistoryOption {
	return func(payload url.Values) {
		payload.Add("ofs", fmt.Sprintf("%v", offset))
	}
}

// Whether or not to consolidate trades by individual taker trades. Defaults to
// true if not called
func THWithConsolidateTaker(consolidateTaker bool) GetTradesHistoryOption {
	return func(payload url.Values) {
		payload.Add("consolidate_taker", fmt.Sprintf("%v", consolidateTaker))
	}
}

type GetTradeInfoOption func(payload url.Values)

// Whether or not to include trades related to position in output. Defaults
// to false if not called
func TIWithTrades(trades bool) GetTradeInfoOption {
	return func(payload url.Values) {
		payload.Add("trades", fmt.Sprintf("%v", trades))
	}
}

type GetOpenPositionsOption func(payload url.Values)

// Comma delimited list of txids to limit output to. Defaults to show all open
// positions if not called
func OPWithTxID(txID string) GetOpenPositionsOption {
	return func(payload url.Values) {
		payload.Add("txid", txID)
	}
}

// Whether to include P&L calculations. Defaults to false if not called
func OPWithDoCalcs(doCalcs bool) GetOpenPositionsOption {
	return func(payload url.Values) {
		payload.Add("docalcs", fmt.Sprintf("%v", doCalcs))
	}
}

type GetOpenPositionsConsolidatedOption func(payload url.Values)

// Comma delimited list of txids to limit output to. Defaults to show all open
// positions if not called
func OPCWithTxID(txID string) GetOpenPositionsConsolidatedOption {
	return func(payload url.Values) {
		payload.Add("txid", txID)
	}
}

// Whether to include P&L calculations. Defaults to false if not called
func OPCWithDoCalcs(doCalcs bool) GetOpenPositionsConsolidatedOption {
	return func(payload url.Values) {
		payload.Add("docalcs", fmt.Sprintf("%v", doCalcs))
	}
}

type GetLedgersInfoOption func(payload url.Values)

// // Filter output by asset or comma delimited list of assets. Defaults to "all"
// if not called.
func LIWithAsset(asset string) GetLedgersInfoOption {
	return func(payload url.Values) {
		payload.Add("asset", asset)
	}
}

// Filter output by asset class. Defaults to "currency" if not called
//
// Enum: "currency", ...?
func LIWithAclass(aclass string) GetLedgersInfoOption {
	return func(payload url.Values) {
		payload.Add("aclass", aclass)
	}
}

// Type of ledger to retrieve. Defaults to "all" if not called or invalid
// 'ledgerType' passed.
//
// Enum: "all", "trade", "deposit", "withdrawal", "transfer", "margin", "adjustment",
// "rollover", "credit", "settled", "staking", "dividend", "sale", "nft_rebate"
func LIWithType(ledgerType string) GetLedgersInfoOption {
	return func(payload url.Values) {
		validLedgerTypes := map[string]bool{
			"all":        true,
			"trade":      true,
			"deposit":    true,
			"withdrawal": true,
			"transfer":   true,
			"margin":     true,
			"adjustment": true,
			"rollover":   true,
			"credit":     true,
			"settled":    true,
			"staking":    true,
			"dividend":   true,
			"sale":       true,
			"nft_rebate": true,
		}
		if validLedgerTypes[ledgerType] {
			payload.Add("type", ledgerType)
		}
	}
}

// Starting unix timestamp or ledger ID of results (exclusive). Defaults to most
// recent ledgers if not called.
func LIWithStart(start int) GetLedgersInfoOption {
	return func(payload url.Values) {
		payload.Add("start", fmt.Sprintf("%v", start))
	}
}

// Ending unix timestamp or ledger ID of results (inclusive). Defaults to most
// recent ledgers if not called.
func LIWithEnd(end int) GetLedgersInfoOption {
	return func(payload url.Values) {
		payload.Add("end", fmt.Sprintf("%v", end))
	}
}

// Result offset for pagination. Defaults to no offset if not called.
func LIWithOffset(offset int) GetLedgersInfoOption {
	return func(payload url.Values) {
		payload.Add("ofs", fmt.Sprintf("%v", offset))
	}
}

// If true, does not retrieve count of ledger entries. Request can be noticeably
// faster for users with many ledger entries as this avoids an extra database query.
// Defaults to false if not called.
func LIWithoutCount(withoutCount bool) GetLedgersInfoOption {
	return func(payload url.Values) {
		payload.Add("without_count", fmt.Sprintf("%v", withoutCount))
	}
}

type GetLedgerOption func(payload url.Values)

// Whether or not to include trades related to position in output. Defaults to
// false if not called.
func GLWithTrades(trades bool) GetLedgerOption {
	return func(payload url.Values) {
		payload.Add("trades", fmt.Sprintf("%v", trades))
	}
}

type GetTradeVolumeOption func(payload url.Values)

// Comma delimited list of asset pairs to get fee info on. Defaults to show none
// if not called.
func TVWithPair(pair string) GetTradeVolumeOption {
	return func(payload url.Values) {
		payload.Add("pair", pair)
	}
}

type RequestTradesExportReportOption func(payload url.Values)

// File format to export. Defaults to "CSV" if not called or invalid value
// passed to arg 'format'
//
// Enum: "CSV", "TSV"
func RTWithFormat(format string) RequestTradesExportReportOption {
	return func(payload url.Values) {
		validFormat := map[string]bool{
			"CSV": true,
			"TSV": true,
		}
		if validFormat[format] {
			payload.Add("format", format)
		}
	}
}

// Accepts comma-delimited list of fields passed as a single string to include
// in report. Defaults to "all" if not called. API will return error:
// [EGeneral:Internal error] if invalid value passed to arg 'fields'. Function has
// no validation checks for passed value to 'fields'
//
// Enum: "ordertxid", "time", "ordertype", "price", "cost", "fee", "vol",
// "margin", "misc", "ledgers"
func RTWithFields(fields string) RequestTradesExportReportOption {
	return func(payload url.Values) {
		payload.Add("fields", fields)
	}
}

// UNIX timestamp for report start time. Defaults to 1st of the current month
// if not called
func RTWithStart(start int) RequestTradesExportReportOption {
	return func(payload url.Values) {
		payload.Add("start", fmt.Sprintf("%v", start))
	}
}

// UNIX timestamp for report end time. Defaults to current time if not called
func RTWithEnd(end int) RequestTradesExportReportOption {
	return func(payload url.Values) {
		payload.Add("end", fmt.Sprintf("%v", end))
	}
}

type RequestLedgersExportReportOption func(payload url.Values)

// File format to export. Defaults to "CSV" if not called or invalid value
// passed to arg 'format'
//
// Enum: "CSV", "TSV"
func RLWithFormat(format string) RequestLedgersExportReportOption {
	return func(payload url.Values) {
		validFormat := map[string]bool{
			"CSV": true,
			"TSV": true,
		}
		if validFormat[format] {
			payload.Add("format", format)
		}
	}
}

// Accepts comma-delimited list of fields passed as a single string to include
// in report. Defaults to "all" if not called. API will return error:
// [EGeneral:Internal error] if invalid value passed to arg 'fields'. Function has
// no validation checks for passed value to 'fields'
//
// Enum: "refid", "time", "type", "aclass", "asset", "amount", "fee", "balance"
func RLWithFields(fields string) RequestLedgersExportReportOption {
	return func(payload url.Values) {
		payload.Add("fields", fields)
	}
}

// UNIX timestamp for report start time. Defaults to 1st of the current month
// if not called
func RLWithStart(start int) RequestLedgersExportReportOption {
	return func(payload url.Values) {
		payload.Add("start", fmt.Sprintf("%v", start))
	}
}

// UNIX timestamp for report end time. Defaults to current time if not called
func RLWithEnd(end int) RequestLedgersExportReportOption {
	return func(payload url.Values) {
		payload.Add("end", fmt.Sprintf("%v", end))
	}
}

// #endregion

// #region Private REST API Trading endpoints functional options

type OrderType func(payload url.Values)

// Instantly market orders in at best current prices
func Market() OrderType {
	return func(payload url.Values) {
		payload.Add("ordertype", "market")
	}
}

// Order type of "limit" where arg 'price' is the level at which the limit order
// will be placed.
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func Limit(price string) OrderType {
	return func(payload url.Values) {
		payload.Add("ordertype", "limit")
		payload.Add("price", price)
	}
}

// Order type of "stop-loss" order type where arg 'price' is the stop loss
// trigger price
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func StopLoss(price string) OrderType {
	return func(payload url.Values) {
		payload.Add("ordertype", "stop-loss")
		payload.Add("price", price)
	}
}

// Order type of "take-profit" where arg 'price' is the take profit trigger price.
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func TakeProfit(price string) OrderType {
	return func(payload url.Values) {
		payload.Add("ordertype", "take-profit")
		payload.Add("price", price)
	}
}

// Order type of "stop-loss-limit" where arg 'price' is the stop loss trigger
// price and arg 'price2' is the limit order that will be placed.
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func StopLossLimit(price, price2 string) OrderType {
	return func(payload url.Values) {
		payload.Add("ordertype", "stop-loss-limit")
		payload.Add("price", price)
		payload.Add("price2", price2)
	}
}

// Order type of "take-profit-limit" where arg 'price' is the take profit trigger
// price and arg 'price2' is the limit order that will be placed.
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func TakeProfitLimit(price, price2 string) OrderType {
	return func(payload url.Values) {
		payload.Add("ordertype", "take-profit-limit")
		payload.Add("price", price)
		payload.Add("price2", price2)
	}
}

// Order type of "trailing-stop" where arg 'price' is the relative stop trigger
// price.
//
// Note: Required arg 'price' must use a relative price for this field, namely
// the + prefix, from which the direction will be automatic based on if the
// original order is a buy or sell (no need to use - or #). The % suffix also
// works for these order types to use a relative percentage price.
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func TrailingStop(price string) OrderType {
	return func(payload url.Values) {
		payload.Add("ordertype", "trailing-stop")
		payload.Add("price", price)
	}
}

// Order type of "trailing-stop-limit" where arg 'price' is the relative stop
// trigger price and arg 'price2' is the limit order that will be placed.
//
// Note: Required arg 'price' must use a relative price for this field, namely
// the + prefix, from which the direction will be automatic based on if the
// original order is a buy or sell (no need to use - or #). The % suffix also
// works for these order types to use a relative percentage price.
//
// Note: In practice, the system will accept either relative or specific (without +
// or -) for arg 'price2' despite what the docs say. The note is included here
// just in case; Kraken API docs: Must use a relative price for this field, namely
// one of the + or - prefixes. This will provide the offset from the trigger price
// to the limit price, i.e. +0 would set the limit price equal to the trigger price.
// The % suffix also works for this field to use a relative percentage limit price.
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func TrailingStopLimit(price, price2 string) OrderType {
	return func(payload url.Values) {
		payload.Add("ordertype", "trailing-stop-limit")
		payload.Add("price", price)
		payload.Add("price2", price2)
	}
}

// Order type of "settle-position". Settles any open margin position of same
// 'direction' and 'pair' by amount 'volume'
//
// Note: AddOrder() arg 'volume' can be set to "0" for closing margin orders to
// automatically fill the requisite quantity
//
// Note: Required arg 'leverage' is a required parameter by the API, but in
// practice can be any valid value. Value of 'leverage' here has no effect as
// leverage is already set by the opened position.
func SettlePosition(leverage string) OrderType {
	return func(payload url.Values) {
		payload.Add("ordertype", "settle-position")
		payload.Add("leverage", leverage)
	}
}

type AddOrderOption func(payload url.Values)

// User reference id 'userref' is an optional user-specified integer id that
// can be associated with any number of orders. Many clients choose a userref
// corresponding to a unique integer id generated by their systems (e.g. a
// timestamp). However, because we don't enforce uniqueness on our side, it
// can also be used to easily group orders by pair, side, strategy, etc. This
// allows clients to more readily cancel or query information about orders in
// a particular group, with fewer API calls by using userref instead of our
// txid, where supported.
func UserRef(userRef string) AddOrderOption {
	return func(payload url.Values) {
		payload.Add("userref", userRef)
	}
}

// Used to create an iceberg order, this is the visible order quantity in terms
// of the base asset. The rest of the order will be hidden, although the full
// volume can be filled at any time by any order of that size or larger that
// matches in the order book. DisplayVolume() can only be used with the Limit()
// order type. Must be greater than 0, and less than AddOrder() arg 'volume'.
func DisplayVolume(displayVol string) AddOrderOption {
	return func(payload url.Values) {
		payload.Add("displayvol", displayVol)
	}
}

// Price signal used to trigger stop-loss, stop-loss-limit, take-profit,
// take-profit-limit, trailing-stop and trailing-stop-limit orders. Defaults to
// "last" trigger type if not called. Calling this function overrides default to
// "index".
//
// Note: This trigger type will also be used for any associated conditional
// close orders.
//
// Note: To keep triggers serviceable, the last price will be used as fallback
// reference price during connectivity issues with external index feeds.
func IndexTrigger() AddOrderOption {
	return func(payload url.Values) {
		payload.Add("trigger", "index")
	}
}

// Amount of leverage desired. Defaults to no leverage if function is not called.
// API accepts string of any number; in practice, must be some integer >= 2
//
// Note: This function should not be used when calling AddOrder() with the
// SettlePosition() 'orderType'
func Leverage(leverage string) AddOrderOption {
	return func(payload url.Values) {
		payload.Add("leverage", leverage)
	}
}

// If true, order will only reduce a currently open position, not increase it
// or open a new position. Defaults to false if not passed.
//
// Note: ReduceOnly() is only usable with leveraged orders. This includes orders
// of 'orderType' SettlePosition() and orders with Leverage() passed to 'options'
func ReduceOnly() AddOrderOption {
	return func(payload url.Values) {
		payload.Add("reduce_only", "true")
	}
}

// Sets self trade behavior to "cancel-oldest". Overrides default value when called.
// Default "cancel-newest"
//
// CAUTION: Mutually exclusive with STPCancelBoth(). Only one of these two functions
// should be called, if any. Order will still pass as valid but one "stptype"
// value will be overridden.
//
// Self trade prevention behavior definition:
//
// "cancel-newest" - if self trade is triggered, arriving order will be canceled
//
// "cancel-oldest" - if self trade is triggered, resting order will be canceled
//
// "cancel-both" - if self trade is triggered, both arriving and resting orders
// will be canceled
func STPCancelOldest() AddOrderOption {
	return func(payload url.Values) {
		payload.Add("stptype", "cancel-oldest")
	}
}

// Sets self trade behavior to "cancel-both". Overrides default value when called.
// Default "cancel-newest"
//
// CAUTION: Mutually exclusive with STPCancelOldest(). Only one of these two functions
// should be called, if any. Order will still pass as valid but one "stptype"
// value will be overridden.
//
// Self trade prevention behavior definition:
//
// "cancel-newest" - if self trade is triggered, arriving order will be canceled
//
// "cancel-oldest" - if self trade is triggered, resting order will be canceled
//
// "cancel-both" - if self trade is triggered, both arriving and resting orders
// will be canceled
func STPCancelBoth() AddOrderOption {
	return func(payload url.Values) {
		payload.Add("stptype", "cancel-both")
	}
}

// Post-only order (available when ordertype = limit)
func PostOnly() AddOrderOption {
	return func(payload url.Values) {
		addFlag(payload, "post")
	}
}

// Prefer fee in base currency (default if selling)
//
// CAUTION: Mutually exclusive with FCIQ(). Only call one of these two at a time
func FCIB() AddOrderOption {
	return func(payload url.Values) {
		addFlag(payload, "fcib")
	}
}

// Prefer fee in quote currency (default if buying)
//
// CAUTION: Mutually exclusive with FCIB(). Only call one of these two at a time
func FCIQ() AddOrderOption {
	return func(payload url.Values) {
		addFlag(payload, "fciq")
	}
}

// Disables market price protection for market orders
func NOMPP() AddOrderOption {
	return func(payload url.Values) {
		addFlag(payload, "nompp")
	}
}

// Order volume expressed in quote currency. This is supported only for market orders.
func VIQC() AddOrderOption {
	return func(payload url.Values) {
		addFlag(payload, "viqc")
	}
}

// Helper function for building "oflags" parameter
func addFlag(payload url.Values, flag string) {
	if oflags, ok := payload["oflags"]; ok {
		// "oflags" key exists, append the new flag
		payload.Set("oflags", oflags[0]+","+flag)
	} else {
		// "oflags" key doesn't exist, add the new flag
		payload.Add("oflags", flag)
	}
}

// Time-in-force of the order to specify how long it should remain in the order
// book before being cancelled. Overrides default value with "IOC" (Immediate Or
// Cancel). IOC will immediately execute the amount possible and cancel any
// remaining balance rather than resting in the book. Defaults to "GTC" (Good
// 'Til Canceled) if function is not called.
//
// CAUTION: Mutually exclusive with GoodTilDate(). Only one of these two functions
// should be called, if any. Order will still pass as valid but one "timeinforce"
// value will be overridden.
func ImmediateOrCancel() AddOrderOption {
	return func(payload url.Values) {
		payload.Add("timeinforce", "IOC")
	}
}

// Time-in-force of the order to specify how long it should remain in the order
// book before being cancelled. Overrides default value with "GTD" (Good Til Date).
// GTD, if called, will cause the order to expire at specified unix time passed
// to arg 'expireTime'. Expiration time, can be specified as an absolute timestamp
// or as a number of seconds in the future:
//
// 0 no expiration (default)
//
// <n> = unix timestamp of expiration time
//
// +<n> = expire <n> seconds from now, minimum 5 seconds
//
// Note: URL encoding of the + character changes it to a space, so please use
// %2b followed by the number of seconds instead of +
//
// CAUTION: Mutually exclusive with ImmediateOrCancel(). Only one of these two functions
// should be called, if any. Order will still pass as valid but one "timeinforce"
// value will be overridden.
func GoodTilDate(expireTime string) AddOrderOption {
	return func(payload url.Values) {
		payload.Add("timeinforce", "GTD")
		payload.Add("expiretm", expireTime)
	}
}

// Conditional close of "limit" order type where arg 'price' is the level at which
// the limit order will be placed.
//
// CAUTION: Mutually exclusive with other conditional close order functions.
// Only one conditional close order function with format Close<orderType>()
// should be passed at a time. If more than one are passed, it will override the
// previous function calls.
//
// Conditional Close Orders: Orders that are triggered by execution of the
// primary order in the same quantity and opposite direction, but once triggered
// are independent orders that may reduce or increase net position.
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func CloseLimit(price string) AddOrderOption {
	return func(payload url.Values) {
		payload.Add("close[ordertype]", "limit")
		payload.Add("close[price]", price)
	}
}

// Conditional close of "stop-loss" order type where arg 'price' is the stop
// loss trigger price.
//
// CAUTION: Mutually exclusive with other conditional close order functions.
// Only one conditional close order function with format Close<orderType>()
// should be passed at a time. If more than one are passed, it will override the
// previous function calls.
//
// Conditional Close Orders: Orders that are triggered by execution of the
// primary order in the same quantity and opposite direction, but once triggered
// are independent orders that may reduce or increase net position
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func CloseStopLoss(price string) AddOrderOption {
	return func(payload url.Values) {
		payload.Add("close[ordertype]", "stop-loss")
		payload.Add("close[price]", price)
	}
}

// Conditional close of "take-profit" order type where arg 'price' is the take
// profit trigger price.
//
// CAUTION: Mutually exclusive with other conditional close order functions.
// Only one conditional close order function with format Close<orderType>()
// should be passed at a time. If more than one are passed, it will override the
// previous function calls.
//
// Conditional Close Orders: Orders that are triggered by execution of the
// primary order in the same quantity and opposite direction, but once triggered
// are independent orders that may reduce or increase net position
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func CloseTakeProfit(price string) AddOrderOption {
	return func(payload url.Values) {
		payload.Add("close[ordertype]", "take-profit")
		payload.Add("close[price]", price)
	}
}

// Conditional close of "stop-loss-limit" order type where arg 'price' is the
// stop loss trigger price and arg 'price2' is the limit order that will be placed.
//
// CAUTION: Mutually exclusive with other conditional close order functions.
// Only one conditional close order function with format Close<orderType>()
// should be passed at a time. If more than one are passed, it will override the
// previous function calls.
//
// Conditional Close Orders: Orders that are triggered by execution of the
// primary order in the same quantity and opposite direction, but once triggered
// are independent orders that may reduce or increase net position
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func CloseStopLossLimit(price, price2 string) AddOrderOption {
	return func(payload url.Values) {
		payload.Add("close[ordertype]", "stop-loss-limit")
		payload.Add("close[price]", price)
		payload.Add("close[price2]", price2)
	}
}

// Conditional close of "take-profit-limit" order type where arg 'price' is the
// take profit trigger price and arg 'price2' is the limit order that will be
// placed.
//
// CAUTION: Mutually exclusive with other conditional close order functions.
// Only one conditional close order function with format Close<orderType>()
// should be passed at a time. If more than one are passed, it will override the
// previous function calls.
//
// Conditional Close Orders: Orders that are triggered by execution of the
// primary order in the same quantity and opposite direction, but once triggered
// are independent orders that may reduce or increase net position
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func CloseTakeProfitLimit(price, price2 string) AddOrderOption {
	return func(payload url.Values) {
		payload.Add("close[ordertype]", "take-profit-limit")
		payload.Add("close[price]", price)
		payload.Add("close[price2]", price2)
	}
}

// Conditional close of "trailing-stop" order type where arg 'price' is the relative
// stop trigger price.
//
// CAUTION: Mutually exclusive with other conditional close order functions.
// Only one conditional close order function with format Close<orderType>()
// should be passed at a time. If more than one are passed, it will override the
// previous function calls.
//
// Conditional Close Orders: Orders that are triggered by execution of the
// primary order in the same quantity and opposite direction, but once triggered
// are independent orders that may reduce or increase net position
//
// Note: Required arg 'price' must use a relative price for this field, namely
// the + prefix, from which the direction will be automatic based on if the
// original order is a buy or sell (no need to use - or #). The % suffix also
// works for these order types to use a relative percentage price.
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func CloseTrailingStop(price string) AddOrderOption {
	return func(payload url.Values) {
		payload.Add("close[ordertype]", "trailing-stop")
		payload.Add("close[price]", price)
	}
}

// Conditional close of "trailing-stop-limit" order type where arg 'price' is the
// relative stop trigger price and arg 'price2' is the limit order that will be
// placed.
//
// CAUTION: Mutually exclusive with other conditional close order functions.
// Only one conditional close order function with format Close<orderType>()
// should be passed at a time. If more than one are passed, it will override the
// previous function calls.
//
// Conditional Close Orders: Orders that are triggered by execution of the
// primary order in the same quantity and opposite direction, but once triggered
// are independent orders that may reduce or increase net position
//
// Note: Required arg 'price' must use a relative price for this field, namely
// the + prefix, from which the direction will be automatic based on if the
// original order is a buy or sell (no need to use - or #). The % suffix also
// works for these order types to use a relative percentage price.
//
// Note: In practice, the system will accept either relative or specific (without +
// or -) for arg 'price2' despite what the docs say. The note is included here
// just in case; Kraken API docs: Must use a relative price for this field, namely
// one of the + or - prefixes. This will provide the offset from the trigger price
// to the limit price, i.e. +0 would set the limit price equal to the trigger price.
// The % suffix also works for this field to use a relative percentage limit price.
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func CloseTrailingStopLimit(price, price2 string) AddOrderOption {
	return func(payload url.Values) {
		payload.Add("close[ordertype]", "trailing-stop-limit")
		payload.Add("close[price]", price)
		payload.Add("close[price2]", price2)
	}
}

// Pass RFC3339 timestamp (e.g. 2021-04-01T00:18:45Z) after which the matching
// engine should reject the new order request to arg 'deadline'.
//
// In presence of latency or order queueing: min now() + 2 seconds, max now() +
// 60 seconds.
//
// Example Usage:
//
// AddWithDeadline(time.Now().Add(time.Second*30).Format(time.RFC3339))
func AddWithDeadline(deadline string) AddOrderOption {
	return func(payload url.Values) {
		payload.Add("deadline", deadline)
	}
}

// Validates inputs only. Does not submit order. Defaults to "false" if not called.
func ValidateAddOrder() AddOrderOption {
	return func(payload url.Values) {
		payload.Add("validate", "true")
	}
}

type AddOrderBatchOption func(payload url.Values)

// Pass RFC3339 timestamp (e.g. 2021-04-01T00:18:45Z) after which the matching
// engine should reject the new order request to arg 'deadline'.
//
// In presence of latency or order queueing: min now() + 2 seconds, max now() +
// 60 seconds.
func AddBatchWithDeadline(deadline string) AddOrderBatchOption {
	return func(payload url.Values) {
		payload.Add("deadline", deadline)
	}
}

// Validates inputs only. Do not submit order. Defaults to "false" if not called.
func ValidateAddOrderBatch() AddOrderBatchOption {
	return func(payload url.Values) {
		payload.Add("validate", "true")
	}
}

type EditOrderOption func(payload url.Values)

// Field "userref" is an optional user-specified integer id associated with
// edit request.
//
// Note: userref from parent order will not be retained on the new order after
// edit.
func NewUserRef(userRef string) EditOrderOption {
	return func(payload url.Values) {
		payload.Add("userref", userRef)
	}
}

// Updates order quantity in terms of the base asset.
func NewVolume(volume string) EditOrderOption {
	return func(payload url.Values) {
		payload.Add("volume", volume)
	}
}

// Used to edit an iceberg order, this is the visible order quantity in terms
// of the base asset. The rest of the order will be hidden, although the full
// volume can be filled at any time by any order of that size or larger that
// matches in the order book. displayvol can only be used with the limit order
// type, must be greater than 0, and less than volume.
func NewDisplayVolume(displayVol string) EditOrderOption {
	return func(payload url.Values) {
		payload.Add("displayvol", displayVol)
	}
}

// Updates limit price for "limit" orders. Updates trigger price for "stop-loss",
// "stop-loss-limit", "take-profit", "take-profit-limit", "trailing-stop" and
// "trailing-stop-limit" orders
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
//
// Trailing Stops: Required arg 'price' must use a relative price for this field,
// namely the + prefix, from which the direction will be automatic based on if
// the original order is a buy or sell (no need to use - or #). The % suffix
// also works for these order types to use a relative percentage price.
func NewPrice(price string) EditOrderOption {
	return func(payload url.Values) {
		payload.Add("price", price)
	}
}

// Updates limit price for "stop-loss-limit", "take-profit-limit" and
// "trailing-stop-limit" orders
//
// Trailing Stops Note: In practice, the system will accept either relative or
// specific (without + or -) for arg 'price2' despite what the docs say. The
// note is included here just in case; Kraken API docs: Must use a relative
// price for this field, namely one of the + or - prefixes. This will provide
// the offset from the trigger price to the limit price, i.e. +0 would set the
// limit price equal to the trigger price. The % suffix also works for this
// field to use a relative percentage limit price.
func NewPrice2(price2 string) EditOrderOption {
	return func(payload url.Values) {
		payload.Add("price2", price2)
	}
}

// Post-only order (available when ordertype = limit). All the flags from the
// parent order are retained except post-only. Post-only needs to be explicitly
// mentioned on every edit request.
func NewPostOnly() EditOrderOption {
	return func(payload url.Values) {
		payload.Add("oflags", "post")
	}
}

// RFC3339 timestamp (e.g. 2021-04-01T00:18:45Z) after which the matching
// engine should reject the new order request, in presence of latency or order
// queueing. min now() + 2 seconds, max now() + 60 seconds.
func NewDeadline(deadline string) EditOrderOption {
	return func(payload url.Values) {
		payload.Add("deadline", deadline)
	}
}

// Used to interpret if client wants to receive pending replace, before the
// order is completely replaced. Defaults to "false" if not called.
func NewCancelResponse() EditOrderOption {
	return func(payload url.Values) {
		payload.Add("cancel_response", "true")
	}
}

// Validate inputs only. Do not submit order. Defaults to false if not called.
func ValidateEditOrder() EditOrderOption {
	return func(payload url.Values) {
		payload.Add("validate", "true")
	}
}

// #endregion

// #region Private Funding endpoints functional options

// For *KrakenClient method GetDepositMethods()
type GetDepositMethodsOption func(payload url.Values)

// Asset class being deposited (optional). Defaults to "currency" if function is
// not called. As of 1/7/24, only known valid asset class is "currency"
func DMWithAssetClass(aclass string) GetDepositMethodsOption {
	return func(payload url.Values) {
		payload.Add("aclass", aclass)
	}
}

// For *KrakenClient method GetDepositAddresses()
type GetDepositAddressesOption func(payload url.Values)

// Whether or not to generate a new address. Defaults to false if function is
// not called.
func DAWithNew() GetDepositAddressesOption {
	return func(payload url.Values) {
		payload.Add("new", "true")
	}
}

// Amount you wish to deposit (only required for method=Bitcoin Lightning)
func DAWithAmount(amount string) GetDepositAddressesOption {
	return func(payload url.Values) {
		payload.Add("amount", amount)
	}
}

// For *KrakenClient method GetDepositsStatus()
type GetDepositsStatusOption func(payload url.Values)

// Filter for specific asset being deposited
func DSWithAsset(asset string) GetDepositsStatusOption {
	return func(payload url.Values) {
		payload.Add("asset", asset)
	}
}

// Filter for specific name of deposit method
func DSWithMethod(method string) GetDepositsStatusOption {
	return func(payload url.Values) {
		payload.Add("method", method)
	}
}

// Start timestamp, deposits created strictly before will not be included in
// the response
func DSWithStart(start string) GetDepositsStatusOption {
	return func(payload url.Values) {
		payload.Add("start", start)
	}
}

// End timestamp, deposits created strictly after will be not be included in
// the response
func DSWithEnd(end string) GetDepositsStatusOption {
	return func(payload url.Values) {
		payload.Add("end", end)
	}
}

// Number of results to include per page
func DSWithLimit(limit uint) GetDepositsStatusOption {
	return func(payload url.Values) {
		payload.Add("limit", fmt.Sprintf("%v", limit))
	}
}

// Filter asset class being deposited. Defaults to "currency" if function is
// not called. As of 1/7/24, only known valid asset class is "currency"
func DSWithAssetClass(aclass string) GetDepositsStatusOption {
	return func(payload url.Values) {
		payload.Add("aclass", aclass)
	}
}

// For *KrakenClient method GetDepositsStatusPaginated()
type GetDepositsStatusPaginatedOption func(payload url.Values)

// Filter for specific asset being deposited
func DPWithAsset(asset string) GetDepositsStatusPaginatedOption {
	return func(payload url.Values) {
		payload.Add("asset", asset)
	}
}

// Filter for specific name of deposit method
func DPWithMethod(method string) GetDepositsStatusPaginatedOption {
	return func(payload url.Values) {
		payload.Add("method", method)
	}
}

// Start timestamp, deposits created strictly before will not be included in
// the response
func DPWithStart(start string) GetDepositsStatusPaginatedOption {
	return func(payload url.Values) {
		payload.Add("start", start)
	}
}

// End timestamp, deposits created strictly after will be not be included in
// the response
func DPWithEnd(end string) GetDepositsStatusPaginatedOption {
	return func(payload url.Values) {
		payload.Add("end", end)
	}
}

// Number of results to include per page
func DPWithLimit(limit uint) GetDepositsStatusPaginatedOption {
	return func(payload url.Values) {
		payload.Add("limit", fmt.Sprintf("%v", limit))
	}
}

// Filter asset class being deposited. Defaults to "currency" if function is
// not called. As of 1/7/24, only known valid asset class is "currency"
func DPWithAssetClass(aclass string) GetDepositsStatusPaginatedOption {
	return func(payload url.Values) {
		payload.Add("aclass", aclass)
	}
}

// For *KrakenClient method GetWithdrawalMethods()
type GetWithdrawalMethodsOption func(payload url.Values)

// Filter methods for specific asset. Defaults to no filter if function is not
// called
func WMWithAsset(asset string) GetWithdrawalMethodsOption {
	return func(payload url.Values) {
		payload.Add("asset", asset)
	}
}

// Filter methods for specific network. Defaults to no filter if function is not
// called
func WMWithNetwork(network string) GetWithdrawalMethodsOption {
	return func(payload url.Values) {
		payload.Add("network", network)
	}
}

// Filter asset class being withdrawn. Defaults to "currency" if function is
// not called. As of 1/7/24, only known valid asset class is "currency"
func WMWithAssetClass(aclass string) GetWithdrawalMethodsOption {
	return func(payload url.Values) {
		payload.Add("aclass", aclass)
	}
}

// For *KrakenClient method GetWithdrawalAddresses()
type GetWithdrawalAddressesOption func(payload url.Values)

// Filter addresses for specific asset
func WAWithAsset(asset string) GetWithdrawalAddressesOption {
	return func(payload url.Values) {
		payload.Add("asset", asset)
	}
}

// Filter addresses for specific method
func WAWithMethod(method string) GetWithdrawalAddressesOption {
	return func(payload url.Values) {
		payload.Add("method", method)
	}
}

// Find address for by withdrawal key name, as set up on your account
func WAWithKey(key string) GetWithdrawalAddressesOption {
	return func(payload url.Values) {
		payload.Add("key", key)
	}
}

// Filter by verification status of the withdrawal address. Withdrawal addresses
// successfully completing email confirmation will have a verification status of
// true.
func WAWithVerified(verified bool) GetWithdrawalAddressesOption {
	return func(payload url.Values) {
		payload.Add("verified", fmt.Sprintf("%v", verified))
	}
}

// Filter asset class being withdrawn. Defaults to "currency" if function is
// not called. As of 1/7/24, only known valid asset class is "currency"
func WAWithAssetClass(aclass string) GetWithdrawalAddressesOption {
	return func(payload url.Values) {
		payload.Add("aclass", aclass)
	}
}

// For *KrakenClient method WithdrawFunds()
type WithdrawFundsOption func(payload url.Values)

// Optional, crypto address that can be used to confirm address matches key
// (will return Invalid withdrawal address error if different)
func WFWithAddress(address string) WithdrawFundsOption {
	return func(payload url.Values) {
		payload.Add("address", address)
	}
}

// Optional, if the processed withdrawal fee is higher than max_fee, withdrawal
// will fail with EFunding:Max fee exceeded
func WFWithMaxFee(maxFee string) WithdrawFundsOption {
	return func(payload url.Values) {
		payload.Add("max_fee", maxFee)
	}
}

// For *KrakenClient method GetWithdrawalsStatus()
type GetWithdrawalsStatusOption func(payload url.Values)

// Filter for specific asset being withdrawn
func WSWithAsset(asset string) GetWithdrawalsStatusOption {
	return func(payload url.Values) {
		payload.Add("asset", asset)
	}
}

// Filter for specific name of withdrawal method
func WSWithMethod(method string) GetWithdrawalsStatusOption {
	return func(payload url.Values) {
		payload.Add("method", method)
	}
}

// Start timestamp, withdrawals created strictly before will not be included in
// the response
func WSWithStart(start string) GetWithdrawalsStatusOption {
	return func(payload url.Values) {
		payload.Add("start", start)
	}
}

// End timestamp, withdrawals created strictly after will be not be included in
// the response
func WSWithEnd(end string) GetWithdrawalsStatusOption {
	return func(payload url.Values) {
		payload.Add("end", end)
	}
}

// Filter asset class being withdrawn. Defaults to "currency" if function is
// not called. As of 1/7/24, only known valid asset class is "currency"
func WSWithAssetClass(aclass string) GetWithdrawalsStatusOption {
	return func(payload url.Values) {
		payload.Add("aclass", aclass)
	}
}

// For *KrakenClient method GetWithdrawalsStatusPaginated()
type GetWithdrawalsStatusPaginatedOption func(payload url.Values)

// Filter for specific asset being withdrawn
func WPWithAsset(asset string) GetWithdrawalsStatusPaginatedOption {
	return func(payload url.Values) {
		payload.Add("asset", asset)
	}
}

// Filter for specific name of withdrawal method
func WPWithMethod(method string) GetWithdrawalsStatusPaginatedOption {
	return func(payload url.Values) {
		payload.Add("method", method)
	}
}

// Start timestamp, withdrawals created strictly before will not be included in
// the response
func WPWithStart(start string) GetWithdrawalsStatusPaginatedOption {
	return func(payload url.Values) {
		payload.Add("start", start)
	}
}

// End timestamp, withdrawals created strictly after will be not be included in
// the response
func WPWithEnd(end string) GetWithdrawalsStatusPaginatedOption {
	return func(payload url.Values) {
		payload.Add("end", end)
	}
}

// Number of results to include per page. Defaults to 500 if function is not called
func WPWithLimit(limit int) GetWithdrawalsStatusPaginatedOption {
	return func(payload url.Values) {
		payload.Add("limit", fmt.Sprintf("%v", limit))
	}
}

// Filter asset class being withdrawn. Defaults to "currency" if function is
// not called. As of 1/7/24, only known valid asset class is "currency"
func WPWithAssetClass(aclass string) GetWithdrawalsStatusPaginatedOption {
	return func(payload url.Values) {
		payload.Add("aclass", aclass)
	}
}

// #endregion

// #region Private Earn endpoints functional options

type GetEarnStrategiesOption func(payload url.Values)

// Sorts strategies by ascending. Defaults to false (descending) if function is
// not called
func ESWithAscending() GetEarnStrategiesOption {
	return func(payload url.Values) {
		payload.Add("ascending", "true")
	}
}

// Filter strategies by asset name. Defaults to no filter if function not called
func ESWithAsset(asset string) GetEarnStrategiesOption {
	return func(payload url.Values) {
		payload.Add("asset", asset)
	}
}

// Sets page ID to display results. Defaults to beginning/end (depending on
// sorting set by ESWithAscending()) if function not called.
func ESWithCursor(cursor string) GetEarnStrategiesOption {
	return func(payload url.Values) {
		payload.Add("cursor", cursor)
	}
}

// Sets number of items to return per page. Note that the limit may be cap'd to
// lower value in the application code.
func ESWithLimit(limit uint16) GetEarnStrategiesOption {
	return func(payload url.Values) {
		payload.Add("limit", fmt.Sprintf("%v", limit))
	}
}

// Filters displayed strategies by lock type. Accepts array of strings for arg
// 'lockTypes' and ignores invalid values passed. Defaults to no filter if
// function not called or only invalid values passed.
//
// Enum - 'lockTypes': "flex", "bonded", "timed", "instant"
func ESWithLockType(lockTypes []string) GetEarnStrategiesOption {
	return func(payload url.Values) {
		validLockTypes := map[string]bool{
			"flex":    true,
			"bonded":  true,
			"timed":   true,
			"instant": true,
		}
		for _, lockType := range lockTypes {
			if validLockTypes[lockType] {
				payload.Add("lock_type[]", lockType)
			}
		}
	}
}

type GetEarnAllocationsOption func(payload url.Values)

// Sorts strategies by ascending. Defaults to false (descending) if function is
// not called
func EAWithAscending() GetEarnAllocationsOption {
	return func(payload url.Values) {
		payload.Add("ascending", "true")
	}
}

// A secondary currency to express the value of your allocations. Defaults
// to express value in USD if function is not called
func EAWithConvertedAsset(asset string) GetEarnAllocationsOption {
	return func(payload url.Values) {
		payload.Add("converted_asset", asset)
	}
}

// Omit entries for strategies that were used in the past but now they don't
// hold any allocation. Defaults to false (don't omit) if function is not called
func EAWithHideZeroAllocations() GetEarnAllocationsOption {
	return func(payload url.Values) {
		payload.Add("hide_zero_allocations", "true")
	}
}

// #endregion

// #region Private WebSocket endpoint functional options

type SubscribeOwnTradesOption func(buffer *bytes.Buffer)

// Whether to consolidate order fills by root taker trade(s). If false, all
// order fills will show separately. Defaults to true if not called.
func WithoutConsolidatedTaker() SubscribeOwnTradesOption {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(`, "consolidate_taker": false`)
	}
}

// Whether to send historical feed data snapshot upon subscription. Defaults to
// true if not called.
func WithoutSnapshot() SubscribeOwnTradesOption {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(`, "snapshot": false`)
	}
}

type SubscribeOpenOrdersOption func(buffer *bytes.Buffer)

// Whether to send rate-limit counter in updates  Defaults to false if not called.
func WithRateCounter() SubscribeOpenOrdersOption {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(`, "ratecounter": true`)
	}
}

// #endregion

// #region Private WebSocket Trading methods functional options

type WSOrderType func(buffer *bytes.Buffer)

// Instantly market orders in at best current prices
func WSMarket() WSOrderType {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(`, "ordertype": "market"`)
	}
}

// Order type of "limit" where arg 'price' is the level at which the limit order
// will be placed.
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func WSLimit(price string) WSOrderType {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(fmt.Sprintf(`, "ordertype": "limit", "price": "%s"`, price))
	}
}

// Order type of "stop-loss" order type where arg 'price' is the stop loss
// trigger price
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func WSStopLoss(price string) WSOrderType {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(fmt.Sprintf(`, "ordertype": "stop-loss", "price": "%s"`, price))
	}
}

// Order type of "take-profit" where arg 'price' is the take profit trigger price.
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func WSTakeProfit(price string) WSOrderType {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(fmt.Sprintf(`, "ordertype": "take-profit", "price": "%s"`, price))
	}
}

// Order type of "stop-loss-limit" where arg 'price' is the stop loss trigger
// price and arg 'price2' is the limit order that will be placed.
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func WSStopLossLimit(price, price2 string) WSOrderType {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(fmt.Sprintf(`, "ordertype": "stop-loss-limit", "price": "%s", "price2": "%s"`, price, price2))
	}
}

// Order type of "take-profit-limit" where arg 'price' is the take profit trigger
// price and arg 'price2' is the limit order that will be placed.
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func WSTakeProfitLimit(price, price2 string) WSOrderType {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(fmt.Sprintf(`, "ordertype": "take-profit-limit", "price": "%s", "price2": "%s"`, price, price2))
	}
}

// Order type of "trailing-stop" where arg 'price' is the relative stop trigger
// price.
//
// Note: Required arg 'price' must use a relative price for this field, namely
// the + prefix, from which the direction will be automatic based on if the
// original order is a buy or sell (no need to use - or #). The % suffix also
// works for these order types to use a relative percentage price.
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func WSTrailingStop(price string) WSOrderType {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(fmt.Sprintf(`, "ordertype": "trailing-stop", "price": "%s"`, price))
	}
}

// Order type of "trailing-stop-limit" where arg 'price' is the relative stop
// trigger price and arg 'price2' is the limit order that will be placed.
//
// Note: Required arg 'price' must use a relative price for this field, namely
// the + prefix, from which the direction will be automatic based on if the
// original order is a buy or sell (no need to use - or #). The % suffix also
// works for these order types to use a relative percentage price.
//
// Note: In practice, the system will accept either relative or specific (without +
// or -) for arg 'price2' despite what the docs say. The note is included here
// just in case; Kraken API docs: Must use a relative price for this field, namely
// one of the + or - prefixes. This will provide the offset from the trigger price
// to the limit price, i.e. +0 would set the limit price equal to the trigger price.
// The % suffix also works for this field to use a relative percentage limit price.
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func WSTrailingStopLimit(price, price2 string) WSOrderType {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(fmt.Sprintf(`, "ordertype": "trailing-stop-limit", "price": "%s", "price2": "%s"`, price, price2))
	}
}

// Order type of "settle-position". Settles any open margin position of same
// 'direction' and 'pair' by amount 'volume'
//
// Note: AddOrder() arg 'volume' can be set to "0" for closing margin orders to
// automatically fill the requisite quantity
//
// Note: Required arg 'leverage' is a required parameter by the API, but in
// practice can be any valid value. Value of 'leverage' here has no effect as
// leverage is already set by the opened position.
func WSSettlePosition(leverage string) WSOrderType {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(fmt.Sprintf(`, "ordertype": "settle-position", "leverage": %s`, leverage))
	}
}

type WSAddOrderOption func(buffer *bytes.Buffer)

// User reference id 'userref' is an optional user-specified integer id that
// can be associated with any number of orders. Many clients choose a userref
// corresponding to a unique integer id generated by their systems (e.g. a
// timestamp). However, because we don't enforce uniqueness on our side, it
// can also be used to easily group orders by pair, side, strategy, etc. This
// allows clients to more readily cancel or query information about orders in
// a particular group, with fewer API calls by using userref instead of our
// txid, where supported.
func WSUserRef(userRef string) WSAddOrderOption {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(fmt.Sprintf(`, "userref": "%s"`, userRef))
	}
}

// Amount of leverage desired. Defaults to no leverage if function is not called.
// API accepts string of any number; in practice, must be some integer >= 2
//
// Note: This function should not be used when calling AddOrder() with the
// SettlePosition() 'orderType'
func WSLeverage(leverage string) WSAddOrderOption {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(fmt.Sprintf(`, "leverage": %s`, leverage))
	}
}

// If true, order will only reduce a currently open position, not increase it
// or open a new position. Defaults to false if not passed.
//
// Note: ReduceOnly() is only usable with leveraged orders. This includes orders
// of 'orderType' SettlePosition() and orders with Leverage() passed to 'options'
func WSReduceOnly() WSAddOrderOption {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(`, "reduce_only": true`)
	}
}

// Add all desired order 'flags' as a single comma-delimited list. Use either
// this function or call (one or many) the individual flag functions below.
//
// Note: "post" is only available for WSLimit() type orders. "nompp" is only
// available for WSMarket() type orders
//
// CAUTION: "fciq" and "fcib" are mutually exclusive, only call one or neither
// at a time
//
// CAUTION: Do not pass this function with any of WSPostOnly(), WSFCIB(), WSFCIQ(),
// WSNOMPP(), WSVIQC()
//
// # Enums:
//
// 'flags': "post", "fciq", "fcib", "nompp", ~"viqc"~
//
// # Example Usage:
//
//	kc.WSAddOrder(WSLimit("42100.2"), "XBT/USD", "1.0", WSOrderFlags("post,fciq"))
func WSOrderFlags(flags string) WSAddOrderOption {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(fmt.Sprintf(`, "oflags": "%s"`, flags))
	}
}

// Post-only order (available when ordertype = limit)
func WSPostOnly() WSAddOrderOption {
	return func(buffer *bytes.Buffer) {
		bufferAddFlag(buffer, "post")
	}
}

// Prefer fee in base currency (default if selling)
//
// CAUTION: Mutually exclusive with FCIQ(). Only call one of these two at a time
func WSFCIB() WSAddOrderOption {
	return func(buffer *bytes.Buffer) {
		bufferAddFlag(buffer, "fcib")
	}
}

// Prefer fee in quote currency (default if buying)
//
// CAUTION: Mutually exclusive with FCIB(). Only call one of these two at a time
func WSFCIQ() WSAddOrderOption {
	return func(buffer *bytes.Buffer) {
		bufferAddFlag(buffer, "fciq")
	}
}

// Disables market price protection for market orders
func WSNOMPP() WSAddOrderOption {
	return func(buffer *bytes.Buffer) {
		bufferAddFlag(buffer, "nompp")
	}
}

// Order volume expressed in quote currency. This is supported only for market
// orders.
//
// Note: Not currently available
func WSVIQC() WSAddOrderOption {
	return func(buffer *bytes.Buffer) {
		bufferAddFlag(buffer, "viqc")
	}
}

// Inserts comma delimited flag string to 'oflags' value or writes new 'oflags'
// if it doesn't exist
func bufferAddFlag(buffer *bytes.Buffer, flag string) {
	bufferStr := buffer.String()
	index := strings.Index(bufferStr, "oflags")
	if index == -1 { // oflags doesnt exist
		buffer.WriteString(fmt.Sprintf(`, "oflags": "%s"`, flag))
	} else { // oflags exists
		for i := 0; i < 3; i++ {
			index = strings.Index(bufferStr[index+1:], `"`) + index + 1
		}
		newBufferStr := bufferStr[:index] + "," + flag + bufferStr[index:]
		buffer.Reset()
		buffer.WriteString(newBufferStr)
	}
}

// Time-in-force of the order to specify how long it should remain in the order
// book before being cancelled. Overrides default value with "IOC" (Immediate Or
// Cancel). IOC will immediately execute the amount possible and cancel any
// remaining balance rather than resting in the book. Defaults to "GTC" (Good
// 'Til Canceled) if function is not called.
//
// CAUTION: Mutually exclusive with GoodTilDate(). Only one of these two functions
// should be called, if any. Order will still pass as valid but one "timeinforce"
// value will be overridden.
func WSImmediateOrCancel() WSAddOrderOption {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(`, "timeinforce": "IOC"`)
	}
}

// Time-in-force of the order to specify how long it should remain in the order
// book before being cancelled. Overrides default value with "GTD" (Good Til Date).
// GTD, if called, will cause the order to expire at specified unix time passed
// to arg 'expireTime'. Expiration time, can be specified as an absolute timestamp
// or as a number of seconds in the future:
//
// 0 no expiration (default)
//
// <n> = unix timestamp of expiration time
//
// +<n> = expire <n> seconds from now, minimum 5 seconds
//
// Note: URL encoding of the + character changes it to a space, so please use
// %2b followed by the number of seconds instead of +
//
// CAUTION: Mutually exclusive with ImmediateOrCancel(). Only one of these two functions
// should be called, if any. Order will still pass as valid but one "timeinforce"
// value will be overridden.
func WSGoodTilDate(expireTime string) WSAddOrderOption {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(fmt.Sprintf(`, "timeinforce": "GTD", "expiretm": "%s"`, expireTime))
	}
}

// Conditional close of "limit" order type where arg 'price' is the level at which
// the limit order will be placed.
//
// CAUTION: Mutually exclusive with other conditional close order functions.
// Only one conditional close order function with format WSClose<orderType>()
// should be passed at a time. If more than one are passed, it will override the
// previous function calls.
//
// Conditional Close Orders: Orders that are triggered by execution of the
// primary order in the same quantity and opposite direction, but once triggered
// are independent orders that may reduce or increase net position.
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func WSCloseLimit(price string) WSAddOrderOption {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(fmt.Sprintf(`, "close[ordertype]": "limit", "close[price]": "%s"`, price))
	}
}

// Conditional close of "stop-loss" order type where arg 'price' is the stop
// loss trigger price.
//
// CAUTION: Mutually exclusive with other conditional close order functions.
// Only one conditional close order function with format WSClose<orderType>()
// should be passed at a time. If more than one are passed, it will override the
// previous function calls.
//
// Conditional Close Orders: Orders that are triggered by execution of the
// primary order in the same quantity and opposite direction, but once triggered
// are independent orders that may reduce or increase net position
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func WSCloseStopLoss(price string) WSAddOrderOption {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(fmt.Sprintf(`, "close[ordertype]": "stop-loss", "close[price]": "%s"`, price))
	}
}

// Conditional close of "take-profit" order type where arg 'price' is the take
// profit trigger price.
//
// CAUTION: Mutually exclusive with other conditional close order functions.
// Only one conditional close order function with format WSClose<orderType>()
// should be passed at a time. If more than one are passed, it will override the
// previous function calls.
//
// Conditional Close Orders: Orders that are triggered by execution of the
// primary order in the same quantity and opposite direction, but once triggered
// are independent orders that may reduce or increase net position
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func WSCloseTakeProfit(price string) WSAddOrderOption {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(fmt.Sprintf(`, "close[ordertype]": "take-profit", "close[price]": "%s"`, price))
	}
}

// Conditional close of "stop-loss-limit" order type where arg 'price' is the
// stop loss trigger price and arg 'price2' is the limit order that will be placed.
//
// CAUTION: Mutually exclusive with other conditional close order functions.
// Only one conditional close order function with format WSClose<orderType>()
// should be passed at a time. If more than one are passed, it will override the
// previous function calls.
//
// Conditional Close Orders: Orders that are triggered by execution of the
// primary order in the same quantity and opposite direction, but once triggered
// are independent orders that may reduce or increase net position
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func WSCloseStopLossLimit(price, price2 string) WSAddOrderOption {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(fmt.Sprintf(`, "close[ordertype]": "stop-loss-limit", "close[price]": "%s", "close[price2]": "%s"`, price, price2))
	}
}

// Conditional close of "take-profit-limit" order type where arg 'price' is the
// take profit trigger price and arg 'price2' is the limit order that will be
// placed.
//
// CAUTION: Mutually exclusive with other conditional close order functions.
// Only one conditional close order function with format WSClose<orderType>()
// should be passed at a time. If more than one are passed, it will override the
// previous function calls.
//
// Conditional Close Orders: Orders that are triggered by execution of the
// primary order in the same quantity and opposite direction, but once triggered
// are independent orders that may reduce or increase net position
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func WSCloseTakeProfitLimit(price, price2 string) WSAddOrderOption {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(fmt.Sprintf(`, "close[ordertype]": "take-profit-limit", "close[price]": "%s", "close[price2]": "%s"`, price, price2))
	}
}

// Conditional close of "trailing-stop" order type where arg 'price' is the relative
// stop trigger price.
//
// CAUTION: Mutually exclusive with other conditional close order functions.
// Only one conditional close order function with format WSClose<orderType>()
// should be passed at a time. If more than one are passed, it will override the
// previous function calls.
//
// Conditional Close Orders: Orders that are triggered by execution of the
// primary order in the same quantity and opposite direction, but once triggered
// are independent orders that may reduce or increase net position
//
// Note: Required arg 'price' must use a relative price for this field, namely
// the + prefix, from which the direction will be automatic based on if the
// original order is a buy or sell (no need to use - or #). The % suffix also
// works for these order types to use a relative percentage price.
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func WSCloseTrailingStop(price string) WSAddOrderOption {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(fmt.Sprintf(`, "close[ordertype]": "trailing-stop", "close[price]": "%s"`, price))
	}
}

// Conditional close of "trailing-stop-limit" order type where arg 'price' is the
// relative stop trigger price and arg 'price2' is the limit order that will be
// placed.
//
// CAUTION: Mutually exclusive with other conditional close order functions.
// Only one conditional close order function with format WSClose<orderType>()
// should be passed at a time. If more than one are passed, it will override the
// previous function calls.
//
// Conditional Close Orders: Orders that are triggered by execution of the
// primary order in the same quantity and opposite direction, but once triggered
// are independent orders that may reduce or increase net position
//
// Note: Required arg 'price' must use a relative price for this field, namely
// the + prefix, from which the direction will be automatic based on if the
// original order is a buy or sell (no need to use - or #). The % suffix also
// works for these order types to use a relative percentage price.
//
// Note: In practice, the system will accept either relative or specific (without +
// or -) for arg 'price2' despite what the docs say. The note is included here
// just in case; Kraken API docs: Must use a relative price for this field, namely
// one of the + or - prefixes. This will provide the offset from the trigger price
// to the limit price, i.e. +0 would set the limit price equal to the trigger price.
// The % suffix also works for this field to use a relative percentage limit price.
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
func WSCloseTrailingStopLimit(price, price2 string) WSAddOrderOption {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(fmt.Sprintf(`, "close[ordertype]": "trailing-stop-limit", "close[price]": "%s", "close[price2]": "%s"`, price, price2))
	}
}

// Pass RFC3339 timestamp (e.g. 2021-04-01T00:18:45Z) after which the matching
// engine should reject the new order request to arg 'deadline'.
//
// In presence of latency or order queueing: min now() + 2 seconds, max now() +
// 60 seconds.
//
// Example Usage:
//
// WSAddWithDeadline(time.Now().Add(time.Second*30).Format(time.RFC3339))
func WSAddWithDeadline(deadline string) WSAddOrderOption {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(fmt.Sprintf(`, "deadline": "%s"`, deadline))
	}
}

// Validates inputs only. Does not submit order. Defaults to "false" if not called.
func WSValidateAddOrder() WSAddOrderOption {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(`, "validate": "true"`)
	}
}

type WSEditOrderOption func(buffer *bytes.Buffer)

// Field "userref" is an optional user-specified integer id associated with
// edit request.
//
// Note: userref from parent order will not be retained on the new order after
// edit.
func WSNewUserRef(userRef string) WSEditOrderOption {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(fmt.Sprintf(`, "newuserref": "%s"`, userRef))
	}
}

// Updates order quantity in terms of the base asset.
func WSNewVolume(volume string) WSEditOrderOption {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(fmt.Sprintf(`, "volume": "%s"`, volume))
	}
}

// Updates limit price for "limit" orders. Updates trigger price for "stop-loss",
// "stop-loss-limit", "take-profit", "take-profit-limit", "trailing-stop" and
// "trailing-stop-limit" orders
//
// Relative Prices: Either price or price2 can (and sometimes must) be preceded
// by +, -, or # to specify the order price as an offset relative to the last
// traded price. + adds the amount to, and - subtracts the amount from the last
// traded price. # will either add or subtract the amount to the last traded price,
// depending on the direction and order type used. Prices can also be suffixed
// with a % to signify the relative amount as a percentage, rather than an absolute
// price difference.
//
// Trailing Stops: Required arg 'price' must use a relative price for this field,
// namely the + prefix, from which the direction will be automatic based on if
// the original order is a buy or sell (no need to use - or #). The % suffix
// also works for these order types to use a relative percentage price.
func WSNewPrice(price string) WSEditOrderOption {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(fmt.Sprintf(`, "price": "%s"`, price))
	}
}

// Updates limit price for "stop-loss-limit", "take-profit-limit" and
// "trailing-stop-limit" orders
//
// Trailing Stops Note: In practice, the system will accept either relative or
// specific (without + or -) for arg 'price2' despite what the docs say. The
// note is included here just in case; Kraken API docs: Must use a relative
// price for this field, namely one of the + or - prefixes. This will provide
// the offset from the trigger price to the limit price, i.e. +0 would set the
// limit price equal to the trigger price. The % suffix also works for this
// field to use a relative percentage limit price.
func WSNewPrice2(price2 string) WSEditOrderOption {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(fmt.Sprintf(`, "price2": "%s"`, price2))
	}
}

// Post-only order (available when ordertype = limit). All the flags from the
// parent order are retained except post-only. Post-only needs to be explicitly
// mentioned on every edit request.
func WSNewPostOnly() WSEditOrderOption {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(`, "oflags": "post"`)
	}
}

// Validate inputs only. Do not submit order. Defaults to false if not called.
func WSValidateEditOrder() WSEditOrderOption {
	return func(buffer *bytes.Buffer) {
		buffer.WriteString(`, "validate": "true"`)
	}
}

// #endregion
