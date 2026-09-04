package pins

import (
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func GetPinData(chatID int64) (pinrc *models.PinSettings) {
	pinrc = &models.PinSettings{}
	err := db.GetRecord(pinrc, models.PinSettings{ChatId: chatID})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		pinrc = &models.PinSettings{ChatId: chatID, MsgId: 0}
		err := db.CreateRecord(pinrc)
		if err != nil {
			log.Errorf("[Database] GetPinData: %v - %d", err, chatID)
		}
	} else if err != nil {
		pinrc = &models.PinSettings{ChatId: chatID, MsgId: 0}
		log.Errorf("[Database] GetPinData: %v - %d", err, chatID)
	}
	log.Infof("[Database] GetPinData: %d", chatID)
	return
}

func SetCleanLinked(chatID int64, pref bool) error {
	GetPinData(chatID)
	err := db.UpdateRecordWithZeroValues(&models.PinSettings{}, models.PinSettings{ChatId: chatID}, map[string]any{"clean_linked": pref})
	if err != nil {
		log.Errorf("[Database] SetCleanLinked: %v", err)
		return err
	}
	return nil
}

func SetAntiChannelPin(chatID int64, pref bool) error {
	GetPinData(chatID)
	err := db.UpdateRecordWithZeroValues(&models.PinSettings{}, models.PinSettings{ChatId: chatID}, map[string]any{"anti_channel_pin": pref})
	if err != nil {
		log.Errorf("[Database] SetAntiChannelPin: %v", err)
		return err
	}
	return nil
}

func LoadPinStats() (acCount, clCount int64) {
	err := db.DB.Model(&models.PinSettings{}).Where("anti_channel_pin = ?", true).Count(&acCount).Error
	if err != nil {
		log.Errorf("[Database] LoadPinStats: Error counting AntiChannelPin: %v", err)
	}

	err = db.DB.Model(&models.PinSettings{}).Where("clean_linked = ?", true).Count(&clCount).Error
	if err != nil {
		log.Errorf("[Database] LoadPinStats: Error counting CleanLinked: %v", err)
	}

	log.Infof("[Database] LoadPinStats: AntiChannelPin=%d, CleanLinked=%d", acCount, clCount)
	return acCount, clCount
}
