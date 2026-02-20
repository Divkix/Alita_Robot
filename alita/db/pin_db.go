package db

import (
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// GetPinData retrieves or creates default pin settings for the specified chat ID.
// Returns default settings with message ID 0 if no settings exist or an error occurs.
func GetPinData(chatID int64) (pinrc *PinSettings) {
	pinrc = &PinSettings{}
	err := GetRecord(pinrc, PinSettings{ChatId: chatID})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create default settings
		pinrc = &PinSettings{ChatId: chatID, MsgId: 0}
		err := CreateRecord(pinrc)
		if err != nil {
			log.Errorf("[Database] GetPinData: %v - %d", err, chatID)
		}
	} else if err != nil {
		// Return default on error
		pinrc = &PinSettings{ChatId: chatID, MsgId: 0}
		log.Errorf("[Database] GetPinData: %v - %d", err, chatID)
	}
	log.Infof("[Database] GetPinData: %d", chatID)
	return
}

// SetCleanLinked updates the clean linked messages preference for the specified chat.
// When enabled, linked channel messages are automatically cleaned from the chat.
func SetCleanLinked(chatID int64, pref bool) {
	err := UpdateRecordWithZeroValues(&PinSettings{}, PinSettings{ChatId: chatID}, map[string]any{"clean_linked": pref})
	if err != nil {
		log.Errorf("[Database] SetCleanLinked: %v", err)
	}
}

// SetAntiChannelPin updates the anti-channel pin preference for the specified chat.
// When enabled, prevents channel messages from being automatically pinned.
func SetAntiChannelPin(chatID int64, pref bool) {
	err := UpdateRecordWithZeroValues(&PinSettings{}, PinSettings{ChatId: chatID}, map[string]any{"anti_channel_pin": pref})
	if err != nil {
		log.Errorf("[Database] SetAntiChannelPin: %v", err)
	}
}

// LoadPinStats returns statistics about pin features across all chats.
// Returns the count of chats with AntiChannelPin enabled and CleanLinked enabled.
func LoadPinStats() (acCount, clCount int64) {
	// Count chats with AntiChannelPin enabled
	err := DB.Model(&PinSettings{}).Where("anti_channel_pin = ?", true).Count(&acCount).Error
	if err != nil {
		log.Errorf("[Database] LoadPinStats: Error counting AntiChannelPin: %v", err)
	}

	// Count chats with CleanLinked enabled
	err = DB.Model(&PinSettings{}).Where("clean_linked = ?", true).Count(&clCount).Error
	if err != nil {
		log.Errorf("[Database] LoadPinStats: Error counting CleanLinked: %v", err)
	}

	log.Infof("[Database] LoadPinStats: AntiChannelPin=%d, CleanLinked=%d", acCount, clCount)
	return acCount, clCount
}
