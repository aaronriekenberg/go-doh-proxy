package proxy

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/miekg/dns"
	"golang.org/x/sync/semaphore"
)

type dohClient struct {
	urlObject        url.URL
	requestTimeout   time.Duration
	semaphore        *semaphore.Weighted
	dohJSONConverter *dohJSONConverter
}

func newDOHClient(configuration DOHClientConfiguration, dohJSONConverter *dohJSONConverter) *dohClient {
	urlObject, err := url.Parse(configuration.URL)
	if err != nil {
		log.Fatalf("error parsing url %q", configuration.URL)
	}

	return &dohClient{
		urlObject:        *urlObject,
		requestTimeout:   (time.Duration(configuration.RequestTimeoutMilliseconds) * time.Millisecond),
		dohJSONConverter: dohJSONConverter,
		semaphore:        semaphore.NewWeighted(configuration.MaxConcurrentRequests),
	}
}

func (dohClient *dohClient) buildRequestURL(request *dns.Msg) (urlString string, err error) {
	urlObject := dohClient.urlObject

	if len(request.Question) != 1 {
		err = fmt.Errorf("invalid question len %v request %v", len(request.Question), request)
		return
	}

	question := &(request.Question[0])

	queryParameters := url.Values{}
	queryParameters.Set("name", question.Name)
	queryParameters.Set("type", dns.Type(question.Qtype).String())

	urlObject.RawQuery = queryParameters.Encode()

	urlString = urlObject.String()
	return
}

func (dohClient *dohClient) acquireSemaphore() (err error) {
	if !dohClient.semaphore.TryAcquire(1) {
		err = fmt.Errorf("semaphore.TryAcquire failed")
	}
	return
}

func (dohClient *dohClient) releaseSemaphore() {
	dohClient.semaphore.Release(1)
}

func (dohClient *dohClient) internalMakeHTTPRequest(ctx context.Context, urlString string) (responseBuffer []byte, err error) {
	const requestMethod = "GET"
	const dnsMessageMIMEType = "application/dns-json"

	err = dohClient.acquireSemaphore()
	if err != nil {
		err = fmt.Errorf("dohClient.acquireSemaphore error: %w", err)
		return
	}
	defer dohClient.releaseSemaphore()

	ctx, cancel := context.WithTimeout(ctx, dohClient.requestTimeout)
	defer cancel()

	httpRequest, err := http.NewRequestWithContext(ctx, requestMethod, urlString, nil)
	if err != nil {
		err = fmt.Errorf("http.NewRequestWithContext error: %w", err)
		return
	}

	httpRequest.Header.Set("Content-Type", dnsMessageMIMEType)
	httpRequest.Header.Set("Accept", dnsMessageMIMEType)
	httpRequest.Header.Set("User-Agent", "")

	httpResponse, err := http.DefaultClient.Do(httpRequest)
	if err != nil {
		err = fmt.Errorf("DefaultClient.Do error: %w", err)
		return
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		err = fmt.Errorf("non 200 http response code %v", httpResponse.StatusCode)
		return
	}

	responseBuffer, err = ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		err = fmt.Errorf("ioutil.ReadAll error: %w", err)
		responseBuffer = nil
		return
	}

	return
}

func (dohClient *dohClient) makeRequest(ctx context.Context, request *dns.Msg) (responseMessage *dns.Msg, err error) {
	urlString, err := dohClient.buildRequestURL(request)
	if err != nil {
		return
	}

	responseBuffer, err := dohClient.internalMakeHTTPRequest(ctx, urlString)
	if err != nil {
		return
	}

	responseMessage, err = dohClient.dohJSONConverter.decodeJSONResponse(request, responseBuffer)
	if err != nil {
		responseMessage = nil
		return
	}

	return
}
