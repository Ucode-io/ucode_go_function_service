package storage

import (
	"context"
	"errors"
	"time"
)

var ErrorTheSameId = errors.New("cannot use the same uuid for 'id' and 'parent_id' fields")
var ErrorProjectId = errors.New("not valid 'project_id'")

type StorageI interface {
	CloseDB()
}

type RedisStorageI interface {
	SetX(ctx context.Context, key string, value string, duration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, key string) error
	Set(ctx context.Context, key string, value any, duration time.Duration) error
	DelMany(ctx context.Context, keys []string) error
}
