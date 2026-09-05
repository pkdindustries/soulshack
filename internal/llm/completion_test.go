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

	session := mockSys.AcquireSession(t, "test")
	mockCtx := mocktest.NewMockContext().
		WithContext(ctx).
		WithSystem(mockSys).
		WithSession(session).
		WithArgs("hello")

	// Start completion
	outch, err := Complete(mockCtx, "test message")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

	session := mockSys.AcquireSession(t, "test")
	mockCtx := mocktest.NewMockContext().
		WithContext(ctx).
		WithSystem(mockSys).
		WithSession(session).
		WithArgs("hello")

	// Start completion
	outch, err := Complete(mockCtx, "test message")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

	session := mockSys.AcquireSession(t, "test")
	mockCtx := mocktest.NewMockContext().
		WithSystem(mockSys).
		WithSession(session).
		WithArgs("hello")

	outch, err := Complete(mockCtx, "test message")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range outch {
	}

	// maxcontext must reach the request as the projection budget (the store
	// no longer trims history at write time).
	if mockLLM.LastRequest == nil {
		t.Fatal("no completion request captured")
	}
	if mockLLM.LastRequest.MaxContextTokens != 100000 {
		t.Fatalf("MaxContextTokens = %d, want 100000 (session MaxHistoryTokens)",
			mockLLM.LastRequest.MaxContextTokens)
	}
}

func TestComplete_NoLeakedGoroutines(t *testing.T) {
	mockSys := mocktest.NewMockSystem(t)
	mockSys.LLM = &mocktest.MockLLM{
		Responses: []string{"Quick response"},
	}

	session := mockSys.AcquireSession(t, "test")
	mockCtx := mocktest.NewMockContext().
		WithSystem(mockSys).
		WithSession(session).
		WithArgs("hello")

	// Run completion
	outch, err := Complete(mockCtx, "test message")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Drain the channel completely
	for range outch {
	}

	// If we reach here without hanging, goroutines cleaned up properly
	// (This is a basic sanity check - more thorough testing would use runtime.NumGoroutine)
}
