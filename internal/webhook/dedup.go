package webhook

import (
	"sync"
	"time"
)

// dedupCache tracks recently seen delivery IDs to suppress duplicate webhook
// deliveries. GitHub may retry a delivery on transient errors; without dedup
// the same event would produce duplicate Slack messages.
//
// Entries are pruned lazily on every SeenOrRecord call, so no background
// goroutine is required.
type dedupCache struct {
	mu      sync.Mutex
	entries map[string]time.Time
	ttl     time.Duration
}

func NewDedupCache(ttl time.Duration) *dedupCache {
	return &dedupCache{
		entries: make(map[string]time.Time),
		ttl:     ttl,
	}
}

// SeenOrRecord returns true if id was already recorded within the TTL window,
// meaning the delivery is a duplicate and should be skipped. If id is new it
// is recorded and false is returned. Expired entries are pruned on each call.
func (d *dedupCache) SeenOrRecord(id string) bool {
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()

	// Lazy GC: evict expired entries.
	for k, exp := range d.entries {
		if now.After(exp) {
			delete(d.entries, k)
		}
	}

	if _, seen := d.entries[id]; seen {
		return true
	}
	d.entries[id] = now.Add(d.ttl)
	return false
}
