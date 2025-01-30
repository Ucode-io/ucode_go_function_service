package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"ucode/ucode_go_function_service/api/models"
	status "ucode/ucode_go_function_service/api/status_http"
	cfg "ucode/ucode_go_function_service/config"
	pb "ucode/ucode_go_function_service/genproto/company_service"
	nb "ucode/ucode_go_function_service/genproto/new_object_builder_service"
	obs "ucode/ucode_go_function_service/genproto/object_builder_service"
	"ucode/ucode_go_function_service/pkg/github"
	"ucode/ucode_go_function_service/pkg/gitlab"
	"ucode/ucode_go_function_service/pkg/helper"
	"ucode/ucode_go_function_service/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/spf13/cast"
)

func (h *Handler) CreateWebhook(c *gin.Context) {
	var (
		createWebhookRequest models.CreateWebhook
		createFunction       *obs.CreateFunctionRequest
	)

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	if err := c.ShouldBindJSON(&createWebhookRequest); err != nil {
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}

	if len(createWebhookRequest.FunctionType) == 0 {
		h.handleResponse(c, status.BadRequest, "function type required")
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

	githubResource, err := h.services.CompanyService().Resource().GetSingleProjectResouece(
		ctx, &pb.PrimaryKeyProjectResource{
			Id:            createWebhookRequest.Resource,
			EnvironmentId: environmentId.(string),
			ProjectId:     projectId.(string),
		})
	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	project, err := h.services.CompanyService().Project().GetById(ctx, &pb.GetProjectByIdRequest{ProjectId: projectId.(string)})
	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	var projectName = strings.ReplaceAll(strings.TrimSpace(project.Title), " ", "-")
	projectName = strings.ToLower(projectName)

	if !strings.HasPrefix(createWebhookRequest.RepoName, projectName) {
		h.handleResponse(c, status.BadRequest, "repository name must start with lowercase project name")
		return
	}

	createWebhookRequest.Username = githubResource.GetSettings().GetGithub().GetUsername()
	createWebhookRequest.GithubToken = githubResource.GetSettings().GetGithub().GetToken()

	if createWebhookRequest.RepoName == "" || createWebhookRequest.Username == "" {
		h.handleResponse(c, status.BadRequest, "Username or RepoName is empty")
		return
	}

	exists, err := github.ListWebhooks(github.ListWebhookRequest{
		Username:    createWebhookRequest.Username,
		RepoName:    createWebhookRequest.RepoName,
		GithubToken: createWebhookRequest.GithubToken,
		ProjectUrl:  h.cfg.ProjectUrl,
	})
	if err != nil {
		h.handleResponse(c, status.InternalServerError, err.Error())
		return
	}

	if exists {
		h.handleResponse(c, status.OK, nil)
		return
	}

	createFunction = &obs.CreateFunctionRequest{
		Path:           createWebhookRequest.RepoName,
		Name:           createWebhookRequest.RepoName,
		Description:    createWebhookRequest.RepoName,
		ProjectId:      resource.ResourceEnvironmentId,
		EnvironmentId:  resource.EnvironmentId,
		Type:           createWebhookRequest.FunctionType,
		FrameworkType:  createWebhookRequest.FrameworkType,
		Url:            "",
		SourceUrl:      fmt.Sprintf("https://github.com/%s/%s", createWebhookRequest.Username, createWebhookRequest.RepoName),
		Branch:         createWebhookRequest.Branch,
		PipelineStatus: "running",
		Resource:       createWebhookRequest.Resource,
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		_, err = h.services.GetBuilderServiceByType(resource.NodeType).Function().Create(ctx, createFunction)

		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		var newCreateFunction = &nb.CreateFunctionRequest{}

		if err = helper.MarshalToStruct(createFunction, &newCreateFunction); err != nil {
			h.handleResponse(c, status.InternalServerError, err.Error())
			return
		}

		_, err = h.services.GoObjectBuilderService().Function().Create(
			ctx, newCreateFunction,
		)
		if err != nil {
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}
	}

	err = github.CreateWebhook(github.CreateWebhookRequest{
		Username:      createWebhookRequest.Username,
		RepoName:      createWebhookRequest.RepoName,
		WebhookSecret: h.cfg.WebhookSecret,
		FrameworkType: createWebhookRequest.FrameworkType,
		Branch:        createWebhookRequest.Branch,
		FunctionType:  createWebhookRequest.FunctionType,
		GithubToken:   createWebhookRequest.GithubToken,
		ProjectUrl:    h.cfg.ProjectUrl,
		Name:          createWebhookRequest.Name,
		ResourceId:    createWebhookRequest.Resource,
		ProjectId:     projectId.(string),
		EnvironmentId: environmentId.(string),
	})
	if err != nil {
		h.handleResponse(c, status.InternalServerError, err.Error())
		return
	}

	h.handleResponse(c, status.Created, nil)
}

func (h *Handler) HandleWebhook(c *gin.Context) {
	var payload map[string]any

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.handleResponse(c, status.BadRequest, "Failed to read request body")
		return
	}

	if err = json.Unmarshal(body, &payload); err != nil {
		h.handleResponse(c, status.BadRequest, "Failed to unmarshal JSON inside handle webhook")
		return
	}

	projectId := c.Query("project_id")
	if !util.IsValidUUID(projectId) {
		h.handleResponse(c, status.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId := c.Query("environment_id")
	if !util.IsValidUUID(environmentId) {
		h.handleResponse(c, status.BadRequest, "environment id id is an invalid uuid")
		return
	}

	projectResourceId := c.Query("resource_id")
	if !util.IsValidUUID(projectResourceId) {
		h.handleResponse(c, status.InvalidArgument, "project resource id is an invalid uuid")
		return
	}

	// if !(github.VerifySignature(c.GetHeader("X-Hub-Signature"), body, []byte(h.cfg.WebhookSecret))) {
	// 	h.handleResponse(c, status.BadRequest, "failed to verify signature")
	// 	return
	// }

	projectResource, err := h.services.CompanyService().Resource().GetSingleProjectResouece(
		c.Request.Context(), &pb.PrimaryKeyProjectResource{
			Id:            projectResourceId,
			ProjectId:     projectId,
			EnvironmentId: environmentId,
		},
	)
	if err != nil {
		h.handleResponse(c, status.InternalServerError, err.Error())
		return
	}

	var (
		branchFromWebhook = cast.ToString(payload["ref"])
		repository        = cast.ToStringMap(payload["repository"])
		hook              = cast.ToStringMap(payload["hook"])

		repoId          = cast.ToString(repository["id"])
		repoName        = cast.ToString(repository["name"])
		repoDescription = cast.ToString(repository["description"])
		htmlUrl         = cast.ToString(repository["html_url"])
		branch          = cast.ToString(repository["default_branch"])

		config = cast.ToStringMap(hook["config"])

		functionType = cast.ToString(config["type"])
		resourceType = cast.ToString(config["resource_id"])
		name         = cast.ToString(config["name"])

		token = projectResource.GetSettings().GetGithub().GetToken()
	)

	if branchFromWebhook != "" {
		parts := strings.Split(branchFromWebhook, "/")
		branch = parts[len(parts)-1]
	}

	resource, err := h.services.CompanyService().ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
			ProjectId:     projectId,
			EnvironmentId: environmentId,
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, status.InternalServerError, err.Error())
		return
	}

	builderService := h.services.GetBuilderServiceByType(resource.NodeType)

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		function, functionErr := builderService.Function().GetSingle(
			c.Request.Context(), &obs.FunctionPrimaryKey{
				ProjectId: resource.ResourceEnvironmentId,
				SourceUrl: htmlUrl,
				Branch:    branch,
			},
		)
		if function != nil {
			functionType = function.Type
		}

		switch functionType {
		case cfg.FUNCTION:
			if functionErr != nil {
				function, err = builderService.Function().Create(
					c.Request.Context(), &obs.CreateFunctionRequest{
						Path:           repoName,
						Name:           name,
						Description:    repoDescription,
						ProjectId:      resource.ResourceEnvironmentId,
						EnvironmentId:  resource.EnvironmentId,
						Type:           cfg.FUNCTION,
						SourceUrl:      htmlUrl,
						Branch:         branch,
						PipelineStatus: "running",
						Resource:       resourceType,
					},
				)
				if err != nil {
					h.handleResponse(c, status.GRPCError, err.Error())
					return
				}
			}
			function.PipelineStatus = "running"

			go h.deployFunction(models.DeployFunctionRequest{
				GithubToken:     token,
				RepoId:          repoId,
				ResourceType:    resource.NodeType,
				Function:        function,
				TargetNamespace: "ucode_functions_group",
			})
		case cfg.KNATIVE:
			if functionErr != nil {
				function, err = builderService.Function().Create(
					c.Request.Context(), &obs.CreateFunctionRequest{
						Path:           repoName,
						Name:           name,
						Description:    repoDescription,
						ProjectId:      resource.ResourceEnvironmentId,
						EnvironmentId:  resource.EnvironmentId,
						Type:           cfg.KNATIVE,
						SourceUrl:      htmlUrl,
						Branch:         branch,
						PipelineStatus: "running",
						Resource:       resourceType,
					},
				)
				if err != nil {
					h.handleResponse(c, status.InvalidArgument, err.Error())
					return
				}
			}
			function.PipelineStatus = "running"

			go h.deployFunction(
				models.DeployFunctionRequest{
					GithubToken:     token,
					RepoId:          repoId,
					ResourceType:    resource.NodeType,
					Function:        function,
					TargetNamespace: cfg.KnativeNamespace,
				},
			)
		case cfg.MICROFE:
			if functionErr != nil {
				function, err = builderService.Function().Create(
					c.Request.Context(), &obs.CreateFunctionRequest{
						Path:           repoName,
						Name:           name,
						Description:    repoDescription,
						ProjectId:      resource.ResourceEnvironmentId,
						EnvironmentId:  resource.EnvironmentId,
						Type:           cfg.MICROFE,
						SourceUrl:      htmlUrl,
						Branch:         branch,
						PipelineStatus: "running",
						Resource:       resourceType,
					},
				)
				if err != nil {
					h.handleResponse(c, status.InvalidArgument, err.Error())
					return
				}
			}
			function.PipelineStatus = "running"
			go h.deployFunction(
				models.DeployFunctionRequest{
					GithubToken:     token,
					RepoId:          repoId,
					Function:        function,
					ResourceType:    resource.NodeType,
					TargetNamespace: cfg.MicroFrontNamaspece,
				},
			)
		}
	case pb.ResourceType_POSTGRESQL:
		function, functionErr := h.services.GoObjectBuilderService().Function().GetSingle(
			c.Request.Context(), &nb.FunctionPrimaryKey{
				ProjectId: resource.ResourceEnvironmentId,
				SourceUrl: htmlUrl,
				Branch:    branch,
			},
		)
		if function != nil {
			functionType = function.Type
		}

		switch functionType {
		case cfg.FUNCTION:
			if functionErr != nil {
				function, err = h.services.GoObjectBuilderService().Function().Create(
					c.Request.Context(), &nb.CreateFunctionRequest{
						Path:           repoName,
						Name:           name,
						Description:    repoDescription,
						ProjectId:      resource.ResourceEnvironmentId,
						EnvironmentId:  resource.EnvironmentId,
						Type:           cfg.FUNCTION,
						SourceUrl:      htmlUrl,
						Branch:         branch,
						PipelineStatus: "running",
						Resource:       resourceType,
					},
				)
				if err != nil {
					h.handleResponse(c, status.GRPCError, err.Error())
					return
				}
			}
			function.PipelineStatus = "running"

			go h.deployFunctionGo(models.DeployFunctionRequestGo{
				GithubToken:     token,
				RepoId:          repoId,
				ResourceType:    resource.NodeType,
				Function:        function,
				TargetNamespace: cfg.OpenFassNamespace,
			})
		case cfg.KNATIVE:
			if functionErr != nil {
				function, err = h.services.GoObjectBuilderService().Function().Create(
					c.Request.Context(), &nb.CreateFunctionRequest{
						Path:           repoName,
						Name:           name,
						Description:    repoDescription,
						ProjectId:      resource.ResourceEnvironmentId,
						EnvironmentId:  resource.EnvironmentId,
						Type:           cfg.KNATIVE,
						SourceUrl:      htmlUrl,
						Branch:         branch,
						PipelineStatus: "running",
						Resource:       resourceType,
					},
				)
				if err != nil {
					h.handleResponse(c, status.InvalidArgument, err.Error())
					return
				}
			}
			function.PipelineStatus = "running"

			go h.deployFunctionGo(
				models.DeployFunctionRequestGo{
					GithubToken:     token,
					RepoId:          repoId,
					ResourceType:    resource.NodeType,
					Function:        function,
					TargetNamespace: cfg.KnativeNamespace,
				},
			)
		case cfg.MICROFE:
			if functionErr != nil {
				function, err = h.services.GoObjectBuilderService().Function().Create(
					c.Request.Context(), &nb.CreateFunctionRequest{
						Path:           repoName,
						Name:           name,
						Description:    repoDescription,
						ProjectId:      resource.ResourceEnvironmentId,
						EnvironmentId:  resource.EnvironmentId,
						Type:           cfg.MICROFE,
						SourceUrl:      htmlUrl,
						Branch:         branch,
						PipelineStatus: "running",
						Resource:       resourceType,
					},
				)
				if err != nil {
					h.handleResponse(c, status.InvalidArgument, err.Error())
					return
				}
			}
			function.PipelineStatus = "running"
			go h.deployFunctionGo(
				models.DeployFunctionRequestGo{
					GithubToken:     token,
					RepoId:          repoId,
					ResourceType:    resource.NodeType,
					Function:        function,
					TargetNamespace: cfg.MicroFrontNamaspece,
				},
			)
		}
	}
}

func (h *Handler) deployFunction(req models.DeployFunctionRequest) (github.ImportResponse, error) {
	var gitlabToken string

	switch req.Function.Type {
	case cfg.FUNCTION:
		gitlabToken = h.cfg.GitlabOpenFassToken
	case cfg.KNATIVE:
		gitlabToken = h.cfg.GitlabKnativeToken
	case cfg.MICROFE:
		gitlabToken = h.cfg.GitlabTokenMicroFront
	}

	importResponse, err := github.ImportFromGithub(github.ImportData{
		PersonalAccessToken: req.GithubToken,
		RepoId:              req.RepoId,
		TargetNamespace:     req.TargetNamespace,
		NewName:             req.Function.Path,
		GitlabToken:         gitlabToken,
	})
	if err != nil {
		return github.ImportResponse{}, err
	}

	time.Sleep(10 * time.Second)

	switch req.Function.Type {
	case cfg.KNATIVE:
		err = github.AddCiFileKnative(gitlabToken, importResponse.ID, req.Function.Branch, cfg.PathToCloneKnative)
		if err != nil {
			if err := github.DeleteRepository(gitlabToken, importResponse.ID); err != nil {
				return github.ImportResponse{}, err
			}
		}
	case cfg.FUNCTION:
		err = github.AddCiFileFunction(gitlabToken, importResponse.ID, req.Function.Branch, cfg.PathToCloneFunction)
		if err != nil {
			if err := github.DeleteRepository(gitlabToken, importResponse.ID); err != nil {
				return github.ImportResponse{}, err
			}
		}
	case cfg.MICROFE:
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
			GitlabIntegrationToken: gitlabToken,
			GitlabProjectId:        importResponse.ID,
			GitlabGroupId:          h.cfg.GitlabGroupIdMicroFront,
		}, host)
		if err != nil {
			return github.ImportResponse{}, err
		}

		err = github.AddFilesMicroFront(gitlabToken, importResponse.ID, req.Function.Branch, cfg.PathToCloneMicroFront)
		if err != nil {
			if err := github.DeleteRepository(gitlabToken, importResponse.ID); err != nil {
				return github.ImportResponse{}, err
			}
		}

		_, err = gitlab.CreatePipeline(
			gitlab.IntegrationData{
				GitlabIntegrationUrl:   h.cfg.GitlabIntegrationURL,
				GitlabIntegrationToken: gitlabToken,
				GitlabProjectId:        importResponse.ID,
				GitlabGroupId:          h.cfg.GitlabGroupIdMicroFront,
			}, map[string]any{
				"variables": data,
			},
		)
		if err != nil {
			return github.ImportResponse{}, err
		}
	}

	for {
		time.Sleep(60 * time.Second)
		pipeline, err := github.GetLatestPipeline(gitlabToken, req.Function.Branch, importResponse.ID)
		if err != nil {
			h.services.GetBuilderServiceByType(req.ResourceType).Function().Update(
				context.Background(), &obs.Function{
					Id:             req.Function.Id,
					Path:           req.Function.Path,
					Name:           req.Function.Name,
					Description:    req.Function.Description,
					ProjectId:      req.Function.ProjectId,
					EnvironmentId:  req.Function.EnvironmentId,
					Type:           req.Function.Type,
					Url:            req.Function.Url,
					SourceUrl:      req.Function.SourceUrl,
					Branch:         req.Function.Branch,
					PipelineStatus: "failed",
					RepoId:         fmt.Sprintf("%v", importResponse.ID),
					ErrorMessage:   "Failed to get pipeline status",
					JobName:        "",
					Resource:       req.Function.Resource,
					ProvidedName:   req.Function.ProvidedName,
				},
			)

			if err := github.DeleteRepository(gitlabToken, importResponse.ID); err != nil {
				return github.ImportResponse{}, err
			}
			return github.ImportResponse{}, err
		}

		if pipeline.Status == "failed" {
			logResp, err := github.GetPipelineLog(fmt.Sprintf("%v", importResponse.ID), h.cfg.GitlabIntegrationURL, gitlabToken)
			if err != nil {
				return github.ImportResponse{}, err
			}

			h.services.GetBuilderServiceByType(req.ResourceType).Function().Update(
				context.Background(), &obs.Function{
					Id:               req.Function.Id,
					Path:             req.Function.Path,
					Name:             req.Function.Name,
					Description:      req.Function.Description,
					FunctionFolderId: req.Function.FunctionFolderId,
					ProjectId:        req.Function.ProjectId,
					EnvironmentId:    req.Function.EnvironmentId,
					Type:             req.Function.Type,
					Url:              req.Function.Url,
					FrameworkType:    req.Function.FrameworkType,
					SourceUrl:        req.Function.SourceUrl,
					Branch:           req.Function.Branch,
					PipelineStatus:   pipeline.Status,
					RepoId:           fmt.Sprintf("%v", importResponse.ID),
					ErrorMessage:     logResp.Log,
					JobName:          logResp.JobName,
					Resource:         req.Function.Resource,
					ProvidedName:     req.Function.ProvidedName,
				},
			)

			err = github.DeleteRepository(gitlabToken, importResponse.ID)
			if err != nil {
				return github.ImportResponse{}, err
			}
			return github.ImportResponse{}, err
		}

		h.services.GetBuilderServiceByType(req.ResourceType).Function().Update(
			context.Background(), &obs.Function{
				Id:               req.Function.Id,
				Path:             req.Function.Path,
				Name:             req.Function.Name,
				Description:      req.Function.Description,
				FunctionFolderId: req.Function.FunctionFolderId,
				ProjectId:        req.Function.ProjectId,
				EnvironmentId:    req.Function.EnvironmentId,
				Type:             req.Function.Type,
				Url:              req.Function.Url,
				FrameworkType:    req.Function.FrameworkType,
				SourceUrl:        req.Function.SourceUrl,
				Branch:           req.Function.Branch,
				PipelineStatus:   pipeline.Status,
				RepoId:           fmt.Sprintf("%v", importResponse.ID),
				ErrorMessage:     "",
				JobName:          "",
				Resource:         req.Function.Resource,
				ProvidedName:     req.Function.ProvidedName,
			},
		)
		if pipeline.Status == "success" || pipeline.Status == "skipped" {
			if err := github.DeleteRepository(gitlabToken, importResponse.ID); err != nil {
				return github.ImportResponse{}, err
			}
			return github.ImportResponse{}, nil
		}
	}
}

func (h *Handler) deployFunctionGo(req models.DeployFunctionRequestGo) (github.ImportResponse, error) {
	var gitlabToken string

	switch req.Function.Type {
	case cfg.FUNCTION:
		gitlabToken = h.cfg.GitlabOpenFassToken
	case cfg.KNATIVE:
		gitlabToken = h.cfg.GitlabKnativeToken
	case cfg.MICROFE:
		gitlabToken = h.cfg.GitlabTokenMicroFront
	}

	importResponse, err := github.ImportFromGithub(github.ImportData{
		PersonalAccessToken: req.GithubToken,
		RepoId:              req.RepoId,
		TargetNamespace:     req.TargetNamespace,
		NewName:             req.Function.Path,
		GitlabToken:         gitlabToken,
	})
	if err != nil {
		return github.ImportResponse{}, err
	}

	time.Sleep(10 * time.Second)
	switch req.Function.Type {
	case cfg.KNATIVE:
		err = github.AddCiFileKnative(gitlabToken, importResponse.ID, req.Function.Branch, cfg.PathToCloneKnative)
		if err != nil {
			if err := github.DeleteRepository(gitlabToken, importResponse.ID); err != nil {
				return github.ImportResponse{}, err
			}
		}
	case cfg.FUNCTION:
		err = github.AddCiFileFunction(gitlabToken, importResponse.ID, req.Function.Branch, cfg.PathToCloneFunction)
		if err != nil {
			if err := github.DeleteRepository(gitlabToken, importResponse.ID); err != nil {
				return github.ImportResponse{}, err
			}
		}
	case cfg.MICROFE:
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
			GitlabIntegrationToken: gitlabToken,
			GitlabProjectId:        importResponse.ID,
			GitlabGroupId:          h.cfg.GitlabGroupIdMicroFront,
		}, host)
		if err != nil {
			return github.ImportResponse{}, err
		}

		err = github.AddFilesMicroFront(gitlabToken, importResponse.ID, req.Function.Branch, cfg.PathToCloneMicroFront)
		if err != nil {
			if err := github.DeleteRepository(gitlabToken, importResponse.ID); err != nil {
				return github.ImportResponse{}, err
			}
		}

		_, err = gitlab.CreatePipeline(
			gitlab.IntegrationData{
				GitlabIntegrationUrl:   h.cfg.GitlabIntegrationURL,
				GitlabIntegrationToken: gitlabToken,
				GitlabProjectId:        importResponse.ID,
				GitlabGroupId:          h.cfg.GitlabGroupIdMicroFront,
			}, map[string]any{
				"variables": data,
			},
		)
		if err != nil {
			return github.ImportResponse{}, err
		}
	}

	for {
		time.Sleep(60 * time.Second)
		pipeline, err := github.GetLatestPipeline(gitlabToken, req.Function.Branch, importResponse.ID)
		if err != nil {
			h.services.GoObjectBuilderService().Function().Update(
				context.Background(), &nb.Function{
					Id:             req.Function.Id,
					Path:           req.Function.Path,
					Name:           req.Function.Name,
					Description:    req.Function.Description,
					ProjectId:      req.Function.ProjectId,
					EnvironmentId:  req.Function.EnvironmentId,
					Type:           req.Function.Type,
					Url:            req.Function.Url,
					SourceUrl:      req.Function.SourceUrl,
					Branch:         req.Function.Branch,
					PipelineStatus: "failed",
					RepoId:         fmt.Sprintf("%v", importResponse.ID),
					ErrorMessage:   "Failed to get pipeline status",
					Resource:       req.Function.Resource,
					ProvidedName:   req.Function.ProvidedName,
				},
			)

			if err := github.DeleteRepository(gitlabToken, importResponse.ID); err != nil {
				return github.ImportResponse{}, err
			}
			return github.ImportResponse{}, err
		}

		if pipeline.Status == "failed" {
			logResp, err := github.GetPipelineLog(fmt.Sprintf("%v", importResponse.ID), h.cfg.GitlabIntegrationURL, gitlabToken)
			if err != nil {
				return github.ImportResponse{}, err
			}

			h.services.GoObjectBuilderService().Function().Update(
				context.Background(), &nb.Function{
					Id:               req.Function.Id,
					Path:             req.Function.Path,
					Name:             req.Function.Name,
					Description:      req.Function.Description,
					FunctionFolderId: req.Function.FunctionFolderId,
					ProjectId:        req.Function.ProjectId,
					EnvironmentId:    req.Function.EnvironmentId,
					Type:             req.Function.Type,
					Url:              req.Function.Url,
					FrameworkType:    req.Function.FrameworkType,
					SourceUrl:        req.Function.SourceUrl,
					Branch:           req.Function.Branch,
					PipelineStatus:   pipeline.Status,
					RepoId:           fmt.Sprintf("%v", importResponse.ID),
					ErrorMessage:     logResp.Log,
					JobName:          logResp.JobName,
					Resource:         req.Function.Resource,
					ProvidedName:     req.Function.ProvidedName,
				},
			)

			err = github.DeleteRepository(gitlabToken, importResponse.ID)
			if err != nil {
				return github.ImportResponse{}, err
			}
			return github.ImportResponse{}, err
		}

		h.services.GoObjectBuilderService().Function().Update(
			context.Background(), &nb.Function{
				Id:               req.Function.Id,
				Path:             req.Function.Path,
				Name:             req.Function.Name,
				Description:      req.Function.Description,
				FunctionFolderId: req.Function.FunctionFolderId,
				ProjectId:        req.Function.ProjectId,
				EnvironmentId:    req.Function.EnvironmentId,
				Type:             req.Function.Type,
				Url:              req.Function.Url,
				FrameworkType:    req.Function.FrameworkType,
				SourceUrl:        req.Function.SourceUrl,
				Branch:           req.Function.Branch,
				PipelineStatus:   pipeline.Status,
				RepoId:           fmt.Sprintf("%v", importResponse.ID),
				ErrorMessage:     "",
				JobName:          "",
				Resource:         req.Function.Resource,
				ProvidedName:     req.Function.ProvidedName,
			},
		)
		if pipeline.Status == "success" || pipeline.Status == "skipped" {
			if err := github.DeleteRepository(gitlabToken, importResponse.ID); err != nil {
				return github.ImportResponse{}, err
			}
			return github.ImportResponse{}, nil
		}
	}
}
