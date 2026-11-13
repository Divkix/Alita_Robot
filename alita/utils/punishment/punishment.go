// Package punishment applies and lifts individual Telegram punishments. Callers
// own exemptions, notices, moderation events and fallback settings. Telegram
// acceptance precedes ban bookkeeping; a failed record write never undoes a ban.
package punishment

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/bans"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/callbackcodec"
	"github.com/divkix/Alita_Robot/alita/utils/punishment/window"
)

type Mode string

const (
	Ban  Mode = "ban"
	Kick Mode = "kick"
	Mute Mode = "mute"
)

// Target distinguishes members from channels posting in the group.
type Target struct {
	ID         int64
	SenderChat bool
}

// Request uses ActorID 0 for the bot.
type Request struct {
	ChatID int64
	Target Target
	Mode   Mode
	// Duration 0 means untimed: a permanent ban or mute. Callers normalise a
	// stored duration first, so a timed setting never reaches Apply as 0.
	Duration time.Duration
	Source   string
	Reason   string
	Detail   string
	ActorID  int64
}

// Result's Refusal says why nothing was applied; it is empty when Applied.
type Result struct {
	Applied   bool
	UntilDate int64
	RecordID  int64
	LogMarkup *gotgbot.InlineKeyboardMarkup
	Refusal   string
}

// RefusalBanned refuses a kick of a user Telegram reports as banned: its
// unban call, which kicks a member, would lift their ban.
const RefusalBanned = "kick target is banned"

// ErrKickTargetBanned is RefusalBanned as an error, for callers that turn a
// refusal into one and answer a banned kick target specially.
var ErrKickTargetBanned = errors.New(RefusalBanned)

// Apply returns a Result with Refusal set when it does not punish, or the
// Telegram error. Most refusals are decided before any API call; a kick reads
// the target's status first, fails when it cannot, and is refused
// (RefusalBanned) when the target is banned. A successful ban remains applied
// even when its record cannot be saved.
func Apply(ctx context.Context, bot *gotgbot.Bot, request Request) (Result, error) {
	if request.Target.SenderChat && (request.Mode != Ban || request.Duration != 0) {
		return Result{Refusal: "sender chats only support permanent bans"}, nil
	}
	if request.Mode == Kick && request.Duration != 0 {
		return Result{Refusal: "kick does not support a duration"}, nil
	}
	now := time.Now()
	var untilDate int64
	if request.Duration != 0 {
		var ok bool
		if request.Duration%time.Second == 0 {
			untilDate, ok = window.UntilDate(now.Unix(), int64(request.Duration/time.Second))
		}
		if !ok {
			return Result{Refusal: "duration outside Telegram's temporary window"}, nil
		}
	}
	var err error
	switch request.Mode {
	case Ban:
		if request.Target.SenderChat {
			_, err = bot.BanChatSenderChatWithContext(ctx, request.ChatID, request.Target.ID, nil)
		} else {
			_, err = bot.BanChatMemberWithContext(ctx, request.ChatID, request.Target.ID, &gotgbot.BanChatMemberOpts{UntilDate: untilDate})
		}
	case Kick:
		var member gotgbot.ChatMember
		if member, err = bot.GetChatMemberWithContext(ctx, request.ChatID, request.Target.ID, nil); err == nil {
			if member.GetStatus() == "kicked" {
				return Result{Refusal: RefusalBanned}, nil
			}
			_, err = bot.UnbanChatMemberWithContext(ctx, request.ChatID, request.Target.ID, &gotgbot.UnbanChatMemberOpts{OnlyIfBanned: false})
		}
	case Mute:
		_, err = bot.RestrictChatMemberWithContext(ctx, request.ChatID, request.Target.ID, MutedPermissions, &gotgbot.RestrictChatMemberOpts{UntilDate: untilDate})
	default:
		return Result{Refusal: "unsupported mode"}, nil
	}
	if err != nil {
		return Result{Refusal: err.Error()}, err
	}
	result := Result{Applied: true, UntilDate: untilDate}
	if request.Mode != Ban {
		return result, nil
	}
	var until *time.Time
	if untilDate != 0 {
		end := time.Unix(untilDate, 0).UTC()
		until = &end
	}
	id, err := bans.Record(models.BanRecord{
		ChatId: request.ChatID, UserId: request.Target.ID, BannedBy: request.ActorID,
		Source: request.Source, Reason: request.Reason, SourceDetail: request.Detail, CreatedAt: now, Until: until,
	})
	if err != nil {
		log.WithContext(ctx).WithError(err).WithFields(log.Fields{
			"chat_id": request.ChatID, "user_id": request.Target.ID, "source": request.Source,
		}).Error("[Bans] Failed to record the ban")
		return result, nil
	}
	result.RecordID = id
	result.LogMarkup = unbanLogMarkup(ctx, request.ChatID, id)
	return result, nil
}

// LogUnbanNamespace is the callback namespace of the Unban button under a
// ban's log channel entry: banlogunban|v1|r=<ban record id>.
const LogUnbanNamespace = "banlogunban"

func unbanLogMarkup(ctx context.Context, chatID, recordID int64) *gotgbot.InlineKeyboardMarkup {
	data, err := callbackcodec.Encode(LogUnbanNamespace, map[string]string{"r": strconv.FormatInt(recordID, 10)})
	if err != nil {
		return nil
	}
	tr := i18n.MustNewTranslator(lang.GetGroupLanguage(ctx, chatID))
	label, _ := tr.GetString("bans_log_unban_button")
	return &gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{{Text: label, CallbackData: data}}}}
}

// LiftRequest names the punishment to remove; ActorID 0 denotes the bot.
type LiftRequest struct {
	ChatID  int64
	Target  Target
	Mode    Mode
	ActorID int64
}

// Lift closes a ban's record only after Telegram accepts the unban; a record
// that cannot be closed is logged and the lift still counts. Mutes restore the
// group's own defaults, never an unrestricted permission set.
func Lift(ctx context.Context, bot *gotgbot.Bot, request LiftRequest) (Result, error) {
	var err error
	switch request.Mode {
	case Ban:
		if request.Target.SenderChat {
			_, err = bot.UnbanChatSenderChatWithContext(ctx, request.ChatID, request.Target.ID, nil)
		} else {
			_, err = bot.UnbanChatMemberWithContext(ctx, request.ChatID, request.Target.ID, &gotgbot.UnbanChatMemberOpts{OnlyIfBanned: true})
		}
	case Mute:
		if request.Target.SenderChat {
			return Result{Refusal: "sender chats cannot be muted"}, nil
		}
		var chat *gotgbot.ChatFullInfo
		chat, err = bot.GetChatWithContext(ctx, request.ChatID, nil)
		if err == nil {
			_, err = bot.RestrictChatMemberWithContext(ctx, request.ChatID, request.Target.ID, unmutePermissions(chat), nil)
		}
	default:
		return Result{Refusal: "unsupported lift mode"}, nil
	}
	if err != nil {
		return Result{Refusal: err.Error()}, err
	}
	if request.Mode == Ban {
		if _, err := bans.Lift(request.ChatID, request.Target.ID, request.ActorID); err != nil {
			log.WithContext(ctx).WithError(err).WithFields(log.Fields{
				"chat_id": request.ChatID, "user_id": request.Target.ID,
			}).Error("[Bans] Failed to close the ban record")
		}
	}
	return Result{Applied: true}, nil
}
