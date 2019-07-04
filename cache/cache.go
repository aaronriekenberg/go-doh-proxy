package cache

import (
	"hash/fnv"
	"sync"
)

const numShards = 256

func hash(s string) uint64 {
	h := fnv.New64()
	h.Write([]byte(s))
	return h.Sum64()
}

// Cache is a cache.
type Cache struct {
	shardSize int
	shards    [numShards]*shard
}

// New returns a new cache.
func New(size int) *Cache {
	shardSize := size / numShards
	if shardSize < 4 {
		shardSize = 4
	}

	cache := &Cache{
		shardSize: shardSize,
	}
	for i := 0; i < numShards; i++ {
		cache.shards[i] = newShard(shardSize)
	}
	return cache
}

// ShardSize returns the size of each shard.
func (c *Cache) ShardSize() int {
	return c.shardSize
}

// Add adds a new element to the cache. If the element already exists it is overwritten.
func (c *Cache) Add(key string, value interface{}) {
	hash := hash(key)
	shardIndex := hash & (numShards - 1)
	c.shards[shardIndex].Add(key, value)
}

// Get looks up element index under key.
func (c *Cache) Get(key string) (interface{}, bool) {
	hash := hash(key)
	shardIndex := hash & (numShards - 1)
	return c.shards[shardIndex].Get(key)
}

// Remove removes the element indexed with key.
func (c *Cache) Remove(key string) {
	hash := hash(key)
	shardIndex := hash & (numShards - 1)
	c.shards[shardIndex].Remove(key)
}

// Len returns the number of elements in the cache.
func (c *Cache) Len() int {
	len := 0
	for _, s := range c.shards {
		len += s.Len()
	}
	return len
}

type shard struct {
	items map[string]interface{}
	size  int
	mu    sync.RWMutex
}

func newShard(size int) *shard {
	return &shard{
		items: make(map[string]interface{}),
		size:  size,
	}
}

func (s *shard) Add(key string, el interface{}) {
	if (s.Len() + 1) > s.size {
		s.Evict()
	}

	s.mu.Lock()
	s.items[key] = el
	s.mu.Unlock()
}

func (s *shard) Remove(key string) {
	s.mu.Lock()
	delete(s.items, key)
	s.mu.Unlock()
}

func (s *shard) Evict() {
	hasKey := false
	var key string

	s.mu.RLock()
	for k := range s.items {
		key = k
		hasKey = true
		break
	}
	s.mu.RUnlock()

	if !hasKey {
		return
	}

	s.Remove(key)
}

func (s *shard) Get(key string) (interface{}, bool) {
	s.mu.RLock()
	value, found := s.items[key]
	s.mu.RUnlock()
	return value, found
}

func (s *shard) Len() int {
	s.mu.RLock()
	len := len(s.items)
	s.mu.RUnlock()
	return len
}
