package modules

import (
	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// captchaOutcome is how a captcha attempt ended.
type captchaOutcome int

const (
	// captchaReleased lets the member in without a verdict: approved, captcha
	// turned off, the challenge could not be delivered, or found unsent after
	// a restart.
	captchaReleased captchaOutcome = iota + 1
)

// settleCaptchaAttempt ends attempt with outcome, in one order for every
// outcome: claim the attempt, deal with the mute, clear or keep the scheduled
// unmute, remove the prompt. settled is false when another ending already
// claimed the attempt; then nothing else happens. err is only work left to
// retry: the scheduled unmute it kept for the unmute worker.
func settleCaptchaAttempt(bot *gotgbot.Bot, attempt *db.CaptchaAttempts, _ captchaOutcome) (settled bool, err error) {
	// The claim deletes the attempt and schedules a fallback unmute in one
	// transaction, so a crash right after it cannot leave the member muted.
	claimed, err := captcha.ReleaseCaptchaAttemptAtomic(attempt.ID, attempt.UserID, attempt.ChatID)
	if err != nil || !claimed {
		return false, err
	}
	err = keepScheduledUnmuteOnFailure(attempt.ChatID, attempt.UserID, unmuteCaptchaUser(bot, attempt.ChatID, attempt.UserID))
	if attempt.MessageID > 0 {
		_ = helpers.DeleteMessageWithErrorHandling(bot, attempt.ChatID, attempt.MessageID)
	}
	return true, err
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
