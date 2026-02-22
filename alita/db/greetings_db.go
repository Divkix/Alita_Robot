package db

import (
	"errors"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// checkGreetingSettings retrieves or creates default greeting settings for a chat.
// Used internally before performing any greeting-related operation.
// Returns default settings if the chat doesn't exist in the database.
func checkGreetingSettings(chatID int64) (greetingSrc *GreetingSettings) {
	greetingSrc = &GreetingSettings{}
	err := GetRecord(greetingSrc, map[string]any{"chat_id": chatID})

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Ensure chat exists before creating greeting settings
		if !ChatExists(chatID) {
			// Chat doesn't exist, return default settings without creating record
			log.Warnf("[Database][checkGreetingSettings]: Chat %d doesn't exist, returning default settings", chatID)
			return &GreetingSettings{
				ChatID:             chatID,
				ShouldCleanService: false,
				WelcomeSettings: &WelcomeSettings{
					LastMsgId:     0,
					CleanWelcome:  false,
					ShouldWelcome: true,
					WelcomeText:   DefaultWelcome,
					WelcomeType:   TEXT,
					Button:        ButtonArray{},
				},
				GoodbyeSettings: &GoodbyeSettings{
					LastMsgId:     0,
					CleanGoodbye:  false,
					ShouldGoodbye: false,
					GoodbyeText:   DefaultGoodbye,
					GoodbyeType:   TEXT,
					Button:        ButtonArray{},
				},
			}
		}

		// Create default settings only if chat exists
		greetingSrc = &GreetingSettings{
			ChatID:             chatID,
			ShouldCleanService: false,
			WelcomeSettings: &WelcomeSettings{
				LastMsgId:     0,
				CleanWelcome:  false,
				ShouldWelcome: true,
				WelcomeText:   DefaultWelcome,
				WelcomeType:   TEXT,
				Button:        ButtonArray{},
			},
			GoodbyeSettings: &GoodbyeSettings{
				LastMsgId:     0,
				CleanGoodbye:  false,
				ShouldGoodbye: false,
				GoodbyeText:   DefaultGoodbye,
				GoodbyeType:   TEXT,
				Button:        ButtonArray{},
			},
		}

		err := CreateRecord(greetingSrc)
		if err != nil {
			log.Errorf("[Database][checkGreetingSettings]: %v ", err)
		}
	} else if err != nil {
		log.Errorf("[Database][checkGreetingSettings]: %v", err)
		// Return default settings on error
		greetingSrc = &GreetingSettings{
			ChatID:             chatID,
			ShouldCleanService: false,
			WelcomeSettings: &WelcomeSettings{
				LastMsgId:     0,
				CleanWelcome:  false,
				ShouldWelcome: true,
				WelcomeText:   DefaultWelcome,
				WelcomeType:   TEXT,
				Button:        ButtonArray{},
			},
			GoodbyeSettings: &GoodbyeSettings{
				LastMsgId:     0,
				CleanGoodbye:  false,
				ShouldGoodbye: false,
				GoodbyeText:   DefaultGoodbye,
				GoodbyeType:   TEXT,
				Button:        ButtonArray{},
			},
		}
	}

	// Ensure WelcomeSettings and GoodbyeSettings are initialized even for existing records
	if greetingSrc.WelcomeSettings == nil {
		greetingSrc.WelcomeSettings = &WelcomeSettings{
			LastMsgId:     0,
			CleanWelcome:  false,
			ShouldWelcome: true,
			WelcomeText:   DefaultWelcome,
			WelcomeType:   TEXT,
			Button:        ButtonArray{},
		}
	} else if greetingSrc.WelcomeSettings.WelcomeText == "" {
		// Set default welcome text if it's empty (for existing records with empty text)
		greetingSrc.WelcomeSettings.WelcomeText = DefaultWelcome
	}

	if greetingSrc.GoodbyeSettings == nil {
		greetingSrc.GoodbyeSettings = &GoodbyeSettings{
			LastMsgId:     0,
			CleanGoodbye:  false,
			ShouldGoodbye: false,
			GoodbyeText:   DefaultGoodbye,
			GoodbyeType:   TEXT,
			Button:        ButtonArray{},
		}
	} else if greetingSrc.GoodbyeSettings.GoodbyeText == "" {
		// Set default goodbye text if it's empty (for existing records with empty text)
		greetingSrc.GoodbyeSettings.GoodbyeText = DefaultGoodbye
	}

	return greetingSrc
}

// GetGreetingSettings returns the greeting settings for the specified chat ID.
// This is the public interface to access greeting settings.
func GetGreetingSettings(chatID int64) *GreetingSettings {
	return checkGreetingSettings(chatID)
}

// GetWelcomeButtons retrieves the welcome message buttons for the specified chat.
// Returns an empty slice if no buttons are configured or settings are missing.
func GetWelcomeButtons(chatId int64) []Button {
	greetingSettings := checkGreetingSettings(chatId)
	if greetingSettings.WelcomeSettings != nil {
		return []Button(greetingSettings.WelcomeSettings.Button)
	}
	return []Button{}
}

// GetGoodbyeButtons retrieves the goodbye message buttons for the specified chat.
// Returns an empty slice if no buttons are configured or settings are missing.
func GetGoodbyeButtons(chatId int64) []Button {
	greetingSettings := checkGreetingSettings(chatId)
	if greetingSettings.GoodbyeSettings != nil {
		return []Button(greetingSettings.GoodbyeSettings.Button)
	}
	return []Button{}
}

// SetWelcomeText updates the welcome message text, file ID, buttons, and type for a chat.
// Creates default greeting settings if they don't exist.
func SetWelcomeText(chatID int64, welcometxt, fileId string, buttons []Button, welcType int) {
	welcomeSrc := checkGreetingSettings(chatID)
	if welcomeSrc.WelcomeSettings == nil {
		welcomeSrc.WelcomeSettings = &WelcomeSettings{}
	}
	welcomeSrc.WelcomeSettings.WelcomeText = welcometxt
	welcomeSrc.WelcomeSettings.Button = ButtonArray(buttons)
	welcomeSrc.WelcomeSettings.WelcomeType = welcType
	welcomeSrc.WelcomeSettings.FileID = fileId

	err := UpdateRecord(&GreetingSettings{}, map[string]any{"chat_id": chatID}, welcomeSrc)
	if err != nil {
		log.Errorf("[Database][SetWelcomeText]: %v", err)
		return
	}
}

// SetWelcomeToggle enables or disables welcome messages for the specified chat.
// Creates default greeting settings if they don't exist.
func SetWelcomeToggle(chatID int64, pref bool) {
	welcomeSrc := checkGreetingSettings(chatID)
	if welcomeSrc.WelcomeSettings == nil {
		welcomeSrc.WelcomeSettings = &WelcomeSettings{}
	}
	welcomeSrc.WelcomeSettings.ShouldWelcome = pref

	// Use map to ensure zero values (false) are properly updated
	updates := map[string]any{
		"welcome_enabled": pref,
		"updated_at":      time.Now(),
	}

	err := DB.Model(&GreetingSettings{}).Where("chat_id = ?", chatID).Updates(updates).Error
	if err != nil {
		log.Errorf("[Database][SetWelcomeToggle]: %v", err)
		return
	}
}

// SetGoodbyeText updates the goodbye message text, file ID, buttons, and type for a chat.
// Creates default greeting settings if they don't exist.
func SetGoodbyeText(chatID int64, goodbyetext, fileId string, buttons []Button, goodbyeType int) {
	goodbyeSrc := checkGreetingSettings(chatID)
	if goodbyeSrc.GoodbyeSettings == nil {
		goodbyeSrc.GoodbyeSettings = &GoodbyeSettings{}
	}
	goodbyeSrc.GoodbyeSettings.GoodbyeText = goodbyetext
	goodbyeSrc.GoodbyeSettings.Button = ButtonArray(buttons)
	goodbyeSrc.GoodbyeSettings.GoodbyeType = goodbyeType
	goodbyeSrc.GoodbyeSettings.FileID = fileId

	err := UpdateRecord(&GreetingSettings{}, map[string]any{"chat_id": chatID}, goodbyeSrc)
	if err != nil {
		log.Errorf("[Database][SetGoodbyeText]: %v", err)
		return
	}
}

// SetGoodbyeToggle enables or disables goodbye messages for the specified chat.
// Creates default greeting settings if they don't exist.
func SetGoodbyeToggle(chatID int64, pref bool) {
	goodbyeSrc := checkGreetingSettings(chatID)
	if goodbyeSrc.GoodbyeSettings == nil {
		goodbyeSrc.GoodbyeSettings = &GoodbyeSettings{}
	}
	goodbyeSrc.GoodbyeSettings.ShouldGoodbye = pref

	// Use map to ensure zero values (false) are properly updated
	updates := map[string]any{
		"goodbye_enabled": pref,
		"updated_at":      time.Now(),
	}

	err := DB.Model(&GreetingSettings{}).Where("chat_id = ?", chatID).Updates(updates).Error
	if err != nil {
		log.Errorf("[Database][SetGoodbyeToggle]: %v", err)
		return
	}
}

// SetShouldCleanService sets whether service messages should be automatically cleaned in the chat.
// Creates default greeting settings if they don't exist.
func SetShouldCleanService(chatID int64, pref bool) {
	cleanServiceSrc := checkGreetingSettings(chatID)
	cleanServiceSrc.ShouldCleanService = pref

	// Use map to ensure zero values (false) are properly updated
	updates := map[string]any{
		"clean_service_settings": pref,
		"updated_at":             time.Now(),
	}

	err := DB.Model(&GreetingSettings{}).Where("chat_id = ?", chatID).Updates(updates).Error
	if err != nil {
		log.Errorf("[Database][SetShouldCleanService]: %v", err)
		return
	}
}

// SetShouldAutoApprove sets whether new members should be automatically approved in the chat.
// Creates default greeting settings if they don't exist.
func SetShouldAutoApprove(chatID int64, pref bool) {
	autoApproveSrc := checkGreetingSettings(chatID)
	autoApproveSrc.ShouldAutoApprove = pref

	// Use map to ensure zero values (false) are properly updated
	updates := map[string]any{
		"auto_approve": pref,
		"updated_at":   time.Now(),
	}

	err := DB.Model(&GreetingSettings{}).Where("chat_id = ?", chatID).Updates(updates).Error
	if err != nil {
		log.Errorf("[Database][SetShouldAutoApprove]: %v", err)
		return
	}
}

// SetCleanWelcomeSetting sets whether old welcome messages should be automatically cleaned.
// Creates default greeting settings if they don't exist.
func SetCleanWelcomeSetting(chatID int64, pref bool) {
	cleanWelcomeSrc := checkGreetingSettings(chatID)
	if cleanWelcomeSrc.WelcomeSettings == nil {
		cleanWelcomeSrc.WelcomeSettings = &WelcomeSettings{}
	}
	cleanWelcomeSrc.WelcomeSettings.CleanWelcome = pref

	// Use map to ensure zero values (false) are properly updated
	updates := map[string]any{
		"welcome_clean_old": pref,
		"updated_at":        time.Now(),
	}

	err := DB.Model(&GreetingSettings{}).Where("chat_id = ?", chatID).Updates(updates).Error
	if err != nil {
		log.Errorf("[Database][SetCleanWelcomeSetting]: %v", err)
		return
	}
}

// SetCleanWelcomeMsgId updates the message ID of the last welcome message for cleanup purposes.
// Creates default greeting settings if they don't exist.
func SetCleanWelcomeMsgId(chatId, msgId int64) {
	_ = checkGreetingSettings(chatId) // ensure record exists

	updates := map[string]any{
		"welcome_last_msg_id": msgId,
		"updated_at":          time.Now(),
	}

	err := DB.Model(&GreetingSettings{}).Where("chat_id = ?", chatId).Updates(updates).Error
	if err != nil {
		log.Errorf("[Database][SetCleanWelcomeMsgId]: %v", err)
		return
	}
}

// SetCleanGoodbyeSetting sets whether old goodbye messages should be automatically cleaned.
// Creates default greeting settings if they don't exist.
func SetCleanGoodbyeSetting(chatID int64, pref bool) {
	cleanGoodbyeSrc := checkGreetingSettings(chatID)
	if cleanGoodbyeSrc.GoodbyeSettings == nil {
		cleanGoodbyeSrc.GoodbyeSettings = &GoodbyeSettings{}
	}
	cleanGoodbyeSrc.GoodbyeSettings.CleanGoodbye = pref

	// Use map to ensure zero values (false) are properly updated
	updates := map[string]any{
		"goodbye_clean_old": pref,
		"updated_at":        time.Now(),
	}

	err := DB.Model(&GreetingSettings{}).Where("chat_id = ?", chatID).Updates(updates).Error
	if err != nil {
		log.Errorf("[Database][SetCleanGoodbyeSetting]: %v", err)
		return
	}
}

// SetCleanGoodbyeMsgId updates the message ID of the last goodbye message for cleanup purposes.
// Creates default greeting settings if they don't exist.
func SetCleanGoodbyeMsgId(chatId, msgId int64) {
	_ = checkGreetingSettings(chatId) // ensure record exists

	updates := map[string]any{
		"goodbye_last_msg_id": msgId,
		"updated_at":          time.Now(),
	}

	err := DB.Model(&GreetingSettings{}).Where("chat_id = ?", chatId).Updates(updates).Error
	if err != nil {
		log.Errorf("[Database][SetCleanGoodbyeMsgId]: %v", err)
		return
	}
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

	err := DB.Raw(query).Scan(&stats).Error
	if err != nil {
		log.Errorf("[Database][LoadGreetingsStats] querying stats: %v", err)
		return 0, 0, 0, 0, 0
	}

	return stats.EnabledWelcome, stats.EnabledGoodbye, stats.CleanServiceEnabled, stats.CleanWelcomeEnabled, stats.CleanGoodbyeEnabled
}
