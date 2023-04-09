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
	mu:         sync.Mutex{},
}

type Chats struct {
	sessionMap map[string]*ChatSession
	mu         sync.Mutex
}

type ChatSession struct {
	Name    string
	History []ai.ChatCompletionMessage
	mu      sync.Mutex
	Last    time.Time
}

// show string of all msg contents
func (s *ChatSession) Debug() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, msg := range s.History {
		ds := ""
		if msg.Role == ai.ChatMessageRoleAssistant {
			ds += "< "
		} else {
			ds += "> "
		}
		ds += fmt.Sprint(msg.Role) + ": " + msg.Content
		log.Println(ds)
	}
}

// pretty print sessions
func (s *ChatSession) Stats() {
	s.mu.Lock()
	defer s.mu.Unlock()
	log.Printf("session '%s':  messages %d, characters %d, idle: %s", s.Name, len(s.History), s.sumMessageLengths(), time.Since(s.Last))
}

func (s *ChatSession) sumMessageLengths() int {
	sum := 0
	for _, m := range s.History {
		sum += len(m.Content)
	}
	return sum
}

func (s *ChatSession) Message(ctx *ChatContext, role string, message string) *ChatSession {
	s.Stats()
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.History) == 0 {
		s.History = append(s.History, ai.ChatCompletionMessage{Role: ai.ChatMessageRoleSystem, Content: ctx.Personality.Prompt})
	}

	s.History = append(s.History, ai.ChatCompletionMessage{Role: role, Content: message})
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

func (chats *Chats) reap() {
	chats.mu.Lock()
	defer chats.mu.Unlock()

	now := time.Now()
	for id, s := range chats.sessionMap {
		if now.Sub(s.Last) > vip.GetDuration("session") {
			log.Printf("expired session: %s", id)
			delete(chats.sessionMap, id)
		} else if len(s.History) > vip.GetInt("history") {
			log.Printf("trimmed session: %s", id)
			s.mu.Lock()
			s.History = append(s.History[:1], s.History[len(s.History)-vip.GetInt("history"):]...)
			s.mu.Unlock()
		}
	}
}

func (chats *Chats) Get(id string) *ChatSession {
	chats.reap()
	chats.mu.Lock()
	defer chats.mu.Unlock()

	if v, ok := chats.sessionMap[id]; ok {
		return v
	}

	log.Println("creating new session for", id)
	session := &ChatSession{
		Name: id,
		Last: time.Now(),
	}
	chats.sessionMap[id] = session
	return session
}
