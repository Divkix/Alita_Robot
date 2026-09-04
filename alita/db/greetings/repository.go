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

func checkGreetingSettings(chatID int64) (greetingSrc *models.GreetingSettings) {
	greetingSrc = &models.GreetingSettings{}
	err := db.GetRecord(greetingSrc, map[string]any{"chat_id": chatID})

	if errors.Is(err, gorm.ErrRecordNotFound) {
		if !db.ChatExists(chatID) {
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
		greetingSrc.GoodbyeSettings.GoodbyeText = db.DefaultGoodbye
	}

	return greetingSrc
}

func GetGreetingSettings(chatID int64) *models.GreetingSettings {
	return checkGreetingSettings(chatID)
}

func GetWelcomeButtons(chatId int64) []models.Button {
	greetingSettings := checkGreetingSettings(chatId)
	if greetingSettings.WelcomeSettings != nil {
		return []models.Button(greetingSettings.WelcomeSettings.Button)
	}
	return []models.Button{}
}

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
	cache.DeleteCache(cache.CacheKey("greetings", chatID))
	return nil
}

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

	return nil
}

func SetWelcomeToggle(chatID int64, pref bool) error {
	updates := map[string]any{
		"welcome_enabled": pref,
	}

	err := upsertGreetingSettings(chatID, updates)
	if err != nil {
		log.Errorf("[Database][SetWelcomeToggle]: %v", err)
		return err
	}

	return nil
}

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

	return nil
}

func SetGoodbyeToggle(chatID int64, pref bool) error {
	updates := map[string]any{
		"goodbye_enabled": pref,
	}

	err := upsertGreetingSettings(chatID, updates)
	if err != nil {
		log.Errorf("[Database][SetGoodbyeToggle]: %v", err)
		return err
	}

	return nil
}

func SetShouldCleanService(chatID int64, pref bool) error {
	updates := map[string]any{
		"clean_service_settings": pref,
	}

	err := upsertGreetingSettings(chatID, updates)
	if err != nil {
		log.Errorf("[Database][SetShouldCleanService]: %v", err)
		return err
	}

	return nil
}

func SetShouldAutoApprove(chatID int64, pref bool) error {
	updates := map[string]any{
		"auto_approve": pref,
	}

	err := upsertGreetingSettings(chatID, updates)
	if err != nil {
		log.Errorf("[Database][SetShouldAutoApprove]: %v", err)
		return err
	}

	return nil
}

func SetCleanWelcomeSetting(chatID int64, pref bool) error {
	updates := map[string]any{
		"welcome_clean_old": pref,
	}

	err := upsertGreetingSettings(chatID, updates)
	if err != nil {
		log.Errorf("[Database][SetCleanWelcomeSetting]: %v", err)
		return err
	}

	return nil
}

func SetCleanWelcomeMsgId(chatId, msgId int64) error {
	updates := map[string]any{
		"welcome_last_msg_id": msgId,
	}

	err := upsertGreetingSettings(chatId, updates)
	if err != nil {
		log.Errorf("[Database][SetCleanWelcomeMsgId]: %v", err)
		return err
	}

	return nil
}

func SetCleanGoodbyeSetting(chatID int64, pref bool) error {
	updates := map[string]any{
		"goodbye_clean_old": pref,
	}

	err := upsertGreetingSettings(chatID, updates)
	if err != nil {
		log.Errorf("[Database][SetCleanGoodbyeSetting]: %v", err)
		return err
	}

	return nil
}

func SetCleanGoodbyeMsgId(chatId, msgId int64) error {
	updates := map[string]any{
		"goodbye_last_msg_id": msgId,
	}

	err := upsertGreetingSettings(chatId, updates)
	if err != nil {
		log.Errorf("[Database][SetCleanGoodbyeMsgId]: %v", err)
		return err
	}

	return nil
}

func LoadGreetingsStats() (enabledWelcome, enabledGoodbye, cleanServiceEnabled, cleanWelcomeEnabled, cleanGoodbyeEnabled int64) {
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
