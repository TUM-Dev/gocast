package tools

import (
	"github.com/dgraph-io/ristretto"
	"time"
)

var cache *ristretto.Cache

func initCache() {
	c, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e6,     // number of keys to track frequency of (1M).
		MaxCost:     1 << 29, // 1 << 30 == 1/2GB Cost of cache
		BufferItems: 64,
	})
	if err != nil {
		panic(err)
	}
	cache = c
}

// GetCacheItem returns the value of the key if it exists in the cache. (nil, err) otherwise
func GetCacheItem(key string) (interface{}, bool) {
	return cache.Get(key)
}

// SetCacheItem adds the key and value to the cache with the given expiration time.
func SetCacheItem(key string, value interface{}, ttl time.Duration) {
	cache.SetWithTTL(key, value, 1, ttl)
}
