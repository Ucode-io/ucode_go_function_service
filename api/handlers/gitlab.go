package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
				c.Request.Context(), &pb.ProjectResource{
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
		projectId = c.Query("repo_id")
		token     = c.Query("token")
		url       = fmt.Sprintf("%s/api/v4/projects/%s/repository/branches", h.cfg.GitlabBaseUrlIntegration, projectId)
		response  models.GitlabBranch
	)

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
