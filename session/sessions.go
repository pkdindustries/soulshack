package session

import (
	"fmt"
	"log"
	"pkdindustries/soulshack/action"
	"pkdindustries/soulshack/model"
	"sync"
	"time"

	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

var SessionStore = SessionMap{
	sessionMap: make(map[string]*Session),
	mu:         sync.RWMutex{},
}

const RoleSystem = "system"
const RoleUser = "user"
const RoleAssistant = "assistant"

type SessionConfig struct {
	model.Personality
	Chunkdelay    time.Duration
	Chunkmax      int
	Chunkquoted   bool
	ClientTimeout time.Duration
	MaxHistory    int
	MaxTokens     int
	ReactMode     bool
	TTL           time.Duration
	Verbose       bool
}

type SessionMap struct {
	sessionMap map[string]*Session
	mu         sync.RWMutex
}

type Session struct {
	Config     *SessionConfig
	history    []ai.ChatCompletionMessage
	Last       time.Time
	mu         sync.RWMutex
	Name       string
	Totalchars int
	Stash      map[string]string
}

func (s *Session) GetHistory() []ai.ChatCompletionMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	history := make([]ai.ChatCompletionMessage, len(s.history))
	copy(history, s.history)

	return history
}

func (s *Session) AddMessage(role string, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.initialMessages()
	s.addMessage(role, message)
	s.trimHistory()
	if s.Config.Verbose {
		s.Debug()
	}
}

func (s *Session) initialMessages() {
	if len(s.history) != 0 {
		return
	}

	s.addMessage(RoleSystem, s.Config.Prompt)
	s.Totalchars += len(s.Config.Prompt)

	if s.Config.ReactMode {
		reactprompt := action.ReactPrompt(&action.ReactConfig{Actions: action.ActionRegistry})
		s.addMessage(RoleSystem, reactprompt)
		s.Totalchars += len(reactprompt)
	}

}

func (s *Session) addMessage(role string, message string) {
	s.history = append(s.history, ai.ChatCompletionMessage{Role: role, Content: message})
	s.Totalchars += len(message)
	s.Last = time.Now()
}

func (s *Session) trimHistory() {
	if len(s.history) <= s.Config.MaxHistory {
		return
	}

	rm := len(s.history) - s.Config.MaxHistory
	for i := 1; i <= s.Config.MaxHistory; i++ {
		s.history[i] = s.history[i+rm-1]
	}
	s.history = s.history[:s.Config.MaxHistory+1]
}

func (s *Session) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Config = SessionFromViper(vip.GetViper())
	s.history = s.history[:0]
	s.Last = time.Now()
}

func (s *Session) Reap() bool {
	now := time.Now()
	SessionStore.mu.Lock()
	defer SessionStore.mu.Unlock()
	if SessionStore.sessionMap[s.Name] == nil {
		return true
	}
	if now.Sub(s.Last) > s.Config.TTL {
		delete(SessionStore.sessionMap, s.Name)
		return true
	}
	return false
}

func (sessions *SessionMap) Get(id string) *Session {
	sessions.mu.Lock()
	defer sessions.mu.Unlock()

	if v, ok := sessions.sessionMap[id]; ok {
		return v
	}

	session := &Session{
		Name:   id,
		Last:   time.Now(),
		Config: SessionFromViper(vip.GetViper()),
		Stash:  make(map[string]string),
	}

	// start session reaper, returns when the session is gone
	go func() {
		for {
			time.Sleep(session.Config.TTL)
			if session.Reap() {
				return
			}
		}
	}()

	sessions.sessionMap[id] = session
	return session
}

// sessionconfigfromviper
func SessionFromViper(v *vip.Viper) *SessionConfig {
	return &SessionConfig{
		Chunkdelay:    v.GetDuration("chunkdelay"),
		Chunkmax:      v.GetInt("chunkmax"),
		ClientTimeout: v.GetDuration("timeout"),
		MaxHistory:    v.GetInt("history"),
		MaxTokens:     v.GetInt("maxtokens"),
		ReactMode:     v.GetBool("reactmode"),
		TTL:           v.GetDuration("session"),
		Personality:   *model.PersonalityFromViper(v),
		Verbose:       v.GetBool("verbose"),
	}
}

// show string of all msg contents
func (s *Session) Debug() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, msg := range s.history {
		ds := ""
		ds += fmt.Sprintf("%s:%s", msg.Role, msg.Content)
		log.Println(ds)
	}
}
