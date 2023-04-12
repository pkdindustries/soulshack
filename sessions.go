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
	Timer      *time.Timer
	Totalchars int
}

func (s *ChatSession) GetHistory() []ai.ChatCompletionMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	historyCopy := make([]ai.ChatCompletionMessage, len(s.History))
	copy(historyCopy, s.History)

	return historyCopy
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

func (s *ChatSession) Message(ctx *ChatContext, role string, message string) *ChatSession {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.History) == 0 {
		s.History = append(s.History, ai.ChatCompletionMessage{Role: ai.ChatMessageRoleSystem, Content: ctx.Personality.Prompt})
		s.Totalchars += len(ctx.Personality.Prompt)
	}

	s.History = append(s.History, ai.ChatCompletionMessage{Role: role, Content: message})
	s.Totalchars += len(message)

	s.trim()

	if s.Timer != nil {
		s.Timer.Stop()
	}
	s.Timer = time.AfterFunc(s.Config.SessionTimeout, func() {
		s.Reap()
	})

	s.Last = time.Now()
	return s
}

func (s *ChatSession) Reset() {
	log.Printf("resetting session %s", s.Name)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.History = []ai.ChatCompletionMessage{}
	s.Last = time.Now()
}

func (s *ChatSession) trim() {
	if len(s.History) > s.Config.MaxHistory {
		log.Printf("trimming session %s", s.Name)
		s.History = append(s.History[:1], s.History[len(s.History)-s.Config.MaxHistory:]...)
	}
}

func (s *ChatSession) Reap() {
	now := time.Now()
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	if now.Sub(s.Last) > s.Config.SessionTimeout {
		log.Printf("expired session: %s", s.Name)
		delete(sessions.sessionMap, s.Name)
	}
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

	chats.sessionMap[id] = session
	return session
}
