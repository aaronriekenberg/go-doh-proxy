package proxy

import (
	"log"
	"net/http"
	"net/http/pprof"
)

func startPprof(configuration *PprofConfiguration) {
	if configuration.Enabled {
		log.Printf("startPprof starting server on %v", configuration.ListenAddress)

		serveMux := http.NewServeMux()

		serveMux.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
		serveMux.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
		serveMux.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
		serveMux.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
		serveMux.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))

		go func() {
			log.Fatalf("http.ListenAndServe error: %v", http.ListenAndServe(configuration.ListenAddress, serveMux))
		}()
	}
}
