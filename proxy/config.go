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

// ForwardDomainConfiguration is the configuration for a forward domain.
type ForwardDomainConfiguration struct {
	Domain             string                 `json:"domain"`
	NamesToAddresses   []ForwardNameToAddress `json:"namesToAddresses"`
	ResponseTTLSeconds uint32                 `json:"responseTTLSeconds"`
}

// ReverseAddressToName is a reverse address to name mapping.
type ReverseAddressToName struct {
	ReverseAddress string `json:"reverseAddress"`
	Name           string `json:"name"`
}

// ReverseDomainConfiguration is the configuration for a reverse domain.
type ReverseDomainConfiguration struct {
	Domain             string                 `json:"domain"`
	AddressesToNames   []ReverseAddressToName `json:"addressesToNames"`
	ResponseTTLSeconds uint32                 `json:"responseTTLSeconds"`
}

// ProxyConfiguration is the proxy configuration.
type ProxyConfiguration struct {
	RemoteHTTPURL      string `json:"remoteHTTPURL"`
	ClampMinTTLSeconds uint32 `json:"clampMinTTLSeconds"`
	ClampMaxTTLSeconds uint32 `json:"clampMaxTTLSeconds"`
}

// CacheConfiguration is the cache configuration.
type CacheConfiguration struct {
	MaxSize              int `json:"maxSize"`
	MaxPurgesPerTimerPop int `json:"maxPurgesPerTimerPop"`
}

// Configuration is the DNS proxy configuration.
type Configuration struct {
	ListenAddress               HostAndPort                  `json:"listenAddress"`
	ForwardDomainConfigurations []ForwardDomainConfiguration `json:"forwardDomainConfigurations"`
	ReverseDomainConfigurations []ReverseDomainConfiguration `json:"reverseDomainConfigurations"`
	ProxyConfiguration          ProxyConfiguration           `json:"proxyConfiguration"`
	CacheConfiguration          CacheConfiguration           `json:"cacheConfiguration"`
	TimerIntervalSeconds        int                          `json:"timerIntervalSeconds"`
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
