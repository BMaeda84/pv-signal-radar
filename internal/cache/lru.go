// Package cache provides a bounded, thread-safe in-memory LRU cache with TTL expiration.
// Designed to keep RAM footprint strictly bounded (< 10 MB) while shielding external APIs from redundant queries.
package cache

import (
	"container/list"
	"sync"
	"time"
)

type cacheItem struct {
	key       string
	value     interface{}
	expiresAt time.Time
	element   *list.Element
}

// LRUCache is a concurrent, bounded LRU cache with TTL.
type LRUCache struct {
	mu        sync.RWMutex
	capacity  int
	ttl       time.Duration
	items     map[string]*cacheItem
	evictList *list.List
}

// New creates an LRUCache with given capacity and TTL.
func New(capacity int, ttl time.Duration) *LRUCache {
	if capacity <= 0 {
		capacity = 500
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &LRUCache{
		capacity:  capacity,
		ttl:       ttl,
		items:     make(map[string]*cacheItem, capacity),
		evictList: list.New(),
	}
}

// Get retrieves an item from the cache. Returns false if missing or expired.
func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	// Check TTL expiration
	if time.Now().After(item.expiresAt) {
		c.removeElement(item)
		return nil, false
	}

	// Move to front (most recently used)
	c.evictList.MoveToFront(item.element)
	return item.value, true
}

// Set adds or updates an item in the cache.
func (c *LRUCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If already present, update value and expiration, move to front
	if item, exists := c.items[key]; exists {
		item.value = value
		item.expiresAt = time.Now().Add(c.ttl)
		c.evictList.MoveToFront(item.element)
		return
	}

	// If at capacity, evict least recently used (back of list)
	if len(c.items) >= c.capacity {
		c.evictOldest()
	}

	// Insert new item
	item := &cacheItem{
		key:       key,
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
	item.element = c.evictList.PushFront(item)
	c.items[key] = item
}

func (c *LRUCache) evictOldest() {
	elem := c.evictList.Back()
	if elem != nil {
		item := elem.Value.(*cacheItem)
		c.removeElement(item)
	}
}

func (c *LRUCache) removeElement(item *cacheItem) {
	c.evictList.Remove(item.element)
	delete(c.items, item.key)
}

// Len returns the current count of cached items.
func (c *LRUCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}
