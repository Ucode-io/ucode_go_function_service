package handlers

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"ucode/ucode_go_function_service/api/models"
	"ucode/ucode_go_function_service/api/status_http"
	status "ucode/ucode_go_function_service/api/status_http"
	"ucode/ucode_go_function_service/config"
	as "ucode/ucode_go_function_service/genproto/auth_service"
	pb "ucode/ucode_go_function_service/genproto/company_service"
	nb "ucode/ucode_go_function_service/genproto/new_object_builder_service"
	obs "ucode/ucode_go_function_service/genproto/object_builder_service"
	"ucode/ucode_go_function_service/pkg/github"
	"ucode/ucode_go_function_service/pkg/gitlab"
	"ucode/ucode_go_function_service/pkg/helper"
	"ucode/ucode_go_function_service/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/uuid"
	"github.com/spf13/cast"
)

// CreateNewFunction godoc
// @Security ApiKeyAuth
// @ID create_new_function
// @Router /v1/function [POST]
// @Summary Create New Function
// @Description Create New Function
// @Tags Function
// @Accept json
// @Produce json
// @Param Function body models.CreateFunctionRequest true "CreateFunctionRequestBody"
// @Success 204
// @Response 400 {object} status.Response{data=string} "Bad Request"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) CreateFunction(c *gin.Context) {
	var function models.CreateFunctionRequest

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	if err := c.ShouldBindJSON(&function); err != nil {
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
		h.handleResponse(c, status.BadRequest, "error getting environment id | not valid")
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
		h.handleResponse(c, status.InternalServerError, err.Error())
		return
	}

	if len(project.GetTitle()) == 0 {
		h.handleResponse(c, status.BadRequest, "error project name is required")
		return
	}

	if len(project.GetFareId()) != 0 {
		var count int32
		switch resource.ResourceType {
		case pb.ResourceType_MONGODB:
			response, err := h.services.GetBuilderServiceByType(resource.NodeType).Function().GetCountByType(ctx, &obs.GetCountByTypeRequest{
				ProjectId: resource.ResourceEnvironmentId,
				Type:      []string{config.KNATIVE, config.FUNCTION},
			})
			if err != nil {
				h.handleResponse(c, status.GRPCError, err.Error())
				return
			}
			count = response.Count
		case pb.ResourceType_POSTGRESQL:
			response, err := h.services.GoObjectBuilderService().Function().GetCountByType(ctx, &nb.GetCountByTypeRequest{
				ProjectId: resource.ResourceEnvironmentId,
				Type:      []string{config.KNATIVE, config.FUNCTION},
			})
			if err != nil {
				h.handleResponse(c, status.GRPCError, err.Error())
				return
			}
			count = response.Count
		}

		response, err := h.services.CompanyService().Billing().CompareFunction(ctx, &pb.CompareFunctionRequest{
			Type:   config.FARE_FUNCTION,
			FareId: project.GetFareId(),
			Count:  count,
		})
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		if !response.HasAccess {
			h.handleResponse(c, status.BadRequest, "you have reached limit of fass")
			return
		}
	}

	var projectName = strings.ReplaceAll(strings.TrimSpace(project.Title), " ", "-")
	projectName = strings.ToLower(projectName)

	var (
		functionPath = projectName + "-" + function.Path
		uuid         = uuid.New()
		url          = "https://" + uuid.String() + ".u-code.io"
		resp         gitlab.ForkResponse
		gitlabToken  string
	)

	switch function.Type {
	case config.FUNCTION:
		resp, err = gitlab.CreateProjectFork(functionPath, gitlab.IntegrationData{
			GitlabIntegrationUrl:   h.cfg.GitlabIntegrationURL,
			GitlabIntegrationToken: h.cfg.GitlabOpenFassToken,
			GitlabGroupId:          h.cfg.GitlabOpenFassGroupId,
			GitlabProjectId:        h.cfg.GitlabOpenFassProjectId,
		})
		if err != nil {
			h.handleResponse(c, status.InvalidArgument, err.Error())
			return
		}
		gitlabToken = h.cfg.GitlabOpenFassToken
	case config.KNATIVE:
		resp, err = gitlab.CreateProjectFork(functionPath, gitlab.IntegrationData{
			GitlabIntegrationUrl:   h.cfg.GitlabIntegrationURL,
			GitlabIntegrationToken: h.cfg.GitlabKnativeToken,
			GitlabGroupId:          h.cfg.GitlabKnativeGroupId,
			GitlabProjectId:        h.cfg.GitlabKnativeProjectId,
		})
		if err != nil {
			h.handleResponse(c, status.InvalidArgument, err.Error())
			return
		}
		gitlabToken = h.cfg.GitlabKnativeToken
	default:
		h.handleResponse(c, status.BadRequest, "not supported function type")
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
			Url:              url,
			Type:             function.Type,
			Resource:         function.ResourceId,
			RepoId:           fmt.Sprintf("%d", resp.ID),
			Branch:           resp.DefaultBranch,
		}

		logReq = &models.CreateVersionHistoryRequest{
			Services:     h.services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   config.CREATE,
			UserInfo:     cast.ToString(userId),
			Request:      createFunction,
			TableSlug:    config.FUNCTION,
		}
	)

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		response, err := h.services.GetBuilderServiceByType(resource.NodeType).Function().Create(ctx, createFunction)
		if err != nil {
			logReq.Response = err.Error()
			github.DeleteRepository(gitlabToken, cast.ToInt(resp.ID))
			h.handleResponse(c, status.GRPCError, err.Error())
		} else {
			logReq.Response = response
			h.handleResponse(c, status.Created, response)
		}

		go h.versionHistory(logReq)
	case pb.ResourceType_POSTGRESQL:
		var newCreateFunction = &nb.CreateFunctionRequest{}

		if err = helper.MarshalToStruct(createFunction, &newCreateFunction); err != nil {
			h.handleResponse(c, status.InternalServerError, err.Error())
			return
		}

		response, err := h.services.GoObjectBuilderService().Function().Create(ctx, newCreateFunction)
		if err != nil {
			logReq.Response = err.Error()
			github.DeleteRepository(gitlabToken, cast.ToInt(resp.ID))
			h.handleResponse(c, status.GRPCError, err.Error())
		} else {
			logReq.Response = response
			h.handleResponse(c, status.Created, response)
		}

		go h.versionHistoryGo(c, logReq)
	}
}

// GetNewFunctionByID godoc
// @Security ApiKeyAuth
// @ID get_new_function_by_id
// @Router /v1/function/{function_id} [GET]
// @Summary Get Function by id
// @Description Get Function by id
// @Tags Function
// @Accept json
// @Produce json
// @Param function_id path string true "function_id"
// @Success 200 {object} status.Response{data=obs.Function} "FunctionBody"
// @Response 400 {object} status.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) GetFunctionByID(c *gin.Context) {
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
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}
	}

	h.handleResponse(c, status.OK, function)
}

// GetAllNewFunctions godoc
// @Security ApiKeyAuth
// @ID get_all_new_functions
// @Router /v1/function [GET]
// @Summary Get all functions
// @Description Get all functions
// @Tags Function
// @Accept json
// @Produce json
// @Param limit query number false "limit"
// @Param offset query number false "offset"
// @Param search query string false "search"
// @Success 200 {object} status.Response{data=obs.GetAllFunctionsResponse} "FunctionBody"
// @Response 400 {object} status.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) GetAllFunctions(c *gin.Context) {
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

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
		ctx,
		&pb.GetSingleServiceResourceReq{
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
		h.handleResponse(c, status.GRPCError, "error getting resource environment id")
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := h.services.GetBuilderServiceByType(resource.NodeType).Function().GetList(
			ctx, &obs.GetAllFunctionsRequest{
				Search:        c.Query("search"),
				Limit:         int32(limit),
				Offset:        int32(offset),
				ProjectId:     resource.ResourceEnvironmentId,
				EnvironmentId: environment.GetId(),
				Type:          []string{config.FUNCTION, config.KNATIVE},
				FunctionId:    c.Query("function_id"),
			},
		)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		h.handleResponse(c, status.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := h.services.GoObjectBuilderService().Function().GetList(
			ctx, &nb.GetAllFunctionsRequest{
				Search:        c.Query("search"),
				Limit:         int32(limit),
				Offset:        int32(offset),
				ProjectId:     resource.ResourceEnvironmentId,
				EnvironmentId: environment.GetId(),
				Type:          []string{config.FUNCTION, config.KNATIVE},
				FunctionId:    c.Query("function_id"),
			},
		)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		h.handleResponse(c, status.OK, resp)
	}
}

// UpdateNewFunction godoc
// @Security ApiKeyAuth
// @ID update_new_function
// @Router /v1/function [PUT]
// @Summary Update new function
// @Description Update new function
// @Tags Function
// @Accept json
// @Produce json
// @Param Function body models.Function  true "UpdateFunctionRequestBody"
// @Success 204
// @Response 400 {object} status.Response{data=string} "Bad Request"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) UpdateFunction(c *gin.Context) {
	var (
		function models.Function
		resp     = &empty.Empty{}
	)

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	if err := c.ShouldBindJSON(&function); err != nil {
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
		h.handleResponse(c, status.BadRequest, "error getting environment id | not valid")
		return
	}

	userId, _ := c.Get("user_id")

	resource, err := h.services.CompanyService().ServiceResource().GetSingle(
		ctx,
		&pb.GetSingleServiceResourceReq{
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

	var (
		updateFunction = &obs.Function{
			Id:               function.ID,
			Description:      function.Description,
			Name:             function.Name,
			Path:             function.Path,
			EnvironmentId:    environment.GetId(),
			ProjectId:        resource.ResourceEnvironmentId,
			FunctionFolderId: function.FuncitonFolderId,
		}
		logReq = &models.CreateVersionHistoryRequest{
			Services:     h.services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   config.UPDATE,
			UserInfo:     cast.ToString(userId),
			Request:      &updateFunction,
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
		resp, err = h.services.GetBuilderServiceByType(resource.NodeType).Function().Update(ctx, updateFunction)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		updateFunction := &nb.Function{}

		if err = helper.MarshalToStruct(updateFunction, &updateFunction); err != nil {
			h.handleResponse(c, status.InternalServerError, err.Error())
			return
		}

		resp, err = h.services.GoObjectBuilderService().Function().Update(ctx, updateFunction)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}
	}

	h.handleResponse(c, status.NoContent, nil)
}

// DeleteNewFunction godoc
// @Security ApiKeyAuth
// @ID delete_new_function
// @Router /v1/function/{function_id} [DELETE]
// @Summary Delete New Function
// @Description Delete New Function
// @Tags Function
// @Accept json
// @Produce json
// @Param function_id path string true "function_id"
// @Success 204
// @Response 400 {object} status.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) DeleteFunction(c *gin.Context) {
	var (
		functionID = c.Param("function_id")
		resp       *obs.Function
	)

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

	switch resp.Type {
	case config.FUNCTION:
		err = github.DeleteRepository(h.cfg.GitlabOpenFassToken, cast.ToInt(resp.RepoId))
		if err != nil {
			h.handleResponse(c, status.InternalServerError, err.Error())
			return
		}
	case config.KNATIVE:
		err = github.DeleteRepository(h.cfg.GitlabKnativeToken, cast.ToInt(resp.RepoId))
		if err != nil {
			h.handleResponse(c, status.InternalServerError, err.Error())
			return
		}
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
			h.handleResponse(c, status.NoContent, resp)
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
		_, err = h.services.GetBuilderServiceByType(resource.NodeType).Function().Delete(
			ctx, &obs.FunctionPrimaryKey{
				Id:        functionID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.handleResponse(c, status.NoContent, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		_, err = h.services.GoObjectBuilderService().Function().Delete(
			ctx, &nb.FunctionPrimaryKey{
				Id:        functionID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.handleResponse(c, status.NoContent, err.Error())
			return
		}
	}
}

// GetAllNewFunctionsForApp godoc
// @Security ApiKeyAuth
// @ID get_all_new_functions_for_app
// @Router /v1/function-for-app [GET]
// @Summary Get all functions
// @Description Get all functions
// @Tags Function
// @Accept json
// @Produce json
// @Param limit query number false "limit"
// @Param offset query number false "offset"
// @Param search query string false "search"
// @Success 200 {object} status.Response{data=obs.GetAllFunctionsResponse} "FunctionBody"
// @Response 400 {object} status.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) GetAllFunctionsForApp(c *gin.Context) {
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

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
		ctx,
		&pb.GetSingleServiceResourceReq{
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
				Type:      []string{config.FUNCTION, config.KNATIVE},
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
				Type:      []string{config.FUNCTION, config.KNATIVE},
			},
		)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		h.handleResponse(c, status.OK, resp)
	}
}

// InvokeFunctionByPath godoc
// @Security ApiKeyAuth
// @Param function-path path string true "function-path"
// @ID invoke_function_by_path_openfass
// @Router /v1/invoke_function/{function-path} [POST]
// @Summary Invoke Function By Path
// @Description Invoke Function By Path
// @Tags InvokeFunction
// @Accept json
// @Produce json
// @Param InvokeFunctionByPathRequest body models.CommonMessage true "InvokeFunctionByPathRequest"
// @Success 201 {object} status.Response{data=models.InvokeFunctionRequest} "Function data"
// @Response 400 {object} status.Response{data=string} "Bad Request"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) InvokeFunctionByPath(c *gin.Context) {
	var invokeFunction models.CommonMessage

	if err := c.ShouldBindJSON(&invokeFunction); err != nil {
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

	apiKeys, err := h.services.AuthService().ApiKey().GetList(ctx, &as.GetListReq{
		EnvironmentId: environmentId.(string),
		ProjectId:     resource.ProjectId,
	})
	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	if len(apiKeys.Data) < 1 {
		h.handleResponse(c, status.InvalidArgument, "Api key not found")
		return
	}
	authInfo, _ := h.GetAuthInfo(c)

	invokeFunction.Data["user_id"] = authInfo.GetUserId()
	invokeFunction.Data["project_id"] = authInfo.GetProjectId()
	invokeFunction.Data["environment_id"] = authInfo.GetEnvId()
	invokeFunction.Data["app_id"] = apiKeys.GetData()[0].GetAppId()

	resp, err := util.DoRequest(h.cfg.OpeFassBaseUrl+c.Param("function-path"), http.MethodPost,
		models.NewInvokeFunctionRequest{Data: invokeFunction.Data},
	)
	if err != nil {
		h.handleResponse(c, status.InvalidArgument, err.Error())
		return
	} else if resp.Status == "error" {
		var errStr = resp.Status
		if resp.Data != nil && resp.Data["message"] != nil {
			errStr = resp.Data["message"].(string)
		}
		h.handleResponse(c, status.InvalidArgument, errStr)
		return
	}

	h.handleResponse(c, status.Created, resp)
}

// InvokeFunction godoc
// @Security ApiKeyAuth
// @ID invoke_function
// @Router /v1/invoke_function [POST]
// @Summary Invoke Function
// @Description Invoke Function
// @Tags InvokeFunction
// @Accept json
// @Produce json
// @Param InvokeFunctionRequest body models.InvokeFunctionRequest true "InvokeFunctionRequest"
// @Success 201 {object} status.Response{data=models.InvokeFunctionRequest} "Function data"
// @Response 400 {object} status.Response{data=string} "Bad Request"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) InvokeFunction(c *gin.Context) {
	var invokeFunction models.InvokeFunctionRequest

	if err := c.ShouldBindJSON(&invokeFunction); err != nil {
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
				Id:        invokeFunction.FunctionID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		newFunction, err := h.services.GoObjectBuilderService().Function().GetSingle(
			ctx, &nb.FunctionPrimaryKey{
				Id:        invokeFunction.FunctionID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		err = helper.MarshalToStruct(newFunction, &function)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}
	}

	apiKeys, err := h.services.AuthService().ApiKey().GetList(context.Background(), &as.GetListReq{
		EnvironmentId: environmentId.(string),
		ProjectId:     resource.ProjectId,
	})
	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	if len(apiKeys.Data) < 1 {
		h.handleResponse(c, status.InvalidArgument, "Api key not found")
		return
	}

	if invokeFunction.Attributes == nil {
		invokeFunction.Attributes = make(map[string]any, 0)
	}

	authInfo, _ := h.GetAuthInfo(c)

	resp, err := util.DoRequest("https://ofs.u-code.io/function/"+function.Path, "POST", models.NewInvokeFunctionRequest{
		Data: map[string]any{
			"object_ids":     invokeFunction.ObjectIDs,
			"app_id":         apiKeys.GetData()[0].GetAppId(),
			"attributes":     invokeFunction.Attributes,
			"user_id":        authInfo.GetUserId(),
			"project_id":     projectId,
			"environment_id": environmentId,
			"action_type":    "HTTP",
			"table_slug":     invokeFunction.TableSlug,
		},
	})
	if err != nil {
		h.handleResponse(c, status.InvalidArgument, err.Error())
		return
	} else if resp.Status == "error" {
		var errStr = resp.Status
		if resp.Data != nil && resp.Data["message"] != nil {
			errStr = resp.Data["message"].(string)
		}
		h.handleResponse(c, status.InvalidArgument, errStr)
		return
	}
	if c.Query("form_input") != "true" && c.Query("use_no_limit") != "true" {
		switch resource.ResourceType {
		case pb.ResourceType_MONGODB:
			_, err = h.services.GetBuilderServiceByType(resource.NodeType).CustomEvent().UpdateByFunctionId(
				ctx, &obs.UpdateByFunctionIdRequest{
					FunctionId: invokeFunction.FunctionID,
					ObjectIds:  invokeFunction.ObjectIDs,
					FieldSlug:  function.Path + "_disable",
					ProjectId:  resource.ResourceEnvironmentId,
				},
			)
			if err != nil {
				h.handleResponse(c, status.GRPCError, err.Error())
				return
			}
		case pb.ResourceType_POSTGRESQL:
			_, err = h.services.GoObjectBuilderService().CustomEvent().UpdateByFunctionId(
				ctx, &nb.UpdateByFunctionIdRequest{
					FunctionId: invokeFunction.FunctionID,
					ObjectIds:  invokeFunction.ObjectIDs,
					FieldSlug:  function.Path + "_disable",
					ProjectId:  resource.ResourceEnvironmentId,
				},
			)
			if err != nil {
				h.handleResponse(c, status.GRPCError, err.Error())
				return
			}
		}
	}
	h.handleResponse(c, status.Created, resp)
}

// InvokeFunctionByPath godoc
// @Security ApiKeyAuth
// @Param function-path path string true "function-path"
// @ID invoke_function_by_path_knative
// @Router /v2/invoke_function/{function-path} [POST]
// @Summary Invoke Function By Path
// @Description Invoke Function By Path
// @Tags Function
// @Accept json
// @Produce json
// @Param InvokeFunctionByPathRequest body models.CommonMessage true "InvokeFunctionByPathRequest"
// @Success 201 {object} status.Response{data=models.InvokeFunctionRequest} "Function data"
// @Response 400 {object} status.Response{data=string} "Bad Request"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) InvokeFuncByPath(c *gin.Context) {
	var (
		invokeFunction models.CommonMessage
		path                = c.Param("function-path")
		permission     bool = true
		apiKey         models.ApiKey
		isPublic       bool
	)

	if err := c.ShouldBindJSON(&invokeFunction); err != nil {
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
		err := errors.New("error getting environment id | not valid")
		h.handleResponse(c, status.BadRequest, err)
		return
	}

	access, exists := c.Get("access")
	if exists {
		permission = access.(bool)
	}

	resourceBody, exist := h.cache.Get(fmt.Sprintf("project:%s:env:%s", projectId.(string), environmentId.(string)))
	if !exist {
		resource, err := h.services.CompanyService().ServiceResource().GetSingle(
			c.Request.Context(),
			&pb.GetSingleServiceResourceReq{
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
				c.Request.Context(), &obs.FunctionPrimaryKey{
					ProjectId: resource.ResourceEnvironmentId,
					Path:      path,
				},
			)
			if err != nil {
				h.handleResponse(c, status_http.GRPCError, err.Error())
				return
			}

			isPublic = function.GetIsPublic()

			if !permission && !isPublic {
				h.handleResponse(c, status_http.Unauthorized, config.AccessDeniedError)
				return
			}
		case pb.ResourceType_POSTGRESQL:
			function, err := h.services.GoObjectBuilderService().Function().GetSingle(
				c.Request.Context(), &nb.FunctionPrimaryKey{
					ProjectId: resource.ResourceEnvironmentId,
					Path:      path,
				},
			)
			if err != nil {
				h.handleResponse(c, status_http.GRPCError, err.Error())
				return
			}

			isPublic = function.GetIsPublic()

			if !permission && !isPublic {
				h.handleResponse(c, status_http.Unauthorized, config.AccessDeniedError)
				return
			}
		}

		apiKeys, err := h.services.AuthService().ApiKey().GetList(c.Request.Context(), &as.GetListReq{
			EnvironmentId: environmentId.(string),
			ProjectId:     resource.ProjectId,
			Limit:         1,
			Offset:        0,
		})
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}
		if len(apiKeys.Data) < 1 {
			h.handleResponse(c, status.InvalidArgument, "Api key not found")
			return
		}

		apiKey = models.ApiKey{
			AppId:    apiKeys.GetData()[0].GetAppId(),
			IsPublic: isPublic,
		}

		appIdByte, err := json.Marshal(apiKey)
		if err != nil {
			h.handleResponse(c, status.InvalidArgument, err.Error())
			return
		}

		h.cache.Add(fmt.Sprintf("project:%s:env:%s", projectId.(string), environmentId.(string)), appIdByte, config.REDIS_KEY_TIMEOUT)
	} else {
		if err := json.Unmarshal(resourceBody, &apiKey); err != nil {
			h.handleResponse(c, status.InvalidArgument, err.Error())
			return
		}

		if !permission && !apiKey.IsPublic {
			h.handleResponse(c, status_http.Unauthorized, config.AccessDeniedError)
			return
		}
	}

	authInfo, _ := h.GetAuthInfo(c)

	invokeFunction.Data["user_id"] = authInfo.GetUserId()
	invokeFunction.Data["project_id"] = authInfo.GetProjectId()
	invokeFunction.Data["environment_id"] = authInfo.GetEnvId()
	invokeFunction.Data["app_id"] = apiKey.AppId
	request := models.NewInvokeFunctionRequest{Data: invokeFunction.Data}

	resp, err := h.ExecKnative(path, request)
	if err != nil {
		h.handleResponse(c, status.InvalidArgument, err.Error())
		return
	} else if resp.Status == "error" {
		var errStr = resp.Status
		if resp.Data != nil && resp.Data["message"] != nil {
			errStr = resp.Data["message"].(string)
		}
		h.handleResponse(c, status.InvalidArgument, errStr)
		return
	}

	h.handleResponse(c, status.Created, resp)
}

func (h *Handler) InvokeFuncByApiPath(c *gin.Context) {
	var (
		invokeFunction map[string]any
		path                = c.Param("function-path")
		permission     bool = true
		apiKey         models.ApiKey
		isPublic       bool
		apiPath        = c.Param("any")
		headers        = make(map[string]string)
	)

	if err := c.ShouldBindJSON(&invokeFunction); err != nil {
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
		err := errors.New("error getting environment id | not valid")
		h.handleResponse(c, status.BadRequest, err)
		return
	}

	// Get the headers from the request and add them to the headers map
	for key, values := range c.Request.Header {
		for _, value := range values {
			headers[key] = value
		}
	}

	access, exists := c.Get("access")
	if exists {
		permission = access.(bool)
	}

	fmt.Println("Before")
	resourceBody, exist := h.cache.Get(fmt.Sprintf("project:%s:env:%s", projectId.(string), environmentId.(string)))
	if !exist {
		resource, err := h.services.CompanyService().ServiceResource().GetSingle(
			c.Request.Context(),
			&pb.GetSingleServiceResourceReq{
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
				c.Request.Context(), &obs.FunctionPrimaryKey{
					ProjectId: resource.ResourceEnvironmentId,
					Path:      path,
				},
			)
			if err != nil {
				h.handleResponse(c, status_http.GRPCError, err.Error())
				return
			}

			isPublic = function.GetIsPublic()

			if !permission && !isPublic {
				h.handleResponse(c, status_http.Unauthorized, config.AccessDeniedError)
				return
			}
		case pb.ResourceType_POSTGRESQL:
			function, err := h.services.GoObjectBuilderService().Function().GetSingle(
				c.Request.Context(), &nb.FunctionPrimaryKey{
					ProjectId: resource.ResourceEnvironmentId,
					Path:      path,
				},
			)
			if err != nil {
				h.handleResponse(c, status_http.GRPCError, err.Error())
				return
			}

			isPublic = function.GetIsPublic()

			if !permission && !isPublic {
				h.handleResponse(c, status_http.Unauthorized, config.AccessDeniedError)
				return
			}
		}

		apiKeys, err := h.services.AuthService().ApiKey().GetList(c.Request.Context(), &as.GetListReq{
			EnvironmentId: environmentId.(string),
			ProjectId:     resource.ProjectId,
			Limit:         1,
			Offset:        0,
		})
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}
		if len(apiKeys.Data) < 1 {
			h.handleResponse(c, status.InvalidArgument, "Api key not found")
			return
		}

		apiKey = models.ApiKey{
			AppId:    apiKeys.GetData()[0].GetAppId(),
			IsPublic: isPublic,
		}

		appIdByte, err := json.Marshal(apiKey)
		if err != nil {
			h.handleResponse(c, status.InvalidArgument, err.Error())
			return
		}

		h.cache.Add(fmt.Sprintf("project:%s:env:%s", projectId.(string), environmentId.(string)), appIdByte, config.REDIS_KEY_TIMEOUT)
	} else {
		if err := json.Unmarshal(resourceBody, &apiKey); err != nil {
			h.handleResponse(c, status.InvalidArgument, err.Error())
			return
		}

		if !permission && !apiKey.IsPublic {
			h.handleResponse(c, status_http.Unauthorized, config.AccessDeniedError)
			return
		}
	}

	resp, statusCode, err := util.DoDynamicRequest(
		fmt.Sprintf("http://%s.%s%s", path, h.cfg.KnativeBaseUrl, apiPath),
		headers,
		http.MethodPost,
		invokeFunction,
	)

	if err != nil {
		c.JSON(statusCode, resp)
		return
	}

	c.JSON(statusCode, resp)
}

func (h *Handler) AlterScale(c *gin.Context) {
	m := make(map[string]any)

	err := c.ShouldBindJSON(&m)
	if err != nil {
		h.handleResponse(c, status.InvalidArgument, err.Error())
		return
	}

	token, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		fmt.Println("Error reading token file:", err)
	}

	fmt.Println("GOT TOKEN:", string(token))

	// Construct the URL
	kubeHost := os.Getenv("KUBERNETES_SERVICE_HOST")
	kubePort := os.Getenv("KUBERNETES_SERVICE_PORT")

	fmt.Println("Kube host:", kubeHost)
	fmt.Println("Kube port:", kubePort)

	url := fmt.Sprintf("https://%s:%s/apis/serving.knative.dev/v1/namespaces/knative-fn/services/%s", kubeHost, kubePort, m["name"])

	payload := map[string]any{
		"spec": map[string]any{
			"template": map[string]any{
				"metadata": map[string]any{
					"annotations": map[string]any{
						"autoscaling.knative.dev/minScale": "0",
						"autoscaling.knative.dev/maxScale": cast.ToString(m["max_scale"]),
					},
				},
			},
		},
	}

	payloadByte, err := json.Marshal(payload)
	if err != nil {
		h.handleResponse(c, status.InvalidArgument, err.Error())
		return
	}

	// Prepare the HTTP PATCH request
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewBuffer(payloadByte))
	if err != nil {
		fmt.Println("Error creating request:", err)
		h.handleResponse(c, status.InvalidArgument, err.Error())
		return
	}
	req.Header.Set("Authorization", "Bearer "+string(token))
	req.Header.Set("Content-Type", "application/merge-patch+json")

	// Disable TLS verification (like `-k` in curl)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		h.handleResponse(c, status.InternalServerError, err.Error())
		return
	}
	defer resp.Body.Close()

	// Read and print the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		h.handleResponse(c, status.InternalServerError, err.Error())
		return
	}

	fmt.Printf("Status: %s\n", resp.Status)
	fmt.Println("Response: ", string(body))
}

func (h *Handler) ExecKnative(path string, req models.NewInvokeFunctionRequest) (models.InvokeFunctionResponse, error) {
	url := fmt.Sprintf("http://%s.%s", path, h.cfg.KnativeBaseUrl)
	resp, err := util.DoRequest(url, http.MethodPost, req)
	if err != nil {
		return models.InvokeFunctionResponse{}, err
	}

	return resp, nil
}
