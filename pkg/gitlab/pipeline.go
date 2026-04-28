package gitlab

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"ucode/ucode_go_function_service/api/status_http"
)

func CreatePipeline(cfg IntegrationData, data map[string]any) (response GitlabIntegrationResponse, err error) {
	var (
		projectId    = cfg.GitlabProjectId
		strProjectId = strconv.Itoa(projectId)
	)

	if _, ok := data["ref"]; !ok {
		data["ref"] = "master"
	}

	resp, err := DoRequest(
		cfg.GitlabIntegrationUrl+"/api/v4/projects/"+strProjectId+"/pipeline",
		cfg.GitlabIntegrationToken,
		"POST",
		data,
	)
	if err != nil {
		return GitlabIntegrationResponse{}, err
	}

	if resp.Code >= 500 {
		return GitlabIntegrationResponse{}, errors.New(status_http.InternalServerError.Description)
	}

	if resp.Code >= 400 {
		msgBytes, _ := json.Marshal(resp.Message)
		return GitlabIntegrationResponse{}, fmt.Errorf("gitlab error %d: %s", resp.Code, string(msgBytes))
	}

	return resp, nil
}
