package warns

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/user"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

// timeNow is the clock for warn times and expiry; tests replace it.
var timeNow = time.Now

func checkWarnSettings(chatID int64) (warnrc *models.WarnSettings) {
	return checkWarnSettingsContext(context.Background(), chatID)
}

func checkWarnSettingsContext(ctx context.Context, chatID int64) (warnrc *models.WarnSettings) {
	defaultWarnSettings := &models.WarnSettings{ChatId: chatID, WarnLimit: 3, WarnMode: "mute"}
	warnrc = &models.WarnSettings{}
	err := db.DB.WithContext(ctx).Where("chat_id = ?", chatID).First(warnrc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if !db.ChatExistsContext(ctx, chatID) {
			log.Debugf("[Database][checkWarnSettings]: Chat %d doesn't exist, returning default settings", chatID)
			return defaultWarnSettings
		}

		warnrc = defaultWarnSettings
		result := db.DB.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "chat_id"}},
			DoNothing: true,
		}).Create(warnrc)
		if result.Error != nil {
			log.Errorf("[Database] checkWarnSettings: %v", result.Error)
		} else if result.RowsAffected == 0 {
			if err := db.DB.WithContext(ctx).Where("chat_id = ?", chatID).First(warnrc).Error; err != nil {
				log.Errorf("[Database] checkWarnSettings reload: %v", err)
				warnrc = defaultWarnSettings
			}
		}
	} else if err != nil {
		log.Errorf("[Database][checkWarnSettings]: %d - %v", chatID, err)
		warnrc = defaultWarnSettings
	}
	return
}

func checkWarns(userId, chatId int64) (warnrc *models.Warns) {
	return checkWarnsContext(context.Background(), userId, chatId)
}

func checkWarnsContext(ctx context.Context, userId, chatId int64) (warnrc *models.Warns) {
	defaultWarnSrc := &models.Warns{UserId: userId, ChatId: chatId, NumWarns: 0, Reasons: make(models.StringArray, 0)}
	warnrc = &models.Warns{}
	err := db.DB.WithContext(ctx).Where("user_id = ? AND chat_id = ?", userId, chatId).First(warnrc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if !db.ChatExistsContext(ctx, chatId) {
			log.Debugf("[Database][checkWarns]: Chat %d doesn't exist, returning default settings", chatId)
			return defaultWarnSrc
		}

		warnrc = defaultWarnSrc
		result := db.DB.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "chat_id"}},
			DoNothing: true,
		}).Create(warnrc)
		if result.Error != nil {
			log.Errorf("[Database] checkWarns: %v", result.Error)
		} else if result.RowsAffected == 0 {
			if err := db.DB.WithContext(ctx).Where("user_id = ? AND chat_id = ?", userId, chatId).First(warnrc).Error; err != nil {
				log.Errorf("[Database] checkWarns reload: %v", err)
				warnrc = defaultWarnSrc
			}
		}
	} else if err != nil {
		log.Errorf("[Database][checkUserWarns]: %d - %v", userId, err)
		warnrc = defaultWarnSrc
	}
	return
}

// WarnUser adds one warn. entry carries the reason and the issuer; the user,
// chat and time are filled in here. It returns the new count and every entry.
func WarnUser(userId, chatId int64, entry models.WarnEntry) (int, []models.WarnEntry, error) {
	var numWarns int
	var entries []models.WarnEntry

	if err := chats.EnsureChatInDb(chatId, ""); err != nil {
		return 0, nil, err
	}
	if err := user.EnsureUserInDb(userId, "", ""); err != nil {
		return 0, nil, err
	}

	err := db.DB.Transaction(func(tx *gorm.DB) error {
		// ponytail: this serializes warnings per chat; use per-user advisory
		// locks only if moderation write throughput becomes material.
		if err := lockChat(tx, chatId); err != nil {
			return err
		}

		warnSettings := &models.WarnSettings{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("chat_id = ?", chatId).
			First(warnSettings).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				warnSettings = &models.WarnSettings{ChatId: chatId, WarnLimit: 3, WarnMode: "mute"}
				if err := tx.Create(warnSettings).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		}

		warnrc := &models.Warns{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND chat_id = ?", userId, chatId).
			First(warnrc).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				warnrc = &models.Warns{
					UserId:  userId,
					ChatId:  chatId,
					Reasons: models.StringArray{},
				}
			} else {
				return err
			}
		}
		if err := syncEntriesWithReasons(tx, warnrc); err != nil {
			return err
		}
		if _, err := pruneExpiredEntries(tx, warnrc, warnSettings.WarnExpiry); err != nil {
			return err
		}

		warnrc.NumWarns++

		reason := entry.Reason
		if reason != "" {
			if len(reason) > 3000 {
				reason = reason[:3000]
				for !utf8.ValidString(reason) {
					reason = reason[:len(reason)-1]
				}
			}
		} else {
			tr := i18n.MustNewTranslator("en")
			noReason, _ := tr.GetString("db_warn_no_reason")
			if noReason == "" {
				noReason = "No Reason"
			}
			reason = noReason
		}
		// The reasons column is still written so an older release reads it.
		warnrc.Reasons = append(warnrc.Reasons, reason)

		if err := tx.Save(warnrc).Error; err != nil {
			return err
		}

		now := timeNow().UTC()
		entry.ID = 0
		entry.UserId = userId
		entry.ChatId = chatId
		entry.Reason = reason
		entry.WarnedAt = &now
		if err := tx.Create(&entry).Error; err != nil {
			return err
		}

		numWarns = warnrc.NumWarns
		var err error
		entries, err = loadEntries(tx, warnrc)
		return err
	})
	if err != nil {
		log.Errorf("[Database] WarnUser: %v", err)
		return 0, nil, err
	}

	cache.DeleteCache(cache.CacheKey("warns", userId, chatId))
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))

	return numWarns, entries, nil
}

// syncEntriesWithReasons runs under the row locks before a warn is added or
// removed, and stores what models.ReconcileWarnEntries makes of the entries.
// An older release writes warns_users alone: its /resetwarns leaves entries
// for a user without warns, its /rmwarn and /warn pop and append reasons
// without their entries, and its restore leaves reasons with no entries.
func syncEntriesWithReasons(tx *gorm.DB, warnrc *models.Warns) error {
	stored, err := findEntries(tx, warnrc)
	if err != nil {
		return err
	}
	entries, kept := models.ReconcileWarnEntries(warnrc, stored)
	if stale := stored[kept:]; len(stale) > 0 {
		ids := make([]uint, len(stale))
		for i, e := range stale {
			ids[i] = e.ID
		}
		if err := tx.Delete(&models.WarnEntry{}, ids).Error; err != nil {
			return err
		}
	}
	if missing := entries[kept:]; len(missing) > 0 {
		return tx.Create(&missing).Error
	}
	return nil
}

// loadEntries returns the user's entries oldest first, reconciled with their
// reasons as syncEntriesWithReasons would store them. It writes nothing.
func loadEntries(tx *gorm.DB, warnrc *models.Warns) ([]models.WarnEntry, error) {
	if warnrc.NumWarns == 0 {
		return nil, nil
	}
	stored, err := findEntries(tx, warnrc)
	if err != nil {
		return nil, err
	}
	entries, _ := models.ReconcileWarnEntries(warnrc, stored)
	return entries, nil
}

func findEntries(tx *gorm.DB, warnrc *models.Warns) ([]models.WarnEntry, error) {
	var entries []models.WarnEntry
	err := tx.Where("chat_id = ? AND user_id = ?", warnrc.ChatId, warnrc.UserId).
		Order("id").
		Find(&entries).Error
	return entries, err
}

// lockChat takes the chat-row lock every warn write holds for its whole
// transaction, so writes of one chat's warns run one at a time.
func lockChat(tx *gorm.DB, chatId int64) error {
	var chat models.Chat
	return tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id").
		Where("chat_id = ?", chatId).
		Take(&chat).Error
}

// lockChatForReset is lockChat for the resets, which also run for a chat with
// no chats row and then have nothing to wait for.
func lockChatForReset(tx *gorm.DB, chatId int64) error {
	if err := lockChat(tx, chatId); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return nil
}

// expiredWarn reports whether e stopped counting: it is expiry seconds old or
// older. Expiry 0 is off, and an entry without a time never expires.
func expiredWarn(e models.WarnEntry, expiry int, now time.Time) bool {
	return expiry > 0 && e.WarnedAt != nil && !e.WarnedAt.After(now.Add(-time.Duration(expiry)*time.Second))
}

// pruneExpiredEntries deletes the user's expired entries and takes them off
// the count; the reasons column is rewritten from the entries left. It runs
// with the user's warns row locked, and the caller saves warnrc.
func pruneExpiredEntries(tx *gorm.DB, warnrc *models.Warns, expiry int) (bool, error) {
	if expiry <= 0 || warnrc.NumWarns == 0 {
		return false, nil
	}
	var entries []models.WarnEntry
	if err := tx.Where("chat_id = ? AND user_id = ?", warnrc.ChatId, warnrc.UserId).
		Order("id").
		Find(&entries).Error; err != nil {
		return false, err
	}
	now := timeNow()
	var expired []uint
	kept := models.StringArray{}
	for _, e := range entries {
		if expiredWarn(e, expiry, now) {
			expired = append(expired, e.ID)
		} else {
			kept = append(kept, e.Reason)
		}
	}
	if len(expired) == 0 {
		return false, nil
	}
	if err := tx.Delete(&models.WarnEntry{}, expired).Error; err != nil {
		return false, err
	}
	warnrc.NumWarns = max(warnrc.NumWarns-len(expired), 0)
	warnrc.Reasons = kept
	if warnrc.NumWarns == 0 {
		// No warns keep no entries, as syncEntriesWithReasons expects.
		warnrc.Reasons = models.StringArray{}
		return true, deleteEntries(tx, "chat_id = ? AND user_id = ?", warnrc.ChatId, warnrc.UserId)
	}
	return true, nil
}

func deleteEntries(tx *gorm.DB, query string, args ...any) error {
	return tx.Where(query, args...).Delete(&models.WarnEntry{}).Error
}

func RemoveWarn(userId, chatId int64) (bool, error) {
	var removed, pruned bool

	err := db.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockChat(tx, chatId); err != nil {
			return err
		}

		warnrc := &models.Warns{}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND chat_id = ?", userId, chatId).
			First(warnrc).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				removed = false
				return nil
			}
			return err
		}
		// The newest entry deleted below must be the popped reason's.
		if err := syncEntriesWithReasons(tx, warnrc); err != nil {
			return err
		}

		var expiry int
		if err := tx.Model(&models.WarnSettings{}).
			Where("chat_id = ?", chatId).
			Limit(1).
			Pluck("warn_expiry", &expiry).Error; err != nil {
			return err
		}
		var err error
		if pruned, err = pruneExpiredEntries(tx, warnrc, expiry); err != nil {
			return err
		}

		if warnrc.NumWarns > 0 {
			warnrc.NumWarns--
			if len(warnrc.Reasons) > 0 {
				warnrc.Reasons = warnrc.Reasons[:len(warnrc.Reasons)-1]
			}
			removed = true

			if err := tx.Save(warnrc).Error; err != nil {
				return err
			}

			var newest models.WarnEntry
			err := tx.Where("chat_id = ? AND user_id = ?", chatId, userId).Order("id DESC").Take(&newest).Error
			if err == nil {
				err = tx.Delete(&newest).Error
			} else if errors.Is(err, gorm.ErrRecordNotFound) {
				err = nil
			}
			if err != nil {
				return err
			}
		} else if pruned {
			if err := tx.Save(warnrc).Error; err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		log.Errorf("[Database] RemoveWarn: %v", err)
		return false, err
	}

	if removed || pruned {
		cache.DeleteCache(cache.CacheKey("warns", userId, chatId))
		cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	}

	return removed, nil
}

func ResetUserWarns(userId, chatId int64) (bool, error) {
	var reset bool
	// The chat-row lock waits for a concurrent WarnUser to commit, so both
	// DELETEs see its warn row and its entry. Without it a first warn, whose
	// warns_users row is still an uncommitted INSERT, can commit between them:
	// its entry is deleted and its warn row survives.
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockChatForReset(tx, chatId); err != nil {
			return err
		}
		result := tx.Where("user_id = ? AND chat_id = ?", userId, chatId).Delete(&models.Warns{})
		if result.Error != nil {
			return result.Error
		}
		reset = result.RowsAffected > 0
		return deleteEntries(tx, "chat_id = ? AND user_id = ?", chatId, userId)
	})
	if err != nil {
		log.Errorf("[Database] ResetUserWarns: %v", err)
		return false, err
	}
	if !reset {
		return false, nil
	}
	cache.DeleteCache(cache.CacheKey("warns", userId, chatId))
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	return true, nil
}

func GetWarns(userId, chatId int64) (int, []models.WarnEntry) {
	return GetWarnsContext(context.Background(), userId, chatId)
}

func GetWarnsContext(ctx context.Context, userId, chatId int64) (int, []models.WarnEntry) {
	// Reasons keeps its pre-entries field name on purpose: a value cached by an
	// older release ([]string) then fails to decode and is reloaded, where a
	// renamed field would decode as an empty list.
	type warnCache struct {
		NumWarns int
		Reasons  []models.WarnEntry
	}
	cached, err := cache.GetFromCacheOrLoad(ctx,
		cache.CacheKey("warns", userId, chatId),
		cache.CacheTTLWarnSettings,
		func(ctx context.Context) (warnCache, error) {
			w := checkWarnsContext(ctx, userId, chatId)
			entries, err := loadEntries(db.DB.WithContext(ctx), w)
			if err != nil {
				return warnCache{}, err
			}
			return warnCache{NumWarns: w.NumWarns, Reasons: entries}, nil
		},
	)
	if err != nil {
		return 0, nil
	}
	if cached.NumWarns == 0 {
		return 0, cached.Reasons
	}
	// Expiry is applied here, not in the loader, so a warn that ages past it
	// stops counting while the cached value is still live.
	expiry := GetWarnSettingContext(ctx, chatId).WarnExpiry
	if expiry <= 0 {
		return cached.NumWarns, cached.Reasons
	}
	now := timeNow()
	numWarns := cached.NumWarns
	live := make([]models.WarnEntry, 0, len(cached.Reasons))
	for _, e := range cached.Reasons {
		if expiredWarn(e, expiry, now) {
			numWarns--
		} else {
			live = append(live, e)
		}
	}
	if numWarns <= 0 {
		return 0, nil
	}
	return numWarns, live
}

func SetWarnLimit(chatId int64, warnLimit int) error {
	// Single-statement upsert: no read-modify-write, concurrent limit/mode sets can't clobber.
	err := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"warn_limit"}),
	}).Create(&models.WarnSettings{ChatId: chatId, WarnLimit: warnLimit, WarnMode: "mute"}).Error
	if err != nil {
		log.Errorf("[Database] SetWarnLimit: %v", err)
		return err
	}
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	return nil
}

// SetWarnMode stores the mode with its tban/tmute duration in seconds; pass 0
// for modes without one so a previous duration does not linger.
func SetWarnMode(chatId int64, warnMode string, actionTime int) error {
	// Single-statement upsert: see SetWarnLimit.
	err := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"warn_mode", "warn_action_time"}),
	}).Create(&models.WarnSettings{ChatId: chatId, WarnLimit: 3, WarnMode: warnMode, WarnActionTime: actionTime}).Error
	if err != nil {
		log.Errorf("[Database] SetWarnMode: %v", err)
		return err
	}
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	return nil
}

// SetWarnExpiry stores how long a warn counts, in seconds; 0 turns expiry off.
func SetWarnExpiry(chatId int64, seconds int) error {
	// Single-statement upsert: see SetWarnLimit.
	err := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"warn_expiry"}),
	}).Create(&models.WarnSettings{ChatId: chatId, WarnLimit: 3, WarnMode: "mute", WarnExpiry: seconds}).Error
	if err != nil {
		log.Errorf("[Database] SetWarnExpiry: %v", err)
		return err
	}
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	return nil
}

func GetWarnSetting(chatId int64) *models.WarnSettings {
	return GetWarnSettingContext(context.Background(), chatId)
}

func GetWarnSettingContext(ctx context.Context, chatId int64) *models.WarnSettings {
	cached, err := cache.GetFromCacheOrLoad(ctx,
		cache.CacheKey("warn_settings", chatId),
		cache.CacheTTLWarnSettings,
		func(ctx context.Context) (models.WarnSettings, error) {
			w := checkWarnSettingsContext(ctx, chatId)
			return *w, nil
		},
	)
	if err != nil {
		return &models.WarnSettings{ChatId: chatId, WarnLimit: 3, WarnMode: "mute"}
	}
	return &cached
}

func GetAllChatWarns(chatId int64) int {
	var count int64
	err := db.DB.Model(&models.Warns{}).Where("chat_id = ?", chatId).Count(&count).Error
	if err != nil {
		log.Errorf("[Database] GetAllChatWarns: %v", err)
		return 0
	}
	return int(count)
}

func ResetAllChatWarns(chatId int64) error {
	var userIds []int64
	// The chat-row lock, for the reason given in ResetUserWarns. The users are
	// read under it, so a warn committed just before is in the cache sweep.
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockChatForReset(tx, chatId); err != nil {
			return err
		}
		if err := tx.Model(&models.Warns{}).Where("chat_id = ?", chatId).Pluck("user_id", &userIds).Error; err != nil {
			return err
		}
		if err := tx.Where("chat_id = ?", chatId).Delete(&models.Warns{}).Error; err != nil {
			return err
		}
		return deleteEntries(tx, "chat_id = ?", chatId)
	})
	if err != nil {
		log.Errorf("[Database] ResetAllChatWarns: %v", err)
		return err
	}
	for _, userId := range userIds {
		cache.DeleteCache(cache.CacheKey("warns", userId, chatId))
	}
	cache.DeleteCache(cache.CacheKey("warn_settings", chatId))
	return nil
}

func LoadWarnsStats() (warnedUsers, warnChats int64) {
	warnedUsers = db.CountRows(&models.Warns{}, "")
	warnChats = db.CountDistinct(&models.Warns{}, "chat_id", "")
	return
}
