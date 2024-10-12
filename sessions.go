package main

import (
	"fmt"
	"log"

	"sync"
	"time"

	ai "github.com/sashabaranov/go-openai"
)

// type SessionStore interface {
// 	Get(id string) *Session
// }

type Sessions struct {
	sync.Map
}

func (sessions *Sessions) Get(id string) *Session {
	if value, ok := sessions.Load(id); ok {
		return value.(*Session)
	}

	session := &Session{
		Name:  id,
		Last:  time.Now(),
		Stash: make(map[string]interface{}),
	}

	// start session reaper, returns when the session is gone
	go func() {
		for {
			time.Sleep(Config.SessionDuration)
			if session.Reap() {
				return
			}
		}
	}()

	sessions.Store(id, session)
	return session
}

type Session struct {
	History    []ai.ChatCompletionMessage
	Last       time.Time
	mu         sync.RWMutex
	Name       string
	TotalChars int
	Stash      map[string]any
}

func (s *Session) GetHistory() []ai.ChatCompletionMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history := make([]ai.ChatCompletionMessage, len(s.History))
	copy(history, s.History)

	return history
}

func (s *Session) AddMessage(msg ai.ChatCompletionMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.History) == 0 {
		s.addMessage(ai.ChatCompletionMessage{Role: ai.ChatMessageRoleSystem, Content: Config.Prompt})
	}

	s.addMessage(msg)
	s.trimHistory()
	if Config.Verbose {
		s.Debug()
	}
}

func (s *Session) addMessage(msg ai.ChatCompletionMessage) {
	s.History = append(s.History, msg)
	s.TotalChars += len(msg.Content)
	s.Last = time.Now()
}

func (s *Session) trimHistory() {
	if len(s.History) <= Config.MaxHistory {
		return
	}
	s.History = append(s.History[:1], s.History[len(s.History)-Config.MaxHistory:]...)

	// "messages with role 'tool' must be a response to a preceeding message with 'tool_calls'."
	// if the second oldest message is a tool, remove it
	// (the first message is the system message)
	if s.History[1].Role == ai.ChatMessageRoleTool {
		s.History = append(s.History[:1], s.History[2:]...)
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

	if _, ok := Config.Sessions.Load(s.Name); !ok {
		return true
	}
	if now.Sub(s.Last) > Config.SessionDuration {
		Config.Sessions.Delete(s.Name)
		return true
	}
	return false
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
	log.Println("Total chars:", s.TotalChars)
}
