package main

import (
	"log"
	"time"

	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

var sessions = Chats{
	sessionMap: make(map[string]*ChatSession),
}

type Chats struct {
	sessionMap map[string]*ChatSession
}

type ChatSession struct {
	Name    string
	History []ai.ChatCompletionMessage
	Last    time.Time
}

func (s *ChatSession) Message(ctx *ChatContext, role string, message string) *ChatSession {
	sessionStats()
	if len(s.History) == 0 {
		s.History = append(s.History, ai.ChatCompletionMessage{Role: ai.ChatMessageRoleSystem, Content: ctx.Cfg.GetString("prompt")})
		s.History = append(s.History, ai.ChatCompletionMessage{Role: ai.ChatMessageRoleSystem, Content: ctx.Cfg.GetString("greeting")})
	}

	s.History = append(s.History, ai.ChatCompletionMessage{Role: role, Content: message})
	s.Last = time.Now()
	return s
}

func (s *ChatSession) Reset() {
	log.Printf("resetting session %s", s.Name)
	s.History = []ai.ChatCompletionMessage{}
	s.Last = time.Now()
}

func (chats *Chats) Reap() {
	now := time.Now()
	for id, s := range chats.sessionMap {
		if now.Sub(s.Last) > vip.GetDuration("session") {
			log.Printf("expired session: %s", id)
			delete(chats.sessionMap, id)
		} else if len(s.History) > vip.GetInt("history") {
			log.Printf("trimmed session: %s", id)
			s.History = append(s.History[:1], s.History[len(s.History)-vip.GetInt("history"):]...)
		}
	}
}

func (chats *Chats) Get(id string) *ChatSession {
	chats.Reap()
	if session, ok := chats.sessionMap[id]; ok {
		return session
	}

	log.Println("creating new session for", id)
	session := &ChatSession{
		Name: id,
		Last: time.Now(),
	}
	chats.sessionMap[id] = session
	return session
}
