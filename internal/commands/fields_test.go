package commands

import (
	"strings"
	"testing"

	"pkdindustries/soulshack/internal/config"
	mocktest "pkdindustries/soulshack/internal/testing"
)

// Every key reads back as something its own setter accepts, and writing that
// value back leaves the setting alone. This is the invariant the field
// constructors share, checked across the whole table.
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

// The keys are listed to users, so they come out in a stable order.
func TestConfigKeysAreSorted(t *testing.T) {
	keys := getConfigKeys()
	if len(keys) != len(configFields) {
		t.Fatalf("listed %d keys for %d fields", len(keys), len(configFields))
	}
	for i := 1; i < len(keys); i++ {
		if keys[i-1] > keys[i] {
			t.Fatalf("keys are not sorted: %v", keys)
		}
	}
}

// A rejected value is explained in terms of what the setting wants, and the
// reply names the setting, rather than echoing the parser's own error.
func TestConfigFieldsRejectWithExplanation(t *testing.T) {
	tests := []struct {
		key   string
		value string
		want  string
	}{
		{"addressed", "maybe", "'true' or 'false'"},
		{"maxtokens", "many", "valid integer"},
		{"maxcontext", "-5", "at least 0"},
		{"temperature", "hot", "valid float"},
		{"top_p", "7", "between 0 and 1"},
		{"sessionduration", "10", "valid duration"},
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
