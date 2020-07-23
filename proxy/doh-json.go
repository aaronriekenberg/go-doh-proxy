package proxy

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/miekg/dns"
)

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
	metrics *metrics
}

func newDOHJSONConverter(metrics *metrics) *dohJSONConverter {
	return &dohJSONConverter{
		metrics: metrics,
	}
}

func (dohJSONConverter *dohJSONConverter) decodeJSONResponse(request *dns.Msg, jsonResponse []byte) (resp *dns.Msg, err error) {
	var dohJSONResponse dohJSONResponse

	err = json.Unmarshal(jsonResponse, &dohJSONResponse)
	if err != nil {
		err = fmt.Errorf("error decoding json response: %w", err)
		return
	}

	resp = new(dns.Msg)
	resp.SetReply(request)

	resp.Rcode = dohJSONResponse.Status
	dohJSONConverter.metrics.recordRcodeMetric(dohJSONResponse.Status)

	for i := range dohJSONResponse.Answer {
		answer := &(dohJSONResponse.Answer[i])
		rrType := uint16(answer.Type)

		dohJSONConverter.metrics.recordRRTypeMetric(dns.Type(answer.Type))

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
			log.Printf("unknown json rrType = %v request = %v", rrType, request)
		}
	}

	return
}
