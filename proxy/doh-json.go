package proxy

import (
	"encoding/json"
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

func decodeJSONResponse(request *dns.Msg, jsonResponse []byte) (resp *dns.Msg, err error) {
	var dohJSONResponse dohJSONResponse

	err = json.Unmarshal(jsonResponse, &dohJSONResponse)
	if err != nil {
		log.Printf("error decoding json response %v", err)
		return
	}

	resp = new(dns.Msg)
	resp.SetReply(request)

	resp.Rcode = dohJSONResponse.Status

	for i := range dohJSONResponse.Answer {
		answer := &(dohJSONResponse.Answer[i])

		rrType := uint16(answer.Type)
		switch rrType {
		case dns.TypeA:
			resp.Answer = append(resp.Answer, &dns.A{
				Hdr: dns.RR_Header{
					Name:   dns.Fqdn(answer.Name),
					Rrtype: rrType,
					Class:  dns.ClassINET,
					Ttl:    uint32(answer.TTL),
				},
				A: net.ParseIP(answer.Data),
			})

		case dns.TypeAAAA:
			resp.Answer = append(resp.Answer, &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:   dns.Fqdn(answer.Name),
					Rrtype: rrType,
					Class:  dns.ClassINET,
					Ttl:    uint32(answer.TTL),
				},
				AAAA: net.ParseIP(answer.Data),
			})

		case dns.TypeCNAME:
			resp.Answer = append(resp.Answer, &dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   dns.Fqdn(answer.Name),
					Rrtype: rrType,
					Class:  dns.ClassINET,
					Ttl:    uint32(answer.TTL),
				},
				Target: dns.Fqdn(answer.Data),
			})

		case dns.TypePTR:
			resp.Answer = append(resp.Answer, &dns.PTR{
				Hdr: dns.RR_Header{
					Name:   dns.Fqdn(answer.Name),
					Rrtype: rrType,
					Class:  dns.ClassINET,
					Ttl:    uint32(answer.TTL),
				},
				Ptr: dns.Fqdn(answer.Data),
			})

		case dns.TypeTXT:
			resp.Answer = append(resp.Answer, &dns.TXT{
				Hdr: dns.RR_Header{
					Name:   dns.Fqdn(answer.Name),
					Rrtype: rrType,
					Class:  dns.ClassINET,
					Ttl:    uint32(answer.TTL),
				},
				// Trim leading and trailing \" from Data
				Txt: []string{strings.Trim(answer.Data, "\"")},
			})

		default:
			log.Printf("unknown json rrType request = %+v rrType = %v", request, rrType)
		}
	}

	return
}
