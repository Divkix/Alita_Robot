package lang

import (
	"errors"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/user"
)

// checkUserInfo is a local helper that delegates to user.GetUserBasicInfoCached.
func checkUserInfo(userId int64) (userc *models.User) {
	userc, err := user.GetUserBasicInfoCached(userId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		log.Errorf("[Database] checkUserInfo: %v - %d", err, userId)
		return &models.User{UserId: userId}
	}
	return userc
}

// GetLanguage determines the appropriate language for the current context.
// Returns the user's language preference for private chats, or the group's language for group chats.
// Defaults to "en" (English) if no preference is found.
func GetLanguage(ctx *ext.Context) string {
	if ctx == nil {
		return "en"
	}

	chat := ctx.EffectiveChat
	if chat == nil {
		// Fallback to default language if we can't determine chat context
		log.Warn("[GetLanguage] Unable to determine chat context, using default language")
		return "en"
	}

	if chat.Type == "private" {
		// Guard against nil EffectiveSender
		if ctx.EffectiveSender == nil {
			log.Debug("[GetLanguage] No sender in private chat context, using default language")
			return "en"
		}
		user := ctx.EffectiveSender.User
		if user == nil {
			return "en"
		}
		return getUserLanguage(user.Id)
	}
	return getGroupLanguage(chat.Id)
}

// getGroupLanguage retrieves the language preference for a specific group.
// Uses caching to improve performance and defaults to "en" if no preference is set.
func getGroupLanguage(GroupID int64) string {
	// Try to get from cache first
	cacheKey := cache.CacheKey("chat_lang", GroupID)
	lang, err := cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLLanguage, func() (string, error) {
		groupc := chats.GetChatSettings(GroupID)
		if groupc.Language == "" {
			return "en", nil
		}
		return groupc.Language, nil
	})
	if err != nil {
		return "en"
	}
	return lang
}

// getUserLanguage retrieves the language preference for a specific user.
// Uses caching to improve performance and defaults to "en" if no preference is set.
func getUserLanguage(UserID int64) string {
	// Try to get from cache first
	cacheKey := cache.CacheKey("user_lang", UserID)
	lang, err := cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLLanguage, func() (string, error) {
		userc := checkUserInfo(UserID)
		if userc == nil {
			return "en", nil
		} else if userc.Language == "" {
			return "en", nil
		}
		return userc.Language, nil
	})
	if err != nil {
		return "en"
	}
	return lang
}

// ChangeUserLanguage updates the language preference for a specific user.
// Creates the user with the specified language if they don't exist.
// Does nothing if the language is already set to the specified value.
// Invalidates the user language cache after successful update.
func ChangeUserLanguage(UserID int64, lang string) error {
	userc := checkUserInfo(UserID)
	if userc == nil {
		// Create new user with the specified language
		newUser := &models.User{
			UserId:   UserID,
			Language: lang,
		}
		err := db.DB.Create(newUser).Error
		if err != nil {
			log.Errorf("[Database] ChangeUserLanguage (create): %v - %d", err, UserID)
			return err
		}
		// Invalidate both language cache and optimized query cache after create
		cache.DeleteCache(cache.CacheKey("user_lang", UserID))
		cache.DeleteCache(cache.CacheKey("user", UserID))
		log.Infof("[Database] ChangeUserLanguage: created new user %d with language %s", UserID, lang)
		return nil
	} else if userc.Language == lang {
		return nil
	}

	err := db.UpdateRecord(&models.User{}, models.User{UserId: UserID}, models.User{Language: lang})
	if err != nil {
		log.Errorf("[Database] ChangeUserLanguage: %v - %d", err, UserID)
		return err
	}
	// Invalidate both language cache and optimized query cache after update
	cache.DeleteCache(cache.CacheKey("user_lang", UserID))
	cache.DeleteCache(cache.CacheKey("user", UserID))
	log.Infof("[Database] ChangeUserLanguage: %d", UserID)
	return nil
}

// ChangeGroupLanguage updates the language preference for a specific group.
// Creates the chat with the specified language if it doesn't exist.
// Does nothing if the language is already set to the specified value.
// Invalidates both the group language and chat settings caches after successful update.
func ChangeGroupLanguage(GroupID int64, lang string) error {
	groupc := chats.GetChatSettings(GroupID)

	// Check if chat exists (GetChatSettings returns empty struct if not found)
	if groupc.ChatId == 0 {
		// Create new chat with the specified language
		newChat := &models.Chat{
			ChatId:   GroupID,
			Language: lang,
		}
		err := db.DB.Create(newChat).Error
		if err != nil {
			log.Errorf("[Database] ChangeGroupLanguage (create): %v - %d", err, GroupID)
			return err
		}
		// Invalidate all cache layers after create
		cache.DeleteCache(cache.CacheKey("chat_lang", GroupID))
		cache.DeleteCache(cache.CacheKey("chat_settings", GroupID))
		cache.DeleteCache(cache.CacheKey("chat", GroupID))
		log.Infof("[Database] ChangeGroupLanguage: created new chat %d with language %s", GroupID, lang)
		return nil
	} else if groupc.Language == lang {
		return nil
	}

	err := db.UpdateRecord(&models.Chat{}, models.Chat{ChatId: GroupID}, models.Chat{Language: lang})
	if err != nil {
		log.Errorf("[Database] ChangeGroupLanguage: %v - %d", err, GroupID)
		return err
	}
	// Invalidate all cache layers after update
	cache.DeleteCache(cache.CacheKey("chat_lang", GroupID))
	cache.DeleteCache(cache.CacheKey("chat_settings", GroupID)) // Also invalidate chat settings cache since language is part of it
	cache.DeleteCache(cache.CacheKey("chat", GroupID))
	log.Infof("[Database] ChangeGroupLanguage: %d", GroupID)
	return nil
}
