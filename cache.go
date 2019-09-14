package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/golang-lru"
	"github.com/miekg/dns"
)

func getQuestionCacheKey(m *dns.Msg) string {
	var builder strings.Builder

	for i, question := range m.Question {
		if i > 0 {
			builder.WriteByte('|')
		}
		fmt.Fprintf(&builder, "%s:%d:%d", strings.ToLower(question.Name), question.Qtype, question.Qclass)
	}

	return builder.String()
}

type cacheObject struct {
	cacheTime      time.Time
	expirationTime time.Time
	message        dns.Msg
}

func (co *cacheObject) Expired(now time.Time) bool {
	return now.After(co.expirationTime)
}

type cache struct {
	lruCache *lru.Cache
}

func newCache(maxCacheSize int) *cache {
	lruCache, err := lru.New(maxCacheSize)
	if err != nil {
		logger.Fatalf("error creating cache %v", err)
	}

	return &cache{
		lruCache: lruCache,
	}
}

func (cache *cache) Get(key string) (*cacheObject, bool) {
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

func (cache *cache) Add(key string, value *cacheObject) {
	cache.lruCache.Add(key, value)
}

func (cache *cache) Len() int {
	return cache.lruCache.Len()
}

func (cache *cache) PeriodicPurge(maxPurgeItems int) (itemsPurged int) {
	for itemsPurged < maxPurgeItems {
		key, value, ok := cache.lruCache.GetOldest()
		if !ok {
			break
		}

		cacheObject := value.(*cacheObject)

		if cacheObject.Expired(time.Now()) {
			cache.lruCache.Remove(key)
			itemsPurged++
		} else {
			break
		}
	}

	return
}
