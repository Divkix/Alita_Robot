package reports

import (
	"context"
	"errors"
	"slices"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/user"
)

func checkChatReportSettings(chatID int64) (*models.ReportChatSettings, error) {
	reportsrc := &models.ReportChatSettings{}
	err := db.GetRecord(reportsrc, models.ReportChatSettings{ChatId: chatID})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := chats.EnsureChatInDb(chatID, ""); err != nil {
			log.Errorf("[Database] GetChatReportSettings: Failed to ensure chat exists for %d: %v", chatID, err)
			return &models.ReportChatSettings{ChatId: chatID, Enabled: true, Status: true}, err
		}

		reportsrc = &models.ReportChatSettings{ChatId: chatID, Enabled: true, Status: true}
		if err := db.CreateRecord(reportsrc); err != nil {
			log.Error(err)
			return reportsrc, err
		}
		return reportsrc, nil
	}
	if err != nil {
		log.Error(err)
		return &models.ReportChatSettings{ChatId: chatID, Enabled: true, Status: true}, err
	}
	return reportsrc, nil
}

func GetChatReportSettings(chatID int64) *models.ReportChatSettings {
	reportsrc, err := cache.GetFromCacheOrLoad(
		context.Background(),
		cache.CacheKey("report_settings", chatID),
		cache.CacheTTLReportSettings,
		func(context.Context) (*models.ReportChatSettings, error) {
			return checkChatReportSettings(chatID)
		},
	)
	if err != nil || reportsrc == nil {
		reportsrc, _ = checkChatReportSettings(chatID)
	}
	return reportsrc
}

func SetChatReportStatus(chatID int64, pref bool) error {
	if _, err := checkChatReportSettings(chatID); err != nil {
		log.Errorf("[Database] SetChatReportStatus: ensure settings for %d: %v", chatID, err)
	}
	err := db.UpdateRecordWithZeroValues(&models.ReportChatSettings{}, models.ReportChatSettings{ChatId: chatID}, map[string]any{
		"enabled": pref,
		"status":  pref,
	})
	if err != nil {
		log.Errorf("[Database] SetChatReportStatus: %v", err)
		return err
	}
	cache.DeleteCache(cache.CacheKey("report_settings", chatID))
	return nil
}

func BlockReportUser(chatId, userId int64) error {
	err := updateReportBlockList(chatId, userId, true)
	if err != nil {
		log.Errorf("[Database] BlockReportUser: %v", err)
	}
	return err
}

func UnblockReportUser(chatId, userId int64) error {
	err := updateReportBlockList(chatId, userId, false)
	if err != nil {
		log.Errorf("[Database] UnblockReportUser: %v", err)
	}
	return err
}

func updateReportBlockList(chatId, userId int64, block bool) error {
	if _, err := checkChatReportSettings(chatId); err != nil {
		log.Errorf("[Database] updateReportBlockList: ensure settings for %d: %v", chatId, err)
	}

	err := db.DB.Transaction(func(tx *gorm.DB) error {
		var chat models.Chat
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").
			Where("chat_id = ?", chatId).
			Take(&chat).Error; err != nil {
			return err
		}

		var settings models.ReportChatSettings
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("chat_id = ?", chatId).
			Take(&settings).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			settings = models.ReportChatSettings{ChatId: chatId, Enabled: true, Status: true}
			if err := tx.Create(&settings).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}

		if block {
			if slices.Contains(settings.BlockedList, userId) {
				return nil
			}
			settings.BlockedList = append(settings.BlockedList, userId)
		} else {
			settings.BlockedList = slices.DeleteFunc(settings.BlockedList, func(blockedId int64) bool {
				return blockedId == userId
			})
		}

		return tx.Model(&settings).Update("blocked_list", settings.BlockedList).Error
	})
	if err != nil {
		return err
	}
	cache.DeleteCache(cache.CacheKey("report_settings", chatId))
	return nil
}

// GetUserReportStatuses returns each user's report-notification setting in one
// query. Users without a row default to enabled, matching GetUserReportSettings.
func GetUserReportStatuses(userIDs []int64) map[int64]bool {
	statuses := make(map[int64]bool, len(userIDs))
	for _, userID := range userIDs {
		statuses[userID] = true
	}
	if len(userIDs) == 0 {
		return statuses
	}
	var rows []models.ReportUserSettings
	if err := db.DB.Model(&models.ReportUserSettings{}).
		Select("user_id", "status").
		Where("user_id IN ?", userIDs).
		Find(&rows).Error; err != nil {
		log.Errorf("[Database] GetUserReportStatuses: %v", err)
		return statuses
	}
	for i := range rows {
		statuses[rows[i].UserId] = rows[i].Status
	}
	return statuses
}

func GetUserReportSettings(userId int64) (reportsrc *models.ReportUserSettings) {
	reportsrc = &models.ReportUserSettings{}
	err := db.GetRecord(reportsrc, models.ReportUserSettings{UserId: userId})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := user.EnsureUserInDb(userId, "", ""); err != nil {
			log.Errorf("[Database] GetUserReportSettings: Failed to ensure user exists for %d: %v", userId, err)
			return &models.ReportUserSettings{UserId: userId, Enabled: true, Status: true}
		}

		reportsrc = &models.ReportUserSettings{UserId: userId, Enabled: true, Status: true}
		err := db.CreateRecord(reportsrc)
		if err != nil {
			log.Error(err)
		}
	} else if err != nil {
		reportsrc = &models.ReportUserSettings{UserId: userId, Enabled: true, Status: true}
		log.Error(err)
	}

	return
}

func SetUserReportSettings(userID int64, pref bool) error {
	GetUserReportSettings(userID)
	err := db.UpdateRecordWithZeroValues(&models.ReportUserSettings{}, models.ReportUserSettings{UserId: userID}, map[string]any{
		"enabled": pref,
		"status":  pref,
	})
	if err != nil {
		log.Errorf("[Database] SetUserReportSettings: %v", err)
	}
	return err
}

func LoadReportStats() (uRCount, gRCount int64) {
	err := db.DB.Model(&models.ReportUserSettings{}).Where("enabled = ?", true).Count(&uRCount).Error
	if err != nil {
		log.Errorf("[Database] LoadReportStats (users): %v", err)
	}

	err = db.DB.Model(&models.ReportChatSettings{}).Where("enabled = ?", true).Count(&gRCount).Error
	if err != nil {
		log.Errorf("[Database] LoadReportStats (chats): %v", err)
	}

	return
}
