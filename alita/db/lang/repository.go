package lang

import (
	"errors"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"

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
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil
		}
		log.Errorf("[Database] checkUserInfo: %v - %d", err, userId)
		return &models.User{UserId: userId}
	}
	return userc
}

// GetLanguage determines the appropriate language for the current context.
func GetLanguage(ctx *ext.Context) string {
	if ctx == nil {
		return "en"
	}

	chat := ctx.EffectiveChat
	if chat == nil {
		log.Warn("[GetLanguage] Unable to determine chat context, using default language")
		return "en"
	}

	if chat.Type == "private" {
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
func getGroupLanguage(GroupID int64) string {
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
func getUserLanguage(UserID int64) string {
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
func ChangeUserLanguage(UserID int64, lang string) error {
	userc := checkUserInfo(UserID)
	if userc == nil {
		// Create new user with the specified language
		newUser := &models.User{
			UserId:    UserID,
			Language:  lang,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := db.CreateRecord(newUser)
		if err != nil {
			log.Errorf("[Database] ChangeUserLanguage (create): %v - %d", err, UserID)
			return err
		}
		// Invalidate caches
		cache.DeleteCache(cache.CacheKey("user_lang", UserID))
		cache.DeleteCache(cache.CacheKey("user", UserID))
		log.Infof("[Database] ChangeUserLanguage: created new user %d with language %s", UserID, lang)
		return nil
	} else if userc.Language == lang {
		return nil
	}

	err := db.UpdateRecord(&models.User{}, bson.M{"user_id": UserID}, map[string]any{"language": lang, "updated_at": time.Now()})
	if err != nil {
		log.Errorf("[Database] ChangeUserLanguage: %v - %d", err, UserID)
		return err
	}
	// Invalidate caches
	cache.DeleteCache(cache.CacheKey("user_lang", UserID))
	cache.DeleteCache(cache.CacheKey("user", UserID))
	log.Infof("[Database] ChangeUserLanguage: %d", UserID)
	return nil
}

// ChangeGroupLanguage updates the language preference for a specific group.
func ChangeGroupLanguage(GroupID int64, lang string) error {
	groupc := chats.GetChatSettings(GroupID)

	if groupc.ChatId == 0 {
		// Create new chat with the specified language
		newChat := &models.Chat{
			ChatId:    GroupID,
			Language:  lang,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := db.CreateRecord(newChat)
		if err != nil {
			log.Errorf("[Database] ChangeGroupLanguage (create): %v - %d", err, GroupID)
			return err
		}
		// Invalidate caches
		cache.DeleteCache(cache.CacheKey("chat_lang", GroupID))
		cache.DeleteCache(cache.CacheKey("chat_settings", GroupID))
		cache.DeleteCache(cache.CacheKey("chat", GroupID))
		log.Infof("[Database] ChangeGroupLanguage: created new chat %d with language %s", GroupID, lang)
		return nil
	} else if groupc.Language == lang {
		return nil
	}

	err := db.UpdateRecord(&models.Chat{}, bson.M{"chat_id": GroupID}, map[string]any{"language": lang, "updated_at": time.Now()})
	if err != nil {
		log.Errorf("[Database] ChangeGroupLanguage: %v - %d", err, GroupID)
		return err
	}
	// Invalidate caches
	cache.DeleteCache(cache.CacheKey("chat_lang", GroupID))
	cache.DeleteCache(cache.CacheKey("chat_settings", GroupID))
	cache.DeleteCache(cache.CacheKey("chat", GroupID))
	log.Infof("[Database] ChangeGroupLanguage: %d", GroupID)
	return nil
}
