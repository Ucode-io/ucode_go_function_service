package services

import (
	"context"
	"ucode/ucode_go_function_service/config"
	nb "ucode/ucode_go_function_service/genproto/new_object_builder_service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GoBuilderServiceI interface {
	Function() nb.FunctionServiceV2Client
	CustomEvent() nb.CustomEventServiceClient
}

type goBuilderServiceClient struct {
	functionService    nb.FunctionServiceV2Client
	customEventService nb.CustomEventServiceClient
}

func NewGoBuilderServiceClient(ctx context.Context, cfg config.Config) (GoBuilderServiceI, error) {
	connGoBuilderService, err := grpc.NewClient(
		cfg.GoObjectBuilderServiceHost+cfg.GoObjectBuilderGRPCPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	if err != nil {
		return nil, err
	}

	return &goBuilderServiceClient{
		functionService:    nb.NewFunctionServiceV2Client(connGoBuilderService),
		customEventService: nb.NewCustomEventServiceClient(connGoBuilderService),
	}, nil
}

func (g *goBuilderServiceClient) Function() nb.FunctionServiceV2Client {
	return g.functionService
}

func (g *goBuilderServiceClient) CustomEvent() nb.CustomEventServiceClient {
	return g.customEventService
}
