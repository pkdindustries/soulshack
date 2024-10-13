package main

import (
	"bytes"
	"log"
	"time"

	"github.com/neurosnap/sentences"
	ai "github.com/sashabaranov/go-openai"
)

type Chunker struct {
	Buffer          *bytes.Buffer
	Delay           time.Duration
	Length          int
	Quote           bool
	Last            time.Time
	Tokenizer       *sentences.DefaultSentenceTokenizer
	assemblingTool  bool
	partialToolCall ai.ToolCall
}

func NewChunker() *Chunker {
	return &Chunker{
		Buffer:    &bytes.Buffer{},
		Length:    Config.ChunkMax,
		Delay:     Config.ChunkDelay,
		Quote:     Config.ChunkQuoted,
		Last:      time.Now(),
		Tokenizer: Config.Tokenizer,
	}
}

func (c *Chunker) Filter(messageChan <-chan StreamResponse) (<-chan StreamResponse, <-chan ai.ToolCall) {
	chunkedChan := make(chan StreamResponse, 10)
	toolChan := make(chan ai.ToolCall, 10)
	go c.processChunks(messageChan, chunkedChan, toolChan)
	return chunkedChan, toolChan
}

func (c *Chunker) processChunks(messageChan <-chan StreamResponse, chunkedChan chan<- StreamResponse, toolChan chan<- ai.ToolCall) {
	defer close(chunkedChan)
	defer close(toolChan)
	for val := range messageChan {
		if val.Err != nil {
			chunkedChan <- StreamResponse{Err: val.Err}
			break
		}

		choice := val.Message
		if len(choice.Delta.ToolCalls) > 0 {
			c.assemblingTool, c.partialToolCall = c.assembleToolCall(choice.Delta.ToolCalls[0])
		}

		c.Buffer.WriteString(choice.Delta.Content)

		for {
			if chunk, chunked := c.chunk(); chunked {
				chunkedChan <- StreamResponse{Message: ai.ChatCompletionStreamChoice{Delta: ai.ChatCompletionStreamChoiceDelta{Role: ai.ChatMessageRoleAssistant, Content: string(chunk)}}}
			} else {
				break
			}
		}

		if choice.FinishReason == ai.FinishReasonToolCalls {
			log.Println("completionStream: tool call assembled", c.partialToolCall)
			toolChan <- c.partialToolCall
			c.assemblingTool = false
		}
	}
}

func (c *Chunker) assembleToolCall(toolCall ai.ToolCall) (bool, ai.ToolCall) {
	if !c.assemblingTool {
		log.Println("completionStream: assembling tool call", toolCall)
		c.partialToolCall = toolCall
		c.assemblingTool = true
	} else {
		c.partialToolCall.Function.Arguments += toolCall.Function.Arguments
	}
	return c.assemblingTool, c.partialToolCall
}

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

func (c *Chunker) chunkByMaxLength() ([]byte, bool) {
	if c.Buffer.Len() >= c.Length {
		return c.readChunk(c.Length), true
	}
	return nil, false
}

func (c *Chunker) chunkBySentenceBoundary() ([]byte, bool) {
	index := c.sentenceBoundary(c.Buffer.Bytes())
	if index != -1 {
		return c.readChunk(index + 1), true
	}
	return nil, false
}

func (c *Chunker) readChunk(n int) []byte {
	chunk := c.Buffer.Next(n)
	c.Last = time.Now()
	return chunk
}

func (c *Chunker) sentenceBoundary(s []byte) int {
	sentences := c.Tokenizer.Tokenize(string(s))
	if len(sentences) > 1 {
		return len([]byte(sentences[0].Text))
	}
	return -1
}
