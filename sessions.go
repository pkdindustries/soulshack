package main

import (
	"fmt"
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

// show string of all msg contents
func (s *ChatSession) Debug() {
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
	for id, session := range sessions.sessionMap {
		log.Printf("session '%s':  messages %d, characters %d, idle: %s", id, len(session.History), session.SumMessageLengths(), time.Since(session.Last))
	}
}
func (s *ChatSession) SumMessageLengths() int {
	sum := 0
	for _, m := range s.History {
		sum += len(m.Content)
	}
	return sum
}

func (s *ChatSession) Message(ctx *ChatContext, role string, message string) *ChatSession {
	s.Stats()
	if len(s.History) == 0 {
		s.History = append(s.History, ai.ChatCompletionMessage{Role: ai.ChatMessageRoleSystem, Content: ctx.Personality.Prompt})
		s.History = append(s.History, ai.ChatCompletionMessage{Role: ai.ChatMessageRoleSystem, Content: ctx.Personality.Greeting})
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
