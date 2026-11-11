package modules

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/actionlog"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// captchaEnding is the kind of a captchaOutcome.
type captchaEnding int

const (
	captchaEndReleased captchaEnding = iota + 1
	captchaEndPassed
)

// captchaOutcome is how a captcha attempt ended.
type captchaOutcome struct {
	ending captchaEnding
	// A pass claims only with answer on the prompt the member saw, at
	// refreshCount; its log entry and summary name the member and count the
	// attempts out of maxAttempts.
	answer       string
	refreshCount int
	name         string
	maxAttempts  int
}

// captchaReleased lets the member in without a verdict: approved, captcha
// turned off, the challenge could not be delivered, or found unsent after a
// restart.
var captchaReleased = captchaOutcome{ending: captchaEndReleased}

// captchaPassed is a tap on answer, the right one, by the member called name.
func captchaPassed(answer string, refreshCount int, name string, maxAttempts int) captchaOutcome {
	return captchaOutcome{ending: captchaEndPassed, answer: answer, refreshCount: refreshCount, name: name, maxAttempts: maxAttempts}
}

// settleCaptchaAttempt ends attempt with outcome, in one order for every
// outcome: claim the attempt, deal with the mute, clear or keep the scheduled
// unmute, remove the prompt, report. settled is false when another ending
// already claimed the attempt; then nothing else happens. err is only work
// left to retry: the scheduled unmute it kept for the unmute worker.
func settleCaptchaAttempt(bot *gotgbot.Bot, attempt *db.CaptchaAttempts, outcome captchaOutcome) (settled bool, err error) {
	// The claim deletes the attempt with its stored messages and schedules a
	// fallback unmute in one transaction, so a crash right after it cannot
	// leave the member muted. A pass reads the stored messages before.
	var stored []*models.StoredMessages
	claimed := false
	switch outcome.ending {
	case captchaEndPassed:
		var storedErr error
		if stored, storedErr = captcha.GetStoredMessagesForAttempt(attempt.ID); storedErr != nil {
			log.Warnf("[Captcha] Failed to read stored messages for attempt %d: %v", attempt.ID, storedErr)
		}
		claimed, err = captcha.CompleteCaptchaAttemptAtomic(attempt.ID, attempt.UserID, attempt.ChatID, outcome.answer, attempt.MessageID, outcome.refreshCount)
	default:
		claimed, err = captcha.ReleaseCaptchaAttemptAtomic(attempt.ID, attempt.UserID, attempt.ChatID)
	}
	if err != nil || !claimed {
		return false, err
	}
	err = keepScheduledUnmuteOnFailure(attempt.ChatID, attempt.UserID, unmuteCaptchaUser(bot, attempt.ChatID, attempt.UserID))
	if attempt.MessageID > 0 {
		_ = helpers.DeleteMessageWithErrorHandling(bot, attempt.ChatID, attempt.MessageID)
	}
	// A member still held has not passed.
	if outcome.ending == captchaEndPassed && err == nil {
		reportCaptchaPass(bot, attempt, outcome, stored)
	}
	return true, err
}

// reportCaptchaPass logs the pass with its attempts, the correct answer
// counting as one, and posts the kinds and count of what the member sent
// while held, never the messages; the summary removes itself after 30s.
func reportCaptchaPass(bot *gotgbot.Bot, attempt *db.CaptchaAttempts, outcome captchaOutcome, stored []*models.StoredMessages) {
	actionlog.Join(bot, captchaLogChat(attempt.ChatID), "CAPTCHA_PASSED", attempt.UserID, outcome.name,
		actionlog.Field{Name: "Attempts", Value: fmt.Sprintf("%d/%d", attempt.Attempts+1, outcome.maxAttempts)},
	)
	if len(stored) == 0 {
		return
	}
	tr := i18n.MustNewTranslator(lang.GetGroupLanguage(context.Background(), attempt.ChatID))
	var kinds []string
	for _, msg := range stored {
		if kind := messageTypeToString(tr, msg.MessageType); !slices.Contains(kinds, kind) {
			kinds = append(kinds, kind)
		}
	}
	text, _ := tr.GetString("captcha_stored_messages_summary",
		captchaStoredSummaryParams(formatting.MentionHtml(attempt.UserID, outcome.name), len(stored), strings.Join(kinds, ", ")))
	sent, _ := helpers.SendMessageWithErrorHandling(bot, attempt.ChatID, text, &gotgbot.SendMessageOpts{ParseMode: formatting.HTML})
	if sent != nil {
		time.AfterFunc(30*time.Second, func() {
			defer error_handling.RecoverFromPanic("captchaSummaryMsgDelete", "captcha")
			_ = helpers.DeleteMessageWithErrorHandling(bot, attempt.ChatID, sent.MessageId)
		})
	}
}

// keepScheduledUnmuteOnFailure is the one rule when lifting a mute fails. A
// permanent Telegram error (member gone, chat gone, member now an admin) leaves
// nothing to unmute: the scheduled unmute is cleared and it is no failure. Any
// other error keeps the scheduled unmute for the unmute worker and is returned.
func keepScheduledUnmuteOnFailure(chatID, userID int64, unmuteErr error) error {
	if unmuteErr != nil && !isPermanentUnmuteError(unmuteErr) {
		return unmuteErr
	}
	return captcha.DeleteMutedUser(userID, chatID)
}
