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
		err := fmt.Errorf("error sending request to server")
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
		err := fmt.Errorf("error sending request to server")
		return nil, err
	}
	defer res.Body.Close()
	var balances map[string]data.ExtendedBalance
	err = processPrivateApiResponse(res, &balances)
	if err != nil {
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
		err := fmt.Errorf("error sending request to server")
		return nil, err
	}
	defer res.Body.Close()
	var balance data.TradeBalance
	err = processPrivateApiResponse(res, &balance)
	if err != nil {
		return nil, err
	}
	return &balance, nil
}

// func (kc *KrakenClient) GetOpenOrders() (, error) {
// 	// TODO create data type if necessary; implement function; write doc comments;
// 	// change return statement
// 	return nil, nil
// }

// func (kc *KrakenClient) GetClosedOrders() (, error) {
// 	// TODO create data type if necessary; implement function; write doc comments;
// 	// change return statement
// 	return nil, nil
// }

// func (kc *KrakenClient) GetOrdersInfo() (, error) {
// 	// TODO create data type if necessary; implement function; write doc comments;
// 	// change return statement
// 	return nil, nil
// }

// func (kc *KrakenClient) GetTradesHistory() (, error) {
// 	// TODO create data type if necessary; implement function; write doc comments;
// 	// change return statement
// 	return nil, nil
// }

// func (kc *KrakenClient) GetTradesInfo() (, error) {
// 	// TODO create data type if necessary; implement function; write doc comments;
// 	// change return statement
// 	return nil, nil
// }

// func (kc *KrakenClient) GetOpenPositions() (, error) {
// 	// TODO create data type if necessary; implement function; write doc comments;
// 	// change return statement
// 	return nil, nil
// }

// func (kc *KrakenClient) GetLedgersInfo() (, error) {
// 	// TODO create data type if necessary; implement function; write doc comments;
// 	// change return statement
// 	return nil, nil
// }

// func (kc *KrakenClient) GetLedgers() (, error) {
// 	// TODO create data type if necessary; implement function; write doc comments;
// 	// change return statement
// 	return nil, nil
// }

// func (kc *KrakenClient) GetTradeVolume() (, error) {
// 	// TODO create data type if necessary; implement function; write doc comments;
// 	// change return statement
// 	return nil, nil
// }

// func (kc *KrakenClient) RequestExportReport() (, error) {
// 	// TODO create data type if necessary; implement function; write doc comments;
// 	// change return statement
// 	return nil, nil
// }

// func (kc *KrakenClient) GetExportReportStatus() (, error) {
// 	// TODO create data type if necessary; implement function; write doc comments;
// 	// change return statement
// 	return nil, nil
// }

// func (kc *KrakenClient) RetrieveDataExport() (, error) {
// 	// TODO create data type if necessary; implement function; write doc comments;
// 	// change return statement
// 	return nil, nil
// }

// func (kc *KrakenClient) DeleteExportReport() (, error) {
// 	// TODO create data type if necessary; implement function; write doc comments;
// 	// change return statement
// 	return nil, nil
// }

// #endregion

// #region Authenticated Trading endpoints

// TODO
// fill in trading endpoints

// #endregion

// #region Authenticated Funding endpoints

// TODO
// fill in Funding endpoints

// #endregion

// #region Authenticated Subaccounts endpoints

// TODO
// fill in Subaccounts endpoints

// #endregion

// #region Authenticated Earn endpoints

// TODO
// fill in Earn endpoints

// #endregion

// #region Websockets Authentication endpoint

// TODO
// fill in one websockets function

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
