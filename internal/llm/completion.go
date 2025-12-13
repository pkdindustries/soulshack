package llm

import (
	"sync"

	"github.com/alexschlessinger/pollytool/llm"
	"github.com/alexschlessinger/pollytool/messages"
	"github.com/alexschlessinger/pollytool/sessions"
	"github.com/alexschlessinger/pollytool/tools"

	"pkdindustries/soulshack/internal/config"
	"pkdindustries/soulshack/internal/irc"
)

type CompletionRequest = llm.CompletionRequest

// Track warned sessions to avoid repeated warnings
var (
	warnedSessions = make(map[string]int) // session_name -> last_warning_percentage
	warningMutex   sync.RWMutex
)

// checkSessionCapacity checks if the session is approaching token limits and sends warnings
func checkSessionCapacity(ctx irc.ChatContextInterface) {
	session := ctx.GetSession()

	// Use polly's capacity calculation
	percentage := session.GetCapacityPercentage()
	if percentage == 0 {
		return // No limit set
	}

	// Get session identifier and check last warning level
	sessionName := session.GetName()

	warningMutex.Lock()
	defer warningMutex.Unlock()

	lastWarning := warnedSessions[sessionName]

	// Send warnings at thresholds, avoiding repeats
	if percentage >= 90 && lastWarning < 90 {
		ctx.ReplyAction("Session at 90% capacity - conversation history will be trimmed soon")
		warnedSessions[sessionName] = 90
	} else if percentage >= 75 && lastWarning < 75 {
		ctx.ReplyAction("Session at 75% capacity")
		warnedSessions[sessionName] = 75
	} else if percentage < 75 && lastWarning > 0 {
		// Reset warning state if capacity drops below thresholds
		delete(warnedSessions, sessionName)
	}
}

func NewCompletionRequest(config *config.Configuration, session sessions.Session, tools []tools.Tool) *CompletionRequest {
	// Parse thinking effort - validated at config load time
	thinkingEffort, _ := llm.ParseThinkingEffort(config.Model.ThinkingEffort)

	req := &CompletionRequest{
		BaseURL:        config.API.OpenAIURL,
		Timeout:        config.API.Timeout,
		Model:          config.Model.Model,
		MaxTokens:      config.Model.MaxTokens,
		Messages:       session.GetHistory(),
		Temperature:    config.Model.Temperature,
		Tools:          tools,
		ThinkingEffort: thinkingEffort,
	}

	// Set streaming mode (nil = streaming default, false = non-streaming)
	if !config.Model.Stream {
		stream := false
		req.Stream = &stream
	}

	return req
}

// Complete processes a user message and returns a channel of response chunks.
func Complete(ctx irc.ChatContextInterface, msg string) (<-chan string, error) {
	// Check session capacity and warn if approaching limits
	checkSessionCapacity(ctx)

	// Add user message to session
	cmsg := messages.ChatMessage{
		Role:    messages.MessageRoleUser,
		Content: msg,
	}
	truncated := msg
	if len(truncated) > 100 {
		truncated = truncated[:100] + "..."
	}
	ctx.GetLogger().Infow("message_received", "message", truncated)
	ctx.GetSession().AddMessage(cmsg)

	// Build completion request
	session := ctx.GetSession()
	cfg := ctx.GetConfig()
	sys := ctx.GetSystem()

	var allTools []tools.Tool
	if sys.GetToolRegistry() != nil {
		allTools = sys.GetToolRegistry().All()
	}

	req := NewCompletionRequest(cfg, session, allTools)

	// Get response stream from LLM
	stream := sys.GetLLM().ChatCompletionStream(ctx, req)

	output := make(chan string, 10)

	go func() {
		defer close(output)
		for chunk := range stream {
			output <- chunk
		}
	}()

	return output, nil
}
