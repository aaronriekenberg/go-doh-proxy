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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/kr/pretty"
	"github.com/miekg/dns"
	"github.com/patrickmn/go-cache"
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
	TimerIntervalSeconds    uint32                 `json:"timerIntervalSeconds"`
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

type metrics struct {
	cacheHits   uint64
	cacheMisses uint64
}

func (metrics *metrics) IncrementCacheHits() {
	atomic.AddUint64(&metrics.cacheHits, 1)
}

func (metrics *metrics) CacheHits() uint64 {
	return atomic.LoadUint64(&metrics.cacheHits)
}

func (metrics *metrics) IncrementCacheMisses() {
	atomic.AddUint64(&metrics.cacheMisses, 1)
}

func (metrics *metrics) CacheMisses() uint64 {
	return atomic.LoadUint64(&metrics.cacheMisses)
}

type cacheObject struct {
	cacheTime      time.Time
	expirationTime time.Time
	message        *dns.Msg
}

func (co *cacheObject) Copy() *cacheObject {
	return &cacheObject{
		cacheTime:      co.cacheTime,
		expirationTime: co.expirationTime,
		message:        co.message.Copy(),
	}
}

type DNSProxy struct {
	configuration *configuration
	dnsClient     *dns.Client
	cache         *cache.Cache
	metrics       metrics
}

func NewDNSProxy(configuration *configuration) *DNSProxy {
	return &DNSProxy{
		configuration: configuration,
		dnsClient:     new(dns.Client),
		cache:         cache.New(cache.NoExpiration, 1*time.Minute),
	}
}

func (dnsProxy *DNSProxy) getMinTTLSeconds(m *dns.Msg) uint32 {
	foundTTL := false
	var minTTLSeconds uint32

	processRR := func(rr dns.RR) {
		ttl := rr.Header().Ttl
		if ttl < dnsProxy.configuration.MinTTLSeconds {
			ttl = dnsProxy.configuration.MinTTLSeconds
		}
		if ttl > dnsProxy.configuration.MaxTTLSeconds {
			ttl = dnsProxy.configuration.MaxTTLSeconds
		}
		if (!foundTTL) || (ttl < minTTLSeconds) {
			minTTLSeconds = ttl
			foundTTL = true
		}
		rr.Header().Ttl = ttl
	}

	for _, rr := range m.Answer {
		processRR(rr)
	}
	for _, rr := range m.Ns {
		processRR(rr)
	}

	return minTTLSeconds
}

func (dnsProxy *DNSProxy) adjustTTL(cacheObject *cacheObject) bool {
	valid := true
	now := time.Now()

	if now.Equal(cacheObject.expirationTime) || now.After(cacheObject.expirationTime) {
		valid = false
	}

	var secondsToSubtractFromTTL float64
	if valid {
		secondsToSubtractFromTTL = now.Sub(cacheObject.cacheTime).Seconds()
		if secondsToSubtractFromTTL <= 0 {
			valid = false
		}
	}

	if valid {
		m := cacheObject.message
		adjustRRTTL := func(rr dns.RR) {
			ttl := rr.Header().Ttl
			ttl -= uint32(secondsToSubtractFromTTL)
			if ttl <= 0 {
				valid = false
			} else {
				rr.Header().Ttl = ttl
			}
		}

		for _, rr := range m.Answer {
			adjustRRTTL(rr)
		}
		for _, rr := range m.Ns {
			adjustRRTTL(rr)
		}
	}

	return valid
}

func (dnsProxy *DNSProxy) createProxyHandlerFunc() dns.HandlerFunc {
	remoteHostAndPortStrings := make([]string, 0, len(dnsProxy.configuration.RemoteAddressesAndPorts))
	for _, hostAndPort := range dnsProxy.configuration.RemoteAddressesAndPorts {
		remoteHostAndPortStrings = append(remoteHostAndPortStrings, hostAndPort.JoinHostPort())
	}

	return func(w dns.ResponseWriter, r *dns.Msg) {

		requestID := r.Id
		responded := false

		co, ok := dnsProxy.cache.Get(getQuestionCacheKey(r))
		if ok {
			cacheObjectCopy := co.(*cacheObject).Copy()
			if dnsProxy.adjustTTL(cacheObjectCopy) {
				dnsProxy.metrics.IncrementCacheHits()
				msg := cacheObjectCopy.message
				msg.Id = requestID
				w.WriteMsg(msg)
				responded = true
			}
		}

		if !responded {
			dnsProxy.metrics.IncrementCacheMisses()
			r.Id = dns.Id()
			remoteHostAndPort := pickRandomStringSliceEntry(remoteHostAndPortStrings)
			resp, _, err := dnsProxy.dnsClient.Exchange(r, remoteHostAndPort)
			if err != nil {
				r.Id = requestID
				dns.HandleFailed(w, r)
			} else {

				if (resp.Rcode == dns.RcodeSuccess) || (resp.Rcode == dns.RcodeNameError) {
					minTTLSeconds := dnsProxy.getMinTTLSeconds(resp)
					respQuestionCacheKey := getQuestionCacheKey(resp)

					if (len(respQuestionCacheKey) > 0) && (minTTLSeconds > 0) {
						ttlDuration := time.Second * time.Duration(minTTLSeconds)
						now := time.Now()
						expirationTime := now.Add(ttlDuration)
						cacheObject := &cacheObject{
							cacheTime:      now,
							expirationTime: expirationTime,
							message:        resp.Copy(),
						}
						dnsProxy.cache.Set(respQuestionCacheKey, cacheObject, ttlDuration)
					}
				}

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
			logger.Printf("timerPop cacheHits = %v cacheMisses = %v cache.ItemCount = %v",
				dnsProxy.metrics.CacheHits(), dnsProxy.metrics.CacheMisses(), dnsProxy.cache.ItemCount())
		}
	}
}

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
