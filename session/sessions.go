package session

import (
	"fmt"
	"log"
	"pkdindustries/soulshack/action"
	"sync"
	"time"

	ai "github.com/sashabaranov/go-openai"
	vip "github.com/spf13/viper"
)

var SessionStore = SessionMap{
	sessionMap: make(map[string]*Session),
	mu:         sync.RWMutex{},
}

type Personality struct {
	Prompt string
	Model  string
	Temp   float64
}

type SessionConfig struct {
	Chunkdelay    time.Duration
	Chunkmax      int
	Chunkquoted   bool
	ClientTimeout time.Duration
	MaxHistory    int
	MaxTokens     int
	ReactMode     bool
	TTL           time.Duration
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

func (s *Session) AddMessage(conf *Personality, role string, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.history) == 0 {
		content := conf.Prompt
		s.history = append(s.history, ai.ChatCompletionMessage{
			Role:    ai.ChatMessageRoleSystem,
			Content: content,
		})
		s.Totalchars += len(content)
		if s.Config.ReactMode {
			reactprompt := action.ReactPrompt(&action.ReactConfig{Actions: action.ActionRegistry})
			s.history = append(s.history, ai.ChatCompletionMessage{
				Role:    ai.ChatMessageRoleSystem,
				Content: reactprompt,
			})
			s.Totalchars += len(reactprompt)
		}
	}

	s.history = append(s.history, ai.ChatCompletionMessage{Role: role, Content: message})
	s.Totalchars += len(message)
	s.Last = time.Now()

	// if vip.GetBool("verbose") {
	// 	s.Debug()
	// }

	s.trim()
}

// contining the no alloc tradition to mock python users
func (s *Session) trim() {
	if len(s.history) > s.Config.MaxHistory {
		rm := len(s.history) - s.Config.MaxHistory
		for i := 1; i <= s.Config.MaxHistory; i++ {
			s.history[i] = s.history[i+rm-1]
		}
		s.history = s.history[:s.Config.MaxHistory+1]
	}
}

func (s *Session) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
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
		Chunkdelay:    vip.GetDuration("chunkdelay"),
		Chunkmax:      vip.GetInt("chunkmax"),
		ClientTimeout: vip.GetDuration("timeout"),
		MaxHistory:    vip.GetInt("history"),
		MaxTokens:     vip.GetInt("maxtokens"),
		ReactMode:     vip.GetBool("react"),
		TTL:           vip.GetDuration("session"),
	}
}

// show string of all msg contents
func (s *Session) Debug() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, msg := range s.history {
		ds := ""
		if msg.Role == ai.ChatMessageRoleAssistant {
			ds += "< "
		} else {
			ds += "> "
		}
		ds += fmt.Sprintf("%s:%s", msg.Role, msg.Content)
		log.Println(ds)
	}
}
