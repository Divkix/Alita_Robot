package reports

import (
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
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
	settings := GetChatReportSettings(chatId)

	// Check if user is already blocked
	for _, blockedId := range settings.BlockedList {
		if blockedId == userId {
			return nil // User already blocked
		}
	}

	// Add user to blocked list
	settings.BlockedList = append(settings.BlockedList, userId)
	err := db.UpdateRecord(&models.ReportChatSettings{}, models.ReportChatSettings{ChatId: chatId}, models.ReportChatSettings{BlockedList: settings.BlockedList})
	if err != nil {
		log.Errorf("[Database] BlockReportUser: %v", err)
	}
	return err
}

// UnblockReportUser removes a user from the chat's report block list.
// Allows the previously blocked user to send reports again.
func UnblockReportUser(chatId, userId int64) error {
	settings := GetChatReportSettings(chatId)

	// Remove user from blocked list
	var newBlockedList models.Int64Array
	for _, blockedId := range settings.BlockedList {
		if blockedId != userId {
			newBlockedList = append(newBlockedList, blockedId)
		}
	}

	err := db.UpdateRecordWithZeroValues(&models.ReportChatSettings{}, models.ReportChatSettings{ChatId: chatId}, map[string]any{
		"blocked_list": newBlockedList,
	})
	if err != nil {
		log.Errorf("[Database] UnblockReportUser: %v", err)
	}
	return err
}

// GetUserReportSettings retrieves or creates default report settings for the specified user.
// Returns settings with reports enabled by default if no settings exist.
func GetUserReportSettings(userId int64) (reportsrc *models.ReportUserSettings) {
	reportsrc = &models.ReportUserSettings{}
	err := db.GetRecord(reportsrc, models.ReportUserSettings{UserId: userId})
	if errors.Is(err, gorm.ErrRecordNotFound) {
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
