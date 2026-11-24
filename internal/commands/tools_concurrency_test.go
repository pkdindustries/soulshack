package commands

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/alexschlessinger/pollytool/tools"
	mocktest "pkdindustries/soulshack/internal/testing"
)

func TestToolsCommand_LiveLoadDoesNotBlockConfigReaders(t *testing.T) {
	sys := mocktest.NewMockSystem(t)
	registry := sys.ToolRegistry
	defer registry.Close()
	var calls atomic.Int32
	loading, resume := make(chan struct{}), make(chan struct{})
	registry.RegisterNative("review__lookup", func() tools.Tool {
		if calls.Add(1) == 2 {
			close(loading)
			<-resume
		}
		return &tools.Func{Name: "review__lookup", Desc: "Look up a value."}
	})
	chat := mocktest.NewTurnContext(t, sys, "review").WithAdmin(true).
		WithArgs("/tools", "add", "review__lookup")
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		(&ToolsCommand{}).Execute(chat.Turn())
	}()
	<-loading
	readStarted, readFinished := make(chan struct{}), make(chan struct{})
	go func() {
		close(readStarted)
		sys.GetConfig()
		close(readFinished)
	}()
	<-readStarted
	select {
	case <-readFinished:
	case <-time.After(250 * time.Millisecond):
		t.Error("unrelated config reader blocked behind live external tool load")
	}
	close(resume)
	<-finished
	<-readFinished
}
