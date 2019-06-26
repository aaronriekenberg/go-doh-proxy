package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/kr/pretty"
	"github.com/miekg/dns"
)

var logger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds)

type hostAndPort struct {
	Host string `json:"host"`
	Port string `json:"port"`
}

func (hostAndPort hostAndPort) JoinHostPort() string {
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
}

type DNSProxy struct {
	configuration *configuration
	dnsClient     *dns.Client
}

func NewDNSProxy(configuration *configuration) *DNSProxy {
	return &DNSProxy{
		configuration: configuration,
		dnsClient:     new(dns.Client),
	}
}

func pickRandomStringSliceEntry(s []string) string {
	return s[rand.Intn(len(s))]
}

func (dnsProxy *DNSProxy) createProxyHandlerFunc() dns.HandlerFunc {
	remoteHostAndPortStrings := make([]string, 0, len(dnsProxy.configuration.RemoteAddressesAndPorts))
	for _, hostAndPort := range dnsProxy.configuration.RemoteAddressesAndPorts {
		remoteHostAndPortStrings = append(remoteHostAndPortStrings, hostAndPort.JoinHostPort())
	}

	return func(w dns.ResponseWriter, r *dns.Msg) {
		originalID := r.Id
		r.Id = dns.Id()
		remoteHostAndPort := pickRandomStringSliceEntry(remoteHostAndPortStrings)
		resp, _, err := dnsProxy.dnsClient.Exchange(r, remoteHostAndPort)
		if err != nil {
			logger.Printf("dnsClient exchange remoteHostAndPort = %v error = %v", remoteHostAndPort, err.Error())
			r.Id = originalID
			dns.HandleFailed(w, r)
		} else {
			resp.Id = originalID
			w.WriteMsg(resp)
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
		logger.Fatalf("Failed to set udp listener %s\n", err.Error())
	}
}

func (dnsProxy *DNSProxy) Start() {
	dnsServeMux := dnsProxy.createServeMux()

	listenAddressAndPort := dnsProxy.configuration.ListenAddress.JoinHostPort()

	go dnsProxy.runServer(dnsServeMux, listenAddressAndPort, "udp")
	go dnsProxy.runServer(dnsServeMux, listenAddressAndPort, "tcp")
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
