package proxy

import (
	"fmt"
	"log"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/miekg/dns"
)

func getCacheKey(question *dns.Question) string {
	return fmt.Sprintf("%s:%d", dns.CanonicalName(question.Name), question.Qtype)
}

type cacheObject struct {
	cacheTime      time.Time
	expirationTime time.Time
	message        dns.Msg
}

func (co *cacheObject) expired(now time.Time) bool {
	return now.After(co.expirationTime)
}

func (co *cacheObject) durationInCache(now time.Time) time.Duration {
	return now.Sub(co.cacheTime)
}

type cache struct {
	lruCache *lru.Cache
}

func newCache(maxCacheSize int) *cache {
	lruCache, err := lru.New(maxCacheSize)
	if err != nil {
		log.Fatalf("error creating cache %v", err)
	}

	return &cache{
		lruCache: lruCache,
	}
}

func (cache *cache) get(key string) (*cacheObject, bool) {
	value, ok := cache.lruCache.Get(key)
	if !ok {
		return nil, false
	}

	cacheObject, ok := value.(*cacheObject)
	if !ok {
		return nil, false
	}

	return cacheObject, true
}

func (cache *cache) add(key string, value *cacheObject) {
	cache.lruCache.Add(key, value)
}

func (cache *cache) len() int {
	return cache.lruCache.Len()
}

func (cache *cache) periodicPurge(maxPurgeItems int) (itemsPurged int) {
	for itemsPurged < maxPurgeItems {
		key, value, ok := cache.lruCache.GetOldest()
		if !ok {
			break
		}

		cacheObject := value.(*cacheObject)

		if cacheObject.expired(time.Now()) {
			cache.lruCache.Remove(key)
			itemsPurged++
		} else {
			break
		}
	}

	return
}
