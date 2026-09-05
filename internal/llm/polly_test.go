package llm

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	polly "github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/sessions"
	"github.com/alexschlessinger/pollytool/tools"
	"github.com/alexschlessinger/pollytool/tools/sandbox"
	"pkdindustries/soulshack/internal/core"
	mocktest "pkdindustries/soulshack/internal/testing"
)

type completionFunc func(context.Context, *polly.CompletionRequest) messages.ChatMessage

func (f completionFunc) ChatCompletionStream(ctx context.Context, req *polly.CompletionRequest, processor polly.EventStreamProcessor) <-chan *messages.StreamEvent {
	input := make(chan messages.ChatMessage, 1)
	input <- f(ctx, req)
	close(input)
	return processor.ProcessMessagesToEvents(input)
}

func TestCreateAgentForRegistryTranscriptIsolation(t *testing.T) {
	registry := tools.NewToolRegistry(nil)
	calls := 0
	client := completionFunc(func(ctx context.Context, _ *polly.CompletionRequest) messages.ChatMessage {
		calls++
		if calls == 1 {
			// Run B while A is waiting for its first model response. A must
			// still read its own transcript when its tool call arrives.
			otherClient := completionFunc(func(context.Context, *polly.CompletionRequest) messages.ChatMessage {
				return messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "noted", StopReason: messages.StopReasonEndTurn}
			})
			otherAgent := CreateAgentForRegistry(otherClient, registry, time.Second)
			defer otherAgent.Close()
			if _, err := otherAgent.Run(ctx, &polly.CompletionRequest{Messages: messages.User("PRIVATE_B_TRANSCRIPT")}, nil); err != nil {
				t.Fatal(err)
			}
			return messages.ChatMessage{
				Role: messages.MessageRoleAssistant, StopReason: messages.StopReasonToolUse,
				ToolCalls: []messages.ChatMessageToolCall{{ID: "recall", Name: "read_transcript", Arguments: "{}"}},
			}
		}
		return messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "done", StopReason: messages.StopReasonEndTurn}
	})
	agent := CreateAgentForRegistry(client, registry, time.Second)
	defer agent.Close()
	response, err := agent.Run(context.Background(), &polly.CompletionRequest{Messages: messages.User("PRIVATE_A_TRANSCRIPT")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, msg := range response.AllMessages {
		if msg.Role == messages.MessageRoleTool && msg.ToolName == "read_transcript" {
			found = true
			if !strings.Contains(msg.Content, "PRIVATE_A_TRANSCRIPT") || strings.Contains(msg.Content, "PRIVATE_B_TRANSCRIPT") {
				t.Fatalf("agent A read the wrong transcript: %s", msg.Content)
			}
		}
	}
	if !found {
		t.Fatal("agent A did not receive its transcript")
	}
	for _, name := range []string{"read_transcript", "view_image"} {
		if _, ok := registry.Get(name); ok {
			t.Errorf("agent added %s to the shared registry", name)
		}
	}
}

func TestCreateAgentForRegistryPreservesImageReadPolicy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private.png")
	if err := os.WriteFile(path, []byte("private file"), 0600); err != nil {
		t.Fatal(err)
	}
	registry := tools.NewToolRegistry(nil, tools.WithSandboxFactory(sandbox.New, sandbox.Config{DenyPaths: []string{path}}))
	args, err := json.Marshal(map[string]string{"source": path})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	client := completionFunc(func(context.Context, *polly.CompletionRequest) messages.ChatMessage {
		calls++
		if calls == 1 {
			return messages.ChatMessage{
				Role: messages.MessageRoleAssistant, StopReason: messages.StopReasonToolUse,
				ToolCalls: []messages.ChatMessageToolCall{{ID: "image", Name: "view_image", Arguments: string(args)}},
			}
		}
		return messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "done", StopReason: messages.StopReasonEndTurn}
	})
	agent := CreateAgentForRegistry(client, registry, time.Second)
	defer agent.Close()
	response, err := agent.Run(context.Background(), &polly.CompletionRequest{Messages: messages.User("view the image")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, msg := range response.AllMessages {
		if msg.Role == messages.MessageRoleTool && msg.ToolName == "view_image" {
			if !strings.Contains(msg.Content, "blocked from reads by the sandbox policy") {
				t.Fatalf("view_image did not enforce the read policy: %s", msg.Content)
			}
			return
		}
	}
	t.Fatal("agent did not execute view_image")
}

// cancelOnSave exercises the deadline boundary between a completed response and
// persistence, while the conversation helper must still hold its lease.
type cancelOnSave struct {
	sessions.Session
	cancel  context.CancelFunc
	saveErr error
}

func (s *cancelOnSave) AddMessages(ctx context.Context, msgs []messages.ChatMessage) error {
	s.cancel()
	s.saveErr = s.Session.AddMessages(ctx, msgs)
	return s.saveErr
}

type saveStore struct {
	sessions.SessionStore
	cancel context.CancelFunc
	last   *cancelOnSave
}

func (s *saveStore) Acquire(ctx context.Context, key string, options sessions.AcquireOptions) (sessions.Session, error) {
	session, err := s.SessionStore.Acquire(ctx, key, options)
	if err != nil {
		return nil, err
	}
	s.last = &cancelOnSave{Session: session, cancel: s.cancel}
	return s.last, nil
}

func TestPollyPersistsProjectionAndFinalReplyBeforeReleasingLease(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := &saveStore{SessionStore: sys.Sessions, cancel: cancel}
	sys.Sessions = store
	base := mocktest.NewMockContext().WithSystem(sys).WithContext(parent)
	const budget = 2000
	modelCalls := 0
	sys.LLM = &PollyLLM{client: completionFunc(func(_ context.Context, req *polly.CompletionRequest) messages.ChatMessage {
		modelCalls++
		for _, msg := range req.Messages {
			if strings.Contains(msg.Content, "OLDEST_PRIVATE_HISTORY") {
				t.Error("oldest exchange was not omitted from provider input")
			}
		}
		return messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "final reply", StopReason: messages.StopReasonEndTurn}
	})}
	core.WithConversation(base, "complete", func(turn core.ChatContextInterface) {
		session := turn.GetSession()
		metadata, err := session.GetMetadata(turn)
		if err != nil {
			t.Fatal(err)
		}
		metadata.MaxHistoryTokens = budget
		if err := session.SetMetadata(turn, metadata); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 8; i++ {
			content := strings.Repeat("word ", 1000)
			if i == 0 {
				content = "OLDEST_PRIVATE_HISTORY " + content
			}
			if err := session.AddMessage(turn, messages.ChatMessage{Role: messages.MessageRoleUser, Content: content}); err != nil {
				t.Fatal(err)
			}
			if err := session.AddMessage(turn, messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "old reply"}); err != nil {
				t.Fatal(err)
			}
		}
		stream, err := Complete(turn, "new question")
		if err != nil {
			t.Fatal(err)
		}
		for range stream {
		}
	}, nil)
	if modelCalls != 1 {
		t.Fatalf("model calls = %d", modelCalls)
	}
	if store.last.saveErr != nil {
		t.Fatalf("final save lost its lease: %v", store.last.saveErr)
	}
	if parent.Err() == nil || store.last.Context().Err() == nil {
		t.Fatal("fixture did not cancel the request or close the lease")
	}
	// Reopen through the underlying store to verify the durable round trip.
	saved, err := store.SessionStore.Acquire(context.Background(), base.GetLockKey(), sessions.AcquireOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer saved.Close()
	history, err := saved.GetHistory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	usage, ok := core.LastContextUsage(history)
	if !ok || usage.Budget != budget || usage.EstimatedTokens <= 0 || usage.EstimatedTokens > budget || usage.OmittedExchanges == 0 {
		t.Fatalf("saved projection = %+v, found=%v", usage, ok)
	}
	if len(history) != 19 || history[len(history)-1].Content != "final reply" {
		t.Fatalf("history was trimmed or final reply lost: %d messages", len(history))
	}
}
