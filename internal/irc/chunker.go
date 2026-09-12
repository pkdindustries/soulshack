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
			c.writeLine(line)
		}
	}

	// Force chunks until the buffer is back under the limit: one write can
	// overshoot it by far more than a single chunk.
	for c.buffer.Len() >= c.maxChunkSize {
		chunk := c.extractBestSplitChunk()
		if chunk == "" {
			break
		}
		c.output <- chunk
	}
}

// writeLine emits a complete line, splitting it when it is longer than one
// message can carry. Like extractBestSplitChunk, it prefers a word boundary.
func (c *Chunker) writeLine(line string) {
	for len(line) > c.maxChunkSize {
		split := c.maxChunkSize
		if idx := strings.LastIndexByte(line[:split], ' '); idx > 0 {
			split = idx
		}
		c.output <- line[:split]
		line = strings.TrimLeft(line[split:], " ")
	}
	if line != "" {
		c.output <- line
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

// Flush emits any remaining buffer content, still respecting the message size
// limit.
func (c *Chunker) Flush() {
	for c.buffer.Len() > c.maxChunkSize {
		chunk := c.extractBestSplitChunk()
		if chunk == "" {
			break
		}
		c.output <- chunk
	}
	if c.buffer.Len() > 0 {
		c.output <- c.buffer.String()
		c.buffer.Reset()
	}
}
