package settings

import (
	"context"
	"fmt"
	"time"

	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/probation"
)

var ProbationEnabled = probationDeclaration("antispam.probation_enabled", Switch, false,
	func(row *models.ProbationSettings) any { return row.Enabled },
	func(input WriteContext, value any) error { return probation.SetEnabled(input.ChatID, value.(bool)) })

var ProbationTime = probationNumber("antispam.probation_time", Duration, int(models.ProbationDefaultDuration/time.Second),
	int(models.ProbationDurationMin/time.Second), int(models.ProbationDurationMax/time.Second),
	"must be between 3600 and 2592000 seconds (1 hour to 30 days)",
	func(row *models.ProbationSettings) any { return row.DurationSeconds },
	func(input WriteContext, value any) error {
		return probation.SetDuration(input.ChatID, time.Duration(value.(int))*time.Second)
	})

var ProbationMessages = probationNumber("antispam.probation_messages", Number, models.ProbationDefaultMessages,
	models.ProbationMessagesMin, models.ProbationMessagesMax, "must be between 1 and 100",
	func(row *models.ProbationSettings) any { return row.MessageCount },
	func(input WriteContext, value any) error { return probation.SetMessageCount(input.ChatID, value.(int)) })

var ProbationMinLength = probationNumber("antispam.probation_min_length", Number, models.ProbationDefaultMinLen,
	models.ProbationMinLengthMin, models.ProbationMinLengthMax, "must be between 1 and 500",
	func(row *models.ProbationSettings) any { return row.MinLength },
	func(input WriteContext, value any) error { return probation.SetMinLength(input.ChatID, value.(int)) })

var ProbationNotice = probationDeclaration("antispam.probation_notice", Switch, false,
	func(row *models.ProbationSettings) any { return row.Notice },
	func(input WriteContext, value any) error { return probation.SetNotice(input.ChatID, value.(bool)) })

var ProbationBlockLinks, ProbationBlockForwards, ProbationBlockMedia = probationBlocks()

func probationNumber(key string, kind Type, def, minValue, maxValue int, message string, read func(*models.ProbationSettings) any, write func(WriteContext, any) error) *Declaration {
	d := probationDeclaration(key, kind, def, read, write)
	d.allowed = func(value any) error {
		if n := value.(int); n < minValue || n > maxValue {
			return fmt.Errorf("%s", message)
		}
		return nil
	}
	return d
}

func probationDeclaration(key string, kind Type, def any, read func(*models.ProbationSettings) any, write func(WriteContext, any) error) *Declaration {
	return &Declaration{
		Key: key, Type: kind, Default: def, Right: RestrictUser,
		read:  storedValue(probation.GetSettings, read),
		write: write,
	}
}

func probationBlocks() (*Declaration, *Declaration, *Declaration) {
	links := probationDeclaration("antispam.probation_block_links", Switch, true, func(row *models.ProbationSettings) any { return row.BlockLinks }, nil)
	forwards := probationDeclaration("antispam.probation_block_forwards", Switch, true, func(row *models.ProbationSettings) any { return row.BlockForwards }, nil)
	media := probationDeclaration("antispam.probation_block_media", Switch, false, func(row *models.ProbationSettings) any { return row.BlockMedia }, nil)
	values := func(input WriteContext, changes map[string]any) (bool, bool, bool, error) {
		row, err := ReadProbationContext(input.Context, input.ChatID)
		if err != nil {
			return false, false, false, err
		}
		l, f, m := row.BlockLinks, row.BlockForwards, row.BlockMedia
		for _, field := range []struct {
			d     *Declaration
			value *bool
		}{{links, &l}, {forwards, &f}, {media, &m}} {
			if value, changed := changes[field.d.Key]; changed {
				*field.value = value.(bool)
			}
		}
		if !l && !f && !m {
			return false, false, false, fmt.Errorf("probation must block at least one of links, forwards and media")
		}
		return l, f, m, nil
	}
	group := &sharedWrite{
		fields: []*Declaration{links, forwards, media},
		validate: func(input WriteContext, changes map[string]any) error {
			_, _, _, err := values(input, changes)
			return err
		},
		write: func(input WriteContext, changes map[string]any) error {
			l, f, m, err := values(input, changes)
			if err != nil {
				return err
			}
			return probation.SetBlocked(input.ChatID, l, f, m)
		},
	}
	links.shared, forwards.shared, media.shared = group, group, group
	return links, forwards, media
}

// ReadProbationContext normalizes a copy without changing the cache or creating
// or repairing a settings row.
func ReadProbationContext(ctx context.Context, chatID int64) (*models.ProbationSettings, error) {
	stored, err := probation.GetSettings(ctx, chatID)
	if err != nil || stored == nil {
		return nil, err
	}
	row := *stored
	row.DurationSeconds = ProbationTime.effective(row.DurationSeconds).(int)
	row.MessageCount = ProbationMessages.effective(row.MessageCount).(int)
	row.MinLength = ProbationMinLength.effective(row.MinLength).(int)
	return &row, nil
}
