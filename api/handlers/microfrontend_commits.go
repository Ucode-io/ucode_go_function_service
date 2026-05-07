package handlers

import (
	"fmt"
	"log"
	"strconv"

	"ucode/ucode_go_function_service/api/models"
	status "ucode/ucode_go_function_service/api/status_http"
	"ucode/ucode_go_function_service/config"
	nb "ucode/ucode_go_function_service/genproto/new_object_builder_service"
	"ucode/ucode_go_function_service/pkg/gitlab"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	gogitlab "github.com/xanzy/go-gitlab"
)

// GetMicrofrontendCommits godoc
// @Security ApiKeyAuth
// @ID get_microfrontend_commits
// @Router /v2/functions/micro-frontend/commits [GET]
// @Summary Get published commit history of a microfrontend
// @Description Returns only pipeline-triggered commits on master (i.e. published versions).
// @Tags MicroFrontend
// @Accept json
// @Produce json
// @Param repo_id  query string true  "GitLab numeric project ID"
// @Param limit    query int    false "Number of pipelines per page (default: 20, max: 100)"
// @Param page     query int    false "Page number (default: 1)"
// @Success 200 {object} status.Response{data=[]models.GitlabCommit} "Commit list"
// @Failure 400 {object} status.Response{data=string} "Bad Request"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) GetMicrofrontendCommits(c *gin.Context) {
	repoID := c.Query("repo_id")
	if repoID == "" {
		h.handleResponse(c, status.InvalidArgument, "repo_id is required")
		return
	}

	limit := cast.ToInt(c.DefaultQuery("limit", "20"))
	page := cast.ToInt(c.DefaultQuery("page", "1"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}

	projectID, err := strconv.Atoi(repoID)
	if err != nil {
		h.handleResponse(c, status.InvalidArgument, "repo_id must be a numeric GitLab project ID")
		return
	}

	client, err := gogitlab.NewClient(
		h.cfg.GitlabTokenMicroFront,
		gogitlab.WithBaseURL(h.cfg.GitlabIntegrationURL+"/api/v4"),
	)
	if err != nil {
		h.handleResponse(c, status.InternalServerError, "failed to create gitlab client: "+err.Error())
		return
	}

	// Get the token owner so we can filter commits by this user only.
	tokenUser, _, err := client.Users.CurrentUser()
	if err != nil {
		h.handleResponse(c, status.InternalServerError, "failed to get token user: "+err.Error())
		return
	}

	// Fetch commits from u-gen — this is where all AI changes are pushed.
	commits, _, err := client.Commits.ListCommits(projectID, &gogitlab.ListCommitsOptions{
		RefName: gogitlab.Ptr(config.UGenBranch),
		ListOptions: gogitlab.ListOptions{
			PerPage: limit,
			Page:    page,
		},
	})
	if err != nil {
		h.handleResponse(c, status.InternalServerError, "gitlab request failed: "+err.Error())
		return
	}

	// Filter by token owner's email to exclude old manual commits by other users.
	result := make([]models.GitlabCommit, 0, len(commits))
	for _, cm := range commits {
		if cm.AuthorEmail != tokenUser.Email {
			continue
		}
		result = append(result, models.GitlabCommit{
			ID:            cm.ID,
			ShortID:       cm.ShortID,
			Title:         cm.Title,
			Message:       cm.Message,
			AuthorName:    cm.AuthorName,
			AuthorEmail:   cm.AuthorEmail,
			AuthoredDate:  cm.AuthoredDate.String(),
			CommitterName: cm.CommitterName,
			CommittedDate: cm.CommittedDate.String(),
			WebURL:        cm.WebURL,
		})
	}

	h.handleResponse(c, status.OK, result)
}

// GetMicrofrontendFilesAtCommit godoc
// @Security ApiKeyAuth
// @ID get_microfrontend_files_at_commit
// @Router /v2/functions/micro-frontend/files-at-commit [GET]
// @Summary Get all file contents of a microfrontend at a specific commit
// @Description Fetches the full file tree and each file's raw content at the given commit SHA for previewing a historical version.
// @Tags MicroFrontend
// @Accept json
// @Produce json
// @Param repo_id    query string true "GitLab numeric project ID"
// @Param commit_sha query string true "Commit SHA to fetch files from"
// @Success 200 {object} status.Response{data=[]models.GitlabFileChange} "File list with contents"
// @Failure 400 {object} status.Response{data=string} "Bad Request"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) GetMicrofrontendFilesAtCommit(c *gin.Context) {
	repoID := c.Query("repo_id")
	if repoID == "" {
		h.handleResponse(c, status.InvalidArgument, "repo_id is required")
		return
	}

	commitSHA := c.Query("commit_sha")
	if commitSHA == "" {
		h.handleResponse(c, status.InvalidArgument, "commit_sha is required")
		return
	}

	projectID, err := strconv.Atoi(repoID)
	if err != nil {
		h.handleResponse(c, status.InvalidArgument, "repo_id must be a numeric GitLab project ID")
		return
	}

	cfg := gitlab.IntegrationData{
		GitlabProjectId:        projectID,
		GitlabIntegrationUrl:   h.cfg.GitlabIntegrationURL,
		GitlabIntegrationToken: h.cfg.GitlabTokenMicroFront,
	}

	filePaths, err := gitlab.GetRepoFilesList(cfg, commitSHA)
	if err != nil {
		h.handleResponse(c, status.InternalServerError, "failed to fetch file tree: "+err.Error())
		return
	}
	if len(filePaths) == 0 {
		h.handleResponse(c, status.InvalidArgument, "no files found at the given commit")
		return
	}

	files := make([]models.GitlabFileChange, 0, len(filePaths))
	for _, path := range filePaths {
		content, err := gitlab.GetFileContentAtRef(cfg, path, commitSHA)
		if err != nil {
			h.handleResponse(c, status.InternalServerError, fmt.Sprintf("failed to fetch file %s: %v", path, err))
			return
		}
		files = append(files, models.GitlabFileChange{
			FilePath: path,
			Content:  content,
		})
	}

	h.handleResponse(c, status.OK, files)
}

// RevertMicrofrontendToCommit godoc
// @Security ApiKeyAuth
// @ID revert_microfrontend_to_commit
// @Router /v2/functions/micro-frontend/revert [POST]
// @Summary Revert a microfrontend to a specific commit
// @Description Restores the snapshot of the chosen master commit to the u-gen branch. The user then publishes to go live.
// @Tags MicroFrontend
// @Accept json
// @Produce json
// @Param body body models.RevertMicrofrontendRequest true "repo_id and commit_sha"
// @Success 200 {object} status.Response "Reverted successfully"
// @Failure 400 {object} status.Response{data=string} "Bad Request"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) RevertMicrofrontendToCommit(c *gin.Context) {
	var req models.RevertMicrofrontendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}

	if req.RepoID == "" {
		h.handleResponse(c, status.InvalidArgument, "repo_id is required")
		return
	}
	if req.CommitSHA == "" {
		h.handleResponse(c, status.InvalidArgument, "commit_sha is required")
		return
	}

	projectID, err := strconv.Atoi(req.RepoID)
	if err != nil {
		h.handleResponse(c, status.InvalidArgument, "repo_id must be a numeric GitLab project ID")
		return
	}

	cfg := gitlab.IntegrationData{
		GitlabProjectId:        projectID,
		GitlabIntegrationUrl:   h.cfg.GitlabIntegrationURL,
		GitlabIntegrationToken: h.cfg.GitlabTokenMicroFront,
		GitlabGroupId:          h.cfg.GitlabGroupIdMicroFront,
	}

	// 1. File tree at the chosen master commit (snapshot to restore).
	targetPaths, err := gitlab.GetRepoFilesList(cfg, req.CommitSHA)
	if err != nil {
		h.handleResponse(c, status.InternalServerError, "failed to fetch target file tree: "+err.Error())
		return
	}
	if len(targetPaths) == 0 {
		h.handleResponse(c, status.InvalidArgument, "no files found at the given commit")
		return
	}

	// 2. Current file tree on u-gen (what is there now).
	currentPaths, err := gitlab.GetRepoFilesList(cfg, config.UGenBranch)
	if err != nil {
		h.handleResponse(c, status.InternalServerError, "failed to fetch u-gen file tree: "+err.Error())
		return
	}

	currentSet := make(map[string]struct{}, len(currentPaths))
	for _, p := range currentPaths {
		currentSet[p] = struct{}{}
	}
	targetSet := make(map[string]struct{}, len(targetPaths))
	for _, p := range targetPaths {
		targetSet[p] = struct{}{}
	}

	// 3. Fetch content and build nb.McpProjectFiles for create/update actions.
	nbFiles := make([]*nb.McpProjectFiles, 0, len(targetPaths))
	for _, path := range targetPaths {
		content, err := gitlab.GetFileContentAtRef(cfg, path, req.CommitSHA)
		if err != nil {
			h.handleResponse(c, status.InternalServerError, fmt.Sprintf("failed to fetch file %s: %v", path, err))
			return
		}
		nbFiles = append(nbFiles, &nb.McpProjectFiles{
			Path:    path,
			Content: content,
		})
	}

	// 4. Push create/update files to u-gen using the existing CommitFiles helper.
	log.Printf("[REVERT] pushing %d file(s) to repo_id=%d branch=%s", len(nbFiles), projectID, config.UGenBranch)
	if _, err = gitlab.CommitFiles(cfg, config.UGenBranch, nbFiles, fmt.Sprintf("revert: restore snapshot from commit %s", req.CommitSHA)); err != nil {
		h.handleResponse(c, status.InternalServerError, "failed to push files to u-gen: "+err.Error())
		return
	}

	// 5. Delete files that exist on u-gen but not in the target snapshot.
	var deleteActions []gitlab.CommitAction
	for _, path := range currentPaths {
		deleteActions = append(deleteActions, gitlab.CommitAction{
			Action:   "delete",
			FilePath: path,
		})
	}

	if len(deleteActions) > 0 {
		log.Printf("[REVERT] deleting %d stale file(s) from u-gen", len(deleteActions))
		commitReq := gitlab.CommitRequest{
			Branch:        config.UGenBranch,
			CommitMessage: fmt.Sprintf("revert: remove stale files from commit %s", req.CommitSHA),
			Actions:       deleteActions,
		}
		apiURL := fmt.Sprintf("%s/api/v4/projects/%d/repository/commits",
			h.cfg.GitlabIntegrationURL, projectID)
		if _, err = gitlab.DoRequestV1(apiURL, h.cfg.GitlabTokenMicroFront, "POST", commitReq); err != nil {
			h.handleResponse(c, status.InternalServerError, "failed to delete stale files from u-gen: "+err.Error())
			return
		}
	}

	h.handleResponse(c, status.OK, gin.H{
		"message":    fmt.Sprintf("Microfrontend reverted to commit %s on %s branch. Publish to go live.", req.CommitSHA, config.UGenBranch),
		"commit_sha": req.CommitSHA,
		"files":      len(nbFiles),
	})
}
