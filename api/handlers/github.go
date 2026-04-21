package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
	"ucode/ucode_go_function_service/api/models"
	"ucode/ucode_go_function_service/api/status_http"
	pb "ucode/ucode_go_function_service/genproto/company_service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	githubAuthURL      = "https://github.com/login/oauth/authorize"
	githubTokenURL     = "https://github.com/login/oauth/access_token"
	githubAPIUserURL   = "https://api.github.com/user"
	githubAPIReposURL  = "https://api.github.com/user/repos"
	githubStatePrefix  = "github:state:"
	githubStateTTL     = 15 * time.Minute
	githubReposPerPage = 100
)

var githubHTTPClient = &http.Client{Timeout: 15 * time.Second}

type githubStatePayload struct {
	ProjectID     string `json:"project_id"`
	EnvironmentID string `json:"environment_id"`
	UserID        string `json:"user_id"`
}

func getProjectAndEnv(c *gin.Context) (projectID, environmentID string, ok bool) {
	pid, _ := c.Get("project_id")
	eid, _ := c.Get("environment_id")
	projectID, _ = pid.(string)
	environmentID, _ = eid.(string)
	ok = projectID != "" && environmentID != ""
	return
}

func githubAPIRequest(ctx context.Context, method, url string, body io.Reader, token string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// ─── Handlers ────────────────────────────────────────────────────────────────

// GithubConnect initiates the GitHub OAuth flow.
// Returns the GitHub authorization URL for the frontend to redirect the user to.
//
// @Security ApiKeyAuth
// @ID github_connect
// @Router /v1/github/connect [GET]
// @Summary Initiate GitHub OAuth
// @Tags GitHub Integration
// @Success 200 {object} status_http.Response{data=string} "GitHub authorization URL"
// @Failure 400 {object} status_http.Response{data=string}
// @Failure 500 {object} status_http.Response{data=string}
func (h *Handler) GithubConnect(c *gin.Context) {
	projectID, environmentID, ok := getProjectAndEnv(c)
	if !ok {
		h.handleResponse(c, status_http.InvalidArgument, "project_id and environment_id required")
		return
	}

	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	payload := githubStatePayload{
		ProjectID:     projectID,
		EnvironmentID: environmentID,
		UserID:        userIDStr,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		h.handleResponse(c, status_http.InternalServerError, "failed to encode state")
		return
	}

	state := uuid.NewString()
	redisKey := githubStatePrefix + state
	if err := h.redis.SetX(c.Request.Context(), redisKey, string(payloadBytes), githubStateTTL); err != nil {
		h.handleResponse(c, status_http.InternalServerError, "failed to store OAuth state")
		return
	}

	params := url.Values{}
	params.Set("client_id", h.cfg.GithubClientId)
	params.Set("redirect_uri", h.cfg.GithubRedirectURI)
	params.Set("state", state)
	params.Set("scope", "repo read:user user:email")

	authURL := githubAuthURL + "?" + params.Encode()
	h.handleResponse(c, status_http.OK, authURL)
}

// GithubCallback handles the OAuth callback from GitHub.
// This endpoint is public (no auth middleware) — GitHub calls it after the user grants access.
// It validates the CSRF state, exchanges the code for a token, saves the integration,
// then redirects the user back to the frontend.
//
// @ID github_callback
// @Router /v1/github/callback [GET]
// @Summary GitHub OAuth Callback
// @Tags GitHub Integration
// @Param code  query string true "Authorization code from GitHub"
// @Param state query string true "CSRF state token"
// @Success 307 "Redirect to frontend success page"
// @Failure 307 "Redirect to frontend error page"
func (h *Handler) GithubCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	errorURL := h.cfg.GithubFrontendErrorURL

	if code == "" || state == "" {
		c.Redirect(http.StatusTemporaryRedirect, errorURL+"?reason=missing_params")
		return
	}

	// Validate and consume the CSRF state from Redis (one-time use)
	redisKey := githubStatePrefix + state
	payloadStr, err := h.redis.Get(c.Request.Context(), redisKey)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, errorURL+"?reason=invalid_state")
		return
	}
	_ = h.redis.Del(c.Request.Context(), redisKey)

	var stateData githubStatePayload
	if err := json.Unmarshal([]byte(payloadStr), &stateData); err != nil {
		c.Redirect(http.StatusTemporaryRedirect, errorURL+"?reason=state_parse_error")
		return
	}

	token, err := h.exchangeGithubCode(c.Request.Context(), code)
	if err != nil {
		h.log.Error("github callback: token exchange failed: " + err.Error())
		c.Redirect(http.StatusTemporaryRedirect, errorURL+"?reason=token_exchange_failed")
		return
	}

	ghUser, err := h.fetchGithubUser(c.Request.Context(), token)
	if err != nil {
		h.log.Error("github callback: fetch user failed: " + err.Error())
		c.Redirect(http.StatusTemporaryRedirect, errorURL+"?reason=fetch_user_failed")
		return
	}

	integrationID, err := h.upsertGithubIntegration(c.Request.Context(), token, ghUser, stateData)
	if err != nil {
		h.log.Error("github callback: save integration failed: " + err.Error())
		c.Redirect(http.StatusTemporaryRedirect, errorURL+"?reason=save_failed")
		return
	}

	successParams := url.Values{}
	successParams.Set("integration_id", integrationID)
	successParams.Set("username", ghUser.Login)
	c.Redirect(http.StatusTemporaryRedirect, h.cfg.GithubFrontendSuccessURL+"?"+successParams.Encode())
}

// GithubGetIntegration returns the stored GitHub integration for the current project/environment.
//
// @Security ApiKeyAuth
// @ID github_get_integration
// @Router /v1/github/integration [GET]
// @Summary Get GitHub Integration
// @Tags GitHub Integration
// @Success 200 {object} status_http.Response{data=models.GithubIntegration}
// @Failure 400 {object} status_http.Response{data=string}
// @Failure 404 {object} status_http.Response{data=string}
func (h *Handler) GithubGetIntegration(c *gin.Context) {
	projectID, environmentID, ok := getProjectAndEnv(c)
	if !ok {
		h.handleResponse(c, status_http.InvalidArgument, "project_id and environment_id required")
		return
	}

	resp, err := h.services.CompanyService().IntegrationResource().GetIntegrationResourceList(
		c.Request.Context(),
		&pb.GetListIntegrationResourceRequest{
			ProjectId:     projectID,
			EnvironmentId: environmentID,
			Type:          pb.ResourceType_GITHUB,
		},
	)
	if err != nil {
		h.handleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	integrations := resp.GetIntegrationResources()
	if len(integrations) == 0 {
		h.handleResponse(c, status_http.NotFound, "no GitHub integration found")
		return
	}

	ir := integrations[0]
	h.handleResponse(c, status_http.OK, models.GithubIntegration{
		ID:            ir.GetId(),
		Username:      ir.GetUsername(),
		Name:          ir.GetName(),
		ProjectID:     ir.GetProjectId(),
		EnvironmentID: ir.GetEnvironmentId(),
	})
}

// GithubValidateToken checks whether the stored GitHub token is still valid.
// Calls the GitHub /user API and returns the connected user info if the token is healthy.
//
// @Security ApiKeyAuth
// @ID github_validate_token
// @Router /v1/github/integration/validate [GET]
// @Summary Validate stored GitHub token
// @Tags GitHub Integration
// @Success 200 {object} status_http.Response{data=models.GithubUser}
// @Failure 400 {object} status_http.Response{data=string}
// @Failure 401 {object} status_http.Response{data=string}
// @Failure 404 {object} status_http.Response{data=string}
func (h *Handler) GithubValidateToken(c *gin.Context) {
	projectID, environmentID, ok := getProjectAndEnv(c)
	if !ok {
		h.handleResponse(c, status_http.InvalidArgument, "project_id and environment_id required")
		return
	}

	token, err := h.getGithubToken(c.Request.Context(), projectID, environmentID)
	if err != nil {
		h.handleResponse(c, status_http.NotFound, "GitHub integration not found: "+err.Error())
		return
	}

	ghUser, err := h.fetchGithubUser(c.Request.Context(), token)
	if err != nil {
		// Token exists in DB but GitHub rejects it — needs reconnection
		h.handleResponse(c, status_http.Unauthorized, "GitHub token is invalid or revoked — please reconnect: "+err.Error())
		return
	}

	h.handleResponse(c, status_http.OK, ghUser)
}

// GithubDeleteIntegration removes a GitHub integration by ID.
//
// @Security ApiKeyAuth
// @ID github_delete_integration
// @Router /v1/github/integration/{id} [DELETE]
// @Summary Delete GitHub Integration
// @Tags GitHub Integration
// @Param id path string true "Integration ID"
// @Success 200 {object} status_http.Response{data=string}
// @Failure 400 {object} status_http.Response{data=string}
// @Failure 500 {object} status_http.Response{data=string}
func (h *Handler) GithubDeleteIntegration(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		h.handleResponse(c, status_http.InvalidArgument, "integration id is required")
		return
	}

	_, err := h.services.CompanyService().IntegrationResource().DeleteIntegrationResource(
		c.Request.Context(),
		&pb.IntegrationResourcePrimaryKey{Id: id},
	)
	if err != nil {
		h.handleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, status_http.OK, "GitHub integration deleted")
}

// GithubCreateRepo creates a new repository on the user's GitHub account.
// Uses the stored token for the current project/environment.
//
// @Security ApiKeyAuth
// @ID github_create_repo
// @Router /v1/github/repo [POST]
// @Summary Create GitHub Repository
// @Tags GitHub Integration
// @Accept  json
// @Produce json
// @Param body body models.GithubCreateRepoRequest true "Repository details"
// @Success 201 {object} status_http.Response{data=models.GithubRepo}
// @Failure 400 {object} status_http.Response{data=string}
// @Failure 404 {object} status_http.Response{data=string}
func (h *Handler) GithubCreateRepo(c *gin.Context) {
	var req models.GithubCreateRepoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.handleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	projectID, environmentID, ok := getProjectAndEnv(c)
	if !ok {
		h.handleResponse(c, status_http.InvalidArgument, "project_id and environment_id required")
		return
	}

	token, err := h.getGithubToken(c.Request.Context(), projectID, environmentID)
	if err != nil {
		h.handleResponse(c, status_http.NotFound, "GitHub integration not found: "+err.Error())
		return
	}

	repo, err := h.createGithubRepo(c.Request.Context(), token, req)
	if err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	h.handleResponse(c, status_http.Created, repo)
}

// GithubGetRepoList returns all repositories for the authenticated GitHub user.
// Uses the stored token for the current project/environment.
//
// @Security ApiKeyAuth
// @ID github_list_repos
// @Router /v1/github/repos [GET]
// @Summary List GitHub Repositories
// @Tags GitHub Integration
// @Success 200 {object} status_http.Response{data=[]models.GithubRepo}
// @Failure 400 {object} status_http.Response{data=string}
// @Failure 404 {object} status_http.Response{data=string}
func (h *Handler) GithubGetRepoList(c *gin.Context) {
	projectID, environmentID, ok := getProjectAndEnv(c)
	if !ok {
		h.handleResponse(c, status_http.InvalidArgument, "project_id and environment_id required")
		return
	}

	token, err := h.getGithubToken(c.Request.Context(), projectID, environmentID)
	if err != nil {
		h.handleResponse(c, status_http.NotFound, "GitHub integration not found: "+err.Error())
		return
	}

	repos, err := h.listGithubRepos(c.Request.Context(), token)
	if err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	h.handleResponse(c, status_http.OK, repos)
}

// ─── Private helpers ──────────────────────────────────────────────────────────

// exchangeGithubCode exchanges the OAuth authorization code for an access token.
func (h *Handler) exchangeGithubCode(ctx context.Context, code string) (string, error) {
	body := url.Values{}
	body.Set("client_id", h.cfg.GithubClientId)
	body.Set("client_secret", h.cfg.GithubClientSecret)
	body.Set("code", code)
	body.Set("redirect_uri", h.cfg.GithubRedirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, githubTokenURL, bytes.NewBufferString(body.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("github token request failed: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp models.GithubTokenExchangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("github token response decode failed: %w", err)
	}
	if tokenResp.Error != "" {
		return "", fmt.Errorf("github oauth error: %s — %s", tokenResp.Error, tokenResp.ErrorDescription)
	}
	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("github returned empty access token")
	}
	return tokenResp.AccessToken, nil
}

// fetchGithubUser calls the GitHub API to get the authenticated user's profile.
func (h *Handler) fetchGithubUser(ctx context.Context, token string) (*models.GithubUser, error) {
	req, err := githubAPIRequest(ctx, http.MethodGet, githubAPIUserURL, nil, token)
	if err != nil {
		return nil, err
	}

	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github user request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github user API returned %d: %s", resp.StatusCode, string(b))
	}

	var user models.GithubUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("github user response decode failed: %w", err)
	}
	return &user, nil
}

// upsertGithubIntegration saves the GitHub token to the company-service.
// Any existing GITHUB integration for the same project/environment is deleted first.
func (h *Handler) upsertGithubIntegration(ctx context.Context, token string, user *models.GithubUser, state githubStatePayload) (string, error) {
	// Delete any previous integration for this project/environment
	existing, err := h.services.CompanyService().IntegrationResource().GetIntegrationResourceList(ctx, &pb.GetListIntegrationResourceRequest{
		ProjectId:     state.ProjectID,
		EnvironmentId: state.EnvironmentID,
		Type:          pb.ResourceType_GITHUB,
	})
	if err == nil {
		for _, ir := range existing.GetIntegrationResources() {
			_, _ = h.services.CompanyService().IntegrationResource().DeleteIntegrationResource(ctx, &pb.IntegrationResourcePrimaryKey{Id: ir.GetId()})
		}
	}

	displayName := user.Name
	if displayName == "" {
		displayName = user.Login
	}

	created, err := h.services.CompanyService().IntegrationResource().CreateIntegrationResource(ctx, &pb.CreateIntegrationResourceRequest{
		Token:         token,
		ProjectId:     state.ProjectID,
		EnvironmentId: state.EnvironmentID,
		Username:      user.Login,
		Name:          displayName,
		Type:          pb.ResourceType_GITHUB,
	})
	if err != nil {
		return "", fmt.Errorf("create integration resource: %w", err)
	}
	return created.GetId(), nil
}

// getGithubToken retrieves the stored GitHub access token for a given project/environment.
func (h *Handler) getGithubToken(ctx context.Context, projectID, environmentID string) (string, error) {
	list, err := h.services.CompanyService().IntegrationResource().GetIntegrationResourceList(ctx, &pb.GetListIntegrationResourceRequest{
		ProjectId:     projectID,
		EnvironmentId: environmentID,
		Type:          pb.ResourceType_GITHUB,
	})
	if err != nil {
		return "", fmt.Errorf("get integration list: %w", err)
	}

	items := list.GetIntegrationResources()
	if len(items) == 0 {
		return "", fmt.Errorf("no GitHub integration found for project %s / environment %s", projectID, environmentID)
	}
	return items[0].GetToken(), nil
}

func (h *Handler) createGithubRepo(ctx context.Context, token string, req models.GithubCreateRepoRequest) (*models.GithubRepo, error) {
	bodyBytes, err := json.Marshal(map[string]any{
		"name":        req.Name,
		"description": req.Description,
		"private":     req.Private,
		"auto_init":   req.AutoInit,
	})
	if err != nil {
		return nil, err
	}

	httpReq, err := githubAPIRequest(ctx, http.MethodPost, "https://api.github.com/user/repos", bytes.NewBuffer(bodyBytes), token)
	if err != nil {
		return nil, err
	}

	resp, err := githubHTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("github create repo request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("GitHub token lacks 'repo' scope — please reconnect GitHub via /v1/github/connect")
		}
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("GitHub token is invalid or revoked — please reconnect GitHub via /v1/github/connect")
		}
		return nil, fmt.Errorf("github create repo returned %d: %s", resp.StatusCode, string(b))
	}

	var repo models.GithubRepo
	if err := json.NewDecoder(resp.Body).Decode(&repo); err != nil {
		return nil, fmt.Errorf("github create repo response decode failed: %w", err)
	}
	return &repo, nil
}

// listGithubRepos returns all repositories for the authenticated GitHub user.
// Fetches all pages automatically (GitHub returns at most 100 per page).
func (h *Handler) listGithubRepos(ctx context.Context, token string) ([]models.GithubRepo, error) {
	var all []models.GithubRepo

	for page := 1; ; page++ {
		params := url.Values{}
		params.Set("per_page", fmt.Sprintf("%d", githubReposPerPage))
		params.Set("sort", "updated")
		params.Set("page", fmt.Sprintf("%d", page))

		req, err := githubAPIRequest(ctx, http.MethodGet, githubAPIReposURL+"?"+params.Encode(), nil, token)
		if err != nil {
			return nil, err
		}

		resp, err := githubHTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("github list repos request failed: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("github list repos returned %d: %s", resp.StatusCode, string(b))
		}

		var pageRepos []models.GithubRepo
		if err := json.NewDecoder(resp.Body).Decode(&pageRepos); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("github list repos response decode failed: %w", err)
		}
		resp.Body.Close()

		all = append(all, pageRepos...)

		// Stop when GitHub returns fewer repos than the page size — last page reached
		if len(pageRepos) < githubReposPerPage {
			break
		}
	}

	return all, nil
}
