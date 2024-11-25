package status_http

// Response ...
type Response struct {
	Status        string      `json:"status"`
	Description   string      `json:"description"`
	Data          interface{} `json:"data"`
	CustomMessage interface{} `json:"custom_message"`
}
