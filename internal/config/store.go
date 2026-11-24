package config

import (
	"slices"
	"sync"
	"sync/atomic"
)

// Store holds the live configuration; readers get immutable snapshots.
type Store struct {
	mu  sync.Mutex // serializes updates, not snapshot readers
	cfg atomic.Pointer[Configuration]
}

// NewStore returns a Store holding cfg.
func NewStore(cfg *Configuration) *Store {
	if cfg == nil {
		cfg = &Configuration{}
	}
	s := &Store{}
	s.cfg.Store(cfg.Clone())
	return s
}

// Snapshot returns a copy of the last committed configuration. It never blocks on Update.
func (s *Store) Snapshot() *Configuration {
	return s.cfg.Load().Clone()
}

// Update applies fn to a candidate copy and publishes it if fn succeeds.
// fn must not call Update.
func (s *Store) Update(fn func(*Configuration) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	candidate := s.Snapshot()
	if err := fn(candidate); err != nil {
		return err
	}
	s.cfg.Store(candidate.Clone())
	return nil
}

// Clone returns a deep copy.
func (c *Configuration) Clone() *Configuration {
	if c == nil {
		return nil
	}
	clone := *c
	if c.Server != nil {
		section := *c.Server
		clone.Server = &section
	}
	if c.Bot != nil {
		section := *c.Bot
		section.Admins = slices.Clone(c.Bot.Admins)
		section.Tools = slices.Clone(c.Bot.Tools)
		clone.Bot = &section
	}
	if c.Model != nil {
		section := *c.Model
		clone.Model = &section
	}
	if c.Session != nil {
		section := *c.Session
		clone.Session = &section
	}
	if c.API != nil {
		section := *c.API
		clone.API = &section
	}
	return &clone
}
