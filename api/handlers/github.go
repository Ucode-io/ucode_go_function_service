package handlers

import (
	"encoding/json"
	"fmt"
	"ucode/ucode_go_function_service/api/models"
	"ucode/ucode_go_function_service/api/status_http"
	"ucode/ucode_go_function_service/pkg/github"

	"github.com/gin-gonic/gin"
)

// Github godoc
// @ID github_login
// @Router /github/login [GET]
// @Summary Github Login
// @Description Github Login
// @Tags Github
// @Accept json
// @Produce json
// @Param code query number false "code"
// @Success 201 {object} status_http.Response{data=string} "Data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *Handler) GithubLogin(c *gin.Context) {
	var (
		code                  = c.Query("code")
		accessTokenUrl string = h.cfg.GithubBaseUrl + "/login/oauth/access_token"
	)

	param := map[string]interface{}{
		"client_id":     h.cfg.GithubClientId,
		"client_secret": h.cfg.GithubClientSecret,
		"code":          code,
	}

	result, err := github.MakeRequest("POST", accessTokenUrl, "", param)
	if err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	sfdasfdas, _ := json.Marshal(result)
	fmt.Println("result", string(sfdasfdas))

	if _, ok := result["error"]; ok {
		h.handleResponse(c, status_http.InvalidArgument, result["error_description"])
		return
	}

	h.handleResponse(c, status_http.OK, result)
}

// Github godoc
// @ID github_get_user
// @Router /github/user [GET]
// @Summary Github User
// @Description Github User
// @Tags Github
// @Accept json
// @Produce json
// @Param token query string false "token"
// @Success 201 {object} status_http.Response{data=models.GithubUser} "Data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *Handler) GithubGetUser(c *gin.Context) {
	var (
		token      = c.Query("token")
		getUserUrl = "https://api.github.com/user"
		response   models.GithubUser
	)

	resultByte, err := github.MakeRequestV1("GET", getUserUrl, token, map[string]interface{}{})
	if err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	if err := json.Unmarshal(resultByte, &response); err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	if response.Status == "401" {
		h.handleResponse(c, status_http.BadRequest, "can not find username wrong token format")
		return
	}

	h.handleResponse(c, status_http.OK, response)
}

// Github godoc
// @ID github_get_repos
// @Router /github/repos [GET]
// @Summary Github Repo
// @Description Github Repo
// @Tags Github
// @Accept json
// @Produce json
// @Param token query string false "token"
// @Param username query string false "username"
// @Success 201 {object} status_http.Response{data=models.GithubRepo} "Data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *Handler) GithubGetRepos(c *gin.Context) {
	var (
		username = c.Query("username")
		token    = c.Query("token")
		url      = fmt.Sprintf("https://api.github.com/users/%s/repos", username)
		response = models.GithubRepo{}
	)

	resultByte, err := github.MakeRequestV1("GET", url, token, map[string]any{})
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

// Github godoc
// @ID github_get_branches
// @Router /github/branches [GET]
// @Summary Github Branches
// @Description Github Branches
// @Tags Github
// @Accept json
// @Produce json
// @Param token query string false "token"
// @Param username query string false "username"
// @Success 201 {object} status_http.Response{data=models.GithubRepo} "Data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *Handler) GithubGetBranches(c *gin.Context) {
	var (
		username = c.Query("username")
		repoName = c.Query("repo")
		token    = c.Query("token")

		url      = fmt.Sprintf("https://api.github.com/repos/%s/%s/branches", username, repoName)
		response models.GithubBranch
	)

	resultByte, err := github.MakeRequestV1("GET", url, token, map[string]interface{}{})
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
