package irc_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alexschlessinger/pollytool/messages"
	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/commands"
	"pkdindustries/soulshack/internal/core"
	"pkdindustries/soulshack/internal/irc"
	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestIsAdminUsesMasksAddedAtRuntime(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	setup := mocktest.NewTurnContext(t, sys, "#test").WithAdmin(true).
		WithArgs("/admins", "add", "alex!*@trusted.example")
	command := &commands.AdminCommand{}
	command.Execute(setup.Turn())
	if admins := sys.GetConfig().Bot.Admins; len(admins) != 1 || admins[0] != "alex!*@trusted.example" {
		t.Fatalf("wildcard admin was not added: %v; replies: %v", admins, setup.Replies)
	}
	for _, tt := range []struct {
		nick, host string
		want       bool
	}{
		{"alex", "trusted.example", true},
		{"Alex", "TRUSTED.example", true},
		{"other", "trusted.example", false},
		{"alex", "untrusted.example", false},
	} {
		ctx := newEventContext(t, sys, &girc.Event{
			Command: girc.PRIVMSG,
			Source:  &girc.Source{Name: tt.nick, Ident: "~alex", Host: tt.host},
			Params:  []string{"#test", "soulshack: op me"},
		})
		if got := ctx.IsAdmin(); got != tt.want {
			t.Errorf("IsAdmin(%s!~alex@%s) = %v, want %v", tt.nick, tt.host, got, tt.want)
		}
	}
}

func newEventContext(t *testing.T, sys core.System, event *girc.Event) irc.ChatContextInterface {
	t.Helper()
	cfg := sys.GetConfig()
	client := girc.New(girc.Config{Server: "localhost", Port: 6667, Nick: cfg.Server.Nick, User: "bot", Name: "bot"})
	ctx, cancel := irc.NewChatContext(context.Background(), sys, client, event, nil)
	t.Cleanup(cancel)
	return ctx
}

func TestChatContextDoesNotOpenConversation(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	for _, event := range []*girc.Event{
		{Command: girc.CONNECTED},
		{Command: girc.ERROR, Params: []string{"server closing"}},
		{Command: girc.PRIVMSG, Source: &girc.Source{Name: "alice"}, Params: []string{"#test", "ignored message"}},
	} {
		newEventContext(t, sys, event)
	}
	if keys := sys.Memory.Keys(); len(keys) != 0 {
		t.Fatalf("conversations after constructing events = %v", keys)
	}
}

func TestChatContextDistinctNickConversations(t *testing.T) {
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
	core.WithConversation(newContext("alice|"), "test", func(turn *core.Turn) {
		turn.Conversation.Append([]messages.ChatMessage{{Role: messages.MessageRoleUser, Content: secret}})
	}, nil)
	for _, nick := range []string{"alice_", "alice|"} {
		core.WithConversation(newContext(nick), "test", func(turn *core.Turn) {
			found := false
			for _, msg := range turn.Conversation.Messages() {
				found = found || msg.Content == secret
			}
			if found != (nick == "alice|") {
				t.Errorf("nick %q can read alice|'s private history: %v", nick, found)
			}
		}, nil)
	}
}

func TestChatContextUsesActualChannelForConversation(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	keys := map[string]bool{}
	for _, channel := range []string{"#one", "#two"} {
		ctx := newEventContext(t, sys, &girc.Event{Command: girc.PRIVMSG, Source: &girc.Source{Name: "alice"}, Params: []string{channel, "hello"}})
		if keys[ctx.GetConversationKey()] {
			t.Fatal("different channels share a conversation")
		}
		keys[ctx.GetConversationKey()] = true
		core.WithConversation(ctx, "test", func(turn *core.Turn) {
			turn.Conversation.Append([]messages.ChatMessage{{Role: messages.MessageRoleUser, Content: channel + " history"}})
		}, nil)
	}

	// Each channel's turn wrote to its own conversation.
	for _, channel := range []string{"#one", "#two"} {
		history := sys.Conversation(t, channel).Messages()
		if len(history) != 1 || history[0].Content != channel+" history" {
			t.Fatalf("channel %s conversation = %+v", channel, history)
		}
	}
}

// Tools find their chat context through the turn they run in, including when
// polly derives a context for the tool call.
func TestChatContextIsReachableFromDerivedContexts(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	ctx := newEventContext(t, sys, &girc.Event{Command: girc.PRIVMSG, Source: &girc.Source{Name: "alice"}, Params: []string{"#test", "hello"}})

	derived, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	found, err := irc.GetIRCContext(derived)
	if err != nil {
		t.Fatalf("derived context lost the chat context: %v", err)
	}
	if found != ctx {
		t.Fatalf("lookup returned %v, want the turn's chat context", found)
	}
	if _, err := irc.GetIRCContext(context.Background()); err == nil {
		t.Fatal("a context with no chat context resolved one")
	}
}

func TestChatContextChunkWriterUsesChunkMax(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	ctx := newEventContext(t, sys, &girc.Event{Command: girc.PRIVMSG, Source: &girc.Source{Name: "alice"}, Params: []string{"#test", "hello"}})

	output := make(chan string, 8)
	writer := ctx.NewChunkWriter(output)
	writer.Write(strings.Repeat("word ", 200))
	writer.Flush()
	close(output)

	got := 0
	for chunk := range output {
		got++
		if len(chunk) > sys.GetConfig().Session.ChunkMax {
			t.Fatalf("chunk of %d bytes exceeds chunkmax", len(chunk))
		}
	}
	if got < 2 {
		t.Fatalf("long output produced %d messages, want it split", got)
	}
}

// Work handed to a child agent outlives the event that asked for it: the
// event's deadline must not be the child's.
func TestBackgroundOutlivesTheEventButNotTheBot(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	bot, shutdown := context.WithCancel(context.Background())
	defer shutdown()

	event := &girc.Event{Command: girc.PRIVMSG, Source: &girc.Source{Name: "alice"}, Params: []string{"#test", "testbot: go look that up"}}
	chat, cancel := irc.NewChatContext(bot, sys, mocktest.NewMockIRCClient(), event, nil)

	background, release := chat.Background(time.Minute)
	defer release()

	// The event handler returns, which is what ends the turn.
	cancel()
	if chat.Err() == nil {
		t.Fatal("the event's context outlived its handler")
	}
	if err := background.Err(); err != nil {
		t.Fatalf("background work ended with the event it came from: %v", err)
	}
	if background.GetConversationKey() != chat.GetConversationKey() {
		t.Fatalf("background work lost its conversation: %q", background.GetConversationKey())
	}

	// Shutting the bot down is what does end it.
	shutdown()
	select {
	case <-background.Done():
	case <-time.After(time.Second):
		t.Fatal("background work outlived the bot")
	}
}

// A background context is still the context its own tools resolve, so a
// child's tools find the chat they belong to.
func TestBackgroundResolvesItsOwnChatContext(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	event := &girc.Event{Command: girc.PRIVMSG, Source: &girc.Source{Name: "alice"}, Params: []string{"#test", "hello"}}
	chat, cancel := irc.NewChatContext(context.Background(), sys, mocktest.NewMockIRCClient(), event, nil)
	defer cancel()

	background, release := chat.Background(time.Minute)
	defer release()

	found, err := irc.GetIRCContext(background)
	if err != nil {
		t.Fatal(err)
	}
	if found != background {
		t.Fatal("background work resolved a chat context other than its own")
	}
}
