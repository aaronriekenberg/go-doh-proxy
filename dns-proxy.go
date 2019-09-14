package main

import (
	"context"
	"log"
	"math"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

type dnsProxy struct {
	configuration           *configuration
	forwardNamesToAddresses map[string]net.IP
	reverseAddressesToNames map[string]string
	dohClient               *dohClient
	cache                   *cache
	metrics                 metrics
}

func newDNSProxy(configuration *configuration) *dnsProxy {

	forwardNamesToAddresses := make(map[string]net.IP)
	for _, forwardNameToAddress := range configuration.ForwardNamesToAddresses {
		forwardNamesToAddresses[strings.ToLower(forwardNameToAddress.Name)] = net.ParseIP(forwardNameToAddress.IPAddress)
	}

	reverseAddressesToNames := make(map[string]string)
	for _, reverseAddressToName := range configuration.ReverseAddressesToNames {
		reverseAddressesToNames[strings.ToLower(reverseAddressToName.ReverseAddress)] = reverseAddressToName.Name
	}

	return &dnsProxy{
		configuration:           configuration,
		forwardNamesToAddresses: forwardNamesToAddresses,
		reverseAddressesToNames: reverseAddressesToNames,
		dohClient:               newDOHClient(configuration.RemoteHTTPURL),
		cache:                   newCache(configuration.MaxCacheSize),
	}
}

func (dnsProxy *dnsProxy) clampAndGetMinTTLSeconds(m *dns.Msg) uint32 {
	foundRRHeaderTTL := false
	minTTLSeconds := dnsProxy.configuration.MinTTLSeconds

	processRRHeader := func(rrHeader *dns.RR_Header) {
		ttl := rrHeader.Ttl
		if ttl < dnsProxy.configuration.MinTTLSeconds {
			ttl = dnsProxy.configuration.MinTTLSeconds
		}
		if ttl > dnsProxy.configuration.MaxTTLSeconds {
			ttl = dnsProxy.configuration.MaxTTLSeconds
		}
		if (!foundRRHeaderTTL) || (ttl < minTTLSeconds) {
			minTTLSeconds = ttl
			foundRRHeaderTTL = true
		}
		rrHeader.Ttl = ttl
	}

	for _, rr := range m.Answer {
		processRRHeader(rr.Header())
	}
	for _, rr := range m.Ns {
		processRRHeader(rr.Header())
	}
	for _, rr := range m.Extra {
		rrHeader := rr.Header()
		if rrHeader.Rrtype != dns.TypeOPT {
			processRRHeader(rrHeader)
		}
	}

	return minTTLSeconds
}

func (dnsProxy *dnsProxy) copyCachedMessageForHit(uncopiedCacheObject *cacheObject) *dns.Msg {

	now := time.Now()

	if uncopiedCacheObject.Expired(now) {
		return nil
	}

	secondsToSubtractFromTTL := int64(now.Sub(uncopiedCacheObject.cacheTime).Seconds())
	if secondsToSubtractFromTTL < 0 {
		return nil
	}

	ok := true

	adjustRRHeaderTTL := func(rrHeader *dns.RR_Header) {
		ttl := int64(rrHeader.Ttl) - secondsToSubtractFromTTL
		if (ttl < 0) || (ttl > math.MaxUint32) {
			ok = false
		} else {
			rrHeader.Ttl = uint32(ttl)
		}
	}

	messageCopy := uncopiedCacheObject.message.Copy()

	for _, rr := range messageCopy.Answer {
		adjustRRHeaderTTL(rr.Header())
	}
	for _, rr := range messageCopy.Ns {
		adjustRRHeaderTTL(rr.Header())
	}
	for _, rr := range messageCopy.Extra {
		rrHeader := rr.Header()
		if rrHeader.Rrtype != dns.TypeOPT {
			adjustRRHeaderTTL(rrHeader)
		}
	}

	if !ok {
		return nil
	}

	return messageCopy
}

func (dnsProxy *dnsProxy) clampTTLAndCacheResponse(resp *dns.Msg) {
	if !((resp.Rcode == dns.RcodeSuccess) || (resp.Rcode == dns.RcodeNameError)) {
		return
	}

	minTTLSeconds := dnsProxy.clampAndGetMinTTLSeconds(resp)
	if minTTLSeconds <= 0 {
		return
	}

	respQuestionCacheKey := getQuestionCacheKey(resp)
	if len(respQuestionCacheKey) == 0 {
		return
	}

	ttlDuration := time.Second * time.Duration(minTTLSeconds)
	now := time.Now()
	expirationTime := now.Add(ttlDuration)

	cacheObject := &cacheObject{
		cacheTime:      now,
		expirationTime: expirationTime,
	}
	resp.CopyTo(&cacheObject.message)

	dnsProxy.cache.Add(respQuestionCacheKey, cacheObject)
}

func (dnsProxy *dnsProxy) writeResponse(w dns.ResponseWriter, r *dns.Msg) {
	if err := w.WriteMsg(r); err != nil {
		dnsProxy.metrics.IncrementWriteResponseErrors()
		log.Printf("writeResponse error = %v", err)
	}
}

func (dnsProxy *dnsProxy) createProxyHandlerFunc() dns.HandlerFunc {

	return func(w dns.ResponseWriter, r *dns.Msg) {

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		requestID := r.Id

		co, ok := dnsProxy.cache.Get(getQuestionCacheKey(r))
		if ok {
			if cacheMessageCopy := dnsProxy.copyCachedMessageForHit(co); cacheMessageCopy != nil {
				dnsProxy.metrics.IncrementCacheHits()
				cacheMessageCopy.Id = requestID
				dnsProxy.writeResponse(w, cacheMessageCopy)
				return
			}
		}

		dnsProxy.metrics.IncrementCacheMisses()
		r.Id = 0
		responseMsg, err := dnsProxy.dohClient.MakeHTTPRequest(ctx, r)
		if err != nil {
			dnsProxy.metrics.IncrementClientErrors()
			log.Printf("makeHttpRequest error %v", err)
			r.Id = requestID
			dns.HandleFailed(w, r)
			return
		}

		dnsProxy.clampTTLAndCacheResponse(responseMsg)
		responseMsg.Id = requestID
		dnsProxy.writeResponse(w, responseMsg)
	}
}

func (dnsProxy *dnsProxy) createForwardDomainHandlerFunc() dns.HandlerFunc {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		if len(r.Question) > 0 {
			question := &(r.Question[0])
			if question.Qtype == dns.TypeA {
				msg := new(dns.Msg)
				address, ok := dnsProxy.forwardNamesToAddresses[strings.ToLower(question.Name)]
				if !ok {
					msg.SetRcode(r, dns.RcodeNameError)
					msg.Authoritative = true
				} else {
					msg.SetReply(r)
					msg.Authoritative = true
					msg.Answer = append(msg.Answer, &dns.A{
						Hdr: dns.RR_Header{Name: question.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
						A:   address,
					})
				}
				dnsProxy.writeResponse(w, msg)
				return
			}
		}
		dns.HandleFailed(w, r)
	}
}

func (dnsProxy *dnsProxy) createReverseHandlerFunc() dns.HandlerFunc {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		if len(r.Question) > 0 {
			question := &(r.Question[0])
			if question.Qtype == dns.TypePTR {
				msg := new(dns.Msg)
				name, ok := dnsProxy.reverseAddressesToNames[strings.ToLower(question.Name)]
				if !ok {
					msg.SetRcode(r, dns.RcodeNameError)
					msg.Authoritative = true
				} else {
					msg.SetReply(r)
					msg.Authoritative = true
					msg.Answer = append(msg.Answer, &dns.PTR{
						Hdr: dns.RR_Header{Name: question.Name, Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: 60},
						Ptr: name,
					})
				}
				dnsProxy.writeResponse(w, msg)
				return
			}
		}
		dns.HandleFailed(w, r)
	}
}

func (dnsProxy *dnsProxy) createServeMux() *dns.ServeMux {

	dnsServeMux := dns.NewServeMux()

	dnsServeMux.HandleFunc(".", dnsProxy.createProxyHandlerFunc())

	dnsServeMux.HandleFunc(dnsProxy.configuration.ForwardDomain, dnsProxy.createForwardDomainHandlerFunc())

	dnsServeMux.HandleFunc(dnsProxy.configuration.ReverseDomain, dnsProxy.createReverseHandlerFunc())

	return dnsServeMux
}

func (dnsProxy *dnsProxy) runServer(listenAddrAndPort, net string, serveMux *dns.ServeMux) {
	srv := &dns.Server{
		Handler: serveMux,
		Addr:    listenAddrAndPort,
		Net:     net,
	}

	log.Printf("starting %v server on %v", net, listenAddrAndPort)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("ListenAndServe error for net %s: %v", net, err)
	}
}

func (dnsProxy *dnsProxy) runPeriodicTimer() {
	ticker := time.NewTicker(time.Second * time.Duration(dnsProxy.configuration.TimerIntervalSeconds))

	for {
		select {
		case <-ticker.C:
			itemsPurged := dnsProxy.cache.PeriodicPurge(dnsProxy.configuration.MaxPurgesPerTimerPop)

			log.Printf("timerPop metrics: %v cache.Len = %v itemsPurged = %v",
				dnsProxy.metrics.String(), dnsProxy.cache.Len(), itemsPurged)
		}
	}
}

func (dnsProxy *dnsProxy) Start() {
	listenAddressAndPort := dnsProxy.configuration.ListenAddress.JoinHostPort()

	serveMux := dnsProxy.createServeMux()

	go dnsProxy.runServer(listenAddressAndPort, "tcp", serveMux)
	go dnsProxy.runServer(listenAddressAndPort, "udp", serveMux)

	go dnsProxy.runPeriodicTimer()
}
