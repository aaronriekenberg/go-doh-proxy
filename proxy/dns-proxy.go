package proxy

import (
	"context"
	"log"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// DNSProxy is the DNS proxy.
type DNSProxy interface {
	Start()
}

type dnsProxy struct {
	configuration *Configuration
	metrics       *metrics
	dohClient     *dohClient
	cache         *cache
	prefetch      *prefetch
}

// NewDNSProxy creates a DNS proxy.
func NewDNSProxy(configuration *Configuration) DNSProxy {
	metrics := newMetrics()

	return &dnsProxy{
		configuration: configuration,
		metrics:       metrics,
		dohClient:     newDOHClient(configuration.DOHClientConfiguration, newDOHJSONConverter(metrics)),
		cache:         newCache(configuration.CacheConfiguration.MaxSize),
		prefetch:      newPrefetch(),
	}
}

func (dnsProxy *dnsProxy) clampAndGetMinTTLSeconds(m *dns.Msg) uint32 {
	clampMinTTLSeconds := dnsProxy.configuration.ProxyConfiguration.ClampMinTTLSeconds
	clampMaxTTLSeconds := dnsProxy.configuration.ProxyConfiguration.ClampMaxTTLSeconds

	foundRRHeaderTTL := false
	rrHeaderMinTTLSeconds := clampMinTTLSeconds

	processRRHeader := func(rrHeader *dns.RR_Header) {
		ttl := rrHeader.Ttl
		if ttl < clampMinTTLSeconds {
			ttl = clampMinTTLSeconds
		}
		if ttl > clampMaxTTLSeconds {
			ttl = clampMaxTTLSeconds
		}
		if (!foundRRHeaderTTL) || (ttl < rrHeaderMinTTLSeconds) {
			rrHeaderMinTTLSeconds = ttl
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

	return rrHeaderMinTTLSeconds
}

func (dnsProxy *dnsProxy) getCachedMessageCopyForHit(cacheKey string) *dns.Msg {

	uncopiedCacheObject, ok := dnsProxy.cache.get(cacheKey)
	if !ok {
		return nil
	}

	now := time.Now()

	if uncopiedCacheObject.expired(now) {
		return nil
	}

	secondsToSubtractFromTTL := uint64(uncopiedCacheObject.durationInCache(now) / time.Second)

	ok = true

	adjustRRHeaderTTL := func(rrHeader *dns.RR_Header) {
		originalTTL := uint64(rrHeader.Ttl)
		if secondsToSubtractFromTTL > originalTTL {
			ok = false
		} else {
			newTTL := originalTTL - secondsToSubtractFromTTL
			rrHeader.Ttl = uint32(newTTL)
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

func (dnsProxy *dnsProxy) clampTTLAndCacheResponse(cacheKey string, resp *dns.Msg) {
	if !((resp.Rcode == dns.RcodeSuccess) || (resp.Rcode == dns.RcodeNameError)) {
		return
	}

	minTTLSeconds := dnsProxy.clampAndGetMinTTLSeconds(resp)
	if minTTLSeconds <= 0 {
		return
	}

	if len(cacheKey) == 0 {
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
	cacheObject.message.Id = 0

	dnsProxy.cache.add(cacheKey, cacheObject)
}

func (dnsProxy *dnsProxy) addToPrefetch(cacheKey string, question *dns.Question, response *dns.Msg) {
	log.Printf("addToPrefetch rcode = %v", response.Rcode)

	if response.Rcode != dns.RcodeSuccess {
		return
	}

	dnsProxy.prefetch.addToPrefetch(cacheKey, question)
}

func (dnsProxy *dnsProxy) writeResponse(w dns.ResponseWriter, response *dns.Msg) {
	if err := w.WriteMsg(response); err != nil {
		dnsProxy.metrics.incrementWriteResponseErrors()
		log.Printf("writeResponse error = %v", err)
	}
}

func (dnsProxy *dnsProxy) makePrefetchRequest(cacheKey string, question *dns.Question) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Printf("makePrefetchRequest %v", cacheKey)

	dnsProxy.metrics.incrementPrefetchRequests()

	request := new(dns.Msg)
	request.Question = append(request.Question, *question)

	responseMsg, err := dnsProxy.dohClient.makeRequest(ctx, request)
	if err != nil {
		dnsProxy.metrics.incrementDOHClientErrors()
		log.Printf("makeHttpRequest error: %v", err)
		return
	}

	dnsProxy.clampTTLAndCacheResponse(cacheKey, responseMsg)
}

func (dnsProxy *dnsProxy) createProxyHandlerFunc() dns.HandlerFunc {

	return func(w dns.ResponseWriter, request *dns.Msg) {

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if len(request.Question) != 1 {
			log.Printf("bad request.Question length %v request %v", len(request.Question), request)
			dns.HandleFailed(w, request)
			return
		}

		requestID := request.Id
		question := &(request.Question[0])
		cacheKey := getCacheKey(question)

		if cacheMessageCopy := dnsProxy.getCachedMessageCopyForHit(cacheKey); cacheMessageCopy != nil {
			dnsProxy.metrics.incrementCacheHits()
			cacheMessageCopy.Id = requestID
			dnsProxy.writeResponse(w, cacheMessageCopy)
			return
		}

		dnsProxy.metrics.incrementCacheMisses()
		request.Id = 0
		responseMsg, err := dnsProxy.dohClient.makeRequest(ctx, request)
		if err != nil {
			dnsProxy.metrics.incrementDOHClientErrors()
			log.Printf("makeHttpRequest error: %v", err)
			request.Id = requestID
			dns.HandleFailed(w, request)
			return
		}

		dnsProxy.addToPrefetch(cacheKey, question, responseMsg)

		dnsProxy.clampTTLAndCacheResponse(cacheKey, responseMsg)

		responseMsg.Id = requestID
		dnsProxy.writeResponse(w, responseMsg)
	}
}

func (dnsProxy *dnsProxy) createForwardDomainHandlerFunc(forwardDomainConfiguration ForwardDomainConfiguration) dns.HandlerFunc {
	forwardNamesToAddresses := make(map[string]net.IP)
	for _, forwardNameToAddress := range forwardDomainConfiguration.NamesToAddresses {
		forwardNamesToAddresses[strings.ToLower(forwardNameToAddress.Name)] = net.ParseIP(forwardNameToAddress.IPAddress)
	}

	return func(w dns.ResponseWriter, r *dns.Msg) {
		if len(r.Question) == 0 {
			dns.HandleFailed(w, r)
			return
		}

		question := &(r.Question[0])
		responseMsg := new(dns.Msg)
		if question.Qtype != dns.TypeA {
			responseMsg.SetRcode(r, dns.RcodeNameError)
			responseMsg.Authoritative = true
			dnsProxy.writeResponse(w, responseMsg)
			return
		}

		address, ok := forwardNamesToAddresses[strings.ToLower(question.Name)]
		if !ok {
			responseMsg.SetRcode(r, dns.RcodeNameError)
			responseMsg.Authoritative = true
			dnsProxy.writeResponse(w, responseMsg)
			return
		}

		responseMsg.SetReply(r)
		responseMsg.Authoritative = true
		responseMsg.Answer = append(responseMsg.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   question.Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    forwardDomainConfiguration.ResponseTTLSeconds,
			},
			A: address,
		})
		dnsProxy.writeResponse(w, responseMsg)
	}
}

func (dnsProxy *dnsProxy) createReverseHandlerFunc(reverseDomainConfiguration ReverseDomainConfiguration) dns.HandlerFunc {
	reverseAddressesToNames := make(map[string]string)
	for _, reverseAddressToName := range reverseDomainConfiguration.AddressesToNames {
		reverseAddressesToNames[strings.ToLower(reverseAddressToName.ReverseAddress)] = reverseAddressToName.Name
	}

	return func(w dns.ResponseWriter, r *dns.Msg) {
		if len(r.Question) == 0 {
			dns.HandleFailed(w, r)
			return
		}

		question := &(r.Question[0])
		responseMsg := new(dns.Msg)
		if question.Qtype != dns.TypePTR {
			responseMsg.SetRcode(r, dns.RcodeNameError)
			responseMsg.Authoritative = true
			dnsProxy.writeResponse(w, responseMsg)
			return
		}

		name, ok := reverseAddressesToNames[strings.ToLower(question.Name)]
		if !ok {
			responseMsg.SetRcode(r, dns.RcodeNameError)
			responseMsg.Authoritative = true
			dnsProxy.writeResponse(w, responseMsg)
			return
		}

		responseMsg.SetReply(r)
		responseMsg.Authoritative = true
		responseMsg.Answer = append(responseMsg.Answer, &dns.PTR{
			Hdr: dns.RR_Header{
				Name:   question.Name,
				Rrtype: dns.TypePTR,
				Class:  dns.ClassINET,
				Ttl:    reverseDomainConfiguration.ResponseTTLSeconds,
			},
			Ptr: name,
		})
		dnsProxy.writeResponse(w, responseMsg)
	}

}

func (dnsProxy *dnsProxy) createServeMux() *dns.ServeMux {

	dnsServeMux := dns.NewServeMux()

	dnsServeMux.HandleFunc(".", dnsProxy.createProxyHandlerFunc())

	for _, forwardDomainConfiguration := range dnsProxy.configuration.ForwardDomainConfigurations {
		dnsServeMux.HandleFunc(forwardDomainConfiguration.Domain, dnsProxy.createForwardDomainHandlerFunc(forwardDomainConfiguration))
	}

	for _, reverseDomainConfiguration := range dnsProxy.configuration.ReverseDomainConfigurations {
		dnsServeMux.HandleFunc(reverseDomainConfiguration.Domain, dnsProxy.createReverseHandlerFunc(reverseDomainConfiguration))
	}

	return dnsServeMux
}

func (dnsProxy *dnsProxy) runServer(listenAddrAndPort, net string, serveMux *dns.ServeMux) {
	srv := &dns.Server{
		Handler: serveMux,
		Addr:    listenAddrAndPort,
		Net:     net,
	}

	log.Printf("starting %v server on %v", net, listenAddrAndPort)

	err := srv.ListenAndServe()
	log.Fatalf("ListenAndServe error for net %s: %v", net, err)
}

func (dnsProxy *dnsProxy) runPeriodicTimer() {
	ticker := time.NewTicker(time.Second * time.Duration(dnsProxy.configuration.TimerIntervalSeconds))

	for {
		<-ticker.C

		cacheItemsPurged := dnsProxy.cache.periodicPurge(dnsProxy.configuration.CacheConfiguration.MaxPurgesPerTimerPop)

		log.Printf("timerPop metrics: %v cache.len = %v cacheItemsPurged = %v prefetch len = %v",
			dnsProxy.metrics, dnsProxy.cache.len(), cacheItemsPurged, dnsProxy.prefetch.len())
	}
}

func (dnsProxy *dnsProxy) Start() {
	listenAddressAndPort := dnsProxy.configuration.ListenAddress.joinHostPort()

	serveMux := dnsProxy.createServeMux()

	go dnsProxy.runServer(listenAddressAndPort, "tcp", serveMux)
	go dnsProxy.runServer(listenAddressAndPort, "udp", serveMux)

	go dnsProxy.runPeriodicTimer()

	dnsProxy.prefetch.start(dnsProxy)
}
