package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/alexschlessinger/pollytool/messages"
	"github.com/lrstanley/girc"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	//log.SetOutput(io.Discard)
	initializeConfig()
	m.Run()
}

var irc = girc.New(girc.Config{
	Server: "irc.example.com",
	Port:   6667,
	Nick:   "chatbot",
	User:   "test",
	Name:   "Test Bot",
	SSL:    false,
})

var event = &girc.Event{
	Source: &girc.Source{
		Name: "test",
	},
	Params: []string{"#test", "test2"},
}

func TestChatSession(t *testing.T) {

	Config := NewConfiguration()
	Config.Session.MaxHistory = 10
	Config.Session.TTL = 1 * time.Hour
	sys := NewSystem(Config)
	ctx, _ := NewChatContext(context.Background(), Config, sys, irc, event)
	//log.SetOutput(io.Discard)

	store := ctx.GetSystem().GetSessionStore()
	t.Run("Test interactions and message history", func(t *testing.T) {
		session1 := store.Get("session1")
		session1.AddMessage(messages.ChatMessage{Role: messages.MessageRoleUser, Content: "Hello!"})
		session1.AddMessage(messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "Hi there!"})

		assert.Len(t, session1.GetHistory(), 3)
		assert.Equal(t, session1.GetHistory()[1].Content, "Hello!")
		assert.Equal(t, session1.GetHistory()[2].Content, "Hi there!")
	})
}
func TestExpiry(t *testing.T) {
	t.Log("Starting TestExpiry")
	Config := NewConfiguration()
	Config.Session.MaxHistory = 3
	Config.Session.TTL = 500 * time.Millisecond
	sys := NewSystem(Config)
	ctx, _ := NewChatContext(context.Background(), Config, sys, irc, event)
	//log.SetOutput(io.Discard)

	store := ctx.GetSystem().GetSessionStore()

	t.Run("Test session expiration and trimming", func(t *testing.T) {

		session2 := store.Get("session2")
		session2.AddMessage(messages.ChatMessage{Role: messages.MessageRoleUser, Content: "How are you?"})
		session2.AddMessage(messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "I'm doing great, thanks!"})
		session2.AddMessage(messages.ChatMessage{Role: messages.MessageRoleUser, Content: "What's your name?"})

		time.Sleep(1 * time.Second)
		session3 := store.Get("session2")

		assert.NotEqual(t, session2, session3, "Expired session should not be reused")
		assert.Len(t, session3.GetHistory(), 1, "New session history should have one system message")
		assert.Equal(t, session3.GetHistory()[0].Role, messages.MessageRoleSystem, "First message should be a system message")

		session3.AddMessage(messages.ChatMessage{Role: messages.MessageRoleUser, Content: "Hello again! I scroll off"})
		session3.AddMessage(messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "Hi! Nice to see you again!"})

		assert.Len(t, session3.GetHistory(), 3, "History should include the latest 2 messages plus the initial system message")
		assert.Equal(t, session3.GetHistory()[1].Content, "Hello again! I scroll off")
		assert.Equal(t, session3.GetHistory()[2].Content, "Hi! Nice to see you again!")
		assert.Equal(t, session3.GetHistory()[0].Role, messages.MessageRoleSystem, "First message should be a system message")

		session3.AddMessage(messages.ChatMessage{Role: messages.MessageRoleUser, Content: "Hello again?"})
		session3.AddMessage(messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: "WHAT?!"})

		assert.Equal(t, session3.GetHistory()[2].Content, "Hello again?")
		assert.Equal(t, session3.GetHistory()[3].Content, "WHAT?!")
	})
}

func TestSessionConcurrency(t *testing.T) {

	log.SetOutput(io.Discard)
	Config := NewConfiguration()
	Config.Session.MaxHistory = 500 * 2000
	Config.Session.TTL = 1 * time.Hour
	sys := NewSystem(Config)
	ctx, _ := NewChatContext(context.Background(), Config, sys, irc, event)
	store := ctx.GetSystem().GetSessionStore()
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
				session := store.Get(sessionID)

				for j := 0; j < messagesPerUser; j++ {
					session.AddMessage(messages.ChatMessage{Role: messages.MessageRoleUser, Content: fmt.Sprintf("User %d message %d", userIndex, j)})
					session.AddMessage(messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: fmt.Sprintf("Assistant response to user %d message %d", userIndex, j)})
				}
			}(i)
		}

		wg.Wait()

		for i := 0; i < concurrentUsers; i++ {
			sessionID := fmt.Sprintf("usersession%d", i)
			session := store.Get(sessionID)
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
	Config := NewConfiguration()
	Config.Session.MaxHistory = 500 * 2000
	Config.Session.TTL = 1 * time.Hour
	ctx, _ := NewChatContext(context.Background(), Config, NewSystem(Config), irc, event)
	store := ctx.GetSystem().GetSessionStore()

	t.Run("Test single session concurrency", func(t *testing.T) {

		const concurrentUsers = 500
		const messagesPerUser = 100

		startTime := time.Now()

		session := store.Get("concurrentSession")

		var wg sync.WaitGroup
		wg.Add(concurrentUsers)

		for i := 0; i < concurrentUsers; i++ {
			go func(userIndex int) {
				defer wg.Done()
				for j := 0; j < messagesPerUser; j++ {
					session.AddMessage(messages.ChatMessage{Role: messages.MessageRoleUser, Content: fmt.Sprintf("User %d message %d", userIndex, j)})
					session.AddMessage(messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: fmt.Sprintf("Assistant response to user %d message %d", userIndex, j)})
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

	Config := NewConfiguration()
	Config.Session.TTL = timeout
	Config.Session.MaxHistory = 10
	Config.Session.ChunkMax = 5
	ctx, _ := NewChatContext(context.Background(), Config, NewSystem(Config), irc, event)

	store := ctx.GetSystem().GetSessionStore()

	// Create and store sessions
	for i := 0; i < numSessions; i++ {
		sessionID := fmt.Sprintf("session-%d", i)
		store.Get(sessionID)
	}

	// Verify that all sessions are created
	sessionCount := -1
	store.Range(func(key, value interface{}) bool {
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
		session := store.Get(sessionID)
		session.AddMessage(messages.ChatMessage{Role: messages.MessageRoleUser, Content: fmt.Sprintf("message-%d", 0)})
		session.AddMessage(messages.ChatMessage{Role: messages.MessageRoleUser, Content: fmt.Sprintf("message-%d", 1)})
	}

	// wait for the unfreshened half to time out
	time.Sleep(55 * time.Millisecond)
	activeSessions := 0
	store.Range(func(key, value interface{}) bool {
		session := value.(*LocalSession)
		if time.Since(session.last) <= Config.Session.TTL {
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
		history    []messages.ChatMessage
		maxHistory int
		expected   []messages.ChatMessage
	}{
		{
			name: "Simple_case",
			history: []messages.ChatMessage{
				{Role: messages.MessageRoleUser, Content: "Prompt"},
				{Role: messages.MessageRoleUser, Content: "Message 1"},
				{Role: messages.MessageRoleUser, Content: "Message 2"},
				{Role: messages.MessageRoleUser, Content: "Message 3"},
				{Role: messages.MessageRoleUser, Content: "Message 4"},
			},
			maxHistory: 2,
			expected: []messages.ChatMessage{
				{Role: messages.MessageRoleUser, Content: "Prompt"},
				{Role: messages.MessageRoleUser, Content: "Message 3"},
				{Role: messages.MessageRoleUser, Content: "Message 4"},
			},
		},
		// Add more test cases if needed
	}

	Config := NewConfiguration()

	ctx, _ := NewChatContext(context.Background(), Config, NewSystem(Config), irc, event)
	store := ctx.GetSystem().GetSessionStore()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			session := store.Get("test").(*LocalSession)
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
		msgs := make([]messages.ChatMessage, msgCount)
		for i := 0; i < msgCount; i++ {
			msgs[i] = messages.ChatMessage{Role: messages.MessageRoleUser, Content: fmt.Sprintf("Message %d", i)}
		}

		Config := NewConfiguration()
		ctx, _ := NewChatContext(context.Background(), Config, NewSystem(Config), irc, event)
		store := ctx.GetSystem().GetSessionStore()

		b.Run(fmt.Sprintf("MsgCount_%d", msgCount), func(b *testing.B) {

			Config.Session.MaxHistory = msgCount / 2
			msg := store.Get("test").(*LocalSession)
			msg.history = msgs

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				msg.trimHistory()
			}
		})
	}
}

func BenchmarkSessionStress(b *testing.B) {

	Config := NewConfiguration()
	Config.Session.TTL = 1 * time.Second
	Config.Session.MaxHistory = 5

	ctx, _ := NewChatContext(context.Background(), Config, NewSystem(Config), irc, event)
	store := ctx.GetSystem().GetSessionStore()
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
							session := store.Get(sessionID)

							action := rand.Intn(3)

							switch action {
							case 0: // Add user message
								for j := 0; j < messagesPerUser; j++ {
									session.AddMessage(messages.ChatMessage{Role: messages.MessageRoleUser, Content: fmt.Sprintf("User %d message %d", userIndex, j)})
								}
							case 1: // Add assistant message
								for j := 0; j < messagesPerUser; j++ {
									session.AddMessage(messages.ChatMessage{Role: messages.MessageRoleAssistant, Content: fmt.Sprintf("Assistant response to user %d message %d", userIndex, j)})
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
