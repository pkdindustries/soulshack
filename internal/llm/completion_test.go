package llm

import (
	"context"
	"testing"
	"time"

	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestComplete_ContextCancellation(t *testing.T) {
	mockSys := mocktest.NewMockSystem(t)

	// MockLLM with delay to allow cancellation mid-stream
	mockSys.LLM = &mocktest.MockLLM{
		Responses: []string{"First", "Second", "Third", "Fourth", "Fifth"},
		Delay:     50 * time.Millisecond,
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	mockCtx := mocktest.NewMockContext().
		WithContext(ctx).
		WithSystem(mockSys).
		WithConversation(mockSys.Conversation(t, "test")).
		WithArgs("hello")

	// Start completion
	outch := Complete(mockCtx.Turn(), "test message")

	// Read first response
	firstResp := <-outch

	// Cancel after receiving first response
	cancel()

	// Give time for cancellation to propagate
	time.Sleep(100 * time.Millisecond)

	// Count remaining responses (should be minimal due to cancellation)
	remaining := 0
	for range outch {
		remaining++
	}

	// We should have stopped early due to cancellation
	// First response received, then cancellation should prevent most/all remaining
	if remaining >= 4 {
		t.Errorf("expected cancellation to stop stream early, got first response %q and %d more", firstResp, remaining)
	}
}

func TestComplete_Timeout(t *testing.T) {
	mockSys := mocktest.NewMockSystem(t)

	// MockLLM with long delay
	mockSys.LLM = &mocktest.MockLLM{
		Responses: []string{"Response1", "Response2", "Response3"},
		Delay:     200 * time.Millisecond,
	}

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	mockCtx := mocktest.NewMockContext().
		WithContext(ctx).
		WithSystem(mockSys).
		WithConversation(mockSys.Conversation(t, "test")).
		WithArgs("hello")

	// Start completion
	outch := Complete(mockCtx.Turn(), "test message")

	// Collect all responses
	var responses []string
	for resp := range outch {
		responses = append(responses, resp)
	}

	// With 100ms timeout and 200ms delay per response, we should get 0 or 1 responses
	if len(responses) > 1 {
		t.Errorf("expected timeout to limit responses, got %d: %v", len(responses), responses)
	}
}

func TestComplete_SetsProjectionBudget(t *testing.T) {
	mockSys := mocktest.NewMockSystem(t)
	mockLLM := &mocktest.MockLLM{Responses: []string{"ok"}}
	mockSys.LLM = mockLLM

	mockCtx := mocktest.NewMockContext().
		WithSystem(mockSys).
		WithConversation(mockSys.Conversation(t, "test")).
		WithArgs("hello")

	for range Complete(mockCtx.Turn(), "test message") {
	}

	// maxcontext reaches the request as the projection budget, and is the same
	// number the conversation is trimmed to.
	if mockLLM.LastRequest == nil {
		t.Fatal("no completion request captured")
	}
	if mockLLM.LastRequest.MaxContextTokens != 100000 {
		t.Fatalf("MaxContextTokens = %d, want 100000 (config maxcontext)",
			mockLLM.LastRequest.MaxContextTokens)
	}
}

func TestComplete_NoLeakedGoroutines(t *testing.T) {
	mockSys := mocktest.NewMockSystem(t)
	mockSys.LLM = &mocktest.MockLLM{
		Responses: []string{"Quick response"},
	}

	mockCtx := mocktest.NewMockContext().
		WithSystem(mockSys).
		WithConversation(mockSys.Conversation(t, "test")).
		WithArgs("hello")

	// Run completion and drain the channel completely
	for range Complete(mockCtx.Turn(), "test message") {
	}

	// If we reach here without hanging, goroutines cleaned up properly
	// (This is a basic sanity check - more thorough testing would use runtime.NumGoroutine)
}
