package locks

import (
	"context"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
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
	cache.DeleteCache(LocksProjection.Key(chatID))
}

func IsPermLocked(chatID int64, perm string) bool {
	return GetChatLocks(chatID)[perm]
}

func LoadLocksStats() (lockedPerms, lockChats int64) {
	lockedPerms = db.CountRows(&models.LockSettings{}, "locked = ?", true)
	lockChats = db.CountDistinct(&models.LockSettings{}, "chat_id", "locked = ?", true)
	return
}

// GetChatAllowlist returns the chat's lock exceptions, host entries first,
// then invites, each sorted by value. A read failure logs and reads as an
// empty list; callers that must tell the two apart use GetChatAllowlistError.
func GetChatAllowlist(chatID int64) []models.LockAllowlist {
	exceptions, err := GetChatAllowlistError(chatID)
	if err != nil {
		log.Errorf("[Database] GetChatAllowlist: %v - %d", err, chatID)
		return nil
	}
	return exceptions
}

// GetChatAllowlistError is GetChatAllowlist with the read error returned.
func GetChatAllowlistError(chatID int64) ([]models.LockAllowlist, error) {
	return GetChatAllowlistCachedContext(context.Background(), chatID)
}

// AddAllowlist stores one exception; adding an existing (kind, value) is a
// no-op, matching /allowlist's "already listed" answer.
func AddAllowlist(chatID int64, kind, value string) error {
	record := models.LockAllowlist{ChatId: chatID, Kind: kind, Value: value}
	err := db.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&record).Error
	if err != nil {
		log.Errorf("[Database] AddAllowlist: %v", err)
		return err
	}
	InvalidateAllowlistCache(chatID)
	return nil
}

// RemoveAllowlist deletes one exception of the given kind; removing an absent
// one is not an error.
func RemoveAllowlist(chatID int64, kind, value string) error {
	err := db.DB.Where("chat_id = ? AND kind = ? AND value = ?", chatID, kind, value).
		Delete(&models.LockAllowlist{}).Error
	if err != nil {
		log.Errorf("[Database] RemoveAllowlist: %v", err)
		return err
	}
	InvalidateAllowlistCache(chatID)
	return nil
}

// ClearAllowlist removes every exception of the chat.
func ClearAllowlist(chatID int64) error {
	err := db.DB.Where("chat_id = ?", chatID).Delete(&models.LockAllowlist{}).Error
	if err != nil {
		log.Errorf("[Database] ClearAllowlist: %v", err)
		return err
	}
	InvalidateAllowlistCache(chatID)
	return nil
}

func InvalidateAllowlistCache(chatID int64) {
	cache.DeleteCache(LockAllowlistProjection.Key(chatID))
}
