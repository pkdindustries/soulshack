package main

import (
	"bytes"
	"log"
	"time"

	"github.com/neurosnap/sentences"
	"github.com/neurosnap/sentences/english"
)

type Chunker struct {
	Buffer     *bytes.Buffer
	Chunkdelay time.Duration
	Chunkmax   int
	Last       time.Time
}

func (c *Chunker) Filter(input <-chan *string) <-chan string {
	out := make(chan string)

	go func() {
		defer close(out)
		for val := range input {
			c.Buffer.WriteString(*val)
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

	end := c.Chunkmax
	if c.Buffer.Len() < end {
		end = c.Buffer.Len()
	}

	// chunk on a newline in first chunksize
	index := bytes.IndexByte(c.Buffer.Bytes()[:end], '\n')
	if index != -1 {
		chunk := c.Buffer.Next(index + 1)
		c.Last = time.Now()
		return true, chunk
	}

	// chunk if full buffer satisfies chunk size
	if c.Buffer.Len() >= c.Chunkmax {
		chunk := c.Buffer.Next(c.Chunkmax)
		c.Last = time.Now()
		return true, chunk
	}

	// chunk on boundary if n seconds have passed since the last chunk
	if time.Since(c.Last) >= c.Chunkdelay {
		content := c.Buffer.Bytes()
		index := dumbBoundary(&content)
		if index != -1 {
			chunk := c.Buffer.Next(index + 1)
			c.Last = time.Now()
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
