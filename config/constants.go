package config

import "time"

const (
	// SERVICE TYPES
	LOW_NODE_TYPE  string = "LOW"
	HIGH_NODE_TYPE string = "HIGH"

	// FUNCTION TYPES
	FUNCTION string = "FUNCTION"
	MICROFE  string = "MICRO_FRONTEND"
	KNATIVE  string = "KNATIVE"
	WORKFLOW string = "WORKFLOW"

	// ACTION TYPES
	DELETE string = "DELETE"
	UPDATE string = "UPDATE"
	CREATE string = "CREATE"

	// Cache settings
	CACHE_WAIT         string        = "WAIT"
	REDIS_KEY_TIMEOUT  time.Duration = 280 * time.Second
	REDIS_TIMEOUT      time.Duration = 5 * time.Minute
	REDIS_WAIT_TIMEOUT time.Duration = 1 * time.Second
	REDIS_SLEEP        time.Duration = 100 * time.Millisecond
	LRU_CACHE_SIZE                   = 10000

	// Path
	PathToCloneKnative    string = "knative_template"
	PathToCloneFunction   string = "openfass_template"
	PathToCloneMicroFront string = "react_template"

	// Gitlab Namespaces
	KnativeNamespace    string = "ucode/knative"
	OpenFassNamespace   string = "ucode_functions_group"
	MicroFrontNamaspece string = "ucode/ucode_micro_frontend"

	// Fare Types
	FARE_FUNCTION      string = "function"
	FARE_MICROFRONTEND string = "microfrontend"
	FARE_PROJECTS      string = "projects"

	ENTER_PRICE_TYPE string = "ENTER_PRICE"

	AccessDeniedError = "you don't have permission to access this function"

	DefaultBranch = "master"
	UGenBranch    = "u-gen"

	// UGenSHAFile is written to master during every promote and contains the
	// u-gen HEAD commit SHA at the time of that promote. check-changes compares
	// the current u-gen HEAD with this value to reliably detect new changes.
	UGenSHAFile = ".u-gen-sha"
)
