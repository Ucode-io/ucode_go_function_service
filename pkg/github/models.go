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
