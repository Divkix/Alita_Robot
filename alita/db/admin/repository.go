package admin

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
		return &models.AdminSettings{ChatId: chatID}, nil
	} else if err != nil {
		log.Errorf("[Database][checkAdminSetting]: %v ", err)
		return &models.AdminSettings{ChatId: chatID, AnonAdmin: false}, err
	}
	return adminSrc, nil
}

func SetAnonAdminMode(chatID int64, val bool) error {
	err := db.DB.Model(&models.AdminSettings{}).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"anon_admin"}),
	}).Create(map[string]any{"chat_id": chatID, "anon_admin": val}).Error
	if err != nil {
		log.Errorf("[Database] SetAnonAdminMode: %v - %d", err, chatID)
		return err
	}
	cache.DeleteCache(cache.CacheKey("admin", chatID))
	return nil
}
