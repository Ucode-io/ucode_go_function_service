package gitlab

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
	"ucode/ucode_go_function_service/api/status_http"
	"ucode/ucode_go_function_service/pkg/helper"
)

func CreateProjectFork(projectName string, data IntegrationData) (response GitlabIntegrationResponse, err error) {
	var (
		projectId    = data.GitlabProjectId
		strProjectId = strconv.Itoa(projectId)
	)

	resp, err := DoRequest(data.GitlabIntegrationUrl+"/api/v4/projects/"+strProjectId+"/fork", data.GitlabIntegrationToken, "POST", CreateProject{
		NamespaceID:          data.GitlabGroupId,
		Name:                 projectName,
		Path:                 projectName,
		InitializeWithReadme: true,
		DefaultBranch:        "master",
		Visibility:           "private",
	})

	if resp.Code >= 400 {
		return GitlabIntegrationResponse{}, errors.New(status_http.BadRequest.Description)
	} else if resp.Code >= 500 {
		return GitlabIntegrationResponse{}, errors.New(status_http.InternalServerError.Description)
	}

	return resp, err
}

func DoRequest(url, token string, method string, body interface{}) (responseModel GitlabIntegrationResponse, err error) {
	data, err := json.Marshal(&body)
	if err != nil {
		return
	}

	client := &http.Client{
		Timeout: time.Duration(5 * time.Second),
	}

	url += "?access_token=" + token

	req, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	respByte, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var emptyMap = make(map[string]interface{})

	if err = json.Unmarshal(respByte, &emptyMap); err != nil {
		return GitlabIntegrationResponse{}, err
	}

	responseModel.Message = emptyMap
	responseModel.Code = resp.StatusCode

	return
}

func UpdateProject(cfg IntegrationData, data map[string]interface{}) (response GitlabIntegrationResponse, err error) {
	// create repo in given group by existing project in gitlab
	projectId := cfg.GitlabProjectId
	strProjectId := strconv.Itoa(projectId)

	resp, err := DoRequest(cfg.GitlabIntegrationUrl+"/api/v4/projects/"+strProjectId, cfg.GitlabIntegrationToken, http.MethodPut, data)

	if resp.Code >= 400 {
		return GitlabIntegrationResponse{}, errors.New(status_http.BadRequest.Description)
	} else if resp.Code >= 500 {
		return GitlabIntegrationResponse{}, errors.New(status_http.InternalServerError.Description)
	}

	return resp, err
}

func CreateProjectVariable(cfg IntegrationData, data map[string]interface{}) (response GitlabIntegrationResponse, err error) {
	// create repo in given group by existing project in gitlab
	projectId := cfg.GitlabProjectId
	strProjectId := strconv.Itoa(projectId)

	resp, err := DoRequest(cfg.GitlabIntegrationUrl+"/api/v4/projects/"+strProjectId+"/variables", cfg.GitlabIntegrationToken, http.MethodPost, data)

	if resp.Code >= 400 {
		return GitlabIntegrationResponse{}, errors.New(status_http.BadRequest.Description)
	} else if resp.Code >= 500 {
		return GitlabIntegrationResponse{}, errors.New(status_http.InternalServerError.Description)
	}

	return resp, err
}

func AddFilesToRepo(gitlabToken string, path string, gitlabRepoId int, branch string) error {
	localFolderPath := "/go/src/gitlab.udevs.io/ucode/ucode_go_admin_api_gateway/github_integration"

	files, err := helper.ListFiles(localFolderPath)
	if err != nil {
		return errors.New("error listing files")
	}

	var actions []map[string]interface{}

	for _, file := range files {
		if file == ".gitlab-ci.yml" {
			continue
		}
		filePath := filepath.Join(localFolderPath, file)
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			return errors.New("failed to read file")
		}

		action := map[string]interface{}{
			"action":    "create",
			"file_path": file,
			"content":   string(fileContent),
		}

		actions = append(actions, action)
	}

	commitURL := fmt.Sprintf("%s/projects/%v/repository/commits", "https://gitlab.udevs.io/api/v4", gitlabRepoId)
	commitPayload := map[string]interface{}{
		"branch":         branch,
		"commit_message": "Added devops files",
		"actions":        actions,
	}

	_, err = MakeGitLabRequest(http.MethodPost, commitURL, commitPayload, gitlabToken)
	if err != nil {
		return errors.New("failed to make GitLab request")
	}

	return nil
}

func MakeGitLabRequest(method, url string, payload map[string]interface{}, token string) (map[string]interface{}, error) {
	reqBody := new(bytes.Buffer)
	if payload != nil {
		json.NewEncoder(reqBody).Encode(payload)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	err = json.Unmarshal(respBody, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
