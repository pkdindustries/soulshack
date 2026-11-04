package config_test

import (
	"fmt"
	"sync"
	"testing"

	"pkdindustries/soulshack/internal/config"
)

func testConfig() *config.Configuration {
	return &config.Configuration{
		Server:  &config.ServerConfig{Nick: "bot", Channel: "#test"},
		Bot:     &config.BotConfig{Prompt: "be nice", Admins: []string{"alice!*@*"}, Tools: []string{"bash"}},
		Model:   &config.ModelConfig{Model: "gpt-4", MaxTokens: 100},
		Session: &config.SessionConfig{MaxContext: 1000},
		API:     &config.APIConfig{Timeout: 0},
	}
}

func TestSnapshotIsIndependent(t *testing.T) {
	store := config.NewStore(testConfig())

	snapshot := store.Snapshot()
	snapshot.Model.MaxTokens = 5
	snapshot.Bot.Admins[0] = "mallory!*@*"
	snapshot.Bot.Prompt = "be mean"

	next := store.Snapshot()
	if next.Model.MaxTokens != 100 || next.Bot.Prompt != "be nice" || next.Bot.Admins[0] != "alice!*@*" {
		t.Fatalf("snapshot leaked into the store: %+v", next)
	}
}

func TestUpdateChangesWhatTheNextSnapshotSees(t *testing.T) {
	store := config.NewStore(testConfig())

	if err := store.Update(func(c *config.Configuration) error {
		c.Session.MaxContext = 4096
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got := store.Snapshot().Session.MaxContext; got != 4096 {
		t.Fatalf("maxcontext = %d", got)
	}
}

// A reader holding a snapshot keeps seeing the settings it read, which is what
// lets a turn read config freely while /set runs elsewhere.
func TestSnapshotSurvivesConcurrentUpdates(t *testing.T) {
	store := config.NewStore(testConfig())

	var wg sync.WaitGroup
	for writer := 0; writer < 4; writer++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				if err := store.Update(func(c *config.Configuration) error {
					c.Model.MaxTokens = n
					c.Bot.Admins = append(c.Bot.Admins, fmt.Sprintf("admin%d!*@*", n))
					return nil
				}); err != nil {
					t.Error(err)
					return
				}
			}
		}(writer)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				snapshot := store.Snapshot()
				_ = snapshot.Model.MaxTokens
				_ = snapshot.Bot.Admins
				_ = snapshot.Bot.Prompt
				_ = snapshot.Session.TTL
			}
		}()
	}
	wg.Wait()
}
