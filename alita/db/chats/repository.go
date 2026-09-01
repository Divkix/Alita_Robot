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

// chatTouchInterval bounds how often an incoming message may refresh
// chats.last_activity. The inactivity sweep works in days, so hourly
// granularity is indistinguishable downstream.
const chatTouchInterval = time.Hour

// GetChatSettings retrieves chat settings using optimized cached queries.
// Returns an empty Chat struct if not found or on error.
func GetChatSettings(chatId int64) (chatSrc *models.Chat) {
	// Use optimized cached query instead of SELECT *
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

// EnsureChatInDb ensures that a chat exists in the database.
// Creates the chat record if it doesn't exist, or updates it if it does.
// This is essential for foreign key constraints that reference the chats table.
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

// UpdateChat updates or creates a chat record with the given information.
// Adds user to the chat's user list atomically if not already present, marks chat as active,
// and updates the last activity timestamp to track when messages are received.
// Returns error if database operation fails.
func UpdateChat(chatId int64, chatname string, userid int64) error {
	now := time.Now()

	columns := []string{"is_inactive", "last_activity", "updated_at"}
	if chatname != "" {
		columns = append(columns, "chat_name")
	}
	chat := &models.Chat{
		ChatId:       chatId,
		ChatName:     chatname,
		Users:        models.Int64Array{userid},
		IsInactive:   false,
		LastActivity: now,
	}
	// Refresh an existing row only when the stored timestamp is stale, the chat
	// needs reactivating, or its name changed. Inactivity is measured in days,
	// so rewriting the row (and its indexes) on every message bought nothing.
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
		log.Errorf("[Database] UpdateChat upsert failed for chat %d: %v", chatId, upsert.Error)
		return upsert.Error
	}
	// A throttled no-op leaves the cached copy accurate, so only evict after a
	// real write; evicting unconditionally would cold-start the lookup below.
	if upsert.RowsAffected > 0 {
		cache.DeleteCache(cache.CacheKey("chat", chatId))
	}

	// Fast path: skip the atomic append when the user is already a member
	// (99.3% of calls). Uses the cached chat-users list (30 min TTL).
	if users, err := GetChatUsersCached(chatId); err == nil && slices.Contains(users, userid) {
		return nil
	}

	// Atomically append userid only if not already present in the JSON array
	result := db.DB.Exec(
		`UPDATE chats SET users = users || to_jsonb(?::bigint) WHERE chat_id = ? AND NOT (users @> to_jsonb(?::bigint))`,
		userid, chatId, userid,
	)
	if result.Error != nil {
		log.Errorf("[Database] UpdateChat atomic append failed for chat %d user %d: %v", chatId, userid, result.Error)
		return result.Error
	}
	// Reaching here means the cached list lacked this user, so evict either way:
	// the append added them, or the row already had them and the entry is stale.
	cache.DeleteCache(cache.CacheKey("chat_users", chatId))

	log.Debugf("[Database] UpdateChat: %d", chatId)
	return nil
}

// GetAllChats retrieves all chat records and returns them as a map indexed by chat ID.
// Returns an empty map if an error occurs.
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

// LoadChatStats returns the count of active and inactive chats.
// Active chats have is_inactive = false, inactive chats have is_inactive = true.
func LoadChatStats() (activeChats, inactiveChats int) {
	var activeCount, inactiveCount int64

	// Count active chats
	err := db.DB.Model(&models.Chat{}).Where("is_inactive = ?", false).Count(&activeCount).Error
	if err != nil {
		log.Errorf("[Database][LoadChatStats] counting active chats: %v", err)
	}

	// Count inactive chats
	err = db.DB.Model(&models.Chat{}).Where("is_inactive = ?", true).Count(&inactiveCount).Error
	if err != nil {
		log.Errorf("[Database][LoadChatStats] counting inactive chats: %v", err)
	}

	activeChats = int(activeCount)
	inactiveChats = int(inactiveCount)
	return
}

// LoadActivityStats returns Daily Active Groups, Weekly Active Groups, and Monthly Active Groups.
// These metrics are based on last_activity timestamps within the respective time periods.
func LoadActivityStats() (dag, wag, mag int64) {
	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)
	weekAgo := now.Add(-7 * 24 * time.Hour)
	monthAgo := now.Add(-30 * 24 * time.Hour)

	// Count daily active groups
	err := db.DB.Model(&models.Chat{}).
		Where("is_inactive = ? AND last_activity >= ?", false, dayAgo).
		Count(&dag).Error
	if err != nil {
		log.Errorf("[Database][LoadActivityStats] counting daily active groups: %v", err)
	}

	// Count weekly active groups
	err = db.DB.Model(&models.Chat{}).
		Where("is_inactive = ? AND last_activity >= ?", false, weekAgo).
		Count(&wag).Error
	if err != nil {
		log.Errorf("[Database][LoadActivityStats] counting weekly active groups: %v", err)
	}

	// Count monthly active groups
	err = db.DB.Model(&models.Chat{}).
		Where("is_inactive = ? AND last_activity >= ?", false, monthAgo).
		Count(&mag).Error
	if err != nil {
		log.Errorf("[Database][LoadActivityStats] counting monthly active groups: %v", err)
	}

	return dag, wag, mag
}
