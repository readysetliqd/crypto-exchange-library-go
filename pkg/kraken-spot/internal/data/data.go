package data

import "encoding/json"

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
	Aclass          string  `json:"aclass"`
	Altname         string  `json:"altname"`
	Decimals        uint8   `json:"decimals"`
	DisplayDecimals uint8   `json:"display_decimals"`
	CollateralValue float32 `json:"collateral_value"`
	Status          string  `json:"status"`
}

type AssetPairInfo struct {
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
	MarginCall  uint8 `json:"margin_call"`
	MarginLevel uint8 `json:"margin_level"`
}

type AssetPairFees struct {
	Fees              [][]float64 `json:"fees"`
	FeesMaker         [][]float64 `json:"fees_maker"`
	FeeVolumeCurrency string      `json:"fee_volume_currency"`
}

type AssetPairLeverage struct {
	LeverageBuy  []uint8 `json:"leverage_buy"`
	LeverageSell []uint8 `json:"leverage_sell"`
}

type TickerInfo struct {
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
