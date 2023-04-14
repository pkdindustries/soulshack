package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

var sessions = Chats{
	sessionMap: make(map[string]*ChatSession),
	mu:         sync.RWMutex{},
}

type Chats struct {
	sessionMap map[string]*ChatSession
	mu         sync.RWMutex
}

type SessionConfig struct {
	MaxTokens      int
	SessionTimeout time.Duration
	MaxHistory     int
	ClientTimeout  time.Duration
	Chunkdelay     time.Duration
	Chunkmax       int
}

type ChatSession struct {
	Config     SessionConfig
	Name       string
	History    []ai.ChatCompletionMessage
	mu         sync.RWMutex
	Last       time.Time
	Totalchars int
}

func (s *ChatSession) GetHistory() []ai.ChatCompletionMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	historyCopy := make([]ai.ChatCompletionMessage, len(s.History))
	copy(historyCopy, s.History)

	return historyCopy
}

func (s *ChatSession) Message(ctx *ChatContext, role string, message string) *ChatSession {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.History) == 0 {
		s.History = append(s.History, ai.ChatCompletionMessage{Role: ai.ChatMessageRoleSystem, Content: ctx.Personality.Prompt})
		s.Totalchars += len(ctx.Personality.Prompt)
	}

	s.History = append(s.History, ai.ChatCompletionMessage{Role: role, Content: message})
	s.Totalchars += len(message)
	s.Last = time.Now()

	s.trim()

	return s
}

// contining the no alloc tradition to mock python users
func (s *ChatSession) trim() {
	if len(s.History) > s.Config.MaxHistory {
		rm := len(s.History) - s.Config.MaxHistory
		for i := 1; i <= s.Config.MaxHistory; i++ {
			s.History[i] = s.History[i+rm-1]
		}
		s.History = s.History[:s.Config.MaxHistory+1]
	}
}

func (s *ChatSession) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.History = s.History[:0]
	s.Last = time.Now()
}

func (s *ChatSession) Reap() bool {
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

func (chats *Chats) Get(id string) *ChatSession {
	chats.mu.Lock()
	defer chats.mu.Unlock()

	if v, ok := chats.sessionMap[id]; ok {
		return v
	}

	session := &ChatSession{
		Name: id,
		Last: time.Now(),
		Config: SessionConfig{
			MaxTokens:      vip.GetInt("maxtokens"),
			SessionTimeout: vip.GetDuration("session"),
			MaxHistory:     vip.GetInt("history"),
			ClientTimeout:  vip.GetDuration("timeout"),
			Chunkdelay:     vip.GetDuration("chunkdelay"),
			Chunkmax:       vip.GetInt("chunkmax"),
		},
	}

	// start session reaper, returns when the session is gone
	go func() {
		for {
			time.Sleep(session.Config.SessionTimeout)
			if session.Reap() {
				log.Println("session reaped:", session.Name)
				return
			}
		}
	}()

	chats.sessionMap[id] = session
	return session
}

// show string of all msg contents
func (s *ChatSession) Debug() {
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
