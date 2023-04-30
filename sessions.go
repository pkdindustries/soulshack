package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

var sessions = SessionMap{
	sessionMap: make(map[string]*Sessions),
	mu:         sync.RWMutex{},
}

type SessionMap struct {
	sessionMap map[string]*Sessions
	mu         sync.RWMutex
}

type Sessions struct {
	Config     *SessionConfig
	history    []ai.ChatCompletionMessage
	Last       time.Time
	mu         sync.RWMutex
	Name       string
	Totalchars int
	Stash      map[string]string
}

func (s *Sessions) GetHistory() []ai.ChatCompletionMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history := make([]ai.ChatCompletionMessage, len(s.history))
	copy(history, s.history)

	return history
}

func (s *Sessions) AddMessage(conf *Personality, role string, message string) {
	s.mu.Lock()

	if len(s.history) == 0 {
		s.history = append(s.history, ai.ChatCompletionMessage{
			Role:    ai.ChatMessageRoleSystem,
			Content: conf.Prompt + ReactPrompt(&ReactConfig{Actions: actionRegistry})},
		)
		s.Totalchars += len(conf.Prompt)
	}

	s.history = append(s.history, ai.ChatCompletionMessage{Role: role, Content: message})
	s.Totalchars += len(message)
	s.Last = time.Now()

	s.trim()
	s.mu.Unlock()

}

// contining the no alloc tradition to mock python users
func (s *Sessions) trim() {
	if len(s.history) > s.Config.MaxHistory {
		rm := len(s.history) - s.Config.MaxHistory
		for i := 1; i <= s.Config.MaxHistory; i++ {
			s.history[i] = s.history[i+rm-1]
		}
		s.history = s.history[:s.Config.MaxHistory+1]
	}
}

func (s *Sessions) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = s.history[:0]
	s.Last = time.Now()
}

func (s *Sessions) Reap() bool {
	now := time.Now()
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	if sessions.sessionMap[s.Name] == nil {
		return true
	}
	if now.Sub(s.Last) > s.Config.TTL {
		delete(sessions.sessionMap, s.Name)
		return true
	}
	return false
}

func (sessions *SessionMap) Get(id string) *Sessions {

	sessions.mu.Lock()
	defer sessions.mu.Unlock()

	if v, ok := sessions.sessionMap[id]; ok {
		return v
	}

	session := &Sessions{
		Name:   id,
		Last:   time.Now(),
		Config: SessionFromViper(vip.GetViper()),
		Stash:  make(map[string]string),
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
func (s *Sessions) Debug() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, msg := range s.history {
		ds := ""
		if msg.Role == ai.ChatMessageRoleAssistant {
			ds += "< "
		} else {
			ds += "> "
		}
		ds += fmt.Sprintf("%s:%s", msg.Role, msg.Content)
		log.Println(ds)
	}
}
