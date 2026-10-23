package slogrus

import (
	"errors"
	"log/slog"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
)

func TestHandlerWritesRecordsToLogrus(t *testing.T) {
	hook := test.NewGlobal()
	t.Cleanup(func() { log.StandardLogger().ReplaceHooks(make(log.LevelHooks)) })
	failure := errors.New("bad gateway")

	logger := slog.New(Handler{}).With("component", "updater").WithGroup("req").With(slog.Attr{}, "id", 7)
	logger.Error("failed to get updates", "error", failure, slog.Group("retry", "after", 1), slog.Group("", "inline", true))

	entry := hook.LastEntry()
	if entry == nil {
		t.Fatal("no logrus entry written")
	}
	if entry.Level != log.ErrorLevel || entry.Message != "failed to get updates" {
		t.Fatalf("entry = %v %q, want error %q", entry.Level, entry.Message, "failed to get updates")
	}
	want := log.Fields{
		"component": "updater", "req.id": int64(7), "req.error": failure,
		"req.retry.after": int64(1), "req.inline": true,
	}
	if len(entry.Data) != len(want) {
		t.Fatalf("fields = %v, want %v", entry.Data, want)
	}
	for key, value := range want {
		if entry.Data[key] != value {
			t.Fatalf("field %s = %v, want %v (fields %v)", key, entry.Data[key], value, entry.Data)
		}
	}
}

func TestHandlerHonorsLogrusLevel(t *testing.T) {
	old := log.GetLevel()
	log.SetLevel(log.InfoLevel)
	t.Cleanup(func() { log.SetLevel(old) })

	logger := slog.New(Handler{})
	if logger.Enabled(t.Context(), slog.LevelDebug) {
		t.Fatal("debug enabled while logrus is at info")
	}
	if !logger.Enabled(t.Context(), slog.LevelWarn) {
		t.Fatal("warn disabled while logrus is at info")
	}
}
