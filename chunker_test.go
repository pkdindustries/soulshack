package main

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/neurosnap/sentences/english"
)

func TestChunker_Chunk(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		size     int
		expected []byte
	}{
		{
			name:     "chunk on newline",
			input:    "Hello\nworld",
			size:     350,
			expected: []byte("Hello\n"),
		},
		{
			name:     "chunk on buffer size",
			input:    "Hello",
			size:     5,
			expected: []byte("Hello"),
		},
		{
			name:     "no chunk",
			input:    "Hello",
			size:     10,
			expected: []byte(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Chunker{
				Length: tt.size,
				Last:   time.Now(),
				buffer: strings.Builder{},
				Delay:  1000 * time.Millisecond,
			}
			c.buffer.WriteString(tt.input)

			chunk, chunked := c.chunk()
			if chunked && string(chunk) != string(tt.expected) {
				t.Errorf("Chunk() got = %v, want = %v", chunk, tt.expected)
			}
		})
	}
}

// // Test for chunking based on timeout
func TestChunker_Chunk_Timeout(t *testing.T) {
	timeout := 100 * time.Millisecond
	tokenizer, err := english.NewSentenceTokenizer(nil)
	if err != nil {
		t.Fatalf("failed to create sentence tokenizer: %v", err)
	}
	c := &Chunker{
		Length:    50,
		Last:      time.Now(),
		buffer:    strings.Builder{},
		Delay:     timeout,
		Tokenizer: tokenizer,
	}
	c.buffer.WriteString("Hello world! How are you?")

	// Wait for timeout duration
	time.Sleep(500 * time.Millisecond)

	chunk, chunked := c.chunk()
	expected := []byte("Hello world! ")
	if chunked && string(chunk) != string(expected) {
		t.Errorf("Chunk() got = %v, want = %v", chunk, expected)
	}
}

func generateRandomText(size int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789                   ............!?\n"
	result := make([]byte, size)
	for i := range result {
		result[i] = charset[rand.Intn(len(charset))]
	}
	return string(result)
}

func BenchmarkChunker_StressTest(b *testing.B) {
	timeout := 1 * time.Nanosecond
	// Test with different buffer sizes
	bufferSizes := []int{500, 1000, 5000, 10000}

	for _, bufSize := range bufferSizes {
		// Generate random text
		text := generateRandomText(bufSize)
		b.ResetTimer()
		b.Run(fmt.Sprintf("StressTest_BufferSize_%d", bufSize), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				c := &Chunker{
					Length: 40,
					Last:   time.Now(),
					buffer: strings.Builder{},
					Delay:  timeout,
				}
				c.buffer.WriteString(text)

				// Continuously call Chunk() until no chunks are left
				for _, chunked := c.chunk(); chunked; _, chunked = c.chunk() {
				}
			}
		})
	}
}
