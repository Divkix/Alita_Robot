package db

import (
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// GetChatReportSettings retrieves or creates default report settings for the specified chat.
// Returns settings with reports enabled by default if no settings exist.
func GetChatReportSettings(chatID int64) (reportsrc *ReportChatSettings) {
	reportsrc = &ReportChatSettings{}
	err := GetRecord(reportsrc, ReportChatSettings{ChatId: chatID})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Ensure chat exists in database before creating settings to satisfy foreign key constraint
		if err := EnsureChatInDb(chatID, ""); err != nil {
			log.Errorf("[Database] GetChatReportSettings: Failed to ensure chat exists for %d: %v", chatID, err)
			return &ReportChatSettings{ChatId: chatID, Enabled: true}
		}

		// Create default settings
		reportsrc = &ReportChatSettings{ChatId: chatID, Enabled: true}
		err := CreateRecord(reportsrc)
		if err != nil {
			log.Error(err)
		}
	} else if err != nil {
		// Return default on error
		reportsrc = &ReportChatSettings{ChatId: chatID, Enabled: true}
		log.Error(err)
	}
	return
}

// SetChatReportStatus updates the report feature status for the specified chat.
// When disabled, users cannot report messages in this chat.
func SetChatReportStatus(chatID int64, pref bool) {
	err := UpdateRecordWithZeroValues(&ReportChatSettings{}, ReportChatSettings{ChatId: chatID}, ReportChatSettings{Enabled: pref})
	if err != nil {
		log.Error(err)
	}
}

// BlockReportUser adds a user to the chat's report block list.
// Blocked users cannot send reports in the specified chat.
// Does nothing if the user is already blocked.
func BlockReportUser(chatId, userId int64) {
	settings := GetChatReportSettings(chatId)

	// Check if user is already blocked
	for _, blockedId := range settings.BlockedList {
		if blockedId == userId {
			return // User already blocked
		}
	}

	// Add user to blocked list
	settings.BlockedList = append(settings.BlockedList, userId)
	err := UpdateRecord(&ReportChatSettings{}, ReportChatSettings{ChatId: chatId}, ReportChatSettings{BlockedList: settings.BlockedList})
	if err != nil {
		log.Errorf("[Database] BlockReportUser: %v", err)
	}
}

// UnblockReportUser removes a user from the chat's report block list.
// Allows the previously blocked user to send reports again.
func UnblockReportUser(chatId, userId int64) {
	settings := GetChatReportSettings(chatId)

	// Remove user from blocked list
	var newBlockedList Int64Array
	for _, blockedId := range settings.BlockedList {
		if blockedId != userId {
			newBlockedList = append(newBlockedList, blockedId)
		}
	}

	err := UpdateRecord(&ReportChatSettings{}, ReportChatSettings{ChatId: chatId}, ReportChatSettings{BlockedList: newBlockedList})
	if err != nil {
		log.Errorf("[Database] UnblockReportUser: %v", err)
	}
}

// GetUserReportSettings retrieves or creates default report settings for the specified user.
// Returns settings with reports enabled by default if no settings exist.
func GetUserReportSettings(userId int64) (reportsrc *ReportUserSettings) {
	reportsrc = &ReportUserSettings{}
	err := GetRecord(reportsrc, ReportUserSettings{UserId: userId})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create default settings
		reportsrc = &ReportUserSettings{UserId: userId, Enabled: true}
		err := CreateRecord(reportsrc)
		if err != nil {
			log.Error(err)
		}
	} else if err != nil {
		// Return default on error
		reportsrc = &ReportUserSettings{UserId: userId, Enabled: true}
		log.Error(err)
	}

	return
}

// SetUserReportSettings updates the global report preference for the specified user.
// When disabled, the user won't receive any report notifications.
func SetUserReportSettings(userID int64, pref bool) {
	err := UpdateRecordWithZeroValues(&ReportUserSettings{}, ReportUserSettings{UserId: userID}, ReportUserSettings{Enabled: pref})
	if err != nil {
		log.Error(userID)
	}
}

// LoadReportStats returns statistics about report features across the system.
// Returns the count of users and chats with reports enabled.
func LoadReportStats() (uRCount, gRCount int64) {
	// Count users with reports enabled
	err := DB.Model(&ReportUserSettings{}).Where("enabled = ?", true).Count(&uRCount).Error
	if err != nil {
		log.Errorf("[Database] LoadReportStats (users): %v", err)
	}

	// Count chats with reports enabled
	err = DB.Model(&ReportChatSettings{}).Where("enabled = ?", true).Count(&gRCount).Error
	if err != nil {
		log.Errorf("[Database] LoadReportStats (chats): %v", err)
	}

	return
}
