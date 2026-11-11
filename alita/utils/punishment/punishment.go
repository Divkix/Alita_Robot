// Package punishment applies and lifts individual Telegram punishments. Callers
// own exemptions, notices, moderation events and fallback settings. Telegram
// acceptance precedes ban bookkeeping; a failed record write never undoes a ban.
package punishment

import (
	"context"
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

// Request uses a zero Duration for a permanent action and ActorID 0 for the bot.
type Request struct {
	ChatID   int64
	Target   Target
	Mode     Mode
	Duration time.Duration
	Source   string
	Reason   string
	Detail   string
	ActorID  int64
}

type Result struct {
	Applied   bool
	UntilDate int64
	RecordID  int64
	LogMarkup *gotgbot.InlineKeyboardMarkup
	Reason    string
}

// Apply reports a refusal without making an API call, or returns the Telegram
// error. A successful ban remains applied even when its record cannot be saved.
func Apply(ctx context.Context, bot *gotgbot.Bot, request Request) (Result, error) {
	if request.Target.SenderChat && (request.Mode != Ban || request.Duration != 0) {
		return Result{Reason: "sender chats only support permanent bans"}, nil
	}
	if request.Mode == Kick && request.Duration != 0 {
		return Result{Reason: "kick does not support a duration"}, nil
	}
	now := time.Now()
	var untilDate int64
	if request.Duration != 0 {
		var ok bool
		if request.Duration > window.MaxSeconds*time.Second {
			return Result{Reason: "duration outside Telegram's temporary window"}, nil
		}
		untilDate, ok = window.UntilDate(now.Unix(), int64(request.Duration/time.Second))
		if !ok {
			return Result{Reason: "duration outside Telegram's temporary window"}, nil
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
		_, err = bot.UnbanChatMemberWithContext(ctx, request.ChatID, request.Target.ID, &gotgbot.UnbanChatMemberOpts{OnlyIfBanned: false})
	case Mute:
		_, err = bot.RestrictChatMemberWithContext(ctx, request.ChatID, request.Target.ID, MutedPermissions, &gotgbot.RestrictChatMemberOpts{UntilDate: untilDate})
	default:
		return Result{Reason: "unsupported mode"}, nil
	}
	if err != nil {
		return Result{Reason: err.Error()}, err
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

func unbanLogMarkup(ctx context.Context, chatID, recordID int64) *gotgbot.InlineKeyboardMarkup {
	data, err := callbackcodec.Encode("banlogunban", map[string]string{"r": strconv.FormatInt(recordID, 10)})
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

// Lift closes a ban's record only after Telegram accepts the unban. Mutes
// restore the group's own defaults, never an unrestricted permission set.
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
			return Result{Reason: "sender chats cannot be muted"}, nil
		}
		var chat *gotgbot.ChatFullInfo
		chat, err = bot.GetChatWithContext(ctx, request.ChatID, nil)
		if err == nil {
			_, err = bot.RestrictChatMemberWithContext(ctx, request.ChatID, request.Target.ID, UnmutePermissions(chat), nil)
		}
	default:
		return Result{Reason: "unsupported lift mode"}, nil
	}
	if err != nil {
		return Result{Reason: err.Error()}, err
	}
	if request.Mode == Ban {
		if _, err := bans.Lift(request.ChatID, request.Target.ID, request.ActorID); err != nil {
			log.WithContext(ctx).WithError(err).WithFields(log.Fields{
				"chat_id": request.ChatID, "user_id": request.Target.ID,
			}).Error("[Bans] Failed to close the ban record")
			return Result{Applied: true}, err
		}
	}
	return Result{Applied: true}, nil
}
