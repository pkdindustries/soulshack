package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// HistoryFilter defines options for filtering history
type HistoryFilter struct {
	Limit     int
	Search    string
	StartTime time.Time
	EndTime   time.Time
	CountOnly bool
}

// HistoryStore defines the interface for storing and retrieving chat history
type HistoryStore interface {
	Add(channel, nick, message string) error
	Get(channel string, filter HistoryFilter) ([]string, int, error)
}

// FileHistory implements HistoryStore using file-based storage
type FileHistory struct {
	baseDir string
	mu      sync.Mutex
}

// NewFileHistory creates a new FileHistory instance
func NewFileHistory(baseDir string) (*FileHistory, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create history directory: %w", err)
	}
	return &FileHistory{
		baseDir: baseDir,
	}, nil
}

func (h *FileHistory) getFilePath(channel string) string {
	// Sanitize channel name for filename (replace / with _)
	safeName := strings.ReplaceAll(channel, "/", "_")
	return filepath.Join(h.baseDir, safeName+".log")
}

// Add appends a message to the channel's history file
func (h *FileHistory) Add(channel, nick, message string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	filename := h.getFilePath(channel)
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	timestamp := time.Now().Format(time.RFC3339)
	// Format: timestamp|nick|message
	line := fmt.Sprintf("%s|%s|%s\n", timestamp, nick, message)

	if _, err := f.WriteString(line); err != nil {
		return err
	}
	return nil
}

// Get retrieves messages from the channel's history based on the filter
func (h *FileHistory) Get(channel string, filter HistoryFilter) ([]string, int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	filename := h.getFilePath(channel)
	f, err := os.Open(filename)
	if os.IsNotExist(err) {
		return []string{}, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	// Read all lines
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, 0, err
	}

	var result []string
	count := 0

	// Process from end to start to get most recent
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		parts := strings.SplitN(line, "|", 3)
		if len(parts) == 3 {
			timestampStr := parts[0]
			nick := parts[1]
			msg := parts[2]
			formattedMsg := fmt.Sprintf("<%s> %s", nick, msg)

			// Parse timestamp
			timestamp, err := time.Parse(time.RFC3339, timestampStr)
			if err != nil {
				continue // Skip malformed timestamps
			}

			// Apply time filters
			if !filter.StartTime.IsZero() && timestamp.Before(filter.StartTime) {
				continue
			}
			if !filter.EndTime.IsZero() && timestamp.After(filter.EndTime) {
				continue
			}

			// Apply search filter
			if filter.Search != "" {
				if !strings.Contains(strings.ToLower(formattedMsg), strings.ToLower(filter.Search)) {
					continue
				}
			}

			count++

			// If we only need the count, we don't need to store the message
			if !filter.CountOnly {
				// Prepend since we are iterating backwards
				result = append([]string{formattedMsg}, result...)
			}
		}

		// If we have a limit and we've reached it, stop
		// Note: For counting, we might want to count ALL matching messages, but usually "how many messages in the last 50" implies a limit.
		// However, "how many messages on Wednesday" implies NO limit on the count, but a time range.
		// If Limit is 0, we assume no limit.
		if filter.Limit > 0 && len(result) >= filter.Limit && !filter.CountOnly {
			break
		}
	}

	return result, count, nil
}
