package models

type CommonMessage struct {
	Data     map[string]interface{} `json:"data"`
	IsCached bool                   `json:"is_cached"`
}
