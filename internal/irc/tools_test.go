package irc

import (
	"context"
	"strings"
	"testing"

	"github.com/alexschlessinger/pollytool/tools"
	"pkdindustries/soulshack/internal/core"
	mocktest "pkdindustries/soulshack/internal/testing"
)

// A native factory alone only makes a tool loadable by name; the model is
// offered registry.All(), so the IRC tools must be registered as instances.
func TestRegisterIRCToolsOffersThemToTheModel(t *testing.T) {
	registry := tools.NewToolRegistry([]tools.Tool{}, tools.WithUnsafeNoSandbox())
	RegisterIRCTools(registry)

	offered := make(map[string]bool)
	for _, tool := range registry.All() {
		offered[tool.GetName()] = true
	}

	for _, name := range []string{
		"irc__op", "irc__kick", "irc__ban", "irc__topic", "irc__action",
		"irc__mode_set", "irc__mode_query", "irc__invite", "irc__names", "irc__whois",
	} {
		if !offered[name] {
			t.Errorf("registry.All() does not offer %s", name)
		}
	}
}

type recordedMode struct {
	target, flags string
	args          []string
}

type modeRecordingContext struct {
	*mocktest.MockChatContext
	modes []recordedMode
}

func (c *modeRecordingContext) SetMode(target, flags string, args ...string) bool {
	c.modes = append(c.modes, recordedMode{target, flags, args})
	return true
}

func TestOpToolAdminOrSelf(t *testing.T) {
	for _, tt := range []struct {
		name                                       string
		admin, opped, grant, canceled, emptyAdmins bool
		users                                      []string
		source, caseMapping, wantMode, denial      string
	}{
		{name: "self op", opped: true, grant: true, wantMode: "+o"},
		{name: "self deop", opped: true, wantMode: "-o"},
		{name: "admin may op anyone", admin: true, opped: true, grant: true, users: []string{"alice", "bob"}, wantMode: "+o"},
		{name: "admin may deop anyone", admin: true, opped: true, users: []string{"alice", "bob"}, wantMode: "-o"},
		{name: "nonadmin cannot op others", opped: true, grant: true, users: []string{"bob"}, denial: "Only configured admins"},
		{name: "nonadmin cannot deop others", opped: true, users: []string{"bob"}, denial: "Only configured admins"},
		{name: "mixed grant rejected before any modes", opped: true, grant: true, users: []string{"alice", "bob"}, denial: "Only configured admins"},
		{name: "mixed revoke rejected before any modes", opped: true, users: []string{"alice", "bob"}, denial: "Only configured admins"},
		{name: "empty admins permit self op", admin: true, emptyAdmins: true, opped: true, grant: true, wantMode: "+o"},
		{name: "empty admins permit self deop", admin: true, emptyAdmins: true, opped: true, wantMode: "-o"},
		{name: "empty admins cannot op others", admin: true, emptyAdmins: true, opped: true, grant: true, users: []string{"bob"}, denial: "Only configured admins"},
		{name: "empty admins cannot deop others", admin: true, emptyAdmins: true, opped: true, users: []string{"bob"}, denial: "Only configured admins"},
		{name: "nickname case", opped: true, grant: true, users: []string{"ALICE"}, wantMode: "+o"},
		{name: "RFC1459 nickname equivalents", opped: true, grant: true, source: "alice[", users: []string{"ALICE{"}, wantMode: "+o"},
		{name: "ASCII distinct nicknames", opped: true, grant: true, source: "alice[", users: []string{"alice{"}, caseMapping: "ascii", denial: "Only configured admins"},
		{name: "strict RFC1459 distinct nicknames", opped: true, grant: true, source: "alice^", users: []string{"alice~"}, caseMapping: "rfc1459-strict", denial: "Only configured admins"},
		{name: "bot needs ops", grant: true, denial: "Bot does not have operator status"},
		{name: "canceled request", opped: true, grant: true, canceled: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := mocktest.DefaultTestConfig()
			if !tt.emptyAdmins {
				cfg.Bot.Admins = []string{"owner!user@trusted.example"}
			}
			source := tt.source
			if source == "" {
				source = "alice"
			}
			users := tt.users
			if users == nil {
				users = []string{"alice"}
			}
			chat := &modeRecordingContext{MockChatContext: mocktest.NewMockContext().WithConfig(cfg).WithAdmin(tt.admin).WithSource(source)}
			chat.ServerOptions = map[string]string{"CASEMAPPING": tt.caseMapping}
			chat.ChannelUsers[cfg.Server.Channel] = []core.ChannelUser{{Nick: chat.GetBotNick(), IsOp: tt.opped}}
			ctx, cancel := context.WithCancel(context.WithValue(chat, kContextKey, ChatContextInterface(chat)))
			defer cancel()
			if tt.canceled {
				cancel()
			}
			result, err := newIrcOpTool().Execute(ctx, map[string]any{"users": users, "grant": tt.grant})
			if tt.canceled {
				if err != context.Canceled {
					t.Fatalf("expected cancellation, got %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if tt.denial != "" && !strings.Contains(result, tt.denial) {
				t.Fatalf("expected denial %q, got %q", tt.denial, result)
			}
			if tt.wantMode == "" {
				if len(chat.modes) != 0 {
					t.Fatalf("denied request sent modes: %+v", chat.modes)
				}
				return
			}
			if len(chat.modes) != len(users) {
				t.Fatalf("expected modes for all requested users, got %+v", chat.modes)
			}
			for i, nick := range users {
				mode := chat.modes[i]
				if mode.target != cfg.Server.Channel || mode.flags != tt.wantMode || len(mode.args) != 1 || mode.args[0] != nick {
					t.Fatalf("unexpected mode: %+v", mode)
				}
			}
		})
	}
}

func TestPublicOpsDoNotAllowOtherAdminTools(t *testing.T) {
	chat := mocktest.NewMockContext().WithConfig(mocktest.DefaultTestConfig()).WithAdmin(false)
	ctx := context.WithValue(chat, kContextKey, ChatContextInterface(chat))
	result, err := newIrcKickTool().Execute(ctx, map[string]any{"users": []string{"alice"}, "reason": "test"})
	if err != nil || result != "You are not authorized to use this tool" {
		t.Fatalf("expected admin restriction, got %q, %v", result, err)
	}
}
