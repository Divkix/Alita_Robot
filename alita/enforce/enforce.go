// Package enforce owns the tail every automated punishment shares: the
// exemption check, the bot-permission precheck, the apply, the automated
// action log and the localized group notice, in that order. A watcher emits
// a Spec of what should happen; Automated runs the sequence and reports the
// punishment's Result so callers that answer differently (a warn reply, a
// bail-out) can.
package enforce

import (
	"context"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/actionlog"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	keyboardutils "github.com/divkix/Alita_Robot/alita/utils/keyboard"
	"github.com/divkix/Alita_Robot/alita/utils/punishment"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

// ApplyFunc is the punishment.Apply seam tests inject through Spec.Apply.
type ApplyFunc func(context.Context, *gotgbot.Bot, punishment.Request) (punishment.Result, error)

// ExemptFunc reports whether the target is exempt from automated enforcement.
// It has chat_status.IsProtectedForUpdate's signature.
type ExemptFunc func(b *gotgbot.Bot, ctx *ext.Context, chatID, userID int64) bool

// PermissionCheckFunc is the bot-permission precheck: it reports whether the
// bot may perform the punishment in chat. chat_status.CanBotRestrict and
// chat_status.IsBotAdminForUpdate satisfy it.
type PermissionCheckFunc func(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat) bool

// NeverExempt is the exemption policy of a site whose own chain already ran:
// warnThisUser refuses admins, invalid and system targets and requires a live
// member before it ever reaches the limit's punishment, so the protected-user
// predicate must not exempt an approved target a second time.
var NeverExempt ExemptFunc = func(*gotgbot.Bot, *ext.Context, int64, int64) bool { return false }

// LogSpec is the automated action-log entry written after a punishment is
// applied: the tag and its fields. The logged user is always the request's
// target; Name overrides its display name (empty uses the sender's) and
// Markup its inline keyboard (nil uses the punishment Result's LogMarkup).
type LogSpec struct {
	Tag    string
	Name   string
	Markup *gotgbot.InlineKeyboardMarkup
	Fields []actionlog.Field
}

// KeyboardPolicy selects the notice's inline keyboard.
type KeyboardPolicy int

const (
	// KeyboardNone attaches no keyboard.
	KeyboardNone KeyboardPolicy = iota
	// KeyboardUnrestrict attaches the unban/unrestrict convention: an Unmute
	// (Admins Only) button for a mute, an Unban (Admins Only) button for a
	// sender-chat ban, nothing otherwise.
	KeyboardUnrestrict
)

// NoticeSpec is the localized group notice sent after the action log. Key is
// a locale key and Params its {named} or positional ("0", "1", ...) arguments;
// the text is parsed as HTML, so callers pass pre-escaped arguments.
type NoticeSpec struct {
	Key      string
	Params   i18n.TranslationParams
	Keyboard KeyboardPolicy
	// ThreadID is the message thread the notice replies in; 0 for none.
	ThreadID int64
	// Raw sends through Bot.SendMessage with formatting.Shtml() — no error
	// suppression and no thread — for the sites that always did.
	Raw bool
}

// Spec is everything Automated needs to run one automated punishment.
type Spec struct {
	// Request is the punishment to apply.
	Request punishment.Request
	// Apply overrides the punishment.Apply seam; nil uses punishment.Apply.
	Apply ApplyFunc
	// Exempt overrides the protected-user predicate; nil uses
	// chat_status.IsProtectedForUpdate. It is consulted only for
	// user-shaped senders: a channel or anonymous sender is never exempt, as
	// the migrated watchers required.
	Exempt ExemptFunc
	// PermissionCheck is the bot-permission precheck; nil skips it.
	PermissionCheck PermissionCheckFunc
	// BeforeApply runs after the precheck and before Apply, for watcher work
	// that must sit inside the window (deleting the triggering message).
	BeforeApply func()
	// Log is the automated action-log entry; nil logs nothing.
	Log *LogSpec
	// Notice is the localized group notice; nil sends nothing.
	Notice *NoticeSpec
	// LogOnly runs the exemption check and the log (and notice) without a
	// punishment: the spec of an enforcement that reports but never punishes.
	// PermissionCheck and Apply are skipped and the Result stays zero.
	LogOnly bool
}

// Automated runs spec's sequence: exemption check → bot-permission precheck →
// BeforeApply → apply → bail when not applied → automated action log →
// localized group notice. It returns the punishment Result so a caller that
// answers the outcome itself (a warn reply) can; watchers that only continue
// ignore it.
func Automated(b *gotgbot.Bot, ctx *ext.Context, spec Spec) (punishment.Result, error) {
	var chat *gotgbot.Chat
	var sender *gotgbot.Sender
	if ctx != nil {
		chat = ctx.EffectiveChat
		sender = ctx.EffectiveSender
	}

	exempt := spec.Exempt
	if exempt == nil {
		exempt = chat_status.IsProtectedForUpdate
	}
	// Only a user-shaped sender can be exempt: channel and anonymous senders
	// stay punishable through Target.SenderChat, and the protected-user
	// predicate would exempt their non-positive IDs outright.
	if sender != nil && sender.IsUser() &&
		exempt(b, ctx, spec.Request.ChatID, spec.Request.Target.ID) {
		return punishment.Result{}, nil
	}

	var result punishment.Result
	if !spec.LogOnly {
		if spec.PermissionCheck != nil && !spec.PermissionCheck(b, ctx, chat) {
			return result, nil
		}

		if spec.BeforeApply != nil {
			spec.BeforeApply()
		}

		apply := spec.Apply
		if apply == nil {
			apply = punishment.Apply
		}
		var err error
		result, err = apply(tracing.UpdateContext(ctx), b, spec.Request)
		if err != nil {
			return result, err
		}
		if !result.Applied {
			return result, nil
		}
	} else if spec.BeforeApply != nil {
		spec.BeforeApply()
	}

	if spec.Log != nil {
		userName := spec.Log.Name
		if userName == "" && sender != nil {
			userName = sender.Name()
		}
		markup := spec.Log.Markup
		if markup == nil {
			markup = result.LogMarkup
		}
		actionlog.AutomatedWithMarkup(b, chat, markup, spec.Log.Tag,
			spec.Request.Target.ID, userName, spec.Log.Fields...)
	}

	if spec.Notice != nil {
		if err := sendNotice(b, ctx, spec); err != nil {
			return result, err
		}
	}

	return result, nil
}

// sendNotice renders the notice's locale key in the update's language and
// sends it to the request's chat.
func sendNotice(b *gotgbot.Bot, ctx *ext.Context, spec Spec) error {
	notice := spec.Notice
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	text := tr.String(notice.Key, notice.Params)

	if notice.Raw {
		_, err := b.SendMessage(spec.Request.ChatID, text, formatting.Shtml())
		return err
	}
	// The keyboard is attached only when the policy yields rows, as the
	// migrated watchers did: an empty markup would still serialize.
	var markup *gotgbot.InlineKeyboardMarkup
	if rows := unrestrictKeyboard(tr, notice, spec.Request); rows != nil {
		markup = &gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}
	}
	opts := &gotgbot.SendMessageOpts{
		ParseMode:       formatting.HTML,
		MessageThreadId: notice.ThreadID,
		ReplyMarkup:     markup,
	}
	_, err := helpers.SendMessageWithErrorHandling(b, spec.Request.ChatID, text, opts)
	return err
}

// unrestrictKeyboard builds the notice's keyboard rows under the
// unban/unrestrict convention; nil when the policy or the request calls for
// none.
func unrestrictKeyboard(tr *i18n.Translator, notice *NoticeSpec, request punishment.Request) [][]gotgbot.InlineKeyboardButton {
	if notice.Keyboard != KeyboardUnrestrict {
		return nil
	}
	userID := request.Target.ID
	if request.Mode == punishment.Mute {
		return [][]gotgbot.InlineKeyboardButton{
			{keyboardutils.UnrestrictButton(tr, "button_unmute_admins", "unmute", userID, 0)},
		}
	}
	if request.Target.SenderChat && request.Mode == punishment.Ban {
		return [][]gotgbot.InlineKeyboardButton{
			{keyboardutils.UnrestrictButton(tr, "antiflood_button_unban_admins", "unban", userID, 0)},
		}
	}
	return nil
}
