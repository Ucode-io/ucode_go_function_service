package main

import (
	"context"
	"ucode/ucode_go_function_service/api"
	"ucode/ucode_go_function_service/api/handlers"
	"ucode/ucode_go_function_service/config"
	"ucode/ucode_go_function_service/pkg/caching"
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

	cache, err := caching.NewExpiringLRUCache(config.LRU_CACHE_SIZE)
	if err != nil {
		log.Error("Error adding caching.", logger.Error(err))
	}

	var (
		h = handlers.NewHandler(cfg, log, grpcSvcs, cache)
		r = gin.New()
	)

	r.Use(gin.Logger(), gin.Recovery())
	api.SetUpAPI(r, h, cfg)

	log.Info("server is running...")
	if err := r.Run(cfg.HTTPPort); err != nil {
		return
	}
}
