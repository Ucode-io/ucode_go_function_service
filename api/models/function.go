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

// PushMicrofrontendChangesRequest is sent from the API gateway to push
// AI-edited files to the u-gen branch of an existing microfrontend repo.
// RepoID is the numeric GitLab project ID stored on the function record.
// GithubRepoName is optional — when set, the promote handler also pushes all
// files to the user's GitHub repository (creating it first if it doesn't exist).
type PushMicrofrontendChangesRequest struct {
	RepoID                int                `json:"repo_id"`
	Files                 []GitlabFileChange `json:"files"`
	GithubRepoName        string             `json:"github_repo_name"`
	CommitMessage         string             `json:"commit_message"`
	FunctionID            string             `json:"function_id"`
	CompanyProjectID      string             `json:"company_project_id"`
	CompanyEnvironmentID  string             `json:"company_environment_id"`
	ResourceEnvironmentID string             `json:"resource_environment_id"`
}

// PublishAiMicroFrontendRequest is sent from the API gateway when the AI generates
// a new project. The handler creates the microfrontend, then pushes all files to
// the u-gen branch (NOT master — master is reserved for pipeline triggers).
type PublishAiMicroFrontendRequest struct {
	ProjectId     string                    `json:"project_id"`
	EnvironmentId string                    `json:"environment_id"`
	Name          string                    `json:"name"`
	Path          string                    `json:"path"`
	FrameworkType string                    `json:"framework_type"`
	Files         []GitlabFileChange `json:"files"`
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
