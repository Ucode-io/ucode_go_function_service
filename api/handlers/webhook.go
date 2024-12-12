package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"ucode/ucode_go_function_service/api/models"
	"ucode/ucode_go_function_service/api/status_http"
	pb "ucode/ucode_go_function_service/genproto/company_service"
	obs "ucode/ucode_go_function_service/genproto/object_builder_service"
	"ucode/ucode_go_function_service/pkg/github"
	"ucode/ucode_go_function_service/pkg/logger"
	"ucode/ucode_go_function_service/pkg/util"
	"ucode/ucode_go_function_service/services"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

func (h *Handler) CreateWebhook(c *gin.Context) {
	var createWebhookRequest models.CreateWebhook

	if err := c.ShouldBindJSON(&createWebhookRequest); err != nil {
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

	githubResource, err := h.services.CompanyService().Resource().GetSingleProjectResouece(
		c.Request.Context(), &pb.PrimaryKeyProjectResource{
			Id:            createWebhookRequest.Resource,
			EnvironmentId: environmentId.(string),
			ProjectId:     projectId.(string),
		})
	if err != nil {
		h.handleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	createWebhookRequest.Username = githubResource.GetSettings().Github.Username
	createWebhookRequest.GithubToken = githubResource.GetSettings().Github.Token

	if createWebhookRequest.RepoName == "" || createWebhookRequest.Username == "" {
		h.handleResponse(c, status_http.BadRequest, "Username or RepoName is empty")
		return
	}

	exists, err := github.ListWebhooks(github.ListWebhookRequest{
		Username:    createWebhookRequest.Username,
		RepoName:    createWebhookRequest.RepoName,
		GithubToken: createWebhookRequest.GithubToken,
		ProjectUrl:  h.cfg.ProjectUrl,
	})
	if err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	if exists {
		h.handleResponse(c, status_http.OK, nil)
		return
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
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	h.handleResponse(c, status_http.Created, nil)
}

func (h *Handler) HandleWebhook(c *gin.Context) {
	var payload map[string]interface{}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.handleResponse(c, status_http.BadRequest, "Failed to read request body")
		return
	}

	err = json.Unmarshal(body, &payload)
	if err != nil {
		h.handleResponse(c, status_http.BadRequest, "Failed to unmarshal JSON inside handle webhook")
		return
	}

	projectId := c.Query("project_id")
	if !util.IsValidUUID(projectId) {
		h.handleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId := c.Query("environment_id")
	if !util.IsValidUUID(environmentId) {
		h.handleResponse(c, status_http.BadRequest, "environment id id is an invalid uuid")
		return
	}

	projectResourceId := c.Query("resource_id")
	if !util.IsValidUUID(projectResourceId) {
		h.handleResponse(c, status_http.InvalidArgument, "project resource id is an invalid uuid")
		return
	}

	h.log.Info("From Webhook", logger.Any("data", payload))

	if !(github.VerifySignature(c.GetHeader("X-Hub-Signature"), body, []byte(h.cfg.WebhookSecret))) {
		h.handleResponse(c, status_http.BadRequest, "Failed to verify signature")
		return
	}

	projectResource, err := h.services.CompanyService().Resource().GetSingleProjectResouece(
		c.Request.Context(),
		&pb.PrimaryKeyProjectResource{Id: projectResourceId},
	)
	if err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
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

		config = cast.ToStringMap(hook["config"])

		// frameworkType = cast.ToString(config["framework_type"])
		functionType = cast.ToString(config["type"])
		branch       = cast.ToString(config["branch"])
		resourceType = cast.ToString(config["resource_id"])
		name         = cast.ToString(config["name"])

		token = projectResource.GetSettings().GetGithub().GetToken()
	)

	if branchFromWebhook != "" {
		parts := strings.Split(branchFromWebhook, "/")
		branch = parts[len(parts)-1]
	}

	resource, err := h.services.CompanyService().ServiceResource().GetSingle(c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId,
			EnvironmentId: environmentId,
			ServiceType:   pb.ServiceType_FUNCTION_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
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
		case "FUNCTION":
		case "KNATIVE":
			if functionErr != nil {
				function, err = builderService.Function().Create(c.Request.Context(),
					&obs.CreateFunctionRequest{
						Path:           repoName,
						Name:           name,
						Description:    repoDescription,
						ProjectId:      resource.ResourceEnvironmentId,
						EnvironmentId:  resource.EnvironmentId,
						Type:           "KNATIVE",
						SourceUrl:      htmlUrl,
						Branch:         branch,
						PipelineStatus: "running",
						Resource:       resourceType,
					},
				)
				if err != nil {
					h.handleResponse(c, status_http.InvalidArgument, err.Error())
					return
				}
			}
			function.PipelineStatus = "running"
			go h.deployOpenfaas(h.services, token, repoId, resource.NodeType, function)
		}
	case pb.ResourceType_POSTGRESQL:

	}
}

func (h *Handler) deployOpenfaas(services services.ServiceManagerI, githubToken, repoId, resourceType string, function *obs.Function) (github.ImportResponse, error) {
	importResponse, err := github.ImportFromGithub(github.ImportData{
		PersonalAccessToken: githubToken,
		RepoId:              repoId,
		TargetNamespace:     "ucode/knative",
		NewName:             function.Path,
		GitlabToken:         h.cfg.GitlabIntegrationToken,
	})
	if err != nil {
		return github.ImportResponse{}, err
	}

	time.Sleep(10 * time.Second)
	err = github.AddCiFile(h.cfg.GitlabIntegrationToken, h.cfg.PathToClone, importResponse.ID, function.Branch, "openfaas_integration")
	if err != nil {
		err := github.DeleteRepository(h.cfg.GitlabIntegrationToken, importResponse.ID)
		if err != nil {
			return github.ImportResponse{}, err
		}
	}

	for {
		// time.Sleep(60 * time.Second)
		pipeline, err := github.GetLatestPipeline(h.cfg.GitlabIntegrationToken, function.Branch, importResponse.ID)
		if err != nil {
			services.GetBuilderServiceByType(resourceType).Function().Update(context.Background(),
				&obs.Function{
					Id:             function.Id,
					Path:           function.Path,
					Name:           function.Name,
					Description:    function.Description,
					ProjectId:      function.ProjectId,
					EnvironmentId:  function.EnvironmentId,
					Type:           function.Type,
					Url:            function.Url,
					SourceUrl:      function.SourceUrl,
					Branch:         function.Branch,
					PipelineStatus: "failed",
					RepoId:         fmt.Sprintf("%v", importResponse.ID),
					ErrorMessage:   "Failed to get pipeline status",
					JobName:        "",
					Resource:       function.Resource,
					ProvidedName:   function.ProvidedName,
				},
			)
			err := github.DeleteRepository(h.cfg.GitlabIntegrationToken, importResponse.ID)
			if err != nil {
				return github.ImportResponse{}, err
			}
			return github.ImportResponse{}, err
		}

		if pipeline.Status == "failed" {
			logResp, err := github.GetPipelineLog(fmt.Sprintf("%v", importResponse.ID), h.cfg.GitlabIntegrationURL, h.cfg.GitlabIntegrationToken)
			if err != nil {
				return github.ImportResponse{}, err
			}

			services.GetBuilderServiceByType(resourceType).Function().Update(context.Background(),
				&obs.Function{
					Id:               function.Id,
					Path:             function.Path,
					Name:             function.Name,
					Description:      function.Description,
					FunctionFolderId: function.FunctionFolderId,
					ProjectId:        function.ProjectId,
					EnvironmentId:    function.EnvironmentId,
					Type:             function.Type,
					Url:              function.Url,
					FrameworkType:    function.FrameworkType,
					SourceUrl:        function.SourceUrl,
					Branch:           function.Branch,
					PipelineStatus:   pipeline.Status,
					RepoId:           fmt.Sprintf("%v", importResponse.ID),
					ErrorMessage:     logResp.Log,
					JobName:          logResp.JobName,
					Resource:         function.Resource,
					ProvidedName:     function.ProvidedName,
				},
			)

			err = github.DeleteRepository(h.cfg.GitlabIntegrationToken, importResponse.ID)
			if err != nil {
				return github.ImportResponse{}, err
			}
			return github.ImportResponse{}, err
		}

		services.GetBuilderServiceByType(resourceType).Function().Update(context.Background(),
			&obs.Function{
				Id:               function.Id,
				Path:             function.Path,
				Name:             function.Name,
				Description:      function.Description,
				FunctionFolderId: function.FunctionFolderId,
				ProjectId:        function.ProjectId,
				EnvironmentId:    function.EnvironmentId,
				Type:             function.Type,
				Url:              function.Url,
				FrameworkType:    function.FrameworkType,
				SourceUrl:        function.SourceUrl,
				Branch:           function.Branch,
				PipelineStatus:   pipeline.Status,
				RepoId:           fmt.Sprintf("%v", importResponse.ID),
				ErrorMessage:     "",
				JobName:          "",
				Resource:         function.Resource,
				ProvidedName:     function.ProvidedName,
			},
		)

		if pipeline.Status == "success" || pipeline.Status == "skipped" {
			err := github.DeleteRepository(h.cfg.GitlabIntegrationToken, importResponse.ID)
			if err != nil {
				return github.ImportResponse{}, err
			}
			return github.ImportResponse{}, nil
		}
	}
}

// func (h *Handler) deployMicrofrontend(githubToken, repoId string, function *obs.Function) (github.ImportResponse, error) {
// 	importResponse, err := github.ImportFromGithub(github.ImportData{
// 		PersonalAccessToken: githubToken,
// 		RepoId:              repoId,
// 		TargetNamespace:     "ucode/ucode_micro_frontend",
// 		NewName:             function.Path,
// 		GitlabToken:         h.cfg.GitlabIntegrationToken,
// 	})
// 	if err != nil {
// 		return github.ImportResponse{}, err
// 	}

// 	_, err = gitlab.UpdateProject(gitlab.IntegrationData{
// 		GitlabIntegrationUrl:   h.cfg.GitlabIntegrationURL,
// 		GitlabIntegrationToken: h.cfg.GitlabIntegrationToken,
// 		GitlabProjectId:        importResponse.ID,
// 		GitlabGroupId:          h.cfg.GitlabGroupIdMicroFE,
// 	}, map[string]interface{}{
// 		"ci_config_path": ".gitlab-ci.yml",
// 	})
// 	if err != nil {
// 		return github.ImportResponse{}, err
// 	}

// 	host := make(map[string]interface{})
// 	host["key"] = "INGRESS_HOST"
// 	host["value"] = function.Url

// 	_, err = gitlab.CreateProjectVariable(gitlab.IntegrationData{
// 		GitlabIntegrationUrl:   h.cfg.GitlabIntegrationURL,
// 		GitlabIntegrationToken: h.cfg.GitlabIntegrationToken,
// 		GitlabProjectId:        importResponse.ID,
// 		GitlabGroupId:          h.cfg.GitlabGroupIdMicroFE,
// 	}, host)
// 	if err != nil {
// 		return github.ImportResponse{}, err
// 	}

// 	time.Sleep(3 * time.Second)

// 	err = gitlab.AddFilesToRepo(h.cfg.GitlabIntegrationToken, h.cfg.PathToClone, importResponse.ID, function.Branch)
// 	if err != nil {
// 		return github.ImportResponse{}, err
// 	}

// 	return
// }

// func (h *Handler) pipelineStatus(services services.ServiceManagerI, function *fn.Function, repoId int) error {
// 	time.Sleep(10 * time.Second)
// 	err := github.AddCiFile(h.cfg.GitlabIntegrationToken, h.cfg.PathToClone, repoId, function.Branch, "github_integration")
// 	if err != nil {
// 		err = github.DeleteRepository(h.cfg.GitlabIntegrationToken, repoId)
// 		if err != nil {
// 			return err
// 		}
// 		return err
// 	}

// 	for {
// 		time.Sleep(70 * time.Second)
// 		pipeline, err := github.GetLatestPipeline(h.cfg.GitlabIntegrationToken, function.Branch, repoId)
// 		if err != nil {
// 			err := github.DeleteRepository(h.cfg.GitlabIntegrationToken, repoId)
// 			if err != nil {
// 				return err
// 			}
// 			return err
// 		}

// 		if pipeline.Status == "failed" {
// 			logResponse, err := github.GetPipelineLog(fmt.Sprintf("%v", repoId))
// 			if err != nil {
// 				return err
// 			}

// 			services.FunctionService().FunctionService().Update(
// 				context.Background(),
// 				&fn.Function{
// 					Id:               function.Id,
// 					Path:             function.Path,
// 					Name:             function.Name,
// 					Description:      function.Description,
// 					FunctionFolderId: function.FunctionFolderId,
// 					ProjectId:        function.ProjectId,
// 					EnvironmentId:    function.EnvironmentId,
// 					Type:             function.Type,
// 					Url:              function.Url,
// 					FrameworkType:    function.FrameworkType,
// 					SourceUrl:        function.SourceUrl,
// 					Branch:           function.Branch,
// 					PipelineStatus:   pipeline.Status,
// 					RepoId:           fmt.Sprintf("%v", repoId),
// 					ErrorMessage:     logResponse.Log,
// 					JobName:          logResponse.JobName,
// 					Resource:         function.Resource,
// 					ProvidedName:     function.ProvidedName,
// 				},
// 			)
// 			err = github.DeleteRepository(h.cfg.GitlabIntegrationToken, repoId)
// 			if err != nil {
// 				return err
// 			}

// 			return nil
// 		}

// 		_, err = services.FunctionService().FunctionService().Update(
// 			context.Background(),
// 			&fn.Function{
// 				Id:               function.Id,
// 				Path:             function.Path,
// 				Name:             function.Name,
// 				Description:      function.Description,
// 				FunctionFolderId: function.FunctionFolderId,
// 				ProjectId:        function.ProjectId,
// 				EnvironmentId:    function.EnvironmentId,
// 				Type:             function.Type,
// 				Url:              function.Url,
// 				FrameworkType:    function.FrameworkType,
// 				SourceUrl:        function.SourceUrl,
// 				Branch:           function.Branch,
// 				PipelineStatus:   pipeline.Status,
// 				RepoId:           fmt.Sprintf("%v", repoId),
// 				ErrorMessage:     "",
// 				JobName:          "",
// 				Resource:         function.Resource,
// 				ProvidedName:     function.ProvidedName,
// 			},
// 		)
// 		if err != nil {
// 			err := github.DeleteRepository(h.cfg.GitlabIntegrationToken, repoId)
// 			if err != nil {
// 				return err
// 			}
// 			return err
// 		}

// 		if pipeline.Status == "success" || pipeline.Status == "skipped" {
// 			err := github.DeleteRepository(h.cfg.GitlabIntegrationToken, repoId)
// 			if err != nil {
// 				return err
// 			}
// 			return nil
// 		}
// 	}
// }
