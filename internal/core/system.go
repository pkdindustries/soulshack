package core

import (
	"github.com/alexschlessinger/pollytool/sessions"
	"github.com/alexschlessinger/pollytool/tools"
)

type System interface {
	GetToolRegistry() *tools.ToolRegistry
	GetSessionStore() sessions.SessionStore
	GetHistory() HistoryStore
}

type SystemImpl struct {
	Store   sessions.SessionStore
	Tools   *tools.ToolRegistry
	History HistoryStore
}

func (s *SystemImpl) GetToolRegistry() *tools.ToolRegistry {
	return s.Tools
}

func (s *SystemImpl) SetToolRegistry(reg *tools.ToolRegistry) {
	s.Tools = reg
}

func (s *SystemImpl) GetSessionStore() sessions.SessionStore {
	return s.Store
}

func (s *SystemImpl) GetHistory() HistoryStore {
	return s.History
}
