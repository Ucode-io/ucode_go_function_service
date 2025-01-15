package gitlab

import (
	"errors"
	"strconv"
	"ucode/ucode_go_function_service/api/status_http"
)

func CreatePipeline(cfg IntegrationData, data map[string]any) (response GitlabIntegrationResponse, err error) {
	var (
		projectId    = cfg.GitlabProjectId
		strProjectId = strconv.Itoa(projectId)
	)

	resp, err := DoRequest(cfg.GitlabIntegrationUrl+"/api/v4/projects/"+strProjectId+"/pipeline", cfg.GitlabIntegrationToken+"&"+"ref=master", "POST", data)

	if resp.Code >= 400 {
		return GitlabIntegrationResponse{}, errors.New(status_http.BadRequest.Description)
	} else if resp.Code >= 500 {
		return GitlabIntegrationResponse{}, errors.New(status_http.InternalServerError.Description)
	}

	return resp, err
}
