package status_http

// Response ...
type Response struct {
	Status        string `json:"status"`
	Description   string `json:"description"`
	Data          any    `json:"data"`
	CustomMessage any    `json:"custom_message"`
}
