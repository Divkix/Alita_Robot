package moderation

import (
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// Record stores one event. A missing CreatedAt is stamped in UTC, so every row
// carries the instant the action happened whatever the caller's clock zone.
// The reason is cut to models.MaxModerationReasonRunes at a rune boundary; no
// message text is ever passed here.
func Record(ev models.ModerationEvent) error {
	if ev.CreatedAt.IsZero() {
		ev.CreatedAt = time.Now().UTC()
	}
	if utf8.RuneCountInString(ev.Reason) > models.MaxModerationReasonRunes {
		ev.Reason = string([]rune(ev.Reason)[:models.MaxModerationReasonRunes])
	}
	// The chat foreign key needs its parent row, and a command can arrive
	// before the users tracker creates it; evidence must not be dropped over
	// that race (reports.Create ensures the same row for the same reason).
	if err := chats.EnsureChatInDb(ev.ChatId, ""); err != nil {
		return fmt.Errorf("record moderation event: %w", err)
	}
	if err := db.DB.Create(&ev).Error; err != nil {
		return fmt.Errorf("record moderation event: %w", err)
	}
	return nil
}

// DeleteExpiredBefore removes events created before cutoff and returns how
// many it removed. Time bounds are compared in UTC, the zone Record writes:
// SQLite compares the stored text, so a local-zone bound would misorder.
func DeleteExpiredBefore(cutoff time.Time) (int64, error) {
	res := db.DB.Where("created_at < ?", cutoff.UTC()).Delete(&models.ModerationEvent{})
	if res.Error != nil {
		return 0, fmt.Errorf("delete expired moderation events: %w", res.Error)
	}
	return res.RowsAffected, nil
}

// ListForChat returns a chat's events from since on, newest first. A limit of
// zero or less returns every match.
func ListForChat(chatID int64, since time.Time, limit int) ([]models.ModerationEvent, error) {
	query := db.DB.
		Where("chat_id = ? AND created_at >= ?", chatID, since.UTC()).
		Order("created_at DESC, id DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	var rows []models.ModerationEvent
	if err := query.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list moderation events for chat: %w", err)
	}
	return rows, nil
}

// ListForTarget returns a target's events from since on, newest first.
func ListForTarget(userID int64, since time.Time) ([]models.ModerationEvent, error) {
	var rows []models.ModerationEvent
	err := db.DB.
		Where("target_user_id = ? AND created_at >= ?", userID, since.UTC()).
		Order("created_at DESC, id DESC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list moderation events for target: %w", err)
	}
	return rows, nil
}

// ReputationSignal is the shared reputation aggregate for one user: how many
// distinct chats banned or kicked them since (inclusive), and whether a human
// unban in the same window reverses one of those removals. An unban counts
// only when the same chat recorded a ban or kick of the same user earlier in
// the window, so a no-op /unban in a chat that never removed the user clears
// nothing; a removal and an unban stamped at the same instant count as a
// reversal, since there is no finer order. Ten bans in one chat still count as
// one chat; mutes never count, because only ban and kick events feed it.
//
// Both reads use the (target_user_id, created_at) index and compare the bound
// in UTC like Record writes it: SQLite compares the stored text, so a
// local-zone bound would misorder.
func ReputationSignal(userID int64, since time.Time) (chats int, unbanned bool, err error) {
	var chatCount int64
	err = db.DB.Model(&models.ModerationEvent{}).
		Where("target_user_id = ? AND created_at >= ? AND action IN ?",
			userID, since.UTC(), []string{models.ModerationActionBan, models.ModerationActionKick}).
		Distinct("chat_id").
		Count(&chatCount).Error
	if err != nil {
		return 0, false, fmt.Errorf("count chats that removed a target: %w", err)
	}

	// Only an unban that follows a removal in the same chat reverses the
	// signal; an unban in a chat that never acted on the user is a no-op.
	var reversingUnban int64
	err = db.DB.Model(&models.ModerationEvent{}).
		Where("target_user_id = ? AND created_at >= ? AND action = ?",
			userID, since.UTC(), models.ModerationActionUnban).
		Where(`EXISTS (
			SELECT 1 FROM moderation_events AS removal
			WHERE removal.target_user_id = moderation_events.target_user_id
				AND removal.chat_id = moderation_events.chat_id
				AND removal.created_at >= ?
				AND removal.created_at <= moderation_events.created_at
				AND removal.action IN (?, ?)
		)`, since.UTC(), models.ModerationActionBan, models.ModerationActionKick).
		Count(&reversingUnban).Error
	if err != nil {
		return 0, false, fmt.Errorf("check for a target unban: %w", err)
	}
	return int(chatCount), reversingUnban > 0, nil
}
