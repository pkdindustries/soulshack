package behaviors

import (
	"testing"

	"github.com/lrstanley/girc"

	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestOpBehaviorCheck(t *testing.T) {
	behavior := &OpBehavior{BotNick: "soulshack"}

	tests := []struct {
		name   string
		params []string
		want   bool
	}{
		{
			name:   "matches +o for bot",
			params: []string{"#test", "+o", "soulshack"},
			want:   true,
		},
		{
			name:   "matches -o for bot",
			params: []string{"#test", "-o", "soulshack"},
			want:   true,
		},
		{
			name:   "ignores voice change for bot",
			params: []string{"#test", "+v", "soulshack"},
			want:   false,
		},
		{
			name:   "ignores op change for another user",
			params: []string{"#test", "+o", "someoneelse"},
			want:   false,
		},
		{
			name:   "handles mixed prefix modes before bot op",
			params: []string{"#test", "+qo", "chanowner", "soulshack"},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := mocktest.NewMockContext().WithConfig(mocktest.DefaultTestConfig())
			ctx.GetConfig().Bot.OpWatcher = true

			event := &girc.Event{
				Command: girc.MODE,
				Params:  tt.params,
			}

			if got := behavior.Check(ctx, event); got != tt.want {
				t.Fatalf("OpBehavior.Check(%v) = %v, want %v", tt.params, got, tt.want)
			}
		})
	}
}

func TestOpActionForNick(t *testing.T) {
	tests := []struct {
		name       string
		params     []string
		wantAction string
		wantOK     bool
	}{
		{
			name:       "returns opped action",
			params:     []string{"#test", "+o", "soulshack"},
			wantAction: "opped",
			wantOK:     true,
		},
		{
			name:       "returns deopped action",
			params:     []string{"#test", "-o", "soulshack"},
			wantAction: "deopped",
			wantOK:     true,
		},
		{
			name:       "ignores unrelated target mode",
			params:     []string{"#test", "+v", "soulshack"},
			wantAction: "",
			wantOK:     false,
		},
		{
			name:       "keeps arguments aligned across mixed modes",
			params:     []string{"#test", "+ov", "soulshack", "otheruser"},
			wantAction: "opped",
			wantOK:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &girc.Event{
				Command: girc.MODE,
				Params:  tt.params,
			}

			gotAction, gotOK := opActionForNick(event, "soulshack")
			if gotAction != tt.wantAction || gotOK != tt.wantOK {
				t.Fatalf("opActionForNick(%v) = (%q, %v), want (%q, %v)", tt.params, gotAction, gotOK, tt.wantAction, tt.wantOK)
			}
		})
	}
}
