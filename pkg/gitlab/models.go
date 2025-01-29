package gitlab

import "time"

type IntegrationData struct {
	GitlabProjectId        int
	GitlabIntegrationUrl   string
	GitlabIntegrationToken string
	GitlabGroupId          int
}

type GitlabIntegrationResponse struct {
	Code    int            `json:"code"`
	Message map[string]any `json:""`
}

type CreateProject struct {
	NamespaceID          int    `json:"namespace_id"`
	Name                 string `json:"name"`
	InitializeWithReadme bool   `json:"initialize_with_readme"`
	DefaultBranch        string `json:"default_branch"`
	Visibility           string `json:"visibility"`
	Path                 string `json:"path"`
}

type ForkResponse struct {
	Code              int       `json:"code"`
	ID                int       `json:"id"`
	Name              string    `json:"name"`
	NameWithNamespace string    `json:"name_with_namespace"`
	Path              string    `json:"path"`
	PathWithNamespace string    `json:"path_with_namespace"`
	CreatedAt         time.Time `json:"created_at"`
	DefaultBranch     string    `json:"default_branch"`
	Message           struct {
		ProjectNamespaceName []string `json:"project_namespace.name"`
		Name                 []string `json:"name"`
		Path                 []string `json:"path"`
	} `json:"message"`
}
