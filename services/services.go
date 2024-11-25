package services

import (
	"context"
	"ucode/ucode_go_function_service/config"
)

type ServiceManagerI interface {
	BuilderService() BuilderServiceI
	HighBuilderService() BuilderServiceI
	GetBuilderServiceByType(nodeType string) BuilderServiceI
	GoObjectBuilderService() GoBuilderServiceI
	CompanyService() CompanyServiceI
}

type grpcClients struct {
	builderService         BuilderServiceI
	highBuilderService     BuilderServiceI
	goObjectBuilderService GoBuilderServiceI
	companyService         CompanyServiceI
}

func NewGrpcClients(ctx context.Context, cfg config.Config) (ServiceManagerI, error) {
	builderServiceClient, err := NewBuilderServiceClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	highBuilderServiceClient, err := NewHighBuilderServiceClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	goObjectBuilderServiceClient, err := NewGoBuilderServiceClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	companyServiceClient, err := NewCompanyServiceClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return grpcClients{
		builderService:         builderServiceClient,
		highBuilderService:     highBuilderServiceClient,
		goObjectBuilderService: goObjectBuilderServiceClient,
		companyService:         companyServiceClient,
	}, nil
}

func (g grpcClients) GetBuilderServiceByType(nodeType string) BuilderServiceI {
	switch nodeType {
	case config.LOW_NODE_TYPE:
		return g.builderService
	case config.HIGH_NODE_TYPE:
		return g.highBuilderService
	}

	return g.builderService
}

func (g grpcClients) BuilderService() BuilderServiceI {
	return g.builderService
}

func (g grpcClients) GoObjectBuilderService() GoBuilderServiceI {
	return g.goObjectBuilderService
}

func (g grpcClients) HighBuilderService() BuilderServiceI {
	return g.highBuilderService
}

func (g grpcClients) CompanyService() CompanyServiceI {
	return g.companyService
}
