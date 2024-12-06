package github

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
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
		body, err := ioutil.ReadAll(resp.Body) // or io.ReadAll in Go 1.16+
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
