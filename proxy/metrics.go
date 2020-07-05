package proxy

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/miekg/dns"
)

type metricValue struct {
	count uint64
}

type metrics struct {
	nonAtomicCacheHits           uint64
	nonAtomicCacheMisses         uint64
	nonAtomicDOHClientErrors     uint64
	nonAtomicWriteResponseErrors uint64
	rcodeMetricsMap              sync.Map
	rrTypeMetricsMap             sync.Map
}

func newMetrics() *metrics {
	return &metrics{}
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

func (metrics *metrics) recordRcodeMetric(rcode int) {

	value, loaded := metrics.rcodeMetricsMap.Load(rcode)

	if !loaded {
		value, loaded = metrics.rcodeMetricsMap.LoadOrStore(rcode, &metricValue{
			count: 1,
		})
	}

	if loaded {
		rrMetricValue := value.(*metricValue)
		atomic.AddUint64(&(rrMetricValue.count), 1)
	}
}

func (metrics *metrics) rcodeMetricsMapSnapshot() map[string]uint64 {

	localMap := make(map[string]uint64)

	metrics.rcodeMetricsMap.Range(func(key, value interface{}) bool {
		rcode := key.(int)
		rcodeString, ok := dns.RcodeToString[rcode]
		if !ok {
			rcodeString = fmt.Sprintf("UNKNOWN:%v", rcode)
		}
		rrMetricValue := value.(*metricValue)
		localMap[rcodeString] = atomic.LoadUint64(&rrMetricValue.count)
		return true
	})

	return localMap
}

func (metrics *metrics) recordRRTypeMetric(rrType dns.Type) {

	value, loaded := metrics.rrTypeMetricsMap.Load(rrType)

	if !loaded {
		value, loaded = metrics.rrTypeMetricsMap.LoadOrStore(rrType, &metricValue{
			count: 1,
		})
	}

	if loaded {
		rrMetricValue := value.(*metricValue)
		atomic.AddUint64(&(rrMetricValue.count), 1)
	}
}

func (metrics *metrics) rrTypeMetricsMapSnapshot() map[dns.Type]uint64 {

	localMap := make(map[dns.Type]uint64)

	metrics.rrTypeMetricsMap.Range(func(key, value interface{}) bool {
		rrType := key.(dns.Type)
		rrMetricValue := value.(*metricValue)
		localMap[rrType] = atomic.LoadUint64(&rrMetricValue.count)
		return true
	})

	return localMap
}

func (metrics *metrics) String() string {
	return fmt.Sprintf("cacheHits = %v cacheMisses = %v dohClientErrors = %v writeResponseErrors = %v rcodeMetrics = %v rrtypeMetrics = %v",
		metrics.cacheHits(), metrics.cacheMisses(), metrics.dohClientErrors(), metrics.writeResponseErrors(),
		metrics.rcodeMetricsMapSnapshot(), metrics.rrTypeMetricsMapSnapshot())
}
