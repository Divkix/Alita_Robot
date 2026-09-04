package captcha

import (
	"errors"
	"fmt"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrInvalidCaptchaMode   = errors.New("INVALID_CAPTCHA_MODE")
	ErrInvalidTimeout       = errors.New("INVALID_TIMEOUT_RANGE")
	ErrInvalidFailureAction = errors.New("INVALID_FAILURE_ACTION")
	ErrInvalidMaxAttempts   = errors.New("INVALID_MAX_ATTEMPTS")
	ErrNoActiveCaptcha      = errors.New("NO_ACTIVE_CAPTCHA")
	ErrCaptchaDisabled      = errors.New("CAPTCHA_DISABLED")
)

func GetCaptchaSettings(chatID int64) (*models.CaptchaSettings, error) {
	return cache.GetFromCacheOrLoad(cache.CacheKey("captcha_settings", chatID), cache.CacheTTLCaptchaSettings, func() (*models.CaptchaSettings, error) {
		settings := &models.CaptchaSettings{}
		err := db.GetRecord(settings, map[string]any{"chat_id": chatID})

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &models.CaptchaSettings{
				ChatID:        chatID,
				Enabled:       false,
				CaptchaMode:   "math",
				Timeout:       2,
				FailureAction: "kick",
				MaxAttempts:   3,
			}, nil
		}

		if err != nil {
			log.Errorf("[Database][GetCaptchaSettings]: %v", err)
			return nil, err
		}

		return settings, nil
	})
}

func SetCaptchaEnabled(chatID int64, enabled bool) error {
	updates := map[string]any{
		"chat_id": chatID,
		"enabled": enabled,
	}

	err := db.DB.Where("chat_id = ?", chatID).Assign(updates).FirstOrCreate(&models.CaptchaSettings{}).Error
	if err != nil {
		log.Errorf("[Database][SetCaptchaEnabled]: %v", err)
		return err
	}

	cache.DeleteCache(cache.CacheKey("captcha_settings", chatID))

	return nil
}

func SetCaptchaMode(chatID int64, mode string) error {
	if mode != "math" && mode != "text" {
		return ErrInvalidCaptchaMode
	}

	updates := map[string]any{
		"chat_id":      chatID,
		"captcha_mode": mode,
	}

	err := db.DB.Where("chat_id = ?", chatID).Assign(updates).FirstOrCreate(&models.CaptchaSettings{}).Error
	if err != nil {
		log.Errorf("[Database][SetCaptchaMode]: %v", err)
		return err
	}

	cache.DeleteCache(cache.CacheKey("captcha_settings", chatID))

	return nil
}

func SetCaptchaTimeout(chatID int64, timeout int) error {
	if timeout < 1 || timeout > 10 {
		return ErrInvalidTimeout
	}

	updates := map[string]any{
		"chat_id": chatID,
		"timeout": timeout,
	}

	err := db.DB.Where("chat_id = ?", chatID).Assign(updates).FirstOrCreate(&models.CaptchaSettings{}).Error
	if err != nil {
		log.Errorf("[Database][SetCaptchaTimeout]: %v", err)
		return err
	}

	cache.DeleteCache(cache.CacheKey("captcha_settings", chatID))

	return nil
}

func SetCaptchaMaxAttempts(chatID int64, maxAttempts int) error {
	if maxAttempts < 1 || maxAttempts > 10 {
		return ErrInvalidMaxAttempts
	}

	err := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.Assignments(map[string]any{"max_attempts": maxAttempts}),
	}).Create(&models.CaptchaSettings{ChatID: chatID, MaxAttempts: maxAttempts}).Error
	if err != nil {
		log.Errorf("[Database][SetCaptchaMaxAttempts]: %v", err)
		return err
	}

	cache.DeleteCache(cache.CacheKey("captcha_settings", chatID))
	return nil
}

func SetCaptchaFailureAction(chatID int64, action string) error {
	if action != "kick" && action != "ban" && action != "mute" {
		return ErrInvalidFailureAction
	}

	updates := map[string]any{
		"chat_id":        chatID,
		"failure_action": action,
	}

	err := db.DB.Where("chat_id = ?", chatID).Assign(updates).FirstOrCreate(&models.CaptchaSettings{}).Error
	if err != nil {
		log.Errorf("[Database][SetCaptchaFailureAction]: %v", err)
		return err
	}

	cache.DeleteCache(cache.CacheKey("captcha_settings", chatID))

	return nil
}

func CreateCaptchaAttemptPreMessage(userID, chatID int64, answer string, timeout int) (*models.CaptchaAttempts, error) {
	return createCaptchaAttemptPreMessage(userID, chatID, answer, timeout, false)
}

func CreateCaptchaAttemptPreMessageIfEnabled(userID, chatID int64, answer string, timeout int) (*models.CaptchaAttempts, error) {
	return createCaptchaAttemptPreMessage(userID, chatID, answer, timeout, true)
}

func createCaptchaAttemptPreMessage(userID, chatID int64, answer string, timeout int, requireEnabled bool) (*models.CaptchaAttempts, error) {
	attempt := &models.CaptchaAttempts{
		UserID:       userID,
		ChatID:       chatID,
		Answer:       answer,
		Attempts:     0,
		MessageID:    0,
		RefreshCount: 0,
		ExpiresAt:    time.Now().Add(time.Duration(timeout) * time.Minute),
	}

	err := db.DB.Transaction(func(tx *gorm.DB) error {
		if tx.Name() == "postgres" {
			lockKey := fmt.Sprintf("%d:%d", chatID, userID)
			if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", lockKey).Error; err != nil {
				return err
			}
		}
		if requireEnabled {
			var settings models.CaptchaSettings
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("chat_id = ?", chatID).
				First(&settings).Error
			if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && !settings.Enabled) {
				return ErrCaptchaDisabled
			}
			if err != nil {
				return err
			}
		}

		var previous models.CaptchaAttempts
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND chat_id = ?", userID, chatID).
			Order("id DESC").
			First(&previous).Error
		if err == nil {
			attempt.PreviousMessageID = previous.MessageID
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// Remove dependent messages explicitly as well as relying on the
		// production foreign-key cascade; SQLite test schemas may not enforce it.
		if err := tx.Where("user_id = ? AND chat_id = ?", userID, chatID).
			Delete(&models.StoredMessages{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ? AND chat_id = ?", userID, chatID).Delete(&models.CaptchaAttempts{}).Error; err != nil {
			return err
		}

		if err := tx.Create(attempt).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		log.Errorf("[Database][CreateCaptchaAttemptPreMessage]: %v", err)
		return nil, err
	}
	return attempt, nil
}

func UpdateCaptchaAttemptMessageID(attemptID uint, messageID int64) error {
	result := db.DB.Model(&models.CaptchaAttempts{}).Where("id = ?", attemptID).Update("message_id", messageID)
	if result.Error != nil {
		log.Errorf("[Database][UpdateCaptchaAttemptMessageID]: %v", result.Error)
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNoActiveCaptcha
	}
	return nil
}

func GetCaptchaAttempt(userID, chatID int64) (*models.CaptchaAttempts, error) {
	attempt := &models.CaptchaAttempts{}
	err := db.DB.Where("user_id = ? AND chat_id = ? AND expires_at > ?",
		userID, chatID, time.Now()).First(attempt).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}

	if err != nil {
		log.Errorf("[Database][GetCaptchaAttempt]: %v", err)
		return nil, err
	}

	return attempt, nil
}

func GetCaptchaAttemptIncludingExpired(userID, chatID int64) (*models.CaptchaAttempts, error) {
	attempt := &models.CaptchaAttempts{}
	err := db.DB.Where("user_id = ? AND chat_id = ?", userID, chatID).
		Order("id DESC").
		First(attempt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		log.Errorf("[Database][GetCaptchaAttemptIncludingExpired]: %v", err)
		return nil, err
	}
	return attempt, nil
}

func GetCaptchaAttemptByID(attemptID uint) (*models.CaptchaAttempts, error) {
	attempt := &models.CaptchaAttempts{}
	err := db.DB.First(attempt, attemptID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		log.Errorf("[Database][GetCaptchaAttemptByID]: %v", err)
		return nil, err
	}
	return attempt, nil
}

func IncrementCaptchaAttempts(
	attemptID uint,
	userID, chatID int64,
	expectedAnswer string,
	expectedMessageID int64,
	expectedRefreshCount int,
) (*models.CaptchaAttempts, error) {
	result := db.DB.Model(&models.CaptchaAttempts{}).
		Where(
			"id = ? AND user_id = ? AND chat_id = ? AND answer = ? AND message_id = ? AND refresh_count = ? AND expires_at > ?",
			attemptID,
			userID,
			chatID,
			expectedAnswer,
			expectedMessageID,
			expectedRefreshCount,
			time.Now(),
		).
		Update("attempts", gorm.Expr("attempts + 1"))
	if result.Error != nil {
		log.Errorf("[Database][IncrementCaptchaAttempts]: %v", result.Error)
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrNoActiveCaptcha
	}

	attempt := &models.CaptchaAttempts{}
	if err := db.DB.First(attempt, attemptID).Error; err != nil {
		log.Errorf("[Database][IncrementCaptchaAttempts:Reload]: %v", err)
		return nil, err
	}
	return attempt, nil
}

func DeleteCaptchaAttempt(userID, chatID int64) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		var ids []uint
		if err := tx.Model(&models.CaptchaAttempts{}).
			Where("user_id = ? AND chat_id = ?", userID, chatID).
			Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		if err := tx.Where("id IN ?", ids).Delete(&models.CaptchaAttempts{}).Error; err != nil {
			return err
		}
		return tx.Where("attempt_id IN ?", ids).Delete(&models.StoredMessages{}).Error
	})
}

func DeleteCaptchaAttemptByIDAtomic(attemptID uint, userID, chatID int64) (bool, error) {
	deleted, err := deleteCaptchaAttemptAtomic(
		attemptID,
		false,
		0,
		0,
		"id = ? AND user_id = ? AND chat_id = ?",
		attemptID,
		userID,
		chatID,
	)
	if err != nil {
		log.Errorf("[Database][DeleteCaptchaAttemptByIDAtomic]: %v", err)
	}
	return deleted, err
}

func CompleteCaptchaAttemptAtomic(
	attemptID uint,
	userID, chatID int64,
	answer string,
	expectedMessageID int64,
	expectedRefreshCount int,
) (bool, error) {
	deleted, err := deleteCaptchaAttemptAtomic(
		attemptID,
		true,
		userID,
		chatID,
		"id = ? AND user_id = ? AND chat_id = ? AND answer = ? AND message_id = ? AND refresh_count = ? AND expires_at > ?",
		attemptID,
		userID,
		chatID,
		answer,
		expectedMessageID,
		expectedRefreshCount,
		time.Now(),
	)
	if err != nil {
		log.Errorf("[Database][CompleteCaptchaAttemptAtomic]: %v", err)
	}
	return deleted, err
}

func ReleaseCaptchaAttemptAtomic(attemptID uint, userID, chatID int64) (bool, error) {
	deleted, err := deleteCaptchaAttemptAtomic(
		attemptID,
		true,
		userID,
		chatID,
		"id = ? AND user_id = ? AND chat_id = ?",
		attemptID,
		userID,
		chatID,
	)
	if err != nil {
		log.Errorf("[Database][ReleaseCaptchaAttemptAtomic]: %v", err)
	}
	return deleted, err
}

func deleteCaptchaAttemptAtomic(
	attemptID uint,
	scheduleUnmute bool,
	userID, chatID int64,
	where string,
	args ...any,
) (bool, error) {
	var deleted bool
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Where(where, args...).Delete(&models.CaptchaAttempts{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		deleted = true
		if err := tx.Where("attempt_id = ?", attemptID).Delete(&models.StoredMessages{}).Error; err != nil {
			return err
		}
		if scheduleUnmute {
			return createMutedUser(tx, userID, chatID, time.Now().UTC())
		}
		return nil
	})
	if err != nil {
		deleted = false
	}
	return deleted, err
}

func DeleteAllCaptchaAttempts(chatID int64) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		var ids []uint
		if err := tx.Model(&models.CaptchaAttempts{}).Where("chat_id = ?", chatID).Pluck("id", &ids).Error; err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		if err := tx.Where("id IN ?", ids).Delete(&models.CaptchaAttempts{}).Error; err != nil {
			return err
		}
		return tx.Where("attempt_id IN ?", ids).Delete(&models.StoredMessages{}).Error
	})
}

func UpdateCaptchaAttemptOnRefreshByID(
	attemptID uint,
	expectedAnswer string,
	expectedMessageID int64,
	expectedRefreshCount int,
	newAnswer string,
	newMessageID int64,
) (*models.CaptchaAttempts, error) {
	updates := map[string]any{
		"answer":        newAnswer,
		"message_id":    newMessageID,
		"refresh_count": gorm.Expr("COALESCE(refresh_count, 0) + 1"),
	}
	result := db.DB.Model(&models.CaptchaAttempts{}).
		Where(
			"id = ? AND answer = ? AND message_id = ? AND refresh_count = ? AND expires_at > ?",
			attemptID,
			expectedAnswer,
			expectedMessageID,
			expectedRefreshCount,
			time.Now(),
		).
		Updates(updates)
	if result.Error != nil {
		log.Errorf("[Database][UpdateCaptchaAttemptOnRefreshByID:Update]: %v", result.Error)
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}

	attempt := &models.CaptchaAttempts{}
	err := db.DB.First(attempt, attemptID).Error
	if err != nil {
		log.Errorf("[Database][UpdateCaptchaAttemptOnRefreshByID:Reload]: %v", err)
		return nil, err
	}
	return attempt, nil
}

func StoreMessageForCaptcha(userID, chatID int64, attemptID uint, messageType int, content, fileID, caption string) error {
	storedMsg := &models.StoredMessages{
		UserID:      userID,
		ChatID:      chatID,
		AttemptID:   attemptID,
		MessageType: messageType,
		Content:     content,
		FileID:      fileID,
		Caption:     caption,
	}

	err := db.DB.Create(storedMsg).Error
	if err != nil {
		log.Errorf("[Database][StoreMessageForCaptcha]: %v", err)
		return err
	}

	return nil
}

func GetStoredMessagesForAttempt(attemptID uint) ([]*models.StoredMessages, error) {
	var messages []*models.StoredMessages
	err := db.DB.Where("attempt_id = ?", attemptID).Order("created_at ASC").Find(&messages).Error
	if err != nil {
		log.Errorf("[Database][GetStoredMessagesForAttempt]: %v", err)
		return nil, err
	}
	return messages, nil
}

func GetStoredMessagesForUser(userID, chatID int64) ([]*models.StoredMessages, error) {
	var messages []*models.StoredMessages
	err := db.DB.Where("user_id = ? AND chat_id = ?", userID, chatID).Order("created_at ASC").Find(&messages).Error
	if err != nil {
		log.Errorf("[Database][GetStoredMessagesForUser]: %v", err)
		return nil, err
	}
	return messages, nil
}

func DeleteStoredMessagesForAttempt(attemptID uint) error {
	result := db.DB.Where("attempt_id = ?", attemptID).Delete(&models.StoredMessages{})
	if result.Error != nil {
		log.Errorf("[Database][DeleteStoredMessagesForAttempt]: %v", result.Error)
		return result.Error
	}

	if result.RowsAffected > 0 {
		log.Debugf("[Database][DeleteStoredMessagesForAttempt]: Deleted %d stored messages for attempt %d", result.RowsAffected, attemptID)
	}

	return nil
}

func DeleteStoredMessagesForUser(userID, chatID int64) error {
	result := db.DB.Where("user_id = ? AND chat_id = ?", userID, chatID).Delete(&models.StoredMessages{})
	if result.Error != nil {
		log.Errorf("[Database][DeleteStoredMessagesForUser]: %v", result.Error)
		return result.Error
	}

	if result.RowsAffected > 0 {
		log.Debugf("[Database][DeleteStoredMessagesForUser]: Deleted %d stored messages for user %d in chat %d", result.RowsAffected, userID, chatID)
	}

	return nil
}

func CountStoredMessagesForAttempt(attemptID uint) (int64, error) {
	var count int64
	err := db.DB.Model(&models.StoredMessages{}).Where("attempt_id = ?", attemptID).Count(&count).Error
	if err != nil {
		log.Errorf("[Database][CountStoredMessagesForAttempt]: %v", err)
		return 0, err
	}
	return count, nil
}

func GetExpiredCaptchaAttempts() ([]*models.CaptchaAttempts, error) {
	var attempts []*models.CaptchaAttempts
	err := db.DB.Where("expires_at < ?", time.Now()).Find(&attempts).Error
	if err != nil {
		log.Errorf("[Database][GetExpiredCaptchaAttempts]: %v", err)
		return nil, err
	}
	return attempts, nil
}

func GetAllPendingCaptchaAttempts() ([]*models.CaptchaAttempts, error) {
	var attempts []*models.CaptchaAttempts
	err := db.DB.Find(&attempts).Error
	if err != nil {
		log.Errorf("[Database][GetAllPendingCaptchaAttempts]: %v", err)
		return nil, err
	}
	return attempts, nil
}

func GetCaptchaAttemptsForChat(chatID int64) ([]*models.CaptchaAttempts, error) {
	var attempts []*models.CaptchaAttempts
	if err := db.DB.Where("chat_id = ?", chatID).Find(&attempts).Error; err != nil {
		log.Errorf("[Database][GetCaptchaAttemptsForChat]: %v", err)
		return nil, err
	}
	return attempts, nil
}

func DeleteCaptchaAttemptsByIDs(ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var deleted int64
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Where("id IN ?", ids).Delete(&models.CaptchaAttempts{})
		if result.Error != nil {
			return result.Error
		}
		deleted = result.RowsAffected
		return tx.Where("attempt_id IN ?", ids).Delete(&models.StoredMessages{}).Error
	})
	if err != nil {
		log.Errorf("[Database][DeleteCaptchaAttemptsByIDs]: %v", err)
		return 0, err
	}
	return deleted, nil
}

func CreateMutedUser(userID, chatID int64, unmuteAt time.Time) error {
	return createMutedUser(db.DB, userID, chatID, unmuteAt)
}

func createMutedUser(database *gorm.DB, userID, chatID int64, unmuteAt time.Time) error {
	mutedUser := &models.CaptchaMutedUsers{
		UserID:   userID,
		ChatID:   chatID,
		UnmuteAt: unmuteAt,
	}
	return database.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "chat_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"unmute_at"}),
	}).Create(mutedUser).Error
}

func GetUsersToUnmute() ([]*models.CaptchaMutedUsers, error) {
	var users []*models.CaptchaMutedUsers
	err := db.DB.Where("unmute_at < ?", time.Now()).Find(&users).Error
	return users, err
}

func GetMutedUsersForChat(chatID int64) ([]*models.CaptchaMutedUsers, error) {
	var users []*models.CaptchaMutedUsers
	err := db.DB.Where("chat_id = ?", chatID).Find(&users).Error
	return users, err
}

func GetMutedUser(userID, chatID int64) (*models.CaptchaMutedUsers, error) {
	var user models.CaptchaMutedUsers
	err := db.DB.Where("user_id = ? AND chat_id = ?", userID, chatID).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &user, err
}

func DeleteMutedUserIfUnchanged(id uint, unmuteAt time.Time) (bool, error) {
	result := db.DB.Where("id = ? AND unmute_at = ?", id, unmuteAt).Delete(&models.CaptchaMutedUsers{})
	return result.RowsAffected == 1, result.Error
}

func DeleteMutedUser(userID, chatID int64) error {
	return db.DB.Where("user_id = ? AND chat_id = ?", userID, chatID).Delete(&models.CaptchaMutedUsers{}).Error
}

func DeleteMutedUsersByIDs(ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := db.DB.Delete(&models.CaptchaMutedUsers{}, ids)
	return result.RowsAffected, result.Error
}
