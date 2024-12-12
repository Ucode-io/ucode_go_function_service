package github

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func GithubPushFiles(in GithubPushRequest) error {
	err := filepath.WalkDir(in.BaseDir, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relativePath := strings.TrimPrefix(filePath, in.BaseDir+"/")

		if !d.IsDir() {
			if strings.HasPrefix(relativePath, "template") || relativePath == ".gitignore" || relativePath == "gitlab-ci.yml" || relativePath == "Makefile" || relativePath == "README.md" {
				uploadFileToGitHub(in.Token, in.RepoOwner, in.RepoName, filePath, relativePath, in.Branch, in.Commit, in.BaseUrl)
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func uploadFileToGitHub(token, repoOwner, repoName, localFilePath, repoFilePath, branch, commitMessage, githubAPIURL string) error {
	fileContent, err := os.ReadFile(localFilePath) // Read the file content
	if err != nil {
		return err
	}

	var (
		encodedContent = base64.StdEncoding.EncodeToString(fileContent)
		fileURL        = fmt.Sprintf("%s/repos/%s/%s/contents/%s", githubAPIURL, repoOwner, repoName, repoFilePath)
	)

	// Check if the file already exists on GitHub
	req, err := http.NewRequest("GET", fileURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "token "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var existingFile struct {
		SHA string `json:"sha"`
	}

	if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(&existingFile); err != nil {
			return err
		}
	} else if resp.StatusCode != http.StatusNotFound {
		return err
	}

	payload := map[string]interface{}{
		"message": commitMessage,
		"content": encodedContent,
		"branch":  branch,
	}

	if existingFile.SHA != "" {
		payload["sha"] = existingFile.SHA
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err = http.NewRequest("PUT", fileURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Printf("Successfully uploaded %s to GitHub!\n", repoFilePath)
	} else {
		body, err := io.ReadAll(resp.Body) // or io.ReadAll in Go 1.16+
		if err != nil {
			return err
		}
		fmt.Printf("Failed to upload %s. Status: %s, Response: %s\n", repoFilePath, resp.Status, string(body))
	}

	return nil
}

func DoRequest(method, url, token string, payload map[string]interface{}) ([]byte, error) {
	var reqBody = new(bytes.Buffer)

	if payload != nil {
		json.NewEncoder(reqBody).Encode(payload)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return respBody, nil
}

func ImportFromGithub(cfg ImportData) (response ImportResponse, err error) {
	gitlabBodyJSON, err := json.Marshal(cfg)
	if err != nil {
		return ImportResponse{}, errors.New("failed to marshal JSON")
	}

	gitlabUrl := "https://gitlab.udevs.io/api/v4/import/github"
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
	err = json.Unmarshal(respBody, &importResponse)
	if err != nil {
		return ImportResponse{}, errors.New("failed to unmarshal response body")
	}
	return importResponse, nil
}

func AddCiFile(gitlabToken, path string, gitlabRepoId int, branch, localFolderPath string) error {
	filePath := fmt.Sprintf("%v/.gitlab-ci.yml", localFolderPath)

	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return errors.New("failed to read file")
	}

	commitURL := fmt.Sprintf("https://gitlab.udevs.io/api/v4/projects/%v/repository/commits", gitlabRepoId)
	commitPayload := map[string]interface{}{
		"branch":         branch,
		"commit_message": "Added ci file",
		"actions": []map[string]interface{}{
			{
				"action":    "create",
				"file_path": ".gitlab-ci.yml",
				"content":   string(fileContent),
			},
		},
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

func DeleteRepository(token string, projectID int) error {
	apiURL := fmt.Sprintf("%s/projects/%v", "https://gitlab.udevs.io/api/v4", projectID)

	req, err := http.NewRequest(http.MethodDelete, apiURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("PRIVATE-TOKEN", token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func GetLatestPipeline(token, branchName string, projectID int) (*Pipeline, error) {
	apiURL := fmt.Sprintf("%s/projects/%v/pipelines?ref=%s&order_by=id&sort=desc&per_page=1", "https://gitlab.udevs.io/api/v4", projectID, branchName)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("PRIVATE-TOKEN", token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get pipeline. Status code: %d", resp.StatusCode)
	}

	var pipelines []Pipeline
	if err := json.NewDecoder(resp.Body).Decode(&pipelines); err != nil {
		return nil, err
	}

	if len(pipelines) == 0 {
		return nil, fmt.Errorf("no pipelines found for the specified branch")
	}

	return &pipelines[0], nil
}

func GetPipelineLog(repoId, gitlabURL, token string) (PipelineLogResponse, error) {
	url := fmt.Sprintf("%s/api/v4/projects/%v/jobs", gitlabURL, repoId)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return PipelineLogResponse{}, err
	}

	req.Header.Set("PRIVATE-TOKEN", token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return PipelineLogResponse{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return PipelineLogResponse{}, err
	}

	var jobs []Job
	err = json.Unmarshal(body, &jobs)
	if err != nil {
		return PipelineLogResponse{}, err
	}

	for _, job := range jobs {
		url := fmt.Sprintf("%s/api/v4/projects/%v/jobs/%v/trace", gitlabURL, repoId, job.Id)

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return PipelineLogResponse{}, err
		}

		req.Header.Set("PRIVATE-TOKEN", token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return PipelineLogResponse{}, err
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return PipelineLogResponse{}, err
		}

		if job.Status == "failed" {
			pipelineResp := PipelineLogResponse{
				JobName: job.Name,
				Log:     string(body),
			}

			return pipelineResp, err
		}
	}

	return PipelineLogResponse{}, nil
}

func VerifySignature(signatureHeader string, body []byte, secret []byte) bool {
	mac := hmac.New(sha1.New, secret)

	mac.Write(body)

	expectedMAC := mac.Sum(nil)

	signature := signatureHeader[len("sha1="):]

	receivedSignature, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}

	return hmac.Equal(receivedSignature, expectedMAC)
}

func MakeRequest(method, url, token string, payload map[string]interface{}) (map[string]interface{}, error) {
	reqBody := new(bytes.Buffer)
	if payload != nil {
		json.NewEncoder(reqBody).Encode(payload)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

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

func MakeRequestV1(method, url, token string, payload map[string]interface{}) ([]byte, error) {
	var reqBody = new(bytes.Buffer)

	if payload != nil {
		json.NewEncoder(reqBody).Encode(payload)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return respBody, nil
}
