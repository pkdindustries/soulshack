package main

import (
	"bytes"
	"log"
	"time"

	"github.com/neurosnap/sentences"
	"github.com/neurosnap/sentences/english"
)

type Chunker struct {
	buffer *bytes.Buffer
	delay  time.Duration
	max    int
	quote  bool
	last   time.Time
}

func (c *Chunker) ChannelFilter(input <-chan *string) <-chan string {
	out := make(chan string, 10)
	go func() {
		defer close(out)
		for val := range input {
			c.buffer.WriteString(*val)
			chunker(c, out)
		}
	}()
	return out
}

// chunker reads chunks from the Chunker and sends them to the output channel
func chunker(c *Chunker, out chan<- string) {
	for {
		if chunked, chunk := c.chunk(); chunked {
			out <- string(chunk)
		} else {
			break
		}
	}
}

func (c *Chunker) chunk() (bool, []byte) {
	// if chunkdelay is -1, huck the buffer right now
	if c.delay == -1 && c.buffer.Len() > 0 {
		chunk := c.buffer.Next(c.buffer.Len())
		c.last = time.Now()
		return true, chunk
	}

	// if chunkquoted is true, chunk a whole block quote
	if c.quote {
		content := c.buffer.Bytes()
		// block quotes
		blockstart := bytes.Index(content, []byte("```"))
		if blockstart != -1 {
			blockend := bytes.Index(content[blockstart+3:], []byte("```"))
			if blockend != -1 {
				chunk := c.buffer.Next(blockstart + 3 + blockend + 3)
				c.last = time.Now()
				return true, chunk
			}
			// not found, don't chunk
			return false, nil
		}
	}
	// chunk on a newline in first chunksize
	end := c.max
	if c.buffer.Len() < end {
		end = c.buffer.Len()
	}

	// chunk on a newline in first chunksize
	index := bytes.IndexByte(c.buffer.Bytes()[:end], '\n')
	if index != -1 {
		chunk := c.buffer.Next(index + 1)
		c.last = time.Now()
		return true, chunk
	}

	// chunk if full buffer satisfies chunk size
	if c.buffer.Len() >= c.max {
		chunk := c.buffer.Next(c.max)
		c.last = time.Now()
		return true, chunk
	}

	// chunk on boundary if n seconds have passed since the last chunk
	if time.Since(c.last) >= c.delay {
		content := c.buffer.Bytes()
		index := dumbBoundary(&content)
		if index != -1 {
			chunk := c.buffer.Next(index + 1)
			c.last = time.Now()
			return true, chunk
		}
	}

	// no chunk
	return false, nil
}

// :(
func dumberBoundary(s *[]byte) int {
	for i := len(*s) - 2; i >= 0; i-- {
		if ((*s)[i] == '.' || (*s)[i] == ':' || (*s)[i] == '!' || (*s)[i] == '?') && ((*s)[i+1] == ' ' || (*s)[i+1] == '\t') {
			return i + 1
		}
	}
	return -1
}

// painfully slow on startup etc but 'correcter..ish' for english
var tokenizer *sentences.DefaultSentenceTokenizer

func init() {
	t, err := english.NewSentenceTokenizer(nil)
	if err != nil {
		log.Fatal("Error creating tokenizer:", err)
	}
	tokenizer = t
}
func dumbBoundary(s *[]byte) int {
	sentences := tokenizer.Tokenize(string(*s))
	if len(sentences) > 1 {
		return len([]byte(sentences[0].Text))
	}
	return -1
}
