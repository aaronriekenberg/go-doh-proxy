package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/miekg/dns"
)

var logger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds)

var remoteHostAndPort = net.JoinHostPort("8.8.8.8", "53")

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

func main() {
	logger.Printf("begin main")

	listenAddrAndPort := ":10053"
	if len(os.Args) == 2 {
		listenAddrAndPort = os.Args[1]
	}
	logger.Printf("listenAddrAndPort = %v", listenAddrAndPort)

	client := new(dns.Client)

	logger.Printf("created client remoteHostAndPort = %v", remoteHostAndPort)

	dnsServeMux := dns.NewServeMux()

	dnsServeMux.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		resp, _, err := client.Exchange(r, remoteHostAndPort)
		if err != nil {
			logger.Printf("client exchange error = %v", err.Error())
			dns.HandleFailed(w, r)
		} else {
			w.WriteMsg(resp)
		}
	})

	dnsServeMux.HandleFunc(forwardDomain, func(w dns.ResponseWriter, r *dns.Msg) {
		if len(r.Question) == 1 {
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
	})

	dnsServeMux.HandleFunc(reverseDomain, func(w dns.ResponseWriter, r *dns.Msg) {
		if len(r.Question) == 1 {
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
	})

	go func() {
		srv := &dns.Server{
			Handler: dnsServeMux,
			Addr:    listenAddrAndPort,
			Net:     "udp"}

		logger.Printf("starting udp server on %v", listenAddrAndPort)

		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Failed to set udp listener %s\n", err.Error())
		}
	}()

	go func() {
		srv := &dns.Server{
			Handler: dnsServeMux,
			Addr:    listenAddrAndPort,
			Net:     "tcp"}

		logger.Printf("starting tcp server on %v", listenAddrAndPort)

		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Failed to set udp listener %s\n", err.Error())
		}
	}()

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig
	log.Fatalf("Signal (%v) received, stopping\n", s)
}
