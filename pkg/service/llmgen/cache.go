package llmgen

import (
	"container/list"
	"sync"
	"time"
)

// Cache is a small in-memory LRU with per-entry TTL. It is not persisted.
type Cache struct {
	mu       sync.Mutex
	max      int
	entries  map[string]*list.Element
	order    *list.List
}

type cacheEntry struct {
	key     string
	value   Response
	expires time.Time
}

// NewCache builds an LRU holding at most maxEntries responses.
func NewCache(maxEntries int) *Cache {
	if maxEntries <= 0 {
		maxEntries = 512
	}
	return &Cache{
		max:     maxEntries,
		entries: make(map[string]*list.Element, maxEntries),
		order:   list.New(),
	}
}

// Get returns a cached response when it exists and has not expired.
func (c *Cache) Get(key string) (Response, bool) {
	if c == nil {
		return Response{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.entries[key]
	if !ok {
		return Response{}, false
	}
	entry := el.Value.(*cacheEntry)
	if !entry.expires.IsZero() && time.Now().After(entry.expires) {
		c.order.Remove(el)
		delete(c.entries, key)
		return Response{}, false
	}
	c.order.MoveToFront(el)
	return entry.value, true
}

// Set stores a response for the given TTL. ttl <= 0 stores it without expiry.
func (c *Cache) Set(key string, r Response, ttl time.Duration) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	expires := time.Time{}
	if ttl > 0 {
		expires = time.Now().Add(ttl)
	}
	if el, ok := c.entries[key]; ok {
		entry := el.Value.(*cacheEntry)
		entry.value = r
		entry.expires = expires
		c.order.MoveToFront(el)
		return
	}
	el := c.order.PushFront(&cacheEntry{key: key, value: r, expires: expires})
	c.entries[key] = el
	for c.order.Len() > c.max {
		back := c.order.Back()
		if back == nil {
			break
		}
		c.order.Remove(back)
		delete(c.entries, back.Value.(*cacheEntry).key)
	}
}
