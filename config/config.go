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
	// Service Creds
	ServiceName   string
	ServiceHost   string
	ServicePort   string
	Environment   string
	Version       string
	HTTPBaseURL   string
	HTTPPort      string
	HTTPScheme    string
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

	// Fass urls
	OpeFassBaseUrl string
	KnativeBaseUrl string

	// Github Creds
	GithubBaseUrl      string
	GithubApiBaseUrl   string
	GithubClientId     string
	GithubClientSecret string
	PathToClone        string

	// Gitlab Creds
	GitlabIntegrationURL string
	ProjectUrl           string
	WebhookSecret        string

	// Knative Gitlab Creds
	GitlabKnativeToken     string
	GitlabKnativeGroupId   int
	GitlabKnativeProjectId int

	// Openfass Gitlab Creds
	GitlabOpenFassToken     string //
	GitlabOpenFassGroupId   int    // 2008
	GitlabOpenFassProjectId int    // 1467

	// Microfront Gitlab Creds
	GitlabHostMicroFront             string
	GitlabTokenMicroFront            string
	GitlabGroupIdMicroFront          int
	GitlabProjectIdMicroFront        int
	GitlabProjectIdMicroFrontReact   int
	GitlabProjectIdMicroFrontVue     int
	GitlabProjectIdMicroFrontAngular int

	// Grafana
	GrafanaBaseUrl string
	GrafanaAuth    string
}

func Load() Config {
	if err := godotenv.Load("/app/.env"); err != nil {
		if err := godotenv.Load(".env"); err != nil {
			log.Println("No .env file found")
		}
		log.Println("No /app/.env file found")
	}

	config := Config{}

	// Service Creds
	config.ServiceName = cast.ToString(getOrReturnDefaultValue("SERVICE_NAME", "ucode_go_function_service"))
	config.HTTPBaseURL = cast.ToString(getOrReturnDefaultValue("HTTP_BASE_URL", "https://api.admin.u-code.io"))
	config.ServiceHost = cast.ToString(getOrReturnDefaultValue("SERVICE_HOST", "localhost"))
	config.HTTPPort = cast.ToString(getOrReturnDefaultValue("HTTP_PORT", ":8080"))
	config.HTTPScheme = cast.ToString(getOrReturnDefaultValue("HTTP_SCHEME", "http"))
	config.Environment = cast.ToString(getOrReturnDefaultValue("ENVIRONMENT", DebugMode))
	config.Version = cast.ToString(getOrReturnDefaultValue("VERSION", "1.0"))
	config.DefaultOffset = cast.ToString(getOrReturnDefaultValue("DEFAULT_OFFSET", "0"))
	config.DefaultLimit = "60"

	// Company Service Creds
	config.CompanyServiceHost = cast.ToString(getOrReturnDefaultValue("COMPANY_SERVICE_HOST", "localhost"))
	config.CompanyServicePort = cast.ToString(getOrReturnDefaultValue("COMPANY_GRPC_PORT", ":8092"))

	// Obs Low Service Creds
	config.ObjectBuilderServiceHost = cast.ToString(getOrReturnDefaultValue("OBJECT_BUILDER_SERVICE_LOW_HOST", ""))
	config.ObjectBuilderGRPCPort = cast.ToString(getOrReturnDefaultValue("OBJECT_BUILDER_LOW_GRPC_PORT", ""))

	// Obs High Service Creds
	config.HighObjectBuilderServiceHost = cast.ToString(getOrReturnDefaultValue("OBJECT_BUILDER_SERVICE_HIGHT_HOST", ""))
	config.HighObjectBuilderGRPCPort = cast.ToString(getOrReturnDefaultValue("OBJECT_BUILDER_HIGH_GRPC_PORT", ""))

	// Go obs Service Creds
	config.GoObjectBuilderServiceHost = cast.ToString(getOrReturnDefaultValue("GO_OBJECT_BUILDER_SERVICE_GRPC_HOST", "localhost"))
	config.GoObjectBuilderGRPCPort = cast.ToString(getOrReturnDefaultValue("GO_OBJECT_BUILDER_SERVICE_GRPC_PORT", ":7107"))

	// Auth Service Creds
	config.AuthServiceHost = cast.ToString(getOrReturnDefaultValue("AUTH_SERVICE_HOST", "localhost"))
	config.AuthGRPCPort = cast.ToString(getOrReturnDefaultValue("AUTH_GRPC_PORT", ":9103"))

	// Fass Urls
	config.OpeFassBaseUrl = cast.ToString(getOrReturnDefaultValue("OPENFASS_BASE_URL", "https://ofs.u-code.io/function/"))
	config.KnativeBaseUrl = cast.ToString(getOrReturnDefaultValue("KNATIVE_BASE_URL", "knative-fn.u-code.io"))

	// Github Creds
	config.GithubBaseUrl = cast.ToString(getOrReturnDefaultValue("GITHUB_BASE_URL", "https://github.com"))
	config.GithubApiBaseUrl = cast.ToString(getOrReturnDefaultValue("GITHUB_API_BASE_URL", "https://api.github.com"))
	config.GithubClientId = cast.ToString(getOrReturnDefaultValue("GITHUB_CLIENT_ID", "Ov23li4UK3p4sN41U3xS"))
	config.GithubClientSecret = cast.ToString(getOrReturnDefaultValue("GITHUB_CLIENT_SECRET", "4dd3740a1c9e0df1c1626d1028e22134c2faef06"))
	config.PathToClone = cast.ToString(getOrReturnDefaultValue("CLONE_PATH", "./app"))

	// Gitlab Creds
	config.GitlabIntegrationURL = cast.ToString(getOrReturnDefaultValue("GITLAB_URL", "https://gitlab.udevs.io"))
	config.ProjectUrl = cast.ToString(getOrReturnDefaultValue("PROJECT_URL", "https://admin-api.ucode.run"))
	config.WebhookSecret = cast.ToString(getOrReturnDefaultValue("WEBHOOK_SECRET", "X8kJnsNHD9f4nRQfjs72YLSfPqxjG+PWRjxN3KBuDhE="))

	// Knative Gitlab Creds
	config.GitlabKnativeToken = cast.ToString(getOrReturnDefaultValue("GITLAB_KNATIVE_TOKEN", "glpat-yfiNvrJqYWczk4Y2dnsq"))
	config.GitlabKnativeGroupId = cast.ToInt(getOrReturnDefaultValue("GITLAB_KNATIVE_GROUP_ID", 5466))
	config.GitlabKnativeProjectId = cast.ToInt(getOrReturnDefaultValue("GITLAB_KNATIVE_PROJECT_ID", 4622))

	// OpenFass Gitlab Creds
	config.GitlabOpenFassToken = cast.ToString(getOrReturnDefaultValue("GITLAB_OPENFASS_TOKEN", "glpat-HimXjgzaysg2EeiqLB_S"))
	config.GitlabOpenFassGroupId = cast.ToInt(getOrReturnDefaultValue("GITLAB_OPENFASS_GROUP_ID", 2008))
	config.GitlabOpenFassProjectId = cast.ToInt(getOrReturnDefaultValue("GITLAB_OPENFASS_PROJECT_ID", 1467))

	// Microfront Gitlab Creds
	config.GitlabGroupIdMicroFront = cast.ToInt(getOrReturnDefaultValue("GITLAB_MICROFRONT_GROUP_ID", 2604))
	config.GitlabProjectIdMicroFront = cast.ToInt(getOrReturnDefaultValue("GITLAB_MICROFRONT_PROJECT_ID", 0))
	config.GitlabTokenMicroFront = cast.ToString(getOrReturnDefaultValue("GITLAB_MICROFRONT_TOKEN", "glpat-wT2-JCyqDx4pWPzAXy12"))
	config.GitlabHostMicroFront = cast.ToString(getOrReturnDefaultValue("GITLAB_MICROFRONT_HOST", "test-page.u-code.io"))
	config.GitlabProjectIdMicroFrontReact = cast.ToInt(getOrReturnDefaultValue("GITLAB_MICROFRONT_REACT_PROJECT_ID", 1993))
	config.GitlabProjectIdMicroFrontVue = cast.ToInt(getOrReturnDefaultValue("GITLAB_MICROFRONT_VUE_PROJECT_ID", 0))
	config.GitlabProjectIdMicroFrontAngular = cast.ToInt(getOrReturnDefaultValue("GITLAB_MICROFRONT_ANGULAR_PROJECT_ID", 0))

	// Grafana Creds
	config.GrafanaBaseUrl = cast.ToString(getOrReturnDefaultValue("GRAFANA_BASE_URL", "https://grafana.u-code.io"))
	config.GrafanaAuth = cast.ToString(getOrReturnDefaultValue("GRAFANA_AUTH", "ucode-dev:sie0eeBuZ3Neigageejo"))

	return config
}

func getOrReturnDefaultValue(key string, defaultValue interface{}) interface{} {
	val, exists := os.LookupEnv(key)

	if exists {
		return val
	}

	return defaultValue
}
