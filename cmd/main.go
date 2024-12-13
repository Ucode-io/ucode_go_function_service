package main

import (
	"context"
	"encoding/json"
	"fmt"
	"ucode/ucode_go_function_service/api"
	"ucode/ucode_go_function_service/api/handlers"
	"ucode/ucode_go_function_service/config"
	"ucode/ucode_go_function_service/pkg/logger"
	"ucode/ucode_go_function_service/services"

	"github.com/gin-gonic/gin"
)

func main() {
	var (
		loggerLevel = new(string)
		cfg         = config.Load()
	)
	*loggerLevel = logger.LevelDebug

	cfgJson, _ := json.Marshal(cfg)
	fmt.Println("-------CONFIG------", string(cfgJson))

	switch cfg.Environment {
	case config.DebugMode:
		*loggerLevel = logger.LevelDebug
		gin.SetMode(gin.DebugMode)
	case config.TestMode:
		*loggerLevel = logger.LevelDebug
		gin.SetMode(gin.TestMode)
	default:
		*loggerLevel = logger.LevelInfo
		gin.SetMode(gin.ReleaseMode)
	}

	log := logger.NewLogger("ucode/ucode_fuction_service", *loggerLevel)
	defer func() {
		err := logger.Cleanup(log)
		if err != nil {
			return
		}
	}()

	grpcSvcs, err := services.NewGrpcClients(context.Background(), cfg)
	if err != nil {
		log.Error("Error adding grpc client with base config. NewGrpcClients", logger.Error(err))
		return
	}

	var (
		h = handlers.NewHandler(cfg, log, grpcSvcs)
		r = gin.New()
	)

	r.Use(gin.Logger(), gin.Recovery())
	api.SetUpAPI(r, h, cfg)

	log.Info("server is running...")
	if err := r.Run(cfg.HTTPPort); err != nil {
		return
	}
}
