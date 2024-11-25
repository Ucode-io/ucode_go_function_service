package services

import (
	"context"
	"ucode/ucode_go_function_service/config"
	"ucode/ucode_go_function_service/genproto/object_builder_service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewHighBuilderServiceClient(ctx context.Context, cfg config.Config) (BuilderServiceI, error) {
	connObjectBuilderService, err := grpc.NewClient(
		cfg.HighObjectBuilderServiceHost+cfg.HighObjectBuilderGRPCPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(52428800), grpc.MaxCallSendMsgSize(52428800)),
	)
	if err != nil {
		return nil, err
	}

	return &builderServiceClient{
		customEventService: object_builder_service.NewCustomEventServiceClient(connObjectBuilderService),
		functionService:    object_builder_service.NewFunctionServiceClient(connObjectBuilderService),
	}, nil
}
