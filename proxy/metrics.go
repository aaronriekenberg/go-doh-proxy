package proxy

import (
	"fmt"
	"sync/atomic"
)

type metrics struct {
	nonAtomicCacheHits           uint64
	nonAtomicCacheMisses         uint64
	nonAtomicDOHClientErrors     uint64
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

func (metrics *metrics) incrementDOHClientErrors() {
	atomic.AddUint64(&metrics.nonAtomicDOHClientErrors, 1)
}

func (metrics *metrics) dohClientErrors() uint64 {
	return atomic.LoadUint64(&metrics.nonAtomicDOHClientErrors)
}

func (metrics *metrics) incrementWriteResponseErrors() {
	atomic.AddUint64(&metrics.nonAtomicWriteResponseErrors, 1)
}

func (metrics *metrics) writeResponseErrors() uint64 {
	return atomic.LoadUint64(&metrics.nonAtomicWriteResponseErrors)
}

func (metrics *metrics) String() string {
	return fmt.Sprintf("cacheHits = %v cacheMisses = %v dohClientErrors = %v writeResponseErrors = %v",
		metrics.cacheHits(), metrics.cacheMisses(), metrics.dohClientErrors(), metrics.writeResponseErrors())
}
