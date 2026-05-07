package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	status "ucode/ucode_go_function_service/api/status_http"
	"ucode/ucode_go_function_service/config"
	nb "ucode/ucode_go_function_service/genproto/new_object_builder_service"
	"ucode/ucode_go_function_service/pkg/gitlab"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

// skipWebhookFile returns true for files that should not be mirrored to GitLab.
func skipWebhookFile(path string) bool {
	skip := []string{"package-lock.json", "yarn.lock", "pnpm-lock.yaml", "node_modules/"}
	for _, s := range skip {
		if strings.HasSuffix(path, s) || strings.HasPrefix(path, s) {
			return true
		}
	}
	return false
}

// githubPushPayload is the subset of the GitHub push event we care about.
type githubPushPayload struct {
	Ref        string `json:"ref"`
	After      string `json:"after"` // HEAD commit SHA after the push
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Commits []struct {
		Added    []string `json:"added"`
		Modified []string `json:"modified"`
		Removed  []string `json:"removed"`
	} `json:"commits"`
}

// HandleGithubWebhook receives GitHub push events and mirrors the changes to
// the GitLab u-gen branch of the corresponding microfrontend.
//
// @ID handle_github_webhook
// @Router /v2/webhook/github [POST]
// @Summary Handle GitHub push webhook
// @Description Verifies HMAC-SHA256 signature, then mirrors added/modified/removed
// @Description files from the GitHub push to the GitLab u-gen branch.
// @Tags GitHub Integration
// @Accept json
// @Produce json
// @Success 200 {object} status_http.Response
// @Failure 400 {object} status_http.Response{data=string}
// @Failure 500 {object} status_http.Response{data=string}
func (h *Handler) HandleGithubWebhook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.handleResponse(c, status.BadRequest, "failed to read request body")
		return
	}

	// Only process push events.
	if c.GetHeader("X-GitHub-Event") != "push" {
		c.Status(200)
		return
	}

	// Verify HMAC-SHA256 signature when a secret is configured.
	if h.cfg.GithubWebhookSecret != "" {
		sig := c.GetHeader("X-Hub-Signature-256")
		if !verifyGithubSignature(body, h.cfg.GithubWebhookSecret, sig) {
			h.handleResponse(c, status.BadRequest, "invalid webhook signature")
			return
		}
	}

	var payload githubPushPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		h.handleResponse(c, status.BadRequest, "failed to parse push payload")
		return
	}

	fullName := payload.Repository.FullName
	if fullName == "" {
		h.handleResponse(c, status.BadRequest, "missing repository.full_name in payload")
		return
	}

	// Ignore branch deletes (after == 000...0).
	if strings.HasPrefix(payload.After, "0000000") {
		c.Status(200)
		return
	}

	ctx := c.Request.Context()

	// Look up the function record by github_repo_name.
	list, err := h.services.GoObjectBuilderService().Function().GetList(ctx, &nb.GetAllFunctionsRequest{
		GithubRepoName: fullName,
		Limit:          1,
	})
	if err != nil || len(list.GetFunctions()) == 0 {
		log.Printf("[GITHUB-WEBHOOK] no function found for repo %s: %v", fullName, err)
		// Return 200 so GitHub doesn't retry — the repo may have been unlinked.
		c.Status(200)
		return
	}

	funcRecord := list.GetFunctions()[0]
	gitlabRepoID := cast.ToInt(funcRecord.GetRepoId())
	if gitlabRepoID == 0 {
		log.Printf("[GITHUB-WEBHOOK] function %s has no gitlab repo_id", funcRecord.GetId())
		c.Status(200)
		return
	}

	// Collect the set of changed paths across all commits.
	added := map[string]bool{}
	modified := map[string]bool{}
	removed := map[string]bool{}

	for _, commit := range payload.Commits {
		for _, p := range commit.Added {
			added[p] = true
		}
		for _, p := range commit.Modified {
			modified[p] = true
		}
		for _, p := range commit.Removed {
			removed[p] = true
		}
	}

	// Split owner/repo for the content API.
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 {
		h.handleResponse(c, status.BadRequest, "malformed repository.full_name")
		return
	}
	owner, repo := parts[0], parts[1]

	// Fetch the GitHub token so we can read file contents.
	token, _, err := h.getGithubIntegration(ctx, funcRecord.GetProjectId(), funcRecord.GetEnvironmentId())
	if err != nil {
		log.Printf("[GITHUB-WEBHOOK] could not get github token for func %s: %v", funcRecord.GetId(), err)
		h.handleResponse(c, status.InternalServerError, fmt.Sprintf("github integration error: %v", err))
		return
	}

	// Build the file list for CommitFiles.
	var toCommit []*nb.McpProjectFiles

	for path := range added {
		if skipWebhookFile(path) {
			continue
		}
		content, fetchErr := h.getGithubFileContent(ctx, token, owner, repo, path, payload.After)
		if fetchErr != nil {
			log.Printf("[GITHUB-WEBHOOK] fetch %s@%s: %v", path, payload.After, fetchErr)
			continue
		}
		toCommit = append(toCommit, &nb.McpProjectFiles{Path: path, Content: content})
	}

	for path := range modified {
		if skipWebhookFile(path) {
			continue
		}
		// Skip if we already have it from the added set (shouldn't happen, but be safe).
		if added[path] {
			continue
		}
		content, fetchErr := h.getGithubFileContent(ctx, token, owner, repo, path, payload.After)
		if fetchErr != nil {
			log.Printf("[GITHUB-WEBHOOK] fetch %s@%s: %v", path, payload.After, fetchErr)
			continue
		}
		toCommit = append(toCommit, &nb.McpProjectFiles{Path: path, Content: content})
	}

	// Handle deletions: CommitFiles only supports create/update actions via the
	// McpProjectFiles wrapper, so we call the GitLab commits API directly for deletes.
	if len(removed) > 0 {
		if err := h.deleteGitlabFiles(gitlabRepoID, removed, payload.After); err != nil {
			log.Printf("[GITHUB-WEBHOOK] delete files on gitlab: %v", err)
		}
	}

	if len(toCommit) == 0 {
		log.Printf("[GITHUB-WEBHOOK] no committable files in push to %s", fullName)
		c.Status(200)
		return
	}

	gitlabCfg := gitlab.IntegrationData{
		GitlabProjectId:        gitlabRepoID,
		GitlabIntegrationUrl:   h.cfg.GitlabIntegrationURL,
		GitlabIntegrationToken: h.cfg.GitlabTokenMicroFront,
	}

	_, err = gitlab.CommitFiles(gitlabCfg, config.UGenBranch, toCommit, "mirror: "+payload.After[:min(7, len(payload.After))])
	if err != nil {
		log.Printf("[GITHUB-WEBHOOK] commit to gitlab u-gen failed for %s: %v", fullName, err)
		h.handleResponse(c, status.InternalServerError, fmt.Sprintf("gitlab commit failed: %v", err))
		return
	}

	log.Printf("[GITHUB-WEBHOOK] mirrored %d file(s) from %s to gitlab repo %d (u-gen)", len(toCommit), fullName, gitlabRepoID)
	h.handleResponse(c, status.OK, gin.H{"mirrored": len(toCommit)})
}

// verifyGithubSignature checks X-Hub-Signature-256: sha256=<hex>.
func verifyGithubSignature(body []byte, secret, sigHeader string) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(sigHeader, prefix) {
		return false
	}
	expected, err := hex.DecodeString(sigHeader[len(prefix):])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(mac.Sum(nil), expected)
}

// deleteGitlabFiles issues a single commit that deletes the given paths on the u-gen branch.
func (h *Handler) deleteGitlabFiles(repoID int, paths map[string]bool, ref string) error {
	type deleteAction struct {
		Action   string `json:"action"`
		FilePath string `json:"file_path"`
	}
	type commitReq struct {
		Branch        string         `json:"branch"`
		CommitMessage string         `json:"commit_message"`
		Actions       []deleteAction `json:"actions"`
	}

	var actions []deleteAction
	for p := range paths {
		if !skipWebhookFile(p) {
			actions = append(actions, deleteAction{Action: "delete", FilePath: p})
		}
	}
	if len(actions) == 0 {
		return nil
	}

	req := commitReq{
		Branch:        config.UGenBranch,
		CommitMessage: "mirror-delete: " + ref[:min(7, len(ref))],
		Actions:       actions,
	}
	url := fmt.Sprintf("%s/api/v4/projects/%d/repository/commits", h.cfg.GitlabIntegrationURL, repoID)
	_, err := gitlab.DoRequestV1(url, h.cfg.GitlabTokenMicroFront, "POST", req)
	return err
}
