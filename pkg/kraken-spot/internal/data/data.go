package data

type ServerTime struct {
	UnixTime int    `json:"unixtime"`
	Rfc1123  string `json:"rfc1123"`
}
type SystemStatus struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

type ApiResp struct {
	Error  []string    `json:"error"`
	Result interface{} `json:"result"`
}
