// Package slogrus sends log/slog records to logrus, so libraries that log
// through slog (gotgbot's dispatcher and updater) get the bot's formatting,
// secret redaction and OTLP export.
package slogrus

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"path"
	"runtime"

	log "github.com/sirupsen/logrus"
)

// Handler is a slog.Handler that writes each record to logrus's standard
// logger, with the record's attributes as fields.
type Handler struct {
	fields log.Fields // from WithAttrs, already flattened
	prefix string     // group names joined with "."
}

var _ slog.Handler = Handler{}

func (Handler) Enabled(_ context.Context, level slog.Level) bool {
	return log.IsLevelEnabled(logrusLevel(level))
}

func (h Handler) Handle(ctx context.Context, r slog.Record) error {
	fields := make(log.Fields, len(h.fields)+r.NumAttrs()+1)
	maps.Copy(fields, h.fields)
	r.Attrs(func(a slog.Attr) bool {
		addField(fields, h.prefix, a)
		return true
	})
	if log.StandardLogger().ReportCaller && r.PC != 0 {
		// logrus would report this file as the caller.
		frame, _ := runtime.CallersFrames([]uintptr{r.PC}).Next()
		fields["source"] = fmt.Sprintf("%s:%d", path.Base(frame.File), frame.Line)
	}
	log.WithContext(ctx).WithTime(r.Time).WithFields(fields).Log(logrusLevel(r.Level), r.Message)
	return nil
}

func (h Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	fields := maps.Clone(h.fields)
	if fields == nil {
		fields = make(log.Fields, len(attrs))
	}
	for _, a := range attrs {
		addField(fields, h.prefix, a)
	}
	h.fields = fields
	return h
}

func (h Handler) WithGroup(name string) slog.Handler {
	if name != "" {
		h.prefix += name + "."
	}
	return h
}

func addField(fields log.Fields, prefix string, a slog.Attr) {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}
	if a.Value.Kind() == slog.KindGroup {
		if a.Key != "" {
			prefix += a.Key + "."
		}
		for _, ga := range a.Value.Group() {
			addField(fields, prefix, ga)
		}
		return
	}
	fields[prefix+a.Key] = a.Value.Any()
}

func logrusLevel(level slog.Level) log.Level {
	switch {
	case level >= slog.LevelError:
		return log.ErrorLevel
	case level >= slog.LevelWarn:
		return log.WarnLevel
	case level >= slog.LevelInfo:
		return log.InfoLevel
	case level >= slog.LevelDebug:
		return log.DebugLevel
	default:
		return log.TraceLevel
	}
}
