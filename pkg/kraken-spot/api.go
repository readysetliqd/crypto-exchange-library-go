package krakenspot

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/readysetliqd/crypto-exchange-library-go/pkg/kraken-spot/internal/data"
)

func GetServerTime() {
	url := data.PublicApiUrl + "Time"
	res, err := http.Get(url)
	if err != nil {
		log.Println("error getting response from url | ", err, url)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		resp := data.ApiResp{}
		msg, err := io.ReadAll(res.Body)
		if err != nil {
			log.Println("error calling io.readall | ", err)
			return
		}

		err = json.Unmarshal(msg, &resp)
		if err != nil {
			log.Println("error unmarshaling msg to json | ", err)
		}
		if len(resp.Error) != 0 {
			log.Println(resp.Error)
		} else {
			log.Println(resp.Result.(*data.ServerTime))
		}
	} else {
		log.Println("http status code not OK status code | ", res.StatusCode)
	}
}

func GetSystemStatus() {
	url := data.PublicApiUrl + "SystemStatus"
	res, err := http.Get(url)
	if err != nil {
		log.Println("error getting response from url | ", err, url)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusOK {
		resp := data.ApiResp{}
		msg, err := io.ReadAll(res.Body)
		if err != nil {
			log.Println("error calling io.readall | ", err)
			return
		}

		err = json.Unmarshal(msg, &resp)
		if err != nil {
			log.Println("error unmarshaling msg to json | ", err)
		}
		if len(resp.Error) != 0 {
			log.Println(resp.Error)
		} else {
			log.Println(resp.Result.(*data.SystemStatus))
		}
	} else {
		log.Println("http status code not OK status code | ", res.StatusCode)
	}
}
