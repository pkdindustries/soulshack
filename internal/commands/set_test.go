package commands

import (
	"strings"
	"testing"

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
	cmd.Execute(ctx)

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
	cmd.Execute(ctx)

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
	cmd.Execute(ctx)

	if ctx.ReplyCount() != 1 {
		t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
	}
	if !strings.Contains(ctx.LastReply(), "Unknown key") {
		t.Errorf("expected unknown key error, got: %s", ctx.LastReply())
	}
}

func TestSetCommand_SetModel(t *testing.T) {
	mockSys := mocktest.NewMockSystem()
	ctx := mocktest.NewMockContext().
		WithAdmin(true).
		WithSystem(mockSys).
		WithArgs("/set", "model", "gpt-4")

	cmd := &SetCommand{}
	cmd.Execute(ctx)

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
	mockSys := mocktest.NewMockSystem()
	ctx := mocktest.NewMockContext().
		WithAdmin(true).
		WithSystem(mockSys).
		WithArgs("/set", "prompt", "You", "are", "helpful")

	cmd := &SetCommand{}
	cmd.Execute(ctx)

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
	mockSys := mocktest.NewMockSystem()
	ctx := mocktest.NewMockContext().
		WithAdmin(true).
		WithSystem(mockSys).
		WithArgs("/set", "addressed", "false")

	cmd := &SetCommand{}
	cmd.Execute(ctx)

	if ctx.ReplyCount() != 1 {
		t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
	}
	if ctx.GetConfig().Bot.Addressed != false {
		t.Error("expected addressed to be false")
	}
}

func TestSetCommand_InvalidBoolValue(t *testing.T) {
	mockSys := mocktest.NewMockSystem()
	ctx := mocktest.NewMockContext().
		WithAdmin(true).
		WithSystem(mockSys).
		WithArgs("/set", "addressed", "notabool")

	cmd := &SetCommand{}
	cmd.Execute(ctx)

	if ctx.ReplyCount() != 1 {
		t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
	}
	if !strings.Contains(ctx.LastReply(), "invalid") {
		t.Errorf("expected invalid value error, got: %s", ctx.LastReply())
	}
}

func TestSetCommand_SetMaxTokens(t *testing.T) {
	mockSys := mocktest.NewMockSystem()
	ctx := mocktest.NewMockContext().
		WithAdmin(true).
		WithSystem(mockSys).
		WithArgs("/set", "maxtokens", "2048")

	cmd := &SetCommand{}
	cmd.Execute(ctx)

	if ctx.ReplyCount() != 1 {
		t.Fatalf("expected 1 reply, got %d", ctx.ReplyCount())
	}
	if ctx.GetConfig().Model.MaxTokens != 2048 {
		t.Errorf("expected maxtokens to be 2048, got: %d", ctx.GetConfig().Model.MaxTokens)
	}
}

func TestSetCommand_InvalidIntValue(t *testing.T) {
	mockSys := mocktest.NewMockSystem()
	ctx := mocktest.NewMockContext().
		WithAdmin(true).
		WithSystem(mockSys).
		WithArgs("/set", "maxtokens", "notanint")

	cmd := &SetCommand{}
	cmd.Execute(ctx)

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
			mockSys := mocktest.NewMockSystem()
			ctx := mocktest.NewMockContext().
				WithAdmin(true).
				WithSystem(mockSys).
				WithArgs("/set", "sessionduration", tt.value)

			cmd := &SetCommand{}
			cmd.Execute(ctx)

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
			mockSys := mocktest.NewMockSystem()
			ctx := mocktest.NewMockContext().
				WithAdmin(true).
				WithSystem(mockSys).
				WithArgs("/set", "top_p", tt.value)

			cmd := &SetCommand{}
			cmd.Execute(ctx)

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
			mockSys := mocktest.NewMockSystem()
			ctx := mocktest.NewMockContext().
				WithAdmin(true).
				WithSystem(mockSys).
				WithArgs("/set", "chunkmax", tt.value)

			cmd := &SetCommand{}
			cmd.Execute(ctx)

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
