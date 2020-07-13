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
	urlObject               url.URL
	sepaphoreAcquireTimeout time.Duration
	requestTimeout          time.Duration
	semaphore               *semaphore.Weighted
	dohJSONConverter        *dohJSONConverter
}

func newDOHClient(configuration DOHClientConfiguration, dohJSONConverter *dohJSONConverter) *dohClient {
	urlObject, err := url.Parse(configuration.URL)
	if err != nil {
		log.Fatalf("error parsing url %q", configuration.URL)
	}

	sepaphoreAcquireTimeout := time.Duration(configuration.SemaphoreAcquireTimeoutMilliseconds) * time.Millisecond
	requestTimeout := time.Duration(configuration.RequestTimeoutMilliseconds) * time.Millisecond

	log.Printf("newDOHClient sepaphoreAcquireTimeout = %v requestTimeout = %v maxConcurrentRequests = %v", sepaphoreAcquireTimeout, requestTimeout, configuration.MaxConcurrentRequests)

	return &dohClient{
		urlObject:               *urlObject,
		sepaphoreAcquireTimeout: sepaphoreAcquireTimeout,
		requestTimeout:          requestTimeout,
		dohJSONConverter:        dohJSONConverter,
		semaphore:               semaphore.NewWeighted(configuration.MaxConcurrentRequests),
	}
}

func (dohClient *dohClient) buildRequestURL(question *dns.Question) string {
	urlObject := dohClient.urlObject

	queryParameters := url.Values{}
	queryParameters.Set("name", question.Name)
	queryParameters.Set("type", dns.Type(question.Qtype).String())

	urlObject.RawQuery = queryParameters.Encode()

	return urlObject.String()
}

func (dohClient *dohClient) acquireSemaphore(ctx context.Context) (err error) {
	ctx, cancel := context.WithTimeout(ctx, dohClient.sepaphoreAcquireTimeout)
	defer cancel()

	err = dohClient.semaphore.Acquire(ctx, 1)
	return
}

func (dohClient *dohClient) releaseSemaphore() {
	dohClient.semaphore.Release(1)
}

func (dohClient *dohClient) internalMakeHTTPRequest(ctx context.Context, urlString string) (responseBuffer []byte, err error) {
	const requestMethod = "GET"
	const dnsMessageMIMEType = "application/dns-json"

	err = dohClient.acquireSemaphore(ctx)
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
	if len(request.Question) != 1 {
		err = fmt.Errorf("invalid question len %v request %v", len(request.Question), request)
		return
	}

	question := &(request.Question[0])

	urlString := dohClient.buildRequestURL(question)

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
