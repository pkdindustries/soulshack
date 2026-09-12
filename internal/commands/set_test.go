package commands

import (
	"strings"
	"sync"
	"testing"
	"time"

	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestSetCommand_Name(t *testing.T) {
	cmd := &SetCommand{}
	if cmd.Name() != "/set" {
		t.Errorf("expected /set, got %s", cmd.Name())
	}
}

func TestSetCommand_AdminOnly(t *testing.T) {
	cmd := &SetCommand{}
	if !cmd.AdminOnly() {
		t.Error("expected SetCommand to be admin-only")
	}
}

func TestSetCommand_MissingArgs(t *testing.T) {
	ctx := mocktest.NewMockContext().
		WithAdmin(true).
		WithArgs("/set")

	cmd := &SetCommand{}
	cmd.Execute(ctx.Turn())

	if ctx.ReplyCount() != 1 {
		t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
	}
	if !strings.Contains(ctx.LastReply(), "Usage:") {
		t.Errorf("expected usage message, got: %s", ctx.LastReply())
	}
}

func TestSetCommand_MissingValue(t *testing.T) {
	ctx := mocktest.NewMockContext().
		WithAdmin(true).
		WithArgs("/set", "model")

	cmd := &SetCommand{}
	cmd.Execute(ctx.Turn())

	if ctx.ReplyCount() != 1 {
		t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
	}
	if !strings.Contains(ctx.LastReply(), "Usage:") {
		t.Errorf("expected usage message, got: %s", ctx.LastReply())
	}
}

func TestSetCommand_UnknownKey(t *testing.T) {
	ctx := mocktest.NewMockContext().
		WithAdmin(true).
		WithArgs("/set", "unknownkey", "somevalue")

	cmd := &SetCommand{}
	cmd.Execute(ctx.Turn())

	if ctx.ReplyCount() != 1 {
		t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
	}
	if !strings.Contains(ctx.LastReply(), "Unknown key") {
		t.Errorf("expected unknown key error, got: %s", ctx.LastReply())
	}
}

func TestSetCommand_SetModel(t *testing.T) {
	mockSys := mocktest.NewMockSystem(t)
	ctx := mocktest.NewTurnContext(t, mockSys, "test").
		WithAdmin(true).
		WithArgs("/set", "model", "gpt-4")

	cmd := &SetCommand{}
	cmd.Execute(ctx.Turn())

	if ctx.ReplyCount() != 1 {
		t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
	}
	if !strings.Contains(ctx.LastReply(), "model set to:") {
		t.Errorf("expected confirmation, got: %s", ctx.LastReply())
	}
	if ctx.GetConfig().Model.Model != "gpt-4" {
		t.Errorf("expected model to be gpt-4, got: %s", ctx.GetConfig().Model.Model)
	}
}

func TestSetCommand_SetPrompt(t *testing.T) {
	mockSys := mocktest.NewMockSystem(t)
	ctx := mocktest.NewTurnContext(t, mockSys, "test").
		WithAdmin(true).
		WithArgs("/set", "prompt", "You", "are", "helpful")

	cmd := &SetCommand{}
	cmd.Execute(ctx.Turn())

	if ctx.ReplyCount() != 1 {
		t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
	}
	// Prompt should be joined with spaces
	expected := "You are helpful"
	if ctx.GetConfig().Bot.Prompt != expected {
		t.Errorf("expected prompt %q, got: %q", expected, ctx.GetConfig().Bot.Prompt)
	}
}

func TestSetCommand_SetAddressed(t *testing.T) {
	mockSys := mocktest.NewMockSystem(t)
	ctx := mocktest.NewTurnContext(t, mockSys, "test").
		WithAdmin(true).
		WithArgs("/set", "addressed", "false")

	cmd := &SetCommand{}
	cmd.Execute(ctx.Turn())

	if ctx.ReplyCount() != 1 {
		t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
	}
	if ctx.GetConfig().Bot.Addressed != false {
		t.Error("expected addressed to be false")
	}
}

func TestSetCommand_InvalidBoolValue(t *testing.T) {
	mockSys := mocktest.NewMockSystem(t)
	ctx := mocktest.NewTurnContext(t, mockSys, "test").
		WithAdmin(true).
		WithArgs("/set", "addressed", "notabool")

	cmd := &SetCommand{}
	cmd.Execute(ctx.Turn())

	if ctx.ReplyCount() != 1 {
		t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
	}
	if !strings.Contains(ctx.LastReply(), "invalid") {
		t.Errorf("expected invalid value error, got: %s", ctx.LastReply())
	}
}

func TestSetCommand_SetMaxTokens(t *testing.T) {
	mockSys := mocktest.NewMockSystem(t)
	ctx := mocktest.NewTurnContext(t, mockSys, "test").
		WithAdmin(true).
		WithArgs("/set", "maxtokens", "2048")

	cmd := &SetCommand{}
	cmd.Execute(ctx.Turn())

	if ctx.ReplyCount() != 1 {
		t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
	}
	if ctx.GetConfig().Model.MaxTokens != 2048 {
		t.Errorf("expected maxtokens to be 2048, got: %d", ctx.GetConfig().Model.MaxTokens)
	}
}

func TestSetCommand_InvalidIntValue(t *testing.T) {
	mockSys := mocktest.NewMockSystem(t)
	ctx := mocktest.NewTurnContext(t, mockSys, "test").
		WithAdmin(true).
		WithArgs("/set", "maxtokens", "notanint")

	cmd := &SetCommand{}
	cmd.Execute(ctx.Turn())

	if ctx.ReplyCount() != 1 {
		t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
	}
	if !strings.Contains(ctx.LastReply(), "invalid") {
		t.Errorf("expected invalid value error, got: %s", ctx.LastReply())
	}
}

func TestSetCommand_InvalidDuration(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"no unit", "10"},
		{"invalid format", "abc"},
		{"spaces", "10 m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSys := mocktest.NewMockSystem(t)
			ctx := mocktest.NewTurnContext(t, mockSys, "test").
				WithAdmin(true).
				WithArgs("/set", "sessionduration", tt.value)

			cmd := &SetCommand{}
			cmd.Execute(ctx.Turn())

			if ctx.ReplyCount() != 1 {
				t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
			}
			if !strings.Contains(ctx.LastReply(), "invalid") {
				t.Errorf("expected invalid duration error for %q, got: %s", tt.value, ctx.LastReply())
			}
		})
	}
}

func TestSetCommand_TopPBounds(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantError bool
	}{
		{"valid 0", "0", false},
		{"valid 0.5", "0.5", false},
		{"valid 1", "1", false},
		{"too low", "-0.1", true},
		{"too high", "1.5", true},
		{"way too high", "2", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSys := mocktest.NewMockSystem(t)
			ctx := mocktest.NewTurnContext(t, mockSys, "test").
				WithAdmin(true).
				WithArgs("/set", "top_p", tt.value)

			cmd := &SetCommand{}
			cmd.Execute(ctx.Turn())

			if ctx.ReplyCount() != 1 {
				t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
			}

			hasError := strings.Contains(ctx.LastReply(), "invalid") ||
				strings.Contains(ctx.LastReply(), "between 0 and 1")
			if hasError != tt.wantError {
				t.Errorf("top_p=%s: wantError=%v but got reply: %s", tt.value, tt.wantError, ctx.LastReply())
			}
		})
	}
}

func TestSetCommand_ChunkMaxEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  int
	}{
		{"zero", "0", 0},
		{"small", "10", 10},
		{"normal", "350", 350},
		{"negative", "-1", -1}, // Currently accepted (no validation)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := mocktest.NewTurnContext(t, mocktest.NewMockSystem(t), "test").
				WithAdmin(true).
				WithArgs("/set", "chunkmax", tt.value)

			cmd := &SetCommand{}
			cmd.Execute(ctx.Turn())

			if ctx.ReplyCount() != 1 {
				t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
			}

			// Currently no bounds validation, so all values are accepted
			if ctx.GetConfig().Session.ChunkMax != tt.want {
				t.Errorf("expected chunkmax=%d, got=%d", tt.want, ctx.GetConfig().Session.ChunkMax)
			}
		})
	}
}

// maxcontext and sessionduration take effect for conversations that already
// exist, not only for ones created after the change.
func TestSetCommand_ConversationLimitsApplyToTheMemory(t *testing.T) {
	mockSys := mocktest.NewMockSystem(t)
	ctx := mocktest.NewTurnContext(t, mockSys, "test").
		WithAdmin(true).
		WithArgs("/set", "maxcontext", "4096")

	cmd := &SetCommand{}
	cmd.Execute(ctx.Turn())
	if got := mockSys.Memory.Budget(); got != 4096 {
		t.Errorf("maxcontext did not reach the memory: %d", got)
	}

	ctx.WithArgs("/set", "sessionduration", "1h")
	cmd.Execute(ctx.Turn())
	if got := mockSys.Memory.TTL(); got != time.Hour {
		t.Errorf("sessionduration did not reach the memory: %s", got)
	}
}

// Changes made with /set are visible to turns reading the configuration at the
// same time: both go through the store, not through a shared pointer.
func TestSetCommand_ConcurrentReadersSeeChanges(t *testing.T) {
	mockSys := mocktest.NewMockSystem(t)
	ctx := mocktest.NewTurnContext(t, mockSys, "test").
		WithAdmin(true)
	turn := ctx.Turn()
	cmd := &SetCommand{}

	stop := make(chan struct{})
	var readers sync.WaitGroup
	sawChange := make(chan struct{})
	for i := 0; i < 4; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				cfg := turn.GetConfig()
				_ = cfg.Bot.Admins
				if cfg.Model.MaxTokens == 4242 {
					select {
					case sawChange <- struct{}{}:
					default:
					}
				}
			}
		}()
	}

	ctx.WithArgs("/set", "maxtokens", "4242")
	cmd.Execute(turn)
	select {
	case <-sawChange:
	case <-time.After(time.Second):
		t.Fatal("a reader never saw the change")
	}
	close(stop)
	readers.Wait()
}
