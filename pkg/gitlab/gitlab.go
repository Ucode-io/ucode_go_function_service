package gitlab

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"
	status "ucode/ucode_go_function_service/api/status_http"
	"ucode/ucode_go_function_service/config"
)

func CreateProjectFork(projectName string, data IntegrationData) (response ForkResponse, err error) {
	var (
		projectId    = data.GitlabProjectId
		strProjectId = strconv.Itoa(projectId)
		resp         ForkResponse
	)

	respByte, err := DoRequestV1(data.GitlabIntegrationUrl+"/api/v4/projects/"+strProjectId+"/fork", data.GitlabIntegrationToken, http.MethodPost, CreateProject{
		NamespaceID:          data.GitlabGroupId,
		Name:                 projectName,
		Path:                 projectName,
		InitializeWithReadme: true,
		DefaultBranch:        "master",
		Visibility:           "private",
	})
	if err != nil {
		return
	}

	if err = json.Unmarshal(respByte, &resp); err != nil {
		return
	}

	if len(resp.Message.Name) != 0 {
		return ForkResponse{}, errors.New(resp.Message.Name[0])
	}

	return resp, err
}

func DoRequest(url, token string, method string, body any) (responseModel GitlabIntegrationResponse, err error) {
	data, err := json.Marshal(&body)
	if err != nil {
		return
	}

	client := &http.Client{
		Timeout: time.Duration(15 * time.Second),
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

	var emptyMap = make(map[string]any)

	if err = json.Unmarshal(respByte, &emptyMap); err != nil {
		return GitlabIntegrationResponse{}, err
	}

	responseModel.Message = emptyMap
	responseModel.Code = resp.StatusCode

	return
}

func DoRequestV1(url, token string, method string, body any) ([]byte, error) {
	data, err := json.Marshal(&body)
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Timeout: time.Duration(15 * time.Second),
	}

	url += "?access_token=" + token

	req, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	respByte, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return respByte, nil
}

func UpdateProject(cfg IntegrationData, data map[string]any) (response GitlabIntegrationResponse, err error) {
	var (
		projectId    = cfg.GitlabProjectId
		strProjectId = strconv.Itoa(projectId)
	)

	resp, err := DoRequest(cfg.GitlabIntegrationUrl+"/api/v4/projects/"+strProjectId, cfg.GitlabIntegrationToken, http.MethodPut, data)

	if resp.Code >= 400 {
		return GitlabIntegrationResponse{}, errors.New(status.BadRequest.Description)
	} else if resp.Code >= 500 {
		return GitlabIntegrationResponse{}, errors.New(status.InternalServerError.Description)
	}

	return resp, err
}

func CreateProjectVariable(cfg IntegrationData, data map[string]any) (response GitlabIntegrationResponse, err error) {
	var (
		projectId    = cfg.GitlabProjectId
		strProjectId = strconv.Itoa(projectId)
	)

	resp, err := DoRequest(cfg.GitlabIntegrationUrl+"/api/v4/projects/"+strProjectId+"/variables", cfg.GitlabIntegrationToken, http.MethodPost, data)

	if resp.Code >= 400 {
		return GitlabIntegrationResponse{}, errors.New(status.BadRequest.Description)
	} else if resp.Code >= 500 {
		return GitlabIntegrationResponse{}, errors.New(status.InternalServerError.Description)
	}

	return resp, err
}

func MakeGitLabRequest(method, url string, payload map[string]any, token string) (map[string]any, error) {
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

	var result map[string]any
	err = json.Unmarshal(respBody, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func DeleteForkedProject(repoName string, cfg config.Config) (response GitlabIntegrationResponse, err error) {
	resp, _ := DoRequest(cfg.GitlabIntegrationURL+"/api/v4/projects/ucode_functions_group%2"+"F"+repoName, cfg.GitlabTokenMicroFront, http.MethodDelete, nil)

	if resp.Code >= 400 {
		return GitlabIntegrationResponse{}, errors.New(status.BadRequest.Description)
	} else if resp.Code >= 500 {
		return GitlabIntegrationResponse{}, errors.New(status.InternalServerError.Description)
	}

	return GitlabIntegrationResponse{
		Code:    200,
		Message: map[string]any{"message": "Successfully deleted"},
	}, nil
}
