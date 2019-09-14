package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/miekg/dns"
)

type dohClient struct {
	remoteHTTPURL string
}

func newDOHClient(remoteHTTPURL string) *dohClient {
	return &dohClient{
		remoteHTTPURL: remoteHTTPURL,
	}
}

func (dohClient *dohClient) MakeHTTPRequest(ctx context.Context, r *dns.Msg) (resp *dns.Msg, err error) {
	const dnsMessageMIMEType = "application/dns-message"
	const maxBodyBytes = 65535 // RFC 8484 section 6
	const requestTimeoutSeconds = 5

	ctx, cancel := context.WithTimeout(ctx, requestTimeoutSeconds*time.Second)
	defer cancel()

	packedRequest, err := r.Pack()
	if err != nil {
		log.Printf("error packing request %v", err)
		return
	}

	httpRequest, err := http.NewRequestWithContext(ctx, "POST", dohClient.remoteHTTPURL, bytes.NewReader(packedRequest))
	if err != nil {
		log.Printf("NewRequest error %v", err)
		return
	}

	httpRequest.Header.Set("Content-Type", dnsMessageMIMEType)
	httpRequest.Header.Set("Accept", dnsMessageMIMEType)

	httpResponse, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		log.Printf("DefaultClient.Do error %v", err)
		return
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		err = fmt.Errorf("non 200 http response code %v", httpResponse.StatusCode)
		return
	}

	bodyBuffer, err := ioutil.ReadAll(io.LimitReader(httpResponse.Body, maxBodyBytes+1))
	if err != nil {
		log.Printf("ioutil.ReadAll error %v", err)
		return
	}

	if len(bodyBuffer) > maxBodyBytes {
		err = errors.New("http response body too large")
		return
	}

	resp = new(dns.Msg)
	err = resp.Unpack(bodyBuffer)
	if err != nil {
		log.Printf("Unpack error %v", err)
		resp = nil
		return
	}

	return
}
