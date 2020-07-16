package main

import (
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/aaronriekenberg/go-doh-proxy/proxy"
	"github.com/kr/pretty"
)

var gitCommit string

func awaitShutdownSignal() {
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig
	log.Fatalf("Signal (%v) received, stopping", s)
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	log.Printf("gitCommit: %q GOMAXPROCS=%v go version: %q", gitCommit, runtime.GOMAXPROCS(0), runtime.Version())

	if len(os.Args) != 2 {
		log.Fatalf("Usage: %v <config json file>", os.Args[0])
	}

	configFile := os.Args[1]
	configuration, err := proxy.ReadConfiguration(configFile)
	if err != nil {
		log.Fatalf("proxy.ReadConfiguration error: %v", err)
	}
	log.Printf("configuration:\n%# v", pretty.Formatter(configuration))

	dnsProxy := proxy.NewDNSProxy(configuration)
	dnsProxy.Start()

	awaitShutdownSignal()
}
