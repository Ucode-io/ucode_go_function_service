package redis

import (
	"context"
	"fmt"
	"time"
	"ucode/ucode_go_function_service/config"
	"ucode/ucode_go_function_service/storage"

	"github.com/go-redis/redis/v8"
)

type Storage struct {
	cfg  config.Config
	pool *redis.Client
}

func NewRedis(cfg config.Config) storage.RedisStorageI {
	var redisPool *redis.Client

	redisPool = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.GetRequestRedisHost, cfg.GetRequestRedisPort),
		DB:       cfg.GetRequestRedisDatabase,
		Password: cfg.GetRequestRedisPassword,
	})

	conf := config.Load()

	return Storage{
		cfg:  conf,
		pool: redisPool,
	}
}

func (s Storage) SetX(ctx context.Context, key string, value string, duration time.Duration) error {
	return s.pool.SetEX(ctx, key, value, duration).Err()
}

func (s Storage) Get(ctx context.Context, key string) (string, error) {
	return s.pool.Get(ctx, key).Result()
}

func (s Storage) Del(ctx context.Context, keys string) error {
	return s.pool.Del(ctx, keys).Err()
}

func (s Storage) Set(ctx context.Context, key string, value any, duration time.Duration) error {

	return s.pool.Set(ctx, key, value, duration).Err()
}

func (s Storage) DelMany(ctx context.Context, keys []string) error {

	return s.pool.Del(ctx, keys...).Err()
}
