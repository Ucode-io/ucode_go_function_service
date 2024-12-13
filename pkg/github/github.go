package github

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

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

	if err = json.Unmarshal(respBody, &importResponse); err != nil {
		return ImportResponse{}, errors.New("failed to unmarshal response body")
	}
	return importResponse, nil
}

func AddCiFile(gitlabToken string, gitlabRepoId int, branch, localFolderPath string) error {
	dir, err := os.Getwd()
	if err != nil {
		return errors.New("failed to get current directory")
	}

	mainFilePath := fmt.Sprintf("%v/%v/.gitlab-ci.yml", dir, localFolderPath)
	packageDirPath := fmt.Sprintf("%v/%v/.gitlab/ci", dir, localFolderPath)

	commitActions := []map[string]interface{}{}

	// Add main .gitlab-ci.yml file to actions
	mainFileContent, err := os.ReadFile(mainFilePath)
	if err != nil {
		return errors.New("failed to read .gitlab-ci.yml file")
	}
	commitActions = append(commitActions, map[string]interface{}{
		"action":    "create",
		"file_path": ".gitlab-ci.yml",
		"content":   string(mainFileContent),
	})

	// Iterate over files in the package directory and add them to actions
	err = filepath.Walk(packageDirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only process files
		if !info.IsDir() {
			// Calculate relative path based on baseDir
			relPath, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}

			// Ensure the file paths start from ".gitlab/ci"
			if strings.HasPrefix(relPath, "knative/.gitlab/ci") {
				relPath = strings.TrimPrefix(relPath, "knative/")
			}

			fileContent, err := os.ReadFile(path)
			if err != nil {
				return errors.New("failed to read a file in the .gitlab/ci directory")
			}

			commitActions = append(commitActions, map[string]interface{}{
				"action":    "create",
				"file_path": relPath,
				"content":   string(fileContent),
			})
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Create commit payload
	commitURL := fmt.Sprintf("https://gitlab.udevs.io/api/v4/projects/%v/repository/commits", gitlabRepoId)
	commitPayload := map[string]interface{}{
		"branch":         branch,
		"commit_message": "Added CI files",
		"actions":        commitActions,
	}

	_, err = DoRequest(http.MethodPost, commitURL, commitPayload, gitlabToken)
	if err != nil {
		return errors.New("failed to make GitLab request")
	}

	return nil
}

func DoRequest(method, url string, payload map[string]interface{}, token string) (map[string]interface{}, error) {
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

	if err = json.Unmarshal(respBody, &result); err != nil {
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
	var apiURL = fmt.Sprintf("%s/projects/%v/pipelines?ref=%s&order_by=id&sort=desc&per_page=1", "https://gitlab.udevs.io/api/v4", projectID, branchName)

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
	var url = fmt.Sprintf("%s/api/v4/projects/%v/jobs", gitlabURL, repoId)

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
