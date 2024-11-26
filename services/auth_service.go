package services

import (
	"context"
	"time"
	"ucode/ucode_go_function_service/config"
	"ucode/ucode_go_function_service/genproto/auth_service"

	grpcpool "github.com/processout/grpc-go-pool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type AuthServiceI interface {
	Session(ctx context.Context) (auth_service.SessionServiceClient, *grpcpool.ClientConn, error)
	Integration() auth_service.IntegrationServiceClient
	ApiKey() auth_service.ApiKeysClient
	ApiKeyUsage() auth_service.ApiKeyUsageServiceClient
}

type authServiceClient struct {
	sessionService     *grpcpool.Pool
	integrationService auth_service.IntegrationServiceClient
	sessionServiceAuth auth_service.SessionServiceClient
	apiKeyService      auth_service.ApiKeysClient
	apiKeyUsageService auth_service.ApiKeyUsageServiceClient
}

func NewAuthServiceClient(ctx context.Context, cfg config.Config) (AuthServiceI, error) {
	connAuthService, err := grpc.NewClient(
		cfg.AuthServiceHost+cfg.AuthGRPCPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	factory := func() (*grpc.ClientConn, error) {
		conn, err := grpc.NewClient(
			cfg.AuthServiceHost+cfg.AuthGRPCPort,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(52428800), grpc.MaxCallSendMsgSize(52428800)),
		)
		if err != nil {
			return nil, err
		}
		return conn, err
	}

	sessionServicePool, err := grpcpool.New(factory, 12, 18, time.Second*3)
	if err != nil {
		return nil, err
	}

	return &authServiceClient{
		sessionService:     sessionServicePool,
		sessionServiceAuth: auth_service.NewSessionServiceClient(connAuthService),
		integrationService: auth_service.NewIntegrationServiceClient(connAuthService),
		apiKeyService:      auth_service.NewApiKeysClient(connAuthService),
		apiKeyUsageService: auth_service.NewApiKeyUsageServiceClient(connAuthService),
	}, nil
}

func (g *authServiceClient) Session(ctx context.Context) (auth_service.SessionServiceClient, *grpcpool.ClientConn, error) {
	conn, err := g.sessionService.Get(ctx)
	if err != nil {
		return nil, nil, err
	}
	service := auth_service.NewSessionServiceClient(conn)

	return service, conn, nil
}

func (g *authServiceClient) Integration() auth_service.IntegrationServiceClient {
	return g.integrationService
}

func (g *authServiceClient) ApiKey() auth_service.ApiKeysClient {
	return g.apiKeyService
}

func (g *authServiceClient) ApiKeyUsage() auth_service.ApiKeyUsageServiceClient {
	return g.apiKeyUsageService
}
