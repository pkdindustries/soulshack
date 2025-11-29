package core

import (
	"context"
	"sync"
)

// RequestLock provides context-aware locking for serializing request processing
type RequestLock struct {
	sem chan struct{}
}

// NewRequestLock creates a new request lock
func NewRequestLock() *RequestLock {
	return &RequestLock{
		sem: make(chan struct{}, 1),
	}
}

// LockWithContext attempts to acquire the lock, respecting context cancellation
func (c *RequestLock) LockWithContext(ctx context.Context) bool {
	select {
	case c.sem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false // Context expired before getting lock
	}
}

// Unlock releases the lock
func (c *RequestLock) Unlock() {
	select {
	case <-c.sem:
	default:
		// Already unlocked, avoid panic
	}
}

// requestLocks stores a lock for each key to serialize request processing
var requestLocks sync.Map

// GetRequestLock returns the lock for a given key, creating it if needed
func GetRequestLock(key string) *RequestLock {
	if lock, ok := requestLocks.Load(key); ok {
		return lock.(*RequestLock)
	}

	// Create new lock for this key
	newLock := NewRequestLock()
	actual, _ := requestLocks.LoadOrStore(key, newLock)
	return actual.(*RequestLock)
}
