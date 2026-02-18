package models

import (
	nb "ucode/ucode_go_function_service/genproto/new_object_builder_service"
	obs "ucode/ucode_go_function_service/genproto/object_builder_service"
)

type PublishMcpProjectFront struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ResourceId  string `json:"resource_id"`
	Type        string `json:"type"`
}

type CreateFunctionRequest struct {
	Path             string `json:"path"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	FunctionFolderId string `json:"function_folder_id"`
	FrameworkType    string `json:"framework_type"`
	Branch           string `json:"branch"`
	RepoName         string `json:"repo_name"`
	ResourceId       string `json:"resource_id"`
	Type             string `json:"type"`
}

type Function struct {
	ID               string `json:"id"`
	Path             string `json:"path"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	FuncitonFolderId string `json:"function_folder_id"`
}

type InvokeFunctionResponse struct {
	Status      string         `json:"status"`
	Data        map[string]any `json:"data"`
	Attributes  map[string]any `json:"attributes"`
	ServerError string         `json:"server_error"`
	Size        int64          `json:"-"`
}

type NewInvokeFunctionRequest struct {
	Data map[string]any `json:"data"`
}

type InvokeFunctionRequest struct {
	FunctionID string   `json:"function_id"`
	ObjectIDs  []string `json:"object_ids"`
	Attributes map[string]any
	TableSlug  string `json:"table_slug"`
}

type DeployFunctionRequest struct {
	GithubToken     string
	RepoId          string
	ResourceType    string
	TargetNamespace string
	IsGitlab        bool
	SourcheFullPath string
	Function        *obs.Function
	GitlabBaseURL   string
}

type DeployFunctionRequestGo struct {
	GithubToken     string
	RepoId          string
	ResourceType    string
	TargetNamespace string
	IsGitlab        bool
	SourcheFullPath string
	Function        *nb.Function
	GitlabBaseURL   string
}
