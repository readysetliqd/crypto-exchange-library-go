package krakenspot

import (
	"bytes"
	"net/url"
	"testing"
)

func TestGetOpenOrdersOptions(t *testing.T) {
	payload := url.Values{}

	// Test OOWithTrades
	OOWithTrades(true)(payload)
	if payload.Get("trades") != "true" {
		t.Errorf("OOWithTrades() didn't set 'trades' correctly")
	}

	// Test OOWithUserRef
	OOWithUserRef(123)(payload)
	if payload.Get("userref") != "123" {
		t.Errorf("OOWithUserRef() didn't set 'userref' correctly")
	}
}

func TestGetClosedOrdersOptions(t *testing.T) {
	payload := url.Values{}

	// Test COWithTrades
	COWithTrades(true)(payload)
	if payload.Get("trades") != "true" {
		t.Errorf("COWithTrades() didn't set 'trades' correctly")
	}

	// Test COWithUserRef
	COWithUserRef(123)(payload)
	if payload.Get("userref") != "123" {
		t.Errorf("COWithUserRef() didn't set 'userref' correctly")
	}

	// Test COWithStart
	COWithStart(123)(payload)
	if payload.Get("start") != "123" {
		t.Errorf("COWithStart() didn't set 'start' correctly")
	}

	// Test COWithEnd
	COWithEnd(123)(payload)
	if payload.Get("end") != "123" {
		t.Errorf("COWithEnd() didn't set 'end' correctly")
	}

	// Test COWithOffset
	COWithOffset(123)(payload)
	if payload.Get("ofs") != "123" {
		t.Errorf("COWithOffset() didn't set 'ofs' correctly")
	}

	// Test COWithCloseTime with invalid arg
	COWithCloseTime("foo")(payload)
	if payload.Get("closetime") != "" {
		t.Errorf("COWithCloseTime() set 'closetime' with invalid arg")
	}

	// Test COWithCloseTime with valid arg
	COWithCloseTime("open")(payload)
	if payload.Get("closetime") != "open" {
		t.Errorf("COWithCloseTime() didn't set 'closetime' correctly")
	}

	// Test COWithConsolidateTaker
	COWithConsolidateTaker(true)(payload)
	if payload.Get("consolidate_taker") != "true" {
		t.Errorf("COWithConsolidateTaker() didn't set 'consolidate_taker' correctly")
	}
}

func TestGetOrdersInfoOptions(t *testing.T) {
	payload := url.Values{}

	// Test OIWithTrades
	OIWithTrades(true)(payload)
	if payload.Get("trades") != "true" {
		t.Errorf("OIWithTrades() didn't set 'trades' correctly")
	}

	// Test OIWithUserRef
	OIWithUserRef(123)(payload)
	if payload.Get("userref") != "123" {
		t.Errorf("OIWithUserRef() didn't set 'userref' correctly")
	}

	// Test OIWithConsolidateTaker
	OIWithConsolidateTaker(true)(payload)
	if payload.Get("consolidate_taker") != "true" {
		t.Errorf("OIWithConsolidateTaker() didn't set 'consolidate_taker' correctly")
	}

}

func TestGetTradesHistoryOptions(t *testing.T) {
	payload := url.Values{}

	// Test THWithType with valid trade type
	THWithType("all")(payload)
	if payload.Get("type") != "all" {
		t.Errorf("THWithType() didn't set 'type' correctly")
	}

	// Test THWithType with invalid trade type
	payload.Del("type")
	THWithType("invalid")(payload)
	if payload.Get("type") != "" {
		t.Errorf("THWithType() with invalid trade type should not set 'type'")
	}

	// Test THWithTrades
	THWithTrades(true)(payload)
	if payload.Get("trades") != "true" {
		t.Errorf("THWithTrades() didn't set 'trades' correctly")
	}

	// Test THWithStart
	THWithStart(123)(payload)
	if payload.Get("start") != "123" {
		t.Errorf("THWithStart() didn't set 'start' correctly")
	}

	// Test THWithEnd
	THWithEnd(123)(payload)
	if payload.Get("end") != "123" {
		t.Errorf("THWithEnd() didn't set 'end' correctly")
	}

	// Test THWithOffset
	THWithOffset(123)(payload)
	if payload.Get("ofs") != "123" {
		t.Errorf("THWithOffset() didn't set 'ofs' correctly")
	}

	// Test THWithConsolidateTaker
	THWithConsolidateTaker(true)(payload)
	if payload.Get("consolidate_taker") != "true" {
		t.Errorf("THWithConsolidateTaker() didn't set 'consolidate_taker' correctly")
	}
}

func TestGetTradeInfoOptions(t *testing.T) {
	payload := url.Values{}

	// Test TIWithTrades
	TIWithTrades(true)(payload)
	if payload.Get("trades") != "true" {
		t.Errorf("TIWithTrades() didn't set 'trades' correctly")
	}
}

func TestGetOpenPositionsOptions(t *testing.T) {
	payload := url.Values{}

	// Test OPWithTxID
	OPWithTxID("123")(payload)
	if payload.Get("txid") != "123" {
		t.Errorf("OPWithTxID() didn't set 'txid' correctly")
	}

	// Test OPWithDoCalcs
	OPWithDoCalcs(true)(payload)
	if payload.Get("docalcs") != "true" {
		t.Errorf("OPWithDoCalcs() didn't set 'docalcs' correctly")
	}
}

func TestGetOpenPositionsConsolidatedOptions(t *testing.T) {
	payload := url.Values{}

	// Test OPCWithTxID
	OPCWithTxID("123")(payload)
	if payload.Get("txid") != "123" {
		t.Errorf("OPCWithTxID() didn't set 'txid' correctly")
	}

	// Test OPCWithDoCalcs
	OPCWithDoCalcs(true)(payload)
	if payload.Get("docalcs") != "true" {
		t.Errorf("OPCWithDoCalcs() didn't set 'docalcs' correctly")
	}
}

func TestGetLedgersInfoOptions(t *testing.T) {
	payload := url.Values{}

	// Test LIWithAsset
	LIWithAsset("BTC")(payload)
	if payload.Get("asset") != "BTC" {
		t.Errorf("LIWithAsset() didn't set 'asset' correctly")
	}

	// Test LIWithAclass
	LIWithAclass("currency")(payload)
	if payload.Get("aclass") != "currency" {
		t.Errorf("LIWithAclass() didn't set 'aclass' correctly")
	}

	// Test LIWithType with valid ledger type
	LIWithType("all")(payload)
	if payload.Get("type") != "all" {
		t.Errorf("LIWithType() didn't set 'type' correctly")
	}

	// Test LIWithType with invalid ledger type
	payload.Del("type")
	LIWithType("invalid")(payload)
	if payload.Get("type") != "" {
		t.Errorf("LIWithType() with invalid ledger type should not set 'type'")
	}

	// Test LIWithStart
	LIWithStart(123)(payload)
	if payload.Get("start") != "123" {
		t.Errorf("LIWithStart() didn't set 'start' correctly")
	}

	// Test LIWithEnd
	LIWithEnd(123)(payload)
	if payload.Get("end") != "123" {
		t.Errorf("LIWithEnd() didn't set 'end' correctly")
	}

	// Test LIWithOffset
	LIWithOffset(123)(payload)
	if payload.Get("ofs") != "123" {
		t.Errorf("LIWithOffset() didn't set 'ofs' correctly")
	}

	// Test LIWithoutCount
	LIWithoutCount(true)(payload)
	if payload.Get("without_count") != "true" {
		t.Errorf("LIWithoutCount() didn't set 'without_count' correctly")
	}
}

func TestGetLedgerOptions(t *testing.T) {
	payload := url.Values{}

	// Test GLWithTrades
	GLWithTrades(true)(payload)
	if payload.Get("trades") != "true" {
		t.Errorf("GLWithTrades() didn't set 'trades' correctly")
	}
}

func TestGetTradeVolumeOptions(t *testing.T) {
	payload := url.Values{}

	// Test TVWithPair
	TVWithPair("BTCUSD")(payload)
	if payload.Get("pair") != "BTCUSD" {
		t.Errorf("TVWithPair() didn't set 'pair' correctly")
	}
}

func TestRequestTradesExportReportOptions(t *testing.T) {
	payload := url.Values{}

	// Test RTWithFormat with valid format
	RTWithFormat("CSV")(payload)
	if payload.Get("format") != "CSV" {
		t.Errorf("RTWithFormat() didn't set 'format' correctly")
	}

	// Test RTWithFormat with invalid format
	payload.Del("format")
	RTWithFormat("invalid")(payload)
	if payload.Get("format") != "" {
		t.Errorf("RTWithFormat() with invalid format should not set 'format'")
	}

	// Test RTWithFields
	RTWithFields("ordertxid,time")(payload)
	if payload.Get("fields") != "ordertxid,time" {
		t.Errorf("RTWithFields() didn't set 'fields' correctly")
	}

	// Test RTWithStart
	RTWithStart(123)(payload)
	if payload.Get("start") != "123" {
		t.Errorf("RTWithStart() didn't set 'start' correctly")
	}

	// Test RTWithEnd
	RTWithEnd(123)(payload)
	if payload.Get("end") != "123" {
		t.Errorf("RTWithEnd() didn't set 'end' correctly")
	}
}

func TestRequestLedgersExportReportOptions(t *testing.T) {
	payload := url.Values{}

	// Test RLWithFormat with valid format
	RLWithFormat("CSV")(payload)
	if payload.Get("format") != "CSV" {
		t.Errorf("RLWithFormat() didn't set 'format' correctly")
	}

	// Test RLWithFormat with invalid format
	payload.Del("format")
	RLWithFormat("invalid")(payload)
	if payload.Get("format") != "" {
		t.Errorf("RLWithFormat() with invalid format should not set 'format'")
	}

	// Test RLWithFields
	RLWithFields("refid,time")(payload)
	if payload.Get("fields") != "refid,time" {
		t.Errorf("RLWithFields() didn't set 'fields' correctly")
	}

	// Test RLWithStart
	RLWithStart(123)(payload)
	if payload.Get("start") != "123" {
		t.Errorf("RLWithStart() didn't set 'start' correctly")
	}

	// Test RLWithEnd
	RLWithEnd(123)(payload)
	if payload.Get("end") != "123" {
		t.Errorf("RLWithEnd() didn't set 'end' correctly")
	}
}

func TestOrderTypeOptions(t *testing.T) {
	payload := url.Values{}

	// Test Market
	Market()(payload)
	if payload.Get("ordertype") != "market" {
		t.Errorf("Market() didn't set 'ordertype' correctly")
	}

	// Test Limit
	payload.Del("ordertype")
	payload.Del("price")
	Limit("123.45")(payload)
	if payload.Get("ordertype") != "limit" || payload.Get("price") != "123.45" {
		t.Errorf("Limit() didn't set 'ordertype' or 'price' correctly")
	}

	// Test StopLoss
	payload.Del("ordertype")
	payload.Del("price")
	StopLoss("123.45")(payload)
	if payload.Get("ordertype") != "stop-loss" || payload.Get("price") != "123.45" {
		t.Errorf("StopLoss() didn't set 'ordertype' or 'price' correctly")
	}

	// Test TakeProfit
	payload.Del("ordertype")
	payload.Del("price")
	TakeProfit("123.45")(payload)
	if payload.Get("ordertype") != "take-profit" || payload.Get("price") != "123.45" {
		t.Errorf("TakeProfit() didn't set 'ordertype' or 'price' correctly")
	}

	// Test StopLossLimit
	payload.Del("ordertype")
	payload.Del("price")
	payload.Del("price2")
	StopLossLimit("123.45", "67.89")(payload)
	if payload.Get("ordertype") != "stop-loss-limit" || payload.Get("price") != "123.45" || payload.Get("price2") != "67.89" {
		t.Errorf("StopLossLimit() didn't set 'ordertype', 'price', or 'price2' correctly")
	}

	// Test TakeProfitLimit
	payload.Del("ordertype")
	payload.Del("price")
	payload.Del("price2")
	TakeProfitLimit("123.45", "67.89")(payload)
	if payload.Get("ordertype") != "take-profit-limit" || payload.Get("price") != "123.45" || payload.Get("price2") != "67.89" {
		t.Errorf("TakeProfitLimit() didn't set 'ordertype', 'price', or 'price2' correctly")
	}

	// Test TrailingStop
	payload.Del("ordertype")
	payload.Del("price")
	TrailingStop("123.45")(payload)
	if payload.Get("ordertype") != "trailing-stop" || payload.Get("price") != "123.45" {
		t.Errorf("TrailingStop() didn't set 'ordertype' or 'price' correctly")
	}

	// Test TrailingStopLimit
	payload.Del("ordertype")
	payload.Del("price")
	payload.Del("price2")
	TrailingStopLimit("123.45", "67.89")(payload)
	if payload.Get("ordertype") != "trailing-stop-limit" || payload.Get("price") != "123.45" || payload.Get("price2") != "67.89" {
		t.Errorf("TrailingStopLimit() didn't set 'ordertype', 'price', or 'price2' correctly")
	}

	// Test SettlePosition
	payload.Del("ordertype")
	SettlePosition("3")(payload)
	if payload.Get("ordertype") != "settle-position" || payload.Get("leverage") != "3" {
		t.Errorf("SettlePosition() didn't set 'ordertype' or 'leverage' correctly")
	}
}

func TestAddOrderOptions(t *testing.T) {
	payload := url.Values{}

	// Test UserRef
	UserRef("123")(payload)
	if payload.Get("userref") != "123" {
		t.Errorf("UserRef() didn't set 'userref' correctly")
	}

	// Test DisplayVolume
	DisplayVolume("123.45")(payload)
	if payload.Get("displayvol") != "123.45" {
		t.Errorf("DisplayVolume() didn't set 'displayvol' correctly")
	}

	// Test IndexTrigger
	IndexTrigger()(payload)
	if payload.Get("trigger") != "index" {
		t.Errorf("IndexTrigger() didn't set 'trigger' correctly")
	}

	// Test Leverage
	Leverage("3")(payload)
	if payload.Get("leverage") != "3" {
		t.Errorf("Leverage() didn't set 'leverage' correctly")
	}

	// Test ReduceOnly
	ReduceOnly()(payload)
	if payload.Get("reduce_only") != "true" {
		t.Errorf("ReduceOnly() didn't set 'reduce_only' correctly")
	}

	// Test STPCancelOldest
	STPCancelOldest()(payload)
	if payload.Get("stptype") != "cancel-oldest" {
		t.Errorf("STPCancelOldest() didn't set 'stptype' correctly")
	}

	// Test STPCancelBoth
	payload.Del("stptype")
	STPCancelBoth()(payload)
	if payload.Get("stptype") != "cancel-both" {
		t.Errorf("STPCancelBoth() didn't set 'stptype' correctly")
	}

	// Test PostOnly
	PostOnly()(payload)
	if payload.Get("oflags") != "post" {
		t.Errorf("PostOnly() didn't set 'oflags' correctly")
	}

	// Test FCIB
	FCIB()(payload)
	if payload.Get("oflags") != "post,fcib" {
		t.Errorf("FCIB() didn't set 'oflags' correctly")
	}

	// Test FCIQ
	FCIQ()(payload)
	if payload.Get("oflags") != "post,fcib,fciq" {
		t.Errorf("FCIQ() didn't set 'oflags' correctly")
	}

	// Test NOMPP
	NOMPP()(payload)
	if payload.Get("oflags") != "post,fcib,fciq,nompp" {
		t.Errorf("NOMPP() didn't set 'oflags' correctly")
	}

	// Test VIQC
	VIQC()(payload)
	if payload.Get("oflags") != "post,fcib,fciq,nompp,viqc" {
		t.Errorf("VIQC() didn't set 'oflags' correctly")
	}

	// Test ImmediateOrCancel
	ImmediateOrCancel()(payload)
	if payload.Get("timeinforce") != "IOC" {
		t.Errorf("ImmediateOrCancel() didn't set 'timeinforce' correctly")
	}

	// Test GoodTilDate
	payload.Del("timeinforce")
	GoodTilDate("1234567890")(payload)
	if payload.Get("timeinforce") != "GTD" || payload.Get("expiretm") != "1234567890" {
		t.Errorf("GoodTilDate() didn't set 'timeinforce' or 'expiretm' correctly")
	}

	// Test CloseLimit
	CloseLimit("123.45")(payload)
	if payload.Get("close[ordertype]") != "limit" || payload.Get("close[price]") != "123.45" {
		t.Errorf("CloseLimit() didn't set 'close[ordertype]' or 'close[price]' correctly")
	}

	// Test CloseStopLoss
	payload.Del("close[ordertype]")
	payload.Del("close[price]")
	CloseStopLoss("123.45")(payload)
	if payload.Get("close[ordertype]") != "stop-loss" || payload.Get("close[price]") != "123.45" {
		t.Errorf("CloseStopLoss() didn't set 'close[ordertype]' or 'close[price]' correctly")
	}

	// Test CloseTakeProfit
	payload.Del("close[ordertype]")
	payload.Del("close[price]")
	CloseTakeProfit("123.45")(payload)
	if payload.Get("close[ordertype]") != "take-profit" || payload.Get("close[price]") != "123.45" {
		t.Errorf("CloseTakeProfit() didn't set 'close[ordertype]' or 'close[price]' correctly")
	}

	// Test CloseStopLossLimit
	payload.Del("close[ordertype]")
	payload.Del("close[price]")
	payload.Del("close[price2]")
	CloseStopLossLimit("123.45", "67.89")(payload)
	if payload.Get("close[ordertype]") != "stop-loss-limit" || payload.Get("close[price]") != "123.45" || payload.Get("close[price2]") != "67.89" {
		t.Errorf("CloseStopLossLimit() didn't set 'close[ordertype]', 'close[price]', or 'close[price2]' correctly")
	}

	// Test CloseTakeProfitLimit
	payload.Del("close[ordertype]")
	payload.Del("close[price]")
	payload.Del("close[price2]")
	CloseTakeProfitLimit("123.45", "67.89")(payload)
	if payload.Get("close[ordertype]") != "take-profit-limit" || payload.Get("close[price]") != "123.45" || payload.Get("close[price2]") != "67.89" {
		t.Errorf("CloseTakeProfitLimit() didn't set 'close[ordertype]', 'close[price]', or 'close[price2]' correctly")
	}

	// Test CloseTrailingStop
	payload.Del("close[ordertype]")
	payload.Del("close[price]")
	CloseTrailingStop("123.45")(payload)
	if payload.Get("close[ordertype]") != "trailing-stop" || payload.Get("close[price]") != "123.45" {
		t.Errorf("CloseTrailingStop() didn't set 'close[ordertype]' or 'close[price]' correctly")
	}

	// Test CloseTrailingStopLimit
	payload.Del("close[ordertype]")
	payload.Del("close[price]")
	payload.Del("close[price2]")
	CloseTrailingStopLimit("123.45", "67.89")(payload)
	if payload.Get("close[ordertype]") != "trailing-stop-limit" || payload.Get("close[price]") != "123.45" || payload.Get("close[price2]") != "67.89" {
		t.Errorf("CloseTrailingStopLimit() didn't set 'close[ordertype]', 'close[price]', or 'close[price2]' correctly")
	}

	// Test AddWithDeadline
	AddWithDeadline("1234567890")(payload)
	if payload.Get("deadline") != "1234567890" {
		t.Errorf("AddWithDeadline() didn't set 'deadline' correctly")
	}

	// Test ValidateAddOrder
	ValidateAddOrder()(payload)
	if payload.Get("validate") != "true" {
		t.Errorf("ValidateAddOrder() didn't set 'validate' correctly")
	}
}

func TestAddOrderBatchOptions(t *testing.T) {
	payload := url.Values{}

	// Test AddBatchWithDeadline
	AddBatchWithDeadline("1234567890")(payload)
	if payload.Get("deadline") != "1234567890" {
		t.Errorf("AddBatchWithDeadline() didn't set 'deadline' correctly")
	}

	// Test ValidateAddOrderBatch
	ValidateAddOrderBatch()(payload)
	if payload.Get("validate") != "true" {
		t.Errorf("ValidateAddOrderBatch() didn't set 'validate' correctly")
	}
}

func TestEditOrderOptions(t *testing.T) {
	payload := url.Values{}

	// Test NewUserRef
	NewUserRef("123")(payload)
	if payload.Get("userref") != "123" {
		t.Errorf("NewUserRef() didn't set 'userref' correctly")
	}

	// Test NewVolume
	NewVolume("123.45")(payload)
	if payload.Get("volume") != "123.45" {
		t.Errorf("NewVolume() didn't set 'volume' correctly")
	}

	// Test NewDisplayVolume
	NewDisplayVolume("123.45")(payload)
	if payload.Get("displayvol") != "123.45" {
		t.Errorf("NewDisplayVolume() didn't set 'displayvol' correctly")
	}

	// Test NewPrice
	NewPrice("123.45")(payload)
	if payload.Get("price") != "123.45" {
		t.Errorf("NewPrice() didn't set 'price' correctly")
	}

	// Test NewPrice2
	NewPrice2("67.89")(payload)
	if payload.Get("price2") != "67.89" {
		t.Errorf("NewPrice2() didn't set 'price2' correctly")
	}

	// Test NewPostOnly
	NewPostOnly()(payload)
	if payload.Get("oflags") != "post" {
		t.Errorf("NewPostOnly() didn't set 'oflags' correctly")
	}

	// Test NewDeadline
	NewDeadline("1234567890")(payload)
	if payload.Get("deadline") != "1234567890" {
		t.Errorf("NewDeadline() didn't set 'deadline' correctly")
	}

	// Test NewCancelResponse
	NewCancelResponse()(payload)
	if payload.Get("cancel_response") != "true" {
		t.Errorf("NewCancelResponse() didn't set 'cancel_response' correctly")
	}

	// Test ValidateEditOrder
	ValidateEditOrder()(payload)
	if payload.Get("validate") != "true" {
		t.Errorf("ValidateEditOrder() didn't set 'validate' correctly")
	}
}

func TestGetDepositMethodsOptions(t *testing.T) {
	payload := url.Values{}

	// Test DMWithAssetClass
	DMWithAssetClass("currency")(payload)
	if payload.Get("aclass") != "currency" {
		t.Errorf("DMWithAssetClass() didn't set 'aclass' correctly")
	}
}

func TestGetDepositAddressesOptions(t *testing.T) {
	payload := url.Values{}

	// Test DAWithNew
	DAWithNew()(payload)
	if payload.Get("new") != "true" {
		t.Errorf("DAWithNew() didn't set 'new' correctly")
	}

	// Test DAWithAmount
	DAWithAmount("123.45")(payload)
	if payload.Get("amount") != "123.45" {
		t.Errorf("DAWithAmount() didn't set 'amount' correctly")
	}
}

func TestGetDepositsStatusOptions(t *testing.T) {
	payload := url.Values{}

	// Test DSWithAsset
	DSWithAsset("BTC")(payload)
	if payload.Get("asset") != "BTC" {
		t.Errorf("DSWithAsset() didn't set 'asset' correctly")
	}

	// Test DSWithMethod
	DSWithMethod("SEPA")(payload)
	if payload.Get("method") != "SEPA" {
		t.Errorf("DSWithMethod() didn't set 'method' correctly")
	}

	// Test DSWithStart
	DSWithStart("1234567890")(payload)
	if payload.Get("start") != "1234567890" {
		t.Errorf("DSWithStart() didn't set 'start' correctly")
	}

	// Test DSWithEnd
	DSWithEnd("1234567890")(payload)
	if payload.Get("end") != "1234567890" {
		t.Errorf("DSWithEnd() didn't set 'end' correctly")
	}

	// Test DSWithLimit
	DSWithLimit(100)(payload)
	if payload.Get("limit") != "100" {
		t.Errorf("DSWithLimit() didn't set 'limit' correctly")
	}

	// Test DSWithAssetClass
	DSWithAssetClass("currency")(payload)
	if payload.Get("aclass") != "currency" {
		t.Errorf("DSWithAssetClass() didn't set 'aclass' correctly")
	}
}

func TestGetDepositsStatusPaginatedOptions(t *testing.T) {
	payload := url.Values{}

	// Test DPWithAsset
	DPWithAsset("BTC")(payload)
	if payload.Get("asset") != "BTC" {
		t.Errorf("DPWithAsset() didn't set 'asset' correctly")
	}

	// Test DPWithMethod
	DPWithMethod("SEPA")(payload)
	if payload.Get("method") != "SEPA" {
		t.Errorf("DPWithMethod() didn't set 'method' correctly")
	}

	// Test DPWithStart
	DPWithStart("1234567890")(payload)
	if payload.Get("start") != "1234567890" {
		t.Errorf("DPWithStart() didn't set 'start' correctly")
	}

	// Test DPWithEnd
	DPWithEnd("1234567890")(payload)
	if payload.Get("end") != "1234567890" {
		t.Errorf("DPWithEnd() didn't set 'end' correctly")
	}

	// Test DPWithLimit
	DPWithLimit(100)(payload)
	if payload.Get("limit") != "100" {
		t.Errorf("DPWithLimit() didn't set 'limit' correctly")
	}

	// Test DPWithAssetClass
	DPWithAssetClass("currency")(payload)
	if payload.Get("aclass") != "currency" {
		t.Errorf("DPWithAssetClass() didn't set 'aclass' correctly")
	}
}

func TestGetWithdrawalMethodsOptions(t *testing.T) {
	payload := url.Values{}

	// Test WMWithAsset
	WMWithAsset("BTC")(payload)
	if payload.Get("asset") != "BTC" {
		t.Errorf("WMWithAsset() didn't set 'asset' correctly")
	}

	// Test WMWithNetwork
	WMWithNetwork("Bitcoin")(payload)
	if payload.Get("network") != "Bitcoin" {
		t.Errorf("WMWithNetwork() didn't set 'network' correctly")
	}

	// Test WMWithAssetClass
	WMWithAssetClass("currency")(payload)
	if payload.Get("aclass") != "currency" {
		t.Errorf("WMWithAssetClass() didn't set 'aclass' correctly")
	}
}

func TestGetWithdrawalAddressesOptions(t *testing.T) {
	payload := url.Values{}

	// Test WAWithAsset
	WAWithAsset("BTC")(payload)
	if payload.Get("asset") != "BTC" {
		t.Errorf("WAWithAsset() didn't set 'asset' correctly")
	}

	// Test WAWithMethod
	WAWithMethod("withdraw")(payload)
	if payload.Get("method") != "withdraw" {
		t.Errorf("WAWithMethod() didn't set 'method' correctly")
	}

	// Test WAWithKey
	WAWithKey("key123")(payload)
	if payload.Get("key") != "key123" {
		t.Errorf("WAWithKey() didn't set 'key' correctly")
	}

	// Test WAWithVerified
	WAWithVerified(true)(payload)
	if payload.Get("verified") != "true" {
		t.Errorf("WAWithVerified() didn't set 'verified' correctly")
	}

	// Test WAWithAssetClass
	WAWithAssetClass("currency")(payload)
	if payload.Get("aclass") != "currency" {
		t.Errorf("WAWithAssetClass() didn't set 'aclass' correctly")
	}
}

func TestWithdrawFundsOptions(t *testing.T) {
	payload := url.Values{}

	// Test WFWithAddress
	WFWithAddress("1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa")(payload)
	if payload.Get("address") != "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa" {
		t.Errorf("WFWithAddress() didn't set 'address' correctly")
	}

	// Test WFWithMaxFee
	WFWithMaxFee("0.001")(payload)
	if payload.Get("max_fee") != "0.001" {
		t.Errorf("WFWithMaxFee() didn't set 'max_fee' correctly")
	}
}

func TestGetWithdrawalsStatusOptions(t *testing.T) {
	payload := url.Values{}

	// Test WSWithAsset
	WSWithAsset("BTC")(payload)
	if payload.Get("asset") != "BTC" {
		t.Errorf("WSWithAsset() didn't set 'asset' correctly")
	}

	// Test WSWithMethod
	WSWithMethod("withdraw")(payload)
	if payload.Get("method") != "withdraw" {
		t.Errorf("WSWithMethod() didn't set 'method' correctly")
	}

	// Test WSWithStart
	WSWithStart("1642099200")(payload)
	if payload.Get("start") != "1642099200" {
		t.Errorf("WSWithStart() didn't set 'start' correctly")
	}

	// Test WSWithEnd
	WSWithEnd("1642185600")(payload)
	if payload.Get("end") != "1642185600" {
		t.Errorf("WSWithEnd() didn't set 'end' correctly")
	}

	// Test WSWithAssetClass
	WSWithAssetClass("currency")(payload)
	if payload.Get("aclass") != "currency" {
		t.Errorf("WSWithAssetClass() didn't set 'aclass' correctly")
	}
}

func TestGetWithdrawalsStatusPaginatedOptions(t *testing.T) {
	payload := url.Values{}

	// Test WPWithAsset
	WPWithAsset("BTC")(payload)
	if payload.Get("asset") != "BTC" {
		t.Errorf("WPWithAsset() didn't set 'asset' correctly")
	}

	// Test WPWithMethod
	WPWithMethod("withdraw")(payload)
	if payload.Get("method") != "withdraw" {
		t.Errorf("WPWithMethod() didn't set 'method' correctly")
	}

	// Test WPWithStart
	WPWithStart("1642099200")(payload)
	if payload.Get("start") != "1642099200" {
		t.Errorf("WPWithStart() didn't set 'start' correctly")
	}

	// Test WPWithEnd
	WPWithEnd("1642185600")(payload)
	if payload.Get("end") != "1642185600" {
		t.Errorf("WPWithEnd() didn't set 'end' correctly")
	}

	// Test WPWithLimit
	WPWithLimit(100)(payload)
	if payload.Get("limit") != "100" {
		t.Errorf("WPWithLimit() didn't set 'limit' correctly")
	}

	// Test WPWithAssetClass
	WPWithAssetClass("currency")(payload)
	if payload.Get("aclass") != "currency" {
		t.Errorf("WPWithAssetClass() didn't set 'aclass' correctly")
	}
}

func TestGetEarnStrategiesOptions(t *testing.T) {
	payload := url.Values{}

	// Test ESWithAscending
	ESWithAscending()(payload)
	if payload.Get("ascending") != "true" {
		t.Errorf("ESWithAscending() didn't set 'ascending' correctly")
	}

	// Test ESWithAsset
	ESWithAsset("BTC")(payload)
	if payload.Get("asset") != "BTC" {
		t.Errorf("ESWithAsset() didn't set 'asset' correctly")
	}

	// Test ESWithCursor
	ESWithCursor("cursor123")(payload)
	if payload.Get("cursor") != "cursor123" {
		t.Errorf("ESWithCursor() didn't set 'cursor' correctly")
	}

	// Test ESWithLimit
	ESWithLimit(100)(payload)
	if payload.Get("limit") != "100" {
		t.Errorf("ESWithLimit() didn't set 'limit' correctly")
	}

	// Test ESWithLockType with valid lock types
	ESWithLockType([]string{"flex", "bonded"})(payload)
	lockTypes, ok := payload["lock_type[]"]
	if !ok {
		t.Errorf("ESWithLockType() didn't set 'lock_type[]'")
	}

	expectedLockTypes := []string{"flex", "bonded"}
	for _, expected := range expectedLockTypes {
		found := false
		for _, lockType := range lockTypes {
			if lockType == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ESWithLockType() didn't set 'lock_type[]' to include '%s'", expected)
		}
	}

	// Test ESWithLockType with an invalid lock type
	payload = url.Values{} // reset payload
	ESWithLockType([]string{"invalid"})(payload)
	if _, ok := payload["lock_type[]"]; ok {
		t.Errorf("ESWithLockType() incorrectly set 'lock_type[]' for an invalid lock type")
	}
}

func TestGetEarnAllocationsOptions(t *testing.T) {
	payload := url.Values{}

	// Test EAWithAscending
	EAWithAscending()(payload)
	if payload.Get("ascending") != "true" {
		t.Errorf("EAWithAscending() didn't set 'ascending' correctly")
	}

	// Test EAWithConvertedAsset
	EAWithConvertedAsset("BTC")(payload)
	if payload.Get("converted_asset") != "BTC" {
		t.Errorf("EAWithConvertedAsset() didn't set 'converted_asset' correctly")
	}

	// Test EAWithHideZeroAllocations
	EAWithHideZeroAllocations()(payload)
	if payload.Get("hide_zero_allocations") != "true" {
		t.Errorf("EAWithHideZeroAllocations() didn't set 'hide_zero_allocations' correctly")
	}
}

func TestReqIDOption(t *testing.T) {
	buffer := &bytes.Buffer{}
	ReqID("1234")(buffer)
	expected := `, "reqid": 1234`
	if buffer.String() != expected {
		t.Errorf("ReqID() didn't set 'reqid' correctly, got: %s, want: %s", buffer.String(), expected)
	}
}

func TestSubscribeOwnTradesOption(t *testing.T) {
	buffer := &bytes.Buffer{}

	// Test SubscribeOwnTradesReqID
	option := SubscribeOwnTradesReqID("1234")
	option.Apply(buffer)
	expected := `, "reqid": 1234`
	if buffer.String() != expected {
		t.Errorf("SubscribeOwnTradesReqID() didn't set 'reqid' correctly, got: %s, want: %s", buffer.String(), expected)
	}
	if option.Type() != PrivateReqIDOption {
		t.Errorf("SubscribeOwnTradesReqID() didn't return correct type got: %v; want %v", option.Type(), PrivateReqIDOption)
	}

	// Test WithoutConsolidatedTaker
	buffer.Reset()
	option = WithoutConsolidatedTaker()
	option.Apply(buffer)
	expected = `, "consolidate_taker": false`
	if buffer.String() != expected {
		t.Errorf("WithoutConsolidatedTaker() didn't set 'consolidate_taker' correctly, got: %s, want: %s", buffer.String(), expected)
	}
	if option.Type() != SubscriptionOption {
		t.Errorf("WithoutConsolidatedTaker() didn't return correct type got: %v; want %v", option.Type(), SubscriptionOption)
	}

	// Test WithoutSnapshot
	buffer.Reset()
	option = WithoutSnapshot()
	option.Apply(buffer)
	expected = `, "snapshot": false`
	if buffer.String() != expected {
		t.Errorf("WithoutSnapshot() didn't set 'snapshot' correctly, got: %s, want: %s", buffer.String(), expected)
	}
	if option.Type() != SnapshotOption {
		t.Errorf("WithoutSnapshot() didn't return correct type got: %v; want %v", option.Type(), SnapshotOption)
	}
}

func TestUnsubscribeOwnTradesOption(t *testing.T) {
	buffer := &bytes.Buffer{}
	UnsubscribeOwnTradesReqID("1234")(buffer)
	expected := `, "reqid": 1234`
	if buffer.String() != expected {
		t.Errorf("UnsubscribeOwnTradesReqID() didn't set 'reqid' correctly, got: %s, want: %s", buffer.String(), expected)
	}
}

func TestSubscribeOpenOrdersOption(t *testing.T) {
	buffer := &bytes.Buffer{}

	// Test WithRateCounter
	option := WithRateCounter()
	option.Apply(buffer)
	expected := `, "ratecounter": true`
	if buffer.String() != expected {
		t.Errorf("WithRateCounter() didn't set 'ratecounter' correctly, got: %s, want: %s", buffer.String(), expected)
	}
	if option.Type() != SubscriptionOption {
		t.Errorf("WithRateCounter() didn't return correct type got: %v; want %v", option.Type(), SubscriptionOption)
	}

	// Test SubscribeOpenOrdersReqID
	buffer.Reset()
	option = SubscribeOpenOrdersReqID("1234")
	option.Apply(buffer)
	expected = `, "reqid": 1234`
	if buffer.String() != expected {
		t.Errorf("SubscribeOpenOrdersReqID() didn't set 'reqid' correctly, got: %s, want: %s", buffer.String(), expected)
	}
	if option.Type() != PrivateReqIDOption {
		t.Errorf("SubscribeOpenOrdersReqID() didn't return correct type got: %v; want %v", option.Type(), PrivateReqIDOption)
	}
}

func TestUnsubscribeOpenOrdersOption(t *testing.T) {
	buffer := &bytes.Buffer{}
	UnsubscribeOpenOrdersReqID("1234")(buffer)
	expected := `, "reqid": 1234`
	if buffer.String() != expected {
		t.Errorf("UnsubscribeOpenOrdersReqID() didn't set 'reqid' correctly, got: %s, want: %s", buffer.String(), expected)
	}
}

func TestWSOrderType(t *testing.T) {
	buffer := &bytes.Buffer{}

	// Test WSMarket
	WSMarket()(buffer)
	expected := `, "ordertype": "market"`
	if buffer.String() != expected {
		t.Errorf("WSMarket() didn't set 'ordertype' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSLimit
	buffer.Reset()
	WSLimit("1000")(buffer)
	expected = `, "ordertype": "limit", "price": "1000"`
	if buffer.String() != expected {
		t.Errorf("WSLimit() didn't set 'ordertype' and 'price' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSStopLoss
	buffer.Reset()
	WSStopLoss("1000")(buffer)
	expected = `, "ordertype": "stop-loss", "price": "1000"`
	if buffer.String() != expected {
		t.Errorf("WSStopLoss() didn't set 'ordertype' and 'price' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSTakeProfit
	buffer.Reset()
	WSTakeProfit("1000")(buffer)
	expected = `, "ordertype": "take-profit", "price": "1000"`
	if buffer.String() != expected {
		t.Errorf("WSTakeProfit() didn't set 'ordertype' and 'price' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSStopLossLimit
	buffer.Reset()
	WSStopLossLimit("1000", "2000")(buffer)
	expected = `, "ordertype": "stop-loss-limit", "price": "1000", "price2": "2000"`
	if buffer.String() != expected {
		t.Errorf("WSStopLossLimit() didn't set 'ordertype', 'price' and 'price2' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSTakeProfitLimit
	buffer.Reset()
	WSTakeProfitLimit("1000", "2000")(buffer)
	expected = `, "ordertype": "take-profit-limit", "price": "1000", "price2": "2000"`
	if buffer.String() != expected {
		t.Errorf("WSTakeProfitLimit() didn't set 'ordertype', 'price' and 'price2' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSTrailingStop
	buffer.Reset()
	WSTrailingStop("1000")(buffer)
	expected = `, "ordertype": "trailing-stop", "price": "1000"`
	if buffer.String() != expected {
		t.Errorf("WSTrailingStop() didn't set 'ordertype' and 'price' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSTrailingStopLimit
	buffer.Reset()
	WSTrailingStopLimit("1000", "2000")(buffer)
	expected = `, "ordertype": "trailing-stop-limit", "price": "1000", "price2": "2000"`
	if buffer.String() != expected {
		t.Errorf("WSTrailingStopLimit() didn't set 'ordertype', 'price' and 'price2' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSSettlePosition
	buffer.Reset()
	WSSettlePosition("2")(buffer)
	expected = `, "ordertype": "settle-position", "leverage": 2`
	if buffer.String() != expected {
		t.Errorf("WSSettlePosition() didn't set 'ordertype' and 'leverage' correctly, got: %s, want: %s", buffer.String(), expected)
	}
}

func TestWSAddOrderOption(t *testing.T) {
	buffer := &bytes.Buffer{}

	// Test WSUserRef
	WSUserRef("1234")(buffer)
	expected := `, "userref": "1234"`
	if buffer.String() != expected {
		t.Errorf("WSUserRef() didn't set 'userref' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSLeverage
	buffer.Reset()
	WSLeverage("2")(buffer)
	expected = `, "leverage": 2`
	if buffer.String() != expected {
		t.Errorf("WSLeverage() didn't set 'leverage' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSReduceOnly
	buffer.Reset()
	WSReduceOnly()(buffer)
	expected = `, "reduce_only": true`
	if buffer.String() != expected {
		t.Errorf("WSReduceOnly() didn't set 'reduce_only' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSOrderFlags
	buffer.Reset()
	WSOrderFlags("flag1,flag2")(buffer)
	expected = `, "oflags": "flag1,flag2"`
	if buffer.String() != expected {
		t.Errorf("WSOrderFlags() didn't set 'oflags' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSPostOnly
	buffer.Reset()
	WSPostOnly()(buffer)
	expected = `, "oflags": "post"`
	if buffer.String() != expected {
		t.Errorf("WSPostOnly() didn't set 'oflags' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSFCIB
	buffer.Reset()
	WSFCIB()(buffer)
	expected = `, "oflags": "fcib"`
	if buffer.String() != expected {
		t.Errorf("WSFCIB() didn't set 'oflags' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSFCIQ
	buffer.Reset()
	WSFCIQ()(buffer)
	expected = `, "oflags": "fciq"`
	if buffer.String() != expected {
		t.Errorf("WSFCIQ() didn't set 'oflags' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSNOMPP
	buffer.Reset()
	WSNOMPP()(buffer)
	expected = `, "oflags": "nompp"`
	if buffer.String() != expected {
		t.Errorf("WSNOMPP() didn't set 'oflags' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSVIQC
	buffer.Reset()
	WSVIQC()(buffer)
	expected = `, "oflags": "viqc"`
	if buffer.String() != expected {
		t.Errorf("WSVIQC() didn't set 'oflags' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test all flags (normal use should never do this)
	buffer.Reset()
	WSPostOnly()(buffer)
	WSFCIB()(buffer)
	WSFCIQ()(buffer)
	WSNOMPP()(buffer)
	WSVIQC()(buffer)
	expected = `, "oflags": "post,fcib,fciq,nompp,viqc"`
	if buffer.String() != expected {
		t.Errorf("WSVIQC() didn't set 'oflags' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSImmediateOrCancel
	buffer.Reset()
	WSImmediateOrCancel()(buffer)
	expected = `, "timeinforce": "IOC"`
	if buffer.String() != expected {
		t.Errorf("WSImmediateOrCancel() didn't set 'timeinforce' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSGoodTilDate
	buffer.Reset()
	WSGoodTilDate("1642099200")(buffer)
	expected = `, "timeinforce": "GTD", "expiretm": "1642099200"`
	if buffer.String() != expected {
		t.Errorf("WSGoodTilDate() didn't set 'timeinforce' and 'expiretm' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSCloseLimit
	buffer.Reset()
	WSCloseLimit("1000")(buffer)
	expected = `, "close[ordertype]": "limit", "close[price]": "1000"`
	if buffer.String() != expected {
		t.Errorf("WSCloseLimit() didn't set 'close[ordertype]' and 'close[price]' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSCloseStopLoss
	buffer.Reset()
	WSCloseStopLoss("1000")(buffer)
	expected = `, "close[ordertype]": "stop-loss", "close[price]": "1000"`
	if buffer.String() != expected {
		t.Errorf("WSCloseStopLoss() didn't set 'close[ordertype]' and 'close[price]' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSCloseTakeProfit
	buffer.Reset()
	WSCloseTakeProfit("1000")(buffer)
	expected = `, "close[ordertype]": "take-profit", "close[price]": "1000"`
	if buffer.String() != expected {
		t.Errorf("WSCloseTakeProfit() didn't set 'close[ordertype]' and 'close[price]' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSCloseStopLossLimit
	buffer.Reset()
	WSCloseStopLossLimit("1000", "1200")(buffer)
	expected = `, "close[ordertype]": "stop-loss-limit", "close[price]": "1000", "close[price2]": "1200"`
	if buffer.String() != expected {
		t.Errorf("WSCloseStopLossLimit() didn't set 'close[ordertype]', 'close[price]', and 'close[price2]' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSCloseTakeProfitLimit
	buffer.Reset()
	WSCloseTakeProfitLimit("1000", "1200")(buffer)
	expected = `, "close[ordertype]": "take-profit-limit", "close[price]": "1000", "close[price2]": "1200"`
	if buffer.String() != expected {
		t.Errorf("WSCloseTakeProfitLimit() didn't set 'close[ordertype]', 'close[price]', and 'close[price2]' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSCloseTrailingStop
	buffer.Reset()
	WSCloseTrailingStop("1000")(buffer)
	expected = `, "close[ordertype]": "trailing-stop", "close[price]": "1000"`
	if buffer.String() != expected {
		t.Errorf("WSCloseTrailingStop() didn't set 'close[ordertype]' and 'close[price]' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSCloseTrailingStopLimit
	buffer.Reset()
	WSCloseTrailingStopLimit("1000", "1200")(buffer)
	expected = `, "close[ordertype]": "trailing-stop-limit", "close[price]": "1000", "close[price2]": "1200"`
	if buffer.String() != expected {
		t.Errorf("WSCloseTrailingStopLimit() didn't set 'close[ordertype]', 'close[price]', and 'close[price2]' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSAddWithDeadline
	buffer.Reset()
	WSAddWithDeadline("1642099200")(buffer)
	expected = `, "deadline": "1642099200"`
	if buffer.String() != expected {
		t.Errorf("WSAddWithDeadline() didn't set 'deadline' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSValidateAddOrder
	buffer.Reset()
	WSValidateAddOrder()(buffer)
	expected = `, "validate": "true"`
	if buffer.String() != expected {
		t.Errorf("WSValidateAddOrder() didn't set 'validate' correctly, got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSAddOrderReqID
	buffer.Reset()
	WSAddOrderReqID("1234")(buffer)
	expected = `, "reqid": 1234`
	if buffer.String() != expected {
		t.Errorf("WSAddOrderReqID() didn't set 'reqid' correctly, got: %s, want: %s", buffer.String(), expected)
	}
}

func TestWSEditOrderOption(t *testing.T) {
	buffer := &bytes.Buffer{}

	// Test WSNewUserRef
	WSNewUserRef("1234")(buffer)
	expected := `, "newuserref": "1234"`
	if buffer.String() != expected {
		t.Errorf("WSNewUserRef() didn't set 'newuserref' correctly; got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSNewVolume
	buffer.Reset()
	WSNewVolume("10.0")(buffer)
	expected = `, "volume": "10.0"`
	if buffer.String() != expected {
		t.Errorf("WSNewVolume() didn't set 'volume' correctly; got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSNewPrice
	buffer.Reset()
	WSNewPrice("1000.0")(buffer)
	expected = `, "price": "1000.0"`
	if buffer.String() != expected {
		t.Errorf("WSNewPrice() didn't set 'price' correctly; got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSNewPrice2
	buffer.Reset()
	WSNewPrice2("1001.0")(buffer)
	expected = `, "price2": "1001.0"`
	if buffer.String() != expected {
		t.Errorf("WSNewPrice2() didn't set 'price2' correctly; got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSNewPostOnly
	buffer.Reset()
	WSNewPostOnly()(buffer)
	expected = `, "oflags": "post"`
	if buffer.String() != expected {
		t.Errorf("WSNewPostOnly() didn't set 'oflags' correctly; got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSValidateEditOrder
	buffer.Reset()
	WSValidateEditOrder()(buffer)
	expected = `, "validate": "true"`
	if buffer.String() != expected {
		t.Errorf("WSValidateEditOrder() didn't set 'validate' correctly; got: %s, want: %s", buffer.String(), expected)
	}

	// Test WSEditOrderReqID
	buffer.Reset()
	WSEditOrderReqID("1234")(buffer)
	expected = `, "reqid": 1234`
	if buffer.String() != expected {
		t.Errorf("WSEditOrderReqID() didn't set 'reqid' correctly; got: %s, want: %s", buffer.String(), expected)
	}
}
