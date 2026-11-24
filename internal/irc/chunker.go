package irc

import (
	"bytes"
	"strings"
	"unicode/utf8"
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
	if maxChunkSize <= 0 {
		maxChunkSize = defaultChunkSize
	}
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

	// One write can overshoot by many chunks.
	for c.buffer.Len() >= c.maxChunkSize {
		chunk := c.extractBestSplitChunk()
		if chunk == "" {
			break
		}
		c.output <- chunk
	}
}

// writeLine emits a line, split at word boundaries if too long.
func (c *Chunker) writeLine(line string) {
	data := []byte(line)
	for len(data) > c.maxChunkSize {
		end, consumed := splitChunk(data, c.maxChunkSize)
		if consumed == 0 {
			break
		}
		c.output <- string(data[:end])
		data = bytes.TrimLeft(data[consumed:], " ")
	}
	if len(data) > 0 {
		c.output <- string(data)
	}
}

func (c *Chunker) extractBestSplitChunk() string {
	if c.buffer.Len() == 0 {
		return ""
	}

	data := c.buffer.Bytes()
	end, consumed := splitChunk(data, c.maxChunkSize)
	chunk := string(data[:end])
	c.buffer.Next(consumed)
	return chunk
}

// splitChunk prefers a word boundary. It holds back a trailing partial rune,
// and lets one rune exceed a smaller limit so output always progresses.
func splitChunk(data []byte, limit int) (end, consumed int) {
	for end < len(data) {
		if !utf8.FullRune(data[end:]) {
			break
		}
		_, width := utf8.DecodeRune(data[end:])
		if end > 0 && end+width > limit {
			break
		}
		end += width
		if end >= limit {
			break
		}
	}
	if idx := bytes.LastIndexByte(data[:end], ' '); idx > 0 {
		return idx, idx + 1
	}
	return end, end
}

// Flush emits the remaining buffer in limit-sized chunks.
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
