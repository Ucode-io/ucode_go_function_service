package models

type AuthData struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}
