package core

import (
	"context"
	"sync"
)

// ChannelLock provides context-aware locking using a buffered channel
type ChannelLock struct {
	sem chan struct{}
}

// NewChannelLock creates a new channel-based lock
func NewChannelLock() *ChannelLock {
	return &ChannelLock{
		sem: make(chan struct{}, 1),
	}
}

// LockWithContext attempts to acquire the lock, respecting context cancellation
func (c *ChannelLock) LockWithContext(ctx context.Context) bool {
	select {
	case c.sem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false // Context expired before getting lock
	}
}

// Unlock releases the lock
func (c *ChannelLock) Unlock() {
	select {
	case <-c.sem:
	default:
		// Already unlocked, avoid panic
	}
}

// channelLocks stores a lock for each channel to serialize message processing
var channelLocks sync.Map

// GetChannelLock returns the lock for a given channel, creating it if needed
func GetChannelLock(channel string) *ChannelLock {
	if lock, ok := channelLocks.Load(channel); ok {
		return lock.(*ChannelLock)
	}

	// Create new lock for this channel
	newLock := NewChannelLock()
	actual, _ := channelLocks.LoadOrStore(channel, newLock)
	return actual.(*ChannelLock)
}
