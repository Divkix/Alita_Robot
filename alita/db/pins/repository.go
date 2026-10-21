package pins

import (
	"context"
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func checkPinData(chatID int64) (*models.PinSettings, error) {
	pinrc := &models.PinSettings{}
	err := db.GetRecord(pinrc, models.PinSettings{ChatId: chatID})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		pinrc = &models.PinSettings{ChatId: chatID, MsgId: 0}
		if err := db.CreateRecord(pinrc); err != nil {
			log.Errorf("[Database] GetPinData: %v - %d", err, chatID)
			return pinrc, err
		}
	} else if err != nil {
		log.Errorf("[Database] GetPinData: %v - %d", err, chatID)
		return &models.PinSettings{ChatId: chatID, MsgId: 0}, err
	}
	return pinrc, nil
}

func GetPinData(chatID int64) *models.PinSettings {
	pinrc, err := cache.GetFromCacheOrLoad(
		context.Background(),
		cache.CacheKey("pins", chatID),
		cache.CacheTTLPins,
		func(context.Context) (*models.PinSettings, error) {
			return checkPinData(chatID)
		},
	)
	if err != nil || pinrc == nil {
		pinrc, _ = checkPinData(chatID)
	}
	return pinrc
}

func SetCleanLinked(chatID int64, pref bool) error {
	if _, err := checkPinData(chatID); err != nil {
		log.Errorf("[Database] SetCleanLinked: ensure settings for %d: %v", chatID, err)
	}
	err := db.UpdateRecordWithZeroValues(&models.PinSettings{}, models.PinSettings{ChatId: chatID}, map[string]any{"clean_linked": pref})
	if err != nil {
		log.Errorf("[Database] SetCleanLinked: %v", err)
		return err
	}
	cache.DeleteCache(cache.CacheKey("pins", chatID))
	return nil
}

func SetAntiChannelPin(chatID int64, pref bool) error {
	if _, err := checkPinData(chatID); err != nil {
		log.Errorf("[Database] SetAntiChannelPin: ensure settings for %d: %v", chatID, err)
	}
	err := db.UpdateRecordWithZeroValues(&models.PinSettings{}, models.PinSettings{ChatId: chatID}, map[string]any{"anti_channel_pin": pref})
	if err != nil {
		log.Errorf("[Database] SetAntiChannelPin: %v", err)
		return err
	}
	cache.DeleteCache(cache.CacheKey("pins", chatID))
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
