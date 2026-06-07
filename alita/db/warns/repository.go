package warns

import (
	"context"
	"errors"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/i18n"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// checkWarnSettings retrieves or creates default warn settings for a chat.
// Returns default settings with warn limit 3 and mute mode if the chat doesn't exist.
func checkWarnSettings(chatID int64) (warnrc *models.WarnSettings) {
	defaultWarnSettings := &models.WarnSettings{ChatId: chatID, WarnLimit: 3, WarnMode: "mute"}
	warnrc = &models.WarnSettings{}
	err := db.DB.Where("chat_id = ?", chatID).First(warnrc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Ensure chat exists before creating warn settings
		if !db.ChatExists(chatID) {
			// Chat doesn't exist, return default settings without creating record
			log.Warnf("[Database][checkWarnSettings]: Chat %d doesn't exist, returning default settings", chatID)
			return defaultWarnSettings
		}

		// Create default settings only if chat exists
		warnrc = defaultWarnSettings
		err := db.DB.Create(warnrc).Error
		if err != nil {
			log.Errorf("[Database] checkWarnSettings: %v", err)
		}
	} else if err != nil {
		log.Errorf("[Database][checkWarnSettings]: %d - %v", chatID, err)
		warnrc = defaultWarnSettings
	}
	return
}

// checkWarns retrieves or creates default warn record for a user in a specific chat.
// Returns default record with 0 warns if the chat doesn't exist or user has no warns.
func checkWarns(userId, chatId int64) (warnrc *models.Warns) {
	defaultWarnSrc := &models.Warns{UserId: userId, ChatId: chatId, NumWarns: 0, Reasons: make(models.StringArray, 0)}
	warnrc = &models.Warns{}
	err := db.DB.Where("user_id = ? AND chat_id = ?", userId, chatId).First(warnrc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Ensure chat exists before creating warn record
		if !db.ChatExists(chatId) {
			// Chat doesn't exist, return default settings without creating record
			log.Warnf("[Database][checkWarns]: Chat %d doesn't exist, returning default settings", chatId)
			return defaultWarnSrc
		}

		// Create default record only if chat exists
		warnrc = defaultWarnSrc
		err := db.DB.Create(warnrc).Error
		if err != nil {
			log.Errorf("[Database] checkWarns: %v", err)
		}
	} else if err != nil {
		log.Errorf("[Database][checkUserWarns]: %d - %v", userId, err)
		warnrc = defaultWarnSrc
	}
	return
}

// WarnUser adds a warning to a user in a specific chat with an optional reason.
// Returns the total number of warnings and all warning reasons for the user.
func WarnUser(userId, chatId int64, reason string) (int, []string) {
	return WarnUserWithContext(context.Background(), userId, chatId, reason)
}

// WarnUserWithContext adds a warning to a user with context support for cancellation.
// Uses database transactions to ensure data consistency and supports context cancellation.
// Returns the total number of warnings and all warning reasons for the user.
func WarnUserWithContext(ctx context.Context, userId, chatId int64, reason string) (int, []string) {
	var numWarns int
	var reasons []string

	err := db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Check warn settings within transaction
		warnSettings := &models.WarnSettings{}
		if err := tx.Where("chat_id = ?", chatId).First(warnSettings).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// Create default settings
				warnSettings = &models.WarnSettings{ChatId: chatId, WarnLimit: 3}
				if err := tx.Create(warnSettings).Error; err != nil {
					return err
				}
			}
		}

		// Check warns within transaction
		warnrc := &models.Warns{}
		if err := tx.Where("user_id = ? AND chat_id = ?", userId, chatId).First(warnrc).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// Create new warn record
				warnrc = &models.Warns{UserId: userId, ChatId: chatId}
			}
		}

		warnrc.NumWarns++ // Increment warns

		// Add reason
		if reason != "" {
			if len(reason) >= 3001 {
				reason = reason[:3000]
			}
			warnrc.Reasons = append(warnrc.Reasons, reason)
		} else {
			// Use default language for "No Reason" - this could be improved to use chat language
			tr := i18n.MustNewTranslator("en")
			noReason, _ := tr.GetString("db_warn_no_reason")
			if noReason == "" {
				noReason = "No Reason" // fallback
			}
			warnrc.Reasons = append(warnrc.Reasons, noReason)
		}

		// Save the warn record
		if err := tx.Save(warnrc).Error; err != nil {
			return err
		}

		numWarns = warnrc.NumWarns
		reasons = []string(warnrc.Reasons)
		return nil
	})
	if err != nil {
		log.Errorf("[Database] WarnUser: %v", err)
		return 0, []string{}
	}

	// Invalidate cache after successful transaction
	cache.DeleteCache(cache.CacheKey("warns", userId, chatId))
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))

	return numWarns, reasons
}

// RemoveWarn removes the most recent warning from a user in a specific chat.
// Returns true if a warning was successfully removed, false otherwise.
func RemoveWarn(userId, chatId int64) bool {
	return RemoveWarnWithContext(context.Background(), userId, chatId)
}

// RemoveWarnWithContext removes the most recent warning with context support.
// Uses database transactions to ensure data consistency and supports context cancellation.
// Returns true if a warning was successfully removed, false otherwise.
func RemoveWarnWithContext(ctx context.Context, userId, chatId int64) bool {
	var removed bool

	err := db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		warnrc := &models.Warns{}
		if err := tx.Where("user_id = ? AND chat_id = ?", userId, chatId).First(warnrc).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// No warns to remove
				removed = false
				return nil
			}
			return err
		}

		// only remove if user has warns
		if warnrc.NumWarns > 0 {
			warnrc.NumWarns-- // Remove last warn num
			if len(warnrc.Reasons) > 0 {
				warnrc.Reasons = warnrc.Reasons[:len(warnrc.Reasons)-1] // Remove last warn reason
			}
			removed = true

			// update record in db within transaction
			if err := tx.Save(warnrc).Error; err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		log.Errorf("[Database] RemoveWarn: %v", err)
		return false
	}

	// Invalidate cache after successful transaction
	if removed {
		cache.DeleteCache(cache.CacheKey("warns", userId, chatId))
		cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	}

	return removed
}

// ResetUserWarns removes all warnings for a specific user in a chat.
// Returns true if a row was actually deleted, false on error or when no warns existed.
func ResetUserWarns(userId, chatId int64) bool {
	result := db.DB.Where("user_id = ? AND chat_id = ?", userId, chatId).Delete(&models.Warns{})
	if result.Error != nil {
		log.Errorf("[Database] ResetUserWarns: %v", result.Error)
		return false
	}
	if result.RowsAffected == 0 {
		return false
	}
	cache.DeleteCache(cache.CacheKey("warns", userId, chatId))
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	return true
}

// GetWarns retrieves the current warning count and reasons for a user in a specific chat.
// Results are cached to avoid repeated database queries.
func GetWarns(userId, chatId int64) (int, []string) {
	type warnCache struct {
		NumWarns int
		Reasons  []string
	}
	cached, err := cache.GetFromCacheOrLoad(
		cache.CacheKey("warns", userId, chatId),
		cache.CacheTTLLanguage,
		func() (warnCache, error) {
			w := checkWarns(userId, chatId)
			return warnCache{NumWarns: w.NumWarns, Reasons: []string(w.Reasons)}, nil
		},
	)
	if err != nil {
		w := checkWarns(userId, chatId)
		return w.NumWarns, []string(w.Reasons)
	}
	return cached.NumWarns, cached.Reasons
}

// SetWarnLimit updates the warning limit for a specific chat.
// When users reach this limit, the configured warn mode action is applied.
func SetWarnLimit(chatId int64, warnLimit int) error {
	warnrc := checkWarnSettings(chatId)
	warnrc.WarnLimit = warnLimit
	err := db.DB.Save(warnrc).Error
	if err != nil {
		log.Errorf("[Database] SetWarnLimit: %v", err)
		return err
	}
	// Invalidate cache after successful update
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	return nil
}

// SetWarnMode updates the action to take when users reach the warning limit.
// Common modes include "mute", "kick", "ban".
func SetWarnMode(chatId int64, warnMode string) error {
	warnrc := checkWarnSettings(chatId)
	warnrc.WarnMode = warnMode
	err := db.DB.Save(warnrc).Error
	if err != nil {
		log.Errorf("[Database] SetWarnMode: %v", err)
		return err
	}
	// Invalidate cache after successful update
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	return nil
}

// GetWarnSetting returns the warning settings for the specified chat.
// This is the public interface to access warning configuration.
func GetWarnSetting(chatId int64) *models.WarnSettings {
	cached, err := cache.GetFromCacheOrLoad(
		cache.CacheKey("warn_settings", chatId),
		cache.CacheTTLLanguage,
		func() (*models.WarnSettings, error) {
			return checkWarnSettings(chatId), nil
		},
	)
	if err != nil {
		return checkWarnSettings(chatId)
	}
	return cached
}

// GetAllChatWarns returns the total count of warned users in a specific chat.
// Used for administrative statistics and monitoring.
func GetAllChatWarns(chatId int64) int {
	var count int64
	err := db.DB.Model(&models.Warns{}).Where("chat_id = ?", chatId).Count(&count).Error
	if err != nil {
		log.Errorf("[Database] GetAllChatWarns: %v", err)
		return 0
	}
	return int(count)
}

// ResetAllChatWarns removes all warning records for all users in a specific chat.
// Returns true if the operation was successful, false on error.
func ResetAllChatWarns(chatId int64) bool {
	// Collect user IDs before deletion so we can invalidate per-user caches
	var userIds []int64
	if err := db.DB.Model(&models.Warns{}).Where("chat_id = ?", chatId).Pluck("user_id", &userIds).Error; err != nil {
		log.Errorf("[Database] ResetAllChatWarns: %v", err)
		return false
	}

	err := db.DB.Where("chat_id = ?", chatId).Delete(&models.Warns{}).Error
	if err != nil {
		log.Errorf("[Database] ResetAllChatWarns: %v", err)
		return false
	}
	for _, userId := range userIds {
		cache.DeleteCache(cache.CacheKey("warns", userId, chatId))
	}
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	return true
}
