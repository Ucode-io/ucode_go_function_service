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
)
