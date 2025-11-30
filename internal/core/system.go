package core

import (
	"pkdindustries/soulshack/internal/config"

	"github.com/alexschlessinger/pollytool/sessions"
	"github.com/alexschlessinger/pollytool/tools"
)

type System interface {
	GetToolRegistry() *tools.ToolRegistry
	GetSessionStore() sessions.SessionStore
	GetLLM() LLM
	UpdateLLM(config.APIConfig) error
}
