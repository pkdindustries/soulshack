package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/lrstanley/girc"
	vip "github.com/spf13/viper"
)

func TestChunker_ChunkFilter(t *testing.T) {
	timeout := 1000 * time.Millisecond

	tests := []struct {
		name     string
		input    []string
		size     int
		expected []string
	}{
		{
			name:     "chunk on newline",
			input:    []string{"Hello\nworld"},
			size:     350,
			expected: []string{"Hello\n"},
		},
		{
			name:     "chunk on buffer size",
			input:    []string{"Hello"},
			size:     5,
			expected: []string{"Hello"},
		},
		{
			name:     "no chunk",
			input:    []string{"Hello"},
			size:     10,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := make(chan *string, len(tt.input))
			for _, s := range tt.input {
				in <- &s
			}
			close(in)

			c := &Chunker{
				Chunkmax:   tt.size,
				Last:       time.Now(),
				Buffer:     &bytes.Buffer{},
				Chunkdelay: timeout,
			}

			out := c.ChannelFilter(in)

			var result []string
			for s := range out {
				result = append(result, s)
			}

			if len(result) != len(tt.expected) {
				t.Errorf("ChunkFilter() got = %v, want = %v", result, tt.expected)
			}

			for i := 0; i < len(result); i++ {
				if result[i] != tt.expected[i] {
					t.Errorf("ChunkFilter() got = %v, want = %v", result, tt.expected)
					break
				}
			}
		})
	}
}

func TestIrcContext_IsAdmin(t *testing.T) {

	vip.Set("admins", []string{"admin1", "admin2"})

	tests := []struct {
		name   string
		source string
		admins []string
		want   bool
	}{
		{
			name:   "admin",
			source: "admin1",
			admins: []string{"admin1", "admin2"},
			want:   true,
		},
		{
			name:   "non-admin",
			source: "non-admin",
			admins: []string{"admin1", "admin2"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &IrcContext{
				config: &IrcConfig{
					Admins: tt.admins,
				},
				event: &girc.Event{
					Source: &girc.Source{
						Name: tt.source,
					},
				},
			}
			if got := c.IsAdmin(); got != tt.want {
				t.Errorf("IsAdmin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestChunker_Chunk(t *testing.T) {
	timeout := 1000 * time.Millisecond

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
				Chunkmax:   tt.size,
				Last:       time.Now(),
				Buffer:     &bytes.Buffer{},
				Chunkdelay: timeout,
			}
			c.Buffer.WriteString(tt.input)

			chunked, chunk := c.chunk()
			if chunked && string(chunk) != string(tt.expected) {
				t.Errorf("Chunk() got = %v, want = %v", chunk, tt.expected)
			}
		})
	}
}

// Test for chunking based on timeout
func TestChunker_Chunk_Timeout(t *testing.T) {
	timeout := 100 * time.Millisecond

	c := &Chunker{
		Chunkmax:   50,
		Last:       time.Now(),
		Buffer:     &bytes.Buffer{},
		Chunkdelay: timeout,
	}
	c.Buffer.WriteString("Hello world! How are you?")

	// Wait for timeout duration
	time.Sleep(500 * time.Millisecond)

	chunked, chunk := c.chunk()
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
		count := 0
		leng := 0
		// Generate random text
		text := generateRandomText(bufSize)
		b.ResetTimer()
		b.Run(fmt.Sprintf("StressTest_BufferSize_%d", bufSize), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				c := &Chunker{
					Chunkmax:   40,
					Last:       time.Now(),
					Buffer:     &bytes.Buffer{},
					Chunkdelay: timeout,
				}
				c.Buffer.WriteString(text)

				// Continuously call Chunk() until no chunks are left
				for chunked, t := c.chunk(); chunked; chunked, t = c.chunk() {
					count++
					leng += len(t)
				}
			}
			b.Logf("Processed %d chunks", count)
			b.Logf("Total length of chunks: %d", leng)

		})
	}
}

func BenchmarkChunker_ChunkFilter(b *testing.B) {
	timeout := 1 * time.Nanosecond
	// Test with different buffer sizes
	bufferSizes := []int{500, 1000, 5000, 10000}

	for _, bufSize := range bufferSizes {
		// Generate random text
		count := 0
		leng := 0

		text := generateRandomText(bufSize)
		b.ResetTimer()
		b.Run(fmt.Sprintf("ChunkFilter_BufferSize_%d", bufSize), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				c := &Chunker{
					Chunkmax:   40,
					Last:       time.Now(),
					Buffer:     &bytes.Buffer{},
					Chunkdelay: timeout,
				}

				// Create an input channel and send the text to it
				input := make(chan *string, 1)

				input <- &text
				close(input)

				// Call ChunkFilter and measure the time it takes to process the input channel
				output := c.ChannelFilter(input)
				// Read all chunks from the output channel
				for t := range output {
					leng += len(t)
					count++
				}
			}
			b.Logf("Processed %d chunks", count)
			b.Logf("Total length of chunks: %d", leng)
		})
	}
}
