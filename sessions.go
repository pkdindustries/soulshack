package main

import (
	"log"
	"sync"
	"time"

	ai "github.com/sashabaranov/go-openai"
)

type Session interface {
	GetHistory() []ai.ChatCompletionMessage
	AddMessage(ai.ChatCompletionMessage)
	Clear()
}

type SessionStore interface {
	Get(string) Session
	Range(func(key, value interface{}) bool)
	Expire()
}

type SyncMapSessionStore struct {
	sync.Map
	config *Configuration
}

type LocalSession struct {
	history []ai.ChatCompletionMessage
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
	sessions.Range(func(key, value interface{}) bool {
		session := value.(*LocalSession)
		if time.Since(session.last) > sessions.config.Session.TTL {
			log.Printf("syncmapsessionstore: %s expired after %f seconds", key, sessions.config.Session.TTL.Seconds())
			sessions.Delete(key)
		}
		return true
	})
}

func (sessions *SyncMapSessionStore) Get(id string) Session {
	if value, ok := sessions.Load(id); ok {
		return value.(*LocalSession)
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

func (s *LocalSession) GetHistory() []ai.ChatCompletionMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history := make([]ai.ChatCompletionMessage, len(s.history))
	copy(history, s.history)

	return history
}

func (s *LocalSession) AddMessage(msg ai.ChatCompletionMessage) {
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
	if s.history[1].Role == ai.ChatMessageRoleTool {
		s.history = append(s.history[:1], s.history[2:]...)
	}
}

func (s *LocalSession) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = s.history[:0]
	s.history = append(s.history, ai.ChatCompletionMessage{Role: ai.ChatMessageRoleSystem, Content: s.config.Bot.Prompt})
	s.last = time.Now()
}
