package warns

import (
	"errors"
	"unicode/utf8"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/user"
	"github.com/divkix/Alita_Robot/alita/i18n"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

		// Use ON CONFLICT DO NOTHING to handle concurrent creation safely
		warnrc = defaultWarnSettings
		result := db.DB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "chat_id"}},
			DoNothing: true,
		}).Create(warnrc)
		if result.Error != nil {
			log.Errorf("[Database] checkWarnSettings: %v", result.Error)
		} else if result.RowsAffected == 0 {
			// Another writer created the row concurrently; reload it
			if err := db.DB.Where("chat_id = ?", chatID).First(warnrc).Error; err != nil {
				log.Errorf("[Database] checkWarnSettings reload: %v", err)
				warnrc = defaultWarnSettings
			}
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

		// Use ON CONFLICT DO NOTHING to handle concurrent creation safely
		warnrc = defaultWarnSrc
		result := db.DB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "chat_id"}},
			DoNothing: true,
		}).Create(warnrc)
		if result.Error != nil {
			log.Errorf("[Database] checkWarns: %v", result.Error)
		} else if result.RowsAffected == 0 {
			// Another writer created the row concurrently; reload it
			if err := db.DB.Where("user_id = ? AND chat_id = ?", userId, chatId).First(warnrc).Error; err != nil {
				log.Errorf("[Database] checkWarns reload: %v", err)
				warnrc = defaultWarnSrc
			}
		}
	} else if err != nil {
		log.Errorf("[Database][checkUserWarns]: %d - %v", userId, err)
		warnrc = defaultWarnSrc
	}
	return
}

// WarnUser adds a warning to a user in a specific chat with an optional reason.
// Returns the total number of warnings, all reasons, and any persistence error.
func WarnUser(userId, chatId int64, reason string) (int, []string, error) {
	var numWarns int
	var reasons []string

	if err := chats.EnsureChatInDb(chatId, ""); err != nil {
		return 0, nil, err
	}
	if err := user.EnsureUserInDb(userId, "", ""); err != nil {
		return 0, nil, err
	}

	err := db.DB.Transaction(func(tx *gorm.DB) error {
		// Lock the parent row first so concurrent first warnings cannot both
		// observe a missing warns_users row.
		// ponytail: this serializes warnings per chat; use per-user advisory
		// locks only if moderation write throughput becomes material.
		var chat models.Chat
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").
			Where("chat_id = ?", chatId).
			Take(&chat).Error; err != nil {
			return err
		}

		warnSettings := &models.WarnSettings{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("chat_id = ?", chatId).
			First(warnSettings).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				warnSettings = &models.WarnSettings{ChatId: chatId, WarnLimit: 3, WarnMode: "mute"}
				if err := tx.Create(warnSettings).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		}

		warnrc := &models.Warns{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND chat_id = ?", userId, chatId).
			First(warnrc).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				warnrc = &models.Warns{
					UserId:  userId,
					ChatId:  chatId,
					Reasons: models.StringArray{},
				}
			} else {
				return err
			}
		}

		warnrc.NumWarns++

		if reason != "" {
			if len(reason) > 3000 {
				reason = reason[:3000]
				for !utf8.ValidString(reason) {
					reason = reason[:len(reason)-1]
				}
			}
			warnrc.Reasons = append(warnrc.Reasons, reason)
		} else {
			tr := i18n.MustNewTranslator("en")
			noReason, _ := tr.GetString("db_warn_no_reason")
			if noReason == "" {
				noReason = "No Reason"
			}
			warnrc.Reasons = append(warnrc.Reasons, noReason)
		}

		if err := tx.Save(warnrc).Error; err != nil {
			return err
		}

		numWarns = warnrc.NumWarns
		reasons = []string(warnrc.Reasons)
		return nil
	})
	if err != nil {
		log.Errorf("[Database] WarnUser: %v", err)
		return 0, nil, err
	}

	// Invalidate cache after successful transaction
	cache.DeleteCache(cache.CacheKey("warns", userId, chatId))
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))

	return numWarns, reasons, nil
}

// RemoveWarn removes the most recent warning from a user in a specific chat.
// Returns whether a warning was removed and any persistence error.
func RemoveWarn(userId, chatId int64) (bool, error) {
	var removed bool

	err := db.DB.Transaction(func(tx *gorm.DB) error {
		var chat models.Chat
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").
			Where("chat_id = ?", chatId).
			Take(&chat).Error; err != nil {
			return err
		}

		warnrc := &models.Warns{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND chat_id = ?", userId, chatId).
			First(warnrc).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				removed = false
				return nil
			}
			return err
		}

		if warnrc.NumWarns > 0 {
			warnrc.NumWarns--
			if len(warnrc.Reasons) > 0 {
				warnrc.Reasons = warnrc.Reasons[:len(warnrc.Reasons)-1]
			}
			removed = true

			if err := tx.Save(warnrc).Error; err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		log.Errorf("[Database] RemoveWarn: %v", err)
		return false, err
	}

	// Invalidate cache after successful transaction
	if removed {
		cache.DeleteCache(cache.CacheKey("warns", userId, chatId))
		cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	}

	return removed, nil
}

// ResetUserWarns removes all warnings for a specific user in a chat.
// Returns whether a row was deleted and any persistence error.
func ResetUserWarns(userId, chatId int64) (bool, error) {
	result := db.DB.Where("user_id = ? AND chat_id = ?", userId, chatId).Delete(&models.Warns{})
	if result.Error != nil {
		log.Errorf("[Database] ResetUserWarns: %v", result.Error)
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	cache.DeleteCache(cache.CacheKey("warns", userId, chatId))
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	return true, nil
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
		cache.CacheTTLWarnSettings,
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
// Caches the value type to avoid double-pointer issues with the generic loader.
func GetWarnSetting(chatId int64) *models.WarnSettings {
	cached, err := cache.GetFromCacheOrLoad(
		cache.CacheKey("warn_settings", chatId),
		cache.CacheTTLWarnSettings,
		func() (models.WarnSettings, error) {
			w := checkWarnSettings(chatId)
			return *w, nil
		},
	)
	if err != nil {
		return checkWarnSettings(chatId)
	}
	return &cached
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
func ResetAllChatWarns(chatId int64) error {
	// Collect user IDs before deletion so we can invalidate per-user caches
	var userIds []int64
	if err := db.DB.Model(&models.Warns{}).Where("chat_id = ?", chatId).Pluck("user_id", &userIds).Error; err != nil {
		log.Errorf("[Database] ResetAllChatWarns: %v", err)
		return err
	}

	err := db.DB.Where("chat_id = ?", chatId).Delete(&models.Warns{}).Error
	if err != nil {
		log.Errorf("[Database] ResetAllChatWarns: %v", err)
		return err
	}
	for _, userId := range userIds {
		cache.DeleteCache(cache.CacheKey("warns", userId, chatId))
	}
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	return nil
}
