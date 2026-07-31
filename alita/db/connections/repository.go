package connections

import (
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/user"
)

// ToggleAllowConnect enables or disables connection functionality for a chat.
func ToggleAllowConnect(chatID int64, pref bool) error {
	GetChatConnectionSetting(chatID)
	err := db.UpdateRecordWithZeroValues(&models.ConnectionChatSettings{}, models.ConnectionChatSettings{ChatId: chatID}, map[string]any{"allow_connect": pref})
	if err != nil {
		log.Errorf("[Database] ToggleAllowConnect: %d - %v", chatID, err)
	}
	return err
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
// The user_id uniqueness constraint makes this a single atomic write.
func ConnectId(UserID, chatID int64) error {
	if chatID == 0 {
		err := fmt.Errorf("invalid chat ID %d", chatID)
		log.WithField("userID", UserID).Warningf("[Database] ConnectId: %v", err)
		return err
	}
	if err := chats.EnsureChatInDb(chatID, ""); err != nil {
		return err
	}
	if err := user.EnsureUserInDb(UserID, "", ""); err != nil {
		return err
	}

	connection := &models.ConnectionSettings{UserId: UserID, ChatId: chatID, Connected: true}
	err := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"chat_id", "connected", "updated_at"}),
	}).Create(connection).Error
	if err != nil {
		log.Errorf("[Database] ConnectId: %v - %d", err, chatID)
	}
	return err
}

// DisconnectId disconnects a user from their current chat connection.
// It deliberately retains chat_id so ReconnectId can restore the connection.
func DisconnectId(UserID int64) error {
	err := db.DB.Model(&models.ConnectionSettings{}).
		Where("user_id = ?", UserID).
		Update("connected", false).Error
	if err != nil {
		log.Errorf("[Database] DisconnectId: %v - %d", err, UserID)
	}
	return err
}

// ReconnectId reconnects a user to their previously connected chat.
// Returns the chat ID the user was reconnected to, or 0 if an error occurs.
func ReconnectId(UserID int64) int64 {
	result := db.DB.Model(&models.ConnectionSettings{}).
		Where("user_id = ?", UserID).
		Update("connected", true)
	if result.Error != nil {
		log.Errorf("[Database] ReconnectId: %v - %d", result.Error, UserID)
		return 0
	}
	if result.RowsAffected == 0 {
		return 0
	}

	connectionUpdate := Connection(UserID)
	if connectionUpdate.ChatId == 0 {
		return 0
	}
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
