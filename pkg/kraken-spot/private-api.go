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
	"strconv"
	"strings"
	"time"

	"github.com/readysetliqd/crypto-exchange-library-go/pkg/kraken-spot/internal/data"
)

// #region KrakenClient type definition and constructor function

var sharedClient = &http.Client{}

type KrakenClient struct {
	APIKey    string
	APISecret []byte
	Client    *http.Client
}

// Creates new authenticated client KrakenClient for Kraken API
func NewKrakenClient(apiKey, apiSecret string) (*KrakenClient, error) {
	decodedSecret, err := base64.StdEncoding.DecodeString(apiSecret)
	if err != nil {
		return nil, err
	}
	return &KrakenClient{
		APIKey:    apiKey,
		APISecret: decodedSecret,
		Client:    sharedClient,
	}, nil
}

// #endregion

// #region Unexported KrakenClient methods

// Generates a signature for a request to the Kraken API
func (kc *KrakenClient) getSignature(urlPath string, values url.Values) string {
	sha := sha256.New()
	sha.Write([]byte(values.Get("nonce") + values.Encode()))
	shasum := sha.Sum(nil)

	mac := hmac.New(sha512.New, kc.APISecret)
	mac.Write(append([]byte(urlPath), shasum...))
	macsum := mac.Sum(nil)
	return base64.StdEncoding.EncodeToString(macsum)
}

// Sends a request to the Kraken API and returns the response
func (kc *KrakenClient) doRequest(urlPath string, values url.Values) (*http.Response, error) {
	signature := kc.getSignature(urlPath, values)

	req, err := http.NewRequest("POST", baseUrl+urlPath, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Add("API-Key", kc.APIKey)
	req.Header.Add("API-Sign", signature)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	return kc.Client.Do(req)
}

// Processes Kraken API response and unmarshals it into ApiResp data struct.
// Passed arg 'target' is unmarshalled into ApiResp 'Result' field.
func processPrivateApiResponse(res *http.Response, target interface{}) error {
	var err error
	if res.StatusCode == http.StatusOK {
		msg, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}
		resp := data.ApiResp{Result: target}
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

// #endregion

// #region Authenticated Account Data endpoints

// Calls Kraken API private Account Data "Balance" endpoint. Returns map of all
// "cash" (including coins) balances, net of pending withdrawals as strings
//
// Required Permissions: Funding Permissions - Query
func (kc *KrakenClient) GetAccountBalances() (*map[string]string, error) {
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	res, err := kc.doRequest(privatePrefix+"Balance", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()
	var balances map[string]string
	err = processPrivateApiResponse(res, &balances)
	if err != nil {
		return nil, err
	}
	return &balances, nil
}

// Calls Kraken API private Account Data "Balance" endpoint. Returns float of
// total USD balance "ZUSD"
//
// Required Permissions: Funding Permissions - Query
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
// Required Permissions: Funding Permissions - Query
func (kc *KrakenClient) GetExtendedBalances() (*map[string]data.ExtendedBalance, error) {
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	res, err := kc.doRequest(privatePrefix+"BalanceEx", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()
	var balances map[string]data.ExtendedBalance
	err = processPrivateApiResponse(res, &balances)
	if err != nil {
		err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
		return nil, err
	}
	return &balances, nil
}

// Calls Kraken API private Account Data "BalanceEx" endpoint. Returns map of all
// available account balances as float64. Balance available for trading is
// calculated as: available balance = balance + credit - credit_used - hold_trade
//
// Required Permissions: Funding Permissions - Query
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
// Required Permissions: Funding Permissions - Query
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
// Required Permissions: Funding Permissions - Query; Order and Trades - Query
// open orders & trades
func (kc *KrakenClient) GetTradeBalance(asset ...string) (*data.TradeBalance, error) {
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	if len(asset) > 0 {
		if len(asset) > 1 {
			err := fmt.Errorf("invalid number of args passed to 'asset'; expected 0 or 1")
			return nil, err
		}
		payload.Add("asset", asset[0])
	}
	res, err := kc.doRequest(privatePrefix+"TradeBalance", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()
	var balance data.TradeBalance
	err = processPrivateApiResponse(res, &balance)
	if err != nil {
		err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
		return nil, err
	}
	return &balance, nil
}

// Calls Kraken API private Account Data "OpenOrders" endpoint. Retrieves
// information for all currently open orders. Accepts functional options args
// 'options'.
//
// Required Permissions: Order and Trades - Query open orders & trades
//
// # Functional Options:
//
// // Whether or not to include trades related to position in output. Defaults
// to false if not called
//
//	func OOWithTrades(trades bool) GetOpenOrdersOption
//
// // Restrict results to given user reference id. Defaults to no restrictions
// if not called
//
//	func OOWithUserRef(userRef int) GetOpenOrdersOption
//
// # Example Usage:
//
//	orders, err := kc.GetOpenOrders(krakenspot.OOWithTrades(true), krakenspot.OOWithUserRef(123))
func (kc *KrakenClient) GetOpenOrders(options ...GetOpenOrdersOption) (*data.OpenOrdersResp, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	for _, option := range options {
		option(payload)
	}

	// Send Request to Kraken API
	res, err := kc.doRequest(privatePrefix+"OpenOrders", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var openOrders data.OpenOrdersResp
	err = processPrivateApiResponse(res, &openOrders)
	if err != nil {
		err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
		return nil, err
	}
	return &openOrders, nil
}

// Calls Kraken API private Account Data "ClosedOrders" endpoint. Retrieves
// information for most recent closed orders. Accepts functional options args
// 'options'.
//
// Required Permissions: Order and Trades - Query closed orders & trades
//
// # Functional Options:
//
// // Whether or not to include trades related to position in output. Defaults
// to false if not called
//
//	func COWithTrades(trades bool) GetClosedOrdersOption
//
// // Restrict results to given user reference id. Defaults to no restrictions
// if not called
//
//	func COWithUserRef(userRef int) GetClosedOrdersOption
//
// // Starting unix timestamp or order tx ID of results (exclusive). If an order's
// tx ID is given for start or end time, the order's opening time (opentm) is used.
// Defaults to show most recent orders if not called
//
//	func COWithStart(start int) GetClosedOrdersOption
//
// // Ending unix timestamp or order tx ID of results (exclusive). If an order's
// tx ID is given for start or end time, the order's opening time (opentm) is used
// Defaults to show most recent orders if not called
//
//	func COWithEnd(end int) GetClosedOrdersOption
//
// // Result offset for pagination. Defaults to no offset if not called
//
//	func COWithOffset(offset int) GetClosedOrdersOption
//
// // Which time to use to search and filter results for COWithStart() and COWithEnd()
// Defaults to "both" if not called or invalid arg 'closeTime' passed
//
// // Enum: "open", "close", "both"
//
//	func COWithCloseTime(closeTime string) GetClosedOrdersOption
//
// // Whether or not to consolidate trades by individual taker trades. Defaults to
// true if not called
//
//	func COWithConsolidateTaker(consolidateTaker bool) GetClosedOrdersOption
//
// # Example Usage:
//
//	orders, err := kc.GetClosedOrders(krakenspot.COWithConsolidateTaker(true), krakenspot.COWithCloseTime("open"))
func (kc *KrakenClient) GetClosedOrders(options ...GetClosedOrdersOption) (*data.ClosedOrdersResp, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	for _, option := range options {
		option(payload)
	}

	// Send Request to Kraken API
	res, err := kc.doRequest(privatePrefix+"ClosedOrders", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var closedOrders data.ClosedOrdersResp
	err = processPrivateApiResponse(res, &closedOrders)
	if err != nil {
		err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
		return nil, err
	}
	return &closedOrders, nil
}

// Calls Kraken API private Account Data "OrdersInfo" endpoint. Retrieves order
// information for specific orders with transaction id passed to arg 'txID'.
// Accepts multiple orders with transaction IDs passed as a single comma delimited
// string with no white-space (50 maximum). Accepts functional options args 'options'.
//
// Required Permissions: Order and Trades - Query closed orders & trades
//
// # Functional Options:
//
// // Whether or not to include trades related to position in output. Defaults
// to false if not called
//
//	func OIWithTrades(trades bool) GetOrdersInfoOptions
//
// // Restrict results to given user reference id. Defaults to no restrictions
// if not called
//
//	func OIWithUserRef(userRef int) GetOrdersInfoOptions
//
// // Whether or not to consolidate trades by individual taker trades. Defaults to
// true if not called
//
//	func OIWithConsolidateTaker(consolidateTaker bool)
//
// Example usage:
//
//	orders, err := kc.GetOrdersInfo("OYR15S-VHRBC-VY5NA2,OYBGFG-LQHXB-RJHY4C", krakenspot.OIWithConsolidateTaker(true))
func (kc *KrakenClient) GetOrdersInfo(txID string, options ...GetOrdersInfoOption) (*map[string]data.Order, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("txid", txID)
	for _, option := range options {
		option(payload)
	}

	// Send request
	res, err := kc.doRequest(privatePrefix+"QueryOrders", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var queriedOrders map[string]data.Order
	err = processPrivateApiResponse(res, &queriedOrders)
	if err != nil {
		err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
		return nil, err
	}
	return &queriedOrders, nil
}

// Calls Kraken API private Account Data "TradesHistory" endpoint. Retrieves
// information about trades/fills. 50 results are returned at a time, the most
// recent by default.
//
// Required Permissions: Order and Trades - Query closed orders & trades
//
// # Functional Options:
//
// // Type of trade. Defaults to "all" if not called or invalid 'tradeType' passed.
//
// // Enum: "all", "any position", "closed position", "closing position", "no position"
//
//	func THWithType(tradeType string) GetTradesHistoryOptions
//
// // Whether or not to include trades related to position in output. Defaults
// to false if not called
//
//	func THWithTrades(trades bool) GetTradesHistoryOptions
//
// // Starting unix timestamp or order tx ID of results (exclusive). If an order's
// tx ID is given for start or end time, the order's opening time (opentm) is used.
// Defaults to show most recent orders if not called
//
//	func THWithStart(start int) GetTradesHistoryOptions
//
// // Ending unix timestamp or order tx ID of results (exclusive). If an order's
// tx ID is given for start or end time, the order's opening time (opentm) is used
// Defaults to show most recent orders if not called
//
//	func THWithEnd(end int) GetTradesHistoryOptions
//
// // Result offset for pagination. Defaults to no offset if not called
//
//	func THWithOffset(offset int) GetTradesHistoryOptions
//
// // Whether or not to consolidate trades by individual taker trades. Defaults to
// true if not called
//
//	func THWithConsolidateTaker(consolidateTaker bool)
//
// # Example Usage:
//
//	trades, err := kc.GetTradesHistory(krakenspot.THWithType("closed position"), krakenspot.THWithOffset(2))
func (kc *KrakenClient) GetTradesHistory(options ...GetTradesHistoryOption) (*data.TradesHistoryResp, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	for _, option := range options {
		option(payload)
	}

	// Send request
	res, err := kc.doRequest(privatePrefix+"TradesHistory", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var tradesHistory data.TradesHistoryResp
	err = processPrivateApiResponse(res, &tradesHistory)
	if err != nil {
		err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
		return nil, err
	}
	return &tradesHistory, nil
}

// Calls Kraken API private Account Data "QueryTrades" endpoint. Retrieves
// information specific trades/fills with transaction id passed to arg 'txID'.
// Accepts multiple trades with transaction IDs passed as a single comma delimited
// string with no white-space (20 maximum). Accepts functional options args 'options'.
//
// Required Permissions: Order and Trades - Query closed orders & trades
//
// # Functional Options:
//
// // Whether or not to include trades related to position in output. Defaults
// to false if not called
//
//	func TIWithTrades(trades bool)
//
// # Example Usage:
//
//	trades, err := kc.GetTradeInfo("TRWCIF-3MJWU-5DYJG5,TNGJFU-5CD67-ZV3AEO")
func (kc *KrakenClient) GetTradeInfo(txID string, options ...GetTradeInfoOption) (*map[string]data.TradeInfo, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("txid", txID)
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	res, err := kc.doRequest(privatePrefix+"QueryTrades", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var tradeInfo map[string]data.TradeInfo
	err = processPrivateApiResponse(res, &tradeInfo)
	if err != nil {
		err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
		return nil, err
	}
	return &tradeInfo, nil
}

// Calls Kraken API private Account Data "OpenPositions" endpoint. Gets information
// about open margin positions. Accepts functional options args 'options'.
//
// Required Permissions: Order and Trades - Query open orders & trades
//
// # Functional Options:
//
// // Comma delimited list of txids to limit output to. Defaults to show all open
// positions if not called
//
//	func OPWithTxID(txID string) GetOpenPositionsOption
//
// // Whether to include P&L calculations. Defaults to false if not called
//
//	func OPWithDoCalcs(doCalcs bool) GetOpenPositionsOption
//
// # Example Usage:
//
//	positions, err := kc.GetOpenPositions(krakenspot.OPWithDoCalcs(true))
func (kc *KrakenClient) GetOpenPositions(options ...GetOpenPositionsOption) (*map[string]data.OpenPosition, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	res, err := kc.doRequest(privatePrefix+"OpenPositions", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var openPositions map[string]data.OpenPosition
	err = processPrivateApiResponse(res, &openPositions)
	if err != nil {
		err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
		return nil, err
	}
	return &openPositions, nil
}

// Calls Kraken API private Account Data "OpenPositions" endpoint. Gets information
// about open margin positions consolidated by market/pair. Accepts functional
// options args 'options'.
//
// Required Permissions: Order and Trades - Query open orders & trades
//
// # Functional Options:
//
// // Comma delimited list of txids to limit output to. Defaults to show all open
// positions if not called
//
//	func OPCWithTxID(txID string) GetOpenPositionsOption
//
// // Whether to include P&L calculations. Defaults to false if not called
//
//	func OPCWithDoCalcs(doCalcs bool) GetOpenPositionsOption
//
// # Example Usage:
//
//	positions, err := kc.GetOpenPositionsConsolidated(krakenspot.OPWithDoCalcs(true))
func (kc *KrakenClient) GetOpenPositionsConsolidated(options ...GetOpenPositionsConsolidatedOption) (*[]data.OpenPositionConsolidated, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("consolidation", "market")
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	res, err := kc.doRequest(privatePrefix+"OpenPositions", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var openPositions []data.OpenPositionConsolidated
	err = processPrivateApiResponse(res, &openPositions)
	if err != nil {
		err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
		return nil, err
	}
	return &openPositions, nil
}

// Calls Kraken API private Account Data "Ledgers" endpoint. Retrieves information
// about ledger entries. 50 results are returned at a time, the most recent by
// default. Accepts functional options args 'options'.
//
// Required Permissions: Data - Query ledger entries
//
// # Functional Options:
//
// // Filter output by asset or comma delimited list of assets. Defaults to "all"
// if not called.
//
//	func LIWithAsset(asset string) GetLedgersInfoOption
//
// // Filter output by asset class. Defaults to "currency" if not called
// //
// // Enum: "currency", ...?
//
//	func LIWithAclass(aclass string) GetLedgersInfoOption
//
// // Type of ledger to retrieve. Defaults to "all" if not called or invalid
// 'ledgerType' passed.
//
// // Enum: "all", "trade", "deposit", "withdrawal", "transfer", "margin", "adjustment",
// "rollover", "credit", "settled", "staking", "dividend", "sale", "nft_rebate"
//
//	func LIWithType(ledgerType string) GetLedgersInfoOption
//
// // Starting unix timestamp or ledger ID of results (exclusive). Defaults to most
// recent ledgers if not called.
//
//	func LIWithStart(start int) GetLedgersInfoOption
//
// // Ending unix timestamp or ledger ID of results (inclusive). Defaults to most
// recent ledgers if not called.
//
//	func LIWithEnd(end int) GetLedgersInfoOption
//
// // Result offset for pagination. Defaults to no offset if not called.
//
//	func LIWithOffset(offset int) GetLedgersInfoOption
//
// // If true, does not retrieve count of ledger entries. Request can be noticeably
// faster for users with many ledger entries as this avoids an extra database query.
// Defaults to false if not called.
//
//	func LIWithoutCount(withoutCount bool) GetLedgersInfoOption
//
// # Example Usage:
//
//	ledgers, err := kc.GetLedgersInfo(krakenspot.LIWithAsset("ZUSD,XXBT"), krakenspot.LIWithoutCount(true), krakenspot.LIWithOffset(5))
func (kc *KrakenClient) GetLedgersInfo(options ...GetLedgersInfoOption) (*data.LedgersInfoResp, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	res, err := kc.doRequest(privatePrefix+"Ledgers", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var ledgersInfo data.LedgersInfoResp
	err = processPrivateApiResponse(res, &ledgersInfo)
	if err != nil {
		err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
		return nil, err
	}
	return &ledgersInfo, nil
}

// Calls Kraken API private Account Data "QueryLedgers" endpoint. Retrieves
// information about specific ledger entries passed to arg 'ledgerID'. Accepts
// multiple ledgers with ledger IDs passed as a single comma delimited string
// with no white-space (20 maximum). Accepts functional options args 'options'.
//
// Required Permissions:
// Data - Query ledger entries
//
// # Functional Options:
//
// // Whether or not to include trades related to position in output. Defaults to
// false if not called.
//
//	func GLWithTrades(trades bool) GetLedgerOption
//
// # Example Usage:
//
//	ledger, err := kc.GetLedger("LGBRJU-SQZ4L-5HLS3C,L3S26P-BHIOV-TTWYYI", krakenspot.GLWithTrades(true))
func (kc *KrakenClient) GetLedger(ledgerID string, options ...GetLedgerOption) (*map[string]data.Ledger, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("id", ledgerID)
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	res, err := kc.doRequest(privatePrefix+"QueryLedgers", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var ledgersInfo map[string]data.Ledger
	err = processPrivateApiResponse(res, &ledgersInfo)
	if err != nil {
		err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
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
// Required Permissions: Funds permissions - Query
//
// # Functional Options:
//
// // Comma delimited list of asset pairs to get fee info on. Defaults to show none
// if not called.
//
//	func TVWithPair(pair string) GetTradeVolumeOption
//
// # Example Usage:
//
//	kc.GetTradeVolume(krakenspot.TVWithPair("XXBTZUSD,XETHZUSD"))
func (kc *KrakenClient) GetTradeVolume(options ...GetTradeVolumeOption) (*data.TradeVolume, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	res, err := kc.doRequest(privatePrefix+"TradeVolume", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var tradeVolume data.TradeVolume
	err = processPrivateApiResponse(res, &tradeVolume)
	if err != nil {
		err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
		return nil, err
	}
	return &tradeVolume, nil
}

// Calls Kraken API private Account Data "AddExport" endpoint. Requests export
// of trades data to file, defaults to CSV file type. Returns string containing
// report ID or empty string if encountered error. Accepts functional options
// args 'options'.
//
// Required Permissions: Orders and trades - Query open orders and trades;
// Orders and trades - Query closed orders and trades; Data - Export data
//
// # Functional Options:
//
// // File format to export. Defaults to "CSV" if not called or invalid value
// passed to arg 'format'
//
// // Enum: "CSV", "TSV"
//
//	func RTWithFormat(format string) RequestTradesExportReportOption
//
// // Accepts comma-delimited list of fields passed as a single string to include
// in report. Defaults to "all" if not called. API will return error:
// [EGeneral:Internal error] if invalid value passed to arg 'fields'. Function has
// no validation checks for passed value to 'fields'
//
// // Enum: "ordertxid", "time", "ordertype", "price", "cost", "fee", "vol",
// "margin", "misc", "ledgers"
//
//	func RTWithFields(fields string) RequestTradesExportReportOption
//
// // UNIX timestamp for report start time. Defaults to 1st of the current month
// if not called
//
//	func RTWithStart(start int) RequestTradesExportReportOption
//
// // UNIX timestamp for report end time. Defaults to current time if not called
//
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
	res, err := kc.doRequest(privatePrefix+"AddExport", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return "", err
	}
	defer res.Body.Close()

	// Process API response
	var exportResp data.RequestExportReportResp
	err = processPrivateApiResponse(res, &exportResp)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "EGeneral:Internal error") {
			err = fmt.Errorf("if RLWithFields() was passed to 'options', check 'fields' format is correct and 'fields' values matche enum | error calling processPrivateApiResponse() | %w", err)
			return "", err
		} else {
			err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
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
// Required Permissions: Data - Query ledger entries; Data - Export data
//
// # Functional Options:
//
// // File format to export. Defaults to "CSV" if not called or invalid value
// passed to arg 'format'
//
// // Enum: "CSV", "TSV"
//
//	func RLWithFormat(format string) RequestLedgersExportReportOption
//
// // Accepts comma-delimited list of fields passed as a single string to include
// in report. Defaults to "all" if not called. API will return error:
// [EGeneral:Internal error] if invalid value passed to arg 'fields'. Function has
// no validation checks for passed value to 'fields'
//
// // Enum: "refid", "time", "type", "aclass", "asset",
// "amount", "fee", "balance"
//
//	func RLWithFields(fields string) RequestLedgersExportReportOption
//
// // UNIX timestamp for report start time. Defaults to 1st of the current month
// if not called
//
//	func RLWithStart(start int) RequestLedgersExportReportOption
//
// // UNIX timestamp for report end time. Defaults to current time if not called
//
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
	res, err := kc.doRequest(privatePrefix+"AddExport", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return "", err
	}
	defer res.Body.Close()

	// Process API response
	var exportResp data.RequestExportReportResp
	err = processPrivateApiResponse(res, &exportResp)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "EGeneral:Internal error") {
			err = fmt.Errorf("if RLWithFields() was passed to 'options', check 'fields' format is correct and 'fields' values matche enum | error calling processPrivateApiResponse() | %w", err)
			return "", err
		} else {
			err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
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
// Required Permissions: Data - Export data
//
// # Example Usage:
//
//	reportStatus, err := kc.GetExportReportStatus("ledgers")
func (kc *KrakenClient) GetExportReportStatus(reportType string) (*[]data.ExportReportStatus, error) {
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
	res, err := kc.doRequest(privatePrefix+"ExportStatus", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var exportReports []data.ExportReportStatus
	err = processPrivateApiResponse(res, &exportReports)
	if err != nil {
		err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
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
// Required Permissions: Data - Export data
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
// Required Permissions: Data - Export data
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
	res, err := kc.doRequest(privatePrefix+"RemoveExport", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return err
	}
	defer res.Body.Close()

	// Process API response
	var deleteResp data.DeleteReportResp
	err = processPrivateApiResponse(res, &deleteResp)
	if err != nil {
		err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
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

// TODO finish implementation checklist
// Calls Kraken API private Trading "AddOrder" endpoint.
// func (kc *KrakenClient) AddOrder() (, error) {
// 	return nil, nil
// }

// TODO finish implementation checklist
// Calls Kraken API private Trading "AddOrderBatch" endpoint.
// func (kc *KrakenClient) AddOrderBatch() (, error) {
// 	return nil, nil
// }

// TODO finish implementation checklist
// Calls Kraken API private Trading "EditOrder" endpoint.
// func (kc *KrakenClient) EditOrder() (, error) {
// 	return nil, nil
// }

// TODO finish implementation checklist
// Calls Kraken API private Trading "CancelOrder" endpoint.
// func (kc *KrakenClient) CancelOrder() (, error) {
// 	return nil, nil
// }

// TODO finish implementation checklist
// Calls Kraken API private Trading "CancelAll" endpoint.
// func (kc *KrakenClient) CancelAllOrders() (uint32, error) {
// 	return 0, nil
// }

// TODO finish implementation checklist
// Calls Kraken API private Trading "CancelAllOrdersAfter" endpoint.
// func (kc *KrakenClient) CancelAllOrdersAfter() (, error) {
// 	return nil, nil
// }

// TODO finish implementation checklist
// Calls Kraken API private Trading "CancelOrderBatch" endpoint.
// func (kc *KrakenClient) CancelOrderBatch() (uint8, error) {
// 	return 0, nil
// }

// #endregion

// #region Authenticated Funding endpoints

// TODO finish implementation checklist
// Calls Kraken API private Funding "DepositMethods" endpoint.
// func (kc *KrakenClient) GetDepositMethods() (, error) {
// 	return nil, nil
// }

// TODO finish implementation checklist
// Calls Kraken API private Funding "DepositAddresses" endpoint.
// func (kc *KrakenClient) GetDepositAddresses() (, error) {
// 	return nil, nil
// }

// TODO finish implementation checklist
// Calls Kraken API private Funding "DepositStatus" endpoint.
// func (kc *KrakenClient) GetDepositsStatus() (, error) {
// 	return nil, nil
// }

// TODO finish implementation checklist
// Calls Kraken API private Funding "WithdrawMethods" endpoint.
// func (kc *KrakenClient) GetWithdrawalMethods() (, error) {
// 	return nil, nil
// }

// TODO finish implementation checklist
// Calls Kraken API private Funding "WithdrawAddresses" endpoint.
// func (kc *KrakenClient) GetWithdrawalAddresses() (, error) {
// 	return nil, nil
// }

// TODO finish implementation checklist
// Calls Kraken API private Funding "WithdrawInfo" endpoint.
// func (kc *KrakenClient) GetWithdrawalInfo() (, error) {
// 	return nil, nil
// }

// TODO finish implementation checklist
// Calls Kraken API private Funding "Withdraw" endpoint.
// func (kc *KrakenClient) WithdrawFunds() (string, error) {
// 	return nil, nil
// }

// TODO finish implementation checklist
// Calls Kraken API private Funding "WithdrawStatus" endpoint.
// func (kc *KrakenClient) GetWithdrawalsStatus() (, error) {
// 	return nil, nil
// }

// TODO finish implementation checklist
// Calls Kraken API private Funding "WithdrawCancel" endpoint.
// func (kc *KrakenClient) CancelWithdrawal() (error) {
// 	return nil
// }

// TODO finish implementation checklist
// Calls Kraken API private Funding "WalletTransfer" endpoint.
// func (kc *KrakenClient) WalletTransfer() (string, error) {
// 	return nil, nil
// }

// #endregion

// #region Authenticated Subaccounts endpoints

// Calls Kraken API private Subaccounts "CreateSubaccount" endpoint. Creates a
// trading subaccount with details passed to args 'username' and 'email'
//
// Note: Subaccounts are currently only available to institutional clients.
// Please contact your Account Manager for more details.
//
// Required Permissions: Institutional verification
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
	res, err := kc.doRequest(privatePrefix+"CreateSubaccount", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return err
	}
	defer res.Body.Close()

	// Process API response
	var result bool
	err = processPrivateApiResponse(res, &result)
	if err != nil {
		err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
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
// Required Permissions: Institutional verification
//
// # Example Usage:
//
//	transfer, err := kc.AccountTransfer("XBT", "1.0", "ABCD 1234 EFGH 5678", "IJKL 0987 MNOP 6543")
func (kc *KrakenClient) AccountTransfer(asset string, amount string, fromAccount string, toAccount string) (*data.AccountTransfer, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	payload.Add("asset", asset)
	payload.Add("amount", amount)
	payload.Add("fromAccount", fromAccount)
	payload.Add("toAccount", toAccount)

	// Send request to server
	res, err := kc.doRequest(privatePrefix+"AccountTransfer", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var transfer data.AccountTransfer
	err = processPrivateApiResponse(res, &transfer)
	if err != nil {
		err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
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
// Required permissions: Funds permissions - Earn
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
	res, err := kc.doRequest(privatePrefix+"Earn/Allocate", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return err
	}
	defer res.Body.Close()

	// Process API response
	var result bool
	err = processPrivateApiResponse(res, &result)
	if err != nil {
		err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
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
// Required permissions: Funds permissions - Earn
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
	res, err := kc.doRequest(privatePrefix+"Earn/Deallocate", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return err
	}
	defer res.Body.Close()

	// Process API response
	var result bool
	err = processPrivateApiResponse(res, &result)
	if err != nil {
		err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
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
// Required Permissions: Funds permissions - Query OR Funds permissions - Earn
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
	res, err := kc.doRequest(privatePrefix+"Earn/AllocateStatus", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return false, err
	}
	defer res.Body.Close()

	// Process API response
	var status data.AllocationStatus
	err = processPrivateApiResponse(res, &status)
	if err != nil {
		err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
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
// Required Permissions: Funds permissions - Query OR Funds permissions - Earn
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
	res, err := kc.doRequest(privatePrefix+"Earn/DeallocateStatus", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return false, err
	}
	defer res.Body.Close()

	// Process API response
	var status data.AllocationStatus
	err = processPrivateApiResponse(res, &status)
	if err != nil {
		err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
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
// Required Permissions: None
//
// # Functional Options:
//
// // Filter strategies by asset name. Defaults to no filter if function not called
//
//	func ESWithAsset(asset string) GetEarnStrategiesOption
//
// // Filters displayed strategies by lock type. Accepts array of strings for arg
// 'lockTypes' and ignores invalid values passed. Defaults to no filter if
// function not called or only invalid values passed.
//
// // Enum - 'lockTypes': "flex", "bonded", "timed", "instant"
//
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
func (kc *KrakenClient) GetEarnStrategies(options ...GetEarnStrategiesOption) (*data.EarnStrategiesResp, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	res, err := kc.doRequest(privatePrefix+"Earn/Strategies", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var strategies data.EarnStrategiesResp
	err = processPrivateApiResponse(res, &strategies)
	if err != nil {
		err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
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
// Required Permissions: Funds permissions - Query
//
// # Functional Options:
//
// // Pass with arg 'ascending' set to true to sort strategies ascending. Defaults
// to false (descending) if function is not called
//
//	func EAWithAscending(ascending bool) GetEarnAllocationsOption
//
// // A secondary currency to express the value of your allocations. Defaults
// to express value in USD if function is not called
//
//	func EAWithConvertedAsset(asset string) GetEarnAllocationsOption
//
// // Omit entries for strategies that were used in the past but now they don't
// hold any allocation. Defaults to false (don't omit) if function is not called
//
//	func EAWithHideZeroAllocations(hide bool) GetEarnAllocationsOption
//
// # Example Usage:
//
//	allocations, err := kc.GetEarnAllocations(krakenspot.EAWithConvertedAsset("XBT"), krakenspot.EAWithHideZeroAllocations())
func (kc *KrakenClient) GetEarnAllocations(options ...GetEarnAllocationsOption) (*data.EarnAllocationsResp, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	for _, option := range options {
		option(payload)
	}

	// Send request to server
	res, err := kc.doRequest(privatePrefix+"Earn/Allocations", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var allocations data.EarnAllocationsResp
	err = processPrivateApiResponse(res, &allocations)
	if err != nil {
		err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
		return nil, err
	}
	return &allocations, nil
}

// #endregion

// #region Websockets Authentication endpoint

// Calls Kraken API private Account Data "GetWebSocketsToken" endpoint. An
// authentication token must be requested via this REST API endpoint in order
// to connect to and authenticate with our Websockets API. The token should be
// used within 15 minutes of creation, but it does not expire once a successful
// Websockets connection and private subscription has been made and is maintained.
//
// Required Permissions: WebSockets interface - On
//
// # Example Usage:
//
//	tokenResp, err := kc.GetWebSocketsToken()
//	token := (*tokenResp).Token
func (kc *KrakenClient) GetWebSocketsToken() (*data.WebSocketsToken, error) {
	// Build payload
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))

	// Send request to server
	res, err := kc.doRequest(privatePrefix+"GetWebSocketsToken", payload)
	if err != nil {
		err = fmt.Errorf("error sending request to server | %w", err)
		return nil, err
	}
	defer res.Body.Close()

	// Process API response
	var token data.WebSocketsToken
	err = processPrivateApiResponse(res, &token)
	if err != nil {
		err = fmt.Errorf("error calling processPrivateApiResponse() | %w", err)
		return nil, err
	}
	return &token, nil
}

// #endregion

// #region Helper functions

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
