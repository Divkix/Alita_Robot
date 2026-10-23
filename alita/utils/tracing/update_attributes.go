package tracing

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/divkix/Alita_Robot/alita/utils/callbackcodec"
	"github.com/divkix/Alita_Robot/alita/utils/logredact"
)

// maxCommandLen bounds the command attribute: the first word is user text.
const maxCommandLen = 32

// annotateUpdate tags the update's span with what the update was and where it
// came from, so usage and failures can be grouped per command and chat.
func annotateUpdate(span trace.Span, ctx *ext.Context) {
	if ctx == nil || !span.IsRecording() {
		return
	}
	attrs := make([]attribute.KeyValue, 0, 6)
	attrs = append(attrs, attribute.String("update.type", updateType(ctx.Update)))
	if chat := ctx.EffectiveChat; chat != nil {
		attrs = append(attrs, attribute.Int64("chat.id", chat.Id), attribute.String("chat.type", chat.Type))
	}
	if user := ctx.EffectiveUser; user != nil {
		attrs = append(attrs, attribute.Int64("user.id", user.Id))
	}
	if msg := ctx.EffectiveMessage; msg != nil {
		if cmd := commandName(msg); cmd != "" {
			attrs = append(attrs, attribute.String("bot.command", cmd))
		}
	}
	if ctx.Update != nil && ctx.CallbackQuery != nil {
		if decoded, err := callbackcodec.Decode(ctx.CallbackQuery.Data); err == nil {
			attrs = append(attrs, attribute.String("callback.namespace", decoded.Namespace))
		}
	}
	span.SetAttributes(attrs...)
}

func updateType(u *gotgbot.Update) string {
	if u == nil {
		return "unknown"
	}
	return u.GetType()
}

// commandName returns "/cmd" for a message Telegram marks as starting with a
// bot command, lowercased and without the "@botname" suffix, or "".
func commandName(msg *gotgbot.Message) string {
	if len(msg.Entities) == 0 || msg.Entities[0].Type != "bot_command" || msg.Entities[0].Offset != 0 {
		return ""
	}
	text := msg.Text
	if i := strings.IndexFunc(text, unicode.IsSpace); i >= 0 {
		text = text[:i]
	}
	cmd, _, _ := strings.Cut(text, "@")
	if len(cmd) < 2 || len(cmd) > maxCommandLen || cmd[0] != '/' {
		return ""
	}
	return strings.ToLower(cmd)
}

// RecordUpdateError marks the update's span as failed. Handler errors end in
// the dispatcher's error handler and never reach the processor, so it is
// called from there. Expected Telegram errors are tagged but leave the span
// successful.
func RecordUpdateError(ctx *ext.Context, err error, expected bool) {
	span := trace.SpanFromContext(UpdateContext(ctx))
	if err == nil || !span.IsRecording() {
		return
	}
	message := logredact.Scrub(err.Error())
	span.SetAttributes(
		attribute.String("error.type", fmt.Sprintf("%T", err)),
		attribute.String("error.message", message),
		attribute.Bool("error.expected", expected),
	)
	if !expected {
		span.SetStatus(codes.Error, message)
	}
}
