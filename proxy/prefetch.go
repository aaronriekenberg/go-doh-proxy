package proxy

import (
	"log"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/miekg/dns"
)

type prefetchCacheEntry struct {
	question       dns.Question
	expirationTime time.Time
}

func (prefetchCacheEntry *prefetchCacheEntry) expired(now time.Time) bool {
	return now.After(prefetchCacheEntry.expirationTime)
}

type prefetchRequest struct {
	cacheKey string
	question dns.Question
}

type prefetch struct {
	cacheKeyToQuestion    *lru.Cache
	prefetchRequstChannel chan *prefetchRequest
	numWorkers            int
	sleepInterval         time.Duration
	maxCacheEntryAge      time.Duration
}

func newPrefetch(prefetchConfiguration *PrefetchConfiguration) *prefetch {
	cacheKeyToQuestion, err := lru.New(prefetchConfiguration.MaxCacheSize)
	if err != nil {
		log.Fatalf("prefetch lru.New error %v", err)
	}

	return &prefetch{
		cacheKeyToQuestion:    cacheKeyToQuestion,
		prefetchRequstChannel: make(chan *prefetchRequest, prefetchConfiguration.NumWorkers),
		numWorkers:            prefetchConfiguration.NumWorkers,
		sleepInterval:         time.Duration(prefetchConfiguration.SleepIntervalSeconds) * time.Second,
		maxCacheEntryAge:      time.Duration(prefetchConfiguration.MaxCacheEntryAgeSeconds) * time.Second,
	}
}

func (prefetch *prefetch) addToPrefetch(cacheKey string, question *dns.Question) {
	prefetch.cacheKeyToQuestion.Add(cacheKey, &prefetchCacheEntry{
		question:       *question,
		expirationTime: time.Now().Add(prefetch.maxCacheEntryAge),
	})
}

func (prefetch *prefetch) runPeriodicPrefetch() {
	log.Printf("runPeriodicPrefetch sleepInterval %v", prefetch.sleepInterval)

	for {
		time.Sleep(prefetch.sleepInterval)

		log.Printf("runPeriodicPrefetch after sleep")

		keys := prefetch.cacheKeyToQuestion.Keys()
		log.Printf("runPeriodicPrefetch keys length = %v", len(keys))

		now := time.Now()
		expiredPrefetchCacheEntries := 0

		for _, key := range keys {
			cacheKey := key.(string)
			value, ok := prefetch.cacheKeyToQuestion.Get(cacheKey)
			if ok {
				entry := value.(*prefetchCacheEntry)

				if entry.expired(now) {
					prefetch.cacheKeyToQuestion.Remove(cacheKey)
					expiredPrefetchCacheEntries++
				} else {
					prefetch.prefetchRequstChannel <- &prefetchRequest{
						cacheKey: cacheKey,
						question: entry.question,
					}
				}
			}
		}

		log.Printf("runPeriodicPrefetch before sleep cacheKeyToQuestion.Len = %v expiredPrefetchCacheEntries = %v", prefetch.cacheKeyToQuestion.Len(), expiredPrefetchCacheEntries)
	}
}

type prefetchRequestor interface {
	makePrefetchRequest(cacheKey string, question *dns.Question)
}

func runPrefetchRequestTask(workerNumber int, prefetchRequstChannel chan *prefetchRequest, prefetchRequestor prefetchRequestor) {
	log.Printf("runPrefetchRequestTask workerNumber = %v", workerNumber)

	for {
		prefetchRequest := <-prefetchRequstChannel

		prefetchRequestor.makePrefetchRequest(prefetchRequest.cacheKey, &prefetchRequest.question)
	}
}

func (prefetch *prefetch) start(prefetchRequestor prefetchRequestor) {
	log.Printf("prefetch.start")

	for i := 0; i < prefetch.numWorkers; i++ {
		go runPrefetchRequestTask(i, prefetch.prefetchRequstChannel, prefetchRequestor)
	}

	go prefetch.runPeriodicPrefetch()
}
