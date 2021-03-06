package proxy

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/miekg/dns"
)

type metricValue struct {
	count uint64
}

func newMetricValue(count uint64) *metricValue {
	return &metricValue{
		count: count,
	}
}

func (metricValue *metricValue) incrementCount() {
	atomic.AddUint64(&(metricValue.count), 1)
}

func (metricValue *metricValue) loadCount() uint64 {
	return atomic.LoadUint64(&(metricValue.count))
}

type metrics struct {
	configuration            *MetricsConfiguration
	blockedValue             metricValue
	cacheHitsValue           metricValue
	cacheMissesValue         metricValue
	prefetchRequestsValue    metricValue
	dohClientErrorsValue     metricValue
	writeResponseErrorsValue metricValue
	rcodeMetricsMap          sync.Map
	rrTypeMetricsMap         sync.Map
}

func newMetrics(configuration *MetricsConfiguration) *metrics {
	return &metrics{
		configuration: configuration,
	}
}

func (metrics *metrics) incrementBlocked() {
	metrics.blockedValue.incrementCount()
}

func (metrics *metrics) blocked() uint64 {
	return metrics.blockedValue.loadCount()
}

func (metrics *metrics) incrementCacheHits() {
	metrics.cacheHitsValue.incrementCount()
}

func (metrics *metrics) cacheHits() uint64 {
	return metrics.cacheHitsValue.loadCount()
}

func (metrics *metrics) incrementCacheMisses() {
	metrics.cacheMissesValue.incrementCount()
}

func (metrics *metrics) cacheMisses() uint64 {
	return metrics.cacheMissesValue.loadCount()
}

func (metrics *metrics) incrementPrefetchRequests() {
	metrics.prefetchRequestsValue.incrementCount()
}

func (metrics *metrics) prefetchRequests() uint64 {
	return metrics.prefetchRequestsValue.loadCount()
}

func (metrics *metrics) incrementDOHClientErrors() {
	metrics.dohClientErrorsValue.incrementCount()
}

func (metrics *metrics) dohClientErrors() uint64 {
	return metrics.dohClientErrorsValue.loadCount()
}

func (metrics *metrics) incrementWriteResponseErrors() {
	metrics.writeResponseErrorsValue.incrementCount()
}

func (metrics *metrics) writeResponseErrors() uint64 {
	return metrics.writeResponseErrorsValue.loadCount()
}

func (metrics *metrics) recordRcodeMetric(rcode int) {

	value, loaded := metrics.rcodeMetricsMap.Load(rcode)

	if !loaded {
		value, loaded = metrics.rcodeMetricsMap.LoadOrStore(rcode, newMetricValue(1))
	}

	if loaded {
		value.(*metricValue).incrementCount()
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
		localMap[rcodeString] = rrMetricValue.loadCount()
		return true
	})

	return localMap
}

func (metrics *metrics) recordRRTypeMetric(rrType dns.Type) {

	value, loaded := metrics.rrTypeMetricsMap.Load(rrType)

	if !loaded {
		value, loaded = metrics.rrTypeMetricsMap.LoadOrStore(rrType, newMetricValue(1))
	}

	if loaded {
		value.(*metricValue).incrementCount()
	}
}

func (metrics *metrics) rrTypeMetricsMapSnapshot() map[dns.Type]uint64 {

	localMap := make(map[dns.Type]uint64)

	metrics.rrTypeMetricsMap.Range(func(key, value interface{}) bool {
		rrType := key.(dns.Type)
		rrMetricValue := value.(*metricValue)
		localMap[rrType] = rrMetricValue.loadCount()
		return true
	})

	return localMap
}

func (metrics *metrics) String() string {
	return fmt.Sprintf(
		"blocked = %v cacheHits = %v cacheMisses = %v prefetchRequests = %v dohClientErrors = %v writeResponseErrors = %v rcodeMetrics = %v rrtypeMetrics = %v",
		metrics.blocked(), metrics.cacheHits(), metrics.cacheMisses(), metrics.prefetchRequests(),
		metrics.dohClientErrors(), metrics.writeResponseErrors(),
		metrics.rcodeMetricsMapSnapshot(), metrics.rrTypeMetricsMapSnapshot())
}

func (metrics *metrics) runPeriodicTimer() {
	ticker := time.NewTicker(time.Duration(metrics.configuration.TimerIntervalSeconds) * time.Second)

	for {
		<-ticker.C

		log.Printf("metrics: %v", metrics)
	}
}

func (metrics *metrics) start() {
	log.Printf("metrics.start")

	go metrics.runPeriodicTimer()
}
