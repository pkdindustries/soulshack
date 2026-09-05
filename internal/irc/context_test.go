package irc_test

import (
	"context"
	"testing"

	"github.com/alexschlessinger/pollytool/messages"
	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
	mocktest "pkdindustries/soulshack/internal/testing"
)

func newEventContext(t *testing.T, sys core.System, event *girc.Event) irc.ChatContextInterface {
	t.Helper()
	cfg := mocktest.DefaultTestConfig()
	client := girc.New(girc.Config{Server: "localhost", Port: 6667, Nick: cfg.Server.Nick, User: "bot", Name: "bot"})
	ctx, cancel := irc.NewChatContext(context.Background(), cfg, sys, client, event, nil)
	t.Cleanup(cancel)
	return ctx
}

func TestChatContextDoesNotOpenSession(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	for _, event := range []*girc.Event{
		{Command: girc.CONNECTED},
		{Command: girc.ERROR, Params: []string{"server closing"}},
		{Command: girc.PRIVMSG, Source: &girc.Source{Name: "alice"}, Params: []string{"#test", "ignored message"}},
	} {
		ctx := newEventContext(t, sys, event)
		if ctx.GetSession() != nil {
			t.Fatal("event construction acquired a session")
		}
	}
	keys, err := sys.Sessions.List(context.Background())
	if err != nil || len(keys) != 0 {
		t.Fatalf("sessions after constructing events = %v, %v", keys, err)
	}
}

func TestChatContextDistinctNickSessions(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	newContext := func(nick string) irc.ChatContextInterface {
		t.Helper()
		if !girc.IsValidNick(nick) {
			t.Fatalf("invalid fixture nick: %q", nick)
		}
		return newEventContext(t, sys, &girc.Event{
			Command: girc.PRIVMSG, Source: &girc.Source{Name: nick},
			Params: []string{mocktest.DefaultTestConfig().Server.Nick, "hello"},
		})
	}
	const secret = "private message from alice|"
	core.WithConversation(newContext("alice|"), "test", func(ctx core.ChatContextInterface) {
		if err := ctx.GetSession().AddMessage(ctx, messages.ChatMessage{Role: messages.MessageRoleUser, Content: secret}); err != nil {
			t.Fatal(err)
		}
	}, nil)
	for _, nick := range []string{"alice_", "alice|"} {
		core.WithConversation(newContext(nick), "test", func(ctx core.ChatContextInterface) {
			history, err := ctx.GetSession().GetHistory(ctx)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, msg := range history {
				found = found || msg.Content == secret
			}
			if found != (nick == "alice|") {
				t.Errorf("nick %q can read alice|'s private history: %v", nick, found)
			}
		}, nil)
	}
}

func TestChatContextUsesActualChannelForLockAndSession(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	keys := map[string]bool{}
	for _, channel := range []string{"#one", "#two"} {
		ctx := newEventContext(t, sys, &girc.Event{Command: girc.PRIVMSG, Source: &girc.Source{Name: "alice"}, Params: []string{channel, "hello"}})
		if keys[ctx.GetLockKey()] {
			t.Fatal("different channels share a lock")
		}
		keys[ctx.GetLockKey()] = true
		core.WithConversation(ctx, "test", func(turn core.ChatContextInterface) {
			name, err := turn.GetSession().GetName(turn)
			if err != nil || name != ctx.GetLockKey() {
				t.Fatalf("session name %q differs from lock key %q: %v", name, ctx.GetLockKey(), err)
			}
		}, nil)
	}
}
