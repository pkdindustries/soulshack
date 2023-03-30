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
	Client    *girc.Client
	Event     *girc.Event
	Tokens    []string
	IsValid   bool
	Private   bool
	Addressed bool
	Command   string
	Session   *chatSession
	Reply     func(event girc.Event, message string)
}

func createChatContext(c *girc.Client, e *girc.Event) (*chatContext, context.CancelFunc) {
	timedctx, cancel := context.WithTimeout(context.Background(), vip.GetDuration("timeout"))

	ctx := &chatContext{
		Context:   timedctx,
		Client:    c,
		Event:     e,
		Addressed: strings.HasPrefix(e.Last(), c.GetNick()),
		Private:   !strings.HasPrefix(e.Params[0], "#"),
		Tokens:    strings.Fields(e.Last()),
	}

	if ctx.Addressed {
		ctx.Tokens = ctx.Tokens[1:]
	}

	if e.Source == nil {
		e.Source = &girc.Source{
			Name: vip.GetString("channel"),
		}
	}

	ctx.IsValid = ctx.Addressed || ctx.Private && len(ctx.Tokens) > 0
	ctx.Command = strings.ToLower(ctx.Tokens[0])

	key := e.Params[0]
	if !girc.IsValidChannel(key) {
		key = e.Source.Name
	}
	ctx.Session = getSession(key)

	ctx.Reply = func(event girc.Event, message string) {
		c.Cmd.Reply(event, message)
	}

	return ctx, cancel
}

func getFromContext(ctx *chatContext) (bool, *girc.Client, *girc.Event, *chatSession, []string) {
	return ctx.IsValid, ctx.Client, ctx.Event, ctx.Session, ctx.Tokens
}

type chatSession struct {
	Name    string
	History []ai.ChatCompletionMessage
	Last    time.Time
}

var sessions = make(map[string]*chatSession)

func (s *chatSession) addMessage(role, message, name string) *chatSession {
	s.History = append(s.History, ai.ChatCompletionMessage{Role: role, Content: message, Name: name})
	s.Last = time.Now()
	printSessions()
	return s

}

func (s *chatSession) Clear() {
	delete(sessions, s.Name)
}

func (s *chatSession) Expire() {
	if time.Since(s.Last) > vip.GetDuration("session") {
		s.Clear()
	}
}

func getSession(id string) *chatSession {
	if session, ok := sessions[id]; ok {
		session.Expire()
		return session
	}

	log.Println("creating new session for", id)
	newSession := &chatSession{}
	newSession.addMessage(ai.ChatMessageRoleAssistant, vip.GetString("prompt"), "")
	sessions[id] = newSession
	return newSession
}

// pretty print sessions
func printSessions() {
	for id, session := range sessions {
		log.Printf("session %s: %d", id, len(session.History))
	}
}
