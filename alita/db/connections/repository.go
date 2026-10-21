package connections

import (
	"context"
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/user"
)

func ToggleAllowConnect(chatID int64, pref bool) error {
	if _, err := checkChatConnectionSetting(chatID); err != nil {
		log.Errorf("[Database] ToggleAllowConnect: ensure settings for %d: %v", chatID, err)
	}
	err := db.UpdateRecordWithZeroValues(&models.ConnectionChatSettings{}, models.ConnectionChatSettings{ChatId: chatID}, map[string]any{"allow_connect": pref})
	if err != nil {
		log.Errorf("[Database] ToggleAllowConnect: %d - %v", chatID, err)
		return err
	}
	cache.DeleteCache(cache.CacheKey("connection_chat", chatID))
	return nil
}

func GetChatConnectionSetting(chatID int64) *models.ConnectionChatSettings {
	connectionSrc, err := cache.GetFromCacheOrLoad(
		context.Background(),
		cache.CacheKey("connection_chat", chatID),
		cache.CacheTTLConnections,
		func(context.Context) (*models.ConnectionChatSettings, error) {
			return checkChatConnectionSetting(chatID)
		},
	)
	if err != nil || connectionSrc == nil {
		connectionSrc, _ = checkChatConnectionSetting(chatID)
	}
	return connectionSrc
}

func checkChatConnectionSetting(chatID int64) (*models.ConnectionChatSettings, error) {
	connectionSrc := &models.ConnectionChatSettings{}
	err := db.GetRecord(connectionSrc, models.ConnectionChatSettings{ChatId: chatID})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := chats.EnsureChatInDb(chatID, ""); err != nil {
			log.Errorf("[Database] GetChatConnectionSetting: Failed to ensure chat exists for %d: %v", chatID, err)
			return &models.ConnectionChatSettings{ChatId: chatID, AllowConnect: false}, err
		}

		connectionSrc = &models.ConnectionChatSettings{ChatId: chatID, AllowConnect: false}
		if err := db.CreateRecord(connectionSrc); err != nil {
			log.Errorf("[Database] GetChatConnectionSetting: %d - %v", chatID, err)
			return connectionSrc, err
		}
		return connectionSrc, nil
	}
	if err != nil {
		log.Errorf("[Database] GetChatConnectionSetting: %d - %v", chatID, err)
		return &models.ConnectionChatSettings{ChatId: chatID, AllowConnect: false}, err
	}
	return connectionSrc, nil
}

func getUserConnectionSetting(userID int64) (*models.ConnectionSettings, error) {
	connectionSrc := &models.ConnectionSettings{}
	err := db.GetRecord(connectionSrc, models.ConnectionSettings{UserId: userID})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &models.ConnectionSettings{UserId: userID, Connected: false}, nil
	}
	if err != nil {
		log.Errorf("[Database] getUserConnectionSetting: %d - %v", userID, err)
		return &models.ConnectionSettings{UserId: userID, Connected: false}, err
	}

	return connectionSrc, nil
}

func Connection(UserID int64) *models.ConnectionSettings {
	connectionSrc, err := cache.GetFromCacheOrLoad(
		context.Background(),
		cache.CacheKey("connection", UserID),
		cache.CacheTTLConnections,
		func(context.Context) (*models.ConnectionSettings, error) {
			return getUserConnectionSetting(UserID)
		},
	)
	if err != nil || connectionSrc == nil {
		connectionSrc, _ = getUserConnectionSetting(UserID)
	}
	return connectionSrc
}

func ConnectId(UserID, chatID int64) error {
	if chatID == 0 {
		err := fmt.Errorf("invalid chat ID %d", chatID)
		log.WithField("userID", UserID).Warningf("[Database] ConnectId: %v", err)
		return err
	}
	if err := chats.EnsureChatInDb(chatID, ""); err != nil {
		return err
	}
	if err := user.EnsureUserInDb(UserID, "", ""); err != nil {
		return err
	}

	connection := &models.ConnectionSettings{UserId: UserID, ChatId: chatID, Connected: true}
	err := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"chat_id", "connected", "updated_at"}),
	}).Create(connection).Error
	if err != nil {
		log.Errorf("[Database] ConnectId: %v - %d", err, chatID)
		return err
	}
	cache.DeleteCache(cache.CacheKey("connection", UserID))
	return nil
}

func DisconnectId(UserID int64) error {
	err := db.DB.Model(&models.ConnectionSettings{}).
		Where("user_id = ?", UserID).
		Update("connected", false).Error
	if err != nil {
		log.Errorf("[Database] DisconnectId: %v - %d", err, UserID)
		return err
	}
	cache.DeleteCache(cache.CacheKey("connection", UserID))
	return nil
}

func LoadConnectionStats() (connectedUsers, connectedChats int64) {
	err := db.DB.Model(&models.ConnectionChatSettings{}).Where("allow_connect = ?", true).Count(&connectedChats).Error
	if err != nil {
		log.Error(err)
		return
	}

	err = db.DB.Model(&models.ConnectionSettings{}).Where("connected = ?", true).Count(&connectedUsers).Error
	if err != nil {
		log.Error(err)
		return
	}

	return
}
