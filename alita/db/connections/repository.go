package connections

import (
	"context"
	"errors"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// ToggleAllowConnect enables or disables connection functionality for a chat.
func ToggleAllowConnect(chatID int64, pref bool) {
	GetChatConnectionSetting(chatID)
	err := db.UpdateRecordWithZeroValues(&models.ConnectionChatSettings{}, bson.M{"chat_id": chatID}, map[string]any{"allow_connect": pref, "updated_at": time.Now()})
	if err != nil {
		log.Errorf("[Database] ToggleAllowConnect: %d - %v", chatID, err)
	}
}

// GetChatConnectionSetting retrieves connection settings for a chat.
// Creates default settings (disabled) if not found.
func GetChatConnectionSetting(chatID int64) (connectionSrc *models.ConnectionChatSettings) {
	connectionSrc = &models.ConnectionChatSettings{}
	err := db.GetRecord(connectionSrc, bson.M{"chat_id": chatID})
	if errors.Is(err, db.ErrRecordNotFound) {
		// Ensure chat exists in database before creating settings
		if err := chats.EnsureChatInDb(chatID, ""); err != nil {
			log.Errorf("[Database] GetChatConnectionSetting: Failed to ensure chat exists for %d: %v", chatID, err)
			return &models.ConnectionChatSettings{ChatId: chatID, AllowConnect: false}
		}

		// Create default settings
		connectionSrc = &models.ConnectionChatSettings{ChatId: chatID, AllowConnect: false, CreatedAt: time.Now(), UpdatedAt: time.Now()}
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
func getUserConnectionSetting(userID int64) (connectionSrc *models.ConnectionSettings) {
	connectionSrc = &models.ConnectionSettings{}
	err := db.GetRecord(connectionSrc, bson.M{"user_id": userID})
	if errors.Is(err, db.ErrRecordNotFound) {
		// Return default settings without creating a record
		connectionSrc = &models.ConnectionSettings{UserId: userID, Connected: false}
	} else if err != nil {
		// Return default on error
		connectionSrc = &models.ConnectionSettings{UserId: userID, Connected: false}
		log.Errorf("[Database] getUserConnectionSetting: %d - %v", userID, err)
	}

	return connectionSrc
}

// Connection returns the connection settings for a user.
func Connection(UserID int64) *models.ConnectionSettings {
	return getUserConnectionSetting(UserID)
}

// ConnectId connects a user to a specific chat.
func ConnectId(UserID, chatID int64) {
	if chatID == 0 {
		log.WithFields(log.Fields{
			"userID": UserID,
			"chatID": chatID,
		}).Warning("[Database] ConnectId: Invalid chatID, skipping connection update")
		return
	}

	collection := db.DB.Collection("connection")
	filter := bson.M{"user_id": UserID}
	update := bson.M{"$set": bson.M{
		"connected":  true,
		"chat_id":    chatID,
		"updated_at": time.Now(),
	}}
	opts := options.Update().SetUpsert(true)

	_, err := collection.UpdateOne(context.Background(), filter, update, opts)
	if err != nil {
		log.Errorf("[Database] ConnectId: %v - %d", err, chatID)
	}
}

// DisconnectId disconnects a user from their current chat connection.
func DisconnectId(UserID int64) {
	collection := db.DB.Collection("connection")
	filter := bson.M{"user_id": UserID}
	update := bson.M{"$set": bson.M{
		"connected":  false,
		"updated_at": time.Now(),
	}}
	opts := options.Update().SetUpsert(true)

	_, err := collection.UpdateOne(context.Background(), filter, update, opts)
	if err != nil {
		log.Errorf("[Database] DisconnectId: %v - %d", err, UserID)
	}
}

// ReconnectId reconnects a user to their previously connected chat.
func ReconnectId(UserID int64) int64 {
	collection := db.DB.Collection("connection")
	filter := bson.M{"user_id": UserID}
	update := bson.M{"$set": bson.M{
		"connected":  true,
		"updated_at": time.Now(),
	}}
	opts := options.Update().SetUpsert(true)

	_, err := collection.UpdateOne(context.Background(), filter, update, opts)
	if err != nil {
		log.Errorf("[Database] ReconnectId: %v - %d", err, UserID)
		return 0
	}
	// Reload after update to get fresh data
	connectionUpdate := Connection(UserID)
	return connectionUpdate.ChatId
}

// LoadConnectionStats returns statistics about connection usage.
func LoadConnectionStats() (connectedUsers, connectedChats int64) {
	var err error

	// Count chats that allow connections
	collectionChats := db.DB.Collection("connection_settings")
	connectedChats, err = collectionChats.CountDocuments(context.Background(), bson.M{"allow_connect": true})
	if err != nil {
		log.Error(err)
	}

	// Count connected users
	collectionUsers := db.DB.Collection("connection")
	connectedUsers, err = collectionUsers.CountDocuments(context.Background(), bson.M{"connected": true})
	if err != nil {
		log.Error(err)
	}

	return
}
