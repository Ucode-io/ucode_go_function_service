package gitlab

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	status "ucode/ucode_go_function_service/api/status_http"
	"ucode/ucode_go_function_service/config"
	"ucode/ucode_go_function_service/pkg/github"
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

	fmt.Println("RefreshGitLabTokenBODY", string(requestBody))

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

	fmt.Println("RefreshGitLabToken", string(body))

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

func ImportFromGitlabCom(cfg ImportData) (response github.ImportResponse, err error) {
	gitlabBodyJSON, err := json.Marshal(cfg)
	if err != nil {
		return github.ImportResponse{}, errors.New("failed to marshal JSON")
	}

	fmt.Println("gitlabBodyJSON", string(gitlabBodyJSON))

	gitlabUrl := "https://gitlab.udevs.io/api/v4/import/gitlab"
	req, err := http.NewRequest(http.MethodPost, gitlabUrl, bytes.NewBuffer(gitlabBodyJSON))
	if err != nil {
		return github.ImportResponse{}, errors.New("failed to create request")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("PRIVATE-TOKEN", cfg.GitlabToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return github.ImportResponse{}, errors.New("failed to send request")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return github.ImportResponse{}, errors.New("failed to read response body")
	}

	fmt.Println("ImportFromGitlabComBODY", string(respBody))
	fmt.Println("ImportFromGitlabComStatus", resp.StatusCode)

	var importResponse github.ImportResponse

	if err = json.Unmarshal(respBody, &importResponse); err != nil {
		return github.ImportResponse{}, errors.New("failed to unmarshal response body")
	}

	return importResponse, nil
}

func IsExpired(createdAt int64, expiresIn int32) bool {
	createdTime := time.Unix(createdAt, 0) // Convert Unix timestamp to time.Time
	expirationTime := createdTime.Add(time.Duration(expiresIn) * time.Second)
	return time.Now().After(expirationTime)
}

func ImportFromGitLab(cfg ImportData) (response github.ImportResponse, err error) {
	fmt.Println("ImportFromGitLab NEW", cfg)
	err = ExportRepo(cfg)
	if err != nil {
		return github.ImportResponse{}, err
	}

	fmt.Println("here again1")

	err = checkExportStatusWithTimeout("https://gitlab.com/api/v4/projects", cfg.RepoId, cfg.PersonalAccessToken)
	if err != nil {
		return github.ImportResponse{}, err
	}

	fmt.Println("PersonalAccessToken", cfg.PersonalAccessToken)
	err = exportProject(fmt.Sprintf("https://gitlab.com/api/v4/projects/%v/export/download", cfg.RepoId), cfg.PersonalAccessToken, cfg.NewName+".tar.gz")
	if err != nil {
		return github.ImportResponse{}, err
	}

	fmt.Println("here again2")

	url := "https://gitlab.udevs.io/api/v4/projects/import"

	importResponse, err := ImportGitLabProject(cfg.GitlabToken, url, fmt.Sprintf("%d", cfg.GitlabGroupId), cfg.NewName, cfg.NewName+".tar.gz")
	if err != nil {
		return github.ImportResponse{}, err
	}

	//remove file
	err = os.Remove(cfg.NewName + ".tar.gz")
	if err != nil {
		return github.ImportResponse{}, err
	}

	// Parse response into ImportResponse struct

	return importResponse, nil
}

func ExportRepo(cfg ImportData) error {
	fmt.Println("export repo id", cfg.RepoId)
	url := fmt.Sprintf("https://gitlab.com/api/v4/projects/%s/export", cfg.RepoId)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		fmt.Println("Error Create request:", err)
		return err
	}

	// Set Authorization Header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.PersonalAccessToken))

	// Send Request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error Do request:", err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		fmt.Println("Error Status code:", resp.StatusCode)
		return errors.New("failed to export repository")
	}

	// Read Response
	return nil
}

func exportProject(exportURL, gitlabToken, exportFilePath string) error {
	req, err := http.NewRequest(http.MethodGet, exportURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+gitlabToken)

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error Do request:", err)
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Error Status code:", resp.StatusCode)
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	file, err := os.Create(exportFilePath)
	if err != nil {
		fmt.Println("Error Create file:", err)
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		fmt.Println("Error Copy:", err)
		return fmt.Errorf("failed to write to file: %w", err)
	}

	fmt.Println("Project exported successfully to", exportFilePath)
	return nil
}

func ImportGitLabProject(token, url, namespace, path, filePath string) (github.ImportResponse, error) {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return github.ImportResponse{}, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	// Create a buffer to store multipart form data
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add namespace and path fields
	_ = writer.WriteField("namespace", namespace)
	_ = writer.WriteField("path", path)

	// Add file to the form
	part, err := writer.CreateFormFile("file", filePath)
	if err != nil {
		return github.ImportResponse{}, fmt.Errorf("error creating form file: %w", err)
	}

	// Copy file content to multipart writer
	_, err = io.Copy(part, file)
	if err != nil {
		return github.ImportResponse{}, fmt.Errorf("error copying file content: %w", err)
	}

	// Close the writer to finalize the form
	writer.Close()

	// Create the request
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return github.ImportResponse{}, fmt.Errorf("error creating request: %w", err)
	}

	// Set headers
	req.Header.Set("PRIVATE-TOKEN", token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return github.ImportResponse{}, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return github.ImportResponse{}, fmt.Errorf("failed to import project, status: %s", resp.Status)
	}

	respByte, err := io.ReadAll(resp.Body)
	if err != nil {
		return github.ImportResponse{}, err
	}

	var importResponse github.ImportResponse
	if err = json.Unmarshal(respByte, &importResponse); err != nil {
		return github.ImportResponse{}, errors.New("failed to unmarshal response body")
	}

	fmt.Println("Project imported successfully!")
	return importResponse, nil
}

func checkExportStatusWithTimeout(baseURL, projectID, exportToken string) error {
	url := fmt.Sprintf("%s/%s/export", baseURL, projectID)
	startTime := time.Now()

	maxWait := 30 * time.Minute

	for {
		if time.Since(startTime) > maxWait {
			return fmt.Errorf("export timed out after %v", maxWait)
		}

		req, _ := http.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+exportToken)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		var result map[string]string
		json.NewDecoder(resp.Body).Decode(&result)

		status := result["export_status"]
		fmt.Println("Export status:", status)

		if status == "finished" {
			return nil
		}

		time.Sleep(10 * time.Second) // Check every 10s
	}
}
