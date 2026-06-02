package greetings

import (
	"errors"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	alitaerrors "github.com/divkix/Alita_Robot/alita/utils/errors"
)

// checkGreetingSettings retrieves or creates default greeting settings for a chat.
// Used internally before performing any greeting-related operation.
// Returns default settings if the chat doesn't exist in the database.
func checkGreetingSettings(chatID int64) (greetingSrc *models.GreetingSettings) {
	greetingSrc = &models.GreetingSettings{}
	err := db.GetRecord(greetingSrc, map[string]any{"chat_id": chatID})

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Ensure chat exists before creating greeting settings
		if !db.ChatExists(chatID) {
			// Chat doesn't exist, return default settings without creating record
			log.Warnf("[Database][checkGreetingSettings]: Chat %d doesn't exist, returning default settings", chatID)
			return &models.GreetingSettings{
				ChatID:             chatID,
				ShouldCleanService: false,
				WelcomeSettings: &models.WelcomeSettings{
					LastMsgId:     0,
					CleanWelcome:  false,
					ShouldWelcome: true,
					WelcomeText:   db.DefaultWelcome,
					WelcomeType:   db.TEXT,
					Button:        models.ButtonArray{},
				},
				GoodbyeSettings: &models.GoodbyeSettings{
					LastMsgId:     0,
					CleanGoodbye:  false,
					ShouldGoodbye: false,
					GoodbyeText:   db.DefaultGoodbye,
					GoodbyeType:   db.TEXT,
					Button:        models.ButtonArray{},
				},
			}
		}

		// Create default settings only if chat exists
		greetingSrc = &models.GreetingSettings{
			ChatID:             chatID,
			ShouldCleanService: false,
			WelcomeSettings: &models.WelcomeSettings{
				LastMsgId:     0,
				CleanWelcome:  false,
				ShouldWelcome: true,
				WelcomeText:   db.DefaultWelcome,
				WelcomeType:   db.TEXT,
				Button:        models.ButtonArray{},
			},
			GoodbyeSettings: &models.GoodbyeSettings{
				LastMsgId:     0,
				CleanGoodbye:  false,
				ShouldGoodbye: false,
				GoodbyeText:   db.DefaultGoodbye,
				GoodbyeType:   db.TEXT,
				Button:        models.ButtonArray{},
			},
		}

		err := db.CreateRecord(greetingSrc)
		if err != nil {
			log.Errorf("[Database][checkGreetingSettings]: %v ", err)
		}
	} else if err != nil {
		log.Errorf("[Database][checkGreetingSettings]: %v", err)
		// Return default settings on error
		greetingSrc = &models.GreetingSettings{
			ChatID:             chatID,
			ShouldCleanService: false,
			WelcomeSettings: &models.WelcomeSettings{
				LastMsgId:     0,
				CleanWelcome:  false,
				ShouldWelcome: true,
				WelcomeText:   db.DefaultWelcome,
				WelcomeType:   db.TEXT,
				Button:        models.ButtonArray{},
			},
			GoodbyeSettings: &models.GoodbyeSettings{
				LastMsgId:     0,
				CleanGoodbye:  false,
				ShouldGoodbye: false,
				GoodbyeText:   db.DefaultGoodbye,
				GoodbyeType:   db.TEXT,
				Button:        models.ButtonArray{},
			},
		}
	}

	// Ensure WelcomeSettings and GoodbyeSettings are initialized even for existing records
	if greetingSrc.WelcomeSettings == nil {
		greetingSrc.WelcomeSettings = &models.WelcomeSettings{
			LastMsgId:     0,
			CleanWelcome:  false,
			ShouldWelcome: true,
			WelcomeText:   db.DefaultWelcome,
			WelcomeType:   db.TEXT,
			Button:        models.ButtonArray{},
		}
	} else if greetingSrc.WelcomeSettings.WelcomeText == "" {
		// Set default welcome text if it's empty (for existing records with empty text)
		greetingSrc.WelcomeSettings.WelcomeText = db.DefaultWelcome
	}

	if greetingSrc.GoodbyeSettings == nil {
		greetingSrc.GoodbyeSettings = &models.GoodbyeSettings{
			LastMsgId:     0,
			CleanGoodbye:  false,
			ShouldGoodbye: false,
			GoodbyeText:   db.DefaultGoodbye,
			GoodbyeType:   db.TEXT,
			Button:        models.ButtonArray{},
		}
	} else if greetingSrc.GoodbyeSettings.GoodbyeText == "" {
		// Set default goodbye text if it's empty (for existing records with empty text)
		greetingSrc.GoodbyeSettings.GoodbyeText = db.DefaultGoodbye
	}

	return greetingSrc
}

// GetGreetingSettings returns the greeting settings for the specified chat ID.
// This is the public interface to access greeting settings.
func GetGreetingSettings(chatID int64) *models.GreetingSettings {
	return checkGreetingSettings(chatID)
}

// GetWelcomeButtons retrieves the welcome message buttons for the specified chat.
// Returns an empty slice if no buttons are configured or settings are missing.
func GetWelcomeButtons(chatId int64) []models.Button {
	greetingSettings := checkGreetingSettings(chatId)
	if greetingSettings.WelcomeSettings != nil {
		return []models.Button(greetingSettings.WelcomeSettings.Button)
	}
	return []models.Button{}
}

// GetGoodbyeButtons retrieves the goodbye message buttons for the specified chat.
// Returns an empty slice if no buttons are configured or settings are missing.
func GetGoodbyeButtons(chatId int64) []models.Button {
	greetingSettings := checkGreetingSettings(chatId)
	if greetingSettings.GoodbyeSettings != nil {
		return []models.Button(greetingSettings.GoodbyeSettings.Button)
	}
	return []models.Button{}
}

func defaultGreetingSettingsAttrs(chatID int64) map[string]any {
	return map[string]any{
		"chat_id":                chatID,
		"clean_service_settings": false,
		"welcome_enabled":        true,
		"welcome_text":           db.DefaultWelcome,
		"welcome_type":           db.TEXT,
		"welcome_btns":           models.ButtonArray{},
		"goodbye_enabled":        false,
		"goodbye_text":           db.DefaultGoodbye,
		"goodbye_type":           db.TEXT,
		"goodbye_btns":           models.ButtonArray{},
		"auto_approve":           false,
	}
}

func upsertGreetingSettings(chatID int64, updates map[string]any) error {
	if !db.ChatExists(chatID) {
		if err := chats.EnsureChatInDb(chatID, ""); err != nil {
			return alitaerrors.Wrapf(err, "ensure chat %d in db", chatID)
		}
	}
	updates["updated_at"] = time.Now()
	settings := models.GreetingSettings{}
	if err := db.DB.Where("chat_id = ?", chatID).
		Attrs(defaultGreetingSettingsAttrs(chatID)).
		FirstOrCreate(&settings).Error; err != nil {
		return alitaerrors.Wrapf(err, "first-or-create greeting settings for chat %d", chatID)
	}
	if err := db.DB.Model(&models.GreetingSettings{}).
		Where("chat_id = ?", chatID).
		Updates(updates).Error; err != nil {
		return alitaerrors.Wrapf(err, "update greeting settings for chat %d", chatID)
	}
	return nil
}

// SetWelcomeText updates the welcome message text, file ID, buttons, and type for a chat.
// Creates default greeting settings if they don't exist.
//
//nolint:dupl // SetGoodbyeText has similar structure but different struct fields
func SetWelcomeText(chatID int64, welcometxt, fileId string, buttons []models.Button, welcType int) error {
	updates := map[string]any{
		"welcome_text":    welcometxt,
		"welcome_btns":    models.ButtonArray(buttons),
		"welcome_type":    welcType,
		"welcome_file_id": fileId,
	}

	err := upsertGreetingSettings(chatID, updates)
	if err != nil {
		log.Errorf("[Database][SetWelcomeText]: %v", err)
		return err
	}

	// Invalidate cache after updating welcome text
	cache.DeleteCache(cache.CacheKey("greetings", chatID))
	return nil
}

// SetWelcomeToggle enables or disables welcome messages for the specified chat.
// Creates default greeting settings if they don't exist.
func SetWelcomeToggle(chatID int64, pref bool) error {
	updates := map[string]any{
		"welcome_enabled": pref,
	}

	err := upsertGreetingSettings(chatID, updates)
	if err != nil {
		log.Errorf("[Database][SetWelcomeToggle]: %v", err)
		return err
	}

	// Invalidate cache after updating welcome toggle
	cache.DeleteCache(cache.CacheKey("greetings", chatID))
	return nil
}

// SetGoodbyeText updates the goodbye message text, file ID, buttons, and type for a chat.
// Creates default greeting settings if they don't exist.
//
//nolint:dupl // SetGoodbyeText has similar structure to SetWelcomeText but different struct fields
func SetGoodbyeText(chatID int64, goodbyetext, fileId string, buttons []models.Button, goodbyeType int) error {
	updates := map[string]any{
		"goodbye_text":    goodbyetext,
		"goodbye_btns":    models.ButtonArray(buttons),
		"goodbye_type":    goodbyeType,
		"goodbye_file_id": fileId,
	}

	err := upsertGreetingSettings(chatID, updates)
	if err != nil {
		log.Errorf("[Database][SetGoodbyeText]: %v", err)
		return err
	}

	// Invalidate cache after updating goodbye text
	cache.DeleteCache(cache.CacheKey("greetings", chatID))
	return nil
}

// SetGoodbyeToggle enables or disables goodbye messages for the specified chat.
// Creates default greeting settings if they don't exist.
func SetGoodbyeToggle(chatID int64, pref bool) error {
	updates := map[string]any{
		"goodbye_enabled": pref,
	}

	err := upsertGreetingSettings(chatID, updates)
	if err != nil {
		log.Errorf("[Database][SetGoodbyeToggle]: %v", err)
		return err
	}

	// Invalidate cache after updating goodbye toggle
	cache.DeleteCache(cache.CacheKey("greetings", chatID))
	return nil
}

// SetShouldCleanService sets whether service messages should be automatically cleaned in the chat.
// Creates default greeting settings if they don't exist.
func SetShouldCleanService(chatID int64, pref bool) error {
	updates := map[string]any{
		"clean_service_settings": pref,
	}

	err := upsertGreetingSettings(chatID, updates)
	if err != nil {
		log.Errorf("[Database][SetShouldCleanService]: %v", err)
		return err
	}

	// Invalidate cache after updating clean service setting
	cache.DeleteCache(cache.CacheKey("greetings", chatID))
	return nil
}

// SetShouldAutoApprove sets whether new members should be automatically approved in the chat.
// Creates default greeting settings if they don't exist.
func SetShouldAutoApprove(chatID int64, pref bool) error {
	updates := map[string]any{
		"auto_approve": pref,
	}

	err := upsertGreetingSettings(chatID, updates)
	if err != nil {
		log.Errorf("[Database][SetShouldAutoApprove]: %v", err)
		return err
	}

	// Invalidate cache after updating auto approve setting
	cache.DeleteCache(cache.CacheKey("greetings", chatID))
	return nil
}

// SetCleanWelcomeSetting sets whether old welcome messages should be automatically cleaned.
// Creates default greeting settings if they don't exist.
func SetCleanWelcomeSetting(chatID int64, pref bool) error {
	updates := map[string]any{
		"welcome_clean_old": pref,
	}

	err := upsertGreetingSettings(chatID, updates)
	if err != nil {
		log.Errorf("[Database][SetCleanWelcomeSetting]: %v", err)
		return err
	}

	// Invalidate cache after updating clean welcome setting
	cache.DeleteCache(cache.CacheKey("greetings", chatID))
	return nil
}

// SetCleanWelcomeMsgId updates the message ID of the last welcome message for cleanup purposes.
// Creates default greeting settings if they don't exist.
func SetCleanWelcomeMsgId(chatId, msgId int64) error {
	updates := map[string]any{
		"welcome_last_msg_id": msgId,
	}

	err := upsertGreetingSettings(chatId, updates)
	if err != nil {
		log.Errorf("[Database][SetCleanWelcomeMsgId]: %v", err)
		return err
	}

	// Invalidate cache after updating welcome message ID
	cache.DeleteCache(cache.CacheKey("greetings", chatId))
	return nil
}

// SetCleanGoodbyeSetting sets whether old goodbye messages should be automatically cleaned.
// Creates default greeting settings if they don't exist.
func SetCleanGoodbyeSetting(chatID int64, pref bool) error {
	updates := map[string]any{
		"goodbye_clean_old": pref,
	}

	err := upsertGreetingSettings(chatID, updates)
	if err != nil {
		log.Errorf("[Database][SetCleanGoodbyeSetting]: %v", err)
		return err
	}

	// Invalidate cache after updating clean goodbye setting
	cache.DeleteCache(cache.CacheKey("greetings", chatID))
	return nil
}

// SetCleanGoodbyeMsgId updates the message ID of the last goodbye message for cleanup purposes.
// Creates default greeting settings if they don't exist.
func SetCleanGoodbyeMsgId(chatId, msgId int64) error {
	updates := map[string]any{
		"goodbye_last_msg_id": msgId,
	}

	err := upsertGreetingSettings(chatId, updates)
	if err != nil {
		log.Errorf("[Database][SetCleanGoodbyeMsgId]: %v", err)
		return err
	}

	// Invalidate cache after updating goodbye message ID
	cache.DeleteCache(cache.CacheKey("greetings", chatId))
	return nil
}

// LoadGreetingsStats returns statistics about greeting features across all chats.
// Returns counts for enabled welcome messages, goodbye messages, clean service, clean welcome, and clean goodbye features.
func LoadGreetingsStats() (enabledWelcome, enabledGoodbye, cleanServiceEnabled, cleanWelcomeEnabled, cleanGoodbyeEnabled int64) {
	// Use a single query with COUNT and CASE WHEN for better performance
	type greetingStats struct {
		EnabledWelcome      int64
		EnabledGoodbye      int64
		CleanServiceEnabled int64
		CleanWelcomeEnabled int64
		CleanGoodbyeEnabled int64
	}

	var stats greetingStats
	query := `
		SELECT
			COUNT(CASE WHEN welcome_enabled = true THEN 1 END) as enabled_welcome,
			COUNT(CASE WHEN goodbye_enabled = true THEN 1 END) as enabled_goodbye,
			COUNT(CASE WHEN clean_service_settings = true THEN 1 END) as clean_service_enabled,
			COUNT(CASE WHEN welcome_clean_old = true THEN 1 END) as clean_welcome_enabled,
			COUNT(CASE WHEN goodbye_clean_old = true THEN 1 END) as clean_goodbye_enabled
		FROM greetings
	`

	err := db.DB.Raw(query).Scan(&stats).Error
	if err != nil {
		log.Errorf("[Database][LoadGreetingsStats] querying stats: %v", err)
		return 0, 0, 0, 0, 0
	}

	return stats.EnabledWelcome, stats.EnabledGoodbye, stats.CleanServiceEnabled, stats.CleanWelcomeEnabled, stats.CleanGoodbyeEnabled
}
