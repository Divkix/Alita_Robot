package tracing

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/divkix/Alita_Robot/alita/utils/callbackcodec"
	"github.com/divkix/Alita_Robot/alita/utils/logredact"
	"github.com/divkix/Alita_Robot/alita/utils/metrics"
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
		attrs = append(attrs, semconv.UserID(strconv.FormatInt(user.Id, 10)))
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

// updateCommand returns the command a new message starts with, without its
// "/", or "". A callback's EffectiveMessage is the bot's own message, so only
// Update.Message counts.
func updateCommand(ctx *ext.Context) string {
	if ctx == nil || ctx.Update == nil || ctx.Message == nil {
		return ""
	}
	return strings.TrimPrefix(commandName(ctx.Message), "/")
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

// outcomeDataKey holds the worst metrics.Outcome a handler gave the update,
// for the processor to label it with.
const outcomeDataKey = "alita.updateOutcome"

// markOutcome raises the update's outcome to o, never lowering it.
func markOutcome(ctx *ext.Context, o metrics.Outcome) {
	if ctx == nil || ctx.Data == nil {
		return
	}
	if current, _ := ctx.Data[outcomeDataKey].(metrics.Outcome); o > current {
		ctx.Data[outcomeDataKey] = o
	}
}

// MarkUpdatePanic labels the update's metrics as a panic. Call it where a
// handler's panic is recovered before it reaches the processor.
func MarkUpdatePanic(ctx *ext.Context) {
	markOutcome(ctx, metrics.OutcomePanic)
}

// RecordUpdateError counts a handler error and marks the update's span as
// failed. Handler errors never reach the processor, so it is called where
// they are logged. Expected Telegram errors are tagged but leave the span
// successful.
func RecordUpdateError(ctx *ext.Context, err error, expected bool) {
	if err == nil {
		return
	}
	outcome := metrics.OutcomeError
	if expected {
		outcome = metrics.OutcomeExpectedError
	}
	markOutcome(ctx, outcome)
	metrics.HandlerErrors.WithLabelValues(metrics.CommandModule(updateCommand(ctx)), strconv.FormatBool(expected)).Inc()

	span := trace.SpanFromContext(UpdateContext(ctx))
	if !span.IsRecording() {
		return
	}
	span.SetAttributes(attribute.Bool("error.expected", expected))
	if !expected {
		RecordError(span, err)
		return
	}
	span.SetAttributes(
		semconv.ErrorTypeKey.String(fmt.Sprintf("%T", err)),
		attribute.String("error.message", logredact.Scrub(err.Error())),
	)
}
