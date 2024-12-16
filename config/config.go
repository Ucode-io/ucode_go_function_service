package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cast"
)

const (
	DebugMode = "debug"
	// TestMode indicates service mode is test.
	TestMode = "test"
	// ReleaseMode indicates service mode is release.
	ReleaseMode = "release"
)

type Config struct {
	ServiceName string
	ServiceHost string
	ServicePort string

	Environment string
	Version     string

	HTTPBaseURL string
	HTTPPort    string
	HTTPScheme  string

	DefaultOffset string
	DefaultLimit  string

	// LOW
	ObjectBuilderServiceHost string
	ObjectBuilderGRPCPort    string

	// HIGH
	HighObjectBuilderServiceHost string
	HighObjectBuilderGRPCPort    string

	AuthServiceHost string
	AuthGRPCPort    string

	CompanyServiceHost string
	CompanyServicePort string

	GoObjectBuilderServiceHost string
	GoObjectBuilderGRPCPort    string

	OpeFassBaseUrl string
	KnativeBaseUrl string

	GithubBaseUrl      string
	GithubApiBaseUrl   string
	GithubClientId     string
	GithubClientSecret string
	PathToClone        string

	GitlabIntegrationURL   string
	GitlabIntegrationToken string
	GitlabGroupId          int
	GitlabProjectId        int
	ProjectUrl             string
	WebhookSecret          string
	GitlabGroupIdMicroFE   int
	GitlabProjectIdMicroFE int
	GitlabHostMicroFE      string
}

func Load() Config {
	if err := godotenv.Load("/app/.env"); err != nil {
		if err := godotenv.Load(".env"); err != nil {
			log.Println("No .env file found")
		}
		log.Println("No /app/.env file found")
	}

	config := Config{}

	config.ServiceName = cast.ToString(getOrReturnDefaultValue("SERVICE_NAME", "ucode_go_function_service"))
	config.HTTPBaseURL = cast.ToString(getOrReturnDefaultValue("HTTP_BASE_URL", "https://api.admin.u-code.io"))
	config.ServiceHost = cast.ToString(getOrReturnDefaultValue("SERVICE_HOST", "localhost"))
	config.HTTPPort = cast.ToString(getOrReturnDefaultValue("HTTP_PORT", ":8080"))
	config.HTTPScheme = cast.ToString(getOrReturnDefaultValue("HTTP_SCHEME", "http"))

	config.Environment = cast.ToString(getOrReturnDefaultValue("ENVIRONMENT", DebugMode))
	config.Version = cast.ToString(getOrReturnDefaultValue("VERSION", "1.0"))

	config.CompanyServiceHost = cast.ToString(getOrReturnDefaultValue("COMPANY_SERVICE_HOST", "localhost"))
	config.CompanyServicePort = cast.ToString(getOrReturnDefaultValue("COMPANY_GRPC_PORT", ":8092"))

	config.ObjectBuilderServiceHost = cast.ToString(getOrReturnDefaultValue("OBJECT_BUILDER_SERVICE_LOW_HOST", ""))
	config.ObjectBuilderGRPCPort = cast.ToString(getOrReturnDefaultValue("OBJECT_BUILDER_LOW_GRPC_PORT", ""))

	config.HighObjectBuilderServiceHost = cast.ToString(getOrReturnDefaultValue("OBJECT_BUILDER_SERVICE_HIGHT_HOST", ""))
	config.HighObjectBuilderGRPCPort = cast.ToString(getOrReturnDefaultValue("OBJECT_BUILDER_HIGH_GRPC_PORT", ""))

	config.GoObjectBuilderServiceHost = cast.ToString(getOrReturnDefaultValue("GO_OBJECT_BUILDER_SERVICE_GRPC_HOST", "localhost"))
	config.GoObjectBuilderGRPCPort = cast.ToString(getOrReturnDefaultValue("GO_OBJECT_BUILDER_SERVICE_GRPC_PORT", ":7107"))

	config.AuthServiceHost = cast.ToString(getOrReturnDefaultValue("AUTH_SERVICE_HOST", "localhost"))
	config.AuthGRPCPort = cast.ToString(getOrReturnDefaultValue("AUTH_GRPC_PORT", ":9103"))

	config.OpeFassBaseUrl = cast.ToString(getOrReturnDefaultValue("OPENFASS_BASE_URL", "https://ofs.u-code.io/function/"))
	config.KnativeBaseUrl = "knative-fn.u-code.io"

	config.GithubBaseUrl = cast.ToString(getOrReturnDefaultValue("GITHUB_BASE_URL", "https://github.com"))
	config.GithubApiBaseUrl = cast.ToString(getOrReturnDefaultValue("GITHUB_API_BASE_URL", "https://api.github.com"))
	config.GithubClientId = cast.ToString(getOrReturnDefaultValue("GITHUB_CLIENT_ID", "Ov23li4UK3p4sN41U3xS"))
	config.GithubClientSecret = cast.ToString(getOrReturnDefaultValue("GITHUB_CLIENT_SECRET", "4dd3740a1c9e0df1c1626d1028e22134c2faef06"))

	config.GitlabIntegrationURL = cast.ToString(getOrReturnDefaultValue("GITLAB_URL", "https://gitlab.udevs.io"))
	config.GitlabIntegrationToken = "glpat-pNPf-Twq4Rte3xbeDCB1"
	config.GitlabGroupId = cast.ToInt(getOrReturnDefaultValue("GITLAB_GROUP_ID", 2008))
	config.GitlabProjectId = cast.ToInt(getOrReturnDefaultValue("GITLAB_PROJECT_ID", 1467))
	config.ProjectUrl = cast.ToString(getOrReturnDefaultValue("PROJECT_URL", "https://admin-api.ucode.run"))
	config.WebhookSecret = cast.ToString(getOrReturnDefaultValue("WEBHOOK_SECRET", "X8kJnsNHD9f4nRQfjs72YLSfPqxjG+PWRjxN3KBuDhE="))

	config.DefaultOffset = cast.ToString(getOrReturnDefaultValue("DEFAULT_OFFSET", "0"))
	config.DefaultLimit = "60"

	return config
}

func getOrReturnDefaultValue(key string, defaultValue interface{}) interface{} {
	val, exists := os.LookupEnv(key)

	if exists {
		return val
	}

	return defaultValue
}
