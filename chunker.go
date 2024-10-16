package main

import (
	"bytes"
	"encoding/json"
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
		// ToolTag: Config.ToolTag,
		// ToolsInline: Config.ToolsInline,
		Tokenizer:  Config.Tokenizer,
		buffer:     &bytes.Buffer{},
		toolBuffer: &bytes.Buffer{},
	}
}

// Filter reads from the input channel and returns two channels:
// one with chunked responses and one with tool calls.
func (c *Chunker) Filter(msgChan <-chan StreamResponse) (<-chan StreamResponse, <-chan ai.ToolCall) {
	chunkedChan := make(chan StreamResponse, 1000)
	toolChan := make(chan ai.ToolCall, 1000)
	contentChan := make(chan []byte, 1000)

	go c.processToolCalls(msgChan, contentChan, toolChan)
	go c.processChunks(contentChan, chunkedChan)

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
	}
}

// processChunks reads data from contentChan, writes it to the buffer, and triggers chunking.
func (c *Chunker) processChunks(contentChan <-chan []byte, chunkedChan chan<- StreamResponse) {
	defer close(chunkedChan)
	log.Println("chunk: processing byte chunks &")

	for content := range contentChan {
		c.buffer.Write(content)
		for {
			chunk, chunked := c.chunk()
			if !chunked {
				break
			}
			chunkedChan <- StreamResponse{
				Message: ai.ChatCompletionStreamChoice{
					Delta: ai.ChatCompletionStreamChoiceDelta{
						Role:    ai.ChatMessageRoleAssistant,
						Content: string(chunk),
					},
				},
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
		c.Last = time.Now()
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

// or we just wait for https://github.com/ollama/ollama/issues/6790
func (c *Chunker) processInlineTools(_ <-chan StreamResponse, _ chan<- []byte, _ chan<- ai.ToolCall) {
	log.Fatalf("processInlineTools: not implemented")
}

func createStartTag(baseName string) string {
	return "<" + baseName + ">"
}

func createEndTag(baseName string) string {
	return "</" + baseName + ">"
}

func toolCallFromBuffer(buffer *bytes.Buffer) (*ai.ToolCall, error) {
	tool := map[string]interface{}{}
	err := json.Unmarshal(buffer.Bytes(), &tool)
	if err != nil {
		return nil, fmt.Errorf("toolcallFromBuffer: map tool: %v", err)
	}

	toolCall := ai.ToolCall{}
	if name, exists := tool["name"].(string); exists {
		toolCall.Function.Name = name
	} else {
		return nil, fmt.Errorf("toolcallFromBuffer: failed to get name: %v", err)
	}

	if id, exists := tool["id"].(string); exists {
		toolCall.ID = id
	} else {
		log.Printf("toolcallFromBuffer: failed get id: %v", err)
	}

	// Extract the params field as a JSON string
	params, paramexists := tool["parameters"]
	args, argsexists := tool["arguments"]

	if !paramexists && !argsexists {
		return nil, fmt.Errorf("toolcallFromBuffer: tool failed to get arguments")
	}
	// Use the parameters field if it exists, otherwise use the arguments field
	var finalargs any
	if paramexists {
		finalargs = params
	} else {
		finalargs = args
	}

	argumentsJSON, err := json.Marshal(finalargs)
	if err != nil {
		return nil, fmt.Errorf("toolcallFromBuffer: failed to marshal arguments: %v", err)
	}

	toolCall.Function.Arguments = string(argumentsJSON)

	log.Printf("callfrombuffer: buffer tool call assembled: %v\n", toolCall)
	return &toolCall, nil
}
