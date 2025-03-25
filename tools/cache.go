package tools

import (
	"time"

	"github.com/dgraph-io/ristretto/v2"
)

var cache *ristretto.Cache[string, any]

func initCache() {
	c, err := ristretto.NewCache[string, any](&ristretto.Config[string, any]{
		NumCounters: 1e6,
		MaxCost:     1 << 29,
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
