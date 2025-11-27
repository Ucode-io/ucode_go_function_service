package services

import (
	"context"
	"ucode/ucode_go_function_service/config"
	obs "ucode/ucode_go_function_service/genproto/object_builder_service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type BuilderServiceI interface {
	Function() obs.FunctionServiceClient
	CustomEvent() obs.CustomEventServiceClient
	VersionHistory() obs.VersionHistoryServiceClient
}

type builderServiceClient struct {
	customEventService    obs.CustomEventServiceClient
	functionService       obs.FunctionServiceClient
	versionHistoryService obs.VersionHistoryServiceClient
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
		customEventService:    obs.NewCustomEventServiceClient(connObjectBuilderService),
		functionService:       obs.NewFunctionServiceClient(connObjectBuilderService),
		versionHistoryService: obs.NewVersionHistoryServiceClient(connObjectBuilderService),
	}, nil
}

func (g *builderServiceClient) CustomEvent() obs.CustomEventServiceClient {
	return g.customEventService
}

func (g *builderServiceClient) Function() obs.FunctionServiceClient {
	return g.functionService
}

func (g *builderServiceClient) VersionHistory() obs.VersionHistoryServiceClient {
	return g.versionHistoryService
}
