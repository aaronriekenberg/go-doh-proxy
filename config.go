package main

import (
	"encoding/json"
	"io/ioutil"
	"net"
)

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
	RemoteHTTPURL           string                 `json:"remoteHTTPURL"`
	ForwardDomain           string                 `json:"forwardDomain"`
	ForwardNamesToAddresses []forwardNameToAddress `json:"forwardNamesToAddresses"`
	ReverseDomain           string                 `json:"reverseDomain"`
	ReverseAddressesToNames []reverseAddressToName `json:"reverseAddressesToNames"`
	MinTTLSeconds           uint32                 `json:"minTTLSeconds"`
	MaxTTLSeconds           uint32                 `json:"maxTTLSeconds"`
	MaxCacheSize            int                    `json:"maxCacheSize"`
	TimerIntervalSeconds    int                    `json:"timerIntervalSeconds"`
	MaxPurgesPerTimerPop    int                    `json:"maxPurgesPerTimerPop"`
}

func readConfiguration(configFile string) *configuration {
	logger.Printf("reading json file %v", configFile)

	source, err := ioutil.ReadFile(configFile)
	if err != nil {
		logger.Fatalf("error reading %v: %v", configFile, err)
	}

	var config configuration
	if err = json.Unmarshal(source, &config); err != nil {
		logger.Fatalf("error parsing %v: %v", configFile, err)
	}

	return &config
}
