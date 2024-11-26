package handlers

import (
	"context"
	"errors"
	"strings"
	"ucode/ucode_go_function_service/api/models"
	"ucode/ucode_go_function_service/api/status_http"
	"ucode/ucode_go_function_service/config"
	pb "ucode/ucode_go_function_service/genproto/company_service"
	nb "ucode/ucode_go_function_service/genproto/new_object_builder_service"
	"ucode/ucode_go_function_service/genproto/object_builder_service"

	"ucode/ucode_go_function_service/pkg/helper"

	"ucode/ucode_go_function_service/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/xtgo/uuid"

	"github.com/spf13/cast"
)

// CreateNewFunction godoc
// @Security ApiKeyAuth
// @ID create_new_function
// @Router /v2/function [POST]
// @Summary Create New Function
// @Description Create New Function
// @Tags Function
// @Accept json
// @Produce json
// @Param Function body models.CreateFunctionRequest true "CreateFunctionRequestBody"
// @Success 201 {object} status_http.Response{data=fc.Function} "Function data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *Handler) CreateFunction(c *gin.Context) {
	var function models.CreateFunctionRequest

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	if err := c.ShouldBindJSON(&function); err != nil {
		h.handleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, status_http.BadRequest, "error getting environment id | not valid")
		return
	}

	userId, _ := c.Get("user_id")

	resource, err := h.services.CompanyService().ServiceResource().GetSingle(
		ctx,
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_FUNCTION_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	environment, err := h.services.CompanyService().Environment().GetById(ctx, &pb.EnvironmentPrimaryKey{
		Id: environmentId.(string),
	})
	if err != nil {
		h.handleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	project, err := h.services.CompanyService().Project().GetById(ctx, &pb.GetProjectByIdRequest{
		ProjectId: environment.GetProjectId(),
	})
	if err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	if project.GetTitle() == "" {
		h.handleResponse(c, status_http.BadRequest, "error project name is required")
		return
	}

	var projectName = strings.ReplaceAll(strings.TrimSpace(project.Title), " ", "-")
	projectName = strings.ToLower(projectName)

	var (
		functionPath = projectName + "-" + function.Path
		uuid         = uuid.NewRandom()
		url          = "https://" + uuid.String() + ".u-code.io"

		createFunction = &object_builder_service.CreateFunctionRequest{
			Path:             functionPath,
			Name:             function.Name,
			Description:      function.Description,
			ProjectId:        resource.ResourceEnvironmentId,
			EnvironmentId:    environmentId.(string),
			FunctionFolderId: function.FunctionFolderId,
			Url:              url,
			Type:             config.FUNCTION,
		}

		logReq = &models.CreateVersionHistoryRequest{
			Services:     h.services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   "CREATE",
			UsedEnvironments: map[string]bool{
				cast.ToString(environmentId): true,
			},
			UserInfo:  cast.ToString(userId),
			Request:   createFunction,
			TableSlug: "FUNCTION",
		}
	)

	// TODO CREATE FUNCTON ON GITHUB, GITLAB, BITBUCKET

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		response, err := h.services.GetBuilderServiceByType(resource.NodeType).Function().Create(
			ctx, createFunction,
		)

		if err != nil {
			logReq.Response = err.Error()
			h.handleResponse(c, status_http.GRPCError, err.Error())
		} else {
			logReq.Response = response
			h.handleResponse(c, status_http.Created, response)
		}
		go h.versionHistory(logReq)
	case pb.ResourceType_POSTGRESQL:
		newCreateFunction := &nb.CreateFunctionRequest{}

		if err = helper.MarshalToStruct(createFunction, &newCreateFunction); err != nil {
			return
		}

		response, err := h.services.GoObjectBuilderService().Function().Create(
			ctx,
			newCreateFunction,
		)

		if err != nil {
			logReq.Response = err.Error()
			h.handleResponse(c, status_http.GRPCError, err.Error())
		} else {
			logReq.Response = response
			h.handleResponse(c, status_http.Created, response)
		}
		go h.versionHistoryGo(c, logReq)
	}
}

// GetNewFunctionByID godoc
// @Security ApiKeyAuth
// @ID get_new_function_by_id
// @Router /v2/function/{function_id} [GET]
// @Summary Get Function by id
// @Description Get Function by id
// @Tags Function
// @Accept json
// @Produce json
// @Param function_id path string true "function_id"
// @Success 200 {object} status_http.Response{data=fc.Function} "FunctionBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *Handler) GetFunctionByID(c *gin.Context) {
	var functionID = c.Param("function_id")
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	if !util.IsValidUUID(functionID) {
		h.handleResponse(c, status_http.InvalidArgument, "function id is an invalid uuid")
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		err := errors.New("error getting environment id | not valid")
		h.handleResponse(c, status_http.BadRequest, err)
		return
	}

	resource, err := h.services.CompanyService().ServiceResource().GetSingle(
		ctx,
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_FUNCTION_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	var function = &object_builder_service.Function{}
	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		function, err = h.services.GetBuilderServiceByType(resource.NodeType).Function().GetSingle(
			ctx,
			&object_builder_service.FunctionPrimaryKey{
				Id:        functionID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.handleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		if function.Url == "" {
			// err = gitlab.CloneForkToPath(function.GetSshUrl(), h.baseConf)
			// if err != nil {
			// 	h.handleResponse(c, status_http.InvalidArgument, err.Error())
			// 	return
			// }
			// uuid, _ := uuid.NewRandom()
			// password, err := code_server.CreateCodeServer(function.Path, h.baseConf, uuid.String())
			// if err != nil {
			// 	h.handleResponse(c, status_http.InvalidArgument, err.Error())
			// 	return
			// }
			// function.Url = "https://" + uuid.String() + ".u-code.io"
			// function.Password = password
		}

		function.ProjectId = resource.ResourceEnvironmentId
		_, err = h.services.GetBuilderServiceByType(resource.NodeType).Function().Update(ctx, function)
		if err != nil {
			h.handleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		resp, err := h.services.GoObjectBuilderService().Function().GetSingle(
			ctx,
			&nb.FunctionPrimaryKey{
				Id:        functionID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.handleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		err = helper.MarshalToStruct(resp, &function)
		if err != nil {
			h.handleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	}

	h.handleResponse(c, status_http.OK, function)
}

// GetAllNewFunctions godoc
// @Security ApiKeyAuth
// @ID get_all_new_functions
// @Router /v2/function [GET]
// @Summary Get all functions
// @Description Get all functions
// @Tags Function
// @Accept json
// @Produce json
// @Param limit query number false "limit"
// @Param offset query number false "offset"
// @Param search query string false "search"
// @Success 200 {object} status_http.Response{data=string} "FunctionBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *Handler) GetAllFunction(c *gin.Context) {
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	limit, err := h.getLimitParam(c)
	if err != nil {
		h.handleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	offset, err := h.getOffsetParam(c)
	if err != nil {
		h.handleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, status_http.BadRequest, "error getting environment id | not valid")
		return
	}

	resource, err := h.services.CompanyService().ServiceResource().GetSingle(
		ctx,
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_FUNCTION_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	environment, err := h.services.CompanyService().Environment().GetById(ctx, &pb.EnvironmentPrimaryKey{Id: environmentId.(string)})
	if err != nil {
		h.handleResponse(c, status_http.GRPCError, "error getting resource environment id")
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := h.services.GetBuilderServiceByType(resource.NodeType).Function().GetList(
			ctx,
			&object_builder_service.GetAllFunctionsRequest{
				Search:        c.DefaultQuery("search", ""),
				Limit:         int32(limit),
				Offset:        int32(offset),
				ProjectId:     resource.ResourceEnvironmentId,
				EnvironmentId: environment.GetId(),
				Type:          config.FUNCTION,
			},
		)
		if err != nil {
			h.handleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.handleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := h.services.GoObjectBuilderService().Function().GetList(
			ctx,
			&nb.GetAllFunctionsRequest{
				Search:    c.DefaultQuery("search", ""),
				Limit:     int32(limit),
				Offset:    int32(offset),
				ProjectId: resource.ResourceEnvironmentId,
				Type:      config.FUNCTION,
			},
		)
		if err != nil {
			h.handleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.handleResponse(c, status_http.OK, resp)
	}
}

// UpdateNewFunction godoc
// @Security ApiKeyAuth
// @ID update_new_function
// @Router /v2/function [PUT]
// @Summary Update new function
// @Description Update new function
// @Tags Function
// @Accept json
// @Produce json
// @Param Function body models.Function  true "UpdateFunctionRequestBody"
// @Success 200 {object} status_http.Response{data=models.Function} "Function data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *Handler) UpdateNewFunction(c *gin.Context) {
	var (
		function models.Function
		resp     = &empty.Empty{}
	)

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	if err := c.ShouldBindJSON(&function); err != nil {
		h.handleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, status_http.BadRequest, "error getting environment id | not valid")
		return
	}

	userId, _ := c.Get("user_id")

	resource, err := h.services.CompanyService().ServiceResource().GetSingle(
		ctx,
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_FUNCTION_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	environment, err := h.services.CompanyService().Environment().GetById(
		ctx, &pb.EnvironmentPrimaryKey{Id: environmentId.(string)},
	)

	var (
		updateFunction = &object_builder_service.Function{
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
			ActionType:   "UPDATE",
			UsedEnvironments: map[string]bool{
				cast.ToString(environmentId): true,
			},
			UserInfo:  cast.ToString(userId),
			Request:   &updateFunction,
			TableSlug: "FUNCTION",
		}
	)

	defer func() {
		if err != nil {
			logReq.Response = err.Error()
			h.handleResponse(c, status_http.GRPCError, err.Error())
		} else {
			h.handleResponse(c, status_http.OK, resp)
		}
		go h.versionHistory(logReq)
	}()

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = h.services.GetBuilderServiceByType(resource.NodeType).Function().Update(
			ctx, updateFunction,
		)
		if err != nil {
			return
		}
	case pb.ResourceType_POSTGRESQL:
		updateFunction := &nb.Function{}

		if err = helper.MarshalToStruct(updateFunction, &updateFunction); err != nil {
			return
		}

		resp, err = h.services.GoObjectBuilderService().Function().Update(
			ctx, updateFunction,
		)
	}

}

// DeleteNewFunction godoc
// @Security ApiKeyAuth
// @ID delete_new_function
// @Router /v2/function/{function_id} [DELETE]
// @Summary Delete New Function
// @Description Delete New Function
// @Tags Function
// @Accept json
// @Produce json
// @Param function_id path string true "function_id"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *Handler) DeleteFunction(c *gin.Context) {
	var functionID = c.Param("function_id")

	if !util.IsValidUUID(functionID) {
		h.handleResponse(c, status_http.InvalidArgument, "function id is an invalid uuid")
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, status_http.BadRequest, "error getting environment id | not valid")
		return
	}

	userId, _ := c.Get("user_id")

	resource, err := h.services.CompanyService().ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_FUNCTION_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	var resp *object_builder_service.Function
	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = h.services.GetBuilderServiceByType(resource.NodeType).Function().GetSingle(
			c.Request.Context(),
			&object_builder_service.FunctionPrimaryKey{
				Id:        functionID,
				ProjectId: environmentId.(string),
			},
		)

		if err != nil {
			h.handleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		goResp, err := h.services.GoObjectBuilderService().Function().GetSingle(
			c.Request.Context(),
			&nb.FunctionPrimaryKey{
				Id:        functionID,
				ProjectId: environmentId.(string),
			},
		)
		if err != nil {
			h.handleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		err = helper.MarshalToStruct(goResp, &resp)
		if err != nil {
			h.handleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	}

	// Should Be Un commented
	// // delete code server
	// err = code_server.DeleteCodeServerByPath(resp.Path, h.baseConf)
	// if err != nil {
	// 	h.handleResponse(c, status_http.GRPCError, err.Error())
	// 	return
	// }

	// // delete cloned repo
	// err = gitlab.DeletedClonedRepoByPath(resp.Path, h.baseConf)
	// if err != nil {
	// 	h.handleResponse(c, status_http.GRPCError, err.Error())
	// 	return
	// }

	// // delete repo by path from gitlab
	// _, err = gitlab.DeleteForkedProject(resp.Path, h.baseConf)
	// if err != nil {
	// 	h.handleResponse(c, status_http.GRPCError, err.Error())
	// 	return
	// }

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     h.services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   "DELETE",
			UserInfo:     cast.ToString(userId),
			TableSlug:    "FUNCTION",
		}
	)

	defer func() {
		if err != nil {
			logReq.Response = err.Error()
			h.handleResponse(c, status_http.GRPCError, err.Error())
		} else {
			h.handleResponse(c, status_http.NoContent, resp)
		}
		go h.versionHistory(logReq)
	}()

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		_, err = h.services.GetBuilderServiceByType(resource.NodeType).Function().Delete(
			context.Background(),
			&object_builder_service.FunctionPrimaryKey{
				Id:        functionID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			return
		}
	case pb.ResourceType_POSTGRESQL:
		_, err = h.services.GoObjectBuilderService().Function().Delete(
			context.Background(),
			&nb.FunctionPrimaryKey{
				Id:        functionID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
	}
}
