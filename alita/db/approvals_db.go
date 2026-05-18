package db

import (
	log "github.com/sirupsen/logrus"
)

// AddApprovedUser adds a user to the approved list for a chat.
// Approved users are immune from anti-spam measures.
func AddApprovedUser(chatID, userID, approvedBy int64, reason string) error {
	approval := &ApprovedUsers{
		ChatID:     chatID,
		UserID:     userID,
		ApprovedBy: approvedBy,
		Reason:     reason,
	}

	err := CreateRecord(approval)
	if err != nil {
		log.Errorf("[Database] AddApprovedUser: %v - chat:%d user:%d", err, chatID, userID)
		return err
	}

	deleteCache(CacheKey("approvals", chatID))
	return nil
}

// IsUserApproved checks if a user is in the approved list for a chat.
func IsUserApproved(chatID, userID int64) bool {
	var user ApprovedUsers
	err := GetRecord(&user, ApprovedUsers{ChatID: chatID, UserID: userID})
	return err == nil
}

// GetApprovedUsers returns all approved users for a chat.
func GetApprovedUsers(chatID int64) []*ApprovedUsers {
	cacheKey := CacheKey("approvals", chatID)
	result, err := getFromCacheOrLoad(cacheKey, CacheTTLApprovals, func() ([]*ApprovedUsers, error) {
		var users []*ApprovedUsers
		err := GetRecords(&users, ApprovedUsers{ChatID: chatID})
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
	result := DB.Where("chat_id = ? AND user_id = ?", chatID, userID).Delete(&ApprovedUsers{})
	if result.Error != nil {
		log.Errorf("[Database] RemoveApprovedUser: %v - chat:%d user:%d", result.Error, chatID, userID)
		return result.Error
	}

	deleteCache(CacheKey("approvals", chatID))
	return nil
}

// RemoveAllApprovedUsers removes all approved users for a chat.
func RemoveAllApprovedUsers(chatID int64) error {
	err := DB.Where("chat_id = ?", chatID).Delete(&ApprovedUsers{}).Error
	if err != nil {
		log.Errorf("[Database] RemoveAllApprovedUsers: %v - chat:%d", err, chatID)
		return err
	}

	deleteCache(CacheKey("approvals", chatID))
	return nil
}
