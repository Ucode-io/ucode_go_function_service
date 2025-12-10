package models

type AuthData struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

type ApiKey struct {
	AppId         string `json:"app_id"`
	IsPublic      bool   `json:"is_public"`
	FunctionId    string `json:"function_id"`
	ResourceType  int    `json:"resource_type"`
	ResourceEnvId string `json:"resource_env_id"`
}
