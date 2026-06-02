package connections

import (
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// ToggleAllowConnect enables or disables connection functionality for a chat.
func ToggleAllowConnect(chatID int64, pref bool) {
	GetChatConnectionSetting(chatID)
	err := db.UpdateRecordWithZeroValues(&models.ConnectionChatSettings{}, models.ConnectionChatSettings{ChatId: chatID}, map[string]any{"allow_connect": pref})
	if err != nil {
		log.Errorf("[Database] ToggleAllowConnect: %d - %v", chatID, err)
	}
}

// GetChatConnectionSetting retrieves connection settings for a chat.
// Creates default settings (disabled) if not found.
func GetChatConnectionSetting(chatID int64) (connectionSrc *models.ConnectionChatSettings) {
	connectionSrc = &models.ConnectionChatSettings{}
	err := db.GetRecord(connectionSrc, models.ConnectionChatSettings{ChatId: chatID})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Ensure chat exists in database before creating settings to satisfy foreign key constraint
		if err := chats.EnsureChatInDb(chatID, ""); err != nil {
			log.Errorf("[Database] GetChatConnectionSetting: Failed to ensure chat exists for %d: %v", chatID, err)
			return &models.ConnectionChatSettings{ChatId: chatID, AllowConnect: false}
		}

		// Create default settings
		connectionSrc = &models.ConnectionChatSettings{ChatId: chatID, AllowConnect: false}
		err := db.CreateRecord(connectionSrc)
		if err != nil {
			log.Errorf("[Database] GetChatConnectionSetting: %d - %v", chatID, err)
		}
	} else if err != nil {
		// Return default on error
		connectionSrc = &models.ConnectionChatSettings{ChatId: chatID, AllowConnect: false}
		log.Errorf("[Database] GetChatConnectionSetting: %d - %v", chatID, err)
	}
	return connectionSrc
}

// getUserConnectionSetting retrieves connection settings for a user.
// Returns default settings (not connected) if not found, without creating a record.
// This avoids violating foreign key constraints when ChatId would be 0.
func getUserConnectionSetting(userID int64) (connectionSrc *models.ConnectionSettings) {
	connectionSrc = &models.ConnectionSettings{}
	err := db.GetRecord(connectionSrc, models.ConnectionSettings{UserId: userID})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Return default settings without creating a record to avoid FK violation with ChatId=0
		connectionSrc = &models.ConnectionSettings{UserId: userID, Connected: false}
	} else if err != nil {
		// Return default on error
		connectionSrc = &models.ConnectionSettings{UserId: userID, Connected: false}
		log.Errorf("[Database] getUserConnectionSetting: %d - %v", userID, err)
	}

	return connectionSrc
}

// Connection returns the connection settings for a user.
// This is a wrapper around getUserConnectionSetting.
func Connection(UserID int64) *models.ConnectionSettings {
	return getUserConnectionSetting(UserID)
}

// ConnectId connects a user to a specific chat.
// Sets the user's connection status to true and associates them with the chat.
// Uses FirstOrCreate to handle both new and existing users.
func ConnectId(UserID, chatID int64) {
	if chatID == 0 {
		log.WithFields(log.Fields{
			"userID": UserID,
			"chatID": chatID,
		}).Warning("[Database] ConnectId: Invalid chatID, skipping connection update")
		return
	}

	err := db.DB.Where("user_id = ?", UserID).Assign(models.ConnectionSettings{Connected: true, ChatId: chatID}).FirstOrCreate(&models.ConnectionSettings{UserId: UserID}).Error
	if err != nil {
		log.Errorf("[Database] ConnectId: %v - %d", err, chatID)
	}
}

// DisconnectId disconnects a user from their current chat connection.
// Sets the user's connection status to false.
// Uses FirstOrCreate to ensure record exists before updating.
func DisconnectId(UserID int64) {
	err := db.DB.Where("user_id = ?", UserID).Assign(map[string]any{"connected": false}).FirstOrCreate(&models.ConnectionSettings{UserId: UserID}).Error
	if err != nil {
		log.Errorf("[Database] DisconnectId: %v - %d", err, UserID)
	}
}

// ReconnectId reconnects a user to their previously connected chat.
// Returns the chat ID the user was reconnected to, or 0 if an error occurs.
// Uses FirstOrCreate to ensure record exists before updating.
func ReconnectId(UserID int64) int64 {
	err := db.DB.Where("user_id = ?", UserID).Assign(models.ConnectionSettings{Connected: true}).FirstOrCreate(&models.ConnectionSettings{UserId: UserID}).Error
	if err != nil {
		log.Errorf("[Database] ReconnectId: %v - %d", err, UserID)
		return 0
	}
	// Reload after update to get fresh data (not stale)
	connectionUpdate := Connection(UserID)
	return connectionUpdate.ChatId
}

// LoadConnectionStats returns statistics about connection usage.
// Returns the count of connected users and chats that allow connections.
func LoadConnectionStats() (connectedUsers, connectedChats int64) {
	// Count chats that allow connections
	err := db.DB.Model(&models.ConnectionChatSettings{}).Where("allow_connect = ?", true).Count(&connectedChats).Error
	if err != nil {
		log.Error(err)
		return
	}

	// Count connected users
	err = db.DB.Model(&models.ConnectionSettings{}).Where("connected = ?", true).Count(&connectedUsers).Error
	if err != nil {
		log.Error(err)
		return
	}

	return
}
