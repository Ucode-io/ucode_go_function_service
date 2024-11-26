package models

import "ucode/ucode_go_function_service/services"

type CreateVersionHistoryRequest struct {
	Services  services.ServiceManagerI
	NodeType  string
	ProjectId string

	Id               string
	ActionSource     string          `json:"action_source"`
	ActionType       string          `json:"action_type"`
	Previous         interface{}     `json:"previous"`
	Current          interface{}     `json:"current"`
	UsedEnvironments map[string]bool `json:"used_environments"`
	Date             string          `json:"date"`
	UserInfo         string          `json:"user_info"`
	Request          interface{}     `json:"request"`
	Response         interface{}     `json:"response"`
	ApiKey           string          `json:"api_key"`
	Type             string          `json:"type"`
	TableSlug        string          `json:"table_slug"`
	VersionId        string          `json:"version_id"`
	ResourceType     int
}
