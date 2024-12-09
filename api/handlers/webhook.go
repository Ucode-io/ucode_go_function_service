package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"ucode/ucode_go_function_service/api/models"
	"ucode/ucode_go_function_service/api/status_http"
	pb "ucode/ucode_go_function_service/genproto/company_service"
	"ucode/ucode_go_function_service/pkg/github"
	"ucode/ucode_go_function_service/pkg/logger"
	"ucode/ucode_go_function_service/pkg/util"

	"github.com/gin-gonic/gin"
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
	})
	if err != nil {
		h.handleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	time.Sleep(2 * time.Second)
	h.handleResponse(c, status_http.Created, nil)
}

func (h *Handler) HandleWebhook(c *gin.Context) {
	var payload map[string]interface{}

	fmt.Println("-----------RemoteAddr---------", c.Request.RemoteAddr)

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

	h.log.Info("From Webhook", logger.Any("data", string(body)))

	// 	if !(github.VerifySignature(c.GetHeader("X-Hub-Signature"), body, []byte(h.cfg.WebhookSecret))) {
	// 		h.handleResponse(c, status_http.BadRequest, "Failed to verify signature")
	// 		return
	// 	}

	// 	var (
	// 		branchFromWebhook = cast.ToString(payload["ref"])
	// 		repository        = cast.ToStringMap(payload["repository"])
	// 		hook              = cast.ToStringMap(payload["hook"])

	// 		owner           = cast.ToStringMap(repository["owner"])
	// 		repoId          = cast.ToString(repository["id"])
	// 		repoName        = cast.ToString(repository["name"])
	// 		repoDescription = cast.ToString(repository["description"])
	// 		htmlUrl         = cast.ToString(repository["html_url"])

	// 		config = cast.ToStringMap(hook["config"])

	// 		frameworkType = cast.ToString(config["framework_type"])
	// 		functionType  = cast.ToString(config["type"])
	// 		branch        = cast.ToString(config["branch"])
	// 		resourceType  = cast.ToString(config["resource_id"])
	// 		name          = cast.ToString(config["name"])

	// 		username = cast.ToString(owner["login"])
	// 	)

	// 	if branchFromWebhook != "" {
	// 		parts := strings.Split(branchFromWebhook, "/")
	// 		branch = parts[len(parts)-1]
	// 	}

	// 	h.services.CompanyService().Resource().GetProjectResourceList()

	// 	resources, err := h.services.CompanyService().IntegrationResource().GetByUsername(
	// 		c.Request.Context(),
	// 		&pb.GetByUsernameRequest{Username: username},
	// 	)
	// 	if err != nil {
	// 		h.handleResponse(c, status_http.InternalServerError, err.Error())
	// 		return
	// 	}

	// 	for _, r := range resources.IntegrationResources {
	// 		resource, err := h.services.CompanyService().ServiceResource().GetSingle(
	// 			c.Request.Context(),
	// 			&pb.GetSingleServiceResourceReq{
	// 				ProjectId:     r.ProjectId,
	// 				EnvironmentId: r.EnvironmentId,
	// 				ServiceType:   pb.ServiceType_FUNCTION_SERVICE,
	// 			},
	// 		)
	// 		if err != nil {
	// 			h.handleResponse(c, status_http.InternalServerError, err.Error())
	// 			return
	// 		}

	// 		services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	// 		if err != nil {
	// 			h.handleResponse(c, status_http.InternalServerError, err.Error())
	// 			return
	// 		}

	// 		function, functionErr := services.FunctionService().FunctionService().GetSingle(
	// 			c.Request.Context(),
	// 			&fn.FunctionPrimaryKey{
	// 				ProjectId: resource.ResourceEnvironmentId,
	// 				SourceUrl: htmlUrl,
	// 				Branch:    branch,
	// 			},
	// 		)
	// 		if function != nil {
	// 			functionType = function.Type
	// 		}

	// 		if functionType == "FUNCTION" {
	// 			if functionErr != nil {
	// 				function, err = services.FunctionService().FunctionService().Create(
	// 					c.Request.Context(),
	// 					&fn.CreateFunctionRequest{
	// 						Path:           repoName,
	// 						Name:           name,
	// 						Description:    repoDescription,
	// 						ProjectId:      resource.ResourceEnvironmentId,
	// 						EnvironmentId:  resource.EnvironmentId,
	// 						Type:           "FUNCTION",
	// 						SourceUrl:      htmlUrl,
	// 						Branch:         branch,
	// 						PipelineStatus: "running",
	// 						Resource:       resourceType,
	// 					},
	// 				)
	// 				if err != nil {
	// 					h.handleResponse(c, status_http.InvalidArgument, err.Error())
	// 					return
	// 				}
	// 			} else {
	// 				_, _ = services.FunctionService().FunctionService().Update(
	// 					c.Request.Context(),
	// 					&fn.Function{
	// 						Id:             function.Id,
	// 						Path:           function.Path,
	// 						Name:           function.Name,
	// 						Description:    function.Description,
	// 						ProjectId:      function.ProjectId,
	// 						EnvironmentId:  function.EnvironmentId,
	// 						Type:           function.Type,
	// 						SourceUrl:      function.SourceUrl,
	// 						Branch:         function.Branch,
	// 						PipelineStatus: "running",
	// 						Resource:       function.Resource,
	// 						ProvidedName:   function.ProvidedName,
	// 					},
	// 				)
	// 				function.PipelineStatus = "running"
	// 			}
	// 			go h.deployOpenfaas(services, r.Token, repoId, function)
	// 		} else {
	// 			repoHost := fmt.Sprintf("%s-%s", uuid.New(), h.cfg.GitlabHostMicroFE)

	// 			if functionErr != nil {
	// 				function, err = services.FunctionService().FunctionService().Create(
	// 					c.Request.Context(),
	// 					&fn.CreateFunctionRequest{
	// 						Path:           fmt.Sprintf("%s_%s", repoName, uuid.New()),
	// 						Name:           repoName,
	// 						Description:    repoDescription,
	// 						ProjectId:      resource.ResourceEnvironmentId,
	// 						EnvironmentId:  resource.EnvironmentId,
	// 						Type:           "MICRO_FRONTEND",
	// 						Url:            repoHost,
	// 						FrameworkType:  frameworkType,
	// 						SourceUrl:      htmlUrl,
	// 						Branch:         branch,
	// 						PipelineStatus: "running",
	// 						Resource:       resourceType,
	// 						ProvidedName:   name,
	// 					},
	// 				)
	// 				if err != nil {
	// 					h.handleResponse(c, status_http.GRPCError, err.Error())
	// 					return
	// 				}
	// 			} else {
	// 				services.FunctionService().FunctionService().Update(
	// 					c.Request.Context(),
	// 					&fn.Function{
	// 						Id:             function.Id,
	// 						Path:           function.Path,
	// 						Name:           function.Name,
	// 						Description:    function.Description,
	// 						ProjectId:      function.ProjectId,
	// 						EnvironmentId:  function.EnvironmentId,
	// 						Type:           function.Type,
	// 						Url:            function.Url,
	// 						FrameworkType:  function.FrameworkType,
	// 						SourceUrl:      function.SourceUrl,
	// 						Branch:         function.Branch,
	// 						PipelineStatus: "running",
	// 						Resource:       function.Resource,
	// 						ProvidedName:   function.ProvidedName,
	// 					},
	// 				)
	// 				function.PipelineStatus = "running"
	// 			}

	// 			importResponse, err := h.deployMicrofrontend(r.Token, repoId, function)
	// 			if err != nil {
	// 				h.handleResponse(c, status_http.GRPCError, err.Error())
	// 				return
	// 			}
	// 			go h.pipelineStatus(services, function, importResponse.ID)
	// 		}
	// 	}
	// }

	// func (h *Handler) deployOpenfaas(services services.ServiceManagerI, githubToken, repoId string, function *fn.Function) (gitlab.ImportResponse, error) {
	// 	importResponse, err := github.ImportFromGithub(github.ImportData{
	// 		PersonalAccessToken: githubToken,
	// 		RepoId:              repoId,
	// 		TargetNamespace:     "ucode_functions_group",
	// 		NewName:             function.Path,
	// 		GitlabToken:         h.cfg.GitlabIntegrationToken,
	// 	})
	// 	if err != nil {
	// 		return github.ImportResponse{}, err
	// 	}

	// 	time.Sleep(10 * time.Second)
	// 	err = github.AddCiFile(h.cfg.GitlabIntegrationToken, h.cfg.PathToClone, importResponse.ID, function.Branch, "openfaas_integration")
	// 	if err != nil {
	// 		err := github.DeleteRepository(h.cfg.GitlabIntegrationToken, importResponse.ID)
	// 		if err != nil {
	// 			return github.ImportResponse{}, err
	// 		}
	// 	}

	// 	for {
	// 		time.Sleep(60 * time.Second)
	// 		pipeline, err := github.GetLatestPipeline(h.cfg.GitlabIntegrationToken, function.Branch, importResponse.ID)
	// 		if err != nil {
	// 			services.FunctionService().FunctionService().Update(
	// 				context.Background(),
	// 				&fn.Function{
	// 					Id:             function.Id,
	// 					Path:           function.Path,
	// 					Name:           function.Name,
	// 					Description:    function.Description,
	// 					ProjectId:      function.ProjectId,
	// 					EnvironmentId:  function.EnvironmentId,
	// 					Type:           function.Type,
	// 					Url:            function.Url,
	// 					SourceUrl:      function.SourceUrl,
	// 					Branch:         function.Branch,
	// 					PipelineStatus: "failed",
	// 					RepoId:         fmt.Sprintf("%v", importResponse.ID),
	// 					ErrorMessage:   "Failed to get pipeline status",
	// 					JobName:        "",
	// 					Resource:       function.Resource,
	// 					ProvidedName:   function.ProvidedName,
	// 				},
	// 			)
	// 			err := github.DeleteRepository(h.cfg.GitlabIntegrationToken, importResponse.ID)
	// 			if err != nil {
	// 				return github.ImportResponse{}, err
	// 			}
	// 			return github.ImportResponse{}, err
	// 		}

	// 		if pipeline.Status == "failed" {
	// 			logResp, err := github.GetPipelineLog(fmt.Sprintf("%v", importResponse.ID), h.cfg.GitlabIntegrationURL, h.cfg.GitlabIntegrationToken)
	// 			if err != nil {
	// 				return github.ImportResponse{}, err
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
	// 					RepoId:           fmt.Sprintf("%v", importResponse.ID),
	// 					ErrorMessage:     logResp.Log,
	// 					JobName:          logResp.JobName,
	// 					Resource:         function.Resource,
	// 					ProvidedName:     function.ProvidedName,
	// 				},
	// 			)

	// 			err = github.DeleteRepository(h.cfg.GitlabIntegrationToken, importResponse.ID)
	// 			if err != nil {
	// 				return github.ImportResponse{}, err
	// 			}
	// 			return github.ImportResponse{}, err
	// 		}

	// 		services.FunctionService().FunctionService().Update(
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
	// 				RepoId:           fmt.Sprintf("%v", importResponse.ID),
	// 				ErrorMessage:     "",
	// 				JobName:          "",
	// 				Resource:         function.Resource,
	// 				ProvidedName:     function.ProvidedName,
	// 			},
	// 		)

	// 		if pipeline.Status == "success" || pipeline.Status == "skipped" {
	// 			err := github.DeleteRepository(h.cfg.GitlabIntegrationToken, importResponse.ID)
	// 			if err != nil {
	// 				return github.ImportResponse{}, err
	// 			}
	// 			return github.ImportResponse{}, nil
	// 		}
	// 	}
	// }

	// func (h *Handler) deployMicrofrontend(githubToken, repoId string, function *fn.Function) (github.ImportResponse, error) {
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

	return
}

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
