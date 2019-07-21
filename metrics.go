package main

import (
	"fmt"
	"sync/atomic"
)

type metrics struct {
	cacheHits    uint64
	cacheMisses  uint64
	clientErrors uint64
}

func (metrics *metrics) IncrementCacheHits() {
	atomic.AddUint64(&metrics.cacheHits, 1)
}

func (metrics *metrics) CacheHits() uint64 {
	return atomic.LoadUint64(&metrics.cacheHits)
}

func (metrics *metrics) IncrementCacheMisses() {
	atomic.AddUint64(&metrics.cacheMisses, 1)
}

func (metrics *metrics) CacheMisses() uint64 {
	return atomic.LoadUint64(&metrics.cacheMisses)
}

func (metrics *metrics) IncrementClientErrors() {
	atomic.AddUint64(&metrics.clientErrors, 1)
}

func (metrics *metrics) ClientErrors() uint64 {
	return atomic.LoadUint64(&metrics.clientErrors)
}

func (metrics *metrics) String() string {
	return fmt.Sprintf("cacheHits = %v cacheMisses = %v clientErrors = %v", metrics.CacheHits(), metrics.CacheMisses(), metrics.ClientErrors())
}
