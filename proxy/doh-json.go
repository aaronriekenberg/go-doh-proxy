package proxy

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/miekg/dns"
)

type metricValue struct {
	count uint64
}

type dohJSONResponseAnswer struct {
	Name string `json:"name"`
	Type int    `json:"type"`
	TTL  int    `json:"TTL"`
	Data string `json:"data"`
}

type dohJSONResponse struct {
	Status int                     `json:"Status"`
	Answer []dohJSONResponseAnswer `json:"Answer"`
}

type dohJSONConverter struct {
	rcodeMetricsMap  sync.Map
	rrTypeMetricsMap sync.Map
}

func newDOHJSONConverter() *dohJSONConverter {
	return &dohJSONConverter{}
}

func (dohJSONConverter *dohJSONConverter) recordRcodeMetric(rcode int) {

	value, loaded := dohJSONConverter.rcodeMetricsMap.Load(rcode)

	if !loaded {
		value, loaded = dohJSONConverter.rcodeMetricsMap.LoadOrStore(rcode, &metricValue{
			count: 1,
		})
	}

	if loaded {
		rrMetricValue := value.(*metricValue)
		atomic.AddUint64(&(rrMetricValue.count), 1)
	}
}

func (dohJSONConverter *dohJSONConverter) rcodeMetricsMapSnapshot() map[string]uint64 {

	localMap := make(map[string]uint64)

	dohJSONConverter.rcodeMetricsMap.Range(func(key, value interface{}) bool {
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

func (dohJSONConverter *dohJSONConverter) recordRRTypeMetric(rrType dns.Type) {

	value, loaded := dohJSONConverter.rrTypeMetricsMap.Load(rrType)

	if !loaded {
		value, loaded = dohJSONConverter.rrTypeMetricsMap.LoadOrStore(rrType, &metricValue{
			count: 1,
		})
	}

	if loaded {
		rrMetricValue := value.(*metricValue)
		atomic.AddUint64(&(rrMetricValue.count), 1)
	}
}

func (dohJSONConverter *dohJSONConverter) rrTypeMetricsMapSnapshot() map[dns.Type]uint64 {

	localMap := make(map[dns.Type]uint64)

	dohJSONConverter.rrTypeMetricsMap.Range(func(key, value interface{}) bool {
		rrType := key.(dns.Type)
		rrMetricValue := value.(*metricValue)
		localMap[rrType] = atomic.LoadUint64(&rrMetricValue.count)
		return true
	})

	return localMap
}

func (dohJSONConverter *dohJSONConverter) decodeJSONResponse(request *dns.Msg, jsonResponse []byte) (resp *dns.Msg, err error) {
	var dohJSONResponse dohJSONResponse

	err = json.Unmarshal(jsonResponse, &dohJSONResponse)
	if err != nil {
		log.Printf("error decoding json response %v", err)
		return
	}

	resp = new(dns.Msg)
	resp.SetReply(request)

	resp.Rcode = dohJSONResponse.Status
	dohJSONConverter.recordRcodeMetric(dohJSONResponse.Status)

	for i := range dohJSONResponse.Answer {
		answer := &(dohJSONResponse.Answer[i])
		rrType := uint16(answer.Type)

		dohJSONConverter.recordRRTypeMetric(dns.Type(answer.Type))

		createRRHeader := func() dns.RR_Header {
			return dns.RR_Header{
				Name:   dns.Fqdn(answer.Name),
				Rrtype: rrType,
				Class:  dns.ClassINET,
				Ttl:    uint32(answer.TTL),
			}
		}

		switch rrType {
		case dns.TypeA:
			resp.Answer = append(resp.Answer, &dns.A{
				Hdr: createRRHeader(),
				A:   net.ParseIP(answer.Data),
			})

		case dns.TypeAAAA:
			resp.Answer = append(resp.Answer, &dns.AAAA{
				Hdr:  createRRHeader(),
				AAAA: net.ParseIP(answer.Data),
			})

		case dns.TypeCNAME:
			resp.Answer = append(resp.Answer, &dns.CNAME{
				Hdr:    createRRHeader(),
				Target: dns.Fqdn(answer.Data),
			})

		case dns.TypePTR:
			resp.Answer = append(resp.Answer, &dns.PTR{
				Hdr: createRRHeader(),
				Ptr: dns.Fqdn(answer.Data),
			})

		case dns.TypeTXT:
			resp.Answer = append(resp.Answer, &dns.TXT{
				Hdr: createRRHeader(),
				// Trim leading and trailing \" from Data
				Txt: []string{strings.Trim(answer.Data, "\"")},
			})

		default:
			log.Printf("unknown json rrType request = %+v rrType = %v", request, rrType)
		}
	}

	return
}
