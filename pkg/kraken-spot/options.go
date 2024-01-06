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
