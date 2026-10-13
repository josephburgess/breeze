package weather

import (
	"sync"
	"time"
)

type entry[V any] struct {
	value   V
	expires time.Time
}

type ttlCache[V any] struct {
	mu      sync.Mutex
	ttl     time.Duration
	entries map[string]entry[V]
}

func newTTLCache[V any](ttl time.Duration) *ttlCache[V] {
	return &ttlCache[V]{ttl: ttl, entries: map[string]entry[V]{}}
}

func (c *ttlCache[V]) get(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || time.Now().After(e.expires) {
		var zero V
		return zero, false
	}
	return e.value, true
}

func (c *ttlCache[V]) put(key string, value V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for k, e := range c.entries {
		if now.After(e.expires) {
			delete(c.entries, k)
		}
	}
	c.entries[key] = entry[V]{value: value, expires: now.Add(c.ttl)}
}
