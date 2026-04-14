package handlers

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"

	status "ucode/ucode_go_function_service/api/status_http"
	"ucode/ucode_go_function_service/config"
	pb "ucode/ucode_go_function_service/genproto/company_service"
	nb "ucode/ucode_go_function_service/genproto/new_object_builder_service"
	obs "ucode/ucode_go_function_service/genproto/object_builder_service"
	"ucode/ucode_go_function_service/pkg/gitlab"
	"ucode/ucode_go_function_service/pkg/helper"
	"ucode/ucode_go_function_service/pkg/util"
)

// GetFunctionCodebase godoc
// @Security ApiKeyAuth
// @ID get_function_codebase
// @Router /v2/function/{function_id}/codebase [GET]
// @Summary Get function codebase from GitLab
// @Description Returns all files of the function's GitLab repository recursively. Tries the stored branch first, then falls back to master/main.
// @Tags Function
// @Accept json
// @Produce json
// @Param function_id path string true "function_id"
// @Success 200 {object} status.Response{data=map[string][]gitlab.RepoFile} "Codebase files"
// @Response 400 {object} status.Response{data=string} "Bad Request"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) GetFunctionCodebase(c *gin.Context) {
	var functionID = c.Param("function_id")

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	if !util.IsValidUUID(functionID) {
		h.handleResponse(c, status.InvalidArgument, "function id is an invalid uuid")
		return
	}

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

	var function = &obs.Function{}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		function, err = h.services.GetBuilderServiceByType(resource.NodeType).Function().GetSingle(
			ctx, &obs.FunctionPrimaryKey{
				Id:        functionID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		resp, err := h.services.GoObjectBuilderService().Function().GetSingle(
			ctx, &nb.FunctionPrimaryKey{
				Id:        functionID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}
		if err = helper.MarshalToStruct(resp, &function); err != nil {
			h.handleResponse(c, status.InternalServerError, err.Error())
			return
		}
	}

	repoID := cast.ToInt(function.RepoId)
	if repoID == 0 {
		h.handleResponse(c, status.BadRequest, "function has no linked gitlab repository")
		return
	}

	var token string
	switch function.Type {
	case config.KNATIVE:
		token = h.cfg.GitlabKnativeToken
	case config.FUNCTION:
		token = h.cfg.GitlabOpenFassToken
	case config.MICROFE:
		token = h.cfg.GitlabTokenMicroFront
	default:
		token = h.cfg.GitlabKnativeToken
	}

	files, err := gitlab.GetRepoCodebase(h.cfg.GitlabIntegrationURL, token, repoID)
	if err != nil {
		h.handleResponse(c, status.InternalServerError, err.Error())
		return
	}

	h.handleResponse(c, status.OK, gin.H{"files": files})
}
