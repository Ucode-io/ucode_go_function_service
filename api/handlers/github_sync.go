package handlers

import (
	"context"
	"fmt"
	"log"
	"strings"

	status "ucode/ucode_go_function_service/api/status_http"
	pb "ucode/ucode_go_function_service/genproto/company_service"
	nb "ucode/ucode_go_function_service/genproto/new_object_builder_service"
	"ucode/ucode_go_function_service/pkg/gitlab"
	"ucode/ucode_go_function_service/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

// GithubSyncMicrofrontendRequest is the request body for the retroactive sync endpoint.
type GithubSyncMicrofrontendRequest struct {
	FunctionID     string `json:"function_id" binding:"required"`
	GithubRepoName string `json:"github_repo_name"` // optional — derived from GitLab path if empty
}

// GithubSyncMicrofrontend godoc
// @Security ApiKeyAuth
// @ID github_sync_microfrontend
// @Router /v2/functions/micro-frontend/github-sync [POST]
// @Summary Sync a microfrontend to the connected GitHub account
// @Description Creates a GitHub mirror repo (if it doesn't exist), pushes all current u-gen files,
// @Description and registers a push webhook so future GitHub commits are mirrored back to GitLab u-gen.
// @Tags GitHub Integration
// @Accept json
// @Produce json
// @Param body body GithubSyncMicrofrontendRequest true "GithubSyncMicrofrontendRequest"
// @Success 200 {object} status_http.Response "ok"
// @Failure 400 {object} status_http.Response{data=string}
// @Failure 500 {object} status_http.Response{data=string}
func (h *Handler) GithubSyncMicrofrontend(c *gin.Context) {
	var req GithubSyncMicrofrontendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleResponse(c, status.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, status.InvalidArgument, "error getting environment id | not valid")
		return
	}

	ctx := c.Request.Context()

	resource, err := h.services.CompanyService().ServiceResource().GetSingle(
		ctx, &pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	funcRecord, err := h.services.GoObjectBuilderService().Function().GetSingle(ctx, &nb.FunctionPrimaryKey{
		Id:        req.FunctionID,
		ProjectId: resource.ResourceEnvironmentId,
	})
	if err != nil {
		h.handleResponse(c, status.GRPCError, fmt.Sprintf("function not found: %v", err))
		return
	}

	if err := h.syncMicrofrontendToGithub(ctx, funcRecord, req.GithubRepoName); err != nil {
		h.handleResponse(c, status.InternalServerError, err.Error())
		return
	}

	h.handleResponse(c, status.OK, gin.H{"status": "ok"})
}

// syncMicrofrontendToGithub is the shared core used by both auto-sync (on publish)
// and the explicit GithubSyncMicrofrontend endpoint.
//
// It: fetches u-gen files from GitLab → pushes to GitHub (creates repo if needed)
//   → registers webhook → updates the function record with github_repo_name + github_webhook_id.
//
// githubRepoName is the desired GitHub repo name (without owner prefix).
// If empty, the existing funcRecord.GithubRepoName is used; if that is also empty
// the GitLab project path is used as a fallback.
func (h *Handler) syncMicrofrontendToGithub(ctx context.Context, funcRecord *nb.Function, githubRepoName string) error {
	projectID := funcRecord.GetProjectId()
	environmentID := funcRecord.GetEnvironmentId()
	gitlabRepoID := cast.ToInt(funcRecord.GetRepoId())

	if gitlabRepoID == 0 {
		return fmt.Errorf("function record has no gitlab repo_id")
	}

	// 1. GitHub integration (token + username)
	token, username, err := h.getGithubIntegration(ctx, projectID, environmentID)
	if err != nil {
		return fmt.Errorf("github integration not found: %w", err)
	}

	// 2. Resolve repo name
	repoName := githubRepoName
	if repoName == "" {
		// Check if already synced before — reuse existing repo name
		existing := funcRecord.GetGithubRepoName()
		if existing != "" {
			// stored as "owner/repo" — extract just the repo part
			parts := strings.SplitN(existing, "/", 2)
			if len(parts) == 2 {
				repoName = parts[1]
			} else {
				repoName = existing
			}
		}
	}
	if repoName == "" {
		// Last resort: derive from GitLab project path
		repoName, err = gitlab.GetProjectPath(h.cfg.GitlabIntegrationURL, h.cfg.GitlabTokenMicroFront, gitlabRepoID)
		if err != nil {
			return fmt.Errorf("could not resolve repo name from gitlab project %d: %w", gitlabRepoID, err)
		}
		log.Printf("[GITHUB-SYNC] using GitLab path as GitHub repo name: %q", repoName)
	}

	fullRepoName := username + "/" + repoName

	// 3. Fetch all files from GitLab u-gen branch
	files, err := gitlab.GetRepoCodebase(h.cfg.GitlabIntegrationURL, h.cfg.GitlabTokenMicroFront, gitlabRepoID)
	if err != nil {
		return fmt.Errorf("fetch gitlab u-gen files: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no files found in u-gen branch for repo %d", gitlabRepoID)
	}

	// 4. Push files to GitHub (creates repo + waits if it doesn't exist yet)
	if err := h.pushMicrofrontendToGithub(ctx, token, username, repoName, files); err != nil {
		return fmt.Errorf("push to github %s: %w", fullRepoName, err)
	}
	log.Printf("[GITHUB-SYNC] pushed %d file(s) to %s", len(files), fullRepoName)

	// 5. Webhook — delete old one if the repo name changed, then register new one
	webhookID := funcRecord.GetGithubWebhookId()
	oldFullName := funcRecord.GetGithubRepoName()

	if webhookID != "" && oldFullName != "" && oldFullName != fullRepoName {
		// Repo was renamed or re-pointed — remove the stale webhook first
		oldParts := strings.SplitN(oldFullName, "/", 2)
		if len(oldParts) == 2 {
			if delErr := h.deleteGithubWebhook(ctx, token, oldParts[0], oldParts[1], webhookID); delErr != nil {
				log.Printf("[GITHUB-SYNC] warning: could not delete old webhook %s on %s: %v", webhookID, oldFullName, delErr)
			}
		}
		webhookID = ""
	}

	if webhookID == "" {
		webhookID, err = h.createGithubWebhook(ctx, token, username, repoName)
		if err != nil {
			// Non-fatal: repo is already synced; log and continue
			log.Printf("[GITHUB-SYNC] warning: could not create webhook on %s: %v", fullRepoName, err)
		} else {
			log.Printf("[GITHUB-SYNC] webhook registered on %s (id=%s)", fullRepoName, webhookID)
		}
	}

	// 6. Persist github_repo_name and github_webhook_id back to the function record
	updated := *funcRecord // shallow copy — only override the two github fields
	updated.GithubRepoName = fullRepoName
	updated.GithubWebhookId = webhookID

	if _, err := h.services.GoObjectBuilderService().Function().Update(ctx, &updated); err != nil {
		return fmt.Errorf("update function record with github info: %w", err)
	}

	log.Printf("[GITHUB-SYNC] function %s linked to github repo %s", funcRecord.GetId(), fullRepoName)
	return nil
}
