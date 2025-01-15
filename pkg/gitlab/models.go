package gitlab

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
