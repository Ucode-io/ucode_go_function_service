package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	status "ucode/ucode_go_function_service/api/status_http"
	"ucode/ucode_go_function_service/config"
	pb "ucode/ucode_go_function_service/genproto/company_service"
	nb "ucode/ucode_go_function_service/genproto/new_object_builder_service"
	"ucode/ucode_go_function_service/pkg/gitlab"
	"ucode/ucode_go_function_service/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/spf13/cast"
)

const (
	extGitlabStatePrefix  = "ext-gitlab:state:"
	bitbucketStatePrefix  = "bitbucket:state:"
	externalOAuthStateTTL = 15 * time.Minute
)

var externalHTTPClient = &http.Client{Timeout: 30 * time.Second}

type externalOAuthToken struct {
	AccessToken         string `json:"access_token"`
	RefreshToken        string `json:"refresh_token,omitempty"`
	TokenType           string `json:"token_type,omitempty"`
	ExpiresAt           int64  `json:"expires_at,omitempty"`
	BitbucketWorkspace  string `json:"bitbucket_workspace,omitempty"`
	BitbucketProjectKey string `json:"bitbucket_project_key,omitempty"`
}

type externalIntegration struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Name          string `json:"name"`
	ProjectID     string `json:"project_id"`
	EnvironmentID string `json:"environment_id"`
}

type extGitlabUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

type bitbucketUser struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AccountID   string `json:"account_id"`
	UUID        string `json:"uuid"`
}

type bitbucketWorkspace struct {
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	UUID       string `json:"uuid"`
	Permission string `json:"permission"`
	IsAdmin    bool   `json:"is_admin"`
}

type bitbucketProject struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	UUID string `json:"uuid"`
}

type providerCommitChange struct {
	Added    []string `json:"added"`
	Modified []string `json:"modified"`
	Removed  []string `json:"removed"`
}

type ExtGitlabSyncMicrofrontendRequest struct {
	FunctionID     string `json:"function_id" binding:"required"`
	GitlabRepoName string `json:"gitlab_repo_name"`
}

type BitbucketSyncMicrofrontendRequest struct {
	FunctionID          string `json:"function_id" binding:"required"`
	BitbucketWorkspace  string `json:"bitbucket_workspace"`
	BitbucketRepoSlug   string `json:"bitbucket_repo_slug"`
	BitbucketProjectKey string `json:"bitbucket_project_key"`
}

type BitbucketWorkspaceRequest struct {
	BitbucketWorkspace  string `json:"bitbucket_workspace" binding:"required"`
	BitbucketProjectKey string `json:"bitbucket_project_key"`
}

func (h *Handler) ExtGitlabConnect(c *gin.Context) {
	projectID, environmentID, ok := getProjectAndEnv(c)
	if !ok {
		h.handleResponse(c, status.InvalidArgument, "project_id and environment_id required")
		return
	}

	state, err := h.storeExternalOAuthState(c.Request.Context(), extGitlabStatePrefix, projectID, environmentID, c.GetString("user_id"))
	if err != nil {
		h.handleResponse(c, status.InternalServerError, err.Error())
		return
	}

	params := url.Values{}
	params.Set("client_id", h.cfg.GitlabClientIdIntegration)
	params.Set("redirect_uri", h.cfg.GitlabRedirectUriIntegration)
	params.Set("response_type", "code")
	params.Set("state", state)
	params.Set("scope", "api read_user read_repository write_repository")

	h.handleResponse(c, status.OK, strings.TrimRight(h.cfg.GitlabBaseUrlIntegration, "/")+"/oauth/authorize?"+params.Encode())
}

func (h *Handler) ExtGitlabCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		redirectWithQuery(c, h.cfg.GitlabFrontendErrorURL, map[string]string{"provider": "gitlab", "reason": "missing_params"})
		return
	}

	stateData, err := h.consumeExternalOAuthState(c.Request.Context(), extGitlabStatePrefix, state)
	if err != nil {
		redirectWithQuery(c, h.cfg.GitlabFrontendErrorURL, map[string]string{"provider": "gitlab", "reason": "invalid_state"})
		return
	}

	token, err := h.exchangeExtGitlabCode(c.Request.Context(), code)
	if err != nil {
		h.log.Error("external gitlab callback token exchange failed: " + err.Error())
		redirectWithQuery(c, h.cfg.GitlabFrontendErrorURL, map[string]string{"provider": "gitlab", "reason": "token_exchange_failed"})
		return
	}

	user, err := h.fetchExtGitlabUser(c.Request.Context(), token.AccessToken)
	if err != nil {
		h.log.Error("external gitlab callback fetch user failed: " + err.Error())
		redirectWithQuery(c, h.cfg.GitlabFrontendErrorURL, map[string]string{"provider": "gitlab", "reason": "fetch_user_failed"})
		return
	}

	integrationID, err := h.upsertExternalIntegration(c.Request.Context(), pb.ResourceType_GITLAB, token, user.Username, user.Name, stateData)
	if err != nil {
		h.log.Error("external gitlab callback save integration failed: " + err.Error())
		redirectWithQuery(c, h.cfg.GitlabFrontendErrorURL, map[string]string{"provider": "gitlab", "reason": "save_failed"})
		return
	}

	redirectWithQuery(c, h.cfg.GitlabFrontendSuccessURL, map[string]string{
		"provider":       "gitlab",
		"integration_id": integrationID,
		"username":       user.Username,
	})
}

func (h *Handler) ExtGitlabGetIntegration(c *gin.Context) {
	h.getExternalIntegrationResponse(c, pb.ResourceType_GITLAB, "external GitLab")
}

func (h *Handler) ExtGitlabValidateToken(c *gin.Context) {
	projectID, environmentID, ok := getProjectAndEnv(c)
	if !ok {
		h.handleResponse(c, status.InvalidArgument, "project_id and environment_id required")
		return
	}

	token, err := h.getExternalProviderAccessToken(c.Request.Context(), pb.ResourceType_GITLAB, projectID, environmentID)
	if err != nil {
		h.handleResponse(c, status.Unauthorized, "external GitLab reconnect required: "+err.Error())
		return
	}

	user, err := h.fetchExtGitlabUser(c.Request.Context(), token)
	if err != nil {
		h.handleResponse(c, status.Unauthorized, "external GitLab token is invalid or revoked: "+err.Error())
		return
	}

	h.handleResponse(c, status.OK, user)
}

func (h *Handler) ExtGitlabDeleteIntegration(c *gin.Context) {
	h.deleteExternalIntegration(c, "external GitLab")
}

func (h *Handler) BitbucketConnect(c *gin.Context) {
	projectID, environmentID, ok := getProjectAndEnv(c)
	if !ok {
		h.handleResponse(c, status.InvalidArgument, "project_id and environment_id required")
		return
	}

	state, err := h.storeExternalOAuthState(c.Request.Context(), bitbucketStatePrefix, projectID, environmentID, c.GetString("user_id"))
	if err != nil {
		h.handleResponse(c, status.InternalServerError, err.Error())
		return
	}

	params := url.Values{}
	params.Set("client_id", h.cfg.BitbucketClientID)
	params.Set("response_type", "code")
	params.Set("state", state)
	params.Set("scope", "account repository repository:write repository:admin webhook")
	if h.cfg.BitbucketRedirectURI != "" {
		params.Set("redirect_uri", h.cfg.BitbucketRedirectURI)
	}

	h.handleResponse(c, status.OK, strings.TrimRight(h.cfg.BitbucketBaseURL, "/")+"/site/oauth2/authorize?"+params.Encode())
}

func (h *Handler) BitbucketCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	if code == "" || state == "" {
		redirectWithQuery(c, h.cfg.BitbucketFrontendErrorURL, map[string]string{"provider": "bitbucket", "reason": "missing_params"})
		return
	}

	stateData, err := h.consumeExternalOAuthState(c.Request.Context(), bitbucketStatePrefix, state)
	if err != nil {
		redirectWithQuery(c, h.cfg.BitbucketFrontendErrorURL, map[string]string{"provider": "bitbucket", "reason": "invalid_state"})
		return
	}

	token, err := h.exchangeBitbucketCode(c.Request.Context(), code)
	if err != nil {
		h.log.Error("bitbucket callback token exchange failed: " + err.Error())
		redirectWithQuery(c, h.cfg.BitbucketFrontendErrorURL, map[string]string{"provider": "bitbucket", "reason": "token_exchange_failed"})
		return
	}

	user, err := h.fetchBitbucketUser(c.Request.Context(), token.AccessToken)
	if err != nil {
		h.log.Error("bitbucket callback fetch user failed: " + err.Error())
		redirectWithQuery(c, h.cfg.BitbucketFrontendErrorURL, map[string]string{"provider": "bitbucket", "reason": "fetch_user_failed"})
		return
	}

	username := user.Username
	if username == "" {
		username = user.AccountID
	}
	name := user.DisplayName
	if name == "" {
		name = username
	}

	integrationID, err := h.upsertExternalIntegration(c.Request.Context(), pb.ResourceType_BITBUCKET, token, username, name, stateData)
	if err != nil {
		h.log.Error("bitbucket callback save integration failed: " + err.Error())
		redirectWithQuery(c, h.cfg.BitbucketFrontendErrorURL, map[string]string{"provider": "bitbucket", "reason": "save_failed"})
		return
	}

	redirectWithQuery(c, h.cfg.BitbucketFrontendSuccessURL, map[string]string{
		"provider":       "bitbucket",
		"integration_id": integrationID,
		"username":       username,
	})
}

func (h *Handler) BitbucketGetIntegration(c *gin.Context) {
	h.getExternalIntegrationResponse(c, pb.ResourceType_BITBUCKET, "Bitbucket")
}

func (h *Handler) BitbucketValidateToken(c *gin.Context) {
	projectID, environmentID, ok := getProjectAndEnv(c)
	if !ok {
		h.handleResponse(c, status.InvalidArgument, "project_id and environment_id required")
		return
	}

	token, err := h.getExternalProviderAccessToken(c.Request.Context(), pb.ResourceType_BITBUCKET, projectID, environmentID)
	if err != nil {
		h.handleResponse(c, status.Unauthorized, "Bitbucket reconnect required: "+err.Error())
		return
	}

	user, err := h.fetchBitbucketUser(c.Request.Context(), token)
	if err != nil {
		h.handleResponse(c, status.Unauthorized, "Bitbucket token is invalid or revoked: "+err.Error())
		return
	}

	h.handleResponse(c, status.OK, user)
}

func (h *Handler) BitbucketDeleteIntegration(c *gin.Context) {
	h.deleteExternalIntegration(c, "Bitbucket")
}

func (h *Handler) BitbucketWorkspaces(c *gin.Context) {
	projectID, environmentID, ok := getProjectAndEnv(c)
	if !ok {
		h.handleResponse(c, status.InvalidArgument, "project_id and environment_id required")
		return
	}

	token, err := h.getExternalProviderAccessToken(c.Request.Context(), pb.ResourceType_BITBUCKET, projectID, environmentID)
	if err != nil {
		h.handleResponse(c, status.NotFound, "Bitbucket integration not found: "+err.Error())
		return
	}

	workspaces, err := h.listBitbucketWorkspaces(c.Request.Context(), token)
	if err != nil {
		h.handleResponse(c, status.InternalServerError, err.Error())
		return
	}

	h.handleResponse(c, status.OK, workspaces)
}

func (h *Handler) BitbucketSaveWorkspace(c *gin.Context) {
	var req BitbucketWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}

	projectID, environmentID, ok := getProjectAndEnv(c)
	if !ok {
		h.handleResponse(c, status.InvalidArgument, "project_id and environment_id required")
		return
	}

	if err := h.rememberBitbucketDefaults(c.Request.Context(), projectID, environmentID, req.BitbucketWorkspace, req.BitbucketProjectKey); err != nil {
		h.handleResponse(c, status.InternalServerError, err.Error())
		return
	}

	h.handleResponse(c, status.OK, gin.H{
		"bitbucket_workspace":   req.BitbucketWorkspace,
		"bitbucket_project_key": req.BitbucketProjectKey,
	})
}

func (h *Handler) ExtGitlabSyncMicrofrontend(c *gin.Context) {
	var req ExtGitlabSyncMicrofrontendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}

	funcRecord, resource, ok := h.resolveMirrorFunction(c, req.FunctionID)
	if !ok {
		return
	}

	if err := h.syncMicrofrontendToExtGitlab(c.Request.Context(), funcRecord, req.GitlabRepoName, resource.ProjectId, resource.EnvironmentId); err != nil {
		h.handleResponse(c, status.InternalServerError, err.Error())
		return
	}

	h.handleResponse(c, status.OK, gin.H{"status": "ok"})
}

func (h *Handler) BitbucketSyncMicrofrontend(c *gin.Context) {
	var req BitbucketSyncMicrofrontendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}

	funcRecord, resource, ok := h.resolveMirrorFunction(c, req.FunctionID)
	if !ok {
		return
	}

	if err := h.syncMicrofrontendToBitbucket(c.Request.Context(), funcRecord, req.BitbucketWorkspace, req.BitbucketRepoSlug, req.BitbucketProjectKey, resource.ProjectId, resource.EnvironmentId); err != nil {
		h.handleResponse(c, status.InternalServerError, err.Error())
		return
	}

	h.handleResponse(c, status.OK, gin.H{"status": "ok"})
}

func (h *Handler) syncAllMicrofrontendMirrors(ctx context.Context, funcRecord *nb.Function, companyProjectID, environmentID string) {
	if err := h.syncMicrofrontendToGithub(ctx, funcRecord, "", companyProjectID, environmentID); err != nil {
		log.Printf("[MIRROR→GITHUB] sync skipped/failed for func_id=%s: %v", funcRecord.GetId(), err)
	}
	if err := h.syncMicrofrontendToExtGitlab(ctx, funcRecord, "", companyProjectID, environmentID); err != nil {
		log.Printf("[MIRROR→EXT-GITLAB] sync skipped/failed for func_id=%s: %v", funcRecord.GetId(), err)
	}
	if err := h.syncMicrofrontendToBitbucket(ctx, funcRecord, "", "", "", companyProjectID, environmentID); err != nil {
		log.Printf("[MIRROR→BITBUCKET] sync skipped/failed for func_id=%s: %v", funcRecord.GetId(), err)
	}
}

func (h *Handler) syncMicrofrontendToExtGitlab(ctx context.Context, funcRecord *nb.Function, repoName, companyProjectID, environmentID string) error {
	gitlabRepoID := cast.ToInt(funcRecord.GetRepoId())
	if gitlabRepoID == 0 {
		return fmt.Errorf("function record has no gitlab repo_id")
	}

	token, err := h.getExternalProviderAccessToken(ctx, pb.ResourceType_GITLAB, companyProjectID, environmentID)
	if err != nil {
		return fmt.Errorf("external GitLab integration not found: %w", err)
	}

	if repoName == "" {
		var nameErr error
		repoName, nameErr = gitlab.GetProjectPath(h.cfg.GitlabIntegrationURL, h.cfg.GitlabTokenMicroFront, gitlabRepoID)
		if nameErr != nil {
			return fmt.Errorf("resolve repo name from internal gitlab: %w", nameErr)
		}
	}
	repoName = safeRepoSlug(repoName)

	files, err := gitlab.GetRepoCodebase(h.cfg.GitlabIntegrationURL, h.cfg.GitlabTokenMicroFront, gitlabRepoID)
	if err != nil {
		return fmt.Errorf("fetch gitlab u-gen files: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no files found in u-gen branch for repo %d", gitlabRepoID)
	}

	project, err := h.ensureExtGitlabProject(ctx, token, repoName)
	if err != nil {
		return err
	}

	if err := h.pushFilesToExtGitlab(ctx, token, project.ID, project.DefaultBranch, files); err != nil {
		return err
	}

	if err := h.ensureExtGitlabWebhook(ctx, token, project.ID, funcRecord.GetId(), companyProjectID, environmentID, funcRecord.GetProjectId()); err != nil {
		log.Printf("[EXT-GITLAB-SYNC] warning: could not create webhook for project %d: %v", project.ID, err)
	}

	log.Printf("[EXT-GITLAB-SYNC] pushed %d file(s) to %s", len(files), project.PathWithNamespace)
	return nil
}

func (h *Handler) syncMicrofrontendToBitbucket(ctx context.Context, funcRecord *nb.Function, workspace, repoSlug, projectKey, companyProjectID, environmentID string) error {
	gitlabRepoID := cast.ToInt(funcRecord.GetRepoId())
	if gitlabRepoID == 0 {
		return fmt.Errorf("function record has no gitlab repo_id")
	}

	token, err := h.getExternalProviderToken(ctx, pb.ResourceType_BITBUCKET, companyProjectID, environmentID)
	if err != nil {
		return fmt.Errorf("Bitbucket integration not found: %w", err)
	}

	if workspace == "" {
		workspace = token.BitbucketWorkspace
	}
	if projectKey == "" {
		projectKey = token.BitbucketProjectKey
	}
	if workspace == "" {
		workspace, err = h.defaultBitbucketWorkspace(ctx, token.AccessToken)
		if err != nil {
			return err
		}
	}
	if projectKey == "" {
		projectKey, err = h.defaultBitbucketProjectKey(ctx, token.AccessToken, workspace)
		if err != nil {
			return err
		}
	}
	if repoSlug == "" {
		repoSlug, err = gitlab.GetProjectPath(h.cfg.GitlabIntegrationURL, h.cfg.GitlabTokenMicroFront, gitlabRepoID)
		if err != nil {
			return fmt.Errorf("resolve repo slug from internal gitlab: %w", err)
		}
	}
	repoSlug = safeRepoSlug(repoSlug)

	files, err := gitlab.GetRepoCodebase(h.cfg.GitlabIntegrationURL, h.cfg.GitlabTokenMicroFront, gitlabRepoID)
	if err != nil {
		return fmt.Errorf("fetch gitlab u-gen files: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no files found in u-gen branch for repo %d", gitlabRepoID)
	}

	if err := h.rememberBitbucketDefaults(ctx, companyProjectID, environmentID, workspace, projectKey); err != nil {
		log.Printf("[BITBUCKET-SYNC] warning: could not remember workspace/project key: %v", err)
	}

	if err := h.ensureBitbucketRepo(ctx, token.AccessToken, workspace, repoSlug, projectKey); err != nil {
		return err
	}
	if err := h.pushFilesToBitbucket(ctx, token.AccessToken, workspace, repoSlug, files); err != nil {
		return err
	}
	if err := h.ensureBitbucketWebhook(ctx, token.AccessToken, workspace, repoSlug, funcRecord.GetId(), companyProjectID, environmentID, funcRecord.GetProjectId()); err != nil {
		log.Printf("[BITBUCKET-SYNC] warning: could not create webhook for %s/%s: %v", workspace, repoSlug, err)
	}

	log.Printf("[BITBUCKET-SYNC] pushed %d file(s) to %s/%s", len(files), workspace, repoSlug)
	return nil
}

func (h *Handler) HandleExtGitlabWebhook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.handleResponse(c, status.BadRequest, "failed to read request body")
		return
	}

	if h.cfg.GitlabWebhookSecret != "" {
		if token := c.GetHeader("X-Gitlab-Token"); token != h.cfg.GitlabWebhookSecret {
			h.handleResponse(c, status.BadRequest, "invalid webhook token")
			return
		}
	}

	var payload struct {
		After   string `json:"after"`
		Project struct {
			ID int `json:"id"`
		} `json:"project"`
		Commits []providerCommitChange `json:"commits"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		h.handleResponse(c, status.BadRequest, "failed to parse GitLab webhook payload")
		return
	}
	if strings.HasPrefix(payload.After, "0000000") {
		c.Status(http.StatusOK)
		return
	}

	funcRecord, companyProjectID, environmentID, ok := h.webhookFunctionContext(c)
	if !ok {
		return
	}

	token, err := h.getExternalProviderAccessToken(c.Request.Context(), pb.ResourceType_GITLAB, companyProjectID, environmentID)
	if err != nil {
		h.handleResponse(c, status.InternalServerError, "external GitLab integration error: "+err.Error())
		return
	}

	added, modified, removed := collectGitChanges(payload.Commits)
	var toCommit []*nb.McpProjectFiles
	for path := range added {
		if skipWebhookFile(path) {
			continue
		}
		content, fetchErr := h.getExtGitlabFileContent(c.Request.Context(), token, payload.Project.ID, path, payload.After)
		if fetchErr != nil {
			log.Printf("[EXT-GITLAB-WEBHOOK] fetch %s@%s: %v", path, payload.After, fetchErr)
			continue
		}
		toCommit = append(toCommit, &nb.McpProjectFiles{Path: path, Content: content})
	}
	for path := range modified {
		if skipWebhookFile(path) || added[path] {
			continue
		}
		content, fetchErr := h.getExtGitlabFileContent(c.Request.Context(), token, payload.Project.ID, path, payload.After)
		if fetchErr != nil {
			log.Printf("[EXT-GITLAB-WEBHOOK] fetch %s@%s: %v", path, payload.After, fetchErr)
			continue
		}
		toCommit = append(toCommit, &nb.McpProjectFiles{Path: path, Content: content})
	}

	if err := h.commitProviderWebhookChanges(funcRecord, toCommit, removed, payload.After, "ext-gitlab"); err != nil {
		h.handleResponse(c, status.InternalServerError, err.Error())
		return
	}

	h.handleResponse(c, status.OK, gin.H{"mirrored": len(toCommit)})
}

func (h *Handler) HandleBitbucketWebhook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.handleResponse(c, status.BadRequest, "failed to read request body")
		return
	}

	if h.cfg.BitbucketWebhookSecret != "" {
		sig := c.GetHeader("X-Hub-Signature-256")
		if sig == "" {
			sig = c.GetHeader("X-Hub-Signature")
		}
		if sig == "" || !verifyGithubSignature(body, h.cfg.BitbucketWebhookSecret, sig) {
			h.handleResponse(c, status.BadRequest, "invalid webhook signature")
			return
		}
	}

	var payload struct {
		Push struct {
			Changes []struct {
				New struct {
					Name   string `json:"name"`
					Target struct {
						Hash string `json:"hash"`
					} `json:"target"`
				} `json:"new"`
			} `json:"changes"`
		} `json:"push"`
		Repository struct {
			FullName  string `json:"full_name"`
			Name      string `json:"name"`
			Workspace struct {
				Slug string `json:"slug"`
			} `json:"workspace"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		h.handleResponse(c, status.BadRequest, "failed to parse Bitbucket webhook payload")
		return
	}

	funcRecord, companyProjectID, environmentID, ok := h.webhookFunctionContext(c)
	if !ok {
		return
	}

	token, err := h.getExternalProviderAccessToken(c.Request.Context(), pb.ResourceType_BITBUCKET, companyProjectID, environmentID)
	if err != nil {
		h.handleResponse(c, status.InternalServerError, "Bitbucket integration error: "+err.Error())
		return
	}

	workspace := payload.Repository.Workspace.Slug
	repoSlug := path.Base(payload.Repository.FullName)
	if repoSlug == "." || repoSlug == "/" || repoSlug == "" {
		repoSlug = safeRepoSlug(payload.Repository.Name)
	}

	var toCommit []*nb.McpProjectFiles
	removed := map[string]bool{}
	ref := ""
	for _, change := range payload.Push.Changes {
		hash := change.New.Target.Hash
		if hash == "" {
			continue
		}
		ref = hash
		changes, diffErr := h.getBitbucketDiffstat(c.Request.Context(), token, workspace, repoSlug, hash)
		if diffErr != nil {
			log.Printf("[BITBUCKET-WEBHOOK] diffstat %s/%s@%s: %v", workspace, repoSlug, hash, diffErr)
			continue
		}
		for _, change := range changes {
			if skipWebhookFile(change.Path) {
				continue
			}
			if change.Removed {
				removed[change.Path] = true
				continue
			}
			content, fetchErr := h.getBitbucketFileContent(c.Request.Context(), token, workspace, repoSlug, change.Path, hash)
			if fetchErr != nil {
				log.Printf("[BITBUCKET-WEBHOOK] fetch %s@%s: %v", change.Path, hash, fetchErr)
				continue
			}
			toCommit = append(toCommit, &nb.McpProjectFiles{Path: change.Path, Content: content})
		}
	}

	if ref == "" {
		c.Status(http.StatusOK)
		return
	}
	if err := h.commitProviderWebhookChanges(funcRecord, toCommit, removed, ref, "bitbucket"); err != nil {
		h.handleResponse(c, status.InternalServerError, err.Error())
		return
	}

	h.handleResponse(c, status.OK, gin.H{"mirrored": len(toCommit)})
}

func (h *Handler) storeExternalOAuthState(ctx context.Context, prefix, projectID, environmentID, userID string) (string, error) {
	payload := githubStatePayload{ProjectID: projectID, EnvironmentID: environmentID, UserID: userID}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to encode state: %w", err)
	}

	state := uuid.NewString()
	if err := h.redis.SetX(ctx, prefix+state, string(payloadBytes), externalOAuthStateTTL); err != nil {
		return "", fmt.Errorf("failed to store OAuth state: %w", err)
	}
	return state, nil
}

func (h *Handler) consumeExternalOAuthState(ctx context.Context, prefix, state string) (githubStatePayload, error) {
	payloadStr, err := h.redis.Get(ctx, prefix+state)
	if err != nil {
		return githubStatePayload{}, err
	}
	_ = h.redis.Del(ctx, prefix+state)

	var stateData githubStatePayload
	if err := json.Unmarshal([]byte(payloadStr), &stateData); err != nil {
		return githubStatePayload{}, err
	}
	return stateData, nil
}

func (h *Handler) exchangeExtGitlabCode(ctx context.Context, code string) (externalOAuthToken, error) {
	body := url.Values{}
	body.Set("client_id", h.cfg.GitlabClientIdIntegration)
	body.Set("client_secret", h.cfg.GitlabClientSecretIntegration)
	body.Set("code", code)
	body.Set("grant_type", "authorization_code")
	body.Set("redirect_uri", h.cfg.GitlabRedirectUriIntegration)
	return h.exchangeExtGitlabToken(ctx, body)
}

func (h *Handler) refreshExtGitlabToken(ctx context.Context, refreshToken string) (externalOAuthToken, error) {
	body := url.Values{}
	body.Set("client_id", h.cfg.GitlabClientIdIntegration)
	body.Set("client_secret", h.cfg.GitlabClientSecretIntegration)
	body.Set("grant_type", "refresh_token")
	body.Set("refresh_token", refreshToken)
	return h.exchangeExtGitlabToken(ctx, body)
}

func (h *Handler) exchangeExtGitlabToken(ctx context.Context, body url.Values) (externalOAuthToken, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(h.cfg.GitlabBaseUrlIntegration, "/")+"/oauth/token", bytes.NewBufferString(body.Encode()))
	if err != nil {
		return externalOAuthToken{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := externalHTTPClient.Do(req)
	if err != nil {
		return externalOAuthToken{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return externalOAuthToken{}, fmt.Errorf("external GitLab token request returned %d: %s", resp.StatusCode, string(b))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		CreatedAt    int64  `json:"created_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return externalOAuthToken{}, err
	}
	if tokenResp.AccessToken == "" {
		return externalOAuthToken{}, fmt.Errorf("external GitLab returned empty access token")
	}
	createdAt := tokenResp.CreatedAt
	if createdAt == 0 {
		createdAt = time.Now().Unix()
	}
	return externalOAuthToken{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenType:    tokenResp.TokenType,
		ExpiresAt:    createdAt + tokenResp.ExpiresIn,
	}, nil
}

func (h *Handler) exchangeBitbucketCode(ctx context.Context, code string) (externalOAuthToken, error) {
	body := url.Values{}
	body.Set("grant_type", "authorization_code")
	body.Set("code", code)
	if h.cfg.BitbucketRedirectURI != "" {
		body.Set("redirect_uri", h.cfg.BitbucketRedirectURI)
	}
	return h.exchangeBitbucketToken(ctx, body)
}

func (h *Handler) refreshBitbucketToken(ctx context.Context, refreshToken string) (externalOAuthToken, error) {
	body := url.Values{}
	body.Set("grant_type", "refresh_token")
	body.Set("refresh_token", refreshToken)
	return h.exchangeBitbucketToken(ctx, body)
}

func (h *Handler) exchangeBitbucketToken(ctx context.Context, body url.Values) (externalOAuthToken, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(h.cfg.BitbucketBaseURL, "/")+"/site/oauth2/access_token", bytes.NewBufferString(body.Encode()))
	if err != nil {
		return externalOAuthToken{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(h.cfg.BitbucketClientID, h.cfg.BitbucketClientSecret)

	resp, err := externalHTTPClient.Do(req)
	if err != nil {
		return externalOAuthToken{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return externalOAuthToken{}, fmt.Errorf("Bitbucket token request returned %d: %s", resp.StatusCode, string(b))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return externalOAuthToken{}, err
	}
	if tokenResp.AccessToken == "" {
		return externalOAuthToken{}, fmt.Errorf("Bitbucket returned empty access token")
	}
	return externalOAuthToken{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		TokenType:    tokenResp.TokenType,
		ExpiresAt:    time.Now().Unix() + tokenResp.ExpiresIn,
	}, nil
}

func (h *Handler) fetchExtGitlabUser(ctx context.Context, token string) (*extGitlabUser, error) {
	req, err := h.externalBearerRequest(ctx, http.MethodGet, strings.TrimRight(h.cfg.GitlabBaseUrlIntegration, "/")+"/api/v4/user", nil, token)
	if err != nil {
		return nil, err
	}
	var user extGitlabUser
	if err := h.doJSON(req, http.StatusOK, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (h *Handler) fetchBitbucketUser(ctx context.Context, token string) (*bitbucketUser, error) {
	req, err := h.externalBearerRequest(ctx, http.MethodGet, strings.TrimRight(h.cfg.BitbucketApiBaseURL, "/")+"/user", nil, token)
	if err != nil {
		return nil, err
	}
	var user bitbucketUser
	if err := h.doJSON(req, http.StatusOK, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (h *Handler) upsertExternalIntegration(ctx context.Context, resourceType pb.ResourceType, token externalOAuthToken, username, name string, state githubStatePayload) (string, error) {
	tokenJSON, err := json.Marshal(token)
	if err != nil {
		return "", err
	}

	existing, err := h.services.CompanyService().IntegrationResource().GetIntegrationResourceList(ctx, &pb.GetListIntegrationResourceRequest{
		ProjectId:     state.ProjectID,
		EnvironmentId: state.EnvironmentID,
		Type:          resourceType,
	})
	if err == nil {
		for _, ir := range existing.GetIntegrationResources() {
			_, _ = h.services.CompanyService().IntegrationResource().DeleteIntegrationResource(ctx, &pb.IntegrationResourcePrimaryKey{Id: ir.GetId()})
		}
	}

	if name == "" {
		name = username
	}
	created, err := h.services.CompanyService().IntegrationResource().CreateIntegrationResource(ctx, &pb.CreateIntegrationResourceRequest{
		Token:         string(tokenJSON),
		ProjectId:     state.ProjectID,
		EnvironmentId: state.EnvironmentID,
		Username:      username,
		Name:          name,
		Type:          resourceType,
	})
	if err != nil {
		return "", err
	}
	return created.GetId(), nil
}

func (h *Handler) getExternalIntegration(ctx context.Context, resourceType pb.ResourceType, projectID, environmentID string) (*pb.IntegrationResource, error) {
	list, err := h.services.CompanyService().IntegrationResource().GetIntegrationResourceList(ctx, &pb.GetListIntegrationResourceRequest{
		ProjectId:     projectID,
		EnvironmentId: environmentID,
		Type:          resourceType,
	})
	if err != nil {
		return nil, err
	}
	items := list.GetIntegrationResources()
	if len(items) == 0 {
		return nil, fmt.Errorf("no %s integration found for project %s / environment %s", resourceType.String(), projectID, environmentID)
	}
	return items[0], nil
}

func (h *Handler) getExternalProviderAccessToken(ctx context.Context, resourceType pb.ResourceType, projectID, environmentID string) (string, error) {
	token, err := h.getExternalProviderToken(ctx, resourceType, projectID, environmentID)
	if err != nil {
		return "", err
	}
	return token.AccessToken, nil
}

func (h *Handler) getExternalProviderToken(ctx context.Context, resourceType pb.ResourceType, projectID, environmentID string) (externalOAuthToken, error) {
	integration, err := h.getExternalIntegration(ctx, resourceType, projectID, environmentID)
	if err != nil {
		return externalOAuthToken{}, err
	}

	token, err := decodeExternalToken(integration.GetToken())
	if err != nil {
		return externalOAuthToken{}, err
	}
	if token.AccessToken == "" {
		return externalOAuthToken{}, fmt.Errorf("stored token is empty")
	}
	if token.ExpiresAt == 0 || time.Now().Add(2*time.Minute).Unix() < token.ExpiresAt {
		return token, nil
	}
	if token.RefreshToken == "" {
		return externalOAuthToken{}, fmt.Errorf("stored token expired and has no refresh token")
	}

	var refreshed externalOAuthToken
	switch resourceType {
	case pb.ResourceType_GITLAB:
		refreshed, err = h.refreshExtGitlabToken(ctx, token.RefreshToken)
	case pb.ResourceType_BITBUCKET:
		refreshed, err = h.refreshBitbucketToken(ctx, token.RefreshToken)
	default:
		err = fmt.Errorf("unsupported provider %s", resourceType.String())
	}
	if err != nil {
		return externalOAuthToken{}, err
	}
	refreshed.BitbucketWorkspace = token.BitbucketWorkspace
	refreshed.BitbucketProjectKey = token.BitbucketProjectKey

	_, err = h.upsertExternalIntegration(ctx, resourceType, refreshed, integration.GetUsername(), integration.GetName(), githubStatePayload{
		ProjectID:     projectID,
		EnvironmentID: environmentID,
	})
	if err != nil {
		return externalOAuthToken{}, fmt.Errorf("save refreshed token: %w", err)
	}
	return refreshed, nil
}

func (h *Handler) rememberBitbucketDefaults(ctx context.Context, projectID, environmentID, workspace, projectKey string) error {
	if workspace == "" && projectKey == "" {
		return nil
	}
	integration, err := h.getExternalIntegration(ctx, pb.ResourceType_BITBUCKET, projectID, environmentID)
	if err != nil {
		return err
	}
	token, err := decodeExternalToken(integration.GetToken())
	if err != nil {
		return err
	}
	changed := false
	if workspace != "" && token.BitbucketWorkspace != workspace {
		token.BitbucketWorkspace = workspace
		changed = true
	}
	if projectKey != "" && token.BitbucketProjectKey != projectKey {
		token.BitbucketProjectKey = projectKey
		changed = true
	}
	if !changed {
		return nil
	}
	_, err = h.upsertExternalIntegration(ctx, pb.ResourceType_BITBUCKET, token, integration.GetUsername(), integration.GetName(), githubStatePayload{
		ProjectID:     projectID,
		EnvironmentID: environmentID,
	})
	return err
}

func decodeExternalToken(raw string) (externalOAuthToken, error) {
	var token externalOAuthToken
	if strings.HasPrefix(strings.TrimSpace(raw), "{") {
		if err := json.Unmarshal([]byte(raw), &token); err != nil {
			return externalOAuthToken{}, err
		}
		return token, nil
	}
	return externalOAuthToken{AccessToken: raw}, nil
}

func (h *Handler) getExternalIntegrationResponse(c *gin.Context, resourceType pb.ResourceType, label string) {
	projectID, environmentID, ok := getProjectAndEnv(c)
	if !ok {
		h.handleResponse(c, status.InvalidArgument, "project_id and environment_id required")
		return
	}
	integration, err := h.getExternalIntegration(c.Request.Context(), resourceType, projectID, environmentID)
	if err != nil {
		h.handleResponse(c, status.NotFound, "no "+label+" integration found")
		return
	}
	h.handleResponse(c, status.OK, externalIntegration{
		ID:            integration.GetId(),
		Username:      integration.GetUsername(),
		Name:          integration.GetName(),
		ProjectID:     integration.GetProjectId(),
		EnvironmentID: integration.GetEnvironmentId(),
	})
}

func (h *Handler) deleteExternalIntegration(c *gin.Context, label string) {
	id := c.Param("id")
	if id == "" {
		h.handleResponse(c, status.InvalidArgument, "integration id is required")
		return
	}
	if _, err := h.services.CompanyService().IntegrationResource().DeleteIntegrationResource(c.Request.Context(), &pb.IntegrationResourcePrimaryKey{Id: id}); err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}
	h.handleResponse(c, status.OK, label+" integration deleted")
}

func (h *Handler) resolveMirrorFunction(c *gin.Context, functionID string) (*nb.Function, *pb.ServiceResourceModel, bool) {
	projectID, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectID.(string)) {
		h.handleResponse(c, status.InvalidArgument, "project id is an invalid uuid")
		return nil, nil, false
	}
	environmentID, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentID.(string)) {
		h.handleResponse(c, status.InvalidArgument, "error getting environment id | not valid")
		return nil, nil, false
	}

	resource, err := h.services.CompanyService().ServiceResource().GetSingle(c.Request.Context(), &pb.GetSingleServiceResourceReq{
		ProjectId:     projectID.(string),
		EnvironmentId: environmentID.(string),
		ServiceType:   pb.ServiceType_BUILDER_SERVICE,
	})
	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return nil, nil, false
	}

	funcRecord, err := h.services.GoObjectBuilderService().Function().GetSingle(c.Request.Context(), &nb.FunctionPrimaryKey{
		Id:        functionID,
		ProjectId: resource.ResourceEnvironmentId,
	})
	if err != nil {
		h.handleResponse(c, status.GRPCError, fmt.Sprintf("function not found: %v", err))
		return nil, nil, false
	}
	return funcRecord, resource, true
}

func (h *Handler) webhookFunctionContext(c *gin.Context) (*nb.Function, string, string, bool) {
	functionID := c.Query("function_id")
	companyProjectID := c.Query("project_id")
	environmentID := c.Query("environment_id")
	resourceEnvID := c.Query("resource_environment_id")
	if functionID == "" || companyProjectID == "" || environmentID == "" || resourceEnvID == "" {
		h.handleResponse(c, status.BadRequest, "missing function_id, project_id, environment_id or resource_environment_id")
		return nil, "", "", false
	}

	funcRecord, err := h.services.GoObjectBuilderService().Function().GetSingle(c.Request.Context(), &nb.FunctionPrimaryKey{
		Id:        functionID,
		ProjectId: resourceEnvID,
	})
	if err != nil {
		log.Printf("[PROVIDER-WEBHOOK] no function found for id %s: %v", functionID, err)
		c.Status(http.StatusOK)
		return nil, "", "", false
	}
	return funcRecord, companyProjectID, environmentID, true
}

func (h *Handler) ensureExtGitlabProject(ctx context.Context, token, repoName string) (struct {
	ID                int    `json:"id"`
	Path              string `json:"path"`
	PathWithNamespace string `json:"path_with_namespace"`
	DefaultBranch     string `json:"default_branch"`
}, error) {
	var project struct {
		ID                int    `json:"id"`
		Path              string `json:"path"`
		PathWithNamespace string `json:"path_with_namespace"`
		DefaultBranch     string `json:"default_branch"`
	}

	user, err := h.fetchExtGitlabUser(ctx, token)
	if err != nil {
		return project, err
	}
	fullPath := url.PathEscape(user.Username + "/" + repoName)
	req, err := h.externalBearerRequest(ctx, http.MethodGet, strings.TrimRight(h.cfg.GitlabBaseUrlIntegration, "/")+"/api/v4/projects/"+fullPath, nil, token)
	if err != nil {
		return project, err
	}
	resp, err := externalHTTPClient.Do(req)
	if err != nil {
		return project, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
			return project, err
		}
		return project, nil
	}
	if resp.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(resp.Body)
		return project, fmt.Errorf("external GitLab check project returned %d: %s", resp.StatusCode, string(b))
	}

	payload, _ := json.Marshal(map[string]any{
		"name":                   repoName,
		"path":                   repoName,
		"visibility":             "private",
		"default_branch":         config.DefaultBranch,
		"initialize_with_readme": true,
	})
	req, err = h.externalBearerRequest(ctx, http.MethodPost, strings.TrimRight(h.cfg.GitlabBaseUrlIntegration, "/")+"/api/v4/projects", bytes.NewBuffer(payload), token)
	if err != nil {
		return project, err
	}
	req.Header.Set("Content-Type", "application/json")
	if err := h.doJSON(req, http.StatusCreated, &project); err != nil {
		return project, err
	}
	return project, nil
}

func (h *Handler) pushFilesToExtGitlab(ctx context.Context, token string, projectID int, branch string, files []gitlab.RepoFile) error {
	if branch == "" {
		branch = config.DefaultBranch
	}
	existing, err := h.extGitlabExistingFiles(ctx, token, projectID, branch)
	if err != nil {
		return err
	}

	actions := make([]gitlab.CommitAction, 0, len(files))
	for _, file := range files {
		action := "create"
		if existing[file.Path] {
			action = "update"
		}
		actions = append(actions, gitlab.CommitAction{
			Action:   action,
			FilePath: file.Path,
			Content:  file.Content,
			Encoding: "text",
		})
	}

	reqBody, _ := json.Marshal(gitlab.CommitRequest{
		Branch:        branch,
		CommitMessage: "Update from ucode platform",
		Actions:       actions,
	})
	req, err := h.externalBearerRequest(ctx, http.MethodPost, fmt.Sprintf("%s/api/v4/projects/%d/repository/commits", strings.TrimRight(h.cfg.GitlabBaseUrlIntegration, "/"), projectID), bytes.NewBuffer(reqBody), token)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return h.doJSON(req, http.StatusCreated, nil)
}

func (h *Handler) extGitlabExistingFiles(ctx context.Context, token string, projectID int, branch string) (map[string]bool, error) {
	result := map[string]bool{}
	page := 1
	for {
		apiURL := fmt.Sprintf("%s/api/v4/projects/%d/repository/tree?recursive=true&ref=%s&per_page=100&page=%d", strings.TrimRight(h.cfg.GitlabBaseUrlIntegration, "/"), projectID, url.QueryEscape(branch), page)
		req, err := h.externalBearerRequest(ctx, http.MethodGet, apiURL, nil, token)
		if err != nil {
			return nil, err
		}
		resp, err := externalHTTPClient.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("external GitLab tree returned %d: %s", resp.StatusCode, string(b))
		}
		var items []struct {
			Type string `json:"type"`
			Path string `json:"path"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
			resp.Body.Close()
			return nil, err
		}
		totalPages := cast.ToInt(resp.Header.Get("X-Total-Pages"))
		resp.Body.Close()
		for _, item := range items {
			if item.Type == "blob" {
				result[item.Path] = true
			}
		}
		if page >= totalPages || len(items) == 0 {
			break
		}
		page++
	}
	return result, nil
}

func (h *Handler) ensureExtGitlabWebhook(ctx context.Context, token string, projectID int, functionID, companyProjectID, environmentID, resourceEnvID string) error {
	hookURL := h.providerWebhookURL("ext-gitlab", functionID, companyProjectID, environmentID, resourceEnvID)
	listURL := fmt.Sprintf("%s/api/v4/projects/%d/hooks", strings.TrimRight(h.cfg.GitlabBaseUrlIntegration, "/"), projectID)
	req, err := h.externalBearerRequest(ctx, http.MethodGet, listURL, nil, token)
	if err != nil {
		return err
	}
	var hooks []struct {
		URL string `json:"url"`
	}
	if err := h.doJSON(req, http.StatusOK, &hooks); err != nil {
		return err
	}
	for _, hook := range hooks {
		if hook.URL == hookURL {
			return nil
		}
	}

	payload, _ := json.Marshal(map[string]any{
		"url":                     hookURL,
		"push_events":             true,
		"merge_requests_events":   false,
		"tag_push_events":         false,
		"enable_ssl_verification": true,
		"token":                   h.cfg.GitlabWebhookSecret,
	})
	req, err = h.externalBearerRequest(ctx, http.MethodPost, listURL, bytes.NewBuffer(payload), token)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return h.doJSON(req, http.StatusCreated, nil)
}

func (h *Handler) getExtGitlabFileContent(ctx context.Context, token string, projectID int, filePath, ref string) (string, error) {
	apiURL := fmt.Sprintf("%s/api/v4/projects/%d/repository/files/%s/raw?ref=%s", strings.TrimRight(h.cfg.GitlabBaseUrlIntegration, "/"), projectID, url.PathEscape(filePath), url.QueryEscape(ref))
	req, err := h.externalBearerRequest(ctx, http.MethodGet, apiURL, nil, token)
	if err != nil {
		return "", err
	}
	resp, err := externalHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("external GitLab file returned %d: %s", resp.StatusCode, string(b))
	}
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}

func (h *Handler) listBitbucketWorkspaces(ctx context.Context, token string) ([]bitbucketWorkspace, error) {
	var result []bitbucketWorkspace
	nextURL := strings.TrimRight(h.cfg.BitbucketApiBaseURL, "/") + "/user/workspaces?pagelen=100"
	for nextURL != "" {
		req, err := h.externalBearerRequest(ctx, http.MethodGet, nextURL, nil, token)
		if err != nil {
			return nil, err
		}
		var page struct {
			Next   string `json:"next"`
			Values []struct {
				Permission    string `json:"permission"`
				Administrator bool   `json:"administrator"`
				Slug          string `json:"slug"`
				Name          string `json:"name"`
				UUID          string `json:"uuid"`
				Workspace     struct {
					Slug string `json:"slug"`
					Name string `json:"name"`
					UUID string `json:"uuid"`
				} `json:"workspace"`
			} `json:"values"`
		}
		if err := h.doJSON(req, http.StatusOK, &page); err != nil {
			return nil, err
		}
		for _, item := range page.Values {
			slug := item.Workspace.Slug
			name := item.Workspace.Name
			uuid := item.Workspace.UUID
			if slug == "" {
				slug = item.Slug
			}
			if name == "" {
				name = item.Name
			}
			if uuid == "" {
				uuid = item.UUID
			}

			permission := item.Permission
			if permission == "" {
				if item.Administrator {
					permission = "admin"
				} else {
					permission = "member"
				}
			}

			result = append(result, bitbucketWorkspace{
				Slug:       slug,
				Name:       name,
				UUID:       uuid,
				Permission: permission,
				IsAdmin:    item.Administrator || permission == "admin" || permission == "owner",
			})
		}
		nextURL = page.Next
	}
	return result, nil
}

func (h *Handler) defaultBitbucketWorkspace(ctx context.Context, token string) (string, error) {
	workspaces, err := h.listBitbucketWorkspaces(ctx, token)
	if err != nil {
		return "", err
	}
	if len(workspaces) > 0 && workspaces[0].Slug != "" {
		return workspaces[0].Slug, nil
	}
	return "", fmt.Errorf("no Bitbucket workspace found")
}

func (h *Handler) listBitbucketProjects(ctx context.Context, token, workspace string) ([]bitbucketProject, error) {
	var result []bitbucketProject
	nextURL := fmt.Sprintf("%s/workspaces/%s/projects?pagelen=100", strings.TrimRight(h.cfg.BitbucketApiBaseURL, "/"), url.PathEscape(workspace))
	for nextURL != "" {
		req, err := h.externalBearerRequest(ctx, http.MethodGet, nextURL, nil, token)
		if err != nil {
			return nil, err
		}
		var page struct {
			Next   string             `json:"next"`
			Values []bitbucketProject `json:"values"`
		}
		if err := h.doJSON(req, http.StatusOK, &page); err != nil {
			return nil, err
		}
		result = append(result, page.Values...)
		nextURL = page.Next
	}
	return result, nil
}

func (h *Handler) defaultBitbucketProjectKey(ctx context.Context, token, workspace string) (string, error) {
	projects, err := h.listBitbucketProjects(ctx, token, workspace)
	if err != nil {
		return "", err
	}
	if len(projects) > 0 && projects[0].Key != "" {
		return projects[0].Key, nil
	}
	return "", fmt.Errorf("no Bitbucket project found in workspace %s", workspace)
}

func (h *Handler) ensureBitbucketRepo(ctx context.Context, token, workspace, repoSlug, projectKey string) error {
	repoURL := fmt.Sprintf("%s/repositories/%s/%s", strings.TrimRight(h.cfg.BitbucketApiBaseURL, "/"), url.PathEscape(workspace), url.PathEscape(repoSlug))
	req, err := h.externalBearerRequest(ctx, http.MethodGet, repoURL, nil, token)
	if err != nil {
		return err
	}
	resp, err := externalHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	if resp.StatusCode != http.StatusNotFound {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Bitbucket check repo returned %d: %s", resp.StatusCode, string(b))
	}

	createPayload := map[string]any{
		"scm":        "git",
		"is_private": true,
	}
	if projectKey != "" {
		createPayload["project"] = map[string]any{"key": strings.ToUpper(projectKey)}
	}
	payload, _ := json.Marshal(createPayload)
	req, err = h.externalBearerRequest(ctx, http.MethodPost, repoURL, bytes.NewBuffer(payload), token)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return h.doJSON(req, http.StatusOK, nil)
}

func (h *Handler) pushFilesToBitbucket(ctx context.Context, token, workspace, repoSlug string, files []gitlab.RepoFile) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("message", "Update from ucode platform")
	_ = writer.WriteField("branch", config.DefaultBranch)
	for _, file := range files {
		fieldPath := "/" + strings.TrimLeft(file.Path, "/")
		part, err := writer.CreateFormFile(fieldPath, path.Base(file.Path))
		if err != nil {
			return err
		}
		if _, err := io.Copy(part, strings.NewReader(file.Content)); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}

	apiURL := fmt.Sprintf("%s/repositories/%s/%s/src", strings.TrimRight(h.cfg.BitbucketApiBaseURL, "/"), url.PathEscape(workspace), url.PathEscape(repoSlug))
	req, err := h.externalBearerRequest(ctx, http.MethodPost, apiURL, &body, token)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return h.doJSON(req, http.StatusCreated, nil)
}

func (h *Handler) ensureBitbucketWebhook(ctx context.Context, token, workspace, repoSlug, functionID, companyProjectID, environmentID, resourceEnvID string) error {
	hookURL := h.providerWebhookURL("bitbucket", functionID, companyProjectID, environmentID, resourceEnvID)
	apiURL := fmt.Sprintf("%s/repositories/%s/%s/hooks", strings.TrimRight(h.cfg.BitbucketApiBaseURL, "/"), url.PathEscape(workspace), url.PathEscape(repoSlug))
	req, err := h.externalBearerRequest(ctx, http.MethodGet, apiURL, nil, token)
	if err != nil {
		return err
	}
	var hooks struct {
		Values []struct {
			URL string `json:"url"`
		} `json:"values"`
	}
	if err := h.doJSON(req, http.StatusOK, &hooks); err != nil {
		return err
	}
	for _, hook := range hooks.Values {
		if hook.URL == hookURL {
			return nil
		}
	}

	hookPayload := map[string]any{
		"description": "ucode microfrontend mirror",
		"url":         hookURL,
		"active":      true,
		"events":      []string{"repo:push"},
	}
	if h.cfg.BitbucketWebhookSecret != "" {
		hookPayload["secret"] = h.cfg.BitbucketWebhookSecret
	}
	payload, _ := json.Marshal(hookPayload)
	req, err = h.externalBearerRequest(ctx, http.MethodPost, apiURL, bytes.NewBuffer(payload), token)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return h.doJSON(req, http.StatusCreated, nil)
}

type bitbucketChangedFile struct {
	Path    string
	Removed bool
}

func (h *Handler) getBitbucketDiffstat(ctx context.Context, token, workspace, repoSlug, hash string) ([]bitbucketChangedFile, error) {
	var result []bitbucketChangedFile
	nextURL := fmt.Sprintf("%s/repositories/%s/%s/diffstat/%s?pagelen=100", strings.TrimRight(h.cfg.BitbucketApiBaseURL, "/"), url.PathEscape(workspace), url.PathEscape(repoSlug), url.PathEscape(hash))
	for nextURL != "" {
		req, err := h.externalBearerRequest(ctx, http.MethodGet, nextURL, nil, token)
		if err != nil {
			return nil, err
		}
		var page struct {
			Next   string `json:"next"`
			Values []struct {
				Status string `json:"status"`
				New    struct {
					Path string `json:"path"`
				} `json:"new"`
				Old struct {
					Path string `json:"path"`
				} `json:"old"`
			} `json:"values"`
		}
		if err := h.doJSON(req, http.StatusOK, &page); err != nil {
			return nil, err
		}
		for _, item := range page.Values {
			p := item.New.Path
			removed := item.Status == "removed"
			if removed {
				p = item.Old.Path
			}
			if p != "" {
				result = append(result, bitbucketChangedFile{Path: p, Removed: removed})
			}
		}
		nextURL = page.Next
	}
	return result, nil
}

func (h *Handler) getBitbucketFileContent(ctx context.Context, token, workspace, repoSlug, filePath, ref string) (string, error) {
	apiURL := fmt.Sprintf("%s/repositories/%s/%s/src/%s/%s", strings.TrimRight(h.cfg.BitbucketApiBaseURL, "/"), url.PathEscape(workspace), url.PathEscape(repoSlug), url.PathEscape(ref), pathEscapeSegments(filePath))
	req, err := h.externalBearerRequest(ctx, http.MethodGet, apiURL, nil, token)
	if err != nil {
		return "", err
	}
	resp, err := externalHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Bitbucket file returned %d: %s", resp.StatusCode, string(b))
	}
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}

func (h *Handler) commitProviderWebhookChanges(funcRecord *nb.Function, toCommit []*nb.McpProjectFiles, removed map[string]bool, ref, provider string) error {
	gitlabRepoID := cast.ToInt(funcRecord.GetRepoId())
	if gitlabRepoID == 0 {
		return fmt.Errorf("function %s has no gitlab repo_id", funcRecord.GetId())
	}

	if len(removed) > 0 {
		if err := h.deleteGitlabFiles(gitlabRepoID, removed, ref); err != nil {
			log.Printf("[%s-WEBHOOK] delete files on internal gitlab: %v", strings.ToUpper(provider), err)
		}
	}
	if len(toCommit) == 0 {
		return nil
	}

	gitlabCfg := gitlab.IntegrationData{
		GitlabProjectId:        gitlabRepoID,
		GitlabIntegrationUrl:   h.cfg.GitlabIntegrationURL,
		GitlabIntegrationToken: h.cfg.GitlabTokenMicroFront,
	}
	_, err := gitlab.CommitFiles(gitlabCfg, config.UGenBranch, toCommit, "mirror-"+provider+": "+ref[:min(7, len(ref))])
	return err
}

func collectGitChanges(commits []providerCommitChange) (map[string]bool, map[string]bool, map[string]bool) {
	added := map[string]bool{}
	modified := map[string]bool{}
	removed := map[string]bool{}
	for _, commit := range commits {
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
	return added, modified, removed
}

func (h *Handler) externalBearerRequest(ctx context.Context, method, apiURL string, body io.Reader, token string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, apiURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func (h *Handler) doJSON(req *http.Request, expectedStatus int, out any) error {
	resp, err := externalHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != expectedStatus {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s %s returned %d: %s", req.Method, req.URL.String(), resp.StatusCode, string(b))
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (h *Handler) providerWebhookURL(provider, functionID, companyProjectID, environmentID, resourceEnvID string) string {
	base := h.cfg.GatewayWebhookURL
	if base == "" {
		base = strings.TrimRight(h.cfg.ProjectUrl, "/") + "/v2/webhook/github"
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return base
	}
	u.Path = "/v2/webhook/" + provider
	q := u.Query()
	q.Set("function_id", functionID)
	q.Set("project_id", companyProjectID)
	q.Set("environment_id", environmentID)
	q.Set("resource_environment_id", resourceEnvID)
	u.RawQuery = q.Encode()
	return u.String()
}

var unsafeRepoChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func safeRepoSlug(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(path.Base(name), ".git")
	name = unsafeRepoChars.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-._")
	if name == "" {
		return "ucode-microfrontend"
	}
	return strings.ToLower(name)
}

func pathEscapeSegments(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func redirectWithQuery(c *gin.Context, base string, params map[string]string) {
	u, err := url.Parse(base)
	if err != nil {
		q := url.Values{}
		for key, value := range params {
			if value != "" {
				q.Set(key, value)
			}
		}
		separator := "?"
		if strings.Contains(base, "?") {
			separator = "&"
		}
		c.Redirect(http.StatusTemporaryRedirect, base+separator+q.Encode())
		return
	}

	q := u.Query()
	for key, value := range params {
		if value != "" {
			q.Set(key, value)
		}
	}
	u.RawQuery = q.Encode()
	c.Redirect(http.StatusTemporaryRedirect, u.String())
}
