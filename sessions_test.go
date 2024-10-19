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

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	initializeConfig()
	m.Run()
}

func TestChatSession(t *testing.T) {

	Config := NewConfig()
	Config.Session.MaxHistory = 10
	Config.Session.SessionDuration = 1 * time.Hour

	Config.Store = NewSessionStore(Config)
	//log.SetOutput(io.Discard)

	t.Run("Test interactions and message history", func(t *testing.T) {
		session1 := Config.Store.Get("session1")
		session1.AddMessage(ai.ChatCompletionMessage{Role: ai.ChatMessageRoleUser, Content: "Hello!"})
		session1.AddMessage(ai.ChatCompletionMessage{Role: ai.ChatMessageRoleAssistant, Content: "Hi there!"})

		assert.Len(t, session1.GetHistory(), 3)
		assert.Equal(t, session1.GetHistory()[1].Content, "Hello!")
		assert.Equal(t, session1.GetHistory()[2].Content, "Hi there!")
	})
}
func TestExpiry(t *testing.T) {
	t.Log("Starting TestExpiry")
	Config := NewConfig()
	Config.Session.MaxHistory = 20
	Config.Session.SessionDuration = 500 * time.Millisecond

	Config.Store = NewSessionStore(Config)
	t.Run("Test session expiration and trimming", func(t *testing.T) {

		session2 := Config.Store.Get("session2")
		session2.AddMessage(ai.ChatCompletionMessage{Role: ai.ChatMessageRoleUser, Content: "How are you?"})
		session2.AddMessage(ai.ChatCompletionMessage{Role: ai.ChatMessageRoleAssistant, Content: "I'm doing great, thanks!"})
		session2.AddMessage(ai.ChatCompletionMessage{Role: ai.ChatMessageRoleUser, Content: "What's your name?"})

		time.Sleep(2 * time.Second)
		session3 := Config.Store.Get("session2")

		assert.NotEqual(t, session2, session3, "Expired session should not be reused")
		assert.Len(t, session3.GetHistory(), 0, "New session history should be empty")

		session3.AddMessage(ai.ChatCompletionMessage{Role: ai.ChatMessageRoleUser, Content: "Hello again!"})
		session3.AddMessage(ai.ChatCompletionMessage{Role: ai.ChatMessageRoleAssistant, Content: "Hi! Nice to see you again!"})

		assert.Len(t, session3.GetHistory(), 3, "History should include the latest 2 messages plus the initial system message")
		assert.Equal(t, session3.GetHistory()[1].Content, "Hello again!")
		assert.Equal(t, session3.GetHistory()[2].Content, "Hi! Nice to see you again!")
	})
}

func TestSessionConcurrency(t *testing.T) {
	vip.Set("session", 1*time.Hour)
	vip.Set("history", 10)

	log.SetOutput(io.Discard)
	Config := NewConfig()
	Config.Session.MaxHistory = 500 * 2000
	Config.Session.SessionDuration = 1 * time.Hour
	Config.Store = NewSessionStore(Config)
	t.Run("Test session concurrency", func(t *testing.T) {

		const concurrentUsers = 1000
		const messagesPerUser = 500

		startTime := time.Now()

		var wg sync.WaitGroup
		wg.Add(concurrentUsers)

		for i := 0; i < concurrentUsers; i++ {
			go func(userIndex int) {
				defer wg.Done()
				sessionID := fmt.Sprintf("usersession%d", userIndex)
				session := Config.Store.Get(sessionID)

				for j := 0; j < messagesPerUser; j++ {
					session.AddMessage(ai.ChatCompletionMessage{Role: ai.ChatMessageRoleUser, Content: fmt.Sprintf("User %d message %d", userIndex, j)})
					session.AddMessage(ai.ChatCompletionMessage{Role: ai.ChatMessageRoleAssistant, Content: fmt.Sprintf("Assistant response to user %d message %d", userIndex, j)})
				}
			}(i)
		}

		wg.Wait()

		for i := 0; i < concurrentUsers; i++ {
			sessionID := fmt.Sprintf("usersession%d", i)
			session := Config.Store.Get(sessionID)
			assert.Len(t, session.GetHistory(), messagesPerUser*2+1, "Each session should have the correct number of messages")
		}
		elapsedTime := time.Since(startTime)
		totalMessages := concurrentUsers * messagesPerUser * 2
		messagesPerSecond := float64(totalMessages) / elapsedTime.Seconds()
		t.Logf("Processed %d messages in %v, which is %.2f messages per second\n", totalMessages, elapsedTime, messagesPerSecond)
	})
}

func TestSingleSessionConcurrency(t *testing.T) {
	log.SetOutput(io.Discard)
	Config := NewConfig()
	Config.Session.MaxHistory = 500 * 2000
	Config.Session.SessionDuration = 1 * time.Hour
	Config.Store = NewSessionStore(Config)
	t.Run("Test single session concurrency", func(t *testing.T) {

		const concurrentUsers = 500
		const messagesPerUser = 100

		startTime := time.Now()

		session := Config.Store.Get("concurrentSession")

		var wg sync.WaitGroup
		wg.Add(concurrentUsers)

		for i := 0; i < concurrentUsers; i++ {
			go func(userIndex int) {
				defer wg.Done()
				for j := 0; j < messagesPerUser; j++ {
					session.AddMessage(ai.ChatCompletionMessage{Role: ai.ChatMessageRoleUser, Content: fmt.Sprintf("User %d message %d", userIndex, j)})
					session.AddMessage(ai.ChatCompletionMessage{Role: ai.ChatMessageRoleAssistant, Content: fmt.Sprintf("Assistant response to user %d message %d", userIndex, j)})
				}
			}(i)
		}

		wg.Wait()

		elapsedTime := time.Since(startTime)
		totalMessages := concurrentUsers * messagesPerUser * 2
		messagesPerSecond := float64(totalMessages) / elapsedTime.Seconds()

		assert.Len(t, session.GetHistory(), totalMessages+1, "The session should have the correct number of messages")
		t.Logf("Processed %d messages in %v, which is %.2f messages per second\n", totalMessages, elapsedTime, messagesPerSecond)
	})
}

func TestSessionReapStress(t *testing.T) {
	// Set up test configurations
	numSessions := 2000
	timeout := 100 * time.Millisecond
	log.SetOutput(io.Discard)

	Config := NewConfig()
	Config.Session.SessionDuration = timeout
	Config.Session.MaxHistory = 10
	Config.Session.ChunkDelay = 200 * time.Millisecond
	Config.Session.ChunkMax = 5

	Config.Store = NewSessionStore(Config)

	// Create and store sessions
	for i := 0; i < numSessions; i++ {
		sessionID := fmt.Sprintf("session-%d", i)
		Config.Store.Get(sessionID)
	}

	// Verify that all sessions are created
	sessionCount := 0
	Config.Store.Range(func(key, value interface{}) bool {
		sessionCount++
		return true
	})
	if sessionCount != numSessions {
		t.Fatalf("Expected %d sessions, got %d", numSessions, sessionCount)
	}

	time.Sleep(50 * time.Millisecond)
	// half are half aged
	for i := 0; i < numSessions/2; i++ {
		sessionID := fmt.Sprintf("session-%d", i)
		session := Config.Store.Get(sessionID)
		session.AddMessage(ai.ChatCompletionMessage{Role: ai.ChatMessageRoleUser, Content: fmt.Sprintf("message-%d", 0)})
		session.AddMessage(ai.ChatCompletionMessage{Role: ai.ChatMessageRoleUser, Content: fmt.Sprintf("message-%d", 1)})
	}

	// wait for the unfreshened half to time out
	time.Sleep(55 * time.Millisecond)
	activeSessions := 0
	Config.Store.Range(func(key, value interface{}) bool {
		session := value.(*LocalSession)
		if time.Since(session.last) <= Config.Session.SessionDuration {
			activeSessions++
		}
		return true
	})

	expectedActiveSessions := numSessions / 2
	if activeSessions != expectedActiveSessions {
		t.Fatalf("Expected %d active sessions, got %d", expectedActiveSessions, activeSessions)
	}

	t.Logf("Reaped %d sessions", numSessions-expectedActiveSessions)
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

	Config := NewConfig()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			session := Config.Store.Get("test").(*LocalSession)
			// cast to SyncMapSessionStore
			session.history = tc.history
			Config.Session.MaxHistory = tc.maxHistory
			session.trimHistory()

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

		Config := NewConfig()
		b.Run(fmt.Sprintf("MsgCount_%d", msgCount), func(b *testing.B) {

			Config.Session.MaxHistory = msgCount / 2
			msg := Config.Store.Get("test").(*LocalSession)
			msg.history = messages

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				msg.trimHistory()
			}
		})
	}
}

func BenchmarkSessionStress(b *testing.B) {

	Config := NewConfig()
	Config.Session.SessionDuration = 1 * time.Second
	Config.Session.MaxHistory = 5
	Config.Store = NewSessionStore(Config)
	log.SetOutput(io.Discard)

	concurrentUsers := []int{10, 100, 1000}
	for _, concurrentUsers := range concurrentUsers {

		b.Run(fmt.Sprintf("SessionStress_%d", concurrentUsers), func(b *testing.B) {

			for i := 0; i < b.N; i++ {

				const sessionsPerUser = 2
				const messagesPerUser = 50

				var wg sync.WaitGroup
				wg.Add(concurrentUsers)

				for i := 0; i < concurrentUsers; i++ {
					go func(userIndex int) {
						defer wg.Done()

						for k := 0; k < sessionsPerUser; k++ {
							sessionID := fmt.Sprintf("session%d-%d", userIndex, k)
							session := Config.Store.Get(sessionID)

							action := rand.Intn(3)

							switch action {
							case 0: // Add user message
								for j := 0; j < messagesPerUser; j++ {
									session.AddMessage(ai.ChatCompletionMessage{Role: ai.ChatMessageRoleUser, Content: fmt.Sprintf("User %d message %d", userIndex, j)})
								}
							case 1: // Add assistant message
								for j := 0; j < messagesPerUser; j++ {
									session.AddMessage(ai.ChatCompletionMessage{Role: ai.ChatMessageRoleAssistant, Content: fmt.Sprintf("Assistant response to user %d message %d", userIndex, j)})
								}
							case 2:
							}
						}
						log.Println("Session stress test: user", userIndex, sessionsPerUser*messagesPerUser)
					}(i)
				}

				wg.Wait()
			}
		})
	}
}
