// Package joinrequestcaptchas stores applicant challenges separately from member
// captchas. Terminal actions atomically delete the observed attempt so a stale
// callback or timeout cannot act on a replacement request.
package joinrequestcaptchas

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

var ErrNoActiveCaptcha = errors.New("NO_ACTIVE_JOIN_REQUEST_CAPTCHA")

func attemptCacheKey(chatID, userID int64) string {
	return cache.CacheKey("join_request_captcha", chatID, userID)
}

func attemptIDCacheKey(id uint) string {
	return cache.CacheKey("join_request_captcha_id", id)
}

func invalidateAttempt(attempt *models.JoinRequestCaptcha) {
	cache.DeleteCache(attemptCacheKey(attempt.ChatID, attempt.UserID))
	cache.DeleteCache(attemptIDCacheKey(attempt.ID))
}

// CreateJoinRequestCaptcha replaces any previous request with a new identity.
// firstName is retained for prompts and logs after startup recovery.
// timeout is in minutes, bounded by Telegram's five-minute PM window.
func CreateJoinRequestCaptcha(chatID, userID, messageID, userChatID int64, firstName, answer string, timeout int64) (*models.JoinRequestCaptcha, error) {
	if timeout > 5 {
		timeout = 5
	}
	attempt := &models.JoinRequestCaptcha{
		ChatID:     chatID,
		UserID:     userID,
		MessageID:  messageID,
		UserChatID: userChatID,
		FirstName:  firstName,
		Answer:     answer,
		ExpiresAt:  time.Now().UTC().Add(time.Duration(timeout) * time.Minute),
	}
	var previous models.JoinRequestCaptcha
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		if tx.Name() == "postgres" {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", fmt.Sprintf("join-request:%d:%d", chatID, userID)).Error; err != nil {
				return err
			}
		}
		err := tx.Where("chat_id = ? AND user_id = ?", chatID, userID).First(&previous).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if previous.ID != 0 {
			if err := tx.Delete(&previous).Error; err != nil {
				return err
			}
		}
		return tx.Create(attempt).Error
	})
	if err != nil {
		return nil, err
	}
	// Invalidate only after commit, otherwise a reader could cache the old row.
	invalidateAttempt(attempt)
	if previous.ID != 0 {
		cache.DeleteCache(attemptIDCacheKey(previous.ID))
	}
	return attempt, nil
}

// GetJoinRequestCaptcha includes expired attempts; expiry is checked atomically
// by mutations, not by a cached read that could outlive the deadline.
func GetJoinRequestCaptcha(chatID, userID int64) (*models.JoinRequestCaptcha, error) {
	return cache.GetFromCacheOrLoad(context.Background(), attemptCacheKey(chatID, userID), time.Minute, func(ctx context.Context) (*models.JoinRequestCaptcha, error) {
		var attempt models.JoinRequestCaptcha
		err := db.DB.WithContext(ctx).Where("chat_id = ? AND user_id = ?", chatID, userID).First(&attempt).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return &attempt, nil
	})
}

func GetJoinRequestCaptchaByID(id uint) (*models.JoinRequestCaptcha, error) {
	return cache.GetFromCacheOrLoad(context.Background(), attemptIDCacheKey(id), time.Minute, func(ctx context.Context) (*models.JoinRequestCaptcha, error) {
		var attempt models.JoinRequestCaptcha
		err := db.DB.WithContext(ctx).First(&attempt, id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return &attempt, nil
	})
}

func UpdateJoinRequestCaptchaMessageID(id uint, messageID int64) error {
	var attempt models.JoinRequestCaptcha
	result := db.DB.Model(&attempt).Clauses(clause.Returning{}).Where("id = ?", id).Update("message_id", messageID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNoActiveCaptcha
	}
	invalidateAttempt(&attempt)
	return nil
}

func IncrementJoinRequestCaptchaAttempts(id uint, refreshCount, maxAttempts int) (int, error) {
	var attempt models.JoinRequestCaptcha
	result := db.DB.Model(&attempt).Clauses(clause.Returning{}).
		Where("id = ? AND refresh_count = ? AND attempts < ? AND expires_at > ?", id, refreshCount, maxAttempts, time.Now()).
		Update("attempts", gorm.Expr("attempts + 1"))
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected == 0 {
		return 0, ErrNoActiveCaptcha
	}
	invalidateAttempt(&attempt)
	return attempt.Attempts, nil
}

// RefreshJoinRequestCaptcha preserves attempts and expiry. A nil result means
// another callback won, the attempt expired, or the refresh cap was reached.
func RefreshJoinRequestCaptcha(id uint, answer string, refreshCount int, messageID int64) (*models.JoinRequestCaptcha, error) {
	var attempt models.JoinRequestCaptcha
	result := db.DB.Model(&attempt).Clauses(clause.Returning{}).
		Where("id = ? AND refresh_count = ? AND refresh_count < ? AND expires_at > ?", id, refreshCount, models.CaptchaMaxRefreshes, time.Now()).
		Updates(map[string]any{"answer": answer, "message_id": messageID, "refresh_count": gorm.Expr("refresh_count + 1")})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}
	invalidateAttempt(&attempt)
	return &attempt, nil
}

func CompleteJoinRequestCaptchaAtomic(id uint, userID, chatID int64, answer string, refreshCount, maxAttempts int) (bool, error) {
	var attempt models.JoinRequestCaptcha
	result := db.DB.Clauses(clause.Returning{}).
		Where("id = ? AND user_id = ? AND chat_id = ? AND answer = ? AND refresh_count = ? AND attempts < ? AND expires_at > ?", id, userID, chatID, answer, refreshCount, maxAttempts, time.Now()).
		Delete(&attempt)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	invalidateAttempt(&attempt)
	return true, nil
}

// ReleaseJoinRequestCaptchaAtomic claims failure from an observed generation,
// including expired attempts. It competes with completion for the same row.
func ReleaseJoinRequestCaptchaAtomic(id uint, userID, chatID int64, answer string, refreshCount int) (bool, error) {
	var attempt models.JoinRequestCaptcha
	result := db.DB.Clauses(clause.Returning{}).
		Where("id = ? AND user_id = ? AND chat_id = ? AND answer = ? AND refresh_count = ?", id, userID, chatID, answer, refreshCount).
		Delete(&attempt)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	invalidateAttempt(&attempt)
	return true, nil
}

func DeleteJoinRequestCaptcha(id uint) error {
	var attempt models.JoinRequestCaptcha
	result := db.DB.Clauses(clause.Returning{}).Where("id = ?", id).Delete(&attempt)
	if result.Error != nil {
		return result.Error
	}
	cache.DeleteCache(attemptIDCacheKey(id))
	if result.RowsAffected > 0 {
		cache.DeleteCache(attemptCacheKey(attempt.ChatID, attempt.UserID))
	}
	return nil
}

func GetExpiredJoinRequestCaptchas() ([]*models.JoinRequestCaptcha, error) {
	var attempts []*models.JoinRequestCaptcha
	err := db.DB.Where("expires_at <= ?", time.Now()).Order("expires_at, id").Find(&attempts).Error
	return attempts, err
}

// GetPendingJoinRequestCaptchas includes expired attempts for startup recovery.
func GetPendingJoinRequestCaptchas() ([]*models.JoinRequestCaptcha, error) {
	var attempts []*models.JoinRequestCaptcha
	err := db.DB.Order("id").Find(&attempts).Error
	return attempts, err
}
