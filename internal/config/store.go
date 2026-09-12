package config

import (
	"slices"
	"sync"
)

// Store holds the configuration a running bot reads its settings from. Readers
// get a snapshot, so a turn can rely on one consistent configuration while
// another turn changes settings with /set.
type Store struct {
	mu  sync.RWMutex
	cfg *Configuration
}

// NewStore returns a Store holding cfg.
func NewStore(cfg *Configuration) *Store {
	if cfg == nil {
		cfg = &Configuration{}
	}
	return &Store{cfg: cfg}
}

// Snapshot returns an independent copy of the current configuration: changing
// the copy does not change the store.
func (s *Store) Snapshot() *Configuration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.Clone()
}

// Update applies fn to the live configuration while holding the write lock.
func (s *Store) Update(fn func(*Configuration) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return fn(s.cfg)
}

// Clone returns a deep copy, nested sections included, so a reader can hold a
// snapshot without seeing later changes.
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
