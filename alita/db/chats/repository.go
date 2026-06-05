package chats

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetChatSettings retrieves chat settings using optimized cached queries.
// Returns an empty Chat struct if not found or on error.
func GetChatSettings(chatId int64) (chatSrc *models.Chat) {
	// Use optimized cached query instead of SELECT *
	chat, err := GetChatBasicInfoCached(chatId)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return &models.Chat{}
		}
		log.Errorf("[Database] GetChatSettings: %v - %d", err, chatId)
		return &models.Chat{}
	}
	return chat
}

// EnsureChatInDb ensures that a chat exists in the database.
// Creates the chat record if it doesn't exist, or updates it if it does.
// This is essential for foreign key constraints that reference the chats table.
func EnsureChatInDb(chatId int64, chatName string) error {
	chatUpdate := bson.M{
		"chat_id":    chatId,
		"chat_name":  chatName,
		"updated_at": time.Now(),
	}

	collection := db.DB.Collection("chats")
	filter := bson.M{"chat_id": chatId}
	update := bson.M{"$set": chatUpdate}
	opts := options.Update().SetUpsert(true)

	_, err := collection.UpdateOne(context.Background(), filter, update, opts)
	if err != nil {
		log.Errorf("[Database] EnsureChatInDb: %v", err)
		return fmt.Errorf("failed to ensure chat %d in database: %w", chatId, err)
	}
	return nil
}

// UpdateChat updates or creates a chat record with the given information.
// Adds user to the chat's user list if not already present, marks chat as active,
// and updates the last activity timestamp to track when messages are received.
// Returns error if database operation fails.
func UpdateChat(chatId int64, chatname string, userid int64) error {
	chatr := GetChatSettings(chatId)
	foundUser := slices.Contains(chatr.Users, userid)
	now := time.Now()

	collection := db.DB.Collection("chats")

	// Always update last_activity to track message activity
	if chatr.ChatName == chatname && foundUser {
		// Only update last_activity and is_inactive
		updates := bson.M{
			"last_activity": now,
			"is_inactive":   false,
			"updated_at":    now,
		}
		_, err := collection.UpdateOne(context.Background(), bson.M{"chat_id": chatId}, bson.M{"$set": updates})
		if err != nil {
			log.Errorf("[Database] UpdateChat (activity only): %d - %v", chatId, err)
			return err
		}
		// Invalidate cache after update
		cache.DeleteCache(cache.CacheKey("chat", chatId))
		return nil
	}

	// Prepare updates for all fields
	updates := bson.M{
		"is_inactive":   false,
		"last_activity": now,
		"updated_at":    now,
	}
	if chatr.ChatName != chatname {
		updates["chat_name"] = chatname
	}
	if !foundUser {
		newUsers := chatr.Users
		newUsers = append(newUsers, userid)
		updates["users"] = newUsers
	}

	if chatr.ChatId == 0 {
		// Create new chat
		newChat := &models.Chat{
			ChatId:       chatId,
			ChatName:     chatname,
			Users:        models.Int64Array{userid},
			IsInactive:   false,
			LastActivity: now,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		_, err := collection.InsertOne(context.Background(), newChat)
		if err != nil {
			log.Errorf("[Database] UpdateChat: %v - %d (%d)", err, chatId, userid)
			return err
		}
	} else {
		// Update existing chat with all changes
		_, err := collection.UpdateOne(context.Background(), bson.M{"chat_id": chatId}, bson.M{"$set": updates})
		if err != nil {
			log.Errorf("[Database] UpdateChat: %v - %d (%d)", err, chatId, userid)
			return err
		}
	}

	// Invalidate cache after update
	cache.DeleteCache(cache.CacheKey("chat", chatId))
	log.Debugf("[Database] UpdateChat: %d", chatId)
	return nil
}

// GetAllChats retrieves all chat records and returns them as a map indexed by chat ID.
// Returns an empty map if an error occurs.
func GetAllChats() map[int64]models.Chat {
	chatMap := make(map[int64]models.Chat)
	collection := db.DB.Collection("chats")

	cursor, err := collection.Find(context.Background(), bson.M{})
	if err != nil {
		log.Errorf("[Database] GetAllChats: %v", err)
		return chatMap
	}
	defer cursor.Close(context.Background())

	for cursor.Next(context.Background()) {
		var chat models.Chat
		if err := cursor.Decode(&chat); err != nil {
			log.Errorf("[Database] GetAllChats decode: %v", err)
			continue
		}
		chatMap[chat.ChatId] = chat
	}

	return chatMap
}

// LoadChatStats returns the count of active and inactive chats.
// Active chats have is_inactive = false, inactive chats have is_inactive = true.
func LoadChatStats() (activeChats, inactiveChats int) {
	collection := db.DB.Collection("chats")

	// Count active chats
	activeCount, err := collection.CountDocuments(context.Background(), bson.M{"is_inactive": false})
	if err != nil {
		log.Errorf("[Database][LoadChatStats] counting active chats: %v", err)
	}

	// Count inactive chats
	inactiveCount, err := collection.CountDocuments(context.Background(), bson.M{"is_inactive": true})
	if err != nil {
		log.Errorf("[Database][LoadChatStats] counting inactive chats: %v", err)
	}

	activeChats = int(activeCount)
	inactiveChats = int(inactiveCount)
	return
}

// LoadActivityStats returns Daily Active Groups, Weekly Active Groups, and Monthly Active Groups.
// These metrics are based on last_activity timestamps within the respective time periods.
func LoadActivityStats() (dag, wag, mag int64) {
	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)
	weekAgo := now.Add(-7 * 24 * time.Hour)
	monthAgo := now.Add(-30 * 24 * time.Hour)

	collection := db.DB.Collection("chats")

	// Count daily active groups
	var err error
	dag, err = collection.CountDocuments(context.Background(), bson.M{
		"is_inactive":   false,
		"last_activity": bson.M{"$gte": dayAgo},
	})
	if err != nil {
		log.Errorf("[Database][LoadActivityStats] counting daily active groups: %v", err)
	}

	// Count weekly active groups
	wag, err = collection.CountDocuments(context.Background(), bson.M{
		"is_inactive":   false,
		"last_activity": bson.M{"$gte": weekAgo},
	})
	if err != nil {
		log.Errorf("[Database][LoadActivityStats] counting weekly active groups: %v", err)
	}

	// Count monthly active groups
	mag, err = collection.CountDocuments(context.Background(), bson.M{
		"is_inactive":   false,
		"last_activity": bson.M{"$gte": monthAgo},
	})
	if err != nil {
		log.Errorf("[Database][LoadActivityStats] counting monthly active groups: %v", err)
	}

	return dag, wag, mag
}
