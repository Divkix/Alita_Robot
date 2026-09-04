package locks

import (
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm/clause"
)

func GetChatLocks(chatID int64) map[string]bool {
	locks, err := GetChatLocksCached(chatID)
	if err != nil {
		log.Errorf("[Database] GetChatLocks: %v - %d", err, chatID)
		return make(map[string]bool)
	}

	return locks
}

func UpdateLock(chatID int64, perm string, val bool) error {
	record := models.LockSettings{
		ChatId:   chatID,
		LockType: perm,
		Locked:   val,
	}

	err := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}, {Name: "lock_type"}},
		DoUpdates: clause.AssignmentColumns([]string{"locked"}),
	}).Create(&record).Error
	if err != nil {
		log.Errorf("[Database] UpdateLock: %v", err)
		return err
	}

	InvalidateLockCache(chatID)
	return nil
}

func InvalidateLockCache(chatID int64) {
	cache.DeleteCache(cache.CacheKey("locks_map", chatID))
}

func IsPermLocked(chatID int64, perm string) bool {
	return GetChatLocks(chatID)[perm]
}
