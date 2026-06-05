package user

import (
	"context"
	"errors"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetUserBasicInfo retrieves only essential user information with minimal column selection.
// Optimized for high-frequency calls (61K+ calls) by selecting only necessary fields.
func GetUserBasicInfo(userID int64) (*models.User, error) {
	if db.DB == nil {
		return nil, errors.New("database not initialized")
	}

	var user models.User
	collection := db.DB.Collection("users")
	err := collection.FindOne(context.Background(), bson.M{"user_id": userID}, options.FindOne().SetProjection(bson.M{
		"user_id":       1,
		"username":      1,
		"name":          1,
		"language":      1,
		"last_activity": 1,
	})).Decode(&user)

	if err != nil && err != db.ErrRecordNotFound {
		log.Errorf("[user.GetUserBasicInfo] GetUserBasicInfo: %v", err)
	}

	return &user, err
}

// GetUserBasicInfoCached retrieves user information with caching layer for improved performance.
// Uses 1-hour cache TTL and falls back to direct query if cache fails.
func GetUserBasicInfoCached(userID int64) (*models.User, error) {
	cacheKey := cache.CacheKey("user", userID)

	cached, err := cache.GetFromCacheOrLoad(cacheKey, 1*time.Hour, func() (*models.User, error) {
		user, err := GetUserBasicInfo(userID)
		if errors.Is(err, db.ErrRecordNotFound) {
			return &models.User{UserId: -9999}, nil
		}
		return user, err
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil, db.ErrRecordNotFound
		}
		return GetUserBasicInfo(userID)
	}

	if cached != nil && cached.UserId == -9999 {
		return nil, db.ErrRecordNotFound
	}

	return cached, nil
}
