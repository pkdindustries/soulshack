package main

import (
	"fmt"
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

	ctx := &ChatContext{
		Personality: &Personality{
			Prompt: "You are a helpful assistant.",
		},
	}

	t.Run("Test interactions and message history", func(t *testing.T) {
		session1 := sessions.Get("session1")
		session1.Message(ctx, ai.ChatMessageRoleUser, "Hello!")
		session1.Message(ctx, ai.ChatMessageRoleAssistant, "Hi there!")

		assert.Len(t, session1.History, 3)
		assert.Equal(t, session1.History[1].Content, "Hello!")
		assert.Equal(t, session1.History[2].Content, "Hi there!")
	})

	t.Run("Test session expiration and trimming", func(t *testing.T) {
		vip.Set("session", 1*time.Millisecond)
		vip.Set("history", 2)

		session2 := sessions.Get("session2")
		session2.Message(ctx, ai.ChatMessageRoleUser, "How are you?")
		session2.Message(ctx, ai.ChatMessageRoleAssistant, "I'm doing great, thanks!")
		session2.Message(ctx, ai.ChatMessageRoleUser, "What's your name?")

		time.Sleep(2 * time.Millisecond)
		session3 := sessions.Get("session2")

		assert.NotEqual(t, session2, session3, "Expired session should not be reused")
		assert.Len(t, session3.History, 0, "New session history should be empty")

		session3.Message(ctx, ai.ChatMessageRoleUser, "Hello again!")
		session3.Message(ctx, ai.ChatMessageRoleAssistant, "Hi! Nice to see you again!")

		assert.Len(t, session3.History, 3, "History should include the latest 2 messages plus the initial system message")
		assert.Equal(t, session3.History[1].Content, "Hello again!")
		assert.Equal(t, session3.History[2].Content, "Hi! Nice to see you again!")
	})

	t.Run("Test session concurrency", func(t *testing.T) {
		vip.Set("session", 1*time.Hour)
		vip.Set("history", 1000)

		ctx := &ChatContext{
			Personality: &Personality{
				Prompt: "You are a helpful assistant.",
			},
		}

		const concurrentUsers = 100
		const messagesPerUser = 50

		startTime := time.Now()

		var wg sync.WaitGroup
		wg.Add(concurrentUsers)

		for i := 0; i < concurrentUsers; i++ {
			go func(userIndex int) {
				defer wg.Done()
				sessionID := fmt.Sprintf("usersession%d", userIndex)
				session := sessions.Get(sessionID)

				for j := 0; j < messagesPerUser; j++ {
					session.Message(ctx, ai.ChatMessageRoleUser, fmt.Sprintf("User %d message %d", userIndex, j))
					session.Message(ctx, ai.ChatMessageRoleAssistant, fmt.Sprintf("Assistant response to user %d message %d", userIndex, j))
				}
			}(i)
		}

		wg.Wait()

		for i := 0; i < concurrentUsers; i++ {
			sessionID := fmt.Sprintf("usersession%d", i)
			session := sessions.Get(sessionID)
			assert.Len(t, session.History, messagesPerUser*2+1, "Each session should have the correct number of messages")
		}
		elapsedTime := time.Since(startTime)
		totalMessages := concurrentUsers * messagesPerUser * 2
		messagesPerSecond := float64(totalMessages) / elapsedTime.Seconds()
		t.Logf("Processed %d messages in %v, which is %.2f messages per second\n", totalMessages, elapsedTime, messagesPerSecond)
	})

	t.Run("Test single session concurrency", func(t *testing.T) {
		vip.Set("session", 1*time.Hour)
		vip.Set("history", 2000)

		ctx := &ChatContext{
			Personality: &Personality{
				Prompt: "You are a helpful assistant.",
			},
		}

		const concurrentUsers = 50
		const messagesPerUser = 10

		startTime := time.Now()

		session := sessions.Get("concurrentSession")

		var wg sync.WaitGroup
		wg.Add(concurrentUsers)

		for i := 0; i < concurrentUsers; i++ {
			go func(userIndex int) {
				defer wg.Done()

				for j := 0; j < messagesPerUser; j++ {
					session.Message(ctx, ai.ChatMessageRoleUser, fmt.Sprintf("User %d message %d", userIndex, j))
					session.Message(ctx, ai.ChatMessageRoleAssistant, fmt.Sprintf("Assistant response to user %d message %d", userIndex, j))
				}
			}(i)
		}

		wg.Wait()

		elapsedTime := time.Since(startTime)
		totalMessages := concurrentUsers * messagesPerUser * 2
		messagesPerSecond := float64(totalMessages) / elapsedTime.Seconds()

		assert.Len(t, session.History, totalMessages+1, "The session should have the correct number of messages")
		t.Logf("Processed %d messages in %v, which is %.2f messages per second\n", totalMessages, elapsedTime, messagesPerSecond)
	})

	t.Run("Test session get", func(t *testing.T) {
		vip.Set("session", 1*time.Hour)
		vip.Set("history", 1000)

		const concurrentGets = 1000
		const sessionID = "sharedSession"

		var wg sync.WaitGroup
		wg.Add(concurrentGets)

		sessionResults := make([]*ChatSession, concurrentGets)

		for i := 0; i < concurrentGets; i++ {
			go func(index int) {
				defer wg.Done()
				sessionResults[index] = sessions.Get(sessionID)
			}(i)
		}

		wg.Wait()

		// Check that all goroutines got the same session instance
		for i := 1; i < concurrentGets; i++ {
			assert.Equal(t, sessionResults[0], sessionResults[i], "All session instances should be the same")
		}
	})
}
