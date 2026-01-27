package handlers

import (
	"context"
	"fmt"
	"strings"
	"ucode/ucode_go_function_service/api/models"
	status "ucode/ucode_go_function_service/api/status_http"
	"ucode/ucode_go_function_service/config"
	pb "ucode/ucode_go_function_service/genproto/company_service"
	nb "ucode/ucode_go_function_service/genproto/new_object_builder_service"
	obs "ucode/ucode_go_function_service/genproto/object_builder_service"
	"ucode/ucode_go_function_service/pkg/gitlab"
	"ucode/ucode_go_function_service/pkg/helper"
	"ucode/ucode_go_function_service/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/spf13/cast"
)

func (h *Handler) PublishMcpProjectFront(c *gin.Context) {
	var (
		mcpProjectId = c.Param("mcp_project_id")

		projectId     any
		environmentId any
		userId        any

		repoId int

		ok bool
	)

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	projectId, ok = c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleResponse(c, status.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok = c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, status.InvalidArgument, "error getting environment id | not valid")
		return
	}

	userId, _ = c.Get("user_id")

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

	if resource.ResourceType != pb.ResourceType_POSTGRESQL {
		h.handleResponse(c, status.InvalidArgument, "resource type must be POSTGRESQL")
		return
	}

	mcpProject, err := h.services.GoObjectBuilderService().McpProject().GetMcpProjectFiles(
		ctx, &nb.McpProjectId{
			ResourceEnvId: resource.ResourceEnvironmentId,
			Id:            mcpProjectId,
		},
	)
	if err != nil {
		h.handleResponse(c, status.GRPCError, fmt.Errorf("error getting mcp_project: %w", err))
		return
	}

	// =========================== CREATING REPO ==========================================================
	if !util.IsValidUUID(mcpProject.GetFunctionId()) {
		project, err := h.services.CompanyService().Project().GetById(ctx, &pb.GetProjectByIdRequest{ProjectId: projectId.(string)})
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}

		if len(project.GetTitle()) == 0 {
			h.handleResponse(c, status.BadRequest, "error project name is required")
			return
		}

		var (
			projectName    = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(project.Title), " ", "-"))
			functionPath   = projectName + "_" + strings.ReplaceAll(mcpProject.GetTitle(), "-", "_")
			respCreateFork gitlab.ForkResponse
		)

		respCreateFork, err = gitlab.CreateProjectFork(
			functionPath, gitlab.IntegrationData{
				GitlabIntegrationUrl:   h.cfg.GitlabIntegrationURL,
				GitlabIntegrationToken: h.cfg.GitlabTokenMicroFront,
				GitlabProjectId:        h.cfg.GitlabProjectIdMicroFrontReact,
				GitlabGroupId:          h.cfg.GitlabGroupIdMicroFront,
			},
		)
		if err != nil {
			h.handleResponse(c, status.InvalidArgument, err.Error())
			return
		}

		_, err = gitlab.UpdateProject(
			gitlab.IntegrationData{
				GitlabIntegrationUrl:   h.cfg.GitlabIntegrationURL,
				GitlabIntegrationToken: h.cfg.GitlabTokenMicroFront,
				GitlabProjectId:        respCreateFork.ID,
				GitlabGroupId:          h.cfg.GitlabGroupIdMicroFront,
			},
			map[string]any{
				"ci_config_path": ".gitlab-ci.yml",
			},
		)
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

		_, err = gitlab.CreateProjectVariable(
			gitlab.IntegrationData{
				GitlabIntegrationUrl:   h.cfg.GitlabIntegrationURL,
				GitlabIntegrationToken: h.cfg.GitlabTokenMicroFront,
				GitlabProjectId:        respCreateFork.ID,
				GitlabGroupId:          h.cfg.GitlabGroupIdMicroFront,
			},
			host,
		)
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
			},
			map[string]any{
				"variables": data,
			},
		)
		if err != nil {
			h.handleResponse(c, status.InvalidArgument, err.Error())
			return
		}

		var (
			createFunction = &obs.CreateFunctionRequest{
				Path:          functionPath,
				Name:          mcpProject.GetTitle(),
				Description:   mcpProject.GetDescription(),
				ProjectId:     resource.ResourceEnvironmentId,
				EnvironmentId: environmentId.(string),
				Type:          config.MICROFE,
				Url:           repoHost,
				RepoId:        fmt.Sprintf("%d", respCreateFork.ID),
				Branch:        respCreateFork.DefaultBranch,
				Resource:      resource.Id,
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

		var newCreateFunc = &nb.CreateFunctionRequest{}

		if err = helper.MarshalToStruct(createFunction, &newCreateFunc); err != nil {
			h.handleResponse(c, status.BadRequest, err.Error())
			return
		}

		funcCreateRsp, err := h.services.GoObjectBuilderService().Function().Create(ctx, newCreateFunc)
		if err != nil {
			logReq.Response = err.Error()
			h.handleResponse(c, status.GRPCError, err.Error())
			go h.versionHistoryGo(c, logReq)
			return
		}

		logReq.Response = funcCreateRsp
		go h.versionHistoryGo(c, logReq)

		mcpProject.FunctionId = funcCreateRsp.Id
		_, err = h.services.GoObjectBuilderService().McpProject().UpdateMcpProject(ctx, mcpProject)
		if err != nil {
			h.handleResponse(c, status.GRPCError, fmt.Errorf("error updating mcp_project: %w", err))
			return
		}

		repoId = respCreateFork.ID
	}

	if repoId == 0 {
		repoId = cast.ToInt(mcpProject.FunctionData.RepoId)
		if repoId == 0 {
			h.handleResponse(c, status.NotFound, "repository not found")
			return
		}
	}

	resp, err := gitlab.CommitFiles(
		gitlab.IntegrationData{
			GitlabIntegrationUrl:   h.cfg.GitlabIntegrationURL,
			GitlabIntegrationToken: h.cfg.GitlabTokenMicroFront,
			GitlabProjectId:        repoId,
			GitlabGroupId:          h.cfg.GitlabGroupIdMicroFront,
		},
		mcpProject.ProjectFiles,
	)

	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, status.OK, map[string]any{"message": "success"})
}
