package modules

import (
	"errors"
	"testing"

	log "github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
)

func TestLogJoinOperationErrorDowngradesExpectedTelegramConditions(t *testing.T) {
	cases := []string{
		"unable to sendMessage: Bad Request: TOPIC_CLOSED",
		"join welcome failed: failed to send text to chat -1001 at errors/errors.go:60 in errors.Wrapf: unable to sendMessage: Bad Request: TOPIC_CLOSED",
		"bot lacks permission to send messages",
		"join welcome failed: bot lacks permission to send messages",
		"unable to sendPhoto: Forbidden: bot was kicked from the group chat",
	}
	for _, msg := range cases {
		hook := logrustest.NewGlobal()
		logJoinOperationError("Failed to process new member join", errors.New(msg))
		entry := hook.LastEntry()
		hook.Reset()
		if entry == nil {
			t.Fatalf("no log entry for %q", msg)
		}
		if entry.Level != log.WarnLevel {
			t.Errorf("logJoinOperationError(%q) logged at %s, want warning", msg, entry.Level)
		}
	}
}

func TestLogJoinOperationErrorKeepsUnexpectedErrorsAtErrorLevel(t *testing.T) {
	hook := logrustest.NewGlobal()
	logJoinOperationError("Failed to process new member join", errors.New("connection reset by peer"))
	entry := hook.LastEntry()
	hook.Reset()
	if entry == nil {
		t.Fatal("no log entry")
	}
	if entry.Level != log.ErrorLevel {
		t.Errorf("unexpected error logged at %s, want error", entry.Level)
	}
}

func TestLogJoinOperationErrorIgnoresNil(t *testing.T) {
	hook := logrustest.NewGlobal()
	logJoinOperationError("Failed to process new member join", nil)
	if n := len(hook.Entries); n != 0 {
		t.Errorf("nil error logged %d entries, want 0", n)
	}
	hook.Reset()
}
