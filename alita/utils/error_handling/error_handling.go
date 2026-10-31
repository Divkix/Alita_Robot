package error_handling

import (
	"context"
	"errors"
	"net/http"
	"runtime/debug"
	"strings"
	"sync/atomic"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"

	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

var onErrorCallback atomic.Value

func SetOnErrorCallback(cb func()) {
	onErrorCallback.Store(cb)
}

func RecoverFromPanic(funcName, modName string) {
	if r := recover(); r != nil {
		handlePanic(context.Background(), funcName, modName, r)
	}
}

// RecoverFromPanicContext is RecoverFromPanic for code running under a span:
// it also records the panic, with its stack, on the span in ctx.
func RecoverFromPanicContext(ctx context.Context, funcName, modName string) {
	if r := recover(); r != nil {
		tracing.RecordPanic(trace.SpanFromContext(ctx), r)
		handlePanic(ctx, funcName, modName, r)
	}
}

// RecoverFromUpdatePanic is RecoverFromPanicContext for a handler: it also
// labels the update's metrics as a panic, which the processor cannot see once
// the panic is recovered here.
func RecoverFromUpdatePanic(ctx *ext.Context, funcName, modName string) {
	if r := recover(); r != nil {
		updateCtx := tracing.UpdateContext(ctx)
		tracing.RecordPanic(trace.SpanFromContext(updateCtx), r)
		tracing.MarkUpdatePanic(ctx)
		handlePanic(updateCtx, funcName, modName, r)
	}
}

func handlePanic(ctx context.Context, funcName, modName string, r any) {
	stackTrace := string(debug.Stack())

	log.WithContext(ctx).Errorf("[%s][%s] Recovered from panic: %v\nStack trace:\n%s",
		modName, funcName, r, stackTrace)

	if cb, ok := onErrorCallback.Load().(func()); ok && cb != nil {
		cb()
	}
}

// IsMissingChatMemberError recognizes only getChatMember's known HTTP 400
// absence replies. Missing chats, denied permissions and other failures remain
// errors; Telegram only guarantees other-user lookups when the bot is an admin.
// https://core.telegram.org/bots/api/methods/getchatmember.md
func IsMissingChatMemberError(err error) bool {
	var telegramErr *gotgbot.TelegramError
	if !errors.As(err, &telegramErr) || telegramErr.Method != "getChatMember" || telegramErr.Code != http.StatusBadRequest {
		return false
	}
	description := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(telegramErr.Description), "bad request:"))
	switch description {
	case "user not found", "member not found", "participant_id_invalid", "user_id_invalid":
		return true
	default:
		return false
	}
}

