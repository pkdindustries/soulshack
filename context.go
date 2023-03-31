package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/lrstanley/girc"
	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

type chatContext struct {
	context.Context
	Client  *girc.Client
	Event   *girc.Event
	Args    []string
	Session *chatSession
}

func (s *chatContext) isAddressed() bool {
	return strings.HasPrefix(s.Event.Last(), s.Client.GetNick())
}
func createChatContext(c *girc.Client, e *girc.Event) (*chatContext, context.CancelFunc) {
	timedctx, cancel := context.WithTimeout(context.Background(), vip.GetDuration("timeout"))

	ctx := &chatContext{
		Context: timedctx,
		Client:  c,
		Event:   e,
		Args:    strings.Fields(e.Last()),
	}

	if ctx.isAddressed() {
		ctx.Args = ctx.Args[1:]
	}

	if e.Source == nil {
		e.Source = &girc.Source{
			Name: vip.GetString("channel"),
		}
	}

	key := e.Params[0]
	if !girc.IsValidChannel(key) {
		key = e.Source.Name
	}
	ctx.Session = sessions.Get(key)

	return ctx, cancel
}

func (c *chatContext) Reply(message string) *chatContext {
	c.Client.Cmd.Reply(*c.Event, message)
	return c
}
func (c *chatContext) isValid() bool {
	return (c.isAddressed() || c.isPrivate()) && len(c.Args) > 0
}

func (c *chatContext) isPrivate() bool {
	return !strings.HasPrefix(c.Event.Params[0], "#")
}

func (c *chatContext) getCommand() string {
	return strings.ToLower(c.Args[0])
}

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
		{Role: ai.ChatMessageRoleSystem, Content: vip.GetString("prompt")},
		{Role: ai.ChatMessageRoleSystem, Content: vip.GetString("answer")},
	}
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
			{Role: ai.ChatMessageRoleSystem, Content: vip.GetString("prompt")},
			{Role: ai.ChatMessageRoleSystem, Content: vip.GetString("answer")},
		},
		Last: time.Now(),
	}
	chats.sessionMap[id] = session
	return session
}
