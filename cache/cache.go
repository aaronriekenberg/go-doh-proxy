package cache

import (
	"hash/fnv"
	"sync"
)

const numShards = 257

func hash(s string) uint32 {
	h := fnv.New32()
	h.Write([]byte(s))
	return h.Sum32()
}

func shardIndex(key string) uint32 {
	return (hash(key) % numShards)
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
	shardIndex := shardIndex(key)
	c.shards[shardIndex].Add(key, value)
}

// Get looks up element index under key.
func (c *Cache) Get(key string) (interface{}, bool) {
	shardIndex := shardIndex(key)
	return c.shards[shardIndex].Get(key)
}

// Remove removes the element indexed with key.
func (c *Cache) Remove(key string) {
	shardIndex := shardIndex(key)
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

func (s *shard) evictWithLockHeld(justAddedKey string) {
	foundKeyToEvict := false
	var keyToEvict string

	for key := range s.items {
		if key != justAddedKey {
			keyToEvict = key
			foundKeyToEvict = true
			break
		}
	}

	if foundKeyToEvict {
		delete(s.items, keyToEvict)
	}
}

func (s *shard) Add(key string, value interface{}) {
	s.mu.Lock()

	s.items[key] = value

	for (len(s.items) + 1) > s.size {
		s.evictWithLockHeld(key)
	}

	s.mu.Unlock()
}

func (s *shard) Get(key string) (interface{}, bool) {
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
