package dataquery

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type cacheEntry struct {
	data    Result
	expires time.Time
}
type flight struct {
	done chan struct{}
	data Result
	err  error
}

// Cache bounds both retained results and concurrently running queries. The
// caller supplies a key that includes current authorization and dependencies.
type Cache struct {
	mu       sync.Mutex
	entries  map[string]cacheEntry
	flights  map[string]*flight
	slots    chan struct{}
	capacity int
}

func NewCache(capacity, concurrency int) *Cache {
	return &Cache{entries: map[string]cacheEntry{}, flights: map[string]*flight{}, slots: make(chan struct{}, concurrency), capacity: capacity}
}
func (c *Cache) Do(ctx context.Context, key string, ttl time.Duration, run func(context.Context) (Result, error)) (Result, error) {
	c.mu.Lock()
	if entry, ok := c.entries[key]; ok && time.Now().Before(entry.expires) {
		c.mu.Unlock()
		result := entry.data
		result.Meta.CacheHit = true
		return result, nil
	}
	if pending := c.flights[key]; pending != nil {
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		case <-pending.done:
			result := pending.data
			result.Meta.CacheHit = true
			return result, pending.err
		}
	}
	if len(c.flights) >= 16 {
		c.mu.Unlock()
		return Result{}, ErrBudget
	}
	pending := &flight{done: make(chan struct{})}
	c.flights[key] = pending
	c.mu.Unlock()
	// Shared work has its own short lifetime. A disconnected leader must not
	// cancel a query another authorized viewer is still waiting on.
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				c.mu.Lock()
				pending.err = fmt.Errorf("Data provider failed / 数据提供器执行失败")
				delete(c.flights, key)
				close(pending.done)
				c.mu.Unlock()
			}
		}()
		work, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		select {
		case c.slots <- struct{}{}:
			defer func() { <-c.slots }()
			pending.data, pending.err = run(work)
		case <-work.Done():
			pending.err = work.Err()
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		delete(c.flights, key)
		encoded, _ := json.Marshal(pending.data)
		if pending.err == nil && ttl > 0 && c.capacity > 0 && len(encoded) <= 1<<20 {
			if len(c.entries) >= c.capacity {
				for k := range c.entries {
					delete(c.entries, k)
					break
				}
			}
			c.entries[key] = cacheEntry{data: pending.data, expires: time.Now().Add(ttl)}
		}
		close(pending.done)
	}()
	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	case <-pending.done:
		return pending.data, pending.err
	}
}
