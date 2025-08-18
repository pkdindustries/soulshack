package main

import (
	"log"
	"sync"
	"time"

	"github.com/alexschlessinger/pollytool/messages"
	"slices"
)

type Session interface {
	GetHistory() []messages.ChatMessage
	AddMessage(messages.ChatMessage)
	Clear()
}

type SessionStore interface {
	Get(string) Session
	Range(func(key, value any) bool)
	Expire()
}

type SyncMapSessionStore struct {
	sync.Map
	config *Configuration
}

type LocalSession struct {
	history []messages.ChatMessage
	config  *Configuration
	last    time.Time
	mu      sync.RWMutex
	name    string
}

var _ SessionStore = (*SyncMapSessionStore)(nil)
var _ Session = (*LocalSession)(nil)

func NewSessionStore(config *Configuration) SessionStore {
	log.Printf("sessionstore: %s", "syncmap")

	store := &SyncMapSessionStore{
		config: config,
	}
	// start expiry goroutine
	go func() {
		for {
			time.Sleep(config.Session.TTL)
			store.Expire()
		}
	}()
	return store
}

func (sessions *SyncMapSessionStore) Expire() {
	sessions.Range(func(key, value any) bool {
		session := value.(*LocalSession)
		if time.Since(session.last) > sessions.config.Session.TTL {
			log.Printf("sessionstore: expiring session '%s' last active %v ago", key, time.Since(session.last))
			sessions.Delete(key)
		}
		return true
	})
}

func (sessions *SyncMapSessionStore) Get(id string) Session {
	if value, ok := sessions.Load(id); ok {
		session := value.(*LocalSession)
		session.mu.Lock()
		session.last = time.Now()
		session.mu.Unlock()
		return session
	}

	session := &LocalSession{
		name:   id,
		last:   time.Now(),
		config: sessions.config,
	}
	session.Clear()
	sessions.Store(id, session)
	return session
}

func (s *LocalSession) GetHistory() []messages.ChatMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history := make([]messages.ChatMessage, len(s.history))
	copy(history, s.history)

	return history
}

func (s *LocalSession) AddMessage(msg messages.ChatMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.history = append(s.history, msg)
	s.last = time.Now()
	s.trimHistory()
}

func (s *LocalSession) trimHistory() {
	if len(s.history) <= s.config.Session.MaxHistory {
		return
	}
	s.history = append(s.history[:1], s.history[len(s.history)-s.config.Session.MaxHistory:]...)

	// "messages with role 'tool' must be a response to a preceding message with 'tool_calls'."
	// if the second oldest message is a tool, remove it
	// (the first message is the system message)
	if s.history[1].Role == messages.MessageRoleTool {
		s.history = slices.Delete(s.history, 1, 2)
	}
}

func (s *LocalSession) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = s.history[:0]
	s.history = append(s.history, messages.ChatMessage{Role: messages.MessageRoleSystem, Content: s.config.Bot.Prompt})
	s.last = time.Now()
}
