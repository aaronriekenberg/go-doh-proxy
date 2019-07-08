package main

import (
	"fmt"
	"sync/atomic"
)

type Metrics struct {
	cacheHits      uint64
	cacheMisses    uint64
	clientErrors   uint64
	notCachedRcode uint64
	notCachedTTL   uint64
	notCachedKey   uint64
}

func (metrics *Metrics) IncrementCacheHits() {
	atomic.AddUint64(&metrics.cacheHits, 1)
}

func (metrics *Metrics) CacheHits() uint64 {
	return atomic.LoadUint64(&metrics.cacheHits)
}

func (metrics *Metrics) IncrementCacheMisses() {
	atomic.AddUint64(&metrics.cacheMisses, 1)
}

func (metrics *Metrics) CacheMisses() uint64 {
	return atomic.LoadUint64(&metrics.cacheMisses)
}

func (metrics *Metrics) IncrementClientErrors() {
	atomic.AddUint64(&metrics.clientErrors, 1)
}

func (metrics *Metrics) ClientErrors() uint64 {
	return atomic.LoadUint64(&metrics.clientErrors)
}

func (metrics *Metrics) IncrementNotCachedRcode() {
	atomic.AddUint64(&metrics.notCachedRcode, 1)
}

func (metrics *Metrics) NotCachedRcode() uint64 {
	return atomic.LoadUint64(&metrics.notCachedRcode)
}

func (metrics *Metrics) IncrementNotCachedTTL() {
	atomic.AddUint64(&metrics.notCachedTTL, 1)
}

func (metrics *Metrics) NotCachedTTL() uint64 {
	return atomic.LoadUint64(&metrics.notCachedTTL)
}

func (metrics *Metrics) IncrementNotCachedKey() {
	atomic.AddUint64(&metrics.notCachedKey, 1)
}

func (metrics *Metrics) NotCachedKey() uint64 {
	return atomic.LoadUint64(&metrics.notCachedKey)
}

func (metrics *Metrics) String() string {
	return fmt.Sprintf("cacheHits = %v cacheMisses = %v clientErrors = %v notCachedRcode = %v notCachedTTL = %v notCachedKey = %v",
		metrics.CacheHits(), metrics.CacheMisses(), metrics.ClientErrors(), metrics.NotCachedRcode(), metrics.NotCachedTTL(), metrics.NotCachedKey())
}
