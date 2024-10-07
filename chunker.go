package main

import (
	"bytes"
	"log"
	"time"

	"github.com/neurosnap/sentences"
	"github.com/neurosnap/sentences/english"
)

// Chunker handles splitting a stream of text into smaller chunks.
type Chunker struct {
	Buffer *bytes.Buffer // Buffer to hold incoming data
	Delay  time.Duration // Time delay before forced chunk
	Length int           // Maximum chunk size
	Quote  bool          // Flag for handling code blocks
	Last   time.Time     // Timestamp of the last chunk
}

// ChunkingFilter reads from the input channel and returns a channel with chunked responses.
func (c *Chunker) ChunkingFilter(input <-chan StreamResponse) <-chan StreamResponse {
	out := make(chan StreamResponse, 10)
	go c.processChunks(input, out)
	return out
}

// processChunks reads data from the input channel, writes it to the buffer, and triggers chunking.
func (c *Chunker) processChunks(input <-chan StreamResponse, out chan<- StreamResponse) {
	defer close(out)
	for val := range input {
		if val.Err != nil {
			out <- StreamResponse{Err: val.Err}
			continue
		}
		c.Buffer.WriteString(val.Content)
		c.chunker(out)
	}
}

// chunker repeatedly checks the buffer for chunks to send out.'
func (c *Chunker) chunker(out chan<- StreamResponse) {
	for {
		if chunk, chunked := c.chunk(); chunked {
			out <- StreamResponse{Content: string(chunk)}
		} else {
			break
		}
	}
}

// chunk decides the method of chunking based on the current state of the buffer.
func (c *Chunker) chunk() ([]byte, bool) {
	if c.Buffer.Len() == 0 {
		return nil, false
	}

	// If Delay is -1, chunk the entire buffer.
	if c.Delay == -1 {
		return c.readChunk(c.Buffer.Len()), true
	}

	// Handle code blocks if the Quote flag is set.
	if c.Quote && bytes.Contains(c.Buffer.Bytes(), []byte("```")) {
		if chunk, ok := c.chunkByBlockQuote(); ok {
			return chunk, true
		}
	}

	// Chunk by newline character.
	if chunk, ok := c.chunkByNewline(); ok {
		return chunk, true
	}

	// Chunk by max length.
	if chunk, ok := c.chunkByMaxLength(); ok {
		return chunk, true
	}

	// Chunk by sentence boundary if delay has passed.
	if time.Since(c.Last) >= c.Delay {
		if chunk, ok := c.chunkBySentenceBoundary(); ok {
			return chunk, true
		}
	}

	return nil, false
}

// chunkByBlockQuote detects and chunks by code block quotes.
func (c *Chunker) chunkByBlockQuote() ([]byte, bool) {
	content := c.Buffer.Bytes()
	blockStart := bytes.Index(content, []byte("```"))
	if blockStart != -1 {
		blockEnd := bytes.Index(content[blockStart+3:], []byte("```"))
		if blockEnd != -1 {
			return c.readChunk(blockStart + 3 + blockEnd + 3), true
		}
	}
	return nil, false
}

// chunkByNewline chunks the buffer up to a newline character.
func (c *Chunker) chunkByNewline() ([]byte, bool) {
	end := c.Length
	if c.Buffer.Len() < end {
		end = c.Buffer.Len()
	}
	index := bytes.IndexByte(c.Buffer.Bytes()[:end], '\n')
	if index != -1 {
		return c.readChunk(index + 1), true
	}
	return nil, false
}

// chunkByMaxLength chunks the buffer based on the maximum allowed length.
func (c *Chunker) chunkByMaxLength() ([]byte, bool) {
	if c.Buffer.Len() >= c.Length {
		return c.readChunk(c.Length), true
	}
	return nil, false
}

// chunkBySentenceBoundary chunks the buffer at a sentence boundary.
func (c *Chunker) chunkBySentenceBoundary() ([]byte, bool) {
	index := sentenceBoundary(c.Buffer.Bytes())
	if index != -1 {
		return c.readChunk(index + 1), true
	}
	return nil, false
}

// readChunk extracts a chunk of bytes from the buffer and updates the last chunk time.
func (c *Chunker) readChunk(n int) []byte {
	chunk := c.Buffer.Next(n)
	c.Last = time.Now()
	return chunk
}

// Tokenizer for sentence boundary detection
var tokenizer *sentences.DefaultSentenceTokenizer

// Initialize the tokenizer during package initialization.
func init() {
	t, err := english.NewSentenceTokenizer(nil)
	if err != nil {
		log.Fatal("Error creating tokenizer:", err)
	}
	tokenizer = t
}

// sentenceBoundary finds the end of the first sentence in the buffer.
func sentenceBoundary(s []byte) int {
	sentences := tokenizer.Tokenize(string(s))
	if len(sentences) > 1 {
		return len([]byte(sentences[0].Text))
	}
	return -1
}
