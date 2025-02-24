package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/uuid"
	"github.com/spf13/cast"

	"ucode/ucode_go_function_service/api/models"
	status "ucode/ucode_go_function_service/api/status_http"
	"ucode/ucode_go_function_service/config"
	pb "ucode/ucode_go_function_service/genproto/company_service"
	nb "ucode/ucode_go_function_service/genproto/new_object_builder_service"
	obs "ucode/ucode_go_function_service/genproto/object_builder_service"
	"ucode/ucode_go_function_service/pkg/github"
	"ucode/ucode_go_function_service/pkg/gitlab"
	"ucode/ucode_go_function_service/pkg/helper"
	"ucode/ucode_go_function_service/pkg/util"

	"github.com/gin-gonic/gin"
)

// CreateMicroFrontEnd godoc
// @Security ApiKeyAuth
// @ID create_micro_frontend
// @Router /v1/functions/micro-frontend [POST]
// @Summary Create Micro Frontend
// @Description Create Micro Frontend
// @Tags MicroFrontend
// @Accept json
// @Produce json
// @Param MicroFrontend body models.CreateFunctionRequest true "MicroFrontend"
// @Success 201 {object} status.Response{data=obs.Function} "Data"
// @Response 400 {object} status.Response{data=string} "Bad Request"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) CreateMicroFrontEnd(c *gin.Context) {
	var (
		function models.CreateFunctionRequest
		response *obs.Function
	)

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	if err := c.ShouldBindJSON(&function); err != nil {
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}

	if !util.IsValidFunctionName(function.Path) {
		h.handleResponse(c, status.InvalidArgument, "function path must be contains [a-z] and hyphen and numbers")
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

	userId, _ := c.Get("user_id")

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

	environment, err := h.services.CompanyService().Environment().GetById(ctx, &pb.EnvironmentPrimaryKey{Id: environmentId.(string)})
	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	project, err := h.services.CompanyService().Project().GetById(ctx, &pb.GetProjectByIdRequest{ProjectId: environment.GetProjectId()})
	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	if len(project.GetTitle()) == 0 {
		h.handleResponse(c, status.BadRequest, "error project name is required")
		return
	}

	var projectName = strings.ReplaceAll(strings.TrimSpace(project.Title), " ", "-")
	projectName = strings.ToLower(projectName)

	var (
		functionPath   = projectName + "_" + strings.ReplaceAll(function.Path, "-", "_")
		respCreateFork gitlab.ForkResponse
	)

	if function.FrameworkType == "REACT" {
		respCreateFork, err = gitlab.CreateProjectFork(functionPath, gitlab.IntegrationData{
			GitlabIntegrationUrl:   h.cfg.GitlabIntegrationURL,
			GitlabIntegrationToken: h.cfg.GitlabTokenMicroFront,
			GitlabProjectId:        h.cfg.GitlabProjectIdMicroFrontReact,
			GitlabGroupId:          h.cfg.GitlabGroupIdMicroFront,
		})
		if err != nil {
			h.handleResponse(c, status.InvalidArgument, err.Error())
			return
		}
	} else {
		h.handleResponse(c, status.NotImplemented, "framework type is not valid, it should be [REACT]")
		return
	}

	_, err = gitlab.UpdateProject(
		gitlab.IntegrationData{
			GitlabIntegrationUrl:   h.cfg.GitlabIntegrationURL,
			GitlabIntegrationToken: h.cfg.GitlabTokenMicroFront,
			GitlabProjectId:        respCreateFork.ID,
			GitlabGroupId:          h.cfg.GitlabGroupIdMicroFront,
		}, map[string]any{
			"ci_config_path": ".gitlab-ci.yml",
		})
	if err != nil {
		h.handleResponse(c, status.InvalidArgument, err.Error())
		return
	}

	var (
		id       = uuid.New().String()
		repoHost = fmt.Sprintf("%s-%s", id, h.cfg.GitlabHostMicroFront)
		data     = make([]map[string]any, 0)
		host     = make(map[string]any)
	)

	host["key"] = "INGRESS_HOST"
	host["value"] = repoHost
	data = append(data, host)

	_, err = gitlab.CreateProjectVariable(gitlab.IntegrationData{
		GitlabIntegrationUrl:   h.cfg.GitlabIntegrationURL,
		GitlabIntegrationToken: h.cfg.GitlabTokenMicroFront,
		GitlabProjectId:        respCreateFork.ID,
		GitlabGroupId:          h.cfg.GitlabGroupIdMicroFront,
	}, host)
	if err != nil {
		h.handleResponse(c, status.InvalidArgument, err.Error())
		return
	}

	_, err = gitlab.CreatePipeline(
		gitlab.IntegrationData{
			GitlabIntegrationUrl:   h.cfg.GitlabIntegrationURL,
			GitlabIntegrationToken: h.cfg.GitlabTokenMicroFront,
			GitlabProjectId:        respCreateFork.ID,
			GitlabGroupId:          h.cfg.GitlabGroupIdMicroFront,
		}, map[string]any{
			"variables": data,
		},
	)
	if err != nil {
		h.handleResponse(c, status.InvalidArgument, err.Error())
		return
	}

	var (
		createFunction = &obs.CreateFunctionRequest{
			Path:             functionPath,
			Name:             function.Name,
			Description:      function.Description,
			ProjectId:        resource.ResourceEnvironmentId,
			EnvironmentId:    environmentId.(string),
			FunctionFolderId: function.FunctionFolderId,
			Type:             config.MICROFE,
			Url:              repoHost,
			FrameworkType:    function.FrameworkType,
			RepoId:           fmt.Sprintf("%d", respCreateFork.ID),
			Branch:           respCreateFork.DefaultBranch,
			Resource:         function.ResourceId,
		}

		logReq = &models.CreateVersionHistoryRequest{
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   config.CREATE,
			UserInfo:     cast.ToString(userId),
			Request:      createFunction,
			Services:     h.services,
		}
	)

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		response, err = h.services.GetBuilderServiceByType(resource.NodeType).Function().Create(
			ctx, createFunction,
		)
		if err != nil {
			logReq.Response = err.Error()
			h.handleResponse(c, status.GRPCError, err.Error())
		} else {
			logReq.Response = response
			h.handleResponse(c, status.OK, response)
		}
		go h.versionHistory(logReq)
	case pb.ResourceType_POSTGRESQL:
		var newCreateFunc = &nb.CreateFunctionRequest{}

		if err := helper.MarshalToStruct(createFunction, &newCreateFunc); err != nil {
			h.handleResponse(c, status.BadRequest, err.Error())
			return
		}

		response, err := h.services.GoObjectBuilderService().Function().Create(
			ctx, newCreateFunc,
		)
		if err != nil {
			logReq.Response = err.Error()
			h.handleResponse(c, status.GRPCError, err.Error())
		} else {
			logReq.Response = response
			h.handleResponse(c, status.OK, response)
		}
		go h.versionHistoryGo(c, logReq)
	}

}

// GetMicroFrontEndByID godoc
// @Security ApiKeyAuth
// @ID get_micro_frontend_by_id
// @Router /v1/functions/micro-frontend/{micro-frontend-id} [GET]
// @Summary Get Micro Frontend By Id
// @Description Get Micro Frontend By Id
// @Tags MicroFrontend
// @Accept json
// @Produce json
// @Param micro-frontend-id path string true "micro-frontend-id"
// @Success 200 {object} status.Response{data=obs.Function} "Data"
// @Response 400 {object} status.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) GetMicroFrontEndByID(c *gin.Context) {
	var functionID = c.Param("micro-frontend-id")

	if !util.IsValidUUID(functionID) {
		h.handleResponse(c, status.InvalidArgument, "function id is an invalid uuid")
		return
	}

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

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

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		function, err := h.services.GetBuilderServiceByType(resource.NodeType).Function().GetSingle(
			ctx, &obs.FunctionPrimaryKey{
				Id:        functionID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		h.handleResponse(c, status.OK, function)
	case pb.ResourceType_POSTGRESQL:
		function, err := h.services.GoObjectBuilderService().Function().GetSingle(
			ctx, &nb.FunctionPrimaryKey{
				Id:        functionID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		h.handleResponse(c, status.OK, function)
	}
}

// GetAllMicroFrontEnd godoc
// @Security ApiKeyAuth
// @ID get_all_micro_frontend
// @Router /v1/functions/micro-frontend [GET]
// @Summary Get All Micro Frontend
// @Description Get All Micro Frontend
// @Tags MicroFrontend
// @Accept json
// @Produce json
// @Param limit query number false "limit"
// @Param offset query number false "offset"
// @Param search query string false "search"
// @Success 200 {object} status.Response{data=obs.GetAllFunctionsResponse} "Data"
// @Response 400 {object} status.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) GetAllMicroFrontEnd(c *gin.Context) {
	limit, err := h.getLimitParam(c)
	if err != nil {
		h.handleResponse(c, status.InvalidArgument, err.Error())
		return
	}

	offset, err := h.getOffsetParam(c)
	if err != nil {
		h.handleResponse(c, status.InvalidArgument, err.Error())
		return
	}

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

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

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := h.services.GetBuilderServiceByType(resource.NodeType).Function().GetList(
			ctx, &obs.GetAllFunctionsRequest{
				Search:    c.DefaultQuery("search", ""),
				Limit:     int32(limit),
				Offset:    int32(offset),
				ProjectId: resource.ResourceEnvironmentId,
				Type:      []string{config.MICROFE},
			},
		)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		h.handleResponse(c, status.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := h.services.GoObjectBuilderService().Function().GetList(
			ctx,
			&nb.GetAllFunctionsRequest{
				Search:    c.DefaultQuery("search", ""),
				Limit:     int32(limit),
				Offset:    int32(offset),
				ProjectId: resource.ResourceEnvironmentId,
				Type:      []string{config.MICROFE},
			},
		)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		h.handleResponse(c, status.OK, resp)
	}
}

// UpdateMicroFrontEnd godoc
// @Security ApiKeyAuth
// @ID update_micro_frontend
// @Router /v1/functions/micro-frontend [PUT]
// @Summary Update Micro Frontend
// @Description Update Micro Frontend
// @Tags MicroFrontend
// @Accept json
// @Produce json
// @Param Data body models.Function  true "Data"
// @Success 204
// @Response 400 {object} status.Response{data=string} "Bad Request"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) UpdateMicroFrontEnd(c *gin.Context) {
	var (
		function models.Function
		resp     *empty.Empty
	)

	if err := c.ShouldBindJSON(&function); err != nil {
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

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

	userId, _ := c.Get("user_id")

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

	var (
		updateFunction = &obs.Function{
			Id:               function.ID,
			Description:      function.Description,
			Name:             function.Name,
			Path:             function.Path,
			ProjectId:        resource.ResourceEnvironmentId,
			FunctionFolderId: function.FuncitonFolderId,
			Type:             config.MICROFE,
		}

		logReq = &models.CreateVersionHistoryRequest{
			Services:     h.services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   config.UPDATE,
			UserInfo:     cast.ToString(userId),
			Request:      updateFunction,
			TableSlug:    config.FUNCTION,
		}
	)

	defer func() {
		if err != nil {
			logReq.Response = err.Error()
			h.handleResponse(c, status.GRPCError, err.Error())
		} else {
			h.handleResponse(c, status.OK, resp)
		}

		switch resource.ResourceType {
		case pb.ResourceType_MONGODB:
			go h.versionHistory(logReq)
		case pb.ResourceType_POSTGRESQL:
			go h.versionHistoryGo(c, logReq)
		}
	}()

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		_, err = h.services.GetBuilderServiceByType(resource.NodeType).Function().Update(ctx, updateFunction)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		var updateFunction = &nb.Function{}
		if err = helper.MarshalToStruct(updateFunction, &updateFunction); err != nil {
			h.handleResponse(c, status.InternalServerError, err.Error())
			return
		}

		_, err = h.services.GoObjectBuilderService().Function().Update(ctx, updateFunction)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}
	}

	h.handleResponse(c, status.NoContent, &empty.Empty{})
}

// DeleteMicroFrontEnd godoc
// @Security ApiKeyAuth
// @ID delete_micro_frontend
// @Router /v1/functions/micro-frontend/{micro-frontend-id} [DELETE]
// @Summary Delete Micro Frontend
// @Description Delete Micro Frontend
// @Tags MicroFrontend
// @Accept json
// @Produce json
// @Param micro-frontend-id path string true "micro-frontend-id"
// @Success 204
// @Response 400 {object} status.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) DeleteMicroFrontEnd(c *gin.Context) {
	var (
		functionID = c.Param("micro-frontend-id")
		deleteResp *empty.Empty
	)

	if !util.IsValidUUID(functionID) {
		h.handleResponse(c, status.InvalidArgument, "micro frontend id is an invalid uuid")
		return
	}

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

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

	userId, _ := c.Get("user_id")

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
	var resp *obs.Function

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = h.services.GetBuilderServiceByType(resource.NodeType).Function().GetSingle(
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
		goResp, err := h.services.GoObjectBuilderService().Function().GetSingle(
			ctx, &nb.FunctionPrimaryKey{
				Id:        functionID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		if err = helper.MarshalToStruct(goResp, &resp); err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}
	}

	_, err = gitlab.DeleteForkedProject(resp.Path, h.cfg)
	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     h.services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   config.DELETE,
			UserInfo:     cast.ToString(userId),
			TableSlug:    config.FUNCTION,
		}
	)

	defer func() {
		if err != nil {
			logReq.Response = err.Error()
			h.handleResponse(c, status.GRPCError, err.Error())
		} else {
			h.handleResponse(c, status.OK, deleteResp)
		}
		switch resource.ResourceType {
		case pb.ResourceType_MONGODB:
			go h.versionHistory(logReq)
		case pb.ResourceType_POSTGRESQL:
			go h.versionHistoryGo(c, logReq)
		}
	}()

	err = github.DeleteRepository(h.cfg.GitlabTokenMicroFront, cast.ToInt(resp.RepoId))
	if err != nil {
		h.handleResponse(c, status.InternalServerError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := h.services.GetBuilderServiceByType(resource.NodeType).Function().Delete(
			ctx, &obs.FunctionPrimaryKey{
				Id:        functionID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}
		h.handleResponse(c, status.NoContent, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := h.services.GoObjectBuilderService().Function().Delete(
			ctx, &nb.FunctionPrimaryKey{
				Id:        functionID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}
		h.handleResponse(c, status.NoContent, resp)
	}
}
