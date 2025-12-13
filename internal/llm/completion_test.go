package llm

import (
	"context"
	"testing"
	"time"

	mocktest "pkdindustries/soulshack/internal/testing"

	"github.com/alexschlessinger/pollytool/messages"
)

func TestComplete_ContextCancellation(t *testing.T) {
	mockSys := mocktest.NewMockSystem()

	// MockLLM with delay to allow cancellation mid-stream
	mockSys.LLM = &mocktest.MockLLM{
		Responses: []string{"First", "Second", "Third", "Fourth", "Fifth"},
		Delay:     50 * time.Millisecond,
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	session, _ := mockSys.SessionStore.Get("test")
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
	mockSys := mocktest.NewMockSystem()

	// MockLLM with long delay
	mockSys.LLM = &mocktest.MockLLM{
		Responses: []string{"Response1", "Response2", "Response3"},
		Delay:     200 * time.Millisecond,
	}

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	session, _ := mockSys.SessionStore.Get("test")
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

func TestComplete_NoLeakedGoroutines(t *testing.T) {
	mockSys := mocktest.NewMockSystem()
	mockSys.LLM = &mocktest.MockLLM{
		Responses: []string{"Quick response"},
	}

	session, _ := mockSys.SessionStore.Get("test")
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

func TestCheckSessionCapacity_NoWarningBelowThreshold(t *testing.T) {
	// Reset warning state
	warnedSessions = make(map[string]int)

	mockSys := mocktest.NewMockSystem()
	session, _ := mockSys.SessionStore.Get("test")

	// Add messages totaling ~50% capacity (50,000 tokens)
	// Using estimation: 1 token ≈ 4 characters
	largeBytes := make([]byte, 200000) // 200,000 chars ≈ 50,000 tokens
	for i := range largeBytes {
		largeBytes[i] = 'a'
	}
	session.AddMessage(messages.ChatMessage{
		Role:    messages.MessageRoleUser,
		Content: string(largeBytes),
	})

	mockCtx := mocktest.NewMockContext().
		WithSystem(mockSys).
		WithSession(session)

	// Check capacity - should not trigger warnings
	checkSessionCapacity(mockCtx)

	// Verify no warnings were sent
	if len(mockCtx.Actions) > 0 {
		t.Errorf("expected no warnings below 75%%, got: %v", mockCtx.Actions)
	}
}

func TestCheckSessionCapacity_Warning75Percent(t *testing.T) {
	// Reset warning state
	warnedSessions = make(map[string]int)

	mockSys := mocktest.NewMockSystem()
	session, _ := mockSys.SessionStore.Get("test")

	// Add messages totaling ~76% capacity (76,000 tokens)
	largeBytes := make([]byte, 304000) // 304,000 chars ≈ 76,000 tokens
	for i := range largeBytes {
		largeBytes[i] = 'a'
	}
	session.AddMessage(messages.ChatMessage{
		Role:    messages.MessageRoleUser,
		Content: string(largeBytes),
	})

	mockCtx := mocktest.NewMockContext().
		WithSystem(mockSys).
		WithSession(session)

	// Check capacity - should trigger 75% warning
	checkSessionCapacity(mockCtx)

	// Verify 75% warning was sent
	if len(mockCtx.Actions) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(mockCtx.Actions))
	}
	if mockCtx.Actions[0] != "Session at 75% capacity" {
		t.Errorf("unexpected warning: %s", mockCtx.Actions[0])
	}
}

func TestCheckSessionCapacity_Warning90Percent(t *testing.T) {
	// Reset warning state
	warnedSessions = make(map[string]int)

	mockSys := mocktest.NewMockSystem()
	session, _ := mockSys.SessionStore.Get("test")

	// Add messages totaling ~91% capacity (91,000 tokens)
	largeBytes := make([]byte, 364000) // 364,000 chars ≈ 91,000 tokens
	for i := range largeBytes {
		largeBytes[i] = 'a'
	}
	session.AddMessage(messages.ChatMessage{
		Role:    messages.MessageRoleUser,
		Content: string(largeBytes),
	})

	mockCtx := mocktest.NewMockContext().
		WithSystem(mockSys).
		WithSession(session)

	// Check capacity - should trigger 90% warning
	checkSessionCapacity(mockCtx)

	// Verify 90% warning was sent
	if len(mockCtx.Actions) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(mockCtx.Actions))
	}
	if mockCtx.Actions[0] != "Session at 90% capacity - conversation history will be trimmed soon" {
		t.Errorf("unexpected warning: %s", mockCtx.Actions[0])
	}
}

func TestCheckSessionCapacity_NoRepeatedWarnings(t *testing.T) {
	// Reset warning state
	warnedSessions = make(map[string]int)

	mockSys := mocktest.NewMockSystem()
	session, _ := mockSys.SessionStore.Get("test")

	// Add messages totaling ~76% capacity
	largeBytes := make([]byte, 304000) // 304,000 chars ≈ 76,000 tokens
	for i := range largeBytes {
		largeBytes[i] = 'a'
	}
	session.AddMessage(messages.ChatMessage{
		Role:    messages.MessageRoleUser,
		Content: string(largeBytes),
	})

	mockCtx := mocktest.NewMockContext().
		WithSystem(mockSys).
		WithSession(session)

	// First check - should trigger warning
	checkSessionCapacity(mockCtx)
	if len(mockCtx.Actions) != 1 {
		t.Fatalf("expected 1 warning on first check, got %d", len(mockCtx.Actions))
	}

	// Second check - should NOT trigger another warning
	checkSessionCapacity(mockCtx)
	if len(mockCtx.Actions) != 1 {
		t.Errorf("expected no additional warnings, total warnings: %d", len(mockCtx.Actions))
	}
}

func TestCheckSessionCapacity_WarningReset(t *testing.T) {
	// Reset warning state
	warnedSessions = make(map[string]int)

	mockSys := mocktest.NewMockSystem()
	session, _ := mockSys.SessionStore.Get("test")

	// First, trigger 75% warning
	largeBytes := make([]byte, 304000) // 76,000 tokens
	for i := range largeBytes {
		largeBytes[i] = 'a'
	}
	session.AddMessage(messages.ChatMessage{
		Role:    messages.MessageRoleUser,
		Content: string(largeBytes),
	})

	mockCtx := mocktest.NewMockContext().
		WithSystem(mockSys).
		WithSession(session)

	checkSessionCapacity(mockCtx)
	if len(mockCtx.Actions) != 1 {
		t.Fatalf("expected initial warning, got %d", len(mockCtx.Actions))
	}

	// Clear session to drop below 75%
	session.Clear()

	// Add small message (below 75%)
	session.AddMessage(messages.ChatMessage{
		Role:    messages.MessageRoleUser,
		Content: "small message",
	})

	// Check again - warning state should be reset
	checkSessionCapacity(mockCtx)

	// Now add content to go above 75% again
	session.AddMessage(messages.ChatMessage{
		Role:    messages.MessageRoleUser,
		Content: string(largeBytes),
	})
	checkSessionCapacity(mockCtx)

	// Should have 2 warnings total (initial + after reset)
	if len(mockCtx.Actions) != 2 {
		t.Errorf("expected 2 total warnings after reset, got %d", len(mockCtx.Actions))
	}
}

func TestCheckSessionCapacity_NoLimitSet(t *testing.T) {
	// Reset warning state
	warnedSessions = make(map[string]int)

	mockSys := mocktest.NewMockSystem()

	// Create custom config with no limit
	cfg := mocktest.DefaultTestConfig()
	cfg.Session.MaxContext = 0 // No limit

	// Create session with metadata that has no limit
	session, _ := mockSys.SessionStore.Get("test-no-limit")
	metadata := session.GetMetadata()
	metadata.MaxHistoryTokens = 0 // No limit
	session.SetMetadata(metadata)

	// Add large amount of content
	largeBytes := make([]byte, 500000)
	for i := range largeBytes {
		largeBytes[i] = 'a'
	}
	session.AddMessage(messages.ChatMessage{
		Role:    messages.MessageRoleUser,
		Content: string(largeBytes),
	})

	mockCtx := mocktest.NewMockContext().
		WithConfig(cfg).
		WithSystem(mockSys).
		WithSession(session)

	// Check capacity - should not trigger warnings when no limit
	checkSessionCapacity(mockCtx)

	// Verify no warnings were sent
	if len(mockCtx.Actions) > 0 {
		t.Errorf("expected no warnings when no limit set, got: %v", mockCtx.Actions)
	}
}
