package cache

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/singleflight"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/eko/gocache/lib/v4/store"

	"github.com/divkix/Alita_Robot/alita/utils/constants"
)

var adminCacheGroup singleflight.Group

func adminCacheKey(chatId int64) string {
	return fmt.Sprintf("alita:adminCache:%d", chatId)
}

// LoadAdminCache retrieves and caches the list of administrators for a given chat.
// It fetches the current administrators from Telegram API and stores them in cache
// with a 30-minute expiration. Returns an AdminCache struct containing the admin list.
// Concurrent callers for the same chat share one in-flight Telegram fetch.
// getChatAdministrators is called with ReturnBots so other administrator bots
// are included; without it Telegram omits them and IsUserAdmin treats them as
// regular members.
func LoadAdminCache(b *gotgbot.Bot, chatId int64) AdminCache {
	if b == nil {
		log.Error("LoadAdminCache: bot is nil")
		return AdminCache{}
	}

	v, _, _ := adminCacheGroup.Do(adminCacheKey(chatId), func() (any, error) {
		return loadAdminCacheFromTelegram(b, chatId), nil
	})
	ac, _ := v.(AdminCache)
	return ac
}

func loadAdminCacheFromTelegram(b *gotgbot.Bot, chatId int64) AdminCache {
	const negativeAdminCacheTTL = 2 * time.Minute
	const botStatusErrorAdminCacheTTL = 30 * time.Second

	storeWithTTL := func(adminCache AdminCache, ttl time.Duration) AdminCache {
		if m := GetMarshal(); m != nil {
			if err := m.Set(Context, adminCacheKey(chatId), adminCache, store.WithExpiration(ttl)); err != nil {
				log.WithFields(log.Fields{
					"chatId": chatId,
					"error":  err,
				}).Error("LoadAdminCache: Failed to cache admin list")
			}
		}
		return adminCache
	}
	storeResult := func(ac AdminCache) AdminCache {
		return storeWithTTL(ac, constants.AdminCacheTTL)
	}
	storeNegativeResult := func(ac AdminCache) AdminCache {
		return storeWithTTL(ac, negativeAdminCacheTTL)
	}

	// Check if bot itself is admin with a dedicated timeout context
	botCtx, botCancel := context.WithTimeout(context.Background(), constants.DefaultTimeout)
	botMember, botErr := b.GetChatMemberWithContext(botCtx, chatId, b.Id, nil)
	botCancel()
	if botErr != nil {
		log.WithFields(log.Fields{
			"chatId": chatId,
			"botId":  b.Id,
			"error":  botErr,
		}).Warning("LoadAdminCache: Could not verify bot admin status")
		// Store short-TTL negative to damp storm but allow quick retry on flaky API
		return storeWithTTL(AdminCache{
			ChatId:   chatId,
			UserInfo: []gotgbot.MergedChatMember{},
			Cached:   true,
		}, botStatusErrorAdminCacheTTL)
	}

	botStatus := botMember.GetStatus()
	if botStatus != "administrator" && botStatus != "creator" {
		return storeNegativeResult(AdminCache{
			ChatId:   chatId,
			UserInfo: []gotgbot.MergedChatMember{},
			Cached:   true,
		})
	}

	// Bot has admin rights — clear any stale restricted flag so sends are
	// no longer short-circuited for this chat.
	MarkChatNotRestricted(chatId)

	log.WithFields(log.Fields{
		"chatId":    chatId,
		"botId":     b.Id,
		"botStatus": botStatus,
	}).Debug("LoadAdminCache: Bot has admin privileges")

	// Retry logic for API call — per-attempt timeout to avoid deadline exceeded after sleeps
	maxRetries := 3
	var adminList []gotgbot.ChatMember
	var err error

	for attempt := 0; attempt < maxRetries; attempt++ {
		attemptCtx, attemptCancel := context.WithTimeout(context.Background(), constants.DefaultTimeout)
		adminList, err = b.GetChatAdministratorsWithContext(
			attemptCtx,
			chatId,
			&gotgbot.GetChatAdministratorsOpts{ReturnBots: true},
		)
		attemptCancel()
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
		return storeResult(AdminCache{
			ChatId:   chatId,
			UserInfo: []gotgbot.MergedChatMember{},
			Cached:   true,
		})
	}

	// Convert ChatMember to MergedChatMember and build lookup map
	userList := make([]gotgbot.MergedChatMember, 0, len(adminList))
	userMap := make(map[int64]gotgbot.MergedChatMember, len(adminList))
	for _, admin := range adminList {
		merged := admin.MergeChatMember()
		userList = append(userList, merged)
		// GetUser returns User by value, so check if ID is valid (non-zero)
		user := admin.GetUser()
		if user.Id != 0 {
			userMap[user.Id] = merged
		}
	}

	adminCache := AdminCache{
		ChatId:   chatId,
		UserInfo: userList,
		UserMap:  userMap,
		Cached:   true,
	}

	return storeResult(adminCache)
}

// GetAdminCacheList retrieves the cached administrator list for a specific chat.
// Returns true and the AdminCache if found in cache, false and empty AdminCache if cache miss.
func GetAdminCacheList(chatId int64) (bool, AdminCache) {
	m := GetMarshal()
	if m == nil {
		return false, AdminCache{}
	}
	gotAdminlist, err := m.Get(
		Context,
		adminCacheKey(chatId),
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
	m := GetMarshal()
	if m == nil {
		return false, gotgbot.MergedChatMember{}
	}
	// Use consistent string key format matching LoadAdminCache
	adminList, err := m.Get(Context, adminCacheKey(chatId), new(AdminCache))
	if err != nil || adminList == nil {
		return false, gotgbot.MergedChatMember{}
	}

	// Type assert with check to prevent panic
	adminCache, ok := adminList.(*AdminCache)
	if !ok || adminCache == nil {
		return false, gotgbot.MergedChatMember{}
	}

	// O(1) lookup via map (primary method)
	if admin, found := adminCache.UserMap[userId]; found {
		return true, admin
	}

	// Fallback to linear search for backwards compatibility (e.g., cached data without UserMap)
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
	m := GetMarshal()
	if m == nil {
		return
	}
	cacheKey := adminCacheKey(chatId)
	if err := m.Delete(Context, cacheKey); err != nil {
		log.Debugf("[AdminCache] Failed to invalidate cache for chat %d: %v", chatId, err)
	} else {
		log.Debugf("[AdminCache] Invalidated admin cache for chat %d", chatId)
	}
}
