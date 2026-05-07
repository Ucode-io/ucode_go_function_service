package models

import "time"

type (
	GitlabProjectResponse []struct {
		Id                int       `json:"id"`
		Name              string    `json:"name"`
		NameWithNamespace string    `json:"name_with_namespace"`
		Path              string    `json:"path"`
		PathWithNamespace string    `json:"path_with_namespace"`
		CreatedAt         time.Time `json:"created_at"`
		DefaultBranch     string    `json:"default_branch"`
		Namespace         struct {
			ID       int    `json:"id"`
			Name     string `json:"name"`
			Path     string `json:"path"`
			Kind     string `json:"kind"`
			FullPath string `json:"full_path"`
			WebURL   string `json:"web_url"`
		} `json:"namespace"`
	}

	GitlabBranch []struct {
		Name string `json:"name"`
	}

	GitlabUser struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
		Name     string `json:"name"`
	}

	GitlabFileChange struct {
		FilePath string `json:"file_path"`
		Content  string `json:"content"`
	}

	GitlabUpdateFileRequest struct {
		Files  []GitlabFileChange `json:"files"`
		Branch string             `json:"branch"`
	}

	GitlabCommit struct {
		ID            string `json:"id"`
		ShortID       string `json:"short_id"`
		Title         string `json:"title"`
		Message       string `json:"message"`
		AuthorName    string `json:"author_name"`
		AuthorEmail   string `json:"author_email"`
		AuthoredDate  string `json:"authored_date"`
		CommitterName string `json:"committer_name"`
		CommittedDate string `json:"committed_date"`
		WebURL        string `json:"web_url"`
	}

	RevertMicrofrontendRequest struct {
		RepoID     string `json:"repo_id"    binding:"required"`
		CommitSHA  string `json:"commit_sha" binding:"required"`
		FunctionID string `json:"function_id"`
	}
)
