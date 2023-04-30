package main

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"sync"
	"testing"
	"time"

	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestChatSession(t *testing.T) {
	vip.Set("session", 1*time.Hour)
	vip.Set("history", 10)

	//log.SetOutput(io.Discard)

	ctx := &IrcContext{
		personality: &Personality{
			Prompt: "You are a helpful assistant.",
		},
	}

	t.Run("Test interactions and message history", func(t *testing.T) {
		session1 := sessions.Get("session1")
		session1.AddMessage(ctx.personality, ai.ChatMessageRoleUser, "Hello!")
		session1.AddMessage(ctx.personality, ai.ChatMessageRoleAssistant, "Hi there!")

		assert.Len(t, session1.history, 3)
		assert.Equal(t, session1.history[1].Content, "Hello!")
		assert.Equal(t, session1.history[2].Content, "Hi there!")
	})
}
func TestExpiry(t *testing.T) {
	//log.SetOutput(io.Discard)
	ctx := &IrcContext{
		personality: &Personality{
			Prompt: "You are a helpful assistant.",
		},
	}
	t.Run("Test session expiration and trimming", func(t *testing.T) {
		vip.Set("session", 500*time.Millisecond)
		vip.Set("history", 20)

		session2 := sessions.Get("session2")
		session2.AddMessage(ctx.personality, ai.ChatMessageRoleUser, "How are you?")
		session2.AddMessage(ctx.personality, ai.ChatMessageRoleAssistant, "I'm doing great, thanks!")
		session2.AddMessage(ctx.personality, ai.ChatMessageRoleUser, "What's your name?")

		time.Sleep(2 * time.Second)
		session3 := sessions.Get("session2")

		assert.NotEqual(t, session2, session3, "Expired session should not be reused")
		assert.Len(t, session3.history, 0, "New session history should be empty")

		session3.AddMessage(ctx.personality, ai.ChatMessageRoleUser, "Hello again!")
		session3.AddMessage(ctx.personality, ai.ChatMessageRoleAssistant, "Hi! Nice to see you again!")

		assert.Len(t, session3.history, 3, "History should include the latest 2 messages plus the initial system message")
		assert.Equal(t, session3.history[1].Content, "Hello again!")
		assert.Equal(t, session3.history[2].Content, "Hi! Nice to see you again!")
	})
}

func TestSessionConcurrency(t *testing.T) {
	vip.Set("session", 1*time.Hour)
	vip.Set("history", 10)
	sessions.sessionMap = make(map[string]*Sessions)
	log.SetOutput(io.Discard)

	t.Run("Test session concurrency", func(t *testing.T) {
		vip.Set("session", 1*time.Hour)
		vip.Set("history", 500*2000)

		ctx := &IrcContext{
			personality: &Personality{
				Prompt: "You are a helpful assistant.",
			},
		}

		const concurrentUsers = 1000
		const messagesPerUser = 500

		startTime := time.Now()

		var wg sync.WaitGroup
		wg.Add(concurrentUsers)

		for i := 0; i < concurrentUsers; i++ {
			go func(userIndex int) {
				defer wg.Done()
				sessionID := fmt.Sprintf("usersession%d", userIndex)
				session := sessions.Get(sessionID)

				for j := 0; j < messagesPerUser; j++ {
					session.AddMessage(ctx.personality, ai.ChatMessageRoleUser, fmt.Sprintf("User %d message %d", userIndex, j))
					session.AddMessage(ctx.personality, ai.ChatMessageRoleAssistant, fmt.Sprintf("Assistant response to user %d message %d", userIndex, j))
				}
			}(i)
		}

		wg.Wait()

		for i := 0; i < concurrentUsers; i++ {
			sessionID := fmt.Sprintf("usersession%d", i)
			session := sessions.Get(sessionID)
			assert.Len(t, session.history, messagesPerUser*2+1, "Each session should have the correct number of messages")
		}
		elapsedTime := time.Since(startTime)
		totalMessages := concurrentUsers * messagesPerUser * 2
		messagesPerSecond := float64(totalMessages) / float64(elapsedTime.Milliseconds())
		t.Logf("Processed %d messages in %v, which is %.2f messages per ms??\n", totalMessages, elapsedTime, messagesPerSecond)
	})
}

func TestSingleSessionConcurrency(t *testing.T) {
	log.SetOutput(io.Discard)

	t.Run("Test single session concurrency", func(t *testing.T) {
		vip.Set("session", 1*time.Hour)
		vip.Set("history", 500*200)

		ctx := &IrcContext{
			personality: &Personality{
				Prompt: "You are a helpful assistant.",
			},
		}

		const concurrentUsers = 500
		const messagesPerUser = 100

		startTime := time.Now()

		session := sessions.Get("concurrentSession")

		var wg sync.WaitGroup
		wg.Add(concurrentUsers)

		for i := 0; i < concurrentUsers; i++ {
			go func(userIndex int) {
				defer wg.Done()
				for j := 0; j < messagesPerUser; j++ {
					session.AddMessage(ctx.personality, ai.ChatMessageRoleUser, fmt.Sprintf("User %d message %d", userIndex, j))
					session.AddMessage(ctx.personality, ai.ChatMessageRoleAssistant, fmt.Sprintf("Assistant response to user %d message %d", userIndex, j))
				}
			}(i)
		}

		wg.Wait()

		elapsedTime := time.Since(startTime)
		totalMessages := concurrentUsers * messagesPerUser * 2
		messagesPerSecond := float64(totalMessages) / float64(elapsedTime.Milliseconds())

		assert.Len(t, session.history, totalMessages+1, "The session should have the correct number of messages")
		t.Logf("Processed %d messages in %v, which is %.2f messages per ms??\n", totalMessages, elapsedTime, messagesPerSecond)
	})
}

func TestSessionReapStress(t *testing.T) {
	// Set up test configurations
	numSessions := 2000
	timeout := 100 * time.Millisecond
	log.SetOutput(io.Discard)
	sessions.sessionMap = make(map[string]*Sessions)
	vip.Set("session", timeout)
	vip.Set("history", 10)
	vip.Set("chunkdelay", 200*time.Millisecond)
	vip.Set("chunkmax", 5)

	// Create and store sessions
	for i := 0; i < numSessions; i++ {
		sessionID := fmt.Sprintf("session-%d", i)
		sessions.Get(sessionID)
	}

	// Verify that all sessions are created
	if len(sessions.sessionMap) != numSessions {
		t.Fatalf("Expected %d sessions, got %d", numSessions, len(sessions.sessionMap))
	}

	// Simulate activity for some sessions
	testPersonality := Personality{
		Prompt: "Test prompt",
	}

	time.Sleep(50 * time.Millisecond)
	// half are half aged
	for i := 0; i < numSessions/2; i++ {
		sessionID := fmt.Sprintf("session-%d", i)
		session := sessions.Get(sessionID)
		session.AddMessage(&testPersonality, ai.ChatMessageRoleUser, fmt.Sprintf("message-%d", 0))
		session.AddMessage(&testPersonality, ai.ChatMessageRoleUser, fmt.Sprintf("message-%d", 1))
	}

	// wait for the unfreshened half to time out
	time.Sleep(55 * time.Millisecond)
	activeSessions := len(sessions.sessionMap)
	expectedActiveSessions := numSessions / 2
	if activeSessions != expectedActiveSessions {
		t.Fatalf("Expected %d active sessions, got %d", expectedActiveSessions, activeSessions)
	}

}

func TestSessionWindow(t *testing.T) {
	testCases := []struct {
		name       string
		history    []ai.ChatCompletionMessage
		maxHistory int
		expected   []ai.ChatCompletionMessage
	}{
		{
			name: "Simple_case",
			history: []ai.ChatCompletionMessage{
				{Role: ai.ChatMessageRoleUser, Content: "Prompt"},
				{Role: ai.ChatMessageRoleUser, Content: "Message 1"},
				{Role: ai.ChatMessageRoleUser, Content: "Message 2"},
				{Role: ai.ChatMessageRoleUser, Content: "Message 3"},
				{Role: ai.ChatMessageRoleUser, Content: "Message 4"},
			},
			maxHistory: 2,
			expected: []ai.ChatCompletionMessage{
				{Role: ai.ChatMessageRoleUser, Content: "Prompt"},
				{Role: ai.ChatMessageRoleUser, Content: "Message 3"},
				{Role: ai.ChatMessageRoleUser, Content: "Message 4"},
			},
		},
		// Add more test cases if needed
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			session := Sessions{
				history: tc.history,
				Config:  &SessionConfig{MaxHistory: tc.maxHistory},
			}

			session.trim()

			if len(session.history) != len(tc.expected) {
				t.Errorf("Expected history length to be %d, but got %d", len(tc.expected), len(session.history))
			}

			for i, msg := range session.history {
				if msg.Role != tc.expected[i].Role || msg.Content != tc.expected[i].Content {
					t.Errorf("Expected message at index %d to be %+v, but got %+v", i, tc.expected[i], msg)
				}
			}
		})
	}
}
func BenchmarkTrim(b *testing.B) {
	testCases := []int{100, 1000, 10000, 20000}
	for _, msgCount := range testCases {
		messages := make([]ai.ChatCompletionMessage, msgCount)
		for i := 0; i < msgCount; i++ {
			messages[i] = ai.ChatCompletionMessage{Role: ai.ChatMessageRoleUser, Content: fmt.Sprintf("Message %d", i)}
		}
		b.Run(fmt.Sprintf("MsgCount_%d", msgCount), func(b *testing.B) {
			session := Sessions{
				history: messages,
				Config:  &SessionConfig{MaxHistory: msgCount / 2},
			}

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				session.trim()
			}
		})
	}
}

func BenchmarkSessionStress(b *testing.B) {
	vip.Set("session", 1*time.Second) // Short session duration to trigger more expirations
	vip.Set("history", 5)             // Shorter history length to trigger more trimming

	ctx := &IrcContext{
		personality: &Personality{
			Prompt: "You are a helpful assistant.",
		},
	}
	log.SetOutput(io.Discard)

	concurrentUsers := []int{10, 100, 1000}
	for _, concurrentUsers := range concurrentUsers {

		b.Run(fmt.Sprintf("SessionStress_%d", concurrentUsers), func(b *testing.B) {

			for i := 0; i < b.N; i++ {

				const sessionsPerUser = 50
				const messagesPerUser = 50

				var wg sync.WaitGroup
				wg.Add(concurrentUsers)

				for i := 0; i < concurrentUsers; i++ {
					go func(userIndex int) {
						defer wg.Done()

						for k := 0; k < sessionsPerUser; k++ {
							sessionID := fmt.Sprintf("session%d-%d", userIndex, k)
							session := sessions.Get(sessionID)

							action := rand.Intn(4)

							switch action {
							case 0: // Add user message
								for j := 0; j < messagesPerUser; j++ {
									session.AddMessage(ctx.personality, ai.ChatMessageRoleUser, fmt.Sprintf("User %d message %d", userIndex, j))
								}
							case 1: // Add assistant message
								for j := 0; j < messagesPerUser; j++ {
									session.AddMessage(ctx.personality, ai.ChatMessageRoleAssistant, fmt.Sprintf("Assistant response to user %d message %d", userIndex, j))
								}
							case 2: // Reset the session
								session.Reset()
							case 3: // Expire the session
								session.Reap()
							}
						}
					}(i)
				}

				wg.Wait()
			}
		})
	}
}
