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
)

type dohClient struct {
	urlObject url.URL
}

func newDOHClient(remoteHTTPURL string) dohClient {
	urlObject, err := url.Parse(remoteHTTPURL)
	if err != nil {
		log.Fatalf("error parsing url %q", remoteHTTPURL)
	}

	log.Printf("urlObject = %+v", urlObject)

	return dohClient{
		urlObject: *urlObject,
	}
}

func (dohClient *dohClient) makeHTTPRequest(ctx context.Context, r *dns.Msg) (resp *dns.Msg, err error) {
	const requestMethod = "GET"
	const dnsMessageMIMEType = "application/dns-json"
	const requestTimeoutSeconds = 5

	ctx, cancel := context.WithTimeout(ctx, requestTimeoutSeconds*time.Second)
	defer cancel()

	if len(r.Question) != 1 {
		err = fmt.Errorf("invalid question len %v", len(r.Question))
		return
	}

	question := &(r.Question[0])

	urlObject := dohClient.urlObject

	queryParameters := url.Values{}
	queryParameters.Set("name", question.Name)
	queryParameters.Set("type", dns.Type(question.Qtype).String())

	urlObject.RawQuery = queryParameters.Encode()

	httpRequest, err := http.NewRequestWithContext(ctx, requestMethod, urlObject.String(), nil)
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

	bodyBuffer, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		log.Printf("ioutil.ReadAll error %v", err)
		return
	}

	resp, err = decodeJSONResponse(r, bodyBuffer)
	if err != nil {
		log.Printf("decodeJSONResponse error %v", err)
		resp = nil
		return
	}

	return
}
