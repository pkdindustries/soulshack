package main

import (
	"log"
	"time"

	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

var sessions = Chats{
	sessionMap: make(map[string]*chatSession),
}

type Chats struct {
	sessionMap map[string]*chatSession
}

type chatSession struct {
	Name    string
	History []ai.ChatCompletionMessage
	Last    time.Time
}

func (s *chatSession) addMessage(role, message string) *chatSession {
	sessionStats()
	s.History = append(s.History, ai.ChatCompletionMessage{Role: role, Content: message})
	s.Last = time.Now()
	return s

}

func (s *chatSession) Reset() {
	s.History = []ai.ChatCompletionMessage{
		{Role: ai.ChatMessageRoleSystem, Content: vip.GetString("prompt")}}
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

func (chats *Chats) Get(id string) *chatSession {
	chats.Reap()
	if session, ok := chats.sessionMap[id]; ok {
		return session
	}

	log.Println("creating new session for", id)
	session := &chatSession{
		Name: id,
		History: []ai.ChatCompletionMessage{
			{Role: ai.ChatMessageRoleSystem, Content: vip.GetString("prompt")}},
		Last: time.Now(),
	}
	chats.sessionMap[id] = session
	return session
}
