package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/kr/pretty"
)

func awaitShutdownSignal() {
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig
	log.Fatalf("Signal (%v) received, stopping", s)
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	if len(os.Args) != 2 {
		log.Fatalf("Usage: %v <config json file>", os.Args[0])
	}

	configFile := os.Args[1]
	configuration := readConfiguration(configFile)
	log.Printf("configuration:\n%# v", pretty.Formatter(configuration))

	dnsProxy := newDNSProxy(configuration)
	dnsProxy.Start()

	awaitShutdownSignal()
}
