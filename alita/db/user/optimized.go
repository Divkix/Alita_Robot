package user

import (
	"context"
	"errors"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func GetUserBasicInfo(userID int64) (*models.User, error) {
	return GetUserBasicInfoContext(context.Background(), userID)
}

func GetUserBasicInfoContext(ctx context.Context, userID int64) (*models.User, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}

	var user models.User
	err := db.DB.WithContext(ctx).Model(&models.User{}).
		Select("id, user_id, username, name, language, last_activity").
		Where("user_id = ?", userID).
		First(&user).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Errorf("[user.GetUserBasicInfo] GetUserBasicInfo: %v", err)
	}

	return &user, err
}

func GetUserBasicInfoCached(userID int64) (*models.User, error) {
	return GetUserBasicInfoCachedContext(context.Background(), userID)
}

func GetUserBasicInfoCachedContext(ctx context.Context, userID int64) (*models.User, error) {
	cacheKey := cache.CacheKey("user", userID)

	cached, err := cache.GetFromCacheOrLoad(ctx, cacheKey, 1*time.Hour, func(ctx context.Context) (*models.User, error) {
		user, err := GetUserBasicInfoContext(ctx, userID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &models.User{UserId: -9999}, nil
		}
		return user, err
	})
	if err != nil {
		return nil, err
	}

	if cached != nil && cached.UserId == -9999 {
		return nil, gorm.ErrRecordNotFound
	}

	return cached, nil
}
