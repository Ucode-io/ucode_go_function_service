package caching

import (
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
)

type ExpiringLRUCache struct {
	cache *lru.Cache
	mutex sync.Mutex
}

type cacheEntry struct {
	value      []byte
	expiration time.Time
}

type cache struct {
	value      bool
	expiration time.Time
}

func NewExpiringLRUCache(size int) (*ExpiringLRUCache, error) {
	baseCache, err := lru.New(size)
	if err != nil {
		return nil, err
	}
	return &ExpiringLRUCache{cache: baseCache}, nil
}

func (c *ExpiringLRUCache) Add(key any, value []byte, expiration time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	entry := cacheEntry{
		value:      value,
		expiration: time.Now().Add(expiration),
	}

	c.cache.Add(key, entry)
}

func (c *ExpiringLRUCache) Get(key any) ([]byte, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	entry, ok := c.cache.Get(key)
	if !ok {
		return nil, false
	}

	cacheEntry := entry.(cacheEntry)
	if cacheEntry.expiration.Before(time.Now()) {
		c.cache.Remove(key)
		return nil, false
	}

	return cacheEntry.value, true
}

func (c *ExpiringLRUCache) AddKey(key any, value bool, expiration time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	entry := cache{
		value:      value,
		expiration: time.Now().Add(expiration),
	}

	c.cache.Add(key, entry)
}

func (c *ExpiringLRUCache) GetValue(key any) (bool, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	entry, ok := c.cache.Get(key)
	if !ok {
		return false, false
	}

	cacheEntry := entry.(cache)
	if cacheEntry.expiration.Before(time.Now()) {
		c.cache.Remove(key)
		return false, false
	}

	return cacheEntry.value, true
}
