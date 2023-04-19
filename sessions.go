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
	sessionMap: make(map[string]*Session),
	mu:         sync.RWMutex{},
}

type SessionMap struct {
	sessionMap map[string]*Session
	mu         sync.RWMutex
}

type Session struct {
	Config     *SessionConfig
	History    []ai.ChatCompletionMessage
	Last       time.Time
	mu         sync.RWMutex
	Name       string
	Totalchars int
}

func (s *Session) GetHistory() []ai.ChatCompletionMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history := make([]ai.ChatCompletionMessage, len(s.History))
	copy(history, s.History)

	return history
}

func (s *Session) AddMessage(ctx ChatContext, role string, message string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	personality := ctx.GetPersonality()
	if len(s.History) == 0 {
		s.History = append(s.History, ai.ChatCompletionMessage{Role: ai.ChatMessageRoleSystem, Content: personality.Prompt})
		s.Totalchars += len(personality.Prompt)
	}

	s.History = append(s.History, ai.ChatCompletionMessage{Role: role, Content: message})
	s.Totalchars += len(message)
	s.Last = time.Now()

	s.trim()

	return s
}

// contining the no alloc tradition to mock python users
func (s *Session) trim() {
	if len(s.History) > s.Config.MaxHistory {
		rm := len(s.History) - s.Config.MaxHistory
		for i := 1; i <= s.Config.MaxHistory; i++ {
			s.History[i] = s.History[i+rm-1]
		}
		s.History = s.History[:s.Config.MaxHistory+1]
	}
}

func (s *Session) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.History = s.History[:0]
	s.Last = time.Now()
}

func (s *Session) Reap() bool {
	now := time.Now()
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	if sessions.sessionMap[s.Name] == nil {
		return true
	}
	if now.Sub(s.Last) > s.Config.SessionTimeout {
		delete(sessions.sessionMap, s.Name)
		return true
	}
	return false
}

func (sessions *SessionMap) Get(id string) *Session {

	sessions.mu.Lock()
	defer sessions.mu.Unlock()

	if v, ok := sessions.sessionMap[id]; ok {
		return v
	}

	session := &Session{
		Name:   id,
		Last:   time.Now(),
		Config: SessionFromViper(vip.GetViper()),
	}

	// start session reaper, returns when the session is gone
	go func() {
		for {
			time.Sleep(session.Config.SessionTimeout)
			if session.Reap() {
				if false {
					log.Println("session reaped:", session.Name)
				}
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
		if msg.Role == ai.ChatMessageRoleAssistant {
			ds += "< "
		} else {
			ds += "> "
		}
		ds += fmt.Sprintf("%s:%s", msg.Role, msg.Content)
		log.Println(ds)
	}
}
