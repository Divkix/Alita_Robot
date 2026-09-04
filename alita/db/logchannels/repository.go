package logchannels

import (
	"errors"
	"strings"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

const cachePrefix = "log_channel"

// Categories are the Rose-compatible log channel category names.
const (
	CategorySettings  = "settings"
	CategoryAdmin     = "admin"
	CategoryUser      = "user"
	CategoryAutomated = "automated"
	CategoryReports   = "reports"
	CategoryOther     = "other"
)

var AllCategories = []string{
	CategorySettings,
	CategoryAdmin,
	CategoryUser,
	CategoryAutomated,
	CategoryReports,
	CategoryOther,
}

func invalidate(chatID int64) {
	cache.DeleteCache(cache.CacheKey(cachePrefix, chatID))
}

func Get(chatID int64) *models.LogChannel {
	result, err := cache.GetFromCacheOrLoad(cache.CacheKey(cachePrefix, chatID), cache.CacheTTLLogChannel, func() (models.LogChannel, error) {
		var row models.LogChannel
		err := db.GetRecord(&row, models.LogChannel{ChatID: chatID})
		if err != nil {
			return models.LogChannel{}, err
		}
		return row, nil
	})
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Errorf("[LogChannels] Get: %v", err)
		}
		return nil
	}
	return &result
}

func Set(chatID int64, chatName string, logChannelID int64) error {
	if err := chats.EnsureChatInDb(chatID, chatName); err != nil {
		return err
	}
	row := models.LogChannel{
		ChatID:       chatID,
		LogChannelID: logChannelID,
		CatSettings:  true,
		CatAdmin:     true,
		CatUser:      true,
		CatAutomated: true,
		CatReports:   true,
		CatOther:     true,
	}
	err := db.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"log_channel_id", "updated_at",
		}),
	}).Create(&row).Error
	if err != nil {
		log.Errorf("[LogChannels] Set: %v", err)
		return err
	}
	invalidate(chatID)
	return nil
}

func Unset(chatID int64) error {
	result := db.DB.Where("chat_id = ?", chatID).Delete(&models.LogChannel{})
	if result.Error != nil {
		log.Errorf("[LogChannels] Unset: %v", result.Error)
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	invalidate(chatID)
	return nil
}

type categoryMeta struct {
	column string
	get    func(*models.LogChannel) bool
}

var logCategories = map[string]categoryMeta{
	CategorySettings:  {column: "cat_settings", get: func(s *models.LogChannel) bool { return s.CatSettings }},
	CategoryAdmin:     {column: "cat_admin", get: func(s *models.LogChannel) bool { return s.CatAdmin }},
	CategoryUser:      {column: "cat_user", get: func(s *models.LogChannel) bool { return s.CatUser }},
	CategoryAutomated: {column: "cat_automated", get: func(s *models.LogChannel) bool { return s.CatAutomated }},
	CategoryReports:   {column: "cat_reports", get: func(s *models.LogChannel) bool { return s.CatReports }},
	CategoryOther:     {column: "cat_other", get: func(s *models.LogChannel) bool { return s.CatOther }},
}

func categoryName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func IsValidCategory(name string) bool {
	_, ok := logCategories[categoryName(name)]
	return ok
}

func categoryColumn(name string) string {
	meta, ok := logCategories[categoryName(name)]
	if !ok {
		return ""
	}
	return meta.column
}

func SetCategory(chatID int64, name string, enabled bool) error {
	col := categoryColumn(name)
	if col == "" {
		return errors.New("unknown log category")
	}
	if Get(chatID) == nil {
		return gorm.ErrRecordNotFound
	}
	err := db.UpdateRecordWithZeroValues(
		&models.LogChannel{},
		models.LogChannel{ChatID: chatID},
		map[string]any{col: enabled},
	)
	if err != nil {
		log.Errorf("[LogChannels] SetCategory: %v", err)
		return err
	}
	invalidate(chatID)
	return nil
}

func CategoryEnabled(settings *models.LogChannel, name string) bool {
	if settings == nil {
		return false
	}
	meta, ok := logCategories[categoryName(name)]
	if !ok {
		return false
	}
	return meta.get(settings)
}
