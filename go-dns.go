package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aaronriekenberg/go-dns/cache"

	"github.com/kr/pretty"
	"github.com/miekg/dns"
)

var logger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds)

type hostAndPort struct {
	Host string `json:"host"`
	Port string `json:"port"`
}

func (hostAndPort *hostAndPort) JoinHostPort() string {
	return net.JoinHostPort(hostAndPort.Host, hostAndPort.Port)
}

type forwardNameToAddress struct {
	Name      string `json:"name"`
	IPAddress string `json:"ipAddress"`
}

type reverseAddressToName struct {
	ReverseAddress string `json:"reverseAddress"`
	Name           string `json:"name"`
}

type configuration struct {
	ListenAddress           hostAndPort            `json:"listenAddress"`
	RemoteAddressesAndPorts []hostAndPort          `json:"remoteAddressesAndPorts"`
	ForwardDomain           string                 `json:"forwardDomain"`
	ForwardNamesToAddresses []forwardNameToAddress `json:"forwardNamesToAddresses"`
	ReverseDomain           string                 `json:"reverseDomain"`
	ReverseAddressesToNames []reverseAddressToName `json:"reverseAddressesToNames"`
	MinTTLSeconds           uint32                 `json:"minTTLSeconds"`
	MaxTTLSeconds           uint32                 `json:"maxTTLSeconds"`
	TimerIntervalSeconds    int                    `json:"timerIntervalSeconds"`
}

func pickRandomStringSliceEntry(s []string) string {
	return s[rand.Intn(len(s))]
}

func getQuestionCacheKey(m *dns.Msg) string {
	key := ""
	first := true

	for _, question := range m.Question {
		if !first {
			key += "|"
		}
		key += fmt.Sprintf("%s:%d:%d", strings.ToLower(question.Name), question.Qtype, question.Qclass)
		first = false
	}

	return key
}

type cacheObject struct {
	cacheTime      time.Time
	expirationTime time.Time
	message        dns.Msg
}

func (co *cacheObject) Expired(now time.Time) bool {
	return (now.After(co.expirationTime) || now.Equal(co.expirationTime))
}

func (co *cacheObject) ExpirationTime() time.Time {
	return co.expirationTime
}

// DNSProxy is the dns proxy
type DNSProxy struct {
	configuration            *configuration
	remoteHostAndPortStrings []string
	dnsClient                *dns.Client
	cache                    *cache.Cache
	metrics                  metrics
}

// NewDNSProxy creates the dns proxy.
func NewDNSProxy(configuration *configuration) *DNSProxy {
	remoteHostAndPortStrings := make([]string, 0, len(configuration.RemoteAddressesAndPorts))
	for _, hostAndPort := range configuration.RemoteAddressesAndPorts {
		remoteHostAndPortStrings = append(remoteHostAndPortStrings, hostAndPort.JoinHostPort())
	}

	return &DNSProxy{
		configuration:            configuration,
		remoteHostAndPortStrings: remoteHostAndPortStrings,
		dnsClient:                new(dns.Client),
		cache:                    cache.New(),
	}
}

func (dnsProxy *DNSProxy) clampAndGetMinTTLSeconds(m *dns.Msg) uint32 {
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

func (dnsProxy *DNSProxy) copyCachedMessageForHit(expirable cache.Expirable) *dns.Msg {

	uncopiedCacheObject, ok := expirable.(*cacheObject)
	if !ok {
		return nil
	}

	now := time.Now()

	if uncopiedCacheObject.Expired(now) {
		return nil
	}

	secondsToSubtractFromTTL := int64(now.Sub(uncopiedCacheObject.cacheTime).Seconds())
	ok = true

	adjustRRHeaderTTL := func(rrHeader *dns.RR_Header) {
		ttl := int64(rrHeader.Ttl) - secondsToSubtractFromTTL
		if ttl <= 0 {
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

func (dnsProxy *DNSProxy) clampTTLAndCacheResponse(resp *dns.Msg) {
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

func (dnsProxy *DNSProxy) createProxyHandlerFunc() dns.HandlerFunc {
	return func(w dns.ResponseWriter, r *dns.Msg) {

		requestID := r.Id
		responded := false

		co, ok := dnsProxy.cache.Get(getQuestionCacheKey(r))
		if ok {
			if cacheMessageCopy := dnsProxy.copyCachedMessageForHit(co); cacheMessageCopy != nil {
				dnsProxy.metrics.IncrementCacheHits()
				cacheMessageCopy.Id = requestID
				w.WriteMsg(cacheMessageCopy)
				responded = true
			}
		}

		if !responded {
			dnsProxy.metrics.IncrementCacheMisses()
			r.Id = dns.Id()
			remoteHostAndPort := pickRandomStringSliceEntry(dnsProxy.remoteHostAndPortStrings)
			resp, _, err := dnsProxy.dnsClient.Exchange(r, remoteHostAndPort)
			if err != nil {
				dnsProxy.metrics.IncrementClientErrors()
				logger.Printf("dnsClient exchange remoteHostAndPort = %v error = %v", remoteHostAndPort, err.Error())
				r.Id = requestID
				dns.HandleFailed(w, r)
			} else {
				dnsProxy.clampTTLAndCacheResponse(resp)
				resp.Id = requestID
				w.WriteMsg(resp)
			}
		}
	}
}

func (dnsProxy *DNSProxy) createForwardDomainHandlerFunc() dns.HandlerFunc {
	forwardNamesToAddresses := make(map[string]net.IP)
	for _, forwardNameToAddress := range dnsProxy.configuration.ForwardNamesToAddresses {
		forwardNamesToAddresses[forwardNameToAddress.Name] = net.ParseIP(forwardNameToAddress.IPAddress)
	}

	return func(w dns.ResponseWriter, r *dns.Msg) {
		if len(r.Question) > 0 {
			question := &(r.Question[0])
			if question.Qtype == dns.TypeA {
				msg := new(dns.Msg)
				address, ok := forwardNamesToAddresses[question.Name]
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
				w.WriteMsg(msg)
				return
			}
		}
		dns.HandleFailed(w, r)
	}
}

func (dnsProxy *DNSProxy) createReverseHandlerFunc() dns.HandlerFunc {
	reverseAddressesToNames := make(map[string]string)
	for _, reverseAddressToName := range dnsProxy.configuration.ReverseAddressesToNames {
		reverseAddressesToNames[reverseAddressToName.ReverseAddress] = reverseAddressToName.Name
	}

	return func(w dns.ResponseWriter, r *dns.Msg) {
		if len(r.Question) > 0 {
			question := &(r.Question[0])
			if question.Qtype == dns.TypePTR {
				msg := new(dns.Msg)
				name, ok := reverseAddressesToNames[question.Name]
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
				w.WriteMsg(msg)
				return
			}
		}
		dns.HandleFailed(w, r)
	}
}

func (dnsProxy *DNSProxy) createServeMux() *dns.ServeMux {

	dnsServeMux := dns.NewServeMux()

	dnsServeMux.HandleFunc(".", dnsProxy.createProxyHandlerFunc())

	dnsServeMux.HandleFunc(dnsProxy.configuration.ForwardDomain, dnsProxy.createForwardDomainHandlerFunc())

	dnsServeMux.HandleFunc(dnsProxy.configuration.ReverseDomain, dnsProxy.createReverseHandlerFunc())

	return dnsServeMux
}

func (dnsProxy *DNSProxy) runServer(dnsServeMux *dns.ServeMux, listenAddrAndPort, net string) {
	srv := &dns.Server{
		Handler: dnsServeMux,
		Addr:    listenAddrAndPort,
		Net:     net,
	}

	logger.Printf("starting %v server on %v", net, listenAddrAndPort)

	if err := srv.ListenAndServe(); err != nil {
		logger.Fatalf("Failed to set %v listener %s\n", net, err.Error())
	}
}

func (dnsProxy *DNSProxy) runPeriodicTimer() {
	ticker := time.NewTicker(time.Second * time.Duration(dnsProxy.configuration.TimerIntervalSeconds))
	for {
		select {
		case <-ticker.C:
			logger.Printf("timerPop metrics: %v cache.Stats: %v",
				dnsProxy.metrics.String(), dnsProxy.cache.Stats())
		}
	}
}

// Start starts the dns proxy.
func (dnsProxy *DNSProxy) Start() {
	dnsServeMux := dnsProxy.createServeMux()

	listenAddressAndPort := dnsProxy.configuration.ListenAddress.JoinHostPort()

	go dnsProxy.runServer(dnsServeMux, listenAddressAndPort, "udp")
	go dnsProxy.runServer(dnsServeMux, listenAddressAndPort, "tcp")

	go dnsProxy.runPeriodicTimer()
}

func readConfiguration(configFile string) *configuration {
	logger.Printf("reading json file %v", configFile)

	source, err := ioutil.ReadFile(configFile)
	if err != nil {
		logger.Fatalf("error reading %v: %v", configFile, err.Error())
	}

	var config configuration
	if err = json.Unmarshal(source, &config); err != nil {
		logger.Fatalf("error parsing %v: %v", configFile, err.Error())
	}

	return &config
}

func main() {
	if len(os.Args) != 2 {
		logger.Fatalf("Usage: %v <config json file>", os.Args[0])
	}

	configFile := os.Args[1]
	configuration := readConfiguration(configFile)
	logger.Printf("configuration:\n%# v", pretty.Formatter(configuration))

	dnsProxy := NewDNSProxy(configuration)
	dnsProxy.Start()

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig
	logger.Fatalf("Signal (%v) received, stopping\n", s)
}
