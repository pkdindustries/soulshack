package main

import (
	"bytes"
	"log"
	"time"

	ai "github.com/sashabaranov/go-openai"
)

type Chunker struct {
	Length        int
	Last          time.Time
	chunkBuffer   *bytes.Buffer
	contentBuffer *bytes.Buffer
}

func NewChunker(config *Configuration) *Chunker {
	return &Chunker{
		Length:        config.Session.ChunkMax,
		Last:          time.Now(),
		chunkBuffer:   &bytes.Buffer{},
		contentBuffer: &bytes.Buffer{},
	}
}

func (c *Chunker) ProcessMessages(msgChan <-chan ai.ChatCompletionMessage) (<-chan []byte, <-chan *ToolCall, <-chan *ai.ChatCompletionMessage) {
	toolChan := make(chan *ToolCall, 10)
	byteChan := make(chan []byte, 10)
	ccmChan := make(chan *ai.ChatCompletionMessage, 10)

	go func() {
		defer close(toolChan)
		defer close(byteChan)
		defer close(ccmChan)
		log.Println("processMessages: start")

		for msg := range msgChan {
			// Handle tool calls
			for _, toolCall := range msg.ToolCalls {
				log.Println("processMessages: tool call found")
				// Convert OpenAI tool call to generic format
				if tc, err := ParseOpenAIToolCall(toolCall); err == nil {
					toolChan <- tc
				} else {
					log.Printf("processMessages: failed to parse tool call: %v", err)
				}
			}

			// Handle content: if this message includes tool calls, defer
			// content emission to the higher-level handler to preserve
			// ordering (content first, then tool execution). Otherwise, emit
			// the content normally for streaming/chunking.
			if msg.Content != "" && len(msg.ToolCalls) == 0 {
				byteChan <- []byte(msg.Content)
			}

			// Pass through the complete message
			ccmChan <- &msg
		}
		log.Println("processMessages: done")
	}()

	return c.chunkTask(byteChan), toolChan, ccmChan
}

// reads a channel of byte slices and chunks them into irc client sized ChatText messages
func (c *Chunker) chunkTask(byteChan <-chan []byte) <-chan []byte {
	chatChan := make(chan []byte, 10)
	log.Println("chunkTask: start")
	go func() {
		defer close(chatChan)
		for content := range byteChan {
			c.chunkBuffer.Write([]byte(content))
			for {
				chunk, chunked := c.chunk()
				if !chunked {
					break
				}
				chatChan <- chunk
			}
		}
		log.Println("chunkTask: flushing buffer")
		if c.chunkBuffer.Len() > 0 {
			chatChan <- c.chunkBuffer.Bytes()
		}
		log.Println("chunkTask: done")
	}()
	return chatChan
}

// chunk decides the method of chunking based on the current state of the buffer.
func (c *Chunker) chunk() ([]byte, bool) {
	if c.chunkBuffer.Len() == 0 {
		return nil, false
	}

	end := c.Length
	if c.chunkBuffer.Len() < end {
		end = c.chunkBuffer.Len()
	}
	content := c.chunkBuffer.Bytes()[:end]

	// Attempt to chunk by newline character.
	index := bytes.IndexByte(content, '\n')
	if index != -1 {
		return c.readChunk(index + 1), true
	}

	// Chunk by maximum length if no other boundary is found.
	if c.chunkBuffer.Len() >= c.Length {
		return c.readChunk(c.Length), true
	}

	return nil, false
}

// readChunk extracts a chunk of bytes from the buffer and updates the last chunk time.
func (c *Chunker) readChunk(n int) []byte {
	chunk := c.chunkBuffer.Next(n)
	c.Last = time.Now()
	return chunk
}
