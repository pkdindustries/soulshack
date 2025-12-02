package testing

import (
	"github.com/lrstanley/girc"
)

// NewMockIRCClient creates a minimal girc.Client for testing
// Note: This client is not connected and cannot send real messages,
// but it provides the structure needed for tests that call GetClient()
func NewMockIRCClient() *girc.Client {
	return girc.New(girc.Config{
		Server: "mock.server",
		Port:   6667,
		Nick:   "testbot",
		User:   "testbot",
		Name:   "Test Bot",
	})
}
