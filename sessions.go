package main

import (
	"fmt"
	"log"

	"sync"
	"time"

	ai "github.com/sashabaranov/go-openai"
)

var Sessions = SessionMap{
	sessionMap: make(map[string]*Session),
	mu:         sync.RWMutex{},
}

const RoleSystem = "system"
const RoleUser = "user"
const RoleAssistant = "assistant"

type SessionMap struct {
	sessionMap map[string]*Session
	mu         sync.RWMutex
}

type Session struct {
	Config     *Config
	History    []ai.ChatCompletionMessage
	Last       time.Time
	mu         sync.RWMutex
	Name       string
	Totalchars int
	Stash      map[string]any
}

func (s *Session) GetHistory() []ai.ChatCompletionMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history := make([]ai.ChatCompletionMessage, len(s.History))
	copy(history, s.History)

	return history
}

func (s *Session) AddMessage(role string, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.History) == 0 {
		s.addMessage(RoleSystem, s.Config.Prompt)
		s.Totalchars += len(s.Config.Prompt)
	}

	s.addMessage(role, message)
	s.trimHistory()
	if s.Config.Verbose {
		s.Debug()
	}
}

func (s *Session) addMessage(role string, message string) {
	s.History = append(s.History, ai.ChatCompletionMessage{Role: role, Content: message})
	s.Totalchars += len(message)
	s.Last = time.Now()
}

func (s *Session) trimHistory() {
	if len(s.History) <= s.Config.MaxHistory {
		return
	}
	s.History = append(s.History[:1], s.History[len(s.History)-s.Config.MaxHistory:]...)
}

func (s *Session) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Config = LoadConfig()
	s.History = s.History[:0]
	s.Last = time.Now()
}

func (s *Session) Reap() bool {
	now := time.Now()
	Sessions.mu.Lock()
	defer Sessions.mu.Unlock()
	if Sessions.sessionMap[s.Name] == nil {
		return true
	}
	if now.Sub(s.Last) > s.Config.TTL {
		delete(Sessions.sessionMap, s.Name)
		return true
	}
	return false
}

func (sessions *SessionMap) Get(id string, config *Config) *Session {
	sessions.mu.Lock()
	defer sessions.mu.Unlock()

	if v, ok := sessions.sessionMap[id]; ok {
		return v
	}

	session := &Session{
		Name:   id,
		Last:   time.Now(),
		Config: config,
		Stash:  make(map[string]any),
	}

	// start session reaper, returns when the session is gone
	go func() {
		for {
			time.Sleep(session.Config.TTL)
			if session.Reap() {
				return
			}
		}
	}()

	sessions.sessionMap[id] = session
	return session
}

// show string of all msg contents
func (s *Session) Debug() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, msg := range s.History {
		ds := ""
		ds += fmt.Sprintf("%s:%s", msg.Role, msg.Content)
		log.Println(ds)
	}
}
