# go-dns

DoH go dns proxy server with authoritative forward and reverse lookups for local domain.

Using [mikeg/dns](https://github.com/miekg/dns) library for dns server (udp and tcp).

Uses [RFC8484 DNS over HTTPS](https://tools.ietf.org/html/rfc8484) for upstream requests with builtin go http client.

Supports clamping ttl in response messages between configurable min and max.  Responses are cached based by question until the response ttl expires.
