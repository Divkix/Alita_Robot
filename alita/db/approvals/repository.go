package approvals

import (
	"context"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
)

// AddApprovedUser adds a user to the approved list for a chat.
// Approved users are immune from anti-spam measures.
func AddApprovedUser(chatID, userID, approvedBy int64, reason string) error {
	approval := &models.ApprovedUsers{
		ChatID:     chatID,
		UserID:     userID,
		ApprovedBy: approvedBy,
		Reason:     reason,
	}

	err := db.CreateRecord(approval)
	if err != nil {
		log.Errorf("[Database] AddApprovedUser: %v - chat:%d user:%d", err, chatID, userID)
		return err
	}

	cache.DeleteCache(cache.CacheKey("approvals", chatID))
	return nil
}

// IsUserApproved checks if a user is in the approved list for a chat.
func IsUserApproved(chatID, userID int64) bool {
	var user models.ApprovedUsers
	err := db.GetRecord(&user, bson.M{"chat_id": chatID, "user_id": userID})
	return err == nil
}

// GetApprovedUsers returns all approved users for a chat.
func GetApprovedUsers(chatID int64) []*models.ApprovedUsers {
	cacheKey := cache.CacheKey("approvals", chatID)
	result, err := cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLApprovals, func() ([]*models.ApprovedUsers, error) {
		var users []*models.ApprovedUsers
		err := db.GetRecords(&users, bson.M{"chat_id": chatID})
		if err != nil {
			log.Errorf("[Database] GetApprovedUsers: %v - chat:%d", err, chatID)
			return nil, err
		}
		return users, nil
	})
	if err != nil {
		return nil
	}
	return result
}

// RemoveApprovedUser removes a user from the approved list for a chat.
func RemoveApprovedUser(chatID, userID int64) error {
	collection := db.DB.Collection("approved_users")
	_, err := collection.DeleteOne(context.Background(), bson.M{"chat_id": chatID, "user_id": userID})
	if err != nil {
		log.Errorf("[Database] RemoveApprovedUser: %v - chat:%d user:%d", err, chatID, userID)
		return err
	}

	cache.DeleteCache(cache.CacheKey("approvals", chatID))
	return nil
}

// RemoveAllApprovedUsers removes all approved users for a chat.
func RemoveAllApprovedUsers(chatID int64) error {
	collection := db.DB.Collection("approved_users")
	_, err := collection.DeleteMany(context.Background(), bson.M{"chat_id": chatID})
	if err != nil {
		log.Errorf("[Database] RemoveAllApprovedUsers: %v - chat:%d", err, chatID)
		return err
	}

	cache.DeleteCache(cache.CacheKey("approvals", chatID))
	return nil
}
