package models

type CommonMessage struct {
	Data     map[string]any `json:"data"`
	IsCached bool           `json:"is_cached"`
}
