package services

import (
	"context"
	"ucode/ucode_go_function_service/config"
	"ucode/ucode_go_function_service/genproto/company_service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type CompanyServiceI interface {
	Project() company_service.ProjectServiceClient
	Environment() company_service.EnvironmentServiceClient
	Resource() company_service.ResourceServiceClient
	ServiceResource() company_service.MicroserviceResourceClient
	Redirect() company_service.RedirectUrlServiceClient
	IntegrationResource() company_service.IntegrationResourceServiceClient
	Billing() company_service.BillingServiceClient
}

type companyServiceClient struct {
	projectService             company_service.ProjectServiceClient
	environmentService         company_service.EnvironmentServiceClient
	resourceService            company_service.ResourceServiceClient
	serviceResource            company_service.MicroserviceResourceClient
	redirectService            company_service.RedirectUrlServiceClient
	integrationResourceService company_service.IntegrationResourceServiceClient
	billingService             company_service.BillingServiceClient
}

func NewCompanyServiceClient(ctx context.Context, cfg config.Config) (CompanyServiceI, error) {
	connCompanyService, err := grpc.NewClient(
		cfg.CompanyServiceHost+cfg.CompanyServicePort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return &companyServiceClient{
		projectService:             company_service.NewProjectServiceClient(connCompanyService),
		environmentService:         company_service.NewEnvironmentServiceClient(connCompanyService),
		resourceService:            company_service.NewResourceServiceClient(connCompanyService),
		serviceResource:            company_service.NewMicroserviceResourceClient(connCompanyService),
		redirectService:            company_service.NewRedirectUrlServiceClient(connCompanyService),
		integrationResourceService: company_service.NewIntegrationResourceServiceClient(connCompanyService),
		billingService:             company_service.NewBillingServiceClient(connCompanyService),
	}, nil
}

func (g *companyServiceClient) Project() company_service.ProjectServiceClient {
	return g.projectService
}

func (g *companyServiceClient) Environment() company_service.EnvironmentServiceClient {
	return g.environmentService
}

func (g *companyServiceClient) Resource() company_service.ResourceServiceClient {
	return g.resourceService
}

func (g *companyServiceClient) ServiceResource() company_service.MicroserviceResourceClient {
	return g.serviceResource
}

func (g *companyServiceClient) Redirect() company_service.RedirectUrlServiceClient {
	return g.redirectService
}

func (g *companyServiceClient) IntegrationResource() company_service.IntegrationResourceServiceClient {
	return g.integrationResourceService
}

func (g *companyServiceClient) Billing() company_service.BillingServiceClient {
	return g.billingService
}
