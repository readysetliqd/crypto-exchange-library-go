package data

type ResponseType struct {
	ServerTime struct {
		UnixTime int    `json:"unixtime"`
		Rfc1123  string `json:"rfc1123"`
	}
	SystemStatus struct {
		Status    string `json:"status"`
		Timestamp string `json:"timestamp"`
	}
}

type ApiResp struct {
	Error  []string     `json:"error"`
	Result ResponseType `json:"result"`
}
