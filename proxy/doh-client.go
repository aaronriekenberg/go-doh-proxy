package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/miekg/dns"
)

type dohClient struct {
	remoteHTTPURLs []string
}

func newDOHClient(remoteHTTPURLs []string) dohClient {
	return dohClient{
		remoteHTTPURLs: remoteHTTPURLs,
	}
}

func (dohClient *dohClient) pickRandomRemoteHTTPURL() string {
	length := len(dohClient.remoteHTTPURLs)

	if length == 1 {
		return dohClient.remoteHTTPURLs[0]
	}

	return dohClient.remoteHTTPURLs[rand.Intn(length)]
}

func (dohClient *dohClient) makeHTTPRequest(ctx context.Context, r *dns.Msg) (resp *dns.Msg, err error) {
	const requestMethod = "POST"
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

	remoteURL := dohClient.pickRandomRemoteHTTPURL()

	httpRequest, err := http.NewRequestWithContext(ctx, requestMethod, remoteURL, bytes.NewReader(packedRequest))
	if err != nil {
		log.Printf("NewRequest error %v", err)
		return
	}

	httpRequest.Header.Set("Content-Type", dnsMessageMIMEType)
	httpRequest.Header.Set("Accept", dnsMessageMIMEType)
	httpRequest.Header.Set("User-Agent", "")

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
