package behaviors

import (
	"strings"
	"testing"

	"github.com/alexschlessinger/pollytool/messages"
	"github.com/lrstanley/girc"

	"pkdindustries/soulshack/internal/core"
	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestOpBehaviorCheck(t *testing.T) {
	behavior := &OpBehavior{}
	channelModes := map[string]string{"CHANMODES": "beI,k,lfj,imnpst"}

	tests := []struct {
		name     string
		params   []string
		nick     string
		options  map[string]string
		disabled bool
		want     bool
	}{
		{name: "op", params: []string{"#test", "+o", "soulshack"}, want: true},
		{name: "deop", params: []string{"#test", "-o", "soulshack"}, want: true},
		{name: "disabled", params: []string{"#test", "+o", "soulshack"}, disabled: true},
		{name: "voice", params: []string{"#test", "+v", "soulshack"}},
		{name: "another user", params: []string{"#test", "+o", "someoneelse"}},
		{name: "mixed modes", params: []string{"#test", "+vo", "otheruser", "soulshack"}, want: true},
		{name: "advertised owner prefix", params: []string{"#test", "+qo", "chanowner", "soulshack"}, options: map[string]string{"PREFIX": "(qaohv)~&@%+"}, want: true},
		{name: "advertised extra prefix", params: []string{"#test", "+Yo", "oper", "soulshack"}, options: map[string]string{"PREFIX": "(Yov)!@+"}, want: true},
		{name: "join throttle before op", params: []string{"#test", "+jo", "3:10", "soulshack"}, options: channelModes, want: true},
		{name: "forward before deop", params: []string{"#test", "+f-o", "#overflow", "soulshack"}, options: channelModes, want: true},
		{name: "unset throttle takes no argument", params: []string{"#test", "-j+o", "soulshack"}, options: channelModes, want: true},
		{name: "current nickname", params: []string{"#test", "+o", "renamedbot"}, nick: "renamedbot", want: true},
		{name: "old nickname", params: []string{"#test", "+o", "soulshack"}, nick: "renamedbot"},
		{name: "nickname case", params: []string{"#test", "+o", "SoulShack"}, want: true},
		{name: "RFC1459 brackets", params: []string{"#test", "+o", "soul{bot}"}, nick: "soul[bot]", want: true},
		{name: "RFC1459 caret", params: []string{"#test", "+o", "soul~"}, nick: "soul^", want: true},
		{name: "ASCII letters", params: []string{"#test", "+o", "SoulShack"}, options: map[string]string{"CASEMAPPING": "ascii"}, want: true},
		{name: "ASCII brackets are distinct", params: []string{"#test", "+o", "soul{bot}"}, nick: "soul[bot]", options: map[string]string{"CASEMAPPING": "ascii"}},
		{name: "strict RFC1459 brackets", params: []string{"#test", "+o", "soul{bot}"}, nick: "soul[bot]", options: map[string]string{"CASEMAPPING": "rfc1459-strict"}, want: true},
		{name: "strict RFC1459 carets are distinct", params: []string{"#test", "+o", "soul~"}, nick: "soul^", options: map[string]string{"CASEMAPPING": "rfc1459-strict"}},
		{name: "strict RFC1459 caret matches itself", params: []string{"#test", "+o", "soul^"}, nick: "soul^", options: map[string]string{"CASEMAPPING": "rfc1459-strict"}, want: true},
		{name: "legacy strict RFC1459 spelling", params: []string{"#test", "+o", "soul~"}, nick: "soul^", options: map[string]string{"CASEMAPPING": "strict-rfc1459"}},
		{name: "missing parameters"},
		{name: "missing nickname", params: []string{"#test", "+o"}},
		{name: "not a channel", params: []string{"soulshack", "+o", "soulshack"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := mocktest.DefaultTestConfig()
			cfg.Bot.OpWatcher = !tt.disabled
			ctx := mocktest.NewMockContext().WithConfig(cfg)
			ctx.ServerOptions = tt.options
			if tt.nick != "" {
				ctx.BotNick = tt.nick
			}
			event := &girc.Event{Command: girc.MODE, Params: tt.params}
			if got := behavior.Check(ctx, event); got != tt.want {
				t.Fatalf("OpBehavior.Check(%v) = %v, want %v", tt.params, got, tt.want)
			}
		})
	}
}

func TestOpBehaviorForwardsEventToConversation(t *testing.T) {
	for _, params := range [][]string{
		{"#test", "+o", "soulshack"},
		{"#test", "-o", "soulshack"},
		{"#test", "+jo", "3:10", "soulshack"},
	} {
		t.Run(strings.Join(params, " "), func(t *testing.T) {
			sys := mocktest.NewMockSystem(t)
			cfg := mocktest.DefaultTestConfig()
			cfg.Bot.OpWatcher = true
			ctx := mocktest.NewMockContext().WithSystem(sys).WithConfig(cfg).WithSource("alice")
			ctx.ServerOptions = map[string]string{"CHANMODES": "beI,k,lfj,imnpst"}
			core.WithConversation(ctx, "seed", func(turn *core.Turn) {
				turn.Conversation.Append(messages.User("earlier channel message"))
			}, nil)

			registry := NewRegistry()
			registry.Register(&OpBehavior{})
			event := &girc.Event{Command: girc.MODE, Source: &girc.Source{Name: "alice"}, Params: params}
			if !registry.Process(ctx, event) {
				t.Fatal("MODE event was not handled")
			}

			request := sys.LLM.(*mocktest.MockLLM).LastRequest
			want := "(nick:alice) MODE " + strings.Join(params, " ")
			if request == nil || len(request.Messages) != 3 {
				t.Fatalf("expected system prompt, channel history, and MODE event; got %+v", request)
			}
			if request.Messages[1].Content != "earlier channel message" || request.Messages[2].Content != want || request.Messages[2].Role != messages.MessageRoleUser {
				t.Fatalf("unexpected model input: %+v", request.Messages)
			}
			if len(ctx.Replies) != 1 || ctx.Replies[0] != "Hello from mock LLM" {
				t.Fatalf("unexpected replies: %v", ctx.Replies)
			}
			core.WithConversation(ctx, "verify", func(turn *core.Turn) {
				history := turn.Conversation.Messages()
				if len(history) != 2 || history[1].Content != want {
					t.Fatalf("event was not retained in channel history: %+v", history)
				}
			}, nil)
		})
	}
}
