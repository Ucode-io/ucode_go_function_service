package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"handler/function"

	cache "github.com/golanguzb70/redis-cache"
	"github.com/rs/zerolog"
)

var (
	acceptingConnections int32
)

const defaultTimeout = 10 * time.Second

type Settings struct {
	enableCache bool
}

func NewSettings() *Settings {
	return &Settings{
		enableCache: false, // make it true to enable cache
	}
}

func NewParams(s *Settings) function.Params {
	response := function.Params{}
	response.Log = zerolog.New(os.Stdout).With().Timestamp().Logger()

	if s.enableCache {
		cacheConfig := &cache.Config{}
		cacheConfig.RedisHost, _ = getOrDefault("REDIS_HOST", "").(string)
		cacheConfig.RedisPort, _ = getOrDefault("REDIS_PORT", 6379).(int)
		cacheConfig.RedisUsername, _ = getOrDefault("REDIS_USERNAME", "").(string)
		cacheConfig.RedisPassword, _ = getOrDefault("REDIS_PASSWORD", "").(string)

		cacheClient, err := cache.New(cacheConfig)
		if err != nil {
			response.Log.Error().Msgf("Error creating cache client: %v", err)
			response.CacheAvailable = false
		} else {
			response.CacheClient = cacheClient
			response.CacheAvailable = true
		}
	}

	return response
}

func main() {
	readTimeout := parseIntOrDurationValue(os.Getenv("read_timeout"), defaultTimeout)
	writeTimeout := parseIntOrDurationValue(os.Getenv("write_timeout"), defaultTimeout)
	healthInterval := parseIntOrDurationValue(os.Getenv("healthcheck_interval"), writeTimeout)

	s := &http.Server{
		Addr:           fmt.Sprintf(":%d", 8082),
		ReadTimeout:    readTimeout,
		WriteTimeout:   writeTimeout,
		MaxHeaderBytes: 1 << 20, // Max header of 1MB
	}

	params := NewParams(NewSettings())
	http.HandleFunc("/", function.NewHadler(params))

	params.Log.Info().Msg("Starting server")
	listenUntilShutdown(s, healthInterval, writeTimeout)
}

func listenUntilShutdown(s *http.Server, shutdownTimeout time.Duration, writeTimeout time.Duration) {
	idleConnsClosed := make(chan struct{})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM)

		<-sig

		log.Printf("[entrypoint] SIGTERM: no connections in: %s", shutdownTimeout.String())
		<-time.Tick(shutdownTimeout)

		ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
		defer cancel()

		if err := s.Shutdown(ctx); err != nil {
			log.Printf("[entrypoint] Error in Shutdown: %v", err)
		}

		log.Printf("[entrypoint] Exiting.")

		close(idleConnsClosed)
	}()

	// Run the HTTP server in a separate go-routine.
	go func() {
		if err := s.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("[entrypoint] Error ListenAndServe: %v", err)
			close(idleConnsClosed)
		}
	}()

	atomic.StoreInt32(&acceptingConnections, 1)

	<-idleConnsClosed
}

func parseIntOrDurationValue(val string, fallback time.Duration) time.Duration {
	if len(val) > 0 {
		parsedVal, parseErr := strconv.Atoi(val)
		if parseErr == nil && parsedVal >= 0 {
			return time.Duration(parsedVal) * time.Second
		}
	}

	duration, durationErr := time.ParseDuration(val)
	if durationErr != nil {
		return fallback
	}
	return duration
}

func getOrDefault(key string, def any) any {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	return val
}
