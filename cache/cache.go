package cache

import (
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
	shardIndex := shardIndex(key)
	c.shards[shardIndex].Add(key, value)
}

// Get looks up element index under key.  May return elements that are expired.
func (c *Cache) Get(key string) (Expirable, bool) {
	shardIndex := shardIndex(key)
	return c.shards[shardIndex].Get(key)
}

// Remove removes the element indexed with key.
func (c *Cache) Remove(key string) {
	shardIndex := shardIndex(key)
	c.shards[shardIndex].Remove(key)
}

// Stats is statistics for the cache.
type Stats struct {
	totalEntries   int
	minShardSize   int
	minShardIndex  int
	maxShardSize   int
	maxShardIndex  int
	numEmptyShards int
}

func (s *Stats) String() string {
	return fmt.Sprintf("totalEntries = %v minShardSize = %v minShardIndex = %v maxShardSize = %v maxShardIndex = %v numEmptyShards = %v",
		s.totalEntries, s.minShardSize, s.minShardIndex, s.maxShardSize, s.maxShardIndex, s.numEmptyShards)
}

// Stats returns Stats for the cache.
func (c *Cache) Stats() *Stats {
	stats := &Stats{}

	for i, s := range c.shards {
		shardSize := s.Len()

		stats.totalEntries += shardSize

		if (i == 0) || (shardSize < stats.minShardSize) {
			stats.minShardSize = shardSize
			stats.minShardIndex = i
		}

		if (i == 0) || (shardSize > stats.maxShardSize) {
			stats.maxShardSize = shardSize
			stats.maxShardIndex = i
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

type shard struct {
	items map[string]Expirable
	mu    sync.RWMutex
}

func newShard() *shard {
	return &shard{
		items: make(map[string]Expirable),
	}
}

func (s *shard) Add(key string, value Expirable) {
	s.mu.Lock()
	s.items[key] = value
	s.mu.Unlock()
}

func (s *shard) Get(key string) (Expirable, bool) {
	s.mu.RLock()
	value, found := s.items[key]
	s.mu.RUnlock()
	return value, found
}

func (s *shard) Remove(key string) {
	s.mu.Lock()
	delete(s.items, key)
	s.mu.Unlock()
}

func (s *shard) Len() int {
	s.mu.RLock()
	len := len(s.items)
	s.mu.RUnlock()
	return len
}

func (s *shard) Expire() {
	var expiredKeys []string

	s.mu.Lock()

	now := time.Now()

	for key, value := range s.items {
		if value.Expired(now) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		delete(s.items, key)
	}

	s.mu.Unlock()
}
