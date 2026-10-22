package reports

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// SetChatReportMuteThreshold sets how many different members' open reports
// mute a user in chatID; 0 turns the auto-mute off. Other values must lie in
// models.ReportMuteThresholdMin..models.ReportMuteThresholdMax.
func SetChatReportMuteThreshold(chatID int64, threshold int) error {
	if threshold != 0 && (threshold < models.ReportMuteThresholdMin || threshold > models.ReportMuteThresholdMax) {
		return fmt.Errorf("set report mute threshold: %d out of range", threshold)
	}
	if _, err := checkChatReportSettings(chatID); err != nil {
		return fmt.Errorf("set report mute threshold: ensure settings for %d: %w", chatID, err)
	}
	err := db.UpdateRecordWithZeroValues(&models.ReportChatSettings{}, models.ReportChatSettings{ChatId: chatID}, map[string]any{
		"mute_threshold": threshold,
	})
	if err != nil {
		return fmt.Errorf("set report mute threshold: %w", err)
	}
	cache.DeleteCache(cache.CacheKey("report_settings", chatID))
	return nil
}

// CountOpenReporters returns how many different members have an open report
// against userID in chatID. A member who reported several of the user's
// messages counts once.
func CountOpenReporters(chatID, userID int64) (int, error) {
	var n int64
	// 'open' is a literal so a prepared generic plan can use the partial index.
	err := db.DB.Model(&models.Report{}).
		Where("chat_id = ? AND reported_user_id = ? AND status = 'open'", chatID, userID).
		Distinct("reporter_id").
		Count(&n).Error
	if err != nil {
		return 0, fmt.Errorf("count open reporters: %w", err)
	}
	return int(n), nil
}

// ClaimAutoMute records that the auto-mute mutes userID in chatID until
// until. It returns false when a mute is already recorded for the user, so
// of two reports crossing the threshold at once only one mutes, and the same
// reports never mute the user again before an admin decides.
func ClaimAutoMute(chatID, userID int64, until time.Time) (bool, error) {
	res := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}, {Name: "user_id"}},
		DoNothing: true,
	}).Create(&models.ReportAutoMute{ChatId: chatID, UserId: userID, MutedUntil: until})
	if res.Error != nil {
		return false, fmt.Errorf("claim report auto-mute: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

// GetAutoMute returns the auto-mute recorded for userID in chatID, or nil.
func GetAutoMute(chatID, userID int64) (*models.ReportAutoMute, error) {
	var m models.ReportAutoMute
	err := db.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Take(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get report auto-mute: %w", err)
	}
	return &m, nil
}

// ReleaseAutoMute deletes the auto-mute recorded for userID in chatID and
// returns it, or nil when there was none or a concurrent caller deleted it
// first: only the caller that gets it back may lift the mute.
func ReleaseAutoMute(chatID, userID int64) (*models.ReportAutoMute, error) {
	var released []models.ReportAutoMute
	err := db.DB.Model(&released).Clauses(clause.Returning{}).
		Where("chat_id = ? AND user_id = ?", chatID, userID).
		Delete(&released).Error
	if err != nil {
		return nil, fmt.Errorf("release report auto-mute: %w", err)
	}
	if len(released) == 0 {
		return nil, nil
	}
	return &released[0], nil
}

// DeleteAutoMutesBefore removes auto-mutes that ended before cutoff and
// returns how many it removed.
func DeleteAutoMutesBefore(cutoff time.Time) (int64, error) {
	res := db.DB.Where("muted_until < ?", cutoff).Delete(&models.ReportAutoMute{})
	if res.Error != nil {
		return 0, fmt.Errorf("delete expired report auto-mutes: %w", res.Error)
	}
	return res.RowsAffected, nil
}
