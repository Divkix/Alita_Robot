package approvals

import (
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
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
	for _, u := range GetApprovedUsers(chatID) {
		if u.UserID == userID {
			return true
		}
	}
	return false
}

// GetApprovedUsers returns all approved users for a chat.
func GetApprovedUsers(chatID int64) []*models.ApprovedUsers {
	cacheKey := cache.CacheKey("approvals", chatID)
	result, err := cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLApprovals, func() ([]*models.ApprovedUsers, error) {
		var users []*models.ApprovedUsers
		err := db.GetRecords(&users, models.ApprovedUsers{ChatID: chatID})
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
	result := db.DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&models.ApprovedUsers{})
	if result.Error != nil {
		log.Errorf("[Database] RemoveApprovedUser: %v - chat:%d user:%d", result.Error, chatID, userID)
		return result.Error
	}

	cache.DeleteCache(cache.CacheKey("approvals", chatID))
	return nil
}

// RemoveAllApprovedUsers removes all approved users for a chat.
func RemoveAllApprovedUsers(chatID int64) error {
	err := db.DB.Where("chat_id = ?", chatID).Delete(&models.ApprovedUsers{}).Error
	if err != nil {
		log.Errorf("[Database] RemoveAllApprovedUsers: %v - chat:%d", err, chatID)
		return err
	}

	cache.DeleteCache(cache.CacheKey("approvals", chatID))
	return nil
}
