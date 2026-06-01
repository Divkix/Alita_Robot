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
// Returns default settings with empty rules if the chat doesn't exist.
func checkRulesSetting(chatID int64) (rulesrc *models.RulesSettings) {
	rulesrc = &models.RulesSettings{}
	err := db.GetRecord(rulesrc, models.RulesSettings{ChatId: chatID})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Ensure chat exists in database before creating rules to satisfy foreign key constraint
		if err := chats.EnsureChatInDb(chatID, ""); err != nil {
			log.Errorf("[Database] checkRulesSetting: Failed to ensure chat exists for %d: %v", chatID, err)
			return &models.RulesSettings{ChatId: chatID, Rules: ""}
		}

		// Create default settings
		rulesrc = &models.RulesSettings{ChatId: chatID, Rules: ""}
		err := db.CreateRecord(rulesrc)
		if err != nil {
			log.Errorf("[Database] checkRulesSetting: %v - %d", err, chatID)
		}
	} else if err != nil {
		// Return default on error
		rulesrc = &models.RulesSettings{ChatId: chatID, Rules: ""}
		log.Errorf("[Database] checkRulesSetting: %v - %d", err, chatID)
	}
	return rulesrc
}

// GetChatRulesInfo returns the rules settings for the specified chat ID.
// This is the public interface to access chat rules information.
func GetChatRulesInfo(chatId int64) *models.RulesSettings {
	return checkRulesSetting(chatId)
}

// SetChatRules updates the rules text for the specified chat.
// Creates default rules settings if they don't exist.
func SetChatRules(chatId int64, rules string) {
	checkRulesSetting(chatId)
	err := db.UpdateRecordWithZeroValues(&models.RulesSettings{}, models.RulesSettings{ChatId: chatId}, map[string]any{"rules": rules})
	if err != nil {
		log.Errorf("[Database] SetChatRules: %v - %d", err, chatId)
	}
}

// SetChatRulesButton updates the rules button text for the specified chat.
// The button is used to display rules in a more interactive format.
func SetChatRulesButton(chatId int64, rulesButton string) {
	checkRulesSetting(chatId)
	err := db.UpdateRecordWithZeroValues(&models.RulesSettings{}, models.RulesSettings{ChatId: chatId}, map[string]any{"rules_btn": rulesButton})
	if err != nil {
		log.Errorf("[Database] SetChatRulesButton: %v", err)
	}
}

// SetPrivateRules sets whether rules should be sent privately to users instead of in the group.
// When enabled, rules are sent as a private message to the requesting user.
func SetPrivateRules(chatId int64, pref bool) {
	checkRulesSetting(chatId)
	err := db.UpdateRecordWithZeroValues(&models.RulesSettings{}, models.RulesSettings{ChatId: chatId}, map[string]any{"private": pref})
	if err != nil {
		log.Errorf("[Database] SetPrivateRules: %v", err)
	}
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
