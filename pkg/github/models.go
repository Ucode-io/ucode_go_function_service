package github

type GithubPushRequest struct {
	Token     string
	RepoOwner string
	RepoName  string
	Branch    string
	Commit    string
	Files     []string
	BaseUrl   string
	BaseDir   string
}

type WebhookPayload struct {
	Name   string   `json:"name"`
	Active bool     `json:"active"`
	Events []string `json:"events"`
	Config Config   `json:"config"`
}

type Config struct {
	URL           string `json:"url"`
	ContentType   string `json:"content_type"`
	Secret        string `json:"secret"`
	FrameworkType string `json:"framework_type"`
	Branch        string `json:"branch"`
	FunctionType  string `json:"type"`
	Resource      string `json:"resource_id"`
	Name          string `json:"provided_name"`
}

type ListWebhookRequest struct {
	Username    string `json:"username"`
	RepoName    string `json:"repo_name"`
	GithubToken string `json:"github_token"`
	ProjectUrl  string `json:"project_url"`
}

type CreateWebhookRequest struct {
	Username      string `json:"username" binding:"required"`
	RepoName      string `json:"repo_name" binding:"required"`
	WebhookSecret string `json:"secret"`
	GithubToken   string `json:"github_token"`
	FrameworkType string `json:"framework_type"`
	Branch        string `json:"branch"`
	FunctionType  string `json:"type"`
	ProjectUrl    string `json:"project_url"`
	ResourceId    string `json:"resource_id"`
	Name          string `json:"provided_name"`
	ProjectId     string `json:"project_id"`
	EnvironmentId string `json:"environment_id"`
}

type ImportResponse struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	FullPath string `json:"full_path"`
	FullName string `json:"full_name"`
	RefsURL  string `json:"refs_url"`
}

type ImportData struct {
	PersonalAccessToken string `json:"personal_access_token"`
	RepoId              string `json:"repo_id"`
	TargetNamespace     string `json:"target_namespace"`
	NewName             string `json:"new_name"`
	GitlabToken         string `json:"gitlab_token"`
}

type Pipeline struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
}

type PipelineLogResponse struct {
	JobName string `json:"job_name"`
	Log     string `json:"log"`
}

type Job struct {
	Id     int    `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}
