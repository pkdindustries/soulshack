package main

import (
	"bytes"
	"fmt"
	"log"
	"time"

	"github.com/neurosnap/sentences"
	ai "github.com/sashabaranov/go-openai"
)

type Chunker struct {
	Delay  time.Duration
	Length int
	Last   time.Time
	// enable streaming input
	Stream bool
	// enable tool calls
	Tools bool
	// ToolsInline     bool
	// ToolTag         string
	Tokenizer       *sentences.DefaultSentenceTokenizer
	buffer          *bytes.Buffer
	toolBuffer      *bytes.Buffer
	partialToolCall *ai.ToolCall
}

// NewChunker creates a new Chunker instance.
func NewChunker() *Chunker {
	log.Println("chunk: creating new chunker")
	return &Chunker{
		Length: Config.ChunkMax,
		Delay:  Config.ChunkDelay,
		Last:   time.Now(),
		Tools:  Config.Tools,
		Stream: Config.Stream,
		// ToolTag: Config.ToolTag,
		// ToolsInline: Config.ToolsInline,
		Tokenizer:  Config.Tokenizer,
		buffer:     &bytes.Buffer{},
		toolBuffer: &bytes.Buffer{},
	}
}

// FilterTask reads from the input channel and returns two channels:
// one with chunked responses and one with tool calls.
func (c *Chunker) FilterTask(msgChan <-chan StreamResponse) (<-chan ChatText, <-chan ai.ToolCall) {
	chunkedChan := make(chan ChatText, 1000)
	toolChan := make(chan ai.ToolCall, 1000)
	textChan := make(chan []byte, 1000)

	go c.processToolCalls(msgChan, textChan, toolChan)
	go c.processChunks(textChan, chunkedChan)

	return chunkedChan, toolChan
}

func (c *Chunker) processToolCalls(messageChan <-chan StreamResponse, contentChan chan<- []byte, toolChan chan<- ai.ToolCall) {
	defer close(toolChan)
	defer close(contentChan)

	log.Println("chunk: processing tool calls &")
	for val := range messageChan {
		if val.Err != nil {
			contentChan <- []byte(fmt.Sprintf("%s\n", val.Err.Error()))
			continue
		}
		choice := val.Message

		if c.Stream {
			if len(choice.Delta.ToolCalls) > 0 {
				toolCall := &choice.Delta.ToolCalls[0]
				if c.partialToolCall == nil {
					// Begin assembling a new tool call
					c.partialToolCall = toolCall
					c.toolBuffer.Reset()
				}
				// Write tool call arguments as bytes
				c.toolBuffer.Write([]byte(toolCall.Function.Arguments))
			}

			if choice.Delta.Content != "" {
				// Send content as bytes
				contentChan <- []byte(choice.Delta.Content)
			}

			if choice.FinishReason == ai.FinishReasonToolCalls {
				// Tool call is fully assembled
				c.partialToolCall.Function.Arguments = c.toolBuffer.String()
				toolChan <- *c.partialToolCall
				c.partialToolCall = nil
				c.toolBuffer.Reset()
			}
		} else {
			for _, toolCall := range choice.Delta.ToolCalls {
				toolChan <- toolCall
			}
			contentChan <- []byte(choice.Delta.Content)
		}
	}

}

// processChunks reads data from contentChan, writes it to the buffer, and triggers chunking.
func (c *Chunker) processChunks(contentChan <-chan []byte, chunkedChan chan<- ChatText) {
	defer close(chunkedChan)
	log.Println("chunk: processing byte chunks &")

	for content := range contentChan {
		c.buffer.Write(content)
		for {
			chunk, chunked := c.chunk()
			if !chunked {
				break
			}
			chunkedChan <- ChatText{
				Text: string(chunk),
				Role: ai.ChatMessageRoleAssistant,
			}
		}
	}
}

// chunk decides the method of chunking based on the current state of the buffer.
func (c *Chunker) chunk() ([]byte, bool) {
	if c.buffer.Len() == 0 {
		return nil, false
	}

	end := c.Length
	if c.buffer.Len() < end {
		end = c.buffer.Len()
	}
	content := c.buffer.Bytes()[:end]

	// Attempt to chunk by newline character.
	index := bytes.IndexByte(content, '\n')
	if index != -1 {
		return c.readChunk(index + 1), true
	}

	// Attempt to chunk by sentence boundary if delay has passed.
	if time.Since(c.Last) >= c.Delay {
		index = c.sentenceBoundary(content)
		if index != -1 {
			return c.readChunk(index), true
		}
	}

	// Chunk by maximum length if no other boundary is found.
	if c.buffer.Len() >= c.Length {
		return c.readChunk(c.Length), true
	}

	return nil, false
}

// readChunk extracts a chunk of bytes from the buffer and updates the last chunk time.
func (c *Chunker) readChunk(n int) []byte {
	chunk := c.buffer.Next(n)
	c.Last = time.Now()
	return chunk
}

// sentenceBoundary finds the end index of the first sentence in the buffer.
func (c *Chunker) sentenceBoundary(s []byte) int {
	log.Println("chunk: attempting to chunk by sentence boundary")
	text := string(s)
	sentences := c.Tokenizer.Tokenize(text)
	if len(sentences) > 1 {
		// Use the End field to get the end index of the first sentence
		return sentences[0].End
	}
	return -1
}
