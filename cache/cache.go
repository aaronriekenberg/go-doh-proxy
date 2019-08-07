package cache

import (
	"container/heap"
	"fmt"
	"hash/fnv"
	"sync"
	"time"
)

const (
	numShards          = 257
	janitorTimeSeconds = 60
)

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func shardIndex(key string) uint32 {
	return (hash(key) % numShards)
}

// Expirable is an object that can expire from the cache a point in time.
type Expirable interface {
	Expired(now time.Time) bool
	ExpirationTime() time.Time
}

// Cache is a cache.
type Cache struct {
	shards [numShards]*shard
}

// New returns a new cache.
func New() *Cache {

	cache := &Cache{}

	for i := 0; i < numShards; i++ {
		cache.shards[i] = newShard()
	}

	go cache.runJanitor()

	return cache
}

// Add adds a new element to the cache. If the element already exists it is overwritten.
func (c *Cache) Add(key string, value Expirable) {
	c.shards[shardIndex(key)].Add(key, value)
}

// Get looks up element index under key.  May return elements that are expired.
func (c *Cache) Get(key string) (value Expirable, ok bool) {
	value, ok = c.shards[shardIndex(key)].Get(key)
	return
}

// Stats is statistics for the cache.
type Stats struct {
	totalEntries   int
	minShardSize   int
	maxShardSize   int
	numEmptyShards int
}

func (s *Stats) String() string {
	return fmt.Sprintf("totalEntries = %v minShardSize = %v maxShardSize = %v numEmptyShards = %v",
		s.totalEntries, s.minShardSize, s.maxShardSize, s.numEmptyShards)
}

// Stats returns Stats for the cache.
func (c *Cache) Stats() *Stats {
	stats := &Stats{}

	for i, s := range c.shards {
		shardSize := s.Len()

		stats.totalEntries += shardSize

		if (i == 0) || (shardSize < stats.minShardSize) {
			stats.minShardSize = shardSize
		}

		if (i == 0) || (shardSize > stats.maxShardSize) {
			stats.maxShardSize = shardSize
		}

		if shardSize == 0 {
			stats.numEmptyShards++
		}
	}

	return stats
}

// Len returns the number of elements in the cache.
func (c *Cache) Len() int {
	len := 0
	for _, s := range c.shards {
		len += s.Len()
	}
	return len
}

func (c *Cache) runJanitor() {
	ticker := time.NewTicker(time.Second * time.Duration(janitorTimeSeconds))
	for {
		select {
		case <-ticker.C:
			for _, s := range c.shards {
				s.Expire()
			}
		}
	}
}

type shardPriorityQueueItem struct {
	key   string
	value Expirable
}

type shardPriorityQueue []*shardPriorityQueueItem

func (pq shardPriorityQueue) Len() int { return len(pq) }

func (pq shardPriorityQueue) Less(i, j int) bool {
	return pq[i].value.ExpirationTime().Before(pq[j].value.ExpirationTime())
}

func (pq shardPriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *shardPriorityQueue) Push(x interface{}) {
	item := x.(*shardPriorityQueueItem)
	*pq = append(*pq, item)
}

func (pq *shardPriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	if n == 1 {
		*pq = nil
	} else {
		old[n-1] = nil
		*pq = old[0 : n-1]
	}
	return item
}

type shard struct {
	items         map[string]Expirable
	priorityQueue shardPriorityQueue
	mu            sync.RWMutex
}

func newShard() *shard {
	s := &shard{
		items: make(map[string]Expirable),
	}
	heap.Init(&s.priorityQueue)
	return s
}

func (s *shard) Add(key string, value Expirable) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items[key] = value

	pqItem := &shardPriorityQueueItem{
		key:   key,
		value: value,
	}
	heap.Push(&s.priorityQueue, pqItem)
}

func (s *shard) Get(key string) (value Expirable, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, ok = s.items[key]

	return
}

func (s *shard) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.items)
}

func (s *shard) Expire() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	done := false

	for (!done) && (s.priorityQueue.Len() > 0) {
		pqItem := s.priorityQueue[0]
		if pqItem.value.Expired(now) {
			heap.Pop(&s.priorityQueue)
			mapItem, ok := s.items[pqItem.key]
			// check expiration of map item.
			// priority queue may contain multiple elements for same key.
			if ok && mapItem.Expired(now) {
				delete(s.items, pqItem.key)
			}
		} else {
			done = true
		}
	}

}
