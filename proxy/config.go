package proxy

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
)

// HostAndPort is a host and port.
type HostAndPort struct {
	Host string `json:"host"`
	Port string `json:"port"`
}

// JoinHostPort joins the host and port.
func (hostAndPort *HostAndPort) joinHostPort() string {
	return net.JoinHostPort(hostAndPort.Host, hostAndPort.Port)
}

// ForwardNameToAddress is a forward name to IP address mapping.
type ForwardNameToAddress struct {
	Name      string `json:"name"`
	IPAddress string `json:"ipAddress"`
}

// ReverseAddressToName is a reverse address to name mapping.
type ReverseAddressToName struct {
	ReverseAddress string `json:"reverseAddress"`
	Name           string `json:"name"`
}

// ProxyConfiguration is the proxy configuration.
type ProxyConfiguration struct {
	MinTTLSeconds uint32 `json:"minTTLSeconds"`
	MaxTTLSeconds uint32 `json:"maxTTLSeconds"`
}

// Configuration is the DNS proxy configuration.
type Configuration struct {
	ListenAddress             HostAndPort            `json:"listenAddress"`
	RemoteHTTPURLs            []string               `json:"remoteHTTPURLs"`
	ForwardDomain             string                 `json:"forwardDomain"`
	ForwardNamesToAddresses   []ForwardNameToAddress `json:"forwardNamesToAddresses"`
	ForwardResponseTTLSeconds uint32                 `json:"forwardResponseTTLSeconds"`
	ReverseDomain             string                 `json:"reverseDomain"`
	ReverseAddressesToNames   []ReverseAddressToName `json:"reverseAddressesToNames"`
	ReverseResponseTTLSeconds uint32                 `json:"reverseResponseTTLSeconds"`
	ProxyConfiguration        ProxyConfiguration     `json:"proxyConfiguration"`
	MaxCacheSize              int                    `json:"maxCacheSize"`
	TimerIntervalSeconds      int                    `json:"timerIntervalSeconds"`
	MaxCachePurgesPerTimerPop int                    `json:"maxCachePurgesPerTimerPop"`
}

// ReadConfiguration reads the DNS proxy configuration from a json file.
func ReadConfiguration(configFile string) (*Configuration, error) {
	log.Printf("reading config file %q", configFile)

	source, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	var config Configuration
	if err = json.Unmarshal(source, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
