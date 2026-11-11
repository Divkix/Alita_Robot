package pins

import (
	"context"
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func checkPinData(chatID int64) (*models.PinSettings, error) {
	pinrc := &models.PinSettings{}
	err := db.GetRecord(pinrc, models.PinSettings{ChatId: chatID})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &models.PinSettings{ChatId: chatID}, nil
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
	return setPinSetting(chatID, "clean_linked", pref)
}

func SetAntiChannelPin(chatID int64, pref bool) error {
	return setPinSetting(chatID, "anti_channel_pin", pref)
}

func setPinSetting(chatID int64, field string, value bool) error {
	err := db.DB.Model(&models.PinSettings{}).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns([]string{field}),
	}).Create(map[string]any{"chat_id": chatID, field: value}).Error
	if err != nil {
		return err
	}
	cache.DeleteCache(cache.CacheKey("pins", chatID))
	return nil
}

func LoadPinStats() (acCount, clCount int64) {
	acCount = db.CountRows(&models.PinSettings{}, "anti_channel_pin = ?", true)
	clCount = db.CountRows(&models.PinSettings{}, "clean_linked = ?", true)

	log.Infof("[Database] LoadPinStats: AntiChannelPin=%d, CleanLinked=%d", acCount, clCount)
	return acCount, clCount
}
