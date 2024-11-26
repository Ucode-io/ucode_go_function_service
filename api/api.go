package api

import (
	"ucode/ucode_go_function_service/api/handlers"
	"ucode/ucode_go_function_service/config"

	"github.com/gin-gonic/gin"
)

func SetUpAPI(r *gin.Engine, h handlers.Handler, cfg config.Config) {
	r.Use(customCORSMiddleware())

	r.Use(h.AdminAuthMiddleware())
	r.POST("/v2/function", h.CreateFunction)


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
