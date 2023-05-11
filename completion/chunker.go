package completion

import (
	"bytes"
	"log"
	"time"

	"github.com/neurosnap/sentences"
	"github.com/neurosnap/sentences/english"
)

type Chunker struct {
	Buffer *bytes.Buffer
	Delay  time.Duration
	Max    int
	Quote  bool
	Last   time.Time
}

func (c *Chunker) Filter(input <-chan *string) <-chan string {
	out := make(chan string, 10)
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

	if c.Buffer.Len() == 0 {
		return false, nil
	}

	// if chunkdelay is -1, huck the buffer right now
	if c.Delay == -1 && c.Buffer.Len() > 0 {
		chunk := c.Buffer.Next(c.Buffer.Len())
		c.Last = time.Now()
		return true, chunk
	}

	// if chunkquoted is true, chunk a whole block quote
	if c.Quote {
		content := c.Buffer.Bytes()
		// block quotes
		blockstart := bytes.Index(content, []byte("```"))
		if blockstart != -1 {
			blockend := bytes.Index(content[blockstart+3:], []byte("```"))
			if blockend != -1 {
				chunk := c.Buffer.Next(blockstart + 3 + blockend + 3)
				c.Last = time.Now()
				return true, chunk
			}
			// not found, don't chunk
			return false, nil
		}
	}
	// chunk on a newline in first chunksize
	end := c.Max
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
	if c.Buffer.Len() >= c.Max {
		chunk := c.Buffer.Next(c.Max)
		c.Last = time.Now()
		return true, chunk
	}

	// chunk on boundary if n seconds have passed since the last chunk
	if time.Since(c.Last) >= c.Delay {
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
