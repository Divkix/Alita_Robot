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

func ChangeUserLanguage(UserID int64, lang string) error {
	userc := checkUserInfo(UserID)
	if userc == nil {
		newUser := &models.User{
			UserId:   UserID,
			Language: lang,
		}
		err := db.DB.Create(newUser).Error
		if err != nil {
			log.Errorf("[Database] ChangeUserLanguage (create): %v - %d", err, UserID)
			return err
		}
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
	cache.DeleteCache(cache.CacheKey("user_lang", UserID))
	cache.DeleteCache(cache.CacheKey("user", UserID))
	log.Infof("[Database] ChangeUserLanguage: %d", UserID)
	return nil
}

func ChangeGroupLanguage(GroupID int64, lang string) error {
	groupc := chats.GetChatSettings(GroupID)

	if groupc.ChatId == 0 {
		newChat := &models.Chat{
			ChatId:   GroupID,
			Language: lang,
		}
		err := db.DB.Create(newChat).Error
		if err != nil {
			log.Errorf("[Database] ChangeGroupLanguage (create): %v - %d", err, GroupID)
			return err
		}
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
	cache.DeleteCache(cache.CacheKey("chat_lang", GroupID))
	cache.DeleteCache(cache.CacheKey("chat_settings", GroupID))
	cache.DeleteCache(cache.CacheKey("chat", GroupID))
	log.Infof("[Database] ChangeGroupLanguage: %d", GroupID)
	return nil
}
