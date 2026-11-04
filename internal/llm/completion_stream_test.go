package llm

import (
	"context"
	"errors"
	"slices"
	"testing"

	"pkdindustries/soulshack/internal/core"
)

func TestDrainReportsCompletionOutcome(t *testing.T) {
	for _, tc := range []struct {
		name   string
		chunks []string
		err    error
		want   bool
	}{
		{"answer", []string{"an answer"}, nil, true},
		{"silent", nil, nil, false},
		{"whitespace", []string{"  "}, nil, false},
		{"error notice", []string{"Error: rejected"}, errors.New("rejected"), false},
		{"partial answer", []string{"part of an answer", "Error: rejected"}, errors.New("rejected"), false},
		{"context limit", []string{"Stopped: this does not fit"}, errors.New("context limit"), false},
		{"canceled", nil, context.Canceled, false},
		// The outcome is not inferred from text the model happens to write.
		{"answer mentions error", []string{"Error: is the prefix in the log"}, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chunks := make(chan string, len(tc.chunks))
			for _, chunk := range tc.chunks {
				chunks <- chunk
			}
			close(chunks)
			done := make(chan error, 1)
			done <- tc.err
			close(done)
			var received []string
			got := Drain(core.CompletionStream{Chunks: chunks, Done: done}, func(chunk string) { received = append(received, chunk) })
			if got != tc.want {
				t.Fatalf("answered=%v, want %v", got, tc.want)
			}
			if !slices.Equal(received, tc.chunks) {
				t.Fatalf("received %q, want all chunks %q", received, tc.chunks)
			}
		})
	}
}
