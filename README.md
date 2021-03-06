# go-doh-proxy

Simple and super useful DNS over HTTPS proxy server.

Tech Stack:
* [mikeg/dns](https://github.com/miekg/dns) library for local dns server (udp and tcp).
* [RFC8484 DNS over HTTPS](https://tools.ietf.org/html/rfc8484) for upstream requests with builtin go http2 client.
* [RFC8467 Padding Policies for EDNS](https://tools.ietf.org/html/rfc8467) optionally pads outgoing DoH requests using block-length padding.
* [hashicorp/golang-lru](https://github.com/hashicorp/golang-lru) LRU cache.

Configurable authoritative forward and reverse lookups for local domain.

Allows clamping TTL in proxied response messages.  Responses are cached based by question until the response TTL expires.

## Configuration
See config directory for examples.

## Systemd
See systemd directory for example user unit file.
