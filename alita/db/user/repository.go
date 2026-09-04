package user

import (
	"errors"
	"fmt"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func EnsureBotInDb(b *gotgbot.Bot) error {
	me, errGet := b.GetMe(nil)
	if errGet != nil {
		log.Errorf("[Database] EnsureBotInDb: failed to fetch bot identity via GetMe: %v", errGet)
	}

	botID := b.Id
	botUsername := b.Username
	botFirstName := b.FirstName
	if me != nil {
		botID = me.Id
		botUsername = me.Username
		botFirstName = me.FirstName
	}

	usersUpdate := &models.User{UserId: botID, UserName: botUsername, Name: botFirstName}
	result := db.DB.Where("user_id = ?", botID).Assign(usersUpdate).FirstOrCreate(&models.User{})
	if result.Error != nil {
		log.Errorf("[Database] EnsureBotInDb: %v", result.Error)
		return fmt.Errorf("failed to ensure bot %d in database: %w", botID, result.Error)
	}
	log.Infof("[Database] Bot Updated in Database! (id=%d username=%s)", botID, botUsername)
	return nil
}

func EnsureUserInDb(userId int64, username, firstName string) error {
	userUpdate := &models.User{
		UserId:   userId,
		UserName: username,
		Name:     firstName,
	}
	columns := make([]string, 0, 3)
	if username != "" {
		columns = append(columns, "username")
	}
	if firstName != "" {
		columns = append(columns, "name")
	}
	onConflict := clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoNothing: len(columns) == 0,
	}
	if len(columns) > 0 {
		columns = append(columns, "updated_at")
		onConflict.DoUpdates = clause.AssignmentColumns(columns)
	}
	result := db.DB.Clauses(onConflict).Create(userUpdate)
	if result.Error != nil {
		log.Errorf("[Database] EnsureUserInDb: %v", result.Error)
		return fmt.Errorf("failed to ensure user %d in database: %w", userId, result.Error)
	}
	cache.DeleteCache(cache.CacheKey("user", userId))
	return nil
}

func checkUserInfo(userId int64) (userc *models.User) {
	userc, err := GetUserBasicInfoCached(userId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		log.Errorf("[Database] checkUserInfo: %v - %d", err, userId)
		return &models.User{UserId: userId}
	}
	return userc
}

func UpdateUser(userId int64, username, name string) error {
	now := time.Now()
	userRecord := &models.User{
		UserId:       userId,
		UserName:     username,
		Name:         name,
		LastActivity: now,
	}
	if err := db.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns(
			[]string{"username", "name", "last_activity", "updated_at"},
		),
	}).Create(userRecord).Error; err != nil {
		log.Errorf("[Database] UpdateUser: %v - %d", err, userId)
		return err
	}
	cache.DeleteCache(cache.CacheKey("user", userId))
	log.Debugf("[Database] UpdateUser: %d", userId)
	return nil
}

func GetUserIdByUserName(username string) int64 {
	var userId int64
	err := db.DB.Model(&models.User{}).Select("user_id").Where("username = ?", username).Scan(&userId).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0
	} else if err != nil {
		log.Errorf("[Database] GetUserIdByUserName: %v - %s", err, username)
		return 0
	}
	log.Debugf("[Database] GetUserIdByUserName: %d", userId)
	return userId
}

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

func LoadUsersStats() (count int64) {
	return db.TableRowCount("users")
}

func LoadUserActivityStats() (dau, wau, mau int64) {
	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)
	weekAgo := now.Add(-7 * 24 * time.Hour)
	monthAgo := now.Add(-30 * 24 * time.Hour)

	err := db.DB.Model(&models.User{}).
		Where("last_activity >= ?", dayAgo).
		Count(&dau).Error
	if err != nil {
		log.Errorf("[Database][LoadUserActivityStats] counting daily active users: %v", err)
	}

	err = db.DB.Model(&models.User{}).
		Where("last_activity >= ?", weekAgo).
		Count(&wau).Error
	if err != nil {
		log.Errorf("[Database][LoadUserActivityStats] counting weekly active users: %v", err)
	}

	err = db.DB.Model(&models.User{}).
		Where("last_activity >= ?", monthAgo).
		Count(&mau).Error
	if err != nil {
		log.Errorf("[Database][LoadUserActivityStats] counting monthly active users: %v", err)
	}

	return dau, wau, mau
}
