package handlers

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

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

	if len(project.GetFareId()) != 0 {
		var count int32
		switch resource.ResourceType {
		case pb.ResourceType_MONGODB:
			countResp, err := h.services.GetBuilderServiceByType(resource.NodeType).Function().GetCountByType(ctx, &obs.GetCountByTypeRequest{
				ProjectId: resource.ResourceEnvironmentId,
				Type:      []string{config.MICROFE},
			})
			if err != nil {
				h.handleResponse(c, status.GRPCError, err.Error())
				return
			}
			count = countResp.Count
		case pb.ResourceType_POSTGRESQL:
			countResp, err := h.services.GoObjectBuilderService().Function().GetCountByType(ctx, &nb.GetCountByTypeRequest{
				ProjectId: resource.ResourceEnvironmentId,
				Type:      []string{config.MICROFE},
			})
			if err != nil {
				h.handleResponse(c, status.GRPCError, err.Error())
				return
			}
			count = countResp.Count
		}

		limitResp, err := h.services.CompanyService().Billing().CompareFunction(ctx, &pb.CompareFunctionRequest{
			Type:   config.FARE_MICROFRONTEND,
			FareId: project.GetFareId(),
			Count:  count + 1,
		})
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		if !limitResp.HasAccess {
			h.handleResponse(c, status.PaymentRequired, models.PaymentRequiredData{
				Type: "payment_required",
				Code: "microfrontend_limit",
				Unit: "microfrontends",
			})
			return
		}
	}

	var (
		functionPath   = helper.GitlabPath(project.Title) + "_" + strings.ReplaceAll(function.Path, "-", "_")
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

	if err = gitlab.WaitForImport(gitlab.IntegrationData{
		GitlabIntegrationUrl:   h.cfg.GitlabIntegrationURL,
		GitlabIntegrationToken: h.cfg.GitlabTokenMicroFront,
		GitlabProjectId:        respCreateFork.ID,
		GitlabGroupId:          h.cfg.GitlabGroupIdMicroFront,
	}, 2*time.Minute); err != nil {
		h.handleResponse(c, status.InvalidArgument, err.Error())
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

// PublishAiGeneratedMicroFrontend godoc
// @Security ApiKeyAuth
// @ID publish_ai_generated_micro_frontend
// @Router /v2/functions/micro-frontend/publish-ai [POST]
// @Summary Publish AI Generated Micro Frontend
// @Description Creates a new microfrontend, then pushes AI-generated files to the u-gen branch.
// @Tags MicroFrontend
// @Accept json
// @Produce json
// @Param body body models.PublishAiMicroFrontendRequest true "PublishAiMicroFrontendRequest"
// @Success 200 {object} status.Response "Data"
// @Failure 400 {object} status.Response{data=string} "Bad Request"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) PublishAiGeneratedMicroFrontend(c *gin.Context) {
	var req models.PublishAiMicroFrontendRequest

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	if err := c.ShouldBindJSON(&req); err != nil {
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}

	if !util.IsValidUUID(req.ProjectId) {
		h.handleResponse(c, status.InvalidArgument, "project_id is an invalid uuid")
		return
	}

	if !util.IsValidUUID(req.EnvironmentId) {
		h.handleResponse(c, status.InvalidArgument, "environment_id is an invalid uuid")
		return
	}

	if len(req.Files) == 0 {
		h.handleResponse(c, status.InvalidArgument, "files are required")
		return
	}

	if !util.IsValidFunctionName(req.Path) {
		h.handleResponse(c, status.InvalidArgument, "path must contain only lowercase letters, numbers, and hyphens")
		return
	}

	resource, err := h.services.CompanyService().ServiceResource().GetSingle(
		ctx, &pb.GetSingleServiceResourceReq{
			ProjectId:     req.ProjectId,
			EnvironmentId: req.EnvironmentId,
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	environment, err := h.services.CompanyService().Environment().GetById(ctx, &pb.EnvironmentPrimaryKey{Id: req.EnvironmentId})
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
		h.handleResponse(c, status.BadRequest, "project name is required")
		return
	}

	if len(project.GetFareId()) != 0 {
		countResp, err := h.services.GoObjectBuilderService().Function().GetCountByType(ctx, &nb.GetCountByTypeRequest{
			ProjectId: resource.ResourceEnvironmentId,
			Type:      []string{config.MICROFE},
		})
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		limitResp, err := h.services.CompanyService().Billing().CompareFunction(ctx, &pb.CompareFunctionRequest{
			Type:   config.FARE_MICROFRONTEND,
			FareId: project.GetFareId(),
			Count:  countResp.Count + 1,
		})
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		if !limitResp.HasAccess {
			h.handleResponse(c, status.PaymentRequired, models.PaymentRequiredData{
				Type: "payment_required",
				Code: "microfrontend_limit",
				Unit: "microfrontends",
			})
			return
		}
	}

	pathPart := strings.ReplaceAll(req.Path, "-", "_")
	projectName := helper.GitlabPath(project.Title)
	maxProjectLen := 20 - 1 - len(pathPart)
	if maxProjectLen < 1 {
		maxProjectLen = 1
	}
	if len(projectName) > maxProjectLen {
		projectName = strings.TrimRight(projectName[:maxProjectLen], "-")
		if projectName == "" {
			projectName = "p"
		}
	}
	functionPath := projectName + "_" + pathPart

	log.Printf("[PUBLISH-AI] project_id=%s env_id=%s raw_title=%q function_path=%q files=%d",
		req.ProjectId, req.EnvironmentId, project.Title, functionPath, len(req.Files))

	// Step 1: Fork the GitLab React template (creates repo with master branch)
	log.Printf("[PUBLISH-AI] step1: forking template → path=%q", functionPath)
	respCreateFork, err := gitlab.CreateProjectFork(functionPath, gitlab.IntegrationData{
		GitlabIntegrationUrl:   h.cfg.GitlabIntegrationURL,
		GitlabIntegrationToken: h.cfg.GitlabTokenMicroFront,
		GitlabProjectId:        h.cfg.GitlabProjectIdMicroFrontReact,
		GitlabGroupId:          h.cfg.GitlabGroupIdMicroFront,
	})
	if err != nil {
		log.Printf("[PUBLISH-AI] step1 FAILED (fork): %v", err)
		h.handleResponse(c, status.InvalidArgument, err.Error())
		return
	}
	log.Printf("[PUBLISH-AI] step1 ok: repo_id=%d default_branch=%s", respCreateFork.ID, respCreateFork.DefaultBranch)

	gitlabCfg := gitlab.IntegrationData{
		GitlabIntegrationUrl:   h.cfg.GitlabIntegrationURL,
		GitlabIntegrationToken: h.cfg.GitlabTokenMicroFront,
		GitlabProjectId:        respCreateFork.ID,
		GitlabGroupId:          h.cfg.GitlabGroupIdMicroFront,
	}

	// Wait for GitLab to finish importing the forked project.
	// Fork/import is async — pipelines and variables fail with 400 if triggered too early.
	log.Printf("[PUBLISH-AI] waiting for repo_id=%d import to complete...", respCreateFork.ID)
	if err = gitlab.WaitForImport(gitlabCfg, 15*time.Minute); err != nil {
		log.Printf("[PUBLISH-AI] import wait FAILED: %v", err)
		h.handleResponse(c, status.InvalidArgument, fmt.Sprintf("gitlab import not ready: %v", err))
		return
	}
	log.Printf("[PUBLISH-AI] repo_id=%d import complete", respCreateFork.ID)

	// Step 2: Update CI config path
	log.Printf("[PUBLISH-AI] step2: updating CI config path for repo_id=%d", respCreateFork.ID)
	_, err = gitlab.UpdateProject(gitlabCfg, map[string]any{"ci_config_path": ".gitlab-ci.yml"})
	if err != nil {
		log.Printf("[PUBLISH-AI] step2 FAILED (update project): %v", err)
		h.handleResponse(c, status.InvalidArgument, err.Error())
		return
	}

	// Step 3: Create INGRESS_HOST variable and trigger initial pipeline on master
	id := uuid.New().String()
	repoHost := fmt.Sprintf("%s-%s", id, h.cfg.GitlabHostMicroFront)
	host := map[string]any{"key": "INGRESS_HOST", "value": repoHost}

	log.Printf("[PUBLISH-AI] step3a: creating INGRESS_HOST variable for repo_id=%d", respCreateFork.ID)
	_, err = gitlab.CreateProjectVariable(gitlabCfg, host)
	if err != nil {
		log.Printf("[PUBLISH-AI] step3a FAILED (create variable): %v", err)
		h.handleResponse(c, status.InvalidArgument, err.Error())
		return
	}

	log.Printf("[PUBLISH-AI] step3b: triggering pipeline for repo_id=%d", respCreateFork.ID)
	_, err = gitlab.CreatePipeline(gitlabCfg, map[string]any{"variables": []map[string]any{host}})
	if err != nil {
		log.Printf("[PUBLISH-AI] step3b FAILED (create pipeline): %v", err)
		h.handleResponse(c, status.InvalidArgument, err.Error())
		return
	}

	// Step 4: Save the function record with master as the default branch
	log.Printf("[PUBLISH-AI] step4: saving function record name=%q path=%q repo_id=%d", req.Name, functionPath, respCreateFork.ID)
	createFunction := &obs.CreateFunctionRequest{
		Path:          functionPath,
		Name:          req.Name,
		Description:   "Generated by AI",
		ProjectId:     resource.ResourceEnvironmentId,
		EnvironmentId: req.EnvironmentId,
		Type:          config.MICROFE,
		Url:           repoHost,
		RepoId:        fmt.Sprintf("%d", respCreateFork.ID),
		Branch:        respCreateFork.DefaultBranch,
		Resource:      resource.Id,
	}

	var newCreateFunc = &nb.CreateFunctionRequest{}
	if err = helper.MarshalToStruct(createFunction, &newCreateFunc); err != nil {
		log.Printf("[PUBLISH-AI] step4 FAILED (marshal): %v", err)
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}
	newCreateFunc.McpProjectId = req.McpProjectId
	newCreateFunc.McpResourceEnvId = req.McpResourceEnvId

	funcRecord, err := h.services.GoObjectBuilderService().Function().Create(ctx, newCreateFunc)
	if err != nil {
		log.Printf("[PUBLISH-AI] step4 FAILED (db create): %v", err)
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}
	log.Printf("[PUBLISH-AI] step4 ok: func_id=%s", funcRecord.GetId())

	// Step 5: Create u-gen branch from master.
	log.Printf("[PUBLISH-AI] step5: creating %s branch for repo_id=%d", config.UGenBranch, respCreateFork.ID)
	if err = gitlab.CreateBranch(gitlabCfg, config.UGenBranch, config.DefaultBranch); err != nil {
		log.Printf("[PUBLISH-AI] step5 FAILED (create branch): %v", err)
		h.handleResponse(c, status.InternalServerError, fmt.Sprintf("failed to create %s branch: %v", config.UGenBranch, err))
		return
	}

	// Step 6: Convert and commit AI-generated files to u-gen branch.
	nbFiles := make([]*nb.McpProjectFiles, 0, len(req.Files))
	for _, f := range req.Files {
		nbFiles = append(nbFiles, &nb.McpProjectFiles{
			Path:    f.FilePath,
			Content: f.Content,
		})
	}

	log.Printf("[PUBLISH-AI] step6: committing %d file(s) to branch %s for repo_id=%d", len(nbFiles), config.UGenBranch, respCreateFork.ID)
	if _, err = gitlab.CommitFiles(gitlabCfg, config.UGenBranch, nbFiles); err != nil {
		log.Printf("[PUBLISH-AI] step6 FAILED (commit files): %v", err)
		h.handleResponse(c, status.InternalServerError, fmt.Sprintf("failed to commit files to %s: %v", config.UGenBranch, err))
		return
	}

	// Step 7: Mirror the new repo to any connected external providers in the background.
	// This is non-blocking — a failure here does not affect the publish response.
	go func(record *nb.Function, companyProjID, envID string) {
		h.syncAllMicrofrontendMirrors(context.Background(), record, companyProjID, envID)
	}(funcRecord, project.ProjectId, req.EnvironmentId)

	log.Printf("[PUBLISH-AI] done: func_id=%s repo_id=%d path=%q", funcRecord.GetId(), respCreateFork.ID, functionPath)
	h.handleResponse(c, status.OK, funcRecord)
}

// GetMicrofrontendFiles godoc
// @Security ApiKeyAuth
// @ID get_microfrontend_files
// @Router /v2/functions/micro-frontend/files [GET]
// @Summary Get files from a microfrontend's u-gen branch
// @Description Returns all source files from the u-gen branch of the microfrontend's GitLab repo.
// @Tags MicroFrontend
// @Accept json
// @Produce json
// @Param repo_id query int true "GitLab numeric project ID"
// @Success 200 {object} status.Response{data=map[string][]gitlab.RepoFile} "Data"
// @Failure 400 {object} status.Response{data=string} "Bad Request"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) GetMicrofrontendFiles(c *gin.Context) {
	repoID := cast.ToInt(c.Query("repo_id"))
	if repoID == 0 {
		h.handleResponse(c, status.InvalidArgument, "repo_id is required")
		return
	}

	files, err := gitlab.GetRepoCodebase(h.cfg.GitlabIntegrationURL, h.cfg.GitlabTokenMicroFront, repoID)
	if err != nil {
		h.handleResponse(c, status.InternalServerError, err.Error())
		return
	}

	h.handleResponse(c, status.OK, gin.H{"files": files})
}

// PushMicrofrontendChanges godoc
// @Security ApiKeyAuth
// @ID push_microfrontend_changes
// @Router /v2/functions/micro-frontend/push-changes [PUT]
// @Summary Push AI-edited files to the u-gen branch of an existing microfrontend
// @Description Commits the provided files to the u-gen branch of the microfrontend's GitLab repo.
// @Tags MicroFrontend
// @Accept json
// @Produce json
// @Param body body models.PushMicrofrontendChangesRequest true "PushMicrofrontendChangesRequest"
// @Success 200 {object} status.Response "OK"
// @Failure 400 {object} status.Response{data=string} "Bad Request"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) PushMicrofrontendChanges(c *gin.Context) {
	var req models.PushMicrofrontendChangesRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}

	if req.RepoID == 0 {
		h.handleResponse(c, status.InvalidArgument, "repo_id is required")
		return
	}

	if len(req.Files) == 0 {
		h.handleResponse(c, status.InvalidArgument, "files are required")
		return
	}

	gitlabCfg := gitlab.IntegrationData{
		GitlabIntegrationUrl:   h.cfg.GitlabIntegrationURL,
		GitlabIntegrationToken: h.cfg.GitlabTokenMicroFront,
		GitlabProjectId:        req.RepoID,
		GitlabGroupId:          h.cfg.GitlabGroupIdMicroFront,
	}

	nbFiles := make([]*nb.McpProjectFiles, 0, len(req.Files))
	for _, f := range req.Files {
		nbFiles = append(nbFiles, &nb.McpProjectFiles{
			Path:    f.FilePath,
			Content: f.Content,
		})
	}

	log.Printf("[PUSH CHANGES] pushing %d file(s) to repo_id=%d branch=%s", len(nbFiles), req.RepoID, config.UGenBranch)
	for _, f := range nbFiles {
		log.Printf("[PUSH CHANGES]   -> %s (%d bytes)", f.Path, len(f.Content))
	}

	result, err := gitlab.CommitFiles(gitlabCfg, config.UGenBranch, nbFiles, req.CommitMessage)
	if err != nil {
		log.Printf("[PUSH CHANGES] commit failed: %v", err)
		h.handleResponse(c, status.InternalServerError, fmt.Sprintf("failed to push to %s branch: %v", config.UGenBranch, err))
		return
	}

	log.Printf("[PUSH CHANGES] commit successful, gitlab response: %s", string(result))

	if req.FunctionID != "" && req.ResourceEnvironmentID != "" {
		go func(funcID, resourceEnvID string) {
			ctx := context.Background()
			resEnv, resErr := h.services.CompanyService().Resource().GetResourceEnvironment(ctx, &pb.GetResourceEnvironmentReq{
				Id: resourceEnvID,
			})
			if resErr != nil {
				log.Printf("[PUSH-CHANGES→MIRRORS] could not get resource_environment %s: %v", resourceEnvID, resErr)
				return
			}
			funcRecord, funcErr := h.services.GoObjectBuilderService().Function().GetSingle(ctx, &nb.FunctionPrimaryKey{
				Id:        funcID,
				ProjectId: resourceEnvID,
			})
			if funcErr != nil {
				log.Printf("[PUSH-CHANGES→MIRRORS] could not get function %s: %v", funcID, funcErr)
				return
			}
			h.syncAllMicrofrontendMirrors(ctx, funcRecord, resEnv.GetProjectId(), resEnv.GetEnvironmentId())
		}(req.FunctionID, req.ResourceEnvironmentID)
	}

	h.handleResponse(c, status.OK, gin.H{"status": "ok"})
}

// PromoteMicrofrontendToMaster godoc
// @Security ApiKeyAuth
// @ID promote_microfrontend_to_master
// @Router /v2/functions/micro-frontend/promote [POST]
// @Summary Promote u-gen branch to master
// @Description Syncs all files from the u-gen branch to master in a single commit, triggering the CI/CD pipeline. u-gen is treated as the source of truth — files missing from u-gen are deleted from master.
// @Tags MicroFrontend
// @Accept json
// @Produce json
// @Param body body models.PushMicrofrontendChangesRequest true "repo_id required, files ignored"
// @Success 200 {object} status.Response "OK"
// @Failure 400 {object} status.Response{data=string} "Bad Request"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) PromoteMicrofrontendToMaster(c *gin.Context) {
	var (
		req       models.PushMicrofrontendChangesRequest
		ctx       = c.Request.Context()
		projectId any
		envId     any
		ok        bool
	)

	if err := c.ShouldBindJSON(&req); err != nil {
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}

	if req.RepoID == 0 || len(req.McpProjectId) == 0 {
		h.handleResponse(c, status.InvalidArgument, "repo_id and mcp_project_id are required")
		return
	}

	projectId, ok = c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleResponse(c, status.InvalidArgument, "invalid project_id")
		return
	}

	envId, ok = c.Get("environment_id")
	if !ok || !util.IsValidUUID(envId.(string)) {
		h.handleResponse(c, status.InvalidArgument, "invalid environment_id")
		return
	}

	resource, err := h.services.CompanyService().ServiceResource().GetSingle(
		ctx,
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: envId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	mcpProject, err := h.services.GoObjectBuilderService().McpProject().GetMcpProjectFiles(
		ctx, &nb.McpProjectId{
			ResourceEnvId: resource.ResourceEnvironmentId,
			Id:            req.McpProjectId,
			WithoutFiles:  true,
		},
	)
	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	if !mcpProject.IsPublished {

		project, err := h.services.CompanyService().Project().GetById(ctx, &pb.GetProjectByIdRequest{ProjectId: projectId.(string)})
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		publishedProjectsCount, err := h.services.GoObjectBuilderService().McpProject().GetPublishedMcpProjectCount(
			ctx, &nb.GetPublishedMcpProjectCountReq{
				ResourceEnvId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		limitResp, err := h.services.CompanyService().Billing().CompareFunction(
			ctx, &pb.CompareFunctionRequest{
				Type:   config.FARE_PROJECTS,
				FareId: project.GetFareId(),
				Count:  publishedProjectsCount.GetCount(),
			},
		)

		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		if !limitResp.GetHasAccess() {
			h.handleResponse(
				c, status.PaymentRequired, models.PaymentRequiredData{
					Type: "payment_required",
					Code: "project_limit",
					Unit: "projects",
				},
			)
			return
		}
	}

	gitlabCfg := gitlab.IntegrationData{
		GitlabIntegrationUrl:   h.cfg.GitlabIntegrationURL,
		GitlabIntegrationToken: h.cfg.GitlabTokenMicroFront,
		GitlabProjectId:        req.RepoID,
		GitlabGroupId:          h.cfg.GitlabGroupIdMicroFront,
	}

	log.Printf("[PROMOTE] promoting repo_id=%d from %s to %s", req.RepoID, config.UGenBranch, config.DefaultBranch)

	pipelineID, err := gitlab.PromoteUGenToMaster(gitlabCfg, h.cfg.GitlabIntegrationURL, h.cfg.GitlabTokenMicroFront, h.cfg.GitlabProjectIdMicroFrontReact)
	if err != nil {
		log.Printf("[PROMOTE] failed: %v", err)
		h.handleResponse(c, status.InternalServerError, err.Error())
		return
	}

	log.Printf("[PROMOTE] repo_id=%d successfully promoted to %s, pipeline_id=%d", req.RepoID, config.DefaultBranch, pipelineID)

	if _, err := h.services.GoObjectBuilderService().McpProject().UpdateMcpProject(
		ctx, &nb.McpProject{
			ResourceEnvId:       resource.ResourceEnvironmentId,
			Id:                  req.McpProjectId,
			IsPublished:         true,
			MicrofrontendRepoId: strconv.Itoa(req.RepoID),
		},
	); err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		log.Printf("[PROMOTE] could not mark mcp_project %s as published: %v", req.McpProjectId, err)
		return
	}

	mcpProjectResource, err := h.services.CompanyService().ServiceResource().GetSingle(
		ctx,
		&pb.GetSingleServiceResourceReq{
			ProjectId:     mcpProject.GetUcodeProjectId(),
			EnvironmentId: mcpProject.GetEnvironmentId(),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	funcRecord, err := h.services.GoObjectBuilderService().Function().GetSingle(
		ctx, &nb.FunctionPrimaryKey{
			ProjectId: mcpProjectResource.GetResourceEnvironmentId(),
			RepoId:    strconv.Itoa(req.RepoID),
		},
	)
	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		log.Printf("[PROMOTE] could not load function for repo_id=%d, skipping mirror sync: %v", req.RepoID, err)
		return
	}

	if funcRecord != nil {
		go h.syncAllMicrofrontendMirrors(context.Background(), funcRecord, projectId.(string), envId.(string))
	}

	h.handleResponse(c, status.OK, gin.H{"status": "pending", "pipeline_id": pipelineID})
}

// GetPromotePipelineStatus godoc
// @Security ApiKeyAuth
// @ID get_promote_pipeline_status
// @Router /v2/functions/micro-frontend/promote/pipeline-status/{pipeline_id} [GET]
// @Summary Get the status of a promote pipeline
// @Description Polls GitLab for the current status of a pipeline. Frontend should poll every 5s until status is "success" or "failed".
// @Tags MicroFrontend
// @Produce json
// @Param pipeline_id path int true "GitLab pipeline ID"
// @Param repo_id query int true "GitLab project ID"
// @Success 200 {object} status.Response{data=object} "OK — {status: string}"
// @Failure 400 {object} status.Response{data=string} "Bad Request"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) GetPromotePipelineStatus(c *gin.Context) {
	pipelineIDStr := c.Param("pipeline_id")
	repoIDStr := c.Query("repo_id")

	if pipelineIDStr == "" || repoIDStr == "" {
		h.handleResponse(c, status.BadRequest, "pipeline_id path param and repo_id query param are required")
		return
	}

	pipelineID, err := strconv.Atoi(pipelineIDStr)
	if err != nil || pipelineID == 0 {
		h.handleResponse(c, status.BadRequest, "pipeline_id must be a valid non-zero integer")
		return
	}

	repoID, err := strconv.Atoi(repoIDStr)
	if err != nil || repoID == 0 {
		h.handleResponse(c, status.BadRequest, "repo_id must be a valid non-zero integer")
		return
	}

	pipelineStatus, err := gitlab.GetPipelineStatus(h.cfg.GitlabIntegrationURL, h.cfg.GitlabTokenMicroFront, repoID, pipelineID)
	if err != nil {
		h.handleResponse(c, status.InternalServerError, err.Error())
		return
	}

	h.handleResponse(c, status.OK, gin.H{"status": pipelineStatus})
}

// CheckPromoteChanges godoc
// @Security ApiKeyAuth
// @ID check_promote_changes
// @Router /v2/functions/micro-frontend/promote/check-changes [GET]
// @Summary Check if u-gen has changes not yet promoted to master
// @Description Calls the GitLab compare API (master...u-gen) and returns whether there are unpromoted commits.
// @Tags MicroFrontend
// @Produce json
// @Param repo_id query int true "GitLab project ID"
// @Success 200 {object} status.Response{data=object} "OK — {hasChanges: bool}"
// @Failure 400 {object} status.Response{data=string} "Bad Request"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) CheckPromoteChanges(c *gin.Context) {
	repoIDStr := c.Query("repo_id")
	if repoIDStr == "" {
		h.handleResponse(c, status.BadRequest, "repo_id query param is required")
		return
	}

	repoID, err := strconv.Atoi(repoIDStr)
	if err != nil || repoID == 0 {
		h.handleResponse(c, status.BadRequest, "repo_id must be a valid non-zero integer")
		return
	}

	result, err := gitlab.CompareUGenToMaster(h.cfg.GitlabIntegrationURL, h.cfg.GitlabTokenMicroFront, repoID)
	if err != nil {
		h.handleResponse(c, status.InternalServerError, err.Error())
		return
	}

	h.handleResponse(c, status.OK, result)
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
