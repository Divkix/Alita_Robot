package db

import (
	"errors"
	"time"

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
)

// GetCaptchaSettings retrieves captcha settings for a chat.
// Returns default settings if the chat doesn't have custom settings.
// Results are cached with stampede protection for performance.
func GetCaptchaSettings(chatID int64) (*CaptchaSettings, error) {
	return getFromCacheOrLoad(captchaSettingsCacheKey(chatID), CacheTTLCaptchaSettings, func() (*CaptchaSettings, error) {
		settings := &CaptchaSettings{}
		err := GetRecord(settings, map[string]any{"chat_id": chatID})

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &CaptchaSettings{
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

	err := DB.Where("chat_id = ?", chatID).Assign(updates).FirstOrCreate(&CaptchaSettings{}).Error
	if err != nil {
		log.Errorf("[Database][SetCaptchaEnabled]: %v", err)
		return err
	}

	// Invalidate cache after update
	deleteCache(captchaSettingsCacheKey(chatID))

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

	err := DB.Where("chat_id = ?", chatID).Assign(updates).FirstOrCreate(&CaptchaSettings{}).Error
	if err != nil {
		log.Errorf("[Database][SetCaptchaMode]: %v", err)
		return err
	}

	// Invalidate cache after update
	deleteCache(captchaSettingsCacheKey(chatID))

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

	err := DB.Where("chat_id = ?", chatID).Assign(updates).FirstOrCreate(&CaptchaSettings{}).Error
	if err != nil {
		log.Errorf("[Database][SetCaptchaTimeout]: %v", err)
		return err
	}

	// Invalidate cache after update
	deleteCache(captchaSettingsCacheKey(chatID))

	return nil
}

// SetCaptchaMaxAttempts sets the maximum number of captcha attempts allowed.
// Creates settings record if it doesn't exist.
func SetCaptchaMaxAttempts(chatID int64, maxAttempts int) error {
	if maxAttempts < 1 || maxAttempts > 10 {
		return ErrInvalidMaxAttempts
	}

	err := DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{"max_attempts": maxAttempts}),
	}).Create(&CaptchaSettings{ChatID: chatID, MaxAttempts: maxAttempts}).Error
	if err != nil {
		log.Errorf("[Database][SetCaptchaMaxAttempts]: %v", err)
		return err
	}

	deleteCache(captchaSettingsCacheKey(chatID))
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

	err := DB.Where("chat_id = ?", chatID).Assign(updates).FirstOrCreate(&CaptchaSettings{}).Error
	if err != nil {
		log.Errorf("[Database][SetCaptchaFailureAction]: %v", err)
		return err
	}

	// Invalidate cache after update
	deleteCache(captchaSettingsCacheKey(chatID))

	return nil
}

// CreateCaptchaAttemptPreMessage creates a captcha attempt before sending a message,
// setting message_id to 0 temporarily and returning the created attempt with ID.
func CreateCaptchaAttemptPreMessage(userID, chatID int64, answer string, timeout int) (*CaptchaAttempts, error) {
	attempt := &CaptchaAttempts{
		UserID:       userID,
		ChatID:       chatID,
		Answer:       answer,
		Attempts:     0,
		MessageID:    0,
		RefreshCount: 0,
		ExpiresAt:    time.Now().Add(time.Duration(timeout) * time.Minute),
	}

	// Use a transaction to ensure atomicity
	err := DB.Transaction(func(tx *gorm.DB) error {
		// Delete any existing attempt for this user in this chat
		if err := tx.Where("user_id = ? AND chat_id = ?", userID, chatID).Delete(&CaptchaAttempts{}).Error; err != nil {
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
	err := DB.Model(&CaptchaAttempts{}).Where("id = ?", attemptID).Update("message_id", messageID).Error
	if err != nil {
		log.Errorf("[Database][UpdateCaptchaAttemptMessageID]: %v", err)
		return err
	}
	return nil
}

// GetCaptchaAttempt retrieves an active captcha attempt for a user in a chat.
// Returns nil if no active attempt exists or if it has expired.
func GetCaptchaAttempt(userID, chatID int64) (*CaptchaAttempts, error) {
	attempt := &CaptchaAttempts{}
	err := DB.Where("user_id = ? AND chat_id = ? AND expires_at > ?",
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

// GetCaptchaAttemptByID retrieves a captcha attempt by ID regardless of expiration.
func GetCaptchaAttemptByID(attemptID uint) (*CaptchaAttempts, error) {
	attempt := &CaptchaAttempts{}
	err := DB.First(attempt, attemptID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		log.Errorf("[Database][GetCaptchaAttemptByID]: %v", err)
		return nil, err
	}
	return attempt, nil
}

// IncrementCaptchaAttempts increments the attempt counter for a captcha.
// Returns the updated attempt record.
// Uses SELECT FOR UPDATE to prevent race conditions on concurrent requests.
func IncrementCaptchaAttempts(userID, chatID int64) (*CaptchaAttempts, error) {
	var attempt CaptchaAttempts
	err := DB.Transaction(func(tx *gorm.DB) error {
		// Use SELECT FOR UPDATE to lock the row and prevent concurrent modifications
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND chat_id = ? AND expires_at > ?", userID, chatID, time.Now()).
			First(&attempt).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNoActiveCaptcha
			}
			return err
		}
		attempt.Attempts++
		return tx.Save(&attempt).Error
	})
	if err != nil {
		if !errors.Is(err, ErrNoActiveCaptcha) {
			log.Errorf("[Database][IncrementCaptchaAttempts]: %v", err)
		}
		return nil, err
	}
	return &attempt, nil
}

// DeleteCaptchaAttempt removes a captcha attempt record.
// Called when verification is successful or when user is kicked/banned.
func DeleteCaptchaAttempt(userID, chatID int64) error {
	result := DB.Where("user_id = ? AND chat_id = ?", userID, chatID).Delete(&CaptchaAttempts{})
	if result.Error != nil {
		log.Errorf("[Database][DeleteCaptchaAttempt]: %v", result.Error)
		return result.Error
	}
	return nil
}

// DeleteCaptchaAttemptAtomic deletes an attempt and returns whether it was deleted.
// This is used to prevent race conditions between timeout and success handlers.
// Returns true if a row was deleted, false if no row existed.
func DeleteCaptchaAttemptAtomic(userID, chatID int64) (bool, error) {
	result := DB.Where("user_id = ? AND chat_id = ?", userID, chatID).Delete(&CaptchaAttempts{})
	if result.Error != nil {
		log.Errorf("[Database][DeleteCaptchaAttemptAtomic]: %v", result.Error)
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// DeleteCaptchaAttemptByIDAtomic deletes a specific attempt and returns whether it was deleted.
// The userID/chatID filter prevents deleting another attempt with the same ID unexpectedly.
func DeleteCaptchaAttemptByIDAtomic(attemptID uint, userID, chatID int64) (bool, error) {
	result := DB.Where("id = ? AND user_id = ? AND chat_id = ?", attemptID, userID, chatID).Delete(&CaptchaAttempts{})
	if result.Error != nil {
		log.Errorf("[Database][DeleteCaptchaAttemptByIDAtomic]: %v", result.Error)
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// CleanupExpiredCaptchaAttempts removes all expired captcha attempts from the database.
// This should be called periodically to clean up old records.
func CleanupExpiredCaptchaAttempts() (int64, error) {
	result := DB.Where("expires_at < ?", time.Now()).Delete(&CaptchaAttempts{})
	if result.Error != nil {
		log.Errorf("[Database][CleanupExpiredCaptchaAttempts]: %v", result.Error)
		return 0, result.Error
	}

	if result.RowsAffected > 0 {
		log.Infof("[Database][CleanupExpiredCaptchaAttempts]: Cleaned up %d expired captcha attempts", result.RowsAffected)
	}

	return result.RowsAffected, nil
}

// DeleteAllCaptchaAttempts removes all captcha attempts for a chat.
// Used when captcha is disabled or for admin cleanup.
func DeleteAllCaptchaAttempts(chatID int64) error {
	result := DB.Where("chat_id = ?", chatID).Delete(&CaptchaAttempts{})
	if result.Error != nil {
		log.Errorf("[Database][DeleteAllCaptchaAttempts]: %v", result.Error)
		return result.Error
	}

	if result.RowsAffected > 0 {
		log.Infof("[Database][DeleteAllCaptchaAttempts]: Deleted %d captcha attempts for chat %d", result.RowsAffected, chatID)
	}

	return nil
}

// UpdateCaptchaAttemptOnRefreshByID updates answer, message ID and increments refresh_count by attempt ID.
func UpdateCaptchaAttemptOnRefreshByID(attemptID uint, newAnswer string, newMessageID int64) (*CaptchaAttempts, error) {
	attempt := &CaptchaAttempts{}
	err := DB.First(attempt, attemptID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		log.Errorf("[Database][UpdateCaptchaAttemptOnRefreshByID:Find]: %v", err)
		return nil, err
	}

	updates := map[string]any{
		"answer":        newAnswer,
		"message_id":    newMessageID,
		"refresh_count": gorm.Expr("COALESCE(refresh_count, 0) + 1"),
	}
	if err := UpdateRecord(&CaptchaAttempts{}, map[string]any{"id": attemptID}, updates); err != nil {
		return nil, err
	}
	// Reload
	err = DB.First(attempt, attemptID).Error
	if err != nil {
		log.Errorf("[Database][UpdateCaptchaAttemptOnRefreshByID:Reload]: %v", err)
		return nil, err
	}
	return attempt, nil
}

// StoreMessageForCaptcha stores a message sent by a user before captcha completion.
// This allows the bot to track what users were trying to send before verification.
func StoreMessageForCaptcha(userID, chatID int64, attemptID uint, messageType int, content, fileID, caption string) error {
	storedMsg := &StoredMessages{
		UserID:      userID,
		ChatID:      chatID,
		AttemptID:   attemptID,
		MessageType: messageType,
		Content:     content,
		FileID:      fileID,
		Caption:     caption,
	}

	err := DB.Create(storedMsg).Error
	if err != nil {
		log.Errorf("[Database][StoreMessageForCaptcha]: %v", err)
		return err
	}

	return nil
}

// GetStoredMessagesForAttempt retrieves all stored messages for a specific captcha attempt.
// Used to show what the user tried to send before verification.
func GetStoredMessagesForAttempt(attemptID uint) ([]*StoredMessages, error) {
	var messages []*StoredMessages
	err := DB.Where("attempt_id = ?", attemptID).Order("created_at ASC").Find(&messages).Error
	if err != nil {
		log.Errorf("[Database][GetStoredMessagesForAttempt]: %v", err)
		return nil, err
	}
	return messages, nil
}

// GetStoredMessagesForUser retrieves all stored messages for a user in a chat.
// Used to get all pending messages when the user completes captcha.
func GetStoredMessagesForUser(userID, chatID int64) ([]*StoredMessages, error) {
	var messages []*StoredMessages
	err := DB.Where("user_id = ? AND chat_id = ?", userID, chatID).Order("created_at ASC").Find(&messages).Error
	if err != nil {
		log.Errorf("[Database][GetStoredMessagesForUser]: %v", err)
		return nil, err
	}
	return messages, nil
}

// DeleteStoredMessagesForAttempt removes all stored messages for a specific captcha attempt.
// Called when captcha is completed successfully or when user is kicked/banned.
func DeleteStoredMessagesForAttempt(attemptID uint) error {
	result := DB.Where("attempt_id = ?", attemptID).Delete(&StoredMessages{})
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
	result := DB.Where("user_id = ? AND chat_id = ?", userID, chatID).Delete(&StoredMessages{})
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
	err := DB.Model(&StoredMessages{}).Where("attempt_id = ?", attemptID).Count(&count).Error
	if err != nil {
		log.Errorf("[Database][CountStoredMessagesForAttempt]: %v", err)
		return 0, err
	}
	return count, nil
}

// GetExpiredCaptchaAttempts returns all expired captcha attempts.
// Used for cleanup to delete Telegram messages before DB cleanup.
func GetExpiredCaptchaAttempts() ([]*CaptchaAttempts, error) {
	var attempts []*CaptchaAttempts
	err := DB.Where("expires_at < ?", time.Now()).Find(&attempts).Error
	if err != nil {
		log.Errorf("[Database][GetExpiredCaptchaAttempts]: %v", err)
		return nil, err
	}
	return attempts, nil
}

// GetAllPendingCaptchaAttempts returns ALL captcha attempts (both expired and valid).
// Used for startup recovery after bot restart.
func GetAllPendingCaptchaAttempts() ([]*CaptchaAttempts, error) {
	var attempts []*CaptchaAttempts
	err := DB.Find(&attempts).Error
	if err != nil {
		log.Errorf("[Database][GetAllPendingCaptchaAttempts]: %v", err)
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
	result := DB.Where("id IN ?", ids).Delete(&CaptchaAttempts{})
	if result.Error != nil {
		log.Errorf("[Database][DeleteCaptchaAttemptsByIDs]: %v", result.Error)
		return 0, result.Error
	}
	return result.RowsAffected, nil
}

// CreateMutedUser stores a user who failed captcha and should be unmuted later
func CreateMutedUser(userID, chatID int64, unmuteAt time.Time) error {
	return DB.Create(&CaptchaMutedUsers{
		UserID:   userID,
		ChatID:   chatID,
		UnmuteAt: unmuteAt,
	}).Error
}

// GetUsersToUnmute returns users whose unmute time has passed
func GetUsersToUnmute() ([]*CaptchaMutedUsers, error) {
	var users []*CaptchaMutedUsers
	err := DB.Where("unmute_at < ?", time.Now()).Find(&users).Error
	return users, err
}

// DeleteMutedUser removes a user from the muted users table
func DeleteMutedUser(id uint) error {
	return DB.Delete(&CaptchaMutedUsers{}, id).Error
}

// DeleteMutedUsersByIDs removes multiple users by their IDs
func DeleteMutedUsersByIDs(ids []uint) (int64, error) {
	result := DB.Delete(&CaptchaMutedUsers{}, ids)
	return result.RowsAffected, result.Error
}
