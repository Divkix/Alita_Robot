package admin

import (
	"context"
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func GetAdminSettings(chatID int64) *models.AdminSettings {
	settings, err := cache.GetFromCacheOrLoad(
		context.Background(),
		cache.CacheKey("admin", chatID),
		cache.CacheTTLAdminSettings,
		func(context.Context) (*models.AdminSettings, error) {
			return checkAdminSetting(chatID)
		},
	)
	if err != nil || settings == nil {
		settings, _ = checkAdminSetting(chatID)
	}
	return settings
}

func checkAdminSetting(chatID int64) (*models.AdminSettings, error) {
	adminSrc := &models.AdminSettings{}

	err := db.GetRecord(adminSrc, models.AdminSettings{ChatId: chatID})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		adminSrc = &models.AdminSettings{ChatId: chatID, AnonAdmin: false}
		if err := db.CreateRecord(adminSrc); err != nil {
			log.Errorf("[Database][checkAdminSetting]: %v ", err)
			return adminSrc, err
		}
	} else if err != nil {
		log.Errorf("[Database][checkAdminSetting]: %v ", err)
		return &models.AdminSettings{ChatId: chatID, AnonAdmin: false}, err
	}
	return adminSrc, nil
}

func SetAnonAdminMode(chatID int64, val bool) error {
	if _, err := checkAdminSetting(chatID); err != nil {
		log.Errorf("[Database] SetAnonAdminMode: ensure settings for %d: %v", chatID, err)
	}

	err := db.UpdateRecordWithZeroValues(&models.AdminSettings{}, models.AdminSettings{ChatId: chatID}, map[string]any{"anon_admin": val})
	if err != nil {
		log.Errorf("[Database] SetAnonAdminMode: %v - %d", err, chatID)
		return err
	}
	cache.DeleteCache(cache.CacheKey("admin", chatID))
	return nil
}
