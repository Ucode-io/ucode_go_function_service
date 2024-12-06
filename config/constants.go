package config

import "time"

const (
	// SERVICE TYPES
	LOW_NODE_TYPE  = "LOW"
	HIGH_NODE_TYPE = "HIGH"

	// FUNCTION TYPES
	FUNCTION = "FUNCTION"
	MICROFE  = "MICRO_FRONTEND"

	// ACTION TYPES
	DELETE string = "DELETE"
	UPDATE string = "UPDATE"
	CREATE string = "CREATE"

	// Cache settings
	CACHE_WAIT         = "WAIT"
	REDIS_KEY_TIMEOUT  = 280 * time.Second
	LRU_CACHE_SIZE     = 10000
	REDIS_TIMEOUT      = 5 * time.Minute
	REDIS_WAIT_TIMEOUT = 1 * time.Second
	REDIS_SLEEP        = 100 * time.Millisecond
)

var FunctionResource = map[string]bool{
	"ucode_gitlab": true,
}
