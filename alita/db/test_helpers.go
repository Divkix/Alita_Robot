package db

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EnsureUserInDb ensures that a user exists in the database.
// This is a test helper to avoid circular imports between db and user packages.
func EnsureUserInDb(userId int64, username, firstName string) error {
	userUpdate := bson.M{
		"user_id":    userId,
		"username":   username,
		"name":       firstName,
		"updated_at": time.Now(),
	}

	collection := DB.Collection("users")
	filter := bson.M{"user_id": userId}
	update := bson.M{"$set": userUpdate}
	opts := options.Update().SetUpsert(true)

	_, err := collection.UpdateOne(context.Background(), filter, update, opts)
	if err != nil {
		log.Errorf("[Database] EnsureUserInDb: %v", err)
		return fmt.Errorf("failed to ensure user %d in database: %w", userId, err)
	}
	return nil
}

// EnsureChatInDb ensures that a chat exists in the database.
// This is a test helper to avoid circular imports between db and chats packages.
func EnsureChatInDb(chatId int64, chatName string) error {
	chatUpdate := bson.M{
		"chat_id":    chatId,
		"chat_name":  chatName,
		"updated_at": time.Now(),
	}

	collection := DB.Collection("chats")
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
