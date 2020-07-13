package proxy

import (
	"log"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/miekg/dns"
)

type prefetchRequest struct {
	cacheKey string
	question *dns.Question
}

type prefetch struct {
	dnsProxy              *dnsProxy
	cacheKeyToQuestion    *lru.Cache
	prefetchRequstChannel chan *prefetchRequest
	sleepInterval         time.Duration
}

func newPrefetch() *prefetch {
	cacheKeyToQuestion, err := lru.New(10000)
	if err != nil {
		log.Fatalf("prefetch lru.New error %v", err)
	}

	return &prefetch{
		cacheKeyToQuestion:    cacheKeyToQuestion,
		prefetchRequstChannel: make(chan *prefetchRequest, 10),
		sleepInterval:         time.Duration(15) * time.Minute,
	}
}

func (prefetch *prefetch) addToPrefetch(cacheKey string, question *dns.Question) {
	questionCopy := *question
	prefetch.cacheKeyToQuestion.Add(cacheKey, &questionCopy)
}

func (prefetch *prefetch) prefetchRequestTask() {
	log.Printf("prefetchRequestTask")

	for {
		prefetchRequest := <-prefetch.prefetchRequstChannel

		prefetch.dnsProxy.makePrefetchRequest(prefetchRequest.cacheKey, prefetchRequest.question)
	}
}

func (prefetch *prefetch) runPeriodicPrefetch() {
	log.Printf("runPeriodicPrefetch sleepInterval %v", prefetch.sleepInterval)

	for {
		log.Printf("runPeriodicPrefetch before sleep")

		time.Sleep(prefetch.sleepInterval)

		log.Printf("runPeriodicPrefetch after sleep")

		keys := prefetch.cacheKeyToQuestion.Keys()
		log.Printf("runPeriodicPrefetch keys length = %v", len(keys))

		for _, key := range keys {
			cacheKey := key.(string)
			value, ok := prefetch.cacheKeyToQuestion.Get(cacheKey)
			if ok {
				question := value.(*dns.Question)
				prefetch.prefetchRequstChannel <- &prefetchRequest{
					cacheKey: cacheKey,
					question: question,
				}
			}
		}
	}
}

func (prefetch *prefetch) len() int {
	return prefetch.cacheKeyToQuestion.Len()
}

func (prefetch *prefetch) start(dnsProxy *dnsProxy) {
	log.Printf("prefetch.start")

	prefetch.dnsProxy = dnsProxy

	go prefetch.prefetchRequestTask()
	go prefetch.runPeriodicPrefetch()
}
