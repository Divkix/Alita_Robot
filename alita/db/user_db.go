package db

import (
	"errors"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/PaulSonOfLars/gotgbot/v2"
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

	usersUpdate := &User{UserId: botID, UserName: botUsername, Name: botFirstName}
	err := DB.Where("user_id = ?", botID).Assign(usersUpdate).FirstOrCreate(&User{})
	if err.Error != nil {
		log.Errorf("[Database] EnsureBotInDb: %v", err.Error)
		return fmt.Errorf("failed to ensure bot %d in database: %w", botID, err.Error)
	}
	log.Infof("[Database] Bot Updated in Database! (id=%d username=%s)", botID, botUsername)
	return nil
}

// EnsureUserInDb ensures that a user exists in the database.
// Creates the user record if it doesn't exist, or updates it if it does.
// This is essential for foreign key constraints that reference the users table.
func EnsureUserInDb(userId int64, username, firstName string) error {
	userUpdate := &User{
		UserId:   userId,
		UserName: username,
		Name:     firstName,
	}
	err := DB.Where("user_id = ?", userId).Assign(userUpdate).FirstOrCreate(&User{})
	if err.Error != nil {
		log.Errorf("[Database] EnsureUserInDb: %v", err.Error)
		return fmt.Errorf("failed to ensure user %d in database: %w", userId, err.Error)
	}
	return nil
}

// checkUserInfo retrieves user information using optimized cached queries.
// Returns nil if the user doesn't exist, or a default User struct on error.
func checkUserInfo(userId int64) (userc *User) {
	// Use optimized cached query instead of SELECT *
	userc, err := GetOptimizedQueries().GetUserBasicInfoCached(userId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		log.Errorf("[Database] checkUserInfo: %v - %d", err, userId)
		return &User{UserId: userId}
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
		updates := map[string]any{
			"last_activity": now,
		}

		// Check if profile updates are needed
		if userc.Name != name {
			updates["name"] = name
		}
		if userc.UserName != username {
			updates["username"] = username
		}

		err := DB.Model(&User{}).Where("user_id = ?", userId).Updates(updates).Error
		if err != nil {
			log.Errorf("[Database] UpdateUser: %v - %d", err, userId)
			return err
		}
		// Invalidate cache after update
		deleteCache(userCacheKey(userId))
		log.Debugf("[Database] UpdateUser: %d", userId)
	} else {
		// Create new user
		userc = &User{
			UserId:       userId,
			UserName:     username,
			Name:         name,
			LastActivity: now,
		}
		err := DB.Create(userc).Error
		if err != nil {
			log.Errorf("[Database] UpdateUser: %v - %d", err, userId)
			return err
		}
		// Invalidate cache after create
		deleteCache(userCacheKey(userId))
		log.Infof("[Database] UpdateUser: created new user %d", userId)
	}
	return nil
}

// GetUserIdByUserName retrieves a user ID by their username.
// Returns 0 if the user is not found or an error occurs.
func GetUserIdByUserName(username string) int64 {
	var userId int64
	// Only fetch the user_id column
	err := DB.Model(&User{}).Select("user_id").Where("username = ?", username).Scan(&userId).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0
	} else if err != nil {
		log.Errorf("[Database] GetUserIdByUserName: %v - %s", err, username)
		return 0
	}
	log.Debugf("[Database] GetUserIdByUserName: %d", userId)
	return userId
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
	err := DB.Model(&User{}).Count(&count)
	if err.Error != nil {
		log.Errorf("[Database] loadStats: %v", err.Error)
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

	// Count daily active users
	err := DB.Model(&User{}).
		Where("last_activity >= ?", dayAgo).
		Count(&dau).Error
	if err != nil {
		log.Errorf("[Database][LoadUserActivityStats] counting daily active users: %v", err)
	}

	// Count weekly active users
	err = DB.Model(&User{}).
		Where("last_activity >= ?", weekAgo).
		Count(&wau).Error
	if err != nil {
		log.Errorf("[Database][LoadUserActivityStats] counting weekly active users: %v", err)
	}

	// Count monthly active users
	err = DB.Model(&User{}).
		Where("last_activity >= ?", monthAgo).
		Count(&mau).Error
	if err != nil {
		log.Errorf("[Database][LoadUserActivityStats] counting monthly active users: %v", err)
	}

	return dau, wau, mau
}
