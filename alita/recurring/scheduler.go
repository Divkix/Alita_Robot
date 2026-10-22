// Package recurring posts the messages admins set with /recurring.
package recurring

import (
	"context"
	"errors"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/recurringmessages"
	"github.com/divkix/Alita_Robot/alita/utils/content"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/keyboard"
	"github.com/divkix/Alita_Robot/alita/utils/media"
	"github.com/divkix/Alita_Robot/alita/utils/ratelimit"
)

const (
	// TickInterval is how often the scheduler looks for due messages, so an
	// occurrence goes out within about a minute of its time.
	TickInterval = time.Minute
	// dueBatch caps the messages one tick handles; the rest stay due for the
	// next tick.
	dueBatch = 500
)

// Scheduler posts every enabled recurring message when it comes due.
//
// A tick lists the due messages and claims each occurrence with a conditional
// update moving next_run_at to the next occurrence; it sends only when the
// claim won, so two replicas ticking in the same minute post one message, not
// two. Occurrences missed while the bot was down are not caught up: the next
// run is the first one after now on the message's schedule, and one message
// goes out for the missed ones.
//
// What a failed send does depends on why it failed:
//   - The chat can never receive it (helpers.IsPermanentSendError, or the
//     topic is gone): the message is stopped; it stays listed until an admin
//     removes it.
//   - Shutdown or a Telegram flood wait stopped it: the claim is given back
//     and the next tick posts it.
//   - Anything else, including a bot that may not write in the chat (an admin
//     can give the right back): this occurrence is skipped and the next one is
//     tried on schedule, so content Telegram rejects is not retried every
//     minute.
//
// Sends are paced by the global send limiter.
type Scheduler struct {
	bot      *gotgbot.Bot
	interval time.Duration
	limiter  *ratelimit.SendLimiter
	// now is the clock a tick reads. Tests replace it; production never
	// writes it after construction.
	now func() time.Time

	startOnce sync.Once
	stopOnce  sync.Once
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewScheduler returns a scheduler for bot. It does not start ticking.
func NewScheduler(bot *gotgbot.Bot) *Scheduler {
	return &Scheduler{bot: bot, interval: TickInterval, limiter: ratelimit.GlobalSendLimiter(), now: time.Now}
}

// Start begins the ticker. Calling it twice does nothing.
func (s *Scheduler) Start() {
	s.startOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		s.cancel = cancel
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer error_handling.RecoverFromPanic("recurringScheduler", "recurring")

			ticker := time.NewTicker(s.interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					func() {
						defer error_handling.RecoverFromPanic("recurringSchedulerTick", "recurring")
						s.runDue(ctx, s.now())
					}()
				}
			}
		}()
	})
}

// Stop stops the ticker and waits for a running tick. It is idempotent and
// safe on a scheduler that was never started.
func (s *Scheduler) Stop() {
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		s.wg.Wait()
	})
}

// runDue posts every message due at now.
func (s *Scheduler) runDue(ctx context.Context, now time.Time) {
	due, err := recurringmessages.ListDue(now, dueBatch)
	if err != nil {
		log.Errorf("[Recurring] could not list due messages: %v", err)
		return
	}
	for i := range due {
		// Shutdown, or a flood wait the limiter will not wait out, ends the
		// tick before the next claim: the messages left stay due.
		if ctx.Err() != nil {
			return
		}
		if paused := s.limiter.PausedFor(); paused > ratelimit.MaxSendRetryAfter {
			log.WithField("paused_for", paused).Warn("[Recurring] Telegram flood wait; leaving the rest for the next tick")
			return
		}
		s.fire(ctx, &due[i], now)
	}
}

// NextRun returns the first occurrence after now on the schedule that starts
// at from and repeats every interval. from itself is returned when it is
// after now.
func NextRun(from time.Time, interval time.Duration, now time.Time) time.Time {
	if from.After(now) || interval <= 0 {
		return from
	}
	missed := now.Sub(from) / interval
	return from.Add((missed + 1) * interval)
}

// fire claims msg's due occurrence and posts it.
func (s *Scheduler) fire(ctx context.Context, msg *models.RecurringMessage, now time.Time) {
	fields := log.Fields{"chat_id": msg.ChatID, "recurring_id": msg.ID}
	next := NextRun(msg.NextRunAt, msg.Interval(), now)

	// Past its end time the message stops without posting, also when the
	// runs it missed were before the end.
	if msg.EndsAt != nil && now.After(*msg.EndsAt) {
		if _, err := recurringmessages.Claim(msg, next, true); err != nil {
			log.WithFields(fields).WithError(err).Error("[Recurring] could not stop an ended message")
		}
		return
	}

	last := msg.EndsAt != nil && next.After(*msg.EndsAt)
	won, err := recurringmessages.Claim(msg, next, last)
	if err != nil {
		log.WithFields(fields).WithError(err).Error("[Recurring] could not claim a due message")
		return
	}
	if !won {
		return
	}

	sent, err := s.post(ctx, msg)
	if err != nil {
		s.handleSendError(ctx, msg, next, err)
		return
	}

	if !msg.DeletePrevious || sent == nil {
		return
	}
	if msg.LastMessageID != 0 {
		// Best effort: without the delete right the bot can only remove its
		// own messages for 48 hours, and an admin may have removed it.
		delErr := s.limiter.Do(ctx, func() error {
			_, err := s.bot.DeleteMessage(msg.ChatID, msg.LastMessageID, nil)
			return err
		})
		if delErr != nil {
			log.WithFields(fields).WithError(delErr).Debug("[Recurring] could not delete the previous occurrence")
		}
	}
	if err := recurringmessages.SetLastMessageID(msg.ChatID, msg.ID, sent.MessageId); err != nil {
		log.WithFields(fields).WithError(err).Error("[Recurring] could not save the posted message")
	}
}

func (s *Scheduler) handleSendError(ctx context.Context, msg *models.RecurringMessage, claimedNext time.Time, err error) {
	entry := log.WithFields(log.Fields{"chat_id": msg.ChatID, "recurring_id": msg.ID, "error": err})
	switch {
	case IsUndeliverable(err):
		entry.Warn("[Recurring] message is undeliverable; stopping it")
		if disableErr := recurringmessages.Disable(msg.ChatID, msg.ID); disableErr != nil {
			entry.WithField("disable_error", disableErr).Error("[Recurring] could not stop an undeliverable message")
		}
	case ctx.Err() != nil || errors.Is(err, ratelimit.ErrSendPaused) || isFloodWait(err):
		if ctx.Err() != nil {
			entry.Info("[Recurring] shutdown stopped a send; giving the claim back")
		} else {
			entry.Warn("[Recurring] Telegram flood wait; giving the claim back")
		}
		if restoreErr := recurringmessages.Restore(msg, claimedNext); restoreErr != nil {
			entry.WithField("restore_error", restoreErr).Error("[Recurring] could not restore the claim")
		}
	default:
		entry.Error("[Recurring] could not post a message; skipping this occurrence")
	}
}

// IsUndeliverable reports whether err means no later occurrence can be posted
// either: a permanent send error, or the topic the message goes to is gone.
// A bot that may not write (media.ErrNoPermission) is not undeliverable: an
// admin can give the right back.
func IsUndeliverable(err error) bool {
	return helpers.IsPermanentSendError(err) ||
		strings.Contains(err.Error(), "message thread not found")
}

func isFloodWait(err error) bool {
	_, flooded := ratelimit.RetryAfter(err)
	return flooded
}

// post renders msg the way a note is rendered and sends it through the
// limiter: a random part of %%%-separated text, the {chatname}/{count}/{rules}
// fillings (there is no user, so user fillings read as a person with no
// name), and the {preview}/{protect}/{nonotif} directives.
func (s *Scheduler) post(ctx context.Context, msg *models.RecurringMessage) (*gotgbot.Message, error) {
	chat := &gotgbot.Chat{Id: msg.ChatID, Type: "supergroup"}
	// The title only fills {chatname}; skip the lookup for other messages.
	if strings.Contains(msg.Text, "{chatname}") {
		if info, err := chats.GetChatBasicInfoCached(msg.ChatID); err == nil && info != nil {
			chat.Title = info.ChatName
		}
	}

	text := msg.Text
	if parts := strings.Split(text, "%%%"); len(parts) > 1 {
		text = parts[rand.Intn(len(parts))] // #nosec G404 - Non-cryptographic random is sufficient for selecting messages
	}
	text, buttons := formatting.FormattingReplacer(s.bot, chat, nil, text, msg.Buttons)
	_, _, _, webPreview, protected, noNotif, text := content.NotesParser(text)
	markup := &gotgbot.InlineKeyboardMarkup{InlineKeyboard: keyboard.BuildKeyboard(buttons)}

	var sent *gotgbot.Message
	err := s.limiter.Do(ctx, func() error {
		var sendErr error
		sent, sendErr = media.Send(s.bot, media.Content{
			Text:    text,
			FileID:  msg.FileID,
			MsgType: msg.MsgType,
			Name:    "recurring",
		}, media.Options{
			ChatID:      msg.ChatID,
			ThreadID:    msg.ThreadID,
			Keyboard:    markup,
			NoNotif:     noNotif,
			WebPreview:  webPreview,
			IsProtected: protected,
		})
		return sendErr
	})
	return sent, err
}
