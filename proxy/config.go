package proxy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
)

// MetricsConfiguration is the metrics configuration.
type MetricsConfiguration struct {
	TimerIntervalSeconds int `json:"timerIntervalSeconds"`
}

// DNSServerConfiguration is the DNS server configuration.
type DNSServerConfiguration struct {
	ListenAddress HostAndPort `json:"listenAddress"`
}

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

// DNSProxyConfiguration is the proxy configuration.
type DNSProxyConfiguration struct {
	ForwardDomainConfigurations []ForwardDomainConfiguration `json:"forwardDomainConfigurations"`
	ReverseDomainConfigurations []ReverseDomainConfiguration `json:"reverseDomainConfigurations"`
	BlockedDomainsFile          string                       `json:"blockedDomainsFile"`
	ClampMinTTLSeconds          uint32                       `json:"clampMinTTLSeconds"`
	ClampMaxTTLSeconds          uint32                       `json:"clampMaxTTLSeconds"`
}

// DOHClientConfiguration is the DOH client configuration
type DOHClientConfiguration struct {
	URL                                 string `json:"url"`
	MaxConcurrentRequests               int64  `json:"maxConcurrentRequests"`
	SemaphoreAcquireTimeoutMilliseconds int    `json:"semaphoreAcquireTimeoutMilliseconds"`
	RequestTimeoutMilliseconds          int    `json:"requestTimeoutMilliseconds"`
}

// CacheConfiguration is the cache configuration.
type CacheConfiguration struct {
	MaxSize              int `json:"maxSize"`
	MaxPurgesPerTimerPop int `json:"maxPurgesPerTimerPop"`
	TimerIntervalSeconds int `json:"timerIntervalSeconds"`
}

// PrefetchConfiguration is the prefetch configuration.
type PrefetchConfiguration struct {
	MaxCacheSize            int `json:"maxCacheSize"`
	NumWorkers              int `json:"numWorkers"`
	SleepIntervalSeconds    int `json:"sleepIntervalSeconds"`
	MaxCacheEntryAgeSeconds int `json:"maxCacheEntryAgeSeconds"`
}

// PprofConfiguration is the pprof configuration.
type PprofConfiguration struct {
	Enabled       bool   `json:"enabled"`
	ListenAddress string `json:"listenAddress"`
}

// Configuration is the DNS proxy configuration.
type Configuration struct {
	MetricsConfiguration   MetricsConfiguration   `json:"metricsConfiguration"`
	DNSServerConfiguration DNSServerConfiguration `json:"dnsServerConfiguration"`
	DOHClientConfiguration DOHClientConfiguration `json:"dohClientConfiguration"`
	DNSProxyConfiguration  DNSProxyConfiguration  `json:"dnsProxyConfiguration"`
	CacheConfiguration     CacheConfiguration     `json:"cacheConfiguration"`
	PrefetchConfiguration  PrefetchConfiguration  `json:"PrefetchConfiguration"`
	PprofConfiguration     PprofConfiguration     `json:"pprofConfiguration"`
}

// ReadConfiguration reads the DNS proxy configuration from a json file.
func ReadConfiguration(configFile string) (*Configuration, error) {
	log.Printf("reading config file %q", configFile)

	source, err := ioutil.ReadFile(configFile)
	if err != nil {
		err = fmt.Errorf("ioutil.ReadFile error: %w", err)
		return nil, err
	}

	var config Configuration
	if err = json.Unmarshal(source, &config); err != nil {
		err = fmt.Errorf("json.Unmarshal error: %w", err)
		return nil, err
	}

	return &config, nil
}
