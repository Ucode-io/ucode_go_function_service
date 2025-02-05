package gitlab

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	status "ucode/ucode_go_function_service/api/status_http"
	"ucode/ucode_go_function_service/config"
)

// Integration gitlab.udevs.io

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

func MakeGitLabRequest(method, url string, payload map[string]any, token string) (*http.Response, error) {
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

	return resp, nil
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

// Integration gitlab.com

func RefreshGitLabToken(request GitLabTokenRequest) (*GitLabTokenResponse, error) {
	client := &http.Client{}

	requestBody, err := json.Marshal(map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": request.RefreshToken,
		"client_id":     request.ClinetId,
		"client_secret": request.ClientSecret,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, "https://gitlab.com/oauth/token", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tokenResponse GitLabTokenResponse

	if err = json.Unmarshal(body, &tokenResponse); err != nil {
		return nil, err
	}

	return &tokenResponse, nil
}

func ListWebhooks(cfg WebhookConfig) (bool, error) {
	var apiURL = fmt.Sprintf("%s/api/v4/projects/%d/hooks", cfg.BaseUrl, cfg.RepoId)

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("failed to list webhooks")
	}

	var webhooks []Webhook
	err = json.NewDecoder(resp.Body).Decode(&webhooks)
	if err != nil {
		return false, err
	}

	for _, webhook := range webhooks {
		if strings.HasPrefix(webhook.URL, cfg.ProjectUrl) {
			return true, nil
		}
	}

	return false, nil
}

func CreateWebhook(cfg WebhookConfig) error {
	var (
		apiURL  = fmt.Sprintf("%s/api/v4/projects/%d/hooks", cfg.BaseUrl, cfg.RepoId)
		webhook = WebhookRequest{
			URL:                    fmt.Sprintf("%s/v2/webhook/handle?project_id=%s&resource_id=%s&environment_id=%s", cfg.ProjectUrl, cfg.ProjectId, cfg.ResourceId, cfg.EnvironmentId),
			PushEvents:             true,
			MergeRequestsEvents:    true,
			TagPushEvents:          true,
			EnableSSLVerification:  true,
			ConfidentialNoteEvents: false,
			IssuesEvents:           false,
			NoteEvents:             false,
			PipelineEvents:         false,
			WikiPageEvents:         false,
		}
	)

	payload, err := json.Marshal(webhook)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create webhook: %s", string(body))
	}

	return nil
}

func ImportFromGitlabCom(cfg ImportData) (response ImportResponse, err error) {
	gitlabBodyJSON, err := json.Marshal(cfg)
	if err != nil {
		return ImportResponse{}, errors.New("failed to marshal JSON")
	}

	gitlabUrl := "https://gitlab.udevs.io/api/v4/import/gitlab"
	req, err := http.NewRequest(http.MethodPost, gitlabUrl, bytes.NewBuffer(gitlabBodyJSON))
	if err != nil {
		return ImportResponse{}, errors.New("failed to create request")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("PRIVATE-TOKEN", cfg.GitlabToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return ImportResponse{}, errors.New("failed to send request")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ImportResponse{}, errors.New("failed to read response body")
	}

	var importResponse ImportResponse

	if err = json.Unmarshal(respBody, &importResponse); err != nil {
		return ImportResponse{}, errors.New("failed to unmarshal response body")
	}

	return importResponse, nil
}
