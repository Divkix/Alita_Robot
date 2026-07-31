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

// Captcha validation errors
var (
	ErrInvalidCaptchaMode   = errors.New("INVALID_CAPTCHA_MODE")
	ErrInvalidTimeout       = errors.New("INVALID_TIMEOUT_RANGE")
	ErrInvalidFailureAction = errors.New("INVALID_FAILURE_ACTION")
	ErrInvalidMaxAttempts   = errors.New("INVALID_MAX_ATTEMPTS")
	ErrNoActiveCaptcha      = errors.New("NO_ACTIVE_CAPTCHA")
	ErrCaptchaDisabled      = errors.New("CAPTCHA_DISABLED")
)

// GetCaptchaSettings retrieves captcha settings for a chat.
// Returns default settings if the chat doesn't have custom settings.
// Results are cached with stampede protection for performance.
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

// SetCaptchaEnabled enables or disables captcha for a chat.
// Creates settings record if it doesn't exist.
func SetCaptchaEnabled(chatID int64, enabled bool) error {
	// Use map-based update to handle zero values correctly
	updates := map[string]any{
		"chat_id": chatID,
		"enabled": enabled,
	}

	err := db.DB.Where("chat_id = ?", chatID).Assign(updates).FirstOrCreate(&models.CaptchaSettings{}).Error
	if err != nil {
		log.Errorf("[Database][SetCaptchaEnabled]: %v", err)
		return err
	}

	// Invalidate cache after update
	cache.DeleteCache(cache.CacheKey("captcha_settings", chatID))

	return nil
}

// SetCaptchaMode sets the captcha mode (math or text) for a chat.
// Creates settings record if it doesn't exist.
func SetCaptchaMode(chatID int64, mode string) error {
	if mode != "math" && mode != "text" {
		return ErrInvalidCaptchaMode
	}

	// Use map-based update to be consistent
	updates := map[string]any{
		"chat_id":      chatID,
		"captcha_mode": mode,
	}

	err := db.DB.Where("chat_id = ?", chatID).Assign(updates).FirstOrCreate(&models.CaptchaSettings{}).Error
	if err != nil {
		log.Errorf("[Database][SetCaptchaMode]: %v", err)
		return err
	}

	// Invalidate cache after update
	cache.DeleteCache(cache.CacheKey("captcha_settings", chatID))

	return nil
}

// SetCaptchaTimeout sets the timeout duration (in minutes) for captcha verification.
// Creates settings record if it doesn't exist.
func SetCaptchaTimeout(chatID int64, timeout int) error {
	if timeout < 1 || timeout > 10 {
		return ErrInvalidTimeout
	}

	// Use map-based update to be consistent
	updates := map[string]any{
		"chat_id": chatID,
		"timeout": timeout,
	}

	err := db.DB.Where("chat_id = ?", chatID).Assign(updates).FirstOrCreate(&models.CaptchaSettings{}).Error
	if err != nil {
		log.Errorf("[Database][SetCaptchaTimeout]: %v", err)
		return err
	}

	// Invalidate cache after update
	cache.DeleteCache(cache.CacheKey("captcha_settings", chatID))

	return nil
}

// SetCaptchaMaxAttempts sets the maximum number of captcha attempts allowed.
// Creates settings record if it doesn't exist.
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

// SetCaptchaFailureAction sets the action to take when captcha verification fails.
// Valid actions are: kick, ban, mute
func SetCaptchaFailureAction(chatID int64, action string) error {
	if action != "kick" && action != "ban" && action != "mute" {
		return ErrInvalidFailureAction
	}

	// Use map-based update to be consistent
	updates := map[string]any{
		"chat_id":        chatID,
		"failure_action": action,
	}

	err := db.DB.Where("chat_id = ?", chatID).Assign(updates).FirstOrCreate(&models.CaptchaSettings{}).Error
	if err != nil {
		log.Errorf("[Database][SetCaptchaFailureAction]: %v", err)
		return err
	}

	// Invalidate cache after update
	cache.DeleteCache(cache.CacheKey("captcha_settings", chatID))

	return nil
}

// CreateCaptchaAttemptPreMessage creates a captcha attempt before sending a message,
// setting message_id to 0 temporarily and returning the created attempt with ID.
func CreateCaptchaAttemptPreMessage(userID, chatID int64, answer string, timeout int) (*models.CaptchaAttempts, error) {
	return createCaptchaAttemptPreMessage(userID, chatID, answer, timeout, false)
}

// CreateCaptchaAttemptPreMessageIfEnabled serializes attempt creation with
// captcha disablement so a disabled chat cannot gain a new pending challenge.
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

	// Use a transaction to ensure atomicity
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

		// Create the new attempt
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

// UpdateCaptchaAttemptMessageID sets the message_id for an existing attempt by ID.
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

// GetCaptchaAttempt retrieves an active captcha attempt for a user in a chat.
// Returns nil if no active attempt exists or if it has expired.
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

// GetCaptchaAttemptIncludingExpired retrieves the latest attempt for cleanup paths.
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

// GetCaptchaAttemptByID retrieves a captcha attempt by ID regardless of expiration.
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

// IncrementCaptchaAttempts increments only the challenge version the callback
// was rendered for. A concurrent refresh makes the old keyboard stale.
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

// DeleteCaptchaAttempt removes a captcha attempt record.
// Called when verification is successful or when user is kicked/banned.
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

// DeleteCaptchaAttemptByIDAtomic deletes a specific attempt and returns whether it was deleted.
// The userID/chatID filter prevents deleting another attempt with the same ID unexpectedly.
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

// CompleteCaptchaAttemptAtomic claims an unexpired attempt only if its answer is
// still current. This prevents a stale answer racing a captcha refresh.
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

// ReleaseCaptchaAttemptAtomic claims an attempt and durably schedules an
// immediate permission restore in the same transaction.
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

// DeleteAllCaptchaAttempts removes all captcha attempts for a chat.
// Used when captcha is disabled or for admin cleanup.
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

// UpdateCaptchaAttemptOnRefreshByID replaces a challenge only if it has not
// changed since the caller read it.
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

// StoreMessageForCaptcha stores a message sent by a user before captcha completion.
// This allows the bot to track what users were trying to send before verification.
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

// GetStoredMessagesForAttempt retrieves all stored messages for a specific captcha attempt.
// Used to show what the user tried to send before verification.
func GetStoredMessagesForAttempt(attemptID uint) ([]*models.StoredMessages, error) {
	var messages []*models.StoredMessages
	err := db.DB.Where("attempt_id = ?", attemptID).Order("created_at ASC").Find(&messages).Error
	if err != nil {
		log.Errorf("[Database][GetStoredMessagesForAttempt]: %v", err)
		return nil, err
	}
	return messages, nil
}

// GetStoredMessagesForUser retrieves all stored messages for a user in a chat.
// Used to get all pending messages when the user completes captcha.
func GetStoredMessagesForUser(userID, chatID int64) ([]*models.StoredMessages, error) {
	var messages []*models.StoredMessages
	err := db.DB.Where("user_id = ? AND chat_id = ?", userID, chatID).Order("created_at ASC").Find(&messages).Error
	if err != nil {
		log.Errorf("[Database][GetStoredMessagesForUser]: %v", err)
		return nil, err
	}
	return messages, nil
}

// DeleteStoredMessagesForAttempt removes all stored messages for a specific captcha attempt.
// Called when captcha is completed successfully or when user is kicked/banned.
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

// DeleteStoredMessagesForUser removes all stored messages for a user in a chat.
// Alternative cleanup method when cleaning up by user instead of attempt.
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

// CountStoredMessagesForAttempt returns the number of stored messages for a captcha attempt.
// Used to show summary information in timeout/failure messages.
func CountStoredMessagesForAttempt(attemptID uint) (int64, error) {
	var count int64
	err := db.DB.Model(&models.StoredMessages{}).Where("attempt_id = ?", attemptID).Count(&count).Error
	if err != nil {
		log.Errorf("[Database][CountStoredMessagesForAttempt]: %v", err)
		return 0, err
	}
	return count, nil
}

// GetExpiredCaptchaAttempts returns all expired captcha attempts.
// Used for cleanup to delete Telegram messages before DB cleanup.
func GetExpiredCaptchaAttempts() ([]*models.CaptchaAttempts, error) {
	var attempts []*models.CaptchaAttempts
	err := db.DB.Where("expires_at < ?", time.Now()).Find(&attempts).Error
	if err != nil {
		log.Errorf("[Database][GetExpiredCaptchaAttempts]: %v", err)
		return nil, err
	}
	return attempts, nil
}

// GetAllPendingCaptchaAttempts returns ALL captcha attempts (both expired and valid).
// Used for startup recovery after bot restart.
func GetAllPendingCaptchaAttempts() ([]*models.CaptchaAttempts, error) {
	var attempts []*models.CaptchaAttempts
	err := db.DB.Find(&attempts).Error
	if err != nil {
		log.Errorf("[Database][GetAllPendingCaptchaAttempts]: %v", err)
		return nil, err
	}
	return attempts, nil
}

// GetCaptchaAttemptsForChat returns every pending attempt for a chat.
func GetCaptchaAttemptsForChat(chatID int64) ([]*models.CaptchaAttempts, error) {
	var attempts []*models.CaptchaAttempts
	if err := db.DB.Where("chat_id = ?", chatID).Find(&attempts).Error; err != nil {
		log.Errorf("[Database][GetCaptchaAttemptsForChat]: %v", err)
		return nil, err
	}
	return attempts, nil
}

// DeleteCaptchaAttemptsByIDs deletes multiple captcha attempts by their IDs.
// Returns the number of deleted rows.
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

// CreateMutedUser stores a user who failed captcha and should be unmuted later
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

// GetUsersToUnmute returns users whose unmute time has passed
func GetUsersToUnmute() ([]*models.CaptchaMutedUsers, error) {
	var users []*models.CaptchaMutedUsers
	err := db.DB.Where("unmute_at < ?", time.Now()).Find(&users).Error
	return users, err
}

// GetMutedUsersForChat returns captcha mutes owned by a chat.
func GetMutedUsersForChat(chatID int64) ([]*models.CaptchaMutedUsers, error) {
	var users []*models.CaptchaMutedUsers
	err := db.DB.Where("chat_id = ?", chatID).Find(&users).Error
	return users, err
}

// GetMutedUser returns the current unmute schedule for a user in a chat.
func GetMutedUser(userID, chatID int64) (*models.CaptchaMutedUsers, error) {
	var user models.CaptchaMutedUsers
	err := db.DB.Where("user_id = ? AND chat_id = ?", userID, chatID).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &user, err
}

// DeleteMutedUserIfUnchanged removes only the schedule version a worker read.
func DeleteMutedUserIfUnchanged(id uint, unmuteAt time.Time) (bool, error) {
	result := db.DB.Where("id = ? AND unmute_at = ?", id, unmuteAt).Delete(&models.CaptchaMutedUsers{})
	return result.RowsAffected == 1, result.Error
}

// DeleteMutedUser removes every scheduled captcha unmute for a user in a chat.
func DeleteMutedUser(userID, chatID int64) error {
	return db.DB.Where("user_id = ? AND chat_id = ?", userID, chatID).Delete(&models.CaptchaMutedUsers{}).Error
}

// DeleteMutedUsersByIDs removes multiple users by their IDs
func DeleteMutedUsersByIDs(ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	result := db.DB.Delete(&models.CaptchaMutedUsers{}, ids)
	return result.RowsAffected, result.Error
}
