package reports

import (
	"errors"
	"slices"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/user"
)

// GetChatReportSettings retrieves or creates default report settings for the specified chat.
// Returns settings with reports enabled by default if no settings exist.
func GetChatReportSettings(chatID int64) (reportsrc *models.ReportChatSettings) {
	reportsrc = &models.ReportChatSettings{}
	err := db.GetRecord(reportsrc, models.ReportChatSettings{ChatId: chatID})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Ensure chat exists in database before creating settings to satisfy foreign key constraint
		if err := chats.EnsureChatInDb(chatID, ""); err != nil {
			log.Errorf("[Database] GetChatReportSettings: Failed to ensure chat exists for %d: %v", chatID, err)
			return &models.ReportChatSettings{ChatId: chatID, Enabled: true, Status: true}
		}

		// Create default settings
		reportsrc = &models.ReportChatSettings{ChatId: chatID, Enabled: true, Status: true}
		err := db.CreateRecord(reportsrc)
		if err != nil {
			log.Error(err)
		}
	} else if err != nil {
		// Return default on error
		reportsrc = &models.ReportChatSettings{ChatId: chatID, Enabled: true, Status: true}
		log.Error(err)
	}
	return
}

// SetChatReportStatus updates the report feature status for the specified chat.
// When disabled, users cannot report messages in this chat.
func SetChatReportStatus(chatID int64, pref bool) error {
	GetChatReportSettings(chatID)
	err := db.UpdateRecordWithZeroValues(&models.ReportChatSettings{}, models.ReportChatSettings{ChatId: chatID}, map[string]any{
		"enabled": pref,
		"status":  pref,
	})
	if err != nil {
		log.Errorf("[Database] SetChatReportStatus: %v", err)
	}
	return err
}

// BlockReportUser adds a user to the chat's report block list.
// Blocked users cannot send reports in the specified chat.
// Does nothing if the user is already blocked.
func BlockReportUser(chatId, userId int64) error {
	err := updateReportBlockList(chatId, userId, true)
	if err != nil {
		log.Errorf("[Database] BlockReportUser: %v", err)
	}
	return err
}

// UnblockReportUser removes a user from the chat's report block list.
// Allows the previously blocked user to send reports again.
func UnblockReportUser(chatId, userId int64) error {
	err := updateReportBlockList(chatId, userId, false)
	if err != nil {
		log.Errorf("[Database] UnblockReportUser: %v", err)
	}
	return err
}

func updateReportBlockList(chatId, userId int64, block bool) error {
	// Ensure both parent and settings rows exist before taking locks. The
	// transaction then serializes every list mutation for this chat.
	GetChatReportSettings(chatId)

	return db.DB.Transaction(func(tx *gorm.DB) error {
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
}

// GetUserReportSettings retrieves or creates default report settings for the specified user.
// Returns settings with reports enabled by default if no settings exist.
func GetUserReportSettings(userId int64) (reportsrc *models.ReportUserSettings) {
	reportsrc = &models.ReportUserSettings{}
	err := db.GetRecord(reportsrc, models.ReportUserSettings{UserId: userId})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := user.EnsureUserInDb(userId, "", ""); err != nil {
			log.Errorf("[Database] GetUserReportSettings: Failed to ensure user exists for %d: %v", userId, err)
			return &models.ReportUserSettings{UserId: userId, Enabled: true, Status: true}
		}

		// Create default settings
		reportsrc = &models.ReportUserSettings{UserId: userId, Enabled: true, Status: true}
		err := db.CreateRecord(reportsrc)
		if err != nil {
			log.Error(err)
		}
	} else if err != nil {
		// Return default on error
		reportsrc = &models.ReportUserSettings{UserId: userId, Enabled: true, Status: true}
		log.Error(err)
	}

	return
}

// SetUserReportSettings updates the global report preference for the specified user.
// When disabled, the user won't receive any report notifications.
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

// LoadReportStats returns statistics about report features across the system.
// Returns the count of users and chats with reports enabled.
func LoadReportStats() (uRCount, gRCount int64) {
	// Count users with reports enabled
	err := db.DB.Model(&models.ReportUserSettings{}).Where("enabled = ?", true).Count(&uRCount).Error
	if err != nil {
		log.Errorf("[Database] LoadReportStats (users): %v", err)
	}

	// Count chats with reports enabled
	err = db.DB.Model(&models.ReportChatSettings{}).Where("enabled = ?", true).Count(&gRCount).Error
	if err != nil {
		log.Errorf("[Database] LoadReportStats (chats): %v", err)
	}

	return
}
