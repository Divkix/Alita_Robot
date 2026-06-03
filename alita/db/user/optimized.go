package user

import (
	"errors"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// GetUserBasicInfo retrieves only essential user information with minimal column selection.
// Optimized for high-frequency calls (61K+ calls) by selecting only necessary fields.
func GetUserBasicInfo(userID int64) (*models.User, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}

	var user models.User
	err := db.DB.Model(&models.User{}).
		Select("id, user_id, username, name, language, last_activity").
		Where("user_id = ?", userID).
		First(&user).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Errorf("[OptimizedUserQueries] GetUserBasicInfo: %v", err)
	}

	return &user, err
}

// GetUserBasicInfoCached retrieves user information with caching layer for improved performance.
// Uses 1-hour cache TTL and falls back to direct query if cache fails.
func GetUserBasicInfoCached(userID int64) (*models.User, error) {
	cacheKey := cache.CacheKey("user", userID)

	cached, err := cache.GetFromCacheOrLoad(cacheKey, 1*time.Hour, func() (*models.User, error) {
		return GetUserBasicInfo(userID)
	})
	if err != nil {
		return GetUserBasicInfo(userID)
	}

	return cached, nil
}
