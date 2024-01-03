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

	req, err := http.NewRequest("POST", "https://api.kraken.com"+urlPath, strings.NewReader(values.Encode()))
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
			err = fmt.Errorf("error unmarshalling msg to resp | %v", err)
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

// Calls Kraken API private Account Data "Balance" endpoint. Returns map of all
// "cash" (including coins) balances, net of pending withdrawals as strings
//
// Required Permissions: Funding Permissions - Query
func (kc *KrakenClient) GetAccountBalance() (*map[string]string, error) {
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	res, err := kc.doRequest("/0/private/Balance", payload)
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
// available USD balance "ZUSD"
//
// Required Permissions: Funding Permissions - Query
func (kc *KrakenClient) USDBalance() (float64, error) {
	payload := url.Values{}
	payload.Add("nonce", strconv.FormatInt(time.Now().UnixNano(), 10))
	res, err := kc.doRequest("/0/private/Balance", payload)
	if err != nil {
		err := fmt.Errorf("error sending request to server")
		return 0.0, err
	}
	defer res.Body.Close()
	var balances map[string]string
	err = processPrivateApiResponse(res, &balances)
	if err != nil {
		return 0.0, err
	}
	usdBal, err := strconv.ParseFloat(balances["ZUSD"], 64)
	if err != nil {
		return 0.0, err
	}
	return usdBal, nil
}
