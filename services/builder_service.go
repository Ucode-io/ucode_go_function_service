package services

import (
	"context"
	"ucode/ucode_go_function_service/config"
	"ucode/ucode_go_function_service/genproto/object_builder_service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type BuilderServiceI interface {
	Function() object_builder_service.FunctionServiceClient
	CustomEvent() object_builder_service.CustomEventServiceClient
}

type builderServiceClient struct {
	customEventService object_builder_service.CustomEventServiceClient
	functionService    object_builder_service.FunctionServiceClient
}

func NewBuilderServiceClient(ctx context.Context, cfg config.Config) (BuilderServiceI, error) {
	connObjectBuilderService, err := grpc.NewClient(
		cfg.ObjectBuilderServiceHost+cfg.ObjectBuilderGRPCPort,
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

func (g *builderServiceClient) CustomEvent() object_builder_service.CustomEventServiceClient {
	return g.customEventService
}

func (g *builderServiceClient) Function() object_builder_service.FunctionServiceClient {
	return g.functionService
}
