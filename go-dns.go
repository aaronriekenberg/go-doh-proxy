package main

import (
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/miekg/dns"
)

var logger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds)

var remoteHostsAndPorts = []string{
	net.JoinHostPort("8.8.8.8", "53"),
	net.JoinHostPort("8.8.4.4", "53"),
}

const forwardDomain = "domain."

var forwardNamesToAddresses = map[string]net.IP{
	"apu2.domain.":        net.ParseIP("192.168.1.1"),
	"raspberrypi.domain.": net.ParseIP("192.168.1.100"),
}

const reverseDomain = "1.168.192.in-addr.arpa."

var reverseAddressToName = map[string]string{
	"1.1.168.192.in-addr.arpa.":   "apu2.domain.",
	"100.1.168.192.in-addr.arpa.": "raspberrypi.domain.",
}

func pickRandomRemoteHostAndPort() string {
	return remoteHostsAndPorts[rand.Intn(len(remoteHostsAndPorts))]
}

func createProxyHandlerFunc(dnsClient *dns.Client) dns.HandlerFunc {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		originalID := r.Id
		r.Id = dns.Id()
		remoteHostAndPort := pickRandomRemoteHostAndPort()
		resp, _, err := dnsClient.Exchange(r, remoteHostAndPort)
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

func createForwardDomainHandlerFunc() dns.HandlerFunc {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		if len(r.Question) > 0 {
			question := r.Question[0]
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

func createReverseHandlerFunc() dns.HandlerFunc {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		if len(r.Question) > 1 {
			question := r.Question[0]
			if question.Qtype == dns.TypePTR {
				msg := new(dns.Msg)
				name, ok := reverseAddressToName[question.Name]
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

func createServeMux(dnsClient *dns.Client) *dns.ServeMux {

	dnsServeMux := dns.NewServeMux()

	dnsServeMux.HandleFunc(".", createProxyHandlerFunc(dnsClient))

	dnsServeMux.HandleFunc(forwardDomain, createForwardDomainHandlerFunc())

	dnsServeMux.HandleFunc(reverseDomain, createReverseHandlerFunc())

	return dnsServeMux
}

func runDNSServer(dnsServeMux *dns.ServeMux, listenAddrAndPort, net string) {
	srv := &dns.Server{
		Handler: dnsServeMux,
		Addr:    listenAddrAndPort,
		Net:     net}

	logger.Printf("starting %v server on %v", net, listenAddrAndPort)

	if err := srv.ListenAndServe(); err != nil {
		logger.Fatalf("Failed to set udp listener %s\n", err.Error())
	}
}

func main() {
	logger.Printf("begin main")

	listenAddrAndPort := ":10053"
	if len(os.Args) == 2 {
		listenAddrAndPort = os.Args[1]
	}
	logger.Printf("listenAddrAndPort = %v", listenAddrAndPort)

	dnsClient := new(dns.Client)

	logger.Printf("created dnsClient remoteHostsAndPorts = %v", remoteHostsAndPorts)

	dnsServeMux := createServeMux(dnsClient)

	go runDNSServer(dnsServeMux, listenAddrAndPort, "udp")
	go runDNSServer(dnsServeMux, listenAddrAndPort, "tcp")

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig
	logger.Fatalf("Signal (%v) received, stopping\n", s)
}
