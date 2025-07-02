package api

import (
	"ucode/ucode_go_function_service/api/docs"
	"ucode/ucode_go_function_service/api/handlers"
	"ucode/ucode_go_function_service/config"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// SetUpAPI @description This is an api gateway
// @termsOfService https://u-code.io/
func SetUpAPI(r *gin.Engine, h handlers.Handler, cfg config.Config) {
	docs.SwaggerInfo.Title = cfg.ServiceName
	docs.SwaggerInfo.Version = cfg.Version
	docs.SwaggerInfo.Schemes = []string{cfg.HTTPScheme}

	r.Use(customCORSMiddleware())

	v1 := r.Group("/v1")
	v1.Use(h.AuthMiddleware(cfg))

	// @securityDefinitions.apikey ApiKeyAuth
	// @in header
	// @name Authorization
	collections := v1.Group("/collections")
	{
		collections.GET("/:collection/automation", h.GetAllAutomation)
		collections.POST("/:collection/automation", h.CreateAutomation)
		collections.PUT("/:collection/automation", h.UpdateAutomation)
		collections.GET("/:collection/automation/:id", h.GetByIdAutomation)
		collections.DELETE("/:collection/automation/:id", h.DeleteAutomation)
	}

	invokeFunction := v1.Group("/invoke_function")
	{
		invokeFunction.POST("", h.InvokeFunction)
		invokeFunction.POST("/:function-path", h.InvokeFunctionByPath)
	}

	github := v1.Group("/github")
	{
		github.GET("/login", h.GithubLogin)
		github.GET("/user", h.GithubGetUser)
		github.GET("/repos", h.GithubGetRepos)
		github.GET("/branches", h.GithubGetBranches)
	}

	gitlab := v1.Group("/gitlab")
	{
		gitlab.GET("/login", h.GitlabLogin)
		gitlab.GET("/user", h.GitlabGetUser)
		gitlab.GET("/repos", h.GitlabGetRepos)
		gitlab.GET("/branches", h.GitlabGetBranches)
	}

	v2 := r.Group("/v2")
	v2.POST("/webhook/handle", h.HandleWebhook)

	grafana := v2.Group("/grafana")
	{
		grafana.POST("/loki", h.GetGrafanaFunctionLogs)
		grafana.GET("/function", h.GetGrafanaFunctionList)
	}

	v2.Use(h.AuthMiddleware(cfg))
	functions := v2.Group("/function")
	{
		// Function (OpenFass, Knative)
		functions.POST("", h.CreateFunction)
		functions.GET("/:function_id", h.GetFunctionByID)
		functions.GET("", h.GetAllFunctions)
		functions.PUT("", h.UpdateFunction)
		functions.DELETE(":function_id", h.DeleteFunction)

	}

	microFe := v2.Group("/functions")
	{
		// MICROFRONTEND (React)
		microFe.POST("/micro-frontend", h.CreateMicroFrontEnd)
		microFe.GET("/micro-frontend/:micro-frontend-id", h.GetMicroFrontEndByID)
		microFe.GET("/micro-frontend", h.GetAllMicroFrontEnd)
		microFe.PUT("/micro-frontend", h.UpdateMicroFrontEnd)
		microFe.DELETE("/micro-frontend/:micro-frontend-id", h.DeleteMicroFrontEnd)
	}

	knativeFunc := r.Group("/v2/invoke_function")
	knativeFunc.Use(h.AuthFunctionMiddleware(cfg))
	{
		knativeFunc.POST("/:function-path", h.InvokeFuncByPath)
		knativeFunc.POST("/:function-path/*any", h.InvokeFuncByApiPath)
	}

	v2Webhook := v2.Group("/webhook")
	{
		v2Webhook.POST("/create", h.CreateWebhook)
	}

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

}

func customCORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "3600")
		c.Header("Access-Control-Allow-Methods", "*")
		c.Header("Access-Control-Allow-Headers", "*")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
