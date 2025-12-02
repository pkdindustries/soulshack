package commands

import (
	"errors"
	"testing"
	"time"

	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestCompletionCommand_BasicFlow(t *testing.T) {
	mockSys := mocktest.NewMockSystem()
	mockSys.LLM = &mocktest.MockLLM{
		Responses: []string{"Hello from the LLM!"},
	}

	ctx := mocktest.NewMockContext().
		WithSystem(mockSys).
		WithArgs("hello", "world")

	cmd := &CompletionCommand{}
	cmd.Execute(ctx)

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
	mockSys := mocktest.NewMockSystem()
	mockSys.LLM = &mocktest.MockLLM{
		Responses: []string{"First chunk", "Second chunk", "Third chunk"},
	}

	ctx := mocktest.NewMockContext().
		WithSystem(mockSys).
		WithArgs("tell", "me", "a", "story")

	cmd := &CompletionCommand{}
	cmd.Execute(ctx)

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
	mockSys := mocktest.NewMockSystem()
	mockSys.LLM = &mocktest.MockLLM{
		Responses: []string{},
		Error:     errors.New("API rate limit exceeded"),
	}

	ctx := mocktest.NewMockContext().
		WithSystem(mockSys).
		WithArgs("hello")

	cmd := &CompletionCommand{}
	cmd.Execute(ctx)

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

func TestCompletionCommand_SessionUpdated(t *testing.T) {
	mockSys := mocktest.NewMockSystem()
	mockSys.LLM = &mocktest.MockLLM{
		Responses: []string{"Response"},
	}

	// Get a session from the store
	session, _ := mockSys.SessionStore.Get("test")

	ctx := mocktest.NewMockContext().
		WithSystem(mockSys).
		WithSession(session).
		WithSource("testuser").
		WithArgs("hello", "world")

	initialHistoryLen := len(session.GetHistory())

	cmd := &CompletionCommand{}
	cmd.Execute(ctx)

	// Wait for completion
	time.Sleep(50 * time.Millisecond)

	// Session should have new messages added
	newHistoryLen := len(session.GetHistory())
	if newHistoryLen <= initialHistoryLen {
		t.Errorf("expected session history to grow, was %d, now %d",
			initialHistoryLen, newHistoryLen)
	}
}
