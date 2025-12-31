package db

import (
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"
)

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
	cacheKey := chatLanguageCacheKey(GroupID)
	lang, err := getFromCacheOrLoad(cacheKey, CacheTTLLanguage, func() (string, error) {
		groupc := GetChatSettings(GroupID)
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
	cacheKey := userLanguageCacheKey(UserID)
	lang, err := getFromCacheOrLoad(cacheKey, CacheTTLLanguage, func() (string, error) {
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
func ChangeUserLanguage(UserID int64, lang string) {
	userc := checkUserInfo(UserID)
	if userc == nil {
		// Create new user with the specified language
		newUser := &User{
			UserId:   UserID,
			Language: lang,
		}
		err := DB.Create(newUser).Error
		if err != nil {
			log.Errorf("[Database] ChangeUserLanguage (create): %v - %d", err, UserID)
			return
		}
		// Invalidate both language cache and optimized query cache after create
		deleteCache(userLanguageCacheKey(UserID))
		deleteCache(userCacheKey(UserID))
		log.Infof("[Database] ChangeUserLanguage: created new user %d with language %s", UserID, lang)
		return
	} else if userc.Language == lang {
		return
	}

	err := UpdateRecord(&User{}, User{UserId: UserID}, User{Language: lang})
	if err != nil {
		log.Errorf("[Database] ChangeUserLanguage: %v - %d", err, UserID)
		return
	}
	// Invalidate both language cache and optimized query cache after update
	deleteCache(userLanguageCacheKey(UserID))
	deleteCache(userCacheKey(UserID))
	log.Infof("[Database] ChangeUserLanguage: %d", UserID)
}

// ChangeGroupLanguage updates the language preference for a specific group.
// Creates the chat with the specified language if it doesn't exist.
// Does nothing if the language is already set to the specified value.
// Invalidates both the group language and chat settings caches after successful update.
func ChangeGroupLanguage(GroupID int64, lang string) {
	groupc := GetChatSettings(GroupID)

	// Check if chat exists (GetChatSettings returns empty struct if not found)
	if groupc.ChatId == 0 {
		// Create new chat with the specified language
		newChat := &Chat{
			ChatId:   GroupID,
			Language: lang,
		}
		err := DB.Create(newChat).Error
		if err != nil {
			log.Errorf("[Database] ChangeGroupLanguage (create): %v - %d", err, GroupID)
			return
		}
		// Invalidate all cache layers after create
		deleteCache(chatLanguageCacheKey(GroupID))
		deleteCache(chatSettingsCacheKey(GroupID))
		deleteCache(chatCacheKey(GroupID))
		log.Infof("[Database] ChangeGroupLanguage: created new chat %d with language %s", GroupID, lang)
		return
	} else if groupc.Language == lang {
		return
	}

	err := UpdateRecord(&Chat{}, Chat{ChatId: GroupID}, Chat{Language: lang})
	if err != nil {
		log.Errorf("[Database] ChangeGroupLanguage: %v - %d", err, GroupID)
		return
	}
	// Invalidate all cache layers after update
	deleteCache(chatLanguageCacheKey(GroupID))
	deleteCache(chatSettingsCacheKey(GroupID)) // Also invalidate chat settings cache since language is part of it
	deleteCache(chatCacheKey(GroupID))
	log.Infof("[Database] ChangeGroupLanguage: %d", GroupID)
}
