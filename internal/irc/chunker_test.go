package irc

import (
	"testing"
)

func TestChunker_SingleLine(t *testing.T) {
	ch := make(chan string, 10)
	chunker := NewChunker(ch, 400)

	// Complete line (ends with \n) should be sent immediately
	chunker.Write("Hello world\n")

	select {
	case msg := <-ch:
		if msg != "Hello world" {
			t.Errorf("expected 'Hello world', got %q", msg)
		}
	default:
		t.Error("expected message to be sent immediately for complete line")
	}
}

func TestChunker_BufferOverflow(t *testing.T) {
	ch := make(chan string, 10)
	maxSize := 20
	chunker := NewChunker(ch, maxSize)

	// Write text that exceeds maxChunkSize (no newlines)
	chunker.Write("This is a message that exceeds the max size")

	// Should trigger a chunk
	select {
	case msg := <-ch:
		if len(msg) > maxSize {
			t.Errorf("chunk size %d exceeds max %d", len(msg), maxSize)
		}
	default:
		t.Error("expected buffer overflow to trigger a chunk")
	}
}

func TestChunker_SplitAtSpace(t *testing.T) {
	ch := make(chan string, 10)
	maxSize := 15
	chunker := NewChunker(ch, maxSize)

	// "Hello there friend" = 18 chars, should split at space
	chunker.Write("Hello there friend")

	select {
	case msg := <-ch:
		// Should split at a word boundary
		if msg != "Hello there" {
			t.Errorf("expected 'Hello there', got %q", msg)
		}
	default:
		t.Error("expected chunk to be emitted")
	}
}

func TestChunker_NoSpaceHardBreak(t *testing.T) {
	ch := make(chan string, 10)
	maxSize := 10
	chunker := NewChunker(ch, maxSize)

	// Long word without spaces should hard break
	chunker.Write("abcdefghijklmnopqrstuvwxyz")

	select {
	case msg := <-ch:
		if len(msg) != maxSize {
			t.Errorf("expected hard break at %d chars, got %d: %q", maxSize, len(msg), msg)
		}
		if msg != "abcdefghij" {
			t.Errorf("expected 'abcdefghij', got %q", msg)
		}
	default:
		t.Error("expected hard break chunk")
	}
}

func TestChunker_Flush(t *testing.T) {
	ch := make(chan string, 10)
	chunker := NewChunker(ch, 400)

	// Write partial content (no newline, under maxSize)
	chunker.Write("Partial content")

	// Nothing should be emitted yet
	select {
	case msg := <-ch:
		t.Errorf("unexpected message before flush: %q", msg)
	default:
		// Expected - no message yet
	}

	// Flush should emit the buffer
	chunker.Flush()

	select {
	case msg := <-ch:
		if msg != "Partial content" {
			t.Errorf("expected 'Partial content', got %q", msg)
		}
	default:
		t.Error("expected flush to emit remaining content")
	}
}
