package rules

import (
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// checkRulesSetting retrieves or creates default rules settings for a chat.
// Used internally before performing any rules-related operation.
// Returns safe defaults alongside any database error.
func checkRulesSetting(chatID int64) (*models.RulesSettings, error) {
	rulesrc := &models.RulesSettings{}
	err := db.GetRecord(rulesrc, models.RulesSettings{ChatId: chatID})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Ensure chat exists in database before creating rules to satisfy foreign key constraint
		if err := chats.EnsureChatInDb(chatID, ""); err != nil {
			log.Errorf("[Database] checkRulesSetting: Failed to ensure chat exists for %d: %v", chatID, err)
			return &models.RulesSettings{ChatId: chatID, Rules: ""}, err
		}

		// Create default settings
		rulesrc = &models.RulesSettings{ChatId: chatID, Rules: ""}
		err := db.CreateRecord(rulesrc)
		if err != nil {
			log.Errorf("[Database] checkRulesSetting: %v - %d", err, chatID)
			return rulesrc, err
		}
	} else if err != nil {
		// Return default on error
		rulesrc = &models.RulesSettings{ChatId: chatID, Rules: ""}
		log.Errorf("[Database] checkRulesSetting: %v - %d", err, chatID)
		return rulesrc, err
	}
	return rulesrc, nil
}

// GetChatRulesInfo returns the rules settings for the specified chat ID.
// This is the public interface to access chat rules information.
func GetChatRulesInfo(chatId int64) *models.RulesSettings {
	rulesrc, _ := checkRulesSetting(chatId)
	return rulesrc
}

// SetChatRules updates the rules text for the specified chat.
// Creates default rules settings if they don't exist.
// Returns any persistence error.
func SetChatRules(chatId int64, rules string) error {
	if _, err := checkRulesSetting(chatId); err != nil {
		return err
	}
	err := db.UpdateRecordWithZeroValues(&models.RulesSettings{}, models.RulesSettings{ChatId: chatId}, map[string]any{"rules": rules})
	if err != nil {
		log.Errorf("[Database] SetChatRules: %v - %d", err, chatId)
	}
	return err
}

// SetChatRulesButton updates the rules button text for the specified chat.
// The button is used to display rules in a more interactive format.
// Returns any persistence error.
func SetChatRulesButton(chatId int64, rulesButton string) error {
	if _, err := checkRulesSetting(chatId); err != nil {
		return err
	}
	err := db.UpdateRecordWithZeroValues(&models.RulesSettings{}, models.RulesSettings{ChatId: chatId}, map[string]any{"rules_btn": rulesButton})
	if err != nil {
		log.Errorf("[Database] SetChatRulesButton: %v", err)
	}
	return err
}

// SetPrivateRules sets whether rules should be sent privately to users instead of in the group.
// When enabled, rules are sent as a private message to the requesting user.
// Returns any persistence error.
func SetPrivateRules(chatId int64, pref bool) error {
	if _, err := checkRulesSetting(chatId); err != nil {
		return err
	}
	err := db.UpdateRecordWithZeroValues(&models.RulesSettings{}, models.RulesSettings{ChatId: chatId}, map[string]any{"private": pref})
	if err != nil {
		log.Errorf("[Database] SetPrivateRules: %v", err)
	}
	return err
}

// LoadRulesStats returns statistics about rules features across all chats.
// Returns the count of chats with rules set and chats with private rules enabled.
func LoadRulesStats() (setRules, pvtRules int64) {
	// Count chats with rules set (non-empty rules)
	err := db.DB.Model(&models.RulesSettings{}).Where("rules != ?", "").Count(&setRules).Error
	if err != nil {
		log.Errorf("[Database] LoadRulesStats (set rules): %v", err)
	}

	// Count chats with private rules enabled
	err = db.DB.Model(&models.RulesSettings{}).Where("private = ?", true).Count(&pvtRules).Error
	if err != nil {
		log.Errorf("[Database] LoadRulesStats (private rules): %v", err)
	}

	return
}
