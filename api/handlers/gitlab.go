package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"ucode/ucode_go_function_service/api/models"
	"ucode/ucode_go_function_service/api/status_http"
	status "ucode/ucode_go_function_service/api/status_http"
	pb "ucode/ucode_go_function_service/genproto/company_service"
	"ucode/ucode_go_function_service/pkg/github"
	"ucode/ucode_go_function_service/pkg/gitlab"
	"ucode/ucode_go_function_service/pkg/logger"
	"ucode/ucode_go_function_service/pkg/util"

	"github.com/gin-gonic/gin"
)

// Gitlab godoc
// @ID gitlab_login
// @Router /gitlab/login [GET]
// @Summary Gitlab Login
// @Description Gitlab Login
// @Tags Gitlab
// @Accept json
// @Produce json
// @Param code query string false "code"
// @Success 201 {object} status_http.Response{data=string} "Data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *Handler) GitlabLogin(c *gin.Context) {
	var (
		code                  = c.Query("code")
		accessTokenUrl string = h.cfg.GitlabBaseUrlIntegration + "/oauth/token"
		params                = map[string]any{
			"client_id":     h.cfg.GitlabClientIdIntegration,
			"client_secret": h.cfg.GitlabClientSecretIntegration,
			"code":          code,
			"grant_type":    "authorization_code",
			"redirect_uri":  h.cfg.GitlabRedirectUriIntegration,
		}
	)

	result, err := github.MakeRequest(http.MethodPost, accessTokenUrl, "", params)
	if err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	if _, ok := result["error"]; ok {
		h.handleResponse(c, status_http.InvalidArgument, result["error_description"])
		return
	}

	h.handleResponse(c, status_http.OK, result)
}

// Gitlab godoc
// @ID gitlab_get_user
// @Router /gitlab/user [GET]
// @Summary Gitlab User
// @Description Gitlab User
// @Tags Gitlab
// @Accept json
// @Produce json
// @Param token query string false "token"
// @Success 201 {object} status_http.Response{data=models.GitlabUser} "Data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *Handler) GitlabGetUser(c *gin.Context) {
	var (
		token      = c.Query("token")
		getUserUrl = h.cfg.GitlabBaseUrlIntegration + "/api/v4/user"
		response   models.GitlabUser
	)

	resultByte, err := github.MakeRequestV1(http.MethodGet, getUserUrl, token, map[string]any{})
	if err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	if err := json.Unmarshal(resultByte, &response); err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	h.handleResponse(c, status_http.OK, response)
}

// Gitlab godoc
// @ID gitlab_get_repos
// @Router /gitlab/repos [GET]
// @Summary Gitlab Repo
// @Description Gitlab Repo
// @Tags Gitlab
// @Accept json
// @Produce json
// @Param token query string false "token"
// @Success 201 {object} status_http.Response{data=models.GitlabProjectResponse} "Data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *Handler) GitlabGetRepos(c *gin.Context) {
	var (
		token      = c.Query("token")
		url        = fmt.Sprintf("%s/api/v4/projects?membership=true", h.cfg.GitlabBaseUrlIntegration)
		response   = models.GitlabProjectResponse{}
		resourceId = c.Query("resource_id")
		resp       *http.Response
	)

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleResponse(c, status.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, status.BadRequest, "error getting environment id | not valid")
		return
	}

	resp, err := gitlab.MakeGitLabRequest(http.MethodGet, url, map[string]any{}, token)
	if err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		projectResource, err := h.services.CompanyService().Resource().GetSingleProjectResouece(
			c.Request.Context(), &pb.PrimaryKeyProjectResource{
				Id:            resourceId,
				ProjectId:     projectId.(string),
				EnvironmentId: environmentId.(string),
			},
		)
		if err != nil {
			h.handleResponse(c, status.InternalServerError, err.Error())
			return
		}

		refreshToken := projectResource.GetSettings().GetGitlab().GetRefreshToken()

		retoken, err := gitlab.RefreshGitLabToken(gitlab.GitLabTokenRequest{
			ClinetId:     h.cfg.GitlabClientIdIntegration,
			ClientSecret: h.cfg.GitlabClientSecretIntegration,
			RefreshToken: refreshToken,
		})
		if err != nil {
			h.handleResponse(c, status.InternalServerError, err.Error())
			return
		}

		go func() {
			_, err := h.services.CompanyService().Resource().UpdateProjectResource(
				context.Background(), &pb.ProjectResource{
					Id:            resourceId,
					ProjectId:     projectId.(string),
					EnvironmentId: environmentId.(string),
					Name:          projectResource.GetName(),
					Settings: &pb.Settings{
						Gitlab: &pb.Gitlab{
							Token:        retoken.AccessToken,
							RefreshToken: retoken.RefreshToken,
							Username:     projectResource.GetSettings().GetGitlab().GetUsername(),
							CreatedAt:    retoken.CreatedAt,
							ExpiresIn:    int32(retoken.ExpiresIn),
						},
					},
				},
			)
			if err != nil {
				h.log.Error("error updating project resource", logger.Error(err))
			}
		}()

		resp, err = gitlab.MakeGitLabRequest(http.MethodGet, url, map[string]any{}, retoken.AccessToken)
		if err != nil {
			h.handleResponse(c, status_http.InternalServerError, err.Error())
			return
		}
	}

	resultByte, err := io.ReadAll(resp.Body)
	if err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	if err := json.Unmarshal(resultByte, &response); err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	h.handleResponse(c, status_http.OK, response)
}

// Gitlab godoc
// @ID gitlab_get_branches
// @Router /gitlab/branches [GET]
// @Summary Gitlab Branches
// @Description Gitlab Branches
// @Tags Gitlab
// @Accept json
// @Produce json
// @Param token query string true "token"
// @Param repo_id query string true "repo_id"
// @Success 201 {object} status_http.Response{data=models.GitlabBranch} "Data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *Handler) GitlabGetBranches(c *gin.Context) {
	var (
		repoId     = c.Query("repo_id")
		token      = c.Query("token")
		resourceId = c.Query("resource_id")

		url      = fmt.Sprintf("%s/api/v4/projects/%s/repository/branches", h.cfg.GitlabBaseUrlIntegration, repoId)
		response models.GitlabBranch
	)

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleResponse(c, status.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, status.BadRequest, "error getting environment id | not valid")
		return
	}

	projectResource, err := h.services.CompanyService().Resource().GetSingleProjectResouece(
		c.Request.Context(), &pb.PrimaryKeyProjectResource{
			Id:            resourceId,
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
		},
	)
	if err != nil {
		h.handleResponse(c, status.InternalServerError, err.Error())
		return
	}

	token = projectResource.GetSettings().GetGitlab().GetToken()
	refreshToken := projectResource.GetSettings().GetGitlab().GetRefreshToken()
	createdAt := projectResource.GetSettings().GetGitlab().GetCreatedAt()
	expiresIn := projectResource.GetSettings().GetGitlab().GetExpiresIn()

	if gitlab.IsExpired(createdAt, expiresIn) {
		refresh, err := gitlab.RefreshGitLabToken(gitlab.GitLabTokenRequest{
			ClinetId:     h.cfg.GitlabClientIdIntegration,
			ClientSecret: h.cfg.GitlabClientSecretIntegration,
			RefreshToken: refreshToken,
		})
		if err != nil {
			h.handleResponse(c, status.InternalServerError, err.Error())
			return
		}

		token = refresh.AccessToken

		go func() {
			_, err := h.services.CompanyService().Resource().UpdateProjectResource(
				context.Background(), &pb.ProjectResource{
					Id:            projectResource.GetId(),
					Name:          projectResource.GetName(),
					ProjectId:     projectResource.GetProjectId(),
					EnvironmentId: projectResource.GetEnvironmentId(),
					Settings: &pb.Settings{
						Gitlab: &pb.Gitlab{
							Token:        refresh.AccessToken,
							RefreshToken: refresh.RefreshToken,
							Username:     projectResource.GetSettings().GetGitlab().GetUsername(),
							CreatedAt:    refresh.CreatedAt,
							ExpiresIn:    int32(refresh.ExpiresIn),
						},
					},
				},
			)
			if err != nil {
				h.log.Error("error updating project resource", logger.Error(err))
			}
		}()
	}

	resultByte, err := github.MakeRequestV1(http.MethodGet, url, token, map[string]any{})
	if err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	if err := json.Unmarshal(resultByte, &response); err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	h.handleResponse(c, status_http.OK, response)
}

// GitlabGetTree godoc
// @Security ApiKeyAuth
// @ID gitlab_get_tree
// @Router /gitlab/tree [GET]
// @Summary Gitlab Get Repository Tree
// @Tags Gitlab
// @Accept json
// @Produce json
// @Param project_id query string true "gitlab numeric project id"
// @Param branch query string false "branch name (default: master)"
// @Success 200 {object} status_http.Response{data=[]map[string]any} "Data"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *Handler) GitlabGetTree(c *gin.Context) {
	projectID := c.Query("project_id")
	branch := c.DefaultQuery("branch", "master")

	if projectID == "" {
		h.handleResponse(c, status_http.InvalidArgument, "project_id is required")
		return
	}

	var allItems []map[string]any
	page := 1

	for {
		apiURL := fmt.Sprintf("%s/api/v4/projects/%s/repository/tree?recursive=true&ref=%s&per_page=100&page=%d",
			h.cfg.GitlabIntegrationURL, projectID, branch, page)

		resp, err := gitlab.MakeGitLabRequest(http.MethodGet, apiURL, nil, h.cfg.GitlabKnativeToken)
		if err != nil {
			h.handleResponse(c, status_http.InternalServerError, err.Error())
			return
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var items []map[string]any
		if err := json.Unmarshal(body, &items); err != nil {
			h.handleResponse(c, status_http.InternalServerError, err.Error())
			return
		}

		allItems = append(allItems, items...)

		totalPages, _ := strconv.Atoi(resp.Header.Get("X-Total-Pages"))
		if page >= totalPages || len(items) == 0 {
			break
		}
		page++
	}

	h.handleResponse(c, status_http.OK, allItems)
}

// GitlabGetFile godoc
// @Security ApiKeyAuth
// @ID gitlab_get_file
// @Router /gitlab/file [GET]
// @Summary Gitlab Get File Content
// @Tags Gitlab
// @Accept json
// @Produce json
// @Param project_id query string true "gitlab numeric project id"
// @Param file_path query string true "file path in the repository"
// @Param branch query string false "branch name (default: master)"
// @Success 200 {object} status_http.Response{data=map[string]any} "Data"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *Handler) GitlabGetFile(c *gin.Context) {
	projectID := c.Query("project_id")
	filePath := c.Query("file_path")
	branch := c.DefaultQuery("branch", "master")

	if projectID == "" || filePath == "" {
		h.handleResponse(c, status_http.InvalidArgument, "project_id and file_path are required")
		return
	}

	encodedPath := url.PathEscape(filePath)
	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/repository/files/%s?ref=%s",
		h.cfg.GitlabIntegrationURL, projectID, encodedPath, branch)

	resp, err := gitlab.MakeGitLabRequest(http.MethodGet, apiURL, nil, h.cfg.GitlabKnativeToken)
	if err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}
	defer resp.Body.Close()

	resultByte, err := io.ReadAll(resp.Body)
	if err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	var result map[string]any
	if err := json.Unmarshal(resultByte, &result); err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	h.handleResponse(c, status_http.OK, result)
}

// GitlabUpdateFile godoc
// @Security ApiKeyAuth
// @ID gitlab_update_file
// @Router /gitlab/file [PUT]
// @Summary Gitlab Update Files
// @Description Commit one or more file changes to a GitLab repository in a single commit
// @Tags Gitlab
// @Accept json
// @Produce json
// @Param project_id query string true "gitlab numeric project id"
// @Param body body models.GitlabUpdateFileRequest true "GitlabUpdateFileRequest"
// @Success 200 {object} status_http.Response{data=map[string]any} "Data"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *Handler) GitlabUpdateFile(c *gin.Context) {
	projectID := c.Query("project_id")
	if projectID == "" {
		h.handleResponse(c, status_http.InvalidArgument, "project_id is required")
		return
	}

	var req models.GitlabUpdateFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.handleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	if len(req.Files) == 0 {
		h.handleResponse(c, status_http.InvalidArgument, "files are required")
		return
	}

	if req.Branch == "" {
		req.Branch = "master"
	}

	fileNames := make([]string, 0, len(req.Files))
	for _, f := range req.Files {
		parts := strings.Split(f.FilePath, "/")
		fileNames = append(fileNames, parts[len(parts)-1])
	}
	commitMessage := fmt.Sprintf("Update %s", strings.Join(fileNames, ", "))

	projectIdInt, err := strconv.Atoi(projectID)
	if err != nil {
		h.handleResponse(c, status_http.InvalidArgument, "project_id must be a number")
		return
	}

	cfg := gitlab.IntegrationData{
		GitlabProjectId:        projectIdInt,
		GitlabIntegrationUrl:   h.cfg.GitlabIntegrationURL,
		GitlabIntegrationToken: h.cfg.GitlabKnativeToken,
	}

	existingFiles, err := gitlab.GetRepoFilesMap(cfg)
	if err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	var actions []gitlab.CommitAction
	for _, f := range req.Files {
		action := "update"
		if !existingFiles[f.FilePath] {
			action = "create"
		}
		actions = append(actions, gitlab.CommitAction{
			Action:   action,
			FilePath: f.FilePath,
			Content:  f.Content,
			Encoding: "text",
		})
	}

	commitReq := gitlab.CommitRequest{
		Branch:        req.Branch,
		CommitMessage: commitMessage,
		Actions:       actions,
	}

	apiURL := fmt.Sprintf("%s/api/v4/projects/%s/repository/commits", h.cfg.GitlabIntegrationURL, projectID)

	resultByte, err := gitlab.DoRequestV1(apiURL, h.cfg.GitlabKnativeToken, http.MethodPost, commitReq)
	if err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	var result map[string]any
	if err := json.Unmarshal(resultByte, &result); err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	h.handleResponse(c, status_http.OK, result)
}
