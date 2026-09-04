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

func checkWarnSettings(chatID int64) (warnrc *models.WarnSettings) {
	defaultWarnSettings := &models.WarnSettings{ChatId: chatID, WarnLimit: 3, WarnMode: "mute"}
	warnrc = &models.WarnSettings{}
	err := db.DB.Where("chat_id = ?", chatID).First(warnrc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if !db.ChatExists(chatID) {
			log.Warnf("[Database][checkWarnSettings]: Chat %d doesn't exist, returning default settings", chatID)
			return defaultWarnSettings
		}

		warnrc = defaultWarnSettings
		result := db.DB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "chat_id"}},
			DoNothing: true,
		}).Create(warnrc)
		if result.Error != nil {
			log.Errorf("[Database] checkWarnSettings: %v", result.Error)
		} else if result.RowsAffected == 0 {
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

func checkWarns(userId, chatId int64) (warnrc *models.Warns) {
	defaultWarnSrc := &models.Warns{UserId: userId, ChatId: chatId, NumWarns: 0, Reasons: make(models.StringArray, 0)}
	warnrc = &models.Warns{}
	err := db.DB.Where("user_id = ? AND chat_id = ?", userId, chatId).First(warnrc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if !db.ChatExists(chatId) {
			log.Warnf("[Database][checkWarns]: Chat %d doesn't exist, returning default settings", chatId)
			return defaultWarnSrc
		}

		warnrc = defaultWarnSrc
		result := db.DB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "chat_id"}},
			DoNothing: true,
		}).Create(warnrc)
		if result.Error != nil {
			log.Errorf("[Database] checkWarns: %v", result.Error)
		} else if result.RowsAffected == 0 {
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

	cache.DeleteCache(cache.CacheKey("warns", userId, chatId))
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))

	return numWarns, reasons, nil
}

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

	if removed {
		cache.DeleteCache(cache.CacheKey("warns", userId, chatId))
		cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	}

	return removed, nil
}

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

func SetWarnLimit(chatId int64, warnLimit int) error {
	warnrc := checkWarnSettings(chatId)
	warnrc.WarnLimit = warnLimit
	err := db.DB.Save(warnrc).Error
	if err != nil {
		log.Errorf("[Database] SetWarnLimit: %v", err)
		return err
	}
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	return nil
}

func SetWarnMode(chatId int64, warnMode string) error {
	warnrc := checkWarnSettings(chatId)
	warnrc.WarnMode = warnMode
	err := db.DB.Save(warnrc).Error
	if err != nil {
		log.Errorf("[Database] SetWarnMode: %v", err)
		return err
	}
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	return nil
}

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

func GetAllChatWarns(chatId int64) int {
	var count int64
	err := db.DB.Model(&models.Warns{}).Where("chat_id = ?", chatId).Count(&count).Error
	if err != nil {
		log.Errorf("[Database] GetAllChatWarns: %v", err)
		return 0
	}
	return int(count)
}

func ResetAllChatWarns(chatId int64) error {
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
