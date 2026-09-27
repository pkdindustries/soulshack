package commands

import (
	"errors"
	"testing"
	"time"

	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestCompletionCommand_BasicFlow(t *testing.T) {
	mockSys := mocktest.NewMockSystem(t)
	mockSys.LLM = &mocktest.MockLLM{
		Responses: []string{"Hello from the LLM!"},
	}

	ctx := mocktest.NewTurnContext(t, mockSys, "test").
		WithArgs("hello", "world")

	cmd := &CompletionCommand{}
	cmd.Execute(ctx.Turn())

	// Wait a bit for the async response
	time.Sleep(50 * time.Millisecond)

	if ctx.ReplyCount() == 0 {
		t.Fatal("expected at least one reply from LLM")
	}
	if ctx.LastReply() != "Hello from the LLM!" {
		t.Errorf("expected 'Hello from the LLM!', got: %s", ctx.LastReply())
	}
}

func TestCompletionCommand_MultiChunkResponse(t *testing.T) {
	mockSys := mocktest.NewMockSystem(t)
	mockSys.LLM = &mocktest.MockLLM{
		Responses: []string{"First chunk", "Second chunk", "Third chunk"},
	}

	ctx := mocktest.NewTurnContext(t, mockSys, "test").
		WithArgs("tell", "me", "a", "story")

	cmd := &CompletionCommand{}
	cmd.Execute(ctx.Turn())

	// Wait for all chunks
	time.Sleep(50 * time.Millisecond)

	if ctx.ReplyCount() != 3 {
		t.Fatalf("expected 3 replies, got %d: %v", ctx.ReplyCount(), ctx.Replies)
	}

	expected := []string{"First chunk", "Second chunk", "Third chunk"}
	for i, exp := range expected {
		if ctx.Replies[i] != exp {
			t.Errorf("reply %d: expected %q, got %q", i, exp, ctx.Replies[i])
		}
	}
}

func TestCompletionCommand_ErrorHandling(t *testing.T) {
	mockSys := mocktest.NewMockSystem(t)
	mockSys.LLM = &mocktest.MockLLM{
		Responses: []string{},
		Error:     errors.New("API rate limit exceeded"),
	}

	ctx := mocktest.NewTurnContext(t, mockSys, "test").
		WithArgs("hello")

	cmd := &CompletionCommand{}
	cmd.Execute(ctx.Turn())

	// Wait for error to propagate
	time.Sleep(50 * time.Millisecond)

	if ctx.ReplyCount() == 0 {
		t.Fatal("expected error reply")
	}

	// The error gets sent as an "Error: ..." message from the chunker
	if !ctx.HasReply("Error:") && !ctx.HasReply("rate limit") {
		t.Errorf("expected error message in replies, got: %v", ctx.Replies)
	}
}

func TestCompletionCommand_ConversationUpdated(t *testing.T) {
	mockSys := mocktest.NewMockSystem(t)
	mockSys.LLM = &mocktest.MockLLM{
		Responses: []string{"Response"},
	}

	conversation := mockSys.Conversation(t, "test")
	ctx := mocktest.NewMockContext().
		WithSystem(mockSys).
		WithConversation(conversation).
		WithSource("testuser").
		WithArgs("hello", "world")

	initialLen := len(conversation.Messages())

	cmd := &CompletionCommand{}
	cmd.Execute(ctx.Turn())

	// Wait for completion
	time.Sleep(50 * time.Millisecond)

	// The conversation should have the new messages
	if newLen := len(conversation.Messages()); newLen <= initialLen {
		t.Errorf("expected conversation to grow, was %d, now %d", initialLen, newLen)
	}
}
