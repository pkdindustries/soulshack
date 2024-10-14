package main

import (
	"log"
	"strings"
	"time"

	"github.com/neurosnap/sentences"
	ai "github.com/sashabaranov/go-openai"
)

type Chunker struct {
	Delay           time.Duration
	Length          int
	Last            time.Time
	Tokenizer       *sentences.DefaultSentenceTokenizer
	buffer          strings.Builder
	toolBuffer      strings.Builder
	partialToolCall *ai.ToolCall
}

func NewChunker() *Chunker {
	return &Chunker{
		Length:    Config.ChunkMax,
		Delay:     Config.ChunkDelay,
		Last:      time.Now(),
		Tokenizer: Config.Tokenizer,
	}
}

func (c *Chunker) Filter(messageChan <-chan StreamResponse) (<-chan StreamResponse, <-chan ai.ToolCall) {
	chunkedChan := make(chan StreamResponse, 10)
	toolChan := make(chan ai.ToolCall, 10)
	contentChan := make(chan string, 10)

	go c.processToolCalls(messageChan, contentChan, toolChan)
	go c.processChunks(contentChan, chunkedChan)

	return chunkedChan, toolChan
}

func (c *Chunker) processToolCalls(messageChan <-chan StreamResponse, contentChan chan<- string, toolChan chan<- ai.ToolCall) {
	defer close(toolChan)
	defer close(contentChan)
	// messagechan closed by caller

	for val := range messageChan {
		if val.Err != nil {
			// Handle error, possibly send it to chunkedChan if needed
			continue
		}

		choice := val.Message
		if len(choice.Delta.ToolCalls) > 0 {
			toolCall := &choice.Delta.ToolCalls[0]
			if c.partialToolCall == nil {
				log.Println("assembling tool call:", toolCall)
				c.partialToolCall = toolCall
				c.toolBuffer.Reset()
			}
			c.toolBuffer.WriteString(toolCall.Function.Arguments)
		}

		if choice.Delta.Content != "" {
			contentChan <- choice.Delta.Content
		}

		if choice.FinishReason == ai.FinishReasonToolCalls {
			c.partialToolCall.Function.Arguments = c.toolBuffer.String()
			log.Println("Tool call assembled:", c.partialToolCall)
			toolChan <- *c.partialToolCall
			c.partialToolCall = nil
			c.toolBuffer.Reset()
		}
	}
}

func (c *Chunker) processChunks(contentChan <-chan string, chunkedChan chan<- StreamResponse) {
	defer close(chunkedChan)
	for content := range contentChan {
		c.buffer.WriteString(content)

		for {
			chunk, chunked := c.chunk()
			if !chunked {
				break
			}
			chunkedChan <- StreamResponse{
				Message: ai.ChatCompletionStreamChoice{
					Delta: ai.ChatCompletionStreamChoiceDelta{
						Role:    ai.ChatMessageRoleAssistant,
						Content: chunk,
					},
				},
			}
		}
	}
}

func (c *Chunker) chunk() (string, bool) {
	if c.buffer.Len() == 0 {
		return "", false
	}

	end := c.Length
	if c.buffer.Len() < end {
		end = c.buffer.Len()
	}
	content := c.buffer.String()[:end]

	// Attempt to chunk by newline character.
	index := strings.IndexByte(content, '\n')
	if index != -1 {
		return c.readChunk(index + 1), true
	}

	// Attempt to chunk by sentence boundary if delay has passed.
	if time.Since(c.Last) >= c.Delay {
		index = c.sentenceBoundary(content)
		if index != -1 {
			return c.readChunk(index + 1), true
		}
	}

	// Chunk by maximum length if no other boundary is found.
	if c.buffer.Len() >= c.Length {
		return c.readChunk(c.Length), true
	}

	return "", false
}

func (c *Chunker) readChunk(n int) string {
	chunk := c.buffer.String()[:n]
	remaining := c.buffer.String()[n:]
	c.buffer.Reset()
	c.buffer.WriteString(remaining)
	c.Last = time.Now()
	return chunk
}

func (c *Chunker) sentenceBoundary(s string) int {
	sentences := c.Tokenizer.Tokenize(s)
	if len(sentences) > 1 {
		return len(sentences[0].Text)
	}
	return -1
}
