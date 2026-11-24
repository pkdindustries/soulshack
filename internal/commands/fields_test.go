package commands

import (
	"strings"
	"testing"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/subagents"
	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestConfigFieldsRoundTrip(t *testing.T) {
	sys := mocktest.NewMockSystem(t)

	for name, field := range configFields {
		t.Run(name, func(t *testing.T) {
			if field.setter == nil || field.getter == nil {
				t.Fatal("field is missing a setter or a getter")
			}
			before := field.getter(sys.GetConfig())
			if strings.HasSuffix(name, "key") {
				return // masked when read back: not a value to write again
			}
			err := sys.UpdateConfig(func(c *config.Configuration) error {
				return field.setter(c, before)
			})
			if err != nil {
				t.Fatalf("%s rejects its own value %q: %v", name, before, err)
			}
			if after := field.getter(sys.GetConfig()); after != before {
				t.Fatalf("%s round-trip changed %q to %q", name, before, after)
			}
		})
	}
}

func TestConfigKeysAreSorted(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	sys.Agents = subagents.NewTracker()
	keys := configKeysFor(sys)
	if len(keys) != len(configFields) {
		t.Fatalf("listed %d keys for %d fields", len(keys), len(configFields))
	}
	for i := 1; i < len(keys); i++ {
		if keys[i-1] > keys[i] {
			t.Fatalf("keys are not sorted: %v", keys)
		}
	}
}

func TestSubagentKeysHiddenWhenDisabled(t *testing.T) {
	disabled := mocktest.NewMockSystem(t)
	for _, key := range configKeysFor(disabled) {
		if strings.HasPrefix(key, "subagent") {
			t.Fatalf("disabled system exposed subagent key %q", key)
		}
	}

	ctx := mocktest.NewTurnContext(t, disabled, "test").WithAdmin(true)
	ctx.WithArgs("/get", "subagentmodel")
	(&GetCommand{}).Execute(ctx.Turn())
	if !strings.Contains(ctx.LastReply(), "Unknown key") {
		t.Fatalf("/get subagentmodel when disabled = %q, want Unknown key", ctx.LastReply())
	}

	ctx.WithArgs("/set", "subagentmodel", "ollama/small")
	(&SetCommand{}).Execute(ctx.Turn())
	if !strings.Contains(ctx.LastReply(), "Unknown key") {
		t.Fatalf("/set subagentmodel when disabled = %q, want Unknown key", ctx.LastReply())
	}

	// Enabled system exposes them and enforces positive subagenttimeout.
	enabled := mocktest.NewMockSystem(t)
	enabled.Agents = subagents.NewTracker()
	ctxOn := mocktest.NewTurnContext(t, enabled, "test").WithAdmin(true)
	ctxOn.WithArgs("/set", "subagenttimeout", "0s")
	(&SetCommand{}).Execute(ctxOn.Turn())
	if !strings.Contains(ctxOn.LastReply(), "positive duration") {
		t.Fatalf("/set subagenttimeout 0s = %q, want positive duration error", ctxOn.LastReply())
	}
	ctxOn.WithArgs("/set", "subagentmodel", "ollama/small")
	(&SetCommand{}).Execute(ctxOn.Turn())
	if got := enabled.GetConfig().Bot.SubagentModel; got != "ollama/small" {
		t.Fatalf("subagentmodel = %q, want ollama/small", got)
	}
}

// Replies name the setting, not the parser's error.
func TestConfigFieldsRejectWithExplanation(t *testing.T) {
	tests := []struct {
		key   string
		value string
		want  string
	}{
		{"addressed", "maybe", "'true' or 'false'"},
		{"maxtokens", "many", "valid integer"},
		{"maxtokens", "0", "at least 1"},
		{"maxcontext", "-5", "at least 0"},
		{"chunkmax", "-1", "at least 0"},
		{"temperature", "hot", "valid float"},
		{"temperature", "-0.5", "at least 0"},
		{"top_p", "7", "between 0 and 1"},
		{"apitimeout", "0s", "positive duration"},
		{"apitimeout", "-5m", "positive duration"},
		{"sessionduration", "10", "valid duration"},
		{"sessionduration", "-1m", "non-negative duration"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			ctx := mocktest.NewTurnContext(t, mocktest.NewMockSystem(t), "test").
				WithAdmin(true).
				WithArgs("/set", tt.key, tt.value)

			(&SetCommand{}).Execute(ctx.Turn())

			reply := ctx.LastReply()
			if !strings.Contains(reply, "invalid value for "+tt.key) {
				t.Errorf("reply %q does not name the setting", reply)
			}
			if !strings.Contains(reply, tt.want) {
				t.Errorf("reply %q does not explain the value (want %q)", reply, tt.want)
			}
		})
	}
}
