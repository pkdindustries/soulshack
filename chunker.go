package main

import (
	"bytes"
	"log"
	"time"

	ai "github.com/sashabaranov/go-openai"
)

type Chunker struct {
	Length          int
	Last            time.Time
	Stream          bool
	chunkBuffer     *bytes.Buffer
	toolBuffer      *bytes.Buffer
	contentBuffer   *bytes.Buffer
	partialToolCall *ai.ToolCall
}

func NewChunker(config *Configuration) *Chunker {
	return &Chunker{
		Length:        config.Session.ChunkMax,
		Last:          time.Now(),
		Stream:        config.API.Stream,
		chunkBuffer:   &bytes.Buffer{},
		toolBuffer:    &bytes.Buffer{},
		contentBuffer: &bytes.Buffer{},
	}
}

func (c *Chunker) FilterTask(msgChan <-chan StreamResponse) (<-chan []byte, <-chan *ai.ToolCall, <-chan *ai.ChatCompletionMessage) {
	var byteChan <-chan []byte
	var toolChan <-chan *ai.ToolCall
	var ccmChan <-chan *ai.ChatCompletionMessage

	if c.Stream {
		byteChan, toolChan, ccmChan = c.streamTask(msgChan)
	} else {
		byteChan, toolChan, ccmChan = c.nonStreamTask(msgChan)
	}

	chatChan := c.chunkTask(byteChan)
	return chatChan, toolChan, ccmChan
}

func (c *Chunker) streamTask(respChan <-chan StreamResponse) (chan []byte, chan *ai.ToolCall, chan *ai.ChatCompletionMessage) {
	toolChan := make(chan *ai.ToolCall, 10)
	byteChan := make(chan []byte, 10)
	ccmChan := make(chan *ai.ChatCompletionMessage, 10)
	go func() {
		defer close(toolChan)
		defer close(byteChan)
		defer close(ccmChan)
		log.Println("streamTask: start")
		for val := range respChan {

			if len(val.Delta.ToolCalls) > 0 {
				toolCall := &val.Delta.ToolCalls[0]
				if c.partialToolCall == nil {
					c.partialToolCall = toolCall
					c.toolBuffer.Reset()
				}
				c.toolBuffer.Write([]byte(toolCall.Function.Arguments))
			}

			if val.Delta.Content != "" {
				byteChan <- []byte(val.Delta.Content)
				c.contentBuffer.Write([]byte(val.Delta.Content))
			}

			if val.FinishReason == ai.FinishReasonToolCalls {
				log.Println("streamTask: tool call assembled")
				c.partialToolCall.Function.Arguments = c.toolBuffer.String()
				toolChan <- c.partialToolCall
				c.partialToolCall = nil
				c.toolBuffer.Reset()
			} else if val.FinishReason == ai.FinishReasonStop {
				if c.partialToolCall != nil {
					log.Println("streamTask: incomplete tool call")
					c.partialToolCall = nil
					c.toolBuffer.Reset()
				}
				ccmChan <- &ai.ChatCompletionMessage{
					Role:    ai.ChatMessageRoleAssistant,
					Content: c.contentBuffer.String(),
				}
			}
		}
		log.Println("streamTask: done")
	}()

	return byteChan, toolChan, ccmChan
}

func (c *Chunker) nonStreamTask(chatChan <-chan StreamResponse) (chan []byte, chan *ai.ToolCall, chan *ai.ChatCompletionMessage) {
	toolChan := make(chan *ai.ToolCall, 10)
	byteChan := make(chan []byte, 10)
	ccmChan := make(chan *ai.ChatCompletionMessage, 10)
	go func() {
		defer close(toolChan)
		defer close(byteChan)
		defer close(ccmChan)
		log.Println("nonStreamTask: start")
		for val := range chatChan {

			for _, toolCall := range val.Delta.ToolCalls {
				log.Println("nonStreamTask: tool call assembled")
				toolChan <- &toolCall
			}

			if val.Delta.Content != "" {
				byteChan <- []byte(val.Delta.Content)
				ccmChan <- &ai.ChatCompletionMessage{
					Role:      val.Delta.Role,
					Content:   val.Delta.Content,
					ToolCalls: val.Delta.ToolCalls,
				}
			}
		}
		log.Println("nonStreamTask: done")
	}()

	return byteChan, toolChan, ccmChan
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
		chatChan <- c.chunkBuffer.Bytes()
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
