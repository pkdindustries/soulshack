package main

import (
	"bytes"
	"log"
	"time"

	ai "github.com/sashabaranov/go-openai"
)

type Chunker struct {
	Delay           time.Duration
	Length          int
	Last            time.Time
	Stream          bool
	buffer          *bytes.Buffer
	toolBuffer      *bytes.Buffer
	partialToolCall *ai.ToolCall
}

func NewChunker(config *Configuration) *Chunker {
	return &Chunker{
		Length:     config.Session.ChunkMax,
		Delay:      config.Session.ChunkDelay,
		Last:       time.Now(),
		Stream:     config.API.Stream,
		buffer:     &bytes.Buffer{},
		toolBuffer: &bytes.Buffer{},
	}
}

func (c *Chunker) FilterTask(msgChan <-chan StreamResponse) (<-chan ChatText, <-chan ai.ToolCall) {
	var byteChan <-chan []byte
	var toolChan <-chan ai.ToolCall

	if c.Stream {
		byteChan, toolChan = c.streamTask(msgChan)
	} else {
		byteChan, toolChan = c.nonStreamTask(msgChan)
	}

	chatChan := c.chunkTask(byteChan)
	return chatChan, toolChan
}

func (c *Chunker) streamTask(chatChan <-chan StreamResponse) (chan []byte, chan ai.ToolCall) {
	toolChan := make(chan ai.ToolCall, 1000)
	byteChan := make(chan []byte, 1000)

	go func() {
		defer close(toolChan)
		defer close(byteChan)

		log.Println("streamTask: start")
		for val := range chatChan {

			if len(val.Delta.ToolCalls) > 0 {
				toolCall := &val.Delta.ToolCalls[0]
				if c.partialToolCall == nil {
					c.partialToolCall = toolCall
					c.toolBuffer.Reset()
				}
				c.toolBuffer.Write([]byte(toolCall.Function.Arguments))
			}

			byteChan <- []byte(val.Delta.Content)

			if val.FinishReason == ai.FinishReasonToolCalls {
				log.Println("streamTask: tool call assembled")
				c.partialToolCall.Function.Arguments = c.toolBuffer.String()
				toolChan <- *c.partialToolCall
				c.partialToolCall = nil
				c.toolBuffer.Reset()
			} else if val.FinishReason == ai.FinishReasonStop {
				if c.partialToolCall != nil {
					log.Println("streamTask: incomplete tool call")
					c.partialToolCall = nil
					c.toolBuffer.Reset()
				}
			}
		}
		log.Println("streamTask: done")
	}()

	return byteChan, toolChan
}

func (c *Chunker) nonStreamTask(chatChan <-chan StreamResponse) (chan []byte, chan ai.ToolCall) {
	toolChan := make(chan ai.ToolCall, 1000)
	byteChan := make(chan []byte, 1000)

	go func() {
		defer close(toolChan)
		defer close(byteChan)

		log.Println("nonStreamTask: start")
		for val := range chatChan {

			for _, toolCall := range val.Delta.ToolCalls {
				log.Println("nonStreamTask: tool call assembled")
				toolChan <- toolCall
			}
			byteChan <- []byte(val.Delta.Content)

		}
		log.Println("nonStreamTask: done")
	}()

	return byteChan, toolChan
}

// reads a channel of byte slices and chunks them into irc client sized ChatText messages
func (c *Chunker) chunkTask(byteChan <-chan []byte) <-chan ChatText {
	chatChan := make(chan ChatText, 1000)
	log.Println("chunkTask: start")
	go func() {
		defer close(chatChan)
		for content := range byteChan {
			c.buffer.Write(content)
			for {
				chunk, chunked := c.chunk()
				if !chunked {
					break
				}
				log.Println("chunkTask: sending chunk")
				chatChan <- ChatText{
					Text: string(chunk),
					Role: ai.ChatMessageRoleAssistant,
				}
			}
		}
		log.Println("chunkTask: flushing buffer")
		chatChan <- ChatText{
			Text: string(c.buffer.String()),
			Role: ai.ChatMessageRoleAssistant,
		}

		log.Println("chunkTask: done")
	}()
	return chatChan
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
