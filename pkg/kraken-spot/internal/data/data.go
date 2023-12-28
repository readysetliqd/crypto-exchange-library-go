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
	Assets []AssetInfoAsset
}

type AssetInfoAsset struct {
	Aclass          string
	Altname         string
	Decimals        uint8
	DisplayDecimals uint8
	CollateralValue float32
	Status          string
}
