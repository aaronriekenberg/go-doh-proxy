package proxy

import (
	"fmt"
	"sync/atomic"
)

type metrics struct {
	nonAtomicCacheHits           uint64
	nonAtomicCacheMisses         uint64
	nonAtomicClientErrors        uint64
	nonAtomicWriteResponseErrors uint64
}

func (metrics *metrics) incrementCacheHits() {
	atomic.AddUint64(&metrics.nonAtomicCacheHits, 1)
}

func (metrics *metrics) cacheHits() uint64 {
	return atomic.LoadUint64(&metrics.nonAtomicCacheHits)
}

func (metrics *metrics) incrementCacheMisses() {
	atomic.AddUint64(&metrics.nonAtomicCacheMisses, 1)
}

func (metrics *metrics) cacheMisses() uint64 {
	return atomic.LoadUint64(&metrics.nonAtomicCacheMisses)
}

func (metrics *metrics) incrementClientErrors() {
	atomic.AddUint64(&metrics.nonAtomicClientErrors, 1)
}

func (metrics *metrics) clientErrors() uint64 {
	return atomic.LoadUint64(&metrics.nonAtomicClientErrors)
}

func (metrics *metrics) incrementWriteResponseErrors() {
	atomic.AddUint64(&metrics.nonAtomicWriteResponseErrors, 1)
}

func (metrics *metrics) writeResponseErrors() uint64 {
	return atomic.LoadUint64(&metrics.nonAtomicWriteResponseErrors)
}

func (metrics *metrics) String() string {
	return fmt.Sprintf("cacheHits = %v cacheMisses = %v clientErrors = %v writeResponseErrors = %v",
		metrics.cacheHits(), metrics.cacheMisses(), metrics.clientErrors(), metrics.writeResponseErrors())
}
