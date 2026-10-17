package chats

import (
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const chatTouchInterval = time.Hour

func GetChatSettings(chatId int64) (chatSrc *models.Chat) {
	chat, err := GetChatBasicInfoCached(chatId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &models.Chat{}
		}
		log.Errorf("[Database] GetChatSettings: %v - %d", err, chatId)
		return &models.Chat{}
	}
	return chat
}

func EnsureChatInDb(chatId int64, chatName string) error {
	chatUpdate := &models.Chat{
		ChatId:   chatId,
		ChatName: chatName,
	}
	onConflict := clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoNothing: true,
	}
	if chatName != "" {
		onConflict.DoNothing = false
		onConflict.DoUpdates = clause.AssignmentColumns([]string{"chat_name", "updated_at"})
	}
	err := db.DB.Clauses(onConflict).Create(chatUpdate).Error
	if err != nil {
		log.Errorf("[Database] EnsureChatInDb: %v", err)
		return fmt.Errorf("failed to ensure chat %d in database: %w", chatId, err)
	}
	cache.DeleteCache(cache.CacheKey("chat", chatId))
	cache.DeleteCache(cache.CacheKey("chat_users", chatId))
	return nil
}

// TouchChat records activity in a chat: it creates the row if missing, and
// otherwise refreshes last_activity, clears is_inactive and stores a new title.
// An active, unrenamed chat touched within chatTouchInterval is left alone.
func TouchChat(chatId int64, chatname string) error {
	now := time.Now()

	columns := []string{"is_inactive", "last_activity", "updated_at"}
	if chatname != "" {
		columns = append(columns, "chat_name")
	}
	chat := &models.Chat{
		ChatId:       chatId,
		ChatName:     chatname,
		IsInactive:   false,
		LastActivity: now,
	}
	touch := "chats.last_activity < ? OR chats.is_inactive"
	if chatname != "" {
		touch += " OR coalesce(chats.chat_name, '') <> excluded.chat_name"
	}

	upsert := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns(columns),
		Where: clause.Where{Exprs: []clause.Expression{
			clause.Expr{SQL: touch, Vars: []any{now.Add(-chatTouchInterval)}},
		}},
	}).Create(chat)
	if upsert.Error != nil {
		log.Errorf("[Database] TouchChat upsert failed for chat %d: %v", chatId, upsert.Error)
		return upsert.Error
	}
	if upsert.RowsAffected > 0 {
		cache.DeleteCache(cache.CacheKey("chat", chatId))
	}
	log.Debugf("[Database] TouchChat: %d", chatId)
	return nil
}

// AddChatMember appends userid to the chat's users list if it is not there
// yet. The chat row must already exist (see TouchChat).
func AddChatMember(chatId, userid int64) error {
	if users, err := GetChatUsersCached(chatId); err == nil && slices.Contains(users, userid) {
		return nil
	}

	result := db.DB.Exec(
		`UPDATE chats SET users = users || to_jsonb(?::bigint) WHERE chat_id = ? AND NOT (users @> to_jsonb(?::bigint))`,
		userid, chatId, userid,
	)
	if result.Error != nil {
		log.Errorf("[Database] AddChatMember atomic append failed for chat %d user %d: %v", chatId, userid, result.Error)
		return result.Error
	}
	cache.DeleteCache(cache.CacheKey("chat_users", chatId))

	log.Debugf("[Database] AddChatMember: %d %d", chatId, userid)
	return nil
}

func GetAllChats() map[int64]models.Chat {
	var (
		chatArray []models.Chat
		chatMap   = make(map[int64]models.Chat)
	)
	err := db.DB.Find(&chatArray).Error
	if err != nil {
		log.Errorf("[Database] GetAllChats: %v", err)
		return chatMap
	}

	for _, i := range chatArray {
		chatMap[i.ChatId] = i
	}

	return chatMap
}

func LoadChatStats() (activeChats, inactiveChats int) {
	var activeCount, inactiveCount int64

	err := db.DB.Model(&models.Chat{}).Where("is_inactive = ?", false).Count(&activeCount).Error
	if err != nil {
		log.Errorf("[Database][LoadChatStats] counting active chats: %v", err)
	}

	err = db.DB.Model(&models.Chat{}).Where("is_inactive = ?", true).Count(&inactiveCount).Error
	if err != nil {
		log.Errorf("[Database][LoadChatStats] counting inactive chats: %v", err)
	}

	activeChats = int(activeCount)
	inactiveChats = int(inactiveCount)
	return
}

func LoadActivityStats() (dag, wag, mag int64) {
	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)
	weekAgo := now.Add(-7 * 24 * time.Hour)
	monthAgo := now.Add(-30 * 24 * time.Hour)

	err := db.DB.Model(&models.Chat{}).
		Where("is_inactive = ? AND last_activity >= ?", false, dayAgo).
		Count(&dag).Error
	if err != nil {
		log.Errorf("[Database][LoadActivityStats] counting daily active groups: %v", err)
	}

	err = db.DB.Model(&models.Chat{}).
		Where("is_inactive = ? AND last_activity >= ?", false, weekAgo).
		Count(&wag).Error
	if err != nil {
		log.Errorf("[Database][LoadActivityStats] counting weekly active groups: %v", err)
	}

	err = db.DB.Model(&models.Chat{}).
		Where("is_inactive = ? AND last_activity >= ?", false, monthAgo).
		Count(&mag).Error
	if err != nil {
		log.Errorf("[Database][LoadActivityStats] counting monthly active groups: %v", err)
	}

	return dag, wag, mag
}
