package user

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// EnsureBotInDb ensures that the bot's information is stored in the database.
// Creates or updates the bot's user record with current username and name.
// Returns error if database operation fails.
func EnsureBotInDb(b *gotgbot.Bot) error {
	// Ensure we have accurate bot identity from Telegram API.
	me, errGet := b.GetMe(nil)
	if errGet != nil {
		log.Errorf("[Database] EnsureBotInDb: failed to fetch bot identity via GetMe: %v", errGet)
		// Continue with whatever is available to avoid blocking startup.
	}

	botID := b.Id
	botUsername := b.Username
	botFirstName := b.FirstName
	if me != nil {
		botID = me.Id
		botUsername = me.Username
		botFirstName = me.FirstName
	}

	usersUpdate := &models.User{UserId: botID, UserName: botUsername, Name: botFirstName, UpdatedAt: time.Now()}

	collection := db.DB.Collection("users")
	filter := bson.M{"user_id": botID}
	update := bson.M{"$set": usersUpdate}
	opts := options.Update().SetUpsert(true)

	_, err := collection.UpdateOne(context.Background(), filter, update, opts)
	if err != nil {
		log.Errorf("[Database] EnsureBotInDb: %v", err)
		return fmt.Errorf("failed to ensure bot %d in database: %w", botID, err)
	}
	log.Infof("[Database] Bot Updated in Database! (id=%d username=%s)", botID, botUsername)
	return nil
}

// EnsureUserInDb ensures that a user exists in the database.
// Creates the user record if it doesn't exist, or updates it if it does.
// This is essential for foreign key constraints that reference the users table.
func EnsureUserInDb(userId int64, username, firstName string) error {
	userUpdate := bson.M{
		"user_id":    userId,
		"username":   username,
		"name":       firstName,
		"updated_at": time.Now(),
	}

	collection := db.DB.Collection("users")
	filter := bson.M{"user_id": userId}
	update := bson.M{"$set": userUpdate}
	opts := options.Update().SetUpsert(true)

	_, err := collection.UpdateOne(context.Background(), filter, update, opts)
	if err != nil {
		log.Errorf("[Database] EnsureUserInDb: %v", err)
		return fmt.Errorf("failed to ensure user %d in database: %w", userId, err)
	}
	return nil
}

// checkUserInfo retrieves user information using optimized cached queries.
// Returns nil if the user doesn't exist, or a default User struct on error.
func checkUserInfo(userId int64) (userc *models.User) {
	// Use optimized cached query instead of SELECT *
	userc, err := GetUserBasicInfoCached(userId)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil
		}
		log.Errorf("[Database] checkUserInfo: %v - %d", err, userId)
		return &models.User{UserId: userId}
	}
	return userc
}

// UpdateUser creates or updates user information in the database.
// Only updates fields that have actually changed to minimize database operations.
// Always updates last_activity to track user interactions.
// Invalidates user cache after successful update.
// Returns error if database operation fails.
func UpdateUser(userId int64, username, name string) error {
	userc := checkUserInfo(userId)
	now := time.Now()

	if userc != nil {
		// Always update last_activity, but only update other fields if changed
		updates := bson.M{
			"last_activity": now,
			"updated_at":    now,
		}

		// Check if profile updates are needed
		if userc.Name != name {
			updates["name"] = name
		}
		if userc.UserName != username {
			updates["username"] = username
		}

		collection := db.DB.Collection("users")
		_, err := collection.UpdateOne(context.Background(), bson.M{"user_id": userId}, bson.M{"$set": updates})
		if err != nil {
			log.Errorf("[Database] UpdateUser: %v - %d", err, userId)
			return err
		}
		// Invalidate cache after update
		cache.DeleteCache(cache.CacheKey("user", userId))
		log.Debugf("[Database] UpdateUser: %d", userId)
	} else {
		// Create new user
		newUser := &models.User{
			UserId:       userId,
			UserName:     username,
			Name:         name,
			LastActivity: now,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		collection := db.DB.Collection("users")
		_, err := collection.InsertOne(context.Background(), newUser)
		if err != nil {
			log.Errorf("[Database] UpdateUser: %v - %d", err, userId)
			return err
		}
		// Invalidate cache after create
		cache.DeleteCache(cache.CacheKey("user", userId))
		log.Infof("[Database] UpdateUser: created new user %d", userId)
	}
	return nil
}

// GetUserIdByUserName retrieves a user ID by their username.
// Returns 0 if the user is not found or an error occurs.
func GetUserIdByUserName(username string) int64 {
	var user models.User
	collection := db.DB.Collection("users")
	err := collection.FindOne(context.Background(), bson.M{"username": username}, options.FindOne().SetProjection(bson.M{"user_id": 1})).Decode(&user)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return 0
		}
		log.Errorf("[Database] GetUserIdByUserName: %v - %s", err, username)
		return 0
	}
	log.Debugf("[Database] GetUserIdByUserName: %d", user.UserId)
	return user.UserId
}

// GetUserInfoById retrieves username and name for a given user ID.
// Returns empty strings and false if the user is not found.
func GetUserInfoById(userId int64) (username, name string, found bool) {
	user := checkUserInfo(userId)
	if user != nil {
		username = user.UserName
		name = user.Name
		found = true
		log.Debugf("%+v", user)
	}
	return
}

// LoadUsersStats returns the total count of users in the database.
// Used for generating system statistics and monitoring.
func LoadUsersStats() (count int64) {
	collection := db.DB.Collection("users")
	count, err := collection.CountDocuments(context.Background(), bson.M{})
	if err != nil {
		log.Errorf("[Database] loadStats: %v", err)
		return
	}
	return
}

// LoadUserActivityStats returns Daily Active Users, Weekly Active Users, and Monthly Active Users.
// These metrics are based on last_activity timestamps within the respective time periods.
func LoadUserActivityStats() (dau, wau, mau int64) {
	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)
	weekAgo := now.Add(-7 * 24 * time.Hour)
	monthAgo := now.Add(-30 * 24 * time.Hour)

	collection := db.DB.Collection("users")

	// Count daily active users
	var err error
	dau, err = collection.CountDocuments(context.Background(), bson.M{"last_activity": bson.M{"$gte": dayAgo}})
	if err != nil {
		log.Errorf("[Database][LoadUserActivityStats] counting daily active users: %v", err)
	}

	// Count weekly active users
	wau, err = collection.CountDocuments(context.Background(), bson.M{"last_activity": bson.M{"$gte": weekAgo}})
	if err != nil {
		log.Errorf("[Database][LoadUserActivityStats] counting weekly active users: %v", err)
	}

	// Count monthly active users
	mau, err = collection.CountDocuments(context.Background(), bson.M{"last_activity": bson.M{"$gte": monthAgo}})
	if err != nil {
		log.Errorf("[Database][LoadUserActivityStats] counting monthly active users: %v", err)
	}

	return dau, wau, mau
}
