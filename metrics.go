package main

import (
	"fmt"
	"sync/atomic"
)

type metrics struct {
	cacheHits      uint64
	cacheMisses    uint64
	clientErrors   uint64
	notCachedRcode uint64
	notCachedTTL   uint64
	notCachedKey   uint64
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

func (metrics *metrics) IncrementNotCachedRcode() {
	atomic.AddUint64(&metrics.notCachedRcode, 1)
}

func (metrics *metrics) NotCachedRcode() uint64 {
	return atomic.LoadUint64(&metrics.notCachedRcode)
}

func (metrics *metrics) IncrementNotCachedTTL() {
	atomic.AddUint64(&metrics.notCachedTTL, 1)
}

func (metrics *metrics) NotCachedTTL() uint64 {
	return atomic.LoadUint64(&metrics.notCachedTTL)
}

func (metrics *metrics) IncrementNotCachedKey() {
	atomic.AddUint64(&metrics.notCachedKey, 1)
}

func (metrics *metrics) NotCachedKey() uint64 {
	return atomic.LoadUint64(&metrics.notCachedKey)
}

func (metrics *metrics) String() string {
	return fmt.Sprintf("cacheHits = %v cacheMisses = %v clientErrors = %v notCachedRcode = %v notCachedTTL = %v notCachedKey = %v",
		metrics.CacheHits(), metrics.CacheMisses(), metrics.ClientErrors(), metrics.NotCachedRcode(), metrics.NotCachedTTL(), metrics.NotCachedKey())
}
