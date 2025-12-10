package core

import (
	"context"
	"sync"

	"go.uber.org/zap"
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

// WithRequestLock acquires a lock for the given key and executes the onSuccess function.
// If the lock cannot be acquired within the context's deadline, onTimeout is called (if provided).
func WithRequestLock(ctx context.Context, key string, operation string, onSuccess func(), onTimeout func()) {
	lock := GetRequestLock(key)

	// Try to get logger from context, fallback to global logger
	var logger *zap.SugaredLogger
	if logCtx, ok := ctx.(interface{ GetLogger() *zap.SugaredLogger }); ok {
		logger = logCtx.GetLogger()
	} else {
		logger = GetLogger()
	}

	logger.Debugw("lock_acquiring", "channel", key, "operation", operation)
	if !lock.LockWithContext(ctx) {
		logger.Warnw("lock_timeout", "channel", key, "operation", operation)
		if onTimeout != nil {
			onTimeout()
		}
		return
	}
	logger.Debugw("lock_acquired", "channel", key, "operation", operation)
	defer func() {
		logger.Debugw("lock_released", "channel", key, "operation", operation)
		lock.Unlock()
	}()

	onSuccess()
}
