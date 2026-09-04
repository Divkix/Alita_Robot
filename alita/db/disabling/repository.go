package disabling

import (
	"slices"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func DisableCMD(chatID int64, cmd string) error {
	disableSetting := &models.DisableSettings{
		ChatId:   chatID,
		Command:  cmd,
		Disabled: true,
	}

	err := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}, {Name: "command"}},
		DoNothing: true,
	}).Create(disableSetting).Error
	if err != nil {
		log.Errorf("[Database][DisableCMD]: %v", err)
		return err
	}

	invalidateDisabledCommandsCache(chatID)
	return nil
}

func EnableCMD(chatID int64, cmd string) error {
	err := db.DB.Where("chat_id = ? AND command = ?", chatID, cmd).Delete(&models.DisableSettings{}).Error
	if err != nil {
		log.Errorf("[Database][EnableCMD]: %v", err)
		return err
	}

	invalidateDisabledCommandsCache(chatID)
	return nil
}

func GetChatDisabledCMDs(chatId int64) []string {
	commands, err := getChatDisabledCMDs(chatId)
	if err != nil {
		log.Errorf("[Database] GetChatDisabledCMDs: %v - %d", err, chatId)
		return []string{}
	}
	return commands
}

func getChatDisabledCMDs(chatId int64) ([]string, error) {
	var disableSettings []*models.DisableSettings
	err := db.GetRecords(&disableSettings, models.DisableSettings{ChatId: chatId, Disabled: true})
	if err != nil {
		return nil, err
	}

	commands := make([]string, len(disableSettings))
	for i, setting := range disableSettings {
		commands[i] = setting.Command
	}
	return commands, nil
}

func GetChatDisabledCMDsCached(chatId int64) []string {
	cacheKey := cache.CacheKey("disabled_cmds", chatId)
	result, err := cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLDisabledCmds, func() ([]string, error) {
		return getChatDisabledCMDs(chatId)
	})
	if err != nil {
		log.Errorf("[Cache] Failed to get disabled commands from cache for chat %d: %v", chatId, err)
		return GetChatDisabledCMDs(chatId)
	}
	return result
}

func IsCommandDisabled(chatId int64, cmd string) bool {
	return slices.Contains(GetChatDisabledCMDsCached(chatId), cmd)
}

func invalidateDisabledCommandsCache(chatID int64) {
	cache.DeleteCache(cache.CacheKey("disabled_cmds", chatID))
}

func ToggleDel(chatId int64, pref bool) error {
	updates := map[string]any{
		"chat_id":         chatId,
		"delete_commands": pref,
	}
	err := db.DB.Where("chat_id = ?", chatId).
		Assign(updates).
		FirstOrCreate(&models.DisableChatSettings{}).Error
	if err != nil {
		log.Errorf("[Database] ToggleDel: %v", err)
		return err
	}
	return nil
}

func ShouldDel(chatId int64) bool {
	var settings models.DisableChatSettings
	err := db.GetRecord(&settings, models.DisableChatSettings{ChatId: chatId})
	if err != nil {
		log.Errorf("[Database] ShouldDel: %v", err)
		return false
	}
	return settings.DeleteCommands
}

func LoadDisableStats() (disabledCmds, disableEnabledChats int64) {
	err := db.DB.Model(&models.DisableSettings{}).Where("disabled = ?", true).Count(&disabledCmds).Error
	if err != nil {
		log.Errorf("[Database] LoadDisableStats (commands): %v", err)
		return 0, 0
	}

	err = db.DB.Model(&models.DisableSettings{}).Where("disabled = ?", true).Distinct("chat_id").Count(&disableEnabledChats).Error
	if err != nil {
		log.Errorf("[Database] LoadDisableStats (chats): %v", err)
		return disabledCmds, 0
	}

	return
}
