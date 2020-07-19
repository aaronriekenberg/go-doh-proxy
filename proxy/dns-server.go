package proxy

import (
	"log"

	"github.com/miekg/dns"
)

type dnsServer struct {
	configuration *DNSServerConfiguration
}

func newDNSServer(configuration *DNSServerConfiguration) *dnsServer {
	return &dnsServer{
		configuration: configuration,
	}
}

func (dnsServer *dnsServer) runServer(listenAddrAndPort, net string, serveMux *dns.ServeMux) {
	srv := &dns.Server{
		Handler: serveMux,
		Addr:    listenAddrAndPort,
		Net:     net,
	}

	log.Printf("starting %v server on %v", net, listenAddrAndPort)

	err := srv.ListenAndServe()
	log.Fatalf("ListenAndServe error for net %s: %v", net, err)
}

func (dnsServer *dnsServer) start(serveMux *dns.ServeMux) {
	log.Printf("dnsServer.start")

	listenAddressAndPort := dnsServer.configuration.ListenAddress.joinHostPort()

	go dnsServer.runServer(listenAddressAndPort, "tcp", serveMux)
	go dnsServer.runServer(listenAddressAndPort, "udp", serveMux)

}
