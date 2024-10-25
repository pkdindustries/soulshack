package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"

	ai "github.com/sashabaranov/go-openai"
)

func TestChunker_Chunk(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		size     int
		expected string
	}{
		{
			name:     "chunk on newline",
			input:    "Hello\nworld",
			size:     350,
			expected: "Hello\n",
		},
		{
			name:     "chunk on buffer size",
			input:    "Hello there",
			size:     7,
			expected: "Hello t",
		},
		{
			name:     "no chunk",
			input:    "Hello",
			size:     10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			c := &Chunker{
				Length:      tt.size,
				chunkBuffer: &bytes.Buffer{},
			}
			c.chunkBuffer.WriteString(tt.input)

			chunk, chunked := c.chunk()
			if chunked && string(chunk) != string(tt.expected) {
				t.Errorf("Chunk() got = %v, want = %v", chunk, tt.expected)
			}
		})
	}
}

func generateRandomText(size int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789                   ............!?\n"
	result := make([]byte, size)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result) + "\n"
}

func BenchmarkFilter_StressTest(b *testing.B) {
	// Test with different buffer sizes
	bufferSizes := []int{1000, 10000, 10000}
	Config := NewConfiguration()
	for _, bufSize := range bufferSizes {
		// Generate random text
		text := generateRandomText(bufSize)
		b.ResetTimer()
		b.Run(fmt.Sprintf("StressTest_BufferSize_%d", bufSize), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				Config.Session.ChunkMax = 50
				Config.API.Stream = false
				c := NewChunker(Config)

				// create streamresponse
				msgsr := StreamResponse{
					ChatCompletionStreamChoice: ai.ChatCompletionStreamChoice{
						Delta: ai.ChatCompletionStreamChoiceDelta{
							Role:    ai.ChatMessageRoleUser,
							Content: text,
						},
					},
				}

				// create tool streamresponse
				toolsr := StreamResponse{
					ChatCompletionStreamChoice: ai.ChatCompletionStreamChoice{
						Delta: ai.ChatCompletionStreamChoiceDelta{
							Role: ai.ChatMessageRoleAssistant,
							ToolCalls: []ai.ToolCall{
								{
									Function: ai.FunctionCall{
										Name:      "get_current_date_with_format",
										Arguments: `{"format": "+%A %B %d %T %Y"}`,
									},
									ID: "12354",
								},
							},
						},
					},
				}

				respch := make(chan StreamResponse, 10)

				respch <- msgsr
				respch <- msgsr
				respch <- toolsr
				cc, tc, ccm := c.FilterTask(respch)
				close(respch)
				ccount := 0
				tcount := 0
				ccmcount := 0
				for range cc {
					ccount++
				}

				for range tc {
					tcount++
				}

				for range ccm {
					ccmcount++
				}

				if ccmcount != 2 {
					b.Errorf("Expected 2 chat completion messages, got %d", ccmcount)
				}

				if ccount < 1 {
					b.Errorf("Expected >=1 chat text, got %d", ccount)
				}
				if tcount != 1 {
					b.Errorf("Expected 1 tool got %d", tcount)
				}

			}
		})
	}
}
