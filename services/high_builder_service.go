package services

import (
	"context"
	"ucode/ucode_go_function_service/config"
	obs "ucode/ucode_go_function_service/genproto/object_builder_service"

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
		customEventService:    obs.NewCustomEventServiceClient(connObjectBuilderService),
		functionService:       obs.NewFunctionServiceV2Client(connObjectBuilderService),
		versionHistoryService: obs.NewVersionHistoryServiceClient(connObjectBuilderService),
	}, nil
}
