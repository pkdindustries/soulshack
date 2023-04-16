package main

import (
	"bytes"
	"log"
	"time"

	"github.com/neurosnap/sentences"
	"github.com/neurosnap/sentences/english"
)

type Chunker struct {
	Size       int
	Last       time.Time
	Buffer     *bytes.Buffer
	Chunkdelay time.Duration
}

func (c *Chunker) ChunkFilter(input <-chan *string) <-chan string {
	out := make(chan string)
	go func() {
		defer close(out)
		for val := range input {
			c.Buffer.WriteString(*val)
			for {
				if chunked, chunk := c.Chunk(); chunked {
					out <- string(*chunk)
				} else {
					break
				}
			}
		}

	}()
	return out
}

func (c *Chunker) Chunk() (bool, *[]byte) {

	end := c.Size
	if c.Buffer.Len() < end {
		end = c.Buffer.Len()
	}

	// chunk on a newline in first chunksize
	index := bytes.IndexByte(c.Buffer.Bytes()[:end], '\n')
	if index != -1 {
		chunk := c.Buffer.Next(index + 1)
		c.Last = time.Now()
		return true, &chunk
	}

	// chunk if full buffer satisfies chunk size
	if c.Buffer.Len() >= c.Size {
		chunk := c.Buffer.Next(c.Size)
		c.Last = time.Now()
		return true, &chunk
	}

	// chunk on boundary if n seconds have passed since the last chunk
	if time.Since(c.Last) >= c.Chunkdelay {
		content := c.Buffer.Bytes()
		index := dumbBoundary(&content)
		if index != -1 {
			chunk := c.Buffer.Next(index + 1)
			c.Last = time.Now()
			return true, &chunk
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
		log.Panicln("Error creating tokenizer:", err)
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
