package data

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

type AssetPair struct {
	Altname            string      `json:"altname"`
	Wsname             string      `json:"wsname"`
	AclassBase         string      `json:"aclass_base"`
	Base               string      `json:"base"`
	AclassQuote        string      `json:"aclass_quote"`
	Quote              string      `json:"quote"`
	PairDecimals       uint8       `json:"pair_decimals"`
	CostDecimals       uint8       `json:"cost_decimals"`
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
