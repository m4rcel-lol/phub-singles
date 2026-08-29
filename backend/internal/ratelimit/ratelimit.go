// Package ratelimit provides a small in-memory token bucket keyed by client.
//
// It is deliberately process-local: the deployment is a single backend
// container, so a shared store would add a dependency without adding value.
package ratelimit

import (
	"sync"
	"time"
)

type bucket struct {
	tokens float64
	last   time.Time
}

// Limiter refills every key's bucket at `rate` tokens per second up to `burst`.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64
	burst   float64
	ttl     time.Duration
	now     func() time.Time
}

// New builds a limiter allowing `burst` requests immediately and `rate` per
// second sustained. Idle keys are dropped after ttl.
func New(rate, burst float64, ttl time.Duration) *Limiter {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &Limiter{
		buckets: make(map[string]*bucket),
		rate:    rate,
		burst:   burst,
		ttl:     ttl,
		now:     time.Now,
	}
}

// Allow consumes one token for key and reports whether the request may proceed.
func (l *Limiter) Allow(key string) bool {
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		l.buckets[key] = &bucket{tokens: l.burst - 1, last: now}
		return true
	}

	b.tokens = min(l.burst, b.tokens+now.Sub(b.last).Seconds()*l.rate)
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Cleanup evicts buckets that have been idle for longer than the TTL. The
// server calls it on a ticker so memory cannot grow without bound.
func (l *Limiter) Cleanup() int {
	cutoff := l.now().Add(-l.ttl)

	l.mu.Lock()
	defer l.mu.Unlock()

	removed := 0
	for key, b := range l.buckets {
		if b.last.Before(cutoff) {
			delete(l.buckets, key)
			removed++
		}
	}
	return removed
}

// Size reports the number of tracked keys (used by tests and /healthz detail).
func (l *Limiter) Size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}
