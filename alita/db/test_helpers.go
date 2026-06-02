package db

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

// EnsureUserInDb ensures that a user exists in the database.
// This is a test helper to avoid circular imports between db and user packages.
func EnsureUserInDb(userId int64, username, firstName string) error {
	userUpdate := &User{
		UserId:   userId,
		UserName: username,
		Name:     firstName,
	}
	result := DB.Where("user_id = ?", userId).Assign(userUpdate).FirstOrCreate(&User{})
	if result.Error != nil {
		log.Errorf("[Database] EnsureUserInDb: %v", result.Error)
		return fmt.Errorf("failed to ensure user %d in database: %w", userId, result.Error)
	}
	return nil
}

// EnsureChatInDb ensures that a chat exists in the database.
// This is a test helper to avoid circular imports between db and chats packages.
func EnsureChatInDb(chatId int64, chatName string) error {
	chatUpdate := &Chat{
		ChatId:   chatId,
		ChatName: chatName,
	}
	err := DB.Where("chat_id = ?", chatId).Assign(chatUpdate).FirstOrCreate(&Chat{}).Error
	if err != nil {
		log.Errorf("[Database] EnsureChatInDb: %v", err)
		return fmt.Errorf("failed to ensure chat %d in database: %w", chatId, err)
	}
	return nil
}
