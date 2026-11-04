package config_test

import (
	"errors"
	"testing"
	"time"

	"pkdindustries/soulshack/internal/config"
)

func TestSnapshotDoesNotWaitForUpdateWork(t *testing.T) {
	store := config.NewStore(testConfig())
	started, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		defer close(finished)
		if err := store.Update(func(c *config.Configuration) error {
			c.Bot.Prompt = "new prompt"
			close(started)
			<-release
			return nil
		}); err != nil {
			t.Error(err)
		}
	}()
	<-started
	read := make(chan *config.Configuration, 1)
	go func() { read <- store.Snapshot() }()
	select {
	case snapshot := <-read:
		if snapshot.Bot.Prompt != "be nice" {
			t.Errorf("reader saw uncommitted prompt %q", snapshot.Bot.Prompt)
		}
	case <-time.After(time.Second):
		t.Error("snapshot blocked on update work")
	}
	close(release)
	<-finished
	if got := store.Snapshot().Bot.Prompt; got != "new prompt" {
		t.Fatalf("committed prompt=%q", got)
	}
}

func TestFailedUpdateDoesNotPublishChanges(t *testing.T) {
	store := config.NewStore(testConfig())
	failure := errors.New("rejected")
	if err := store.Update(func(c *config.Configuration) error {
		c.Bot.Admins[0] = "different!*@*"
		c.Bot.Prompt = "uncommitted"
		return failure
	}); !errors.Is(err, failure) {
		t.Fatalf("error=%v", err)
	}
	got := store.Snapshot()
	if got.Bot.Prompt != "be nice" || got.Bot.Admins[0] != "alice!*@*" {
		t.Fatalf("failed update leaked: %+v", got.Bot)
	}
}

func TestStoreDoesNotRetainCallerOwnedSettings(t *testing.T) {
	initial := testConfig()
	store := config.NewStore(initial)
	initial.Bot.Prompt = "changed after construction"
	if got := store.Snapshot().Bot.Prompt; got != "be nice" {
		t.Fatalf("initial config alias leaked: %q", got)
	}
	var candidate *config.Configuration
	if err := store.Update(func(c *config.Configuration) error { candidate = c; return nil }); err != nil {
		t.Fatal(err)
	}
	candidate.Bot.Prompt = "changed after update"
	if got := store.Snapshot().Bot.Prompt; got != "be nice" {
		t.Fatalf("update alias leaked: %q", got)
	}
}
