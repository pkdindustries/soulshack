package irc

import (
	"bytes"
	"strings"
)

// Chunker handles chunking of content for IRC message limits.
// It buffers content and emits complete lines or chunks when the buffer
// exceeds the maximum chunk size.
type Chunker struct {
	output       chan<- string
	buffer       *bytes.Buffer
	maxChunkSize int
}

// NewChunker creates a new IRC chunker that writes to the given output channel.
func NewChunker(output chan<- string, maxChunkSize int) *Chunker {
	return &Chunker{
		output:       output,
		buffer:       &bytes.Buffer{},
		maxChunkSize: maxChunkSize,
	}
}

// Write adds content to the buffer and emits complete lines immediately.
// If the buffer grows too large, it forces a chunk to be emitted.
func (c *Chunker) Write(content string) {
	c.buffer.WriteString(content)

	// Emit complete lines immediately
	for {
		line, err := c.buffer.ReadString('\n')
		if err != nil {
			// No more complete lines, put back what we read
			if line != "" {
				c.buffer.WriteString(line)
			}
			break
		}
		// Remove the newline and send
		if line = strings.TrimSuffix(line, "\n"); line != "" {
			c.output <- line
		}
	}

	// If buffer is getting too large, force a chunk
	if c.buffer.Len() >= c.maxChunkSize {
		chunk := c.extractBestSplitChunk()
		if chunk != "" {
			c.output <- chunk
		}
	}
}

func (c *Chunker) extractBestSplitChunk() string {
	if c.buffer.Len() == 0 {
		return ""
	}

	data := c.buffer.Bytes()
	end := min(c.maxChunkSize, len(data))

	// Try to find a space within the allowed range to break cleanly
	if idx := bytes.LastIndexByte(data[:end], ' '); idx > 0 {
		chunk := string(data[:idx])
		c.buffer.Next(idx + 1) // Skip the space itself
		return chunk
	}

	// If no space is found, hard break at maxChunkSize
	chunk := string(data[:end])
	c.buffer.Next(end)
	return chunk
}

// Flush emits any remaining buffer content.
func (c *Chunker) Flush() {
	if c.buffer.Len() > 0 {
		c.output <- c.buffer.String()
		c.buffer.Reset()
	}
}
