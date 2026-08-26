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

// AllCategories is the documented default set; all are enabled by default.
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

// Get returns the log-channel binding for a chat, or nil.
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

// Set binds a group chat to a log channel. Categories default to all-on.
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

// Unset removes the log-channel binding.
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

// IsValidCategory reports whether name is a documented log category.
func IsValidCategory(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case CategorySettings, CategoryAdmin, CategoryUser, CategoryAutomated, CategoryReports, CategoryOther:
		return true
	default:
		return false
	}
}

func categoryColumn(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case CategorySettings:
		return "cat_settings"
	case CategoryAdmin:
		return "cat_admin"
	case CategoryUser:
		return "cat_user"
	case CategoryAutomated:
		return "cat_automated"
	case CategoryReports:
		return "cat_reports"
	case CategoryOther:
		return "cat_other"
	default:
		return ""
	}
}

// SetCategory enables or disables one category. The chat must already have a
// log channel configured.
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

// CategoryEnabled reports whether a category is currently logged.
func CategoryEnabled(settings *models.LogChannel, name string) bool {
	if settings == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case CategorySettings:
		return settings.CatSettings
	case CategoryAdmin:
		return settings.CatAdmin
	case CategoryUser:
		return settings.CatUser
	case CategoryAutomated:
		return settings.CatAutomated
	case CategoryReports:
		return settings.CatReports
	case CategoryOther:
		return settings.CatOther
	default:
		return false
	}
}
