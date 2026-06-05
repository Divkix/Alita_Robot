package greetings

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	alitaerrors "github.com/divkix/Alita_Robot/alita/utils/errors"
)

// checkGreetingSettings retrieves or creates default greeting settings for a chat.
func checkGreetingSettings(chatID int64) (greetingSrc *models.GreetingSettings) {
	greetingSrc = &models.GreetingSettings{}
	err := db.GetRecord(greetingSrc, bson.M{"chat_id": chatID})

	if err == db.ErrRecordNotFound {
		// Ensure chat exists before creating greeting settings
		if !db.ChatExists(chatID) {
			// Chat doesn't exist, return default settings without creating record
			log.Warnf("[Database][checkGreetingSettings]: Chat %d doesn't exist, returning default settings", chatID)
			return defaultGreetingSettings(chatID)
		}

		// Create default settings only if chat exists
		greetingSrc = defaultGreetingSettings(chatID)
		greetingSrc.CreatedAt = time.Now()
		greetingSrc.UpdatedAt = time.Now()

		err := db.CreateRecord(greetingSrc)
		if err != nil {
			log.Errorf("[Database][checkGreetingSettings]: %v ", err)
		}
	} else if err != nil {
		log.Errorf("[Database][checkGreetingSettings]: %v", err)
		// Return default settings on error
		greetingSrc = defaultGreetingSettings(chatID)
	}

	// Ensure WelcomeSettings and GoodbyeSettings are initialized
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

func defaultGreetingSettings(chatID int64) *models.GreetingSettings {
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
		ShouldAutoApprove: false,
	}
}

// GetGreetingSettings returns the greeting settings for the specified chat ID.
func GetGreetingSettings(chatID int64) *models.GreetingSettings {
	return checkGreetingSettings(chatID)
}

// GetWelcomeButtons retrieves the welcome message buttons for the specified chat.
func GetWelcomeButtons(chatId int64) []models.Button {
	greetingSettings := checkGreetingSettings(chatId)
	if greetingSettings.WelcomeSettings != nil {
		return []models.Button(greetingSettings.WelcomeSettings.Button)
	}
	return []models.Button{}
}

// GetGoodbyeButtons retrieves the goodbye message buttons for the specified chat.
func GetGoodbyeButtons(chatId int64) []models.Button {
	greetingSettings := checkGreetingSettings(chatId)
	if greetingSettings.GoodbyeSettings != nil {
		return []models.Button(greetingSettings.GoodbyeSettings.Button)
	}
	return []models.Button{}
}

func upsertGreetingSettings(chatID int64, updates map[string]any) error {
	if !db.ChatExists(chatID) {
		if err := chats.EnsureChatInDb(chatID, ""); err != nil {
			return alitaerrors.Wrapf(err, "ensure chat %d in db", chatID)
		}
	}

	collection := db.DB.Collection("greetings")

	// Create MongoDB update document from map
	mongoUpdate := bson.M{}
	for k, v := range updates {
		// Map GORM column names to BSON field names if necessary
		// In GreetingSettings, some fields are embedded, so we need to map them correctly.
		switch k {
		case "welcome_text":
			mongoUpdate["welcome_settings.text"] = v
		case "welcome_btns":
			mongoUpdate["welcome_settings.btns"] = v
		case "welcome_type":
			mongoUpdate["welcome_settings.type"] = v
		case "welcome_file_id":
			mongoUpdate["welcome_settings.file_id"] = v
		case "welcome_enabled":
			mongoUpdate["welcome_settings.enabled"] = v
		case "goodbye_text":
			mongoUpdate["goodbye_settings.text"] = v
		case "goodbye_btns":
			mongoUpdate["goodbye_settings.btns"] = v
		case "goodbye_type":
			mongoUpdate["goodbye_settings.type"] = v
		case "goodbye_file_id":
			mongoUpdate["goodbye_settings.file_id"] = v
		case "goodbye_enabled":
			mongoUpdate["goodbye_settings.enabled"] = v
		case "welcome_clean_old":
			mongoUpdate["welcome_settings.clean_old"] = v
		case "welcome_last_msg_id":
			mongoUpdate["welcome_settings.last_msg_id"] = v
		case "goodbye_clean_old":
			mongoUpdate["goodbye_settings.clean_old"] = v
		case "goodbye_last_msg_id":
			mongoUpdate["goodbye_settings.last_msg_id"] = v
		case "clean_service_settings":
			mongoUpdate["clean_service_settings"] = v
		case "auto_approve":
			mongoUpdate["auto_approve"] = v
		default:
			mongoUpdate[k] = v
		}
	}
	mongoUpdate["updated_at"] = time.Now()

	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(context.Background(), bson.M{"chat_id": chatID}, bson.M{"$set": mongoUpdate}, opts)
	if err != nil {
		return alitaerrors.Wrapf(err, "update greeting settings for chat %d", chatID)
	}
	return nil
}

// SetWelcomeText updates the welcome message text, file ID, buttons, and type for a chat.
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
func LoadGreetingsStats() (enabledWelcome, enabledGoodbye, cleanServiceEnabled, cleanWelcomeEnabled, cleanGoodbyeEnabled int64) {
	collection := db.DB.Collection("greetings")

	enabledWelcome, _ = collection.CountDocuments(context.Background(), bson.M{"welcome_settings.enabled": true})
	enabledGoodbye, _ = collection.CountDocuments(context.Background(), bson.M{"goodbye_settings.enabled": true})
	cleanServiceEnabled, _ = collection.CountDocuments(context.Background(), bson.M{"clean_service_settings": true})
	cleanWelcomeEnabled, _ = collection.CountDocuments(context.Background(), bson.M{"welcome_settings.clean_old": true})
	cleanGoodbyeEnabled, _ = collection.CountDocuments(context.Background(), bson.M{"goodbye_settings.clean_old": true})

	return
}
