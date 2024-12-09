package github

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func ListWebhooks(cfg ListWebhookRequest) (bool, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/hooks", cfg.Username, cfg.RepoName)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, err
	}

	req.Header.Set("Authorization", "Bearer "+cfg.GithubToken)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, errors.New("failed to send HTTP request")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	fmt.Println(string(body))

	var webhooks []interface{}
	if err := json.Unmarshal(body, &webhooks); err != nil {
		return false, err
	}

	for _, webhook := range webhooks {
		webhookMap := webhook.(map[string]interface{})
		if webhookMap["config"] == nil {
			continue
		}
		config := webhookMap["config"].(map[string]interface{})
		if config["url"] == nil {
			continue
		}
		url := config["url"].(string)
		if strings.HasPrefix(url, cfg.ProjectUrl) {
			return true, nil
		}
	}
	return false, nil
}

func CreateWebhook(cfg CreateWebhookRequest) error {
	apiUrl := fmt.Sprintf(`https://api.github.com/repos/%s/%s/hooks`, cfg.Username, cfg.RepoName)
	handleUrl := fmt.Sprintf(`%s/v2/webhook/handle?project_id=%s&resource_id=%s`, cfg.ProjectUrl, cfg.ProjectId, cfg.ResourceId)

	payload := WebhookPayload{
		Name:   "web",
		Active: true,
		Events: []string{"push"},
		Config: Config{
			URL:         handleUrl,
			ContentType: "json",
			Secret:      cfg.WebhookSecret,
			Name:        cfg.Name,
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return errors.New("failed to marshal JSON")
	}

	req, err := http.NewRequest(http.MethodPost, apiUrl, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return errors.New("failed to create HTTP request")
	}

	req.Header.Set("Authorization", "Bearer "+cfg.GithubToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return errors.New("failed to send HTTP request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return errors.New("failed to create webhook")
	}

	return nil
}
