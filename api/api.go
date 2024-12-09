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
	v2 := r.Group("/v2")
	v2.Use(h.AuthMiddleware(cfg))
	// @securityDefinitions.apikey ApiKeyAuth
	// @in header
	// @name Authorization
	function := v1.Group("/function")
	{
		// Function (OpenFass, Knative)
		function.POST("", h.CreateFunction)
		function.GET("/:function_id", h.GetFunctionByID)
		function.GET("", h.GetAllFunctions)
		function.PUT("", h.UpdateFunction)
		function.DELETE(":function_id", h.DeleteFunction)

	}

	microFe := function.Group("/micro-frontend")
	{
		// MICROFRONTEND (React, Vue, Angular)
		microFe.POST("", h.CreateMicroFrontEnd)
		microFe.GET("/:micro-frontend-id", h.GetMicroFrontEndByID)
		microFe.GET("", h.GetAllMicroFrontEnd)
		microFe.PUT("", h.UpdateMicroFrontEnd)
		microFe.DELETE("/:micro-frontend-id", h.DeleteMicroFrontEnd)
	}

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

	github := r.Group("/github")
	{
		github.GET("/login", h.GithubLogin)
		github.GET("/user", h.GithubGetUser)
		github.GET("/repos", h.GithubGetRepos)
		github.GET("/branches", h.GithubGetBranches)
	}

	knativeFunc := v2.Group("invoke_function")
	{
		knativeFunc.POST("/:function-path", h.InvokeFuncByPath)
	}

	v2Webhook := v2.Group("/webhook")
	{
		v2Webhook.POST("/create", h.CreateWebhook)
		v2Webhook.POST("/handle", h.HandleWebhook)

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

func MaxAllowed(n int) gin.HandlerFunc {
	var (
		countReq int64
		sem      = make(chan struct{}, n)
		acquire  = func() {
			sem <- struct{}{}
			countReq++
		}
		release = func() {
			select {
			case <-sem:
			default:
			}
			countReq--
		}
	)

	return func(c *gin.Context) {
		go func() {
			acquire()       // before request
			defer release() // after request

			c.Set("sem", sem)
			c.Set("count_request", countReq)
		}()

		c.Next()
	}
}
