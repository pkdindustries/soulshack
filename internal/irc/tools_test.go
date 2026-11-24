package irc

import (
	"context"
	"strings"
	"testing"

	"github.com/alexschlessinger/pollytool/tools"
	"pkdindustries/soulshack/internal/core"
	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestRegisterIRCToolsAreOptIn(t *testing.T) {
	registry := tools.NewToolRegistry([]tools.Tool{}, tools.WithUnsafeNoSandbox())
	RegisterIRCTools(registry)

	if got := registry.All(); len(got) != 0 {
		t.Fatalf("expected no IRC tools active by default, got %d", len(got))
	}

	for _, name := range []string{
		"irc__op", "irc__kick", "irc__ban", "irc__topic", "irc__action",
		"irc__mode_set", "irc__mode_query", "irc__invite", "irc__names", "irc__whois",
	} {
		if _, err := registry.LoadToolAuto(name); err != nil {
			t.Fatalf("LoadToolAuto(%s) failed: %v", name, err)
		}
		if _, ok := registry.Get(name); !ok {
			t.Errorf("registry does not offer %s after opt-in load", name)
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

func TestOpToolAdminOnly(t *testing.T) {
	for _, tt := range []struct {
		name                                 string
		admin, opped, grant, canceled, empty bool
		users                                []string
		wantMode, denial                     string
	}{
		{name: "admin may op self", admin: true, opped: true, grant: true, wantMode: "+o"},
		{name: "admin may op anyone", admin: true, opped: true, grant: true, users: []string{"alice", "bob"}, wantMode: "+o"},
		{name: "admin may deop anyone", admin: true, opped: true, users: []string{"alice", "bob"}, wantMode: "-o"},
		{name: "nonadmin cannot op self", opped: true, grant: true, denial: "not authorized"},
		{name: "nonadmin cannot deop self", opped: true, denial: "not authorized"},
		{name: "nonadmin cannot op others", opped: true, grant: true, users: []string{"bob"}, denial: "not authorized"},
		{name: "empty admins deny even when IsAdmin is true", admin: true, empty: true, opped: true, grant: true, denial: "not authorized"},
		{name: "bot needs ops", admin: true, grant: true, denial: "Bot does not have operator status"},
		{name: "canceled request", admin: true, opped: true, grant: true, canceled: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := mocktest.DefaultTestConfig()
			if !tt.empty {
				cfg.Bot.Admins = []string{"owner!user@trusted.example"}
			}
			users := tt.users
			if users == nil {
				users = []string{"alice"}
			}
			chat := &modeRecordingContext{MockChatContext: mocktest.NewMockContext().WithConfig(cfg).WithAdmin(tt.admin).WithSource("alice")}
			chat.ChannelUsers[cfg.Server.Channel] = []core.ChannelUser{{Nick: chat.GetBotNick(), IsOp: tt.opped}}
			ctx, cancel := context.WithCancel(core.WithChat(chat, chat))
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

func TestChannelModToolsRequireConfiguredAdmins(t *testing.T) {
	cases := []struct {
		name string
		tool tools.Tool
		args map[string]any
	}{
		{"irc__op", newIrcOpTool(), map[string]any{"users": []string{"bob"}, "grant": true}},
		{"irc__kick", newIrcKickTool(), map[string]any{"users": []string{"bob"}, "reason": "bye"}},
		{"irc__ban", newIrcBanTool(), map[string]any{"target": "bob", "ban": true}},
		{"irc__topic", newIrcTopicTool(), map[string]any{"topic": "new topic"}},
		{"irc__mode_set", newIrcModeSetTool(), map[string]any{"modes": "+m"}},
		{"irc__invite", newIrcInviteTool(), map[string]any{"users": []string{"bob"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := mocktest.DefaultTestConfig() // empty Admins
			chat := mocktest.NewMockContext().WithConfig(cfg).WithAdmin(true).WithSource("alice")
			chat.ChannelUsers[cfg.Server.Channel] = []core.ChannelUser{{Nick: chat.GetBotNick(), IsOp: true}}
			result, err := tc.tool.Execute(core.WithChat(chat, chat), tc.args)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(result, "not authorized") {
				t.Fatalf("%s with empty admins returned %q, want denial", tc.name, result)
			}
		})
	}
}

func TestPublicOpsDoNotAllowOtherAdminTools(t *testing.T) {
	chat := mocktest.NewMockContext().WithConfig(mocktest.DefaultTestConfig()).WithAdmin(false)
	ctx := core.WithChat(chat, chat)
	result, err := newIrcKickTool().Execute(ctx, map[string]any{"users": []string{"alice"}, "reason": "test"})
	if err != nil || result != "You are not authorized to use this tool" {
		t.Fatalf("expected admin restriction, got %q, %v", result, err)
	}
}
