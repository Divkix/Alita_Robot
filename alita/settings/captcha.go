package settings

import (
	"context"
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db/captcha"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

var captchaRelease func(*gotgbot.Bot, int64) error

// AttachCaptchaRelease is called by the captcha owner during startup, before
// constructing the panel or serving updates. Nil leaves the declaration unbound.
func AttachCaptchaRelease(release func(*gotgbot.Bot, int64) error) {
	captchaRelease = release
}

var CaptchaEnabled = func() *Declaration {
	d := captchaDeclaration("captcha.enabled", Switch, false, AdminBot,
		func(row *models.CaptchaSettings) any { return row.Enabled },
		func(input WriteContext, value any) error {
			if err := captcha.SetCaptchaEnabled(input.ChatID, input.Title, value.(bool)); err != nil {
				return err
			}
			if !value.(bool) {
				if err := captchaRelease(input.Bot, input.ChatID); err != nil {
					return fmt.Errorf("captcha was turned off, but pending users could not be unmuted: %w", err)
				}
			}
			return nil
		})
	d.attachment = func() error {
		if captchaRelease == nil {
			return fmt.Errorf("captcha release is not attached")
		}
		return nil
	}
	d.precondition = func(input WriteContext, value any) error {
		if !value.(bool) && input.Bot == nil {
			return fmt.Errorf("captcha release requires a bot")
		}
		return nil
	}
	return d
}()

var CaptchaJoinRequests = captchaDeclaration("captcha.join_requests", Switch, false, Admin,
	func(row *models.CaptchaSettings) any { return row.JoinRequests },
	func(input WriteContext, value any) error {
		return captcha.SetCaptchaJoinRequests(input.ChatID, input.Title, value.(bool))
	})

var CaptchaMode = func() *Declaration {
	d := captchaDeclaration("captcha.mode", Choice, "math", Admin,
		func(row *models.CaptchaSettings) any { return row.CaptchaMode },
		func(input WriteContext, value any) error {
			return captcha.SetCaptchaMode(input.ChatID, input.Title, value.(string))
		})
	d.Options = []string{"math", "text"}
	return d
}()

// CaptchaTimeout is measured in minutes, matching /captchatime and persistence.
var CaptchaTimeout = captchaNumber("captcha.timeout", 2, Admin,
	func(row *models.CaptchaSettings) any { return row.Timeout },
	func(input WriteContext, value any) error {
		return captcha.SetCaptchaTimeout(input.ChatID, input.Title, value.(int))
	})

var CaptchaAction = func() *Declaration {
	d := captchaDeclaration("captcha.action", Choice, "kick", Admin,
		func(row *models.CaptchaSettings) any { return row.FailureAction },
		func(input WriteContext, value any) error {
			return captcha.SetCaptchaFailureAction(input.ChatID, input.Title, value.(string))
		})
	d.Options = []string{"kick", "ban", "mute"}
	return d
}()

var CaptchaMaxAttempts = captchaNumber("captcha.max_attempts", 3, AdminBot,
	func(row *models.CaptchaSettings) any { return row.MaxAttempts },
	func(input WriteContext, value any) error {
		return captcha.SetCaptchaMaxAttempts(input.ChatID, input.Title, value.(int))
	})

func captchaNumber(key string, def int, right Right, read func(*models.CaptchaSettings) any, write func(WriteContext, any) error) *Declaration {
	d := captchaDeclaration(key, Number, def, right, read, write)
	d.allowed = func(value any) error {
		if n := value.(int); n < 1 || n > 10 {
			if key == "captcha.timeout" {
				return fmt.Errorf("must be between 1 and 10 minutes")
			}
			return fmt.Errorf("must be between 1 and 10")
		}
		return nil
	}
	return d
}

func captchaDeclaration(key string, kind Type, def any, right Right, read func(*models.CaptchaSettings) any, write func(WriteContext, any) error) *Declaration {
	return &Declaration{
		Key: key, Type: kind, Default: def, Right: right,
		read: func(ctx context.Context, chatID int64) any {
			row, err := captcha.GetCaptchaSettingsContext(ctx, chatID)
			if err != nil || row == nil || row.ID == 0 {
				return nil
			}
			return read(row)
		},
		write: write,
	}
}

func ReadCaptcha(chatID int64) (*models.CaptchaSettings, error) {
	return ReadCaptchaContext(context.Background(), chatID)
}

// ReadCaptchaContext copies the cached row and normalizes each declared field.
// Missing settings use declaration defaults without inserting or repairing a row.
func ReadCaptchaContext(ctx context.Context, chatID int64) (*models.CaptchaSettings, error) {
	stored, err := captcha.GetCaptchaSettingsContext(ctx, chatID)
	if err != nil {
		return nil, err
	}
	row := *stored
	if row.ID == 0 {
		row.Enabled = CaptchaEnabled.Default.(bool)
		row.JoinRequests = CaptchaJoinRequests.Default.(bool)
		row.CaptchaMode = CaptchaMode.Default.(string)
		row.Timeout = CaptchaTimeout.Default.(int)
		row.FailureAction = CaptchaAction.Default.(string)
		row.MaxAttempts = CaptchaMaxAttempts.Default.(int)
	} else {
		row.Enabled = CaptchaEnabled.effective(row.Enabled).(bool)
		row.JoinRequests = CaptchaJoinRequests.effective(row.JoinRequests).(bool)
		row.CaptchaMode = CaptchaMode.effective(row.CaptchaMode).(string)
		row.Timeout = CaptchaTimeout.effective(row.Timeout).(int)
		row.FailureAction = CaptchaAction.effective(row.FailureAction).(string)
		row.MaxAttempts = CaptchaMaxAttempts.effective(row.MaxAttempts).(int)
	}
	return &row, nil
}
