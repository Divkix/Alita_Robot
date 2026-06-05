package pins

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// GetPinData retrieves or creates default pin settings for the specified chat ID.
func GetPinData(chatID int64) (pinrc *models.PinSettings) {
	pinrc = &models.PinSettings{}
	err := db.GetRecord(pinrc, bson.M{"chat_id": chatID})
	if err == db.ErrRecordNotFound {
		// Create default settings
		pinrc = &models.PinSettings{ChatId: chatID, MsgId: 0, CreatedAt: time.Now(), UpdatedAt: time.Now()}
		err := db.CreateRecord(pinrc)
		if err != nil {
			log.Errorf("[Database] GetPinData: %v - %d", err, chatID)
		}
	} else if err != nil {
		// Return default on error
		pinrc = &models.PinSettings{ChatId: chatID, MsgId: 0}
		log.Errorf("[Database] GetPinData: %v - %d", err, chatID)
	}
	log.Infof("[Database] GetPinData: %d", chatID)
	return
}

// SetCleanLinked updates the clean linked messages preference for the specified chat.
func SetCleanLinked(chatID int64, pref bool) error {
	GetPinData(chatID)
	err := db.UpdateRecordWithZeroValues(&models.PinSettings{}, bson.M{"chat_id": chatID}, map[string]any{"clean_linked": pref, "updated_at": time.Now()})
	if err != nil {
		log.Errorf("[Database] SetCleanLinked: %v", err)
		return err
	}
	return nil
}

// SetAntiChannelPin updates the anti-channel pin preference for the specified chat.
func SetAntiChannelPin(chatID int64, pref bool) error {
	GetPinData(chatID)
	err := db.UpdateRecordWithZeroValues(&models.PinSettings{}, bson.M{"chat_id": chatID}, map[string]any{"anti_channel_pin": pref, "updated_at": time.Now()})
	if err != nil {
		log.Errorf("[Database] SetAntiChannelPin: %v", err)
		return err
	}
	return nil
}

// LoadPinStats returns statistics about pin features across all chats.
func LoadPinStats() (acCount, clCount int64) {
	collection := db.DB.Collection("pins")

	// Count chats with AntiChannelPin enabled
	var err error
	acCount, err = collection.CountDocuments(context.Background(), bson.M{"anti_channel_pin": true})
	if err != nil {
		log.Errorf("[Database] LoadPinStats: Error counting AntiChannelPin: %v", err)
	}

	// Count chats with CleanLinked enabled
	clCount, err = collection.CountDocuments(context.Background(), bson.M{"clean_linked": true})
	if err != nil {
		log.Errorf("[Database] LoadPinStats: Error counting CleanLinked: %v", err)
	}

	log.Infof("[Database] LoadPinStats: AntiChannelPin=%d, CleanLinked=%d", acCount, clCount)
	return acCount, clCount
}
