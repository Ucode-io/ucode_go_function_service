package models

type (
	GihubLogin struct {
		AccessToken string `json:"access_token"`
		Scope       string `json:"scope"`
		TokenType   string `json:"token_type"`
	}

	GithubUser struct {
		Status            string `json:"status"`
		Login             string `json:"login"`
		ID                int    `json:"id"`
		NodeID            string `json:"node_id"`
		AvatarURL         string `json:"avatar_url"`
		GravatarID        string `json:"gravatar_id"`
		URL               string `json:"url"`
		HTMLURL           string `json:"html_url"`
		FollowersURL      string `json:"followers_url"`
		FollowingURL      string `json:"following_url"`
		GistsURL          string `json:"gists_url"`
		StarredURL        string `json:"starred_url"`
		SubscriptionsURL  string `json:"subscriptions_url"`
		OrganizationsURL  string `json:"organizations_url"`
		ReposURL          string `json:"repos_url"`
		EventsURL         string `json:"events_url"`
		ReceivedEventsURL string `json:"received_events_url"`
		Type              string `json:"type"`
		UserViewType      string `json:"user_view_type"`
		SiteAdmin         bool   `json:"site_admin"`
		Name              string `json:"name"`
		Company           any    `json:"company"`
		Blog              string `json:"blog"`
		Location          any    `json:"location"`
		Email             any    `json:"email"`
		Hireable          any    `json:"hireable"`
		Bio               any    `json:"bio"`
	}

	GithubRepo []struct {
		ID       int    `json:"id"`
		NodeID   string `json:"node_id"`
		Name     string `json:"name"`
		FullName string `json:"full_name"`
		Private  bool   `json:"private"`
	}

	GithubBranch []struct {
		Name          string `json:"name"`
		Protected     bool   `json:"protected"`
		ProtectionURL string `json:"protection_url"`
	}

	CreateWebhook struct {
		Username      string `json:"username"`
		RepoName      string `json:"repo_name" binding:"required"`
		Branch        string `json:"branch"`
		FrameworkType string `json:"framework_type"`
		GithubToken   string `json:"github_token"`
		FunctionType  string `json:"type"`
		Resource      string `json:"resource_id"`
		Name          string `json:"provided_name"`
	}
)
