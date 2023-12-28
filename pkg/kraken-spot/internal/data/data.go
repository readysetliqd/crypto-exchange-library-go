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
