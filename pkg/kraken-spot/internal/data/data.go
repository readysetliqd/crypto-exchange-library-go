package data

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type ApiResp struct {
	Error  []string    `json:"error"`
	Result interface{} `json:"result"`
}

type ServerTime struct {
	UnixTime int    `json:"unixtime"`
	Rfc1123  string `json:"rfc1123"`
}

type SystemStatus struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

type AssetInfo struct {
	Ticker          string
	Aclass          string  `json:"aclass"`
	Altname         string  `json:"altname"`
	Decimals        uint8   `json:"decimals"`
	DisplayDecimals uint8   `json:"display_decimals"`
	CollateralValue float32 `json:"collateral_value"`
	Status          string  `json:"status"`
}

type AssetPairInfo struct {
	Ticker             string
	Altname            string      `json:"altname"`
	Wsname             string      `json:"wsname"`
	AclassBase         string      `json:"aclass_base"`
	Base               string      `json:"base"`
	AclassQuote        string      `json:"aclass_quote"`
	Quote              string      `json:"quote"`
	CostDecimals       uint8       `json:"cost_decimals"`
	PairDecimals       uint8       `json:"pair_decimals"`
	LotDecimals        uint8       `json:"lot_decimals"`
	LotMultiplier      uint8       `json:"lot_multiplier"`
	LeverageBuy        []uint8     `json:"leverage_buy"`
	LeverageSell       []uint8     `json:"leverage_sell"`
	Fees               [][]float64 `json:"fees"`
	FeesMaker          [][]float64 `json:"fees_maker"`
	FeeVolumeCurrency  string      `json:"fee_volume_currency"`
	MarginCall         uint8       `json:"margin_call"`
	MarginStop         uint8       `json:"margin_stop"`
	OrderMin           string      `json:"ordermin"`
	CostMin            string      `json:"costmin"`
	TickSize           string      `json:"tick_size"`
	Status             string      `json:"status"`
	LongPositionLimit  uint32      `json:"long_position_limit"`
	ShortPositionLimit uint32      `json:"short_position_limit"`
}

type AssetPairMargin struct {
	Ticker      string
	MarginCall  uint8 `json:"margin_call"`
	MarginLevel uint8 `json:"margin_level"`
}

type AssetPairFees struct {
	Ticker            string
	Fees              [][]float64 `json:"fees"`
	FeesMaker         [][]float64 `json:"fees_maker"`
	FeeVolumeCurrency string      `json:"fee_volume_currency"`
}

type AssetPairLeverage struct {
	Ticker       string
	LeverageBuy  []uint8 `json:"leverage_buy"`
	LeverageSell []uint8 `json:"leverage_sell"`
}

type TickerInfo struct {
	Ticker          string
	Ask             TickerBookInfo      `json:"a"`
	Bid             TickerBookInfo      `json:"b"`
	LastTradeClosed TickerLastTradeInfo `json:"c"`
	Volume          TickerDailyInfo     `json:"v"`
	VWAP            TickerDailyInfo     `json:"p"`
	NumberOfTrades  TickerDailyInfoInt  `json:"t"`
	Low             TickerDailyInfo     `json:"l"`
	High            TickerDailyInfo     `json:"h"`
	Open            string              `json:"o"`
}

type TickerBookInfo struct {
	Price          string
	WholeLotVolume string
	LotVolume      string
}

type TickerLastTradeInfo struct {
	Price     string
	LotVolume string
}

type TickerDailyInfo struct {
	Today       string
	Last24Hours string
}

type TickerDailyInfoInt struct {
	Today       int
	Last24Hours int
}

func (pi *TickerBookInfo) UnmarshalJSON(data []byte) error {
	var v []string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if len(v) >= 3 {
		pi.Price = v[0]
		pi.WholeLotVolume = v[1]
		pi.LotVolume = v[2]
	}
	return nil
}

func (pi *TickerLastTradeInfo) UnmarshalJSON(data []byte) error {
	var v []string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if len(v) >= 2 {
		pi.Price = v[0]
		pi.LotVolume = v[1]
	}
	return nil
}

func (pi *TickerDailyInfo) UnmarshalJSON(data []byte) error {
	var v []string
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if len(v) >= 2 {
		pi.Today = v[0]
		pi.Last24Hours = v[1]
	}
	return nil
}

func (pi *TickerDailyInfoInt) UnmarshalJSON(data []byte) error {
	var v []int
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if len(v) >= 2 {
		pi.Today = v[0]
		pi.Last24Hours = v[1]
	}
	return nil
}

type TickerVolume struct {
	Ticker string
	Volume float64
}

type TickerTrades struct {
	Ticker    string
	NumTrades int
}

type OHLCResp struct {
	Ticker  string
	Data    OHLCDataSlice
	Current OHLCData
	Last    uint64 `json:"last"`
}

type OHLCDataSlice []OHLCData

type OHLCData struct {
	Time   uint64
	Open   string
	High   string
	Low    string
	Close  string
	VWAP   string
	Volume string
	Count  uint32
}

func (pi *OHLCResp) UnmarshalJSON(data []byte) error {
	dataMap := make(map[string]interface{})
	json.Unmarshal(data, &dataMap)
	pi.Last = uint64(dataMap["last"].(float64))
	for key := range dataMap {
		if key != "last" {
			pi.Ticker = key
			tempDataSlice, ok := dataMap[key].([]interface{})
			if !ok {
				return fmt.Errorf("OHLCDataSlice assertion error")
			} else {
				for i, v := range tempDataSlice {
					item, ok := v.([]interface{})
					if !ok {
						return fmt.Errorf("OHLCData item assertion error")
					} else {
						ohlcData := OHLCData{
							Time:   uint64(item[0].(float64)),
							Open:   item[1].(string),
							High:   item[2].(string),
							Low:    item[3].(string),
							Close:  item[4].(string),
							VWAP:   item[5].(string),
							Volume: item[6].(string),
							Count:  uint32(item[7].(float64)),
						}
						if i == len(tempDataSlice)-1 {
							pi.Current = ohlcData
						} else {
							pi.Data = append(pi.Data, ohlcData)
						}
					}
				}
			}
		}
	}
	return nil
}

type OrderBook struct {
	Ticker string
	Asks   []BookEntry `json:"asks"`
	Bids   []BookEntry `json:"bids"`
}

type BookEntry struct {
	Price  string
	Volume string
	Time   uint64
}

func (pi *BookEntry) UnmarshalJSON(data []byte) error {
	var v []interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if len(v) >= 3 {
		pi.Price = v[0].(string)
		pi.Volume = v[1].(string)
		pi.Time = uint64(v[2].(float64))
	}
	return nil
}

type TradesResp struct {
	Ticker string
	Trades TradeSlice
	Last   float64 `json:"last"`
}

type TradeSlice []Trade

type Trade struct {
	Price     string
	Volume    string
	Time      float64
	Side      string
	OrderType string
	Misc      string
}

func (pi *TradesResp) UnmarshalJSON(data []byte) error {
	var err error
	dataMap := make(map[string]interface{})
	json.Unmarshal(data, &dataMap)
	lastStr, ok := dataMap["last"].(string)
	if !ok {
		err = fmt.Errorf("error asserting 'last' to string")
		return err
	}
	pi.Last, err = strconv.ParseFloat(lastStr, 64)
	if err != nil {
		return err
	}
	for key, data := range dataMap {
		if key != "last" {
			pi.Ticker = key
			tradeData, ok := data.([]interface{})
			if !ok {
				err = fmt.Errorf("error asserting 'data' to TradeSlice")
				return err
			}
			trades := make(TradeSlice, len(tradeData))
			for i, td := range tradeData {
				tradeInfo, ok := td.([]interface{})
				if !ok || len(tradeInfo) < 6 {
					err = fmt.Errorf("error asserting 'tradeData' to []interface{} or not enough data")
					return err
				}
				trades[i] = Trade{
					Price:     tradeInfo[0].(string),
					Volume:    tradeInfo[1].(string),
					Time:      tradeInfo[2].(float64),
					Side:      tradeInfo[3].(string),
					OrderType: tradeInfo[4].(string),
					Misc:      tradeInfo[5].(string),
				}
			}
			pi.Trades = trades
			break
		}
	}
	return nil
}
