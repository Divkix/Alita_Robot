package reports

import (
	"context"
	"errors"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// GetChatReportSettings retrieves or creates default report settings for the specified chat.
func GetChatReportSettings(chatID int64) (reportsrc *models.ReportChatSettings) {
	reportsrc = &models.ReportChatSettings{}
	err := db.GetRecord(reportsrc, bson.M{"chat_id": chatID})
	if errors.Is(err, db.ErrRecordNotFound) {
		// Ensure chat exists in database before creating settings
		if err := chats.EnsureChatInDb(chatID, ""); err != nil {
			log.Errorf("[Database] GetChatReportSettings: Failed to ensure chat exists for %d: %v", chatID, err)
			return &models.ReportChatSettings{ChatId: chatID, Enabled: true, Status: true}
		}

		// Create default settings
		reportsrc = &models.ReportChatSettings{ChatId: chatID, Enabled: true, Status: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
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
func SetChatReportStatus(chatID int64, pref bool) error {
	GetChatReportSettings(chatID)
	err := db.UpdateRecordWithZeroValues(&models.ReportChatSettings{}, bson.M{"chat_id": chatID}, map[string]any{
		"enabled":    pref,
		"status":     pref,
		"updated_at": time.Now(),
	})
	if err != nil {
		log.Errorf("[Database] SetChatReportStatus: %v", err)
	}
	return err
}

// BlockReportUser adds a user to the chat's report block list.
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
	err := db.UpdateRecord(&models.ReportChatSettings{}, bson.M{"chat_id": chatId}, bson.M{"blocked_list": settings.BlockedList, "updated_at": time.Now()})
	if err != nil {
		log.Errorf("[Database] BlockReportUser: %v", err)
	}
	return err
}

// UnblockReportUser removes a user from the chat's report block list.
func UnblockReportUser(chatId, userId int64) error {
	settings := GetChatReportSettings(chatId)

	// Remove user from blocked list
	var newBlockedList models.Int64Array
	for _, blockedId := range settings.BlockedList {
		if blockedId != userId {
			newBlockedList = append(newBlockedList, blockedId)
		}
	}

	err := db.UpdateRecordWithZeroValues(&models.ReportChatSettings{}, bson.M{"chat_id": chatId}, map[string]any{
		"blocked_list": newBlockedList,
		"updated_at":   time.Now(),
	})
	if err != nil {
		log.Errorf("[Database] UnblockReportUser: %v", err)
	}
	return err
}

// GetUserReportSettings retrieves or creates default report settings for the specified user.
func GetUserReportSettings(userId int64) (reportsrc *models.ReportUserSettings) {
	reportsrc = &models.ReportUserSettings{}
	err := db.GetRecord(reportsrc, bson.M{"user_id": userId})
	if errors.Is(err, db.ErrRecordNotFound) {
		// Create default settings
		reportsrc = &models.ReportUserSettings{UserId: userId, Enabled: true, Status: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
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
func SetUserReportSettings(userID int64, pref bool) error {
	GetUserReportSettings(userID)
	err := db.UpdateRecordWithZeroValues(&models.ReportUserSettings{}, bson.M{"user_id": userID}, map[string]any{
		"enabled":    pref,
		"status":     pref,
		"updated_at": time.Now(),
	})
	if err != nil {
		log.Errorf("[Database] SetUserReportSettings: %v", err)
	}
	return err
}

// LoadReportStats returns statistics about report features across the system.
func LoadReportStats() (uRCount, gRCount int64) {
	collectionUsers := db.DB.Collection("report_user_settings")
	uRCount, _ = collectionUsers.CountDocuments(context.Background(), bson.M{"enabled": true})

	collectionChats := db.DB.Collection("report_chat_settings")
	gRCount, _ = collectionChats.CountDocuments(context.Background(), bson.M{"enabled": true})

	return
}
