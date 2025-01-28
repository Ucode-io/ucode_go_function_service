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
)
