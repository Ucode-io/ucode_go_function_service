package gitlab

import "time"

type (
	IntegrationData struct {
		GitlabProjectId        int
		GitlabIntegrationUrl   string
		GitlabIntegrationToken string
		GitlabGroupId          int
	}

	GitlabIntegrationResponse struct {
		Code    int            `json:"code"`
		Message map[string]any `json:""`
	}

	CreateProject struct {
		NamespaceID          int    `json:"namespace_id"`
		Name                 string `json:"name"`
		InitializeWithReadme bool   `json:"initialize_with_readme"`
		DefaultBranch        string `json:"default_branch"`
		Visibility           string `json:"visibility"`
		Path                 string `json:"path"`
	}

	ForkResponse struct {
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

	GitLabTokenRequest struct {
		ClinetId     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		RefreshToken string `json:"refresh_token"`
	}

	GitLabTokenResponse struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		CreatedAt    int64  `json:"created_at"`
	}

	WebhookConfig struct {
		ProjectUrl    string `json:"project_url"`
		BaseUrl       string `json:"base_url"`
		Token         string `json:"token"`
		ResourceId    string `json:"resource_id"`
		EnvironmentId string `json:"environment_id"`
		RepoId        int64  `json:"repo_id"`
		ProjectId     string `json:"project_id"`
	}

	Webhook struct {
		ID  int    `json:"id"`
		URL string `json:"url"`
	}

	WebhookRequest struct {
		URL                      string `json:"url"`
		PushEvents               bool   `json:"push_events"`
		MergeRequestsEvents      bool   `json:"merge_requests_events"`
		TagPushEvents            bool   `json:"tag_push_events"`
		EnableSSLVerification    bool   `json:"enable_ssl_verification"`
		ConfidentialNoteEvents   bool   `json:"confidential_note_events"`
		ConfidentialIssuesEvents bool   `json:"confidential_issues_events"`
		IssuesEvents             bool   `json:"issues_events"`
		NoteEvents               bool   `json:"note_events"`
		PipelineEvents           bool   `json:"pipeline_events"`
		WikiPageEvents           bool   `json:"wiki_page_events"`
	}
)

type ImportData struct {
	PersonalAccessToken string `json:"personal_access_token"`
	RepoId              string `json:"repo_id"`
	TargetNamespace     string `json:"target_namespace"`
	NewName             string `json:"new_name"`
	GitlabToken         string `json:"gitlab_token"`
	SourceFullPath      string `json:"source_full_path"`
	GitlabGroupId       int    `json:"-"`
}

type ImportResponse struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	FullPath string `json:"full_path"`
	FullName string `json:"full_name"`
	RefsURL  string `json:"refs_url"`
}

type CommitAction struct {
	Action   string `json:"action"`
	FilePath string `json:"file_path"`
	Content  string `json:"content,omitempty"`
	Encoding string `json:"encoding,omitempty"`
}

type CommitRequest struct {
	Branch        string         `json:"branch"`
	CommitMessage string         `json:"commit_message"`
	Actions       []CommitAction `json:"actions"`
}
