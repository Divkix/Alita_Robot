package cache

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/eko/gocache/lib/v4/store"

	"github.com/divkix/Alita_Robot/alita/utils/constants"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
)

// LoadAdminCache retrieves and caches the list of administrators for a given chat.
// It fetches the current administrators from Telegram API and stores them in cache
// with a 30-minute expiration. Returns an AdminCache struct containing the admin list.
func LoadAdminCache(b *gotgbot.Bot, chatId int64) AdminCache {
	if b == nil {
		log.Error("LoadAdminCache: bot is nil")
		return AdminCache{}
	}

	// Create context with timeout to prevent indefinite blocking
	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultTimeout)
	defer cancel()

	// First, check if bot itself is admin to diagnose permission issues
	botMember, botErr := b.GetChatMemberWithContext(ctx, chatId, b.Id, nil)
	if botErr != nil {
		log.WithFields(log.Fields{
			"chatId": chatId,
			"botId":  b.Id,
			"error":  botErr,
		}).Warning("LoadAdminCache: Could not verify bot admin status")
		// If we can't even check bot status, likely not admin - return empty cache
		return AdminCache{
			ChatId:   chatId,
			UserInfo: []gotgbot.MergedChatMember{},
			Cached:   true,
		}
	}

	botStatus := botMember.GetStatus()
	if botStatus != "administrator" && botStatus != "creator" {
		return AdminCache{
			ChatId:   chatId,
			UserInfo: []gotgbot.MergedChatMember{},
			Cached:   true,
		}
	}

	log.WithFields(log.Fields{
		"chatId":    chatId,
		"botId":     b.Id,
		"botStatus": botStatus,
	}).Debug("LoadAdminCache: Bot has admin privileges")

	// Retry logic for API call
	maxRetries := 3
	var adminList []gotgbot.ChatMember
	var err error

	for attempt := 0; attempt < maxRetries; attempt++ {
		adminList, err = b.GetChatAdministratorsWithContext(ctx, chatId, nil)
		if err != nil {
			log.WithFields(log.Fields{
				"chatId":    chatId,
				"error":     err,
				"attempt":   attempt + 1,
				"errorType": fmt.Sprintf("%T", err),
			}).Warning("LoadAdminCache: Failed to get chat administrators, retrying...")

			if attempt < maxRetries-1 {
				time.Sleep(time.Duration(attempt+1) * time.Second) // Exponential backoff
				continue
			}

			log.WithFields(log.Fields{
				"chatId": chatId,
				"error":  err,
			}).Error("LoadAdminCache: Failed to get chat administrators after all retries")
			return AdminCache{}
		}
		break // Success
	}

	if len(adminList) == 0 {
		log.WithFields(log.Fields{
			"chatId": chatId,
		}).Warning("LoadAdminCache: No administrators found - this is unusual for a valid group")
		// Empty admin list is unusual but not necessarily an error
		// Return empty cache but mark it as cached to avoid infinite retries
		return AdminCache{
			ChatId:   chatId,
			UserInfo: []gotgbot.MergedChatMember{},
			Cached:   true,
		}
	}

	// Convert ChatMember to MergedChatMember
	var userList []gotgbot.MergedChatMember
	for _, admin := range adminList {
		userList = append(userList, admin.MergeChatMember())
	}

	adminCache := AdminCache{
		ChatId:   chatId,
		UserInfo: userList,
		Cached:   true,
	}

	// Cache the admin list with retry on failure in background
	go func() {
		defer error_handling.RecoverFromPanic("LoadAdminCache.cacheRoutine", "adminCache")
		maxRetries := 3
		for i := range maxRetries {
			if err := Marshal.Set(Context, fmt.Sprintf("alita:adminCache:%d", chatId), adminCache, store.WithExpiration(constants.AdminCacheTTL)); err != nil {
				log.WithFields(log.Fields{
					"chatId": chatId,
					"error":  err,
					"retry":  i + 1,
				}).Error("LoadAdminCache: Failed to cache admin list")

				if i < maxRetries-1 {
					time.Sleep(time.Second * 2) // Wait before retry
					continue
				}
			} else {
				log.WithFields(log.Fields{
					"chatId": chatId,
				}).Debug("LoadAdminCache: Successfully cached admin list")
				break
			}
		}
	}()

	return adminCache
}

// GetAdminCacheList retrieves the cached administrator list for a specific chat.
// Returns true and the AdminCache if found in cache, false and empty AdminCache if cache miss.
func GetAdminCacheList(chatId int64) (bool, AdminCache) {
	gotAdminlist, err := Marshal.Get(
		Context,
		fmt.Sprintf("alita:adminCache:%d", chatId),
		new(AdminCache),
	)
	if err != nil {
		log.WithFields(log.Fields{
			"chatId": chatId,
			"error":  err,
		}).Debug("GetAdminCacheList: Cache miss, will attempt fallback")
		return false, AdminCache{}
	}
	if gotAdminlist == nil {
		log.WithFields(log.Fields{
			"chatId": chatId,
		}).Debug("GetAdminCacheList: Cache empty, will attempt fallback")
		return false, AdminCache{}
	}
	return true, *gotAdminlist.(*AdminCache)
}

// GetAdminCacheUser searches for a specific user in the cached administrator list of a chat.
// Returns true and the MergedChatMember if the user is found as an admin,
// false and empty MergedChatMember if not found or cache miss.
func GetAdminCacheUser(chatId, userId int64) (bool, gotgbot.MergedChatMember) {
	// Use consistent string key format matching LoadAdminCache
	adminList, err := Marshal.Get(Context, fmt.Sprintf("alita:adminCache:%d", chatId), new(AdminCache))
	if err != nil || adminList == nil {
		return false, gotgbot.MergedChatMember{}
	}

	// Type assert with check to prevent panic
	adminCache, ok := adminList.(*AdminCache)
	if !ok || adminCache == nil {
		return false, gotgbot.MergedChatMember{}
	}

	for i := range adminCache.UserInfo {
		admin := &adminCache.UserInfo[i]
		if admin.User.Id == userId {
			return true, *admin
		}
	}
	return false, gotgbot.MergedChatMember{}
}

// InvalidateAdminCache removes the cached admin list for a chat.
// Should be called when admins are promoted/demoted to ensure fresh data.
func InvalidateAdminCache(chatId int64) {
	cacheKey := fmt.Sprintf("alita:adminCache:%d", chatId)
	if err := Marshal.Delete(Context, cacheKey); err != nil {
		log.Debugf("[AdminCache] Failed to invalidate cache for chat %d: %v", chatId, err)
	} else {
		log.Debugf("[AdminCache] Invalidated admin cache for chat %d", chatId)
	}
}
