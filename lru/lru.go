package lru

import (
	"container/list"
	"sync"
)

// Lru is an LRU cache. It is safe for concurrent access.
type Lru[V any] struct {
	lock sync.RWMutex

	// MaxEntries is the maximum number of cache entries before
	// an item is evicted. Zero means no limit.
	MaxEntries int

	// OnEvicted optionally specifies a callback function to be
	// executed when an entry is purged from the cache.
	OnEvicted func(key interface{}, value V)

	ll    *list.List
	cache map[interface{}]*list.Element
}

type entry[V any] struct {
	key   interface{}
	value V
}

// New creates a new Lru.
// If maxEntries is zero, the cache has no limit, and it's assumed
// that eviction is done by the caller.
func New[V any](maxEntries int) *Lru[V] {
	return &Lru[V]{
		MaxEntries: maxEntries,
		ll:         list.New(),
		cache:      make(map[interface{}]*list.Element),
	}
}

// Add adds a value to the cache.
func (c *Lru[V]) Add(key interface{}, value V) {
	if c.cache == nil {
		c.cache = make(map[interface{}]*list.Element)
		c.ll = list.New()
	}

	c.lock.RLock()
	ee, ok := c.cache[key]
	c.lock.RUnlock()
	if ok {
		c.ll.MoveToFront(ee)
		ee.Value.(*entry[V]).value = value
		return
	}

	ele := c.ll.PushFront(&entry[V]{key, value})
	c.lock.Lock()
	c.cache[key] = ele
	c.lock.Unlock()
	if c.MaxEntries != 0 && c.ll.Len() > c.MaxEntries {
		c.RemoveOldest()
	}
}

// Get looks up a key's value from the cache.
func (c *Lru[V]) Get(key interface{}) (value V, ok bool) {
	if c.cache == nil {
		return
	}

	c.lock.RLock()
	ele, hit := c.cache[key]
	c.lock.RUnlock()
	if hit {
		c.ll.MoveToFront(ele)
		return ele.Value.(*entry[V]).value, true
	}
	return
}

// Remove removes the provided interface{} from the cache.
func (c *Lru[V]) Remove(key interface{}) {
	if c.cache == nil {
		return
	}

	c.lock.RLock()
	ele, hit := c.cache[key]
	c.lock.RUnlock()
	if hit {
		c.removeElement(ele)
	}
}

// RemoveOldest removes the oldest item from the cache.
func (c *Lru[V]) RemoveOldest() {
	if c.cache == nil {
		return
	}
	ele := c.ll.Back()
	if ele != nil {
		c.removeElement(ele)
	}
}

func (c *Lru[V]) removeElement(e *list.Element) {
	c.ll.Remove(e)
	kv := e.Value.(*entry[V])
	c.lock.Lock()
	delete(c.cache, kv.key)
	c.lock.Unlock()
	if c.OnEvicted != nil {
		c.OnEvicted(kv.key, kv.value)
	}
}

// Len returns the number of items in the cache.
func (c *Lru[V]) Len() int {
	if c.cache == nil {
		return 0
	}
	return c.ll.Len()
}

// Clear purges all stored items from the cache.
func (c *Lru[V]) Clear() {
	c.lock.Lock()
	defer c.lock.RLock()
	if c.OnEvicted != nil {
		for _, e := range c.cache {
			kv := e.Value.(*entry[V])
			c.OnEvicted(kv.key, kv.value)
		}
	}
	c.ll = nil
	c.cache = nil
}
