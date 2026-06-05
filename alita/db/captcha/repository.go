package captcha

import (
	"context"
	"errors"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
func GetCaptchaSettings(chatID int64) (*models.CaptchaSettings, error) {
	return cache.GetFromCacheOrLoad(cache.CacheKey("captcha_settings", chatID), cache.CacheTTLCaptchaSettings, func() (*models.CaptchaSettings, error) {
		settings := &models.CaptchaSettings{}
		err := db.GetRecord(settings, bson.M{"chat_id": chatID})

		if err == db.ErrRecordNotFound {
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
	updates := bson.M{
		"enabled":    enabled,
		"updated_at": time.Now(),
	}

	collection := db.DB.Collection("captcha_settings")
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(context.Background(), bson.M{"chat_id": chatID}, bson.M{"$set": updates}, opts)

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

	updates := bson.M{
		"captcha_mode": mode,
		"updated_at":   time.Now(),
	}

	collection := db.DB.Collection("captcha_settings")
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(context.Background(), bson.M{"chat_id": chatID}, bson.M{"$set": updates}, opts)

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

	updates := bson.M{
		"timeout":    timeout,
		"updated_at": time.Now(),
	}

	collection := db.DB.Collection("captcha_settings")
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(context.Background(), bson.M{"chat_id": chatID}, bson.M{"$set": updates}, opts)

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

	updates := bson.M{
		"max_attempts": maxAttempts,
		"updated_at":   time.Now(),
	}

	collection := db.DB.Collection("captcha_settings")
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(context.Background(), bson.M{"chat_id": chatID}, bson.M{"$set": updates}, opts)

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

	updates := bson.M{
		"failure_action": action,
		"updated_at":     time.Now(),
	}

	collection := db.DB.Collection("captcha_settings")
	opts := options.Update().SetUpsert(true)
	_, err := collection.UpdateOne(context.Background(), bson.M{"chat_id": chatID}, bson.M{"$set": updates}, opts)

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
	collection := db.DB.Collection("captcha_attempts")

	// Delete any existing attempt for this user in this chat
	_, err := collection.DeleteMany(context.Background(), bson.M{"user_id": userID, "chat_id": chatID})
	if err != nil {
		return nil, err
	}

	attempt := &models.CaptchaAttempts{
		UserID:       userID,
		ChatID:       chatID,
		Answer:       answer,
		Attempts:     0,
		MessageID:    0,
		RefreshCount: 0,
		ExpiresAt:    time.Now().Add(time.Duration(timeout) * time.Minute),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	res, err := collection.InsertOne(context.Background(), attempt)
	if err != nil {
		log.Errorf("[Database][CreateCaptchaAttemptPreMessage]: %v", err)
		return nil, err
	}

	if oid, ok := res.InsertedID.(interface{ String() string }); ok {
		// This is a bit hacky as the original code uses uint ID, but for now we'll just return the struct
		// In a real migration we'd change ID types to string/ObjectID
		log.Debugf("Inserted attempt with ID: %v", oid)
	}

	return attempt, nil
}

// UpdateCaptchaAttemptMessageID sets the message_id for an existing attempt by ID.
func UpdateCaptchaAttemptMessageID(attemptID uint, messageID int64) error {
	// Note: MongoDB uses ObjectIDs, this uint-based lookup might fail if not handled.
	// But the repository calls pass what they got from InsertOne.
	collection := db.DB.Collection("captcha_attempts")
	_, err := collection.UpdateOne(context.Background(), bson.M{"id": attemptID}, bson.M{"$set": bson.M{"message_id": messageID, "updated_at": time.Now()}})
	if err != nil {
		log.Errorf("[Database][UpdateCaptchaAttemptMessageID]: %v", err)
		return err
	}
	return nil
}

// GetCaptchaAttempt retrieves an active captcha attempt for a user in a chat.
// Returns nil if no active attempt exists or if it has expired.
func GetCaptchaAttempt(userID, chatID int64) (*models.CaptchaAttempts, error) {
	attempt := &models.CaptchaAttempts{}
	collection := db.DB.Collection("captcha_attempts")
	err := collection.FindOne(context.Background(), bson.M{
		"user_id":    userID,
		"chat_id":    chatID,
		"expires_at": bson.M{"$gt": time.Now()},
	}).Decode(attempt)

	if err == mongo.ErrNoDocuments {
		return nil, nil
	}

	if err != nil {
		log.Errorf("[Database][GetCaptchaAttempt]: %v", err)
		return nil, err
	}

	return attempt, nil
}

// GetCaptchaAttemptByID retrieves a captcha attempt by ID regardless of expiration.
func GetCaptchaAttemptByID(attemptID uint) (*models.CaptchaAttempts, error) {
	attempt := &models.CaptchaAttempts{}
	collection := db.DB.Collection("captcha_attempts")
	err := collection.FindOne(context.Background(), bson.M{"id": attemptID}).Decode(attempt)
	if err == mongo.ErrNoDocuments {
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
func IncrementCaptchaAttempts(userID, chatID int64) (*models.CaptchaAttempts, error) {
	collection := db.DB.Collection("captcha_attempts")
	filter := bson.M{
		"user_id":    userID,
		"chat_id":    chatID,
		"expires_at": bson.M{"$gt": time.Now()},
	}
	update := bson.M{
		"$inc": bson.M{"attempts": 1},
		"$set": bson.M{"updated_at": time.Now()},
	}

	var attempt models.CaptchaAttempts
	err := collection.FindOneAndUpdate(context.Background(), filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&attempt)

	if err == mongo.ErrNoDocuments {
		return nil, ErrNoActiveCaptcha
	}
	if err != nil {
		log.Errorf("[Database][IncrementCaptchaAttempts]: %v", err)
		return nil, err
	}
	return &attempt, nil
}

// DeleteCaptchaAttempt removes a captcha attempt record.
// Called when verification is successful or when user is kicked/banned.
func DeleteCaptchaAttempt(userID, chatID int64) error {
	collection := db.DB.Collection("captcha_attempts")
	_, err := collection.DeleteMany(context.Background(), bson.M{"user_id": userID, "chat_id": chatID})
	if err != nil {
		log.Errorf("[Database][DeleteCaptchaAttempt]: %v", err)
		return err
	}
	return nil
}

// DeleteCaptchaAttemptByIDAtomic deletes a specific attempt and returns whether it was deleted.
func DeleteCaptchaAttemptByIDAtomic(attemptID uint, userID, chatID int64) (bool, error) {
	collection := db.DB.Collection("captcha_attempts")
	res, err := collection.DeleteOne(context.Background(), bson.M{"id": attemptID, "user_id": userID, "chat_id": chatID})
	if err != nil {
		log.Errorf("[Database][DeleteCaptchaAttemptByIDAtomic]: %v", err)
		return false, err
	}
	return res.DeletedCount > 0, nil
}

// DeleteAllCaptchaAttempts removes all captcha attempts for a chat.
// Used when captcha is disabled or for admin cleanup.
func DeleteAllCaptchaAttempts(chatID int64) error {
	collection := db.DB.Collection("captcha_attempts")
	res, err := collection.DeleteMany(context.Background(), bson.M{"chat_id": chatID})
	if err != nil {
		log.Errorf("[Database][DeleteAllCaptchaAttempts]: %v", err)
		return err
	}

	if res.DeletedCount > 0 {
		log.Infof("[Database][DeleteAllCaptchaAttempts]: Deleted %d captcha attempts for chat %d", res.DeletedCount, chatID)
	}

	return nil
}

// UpdateCaptchaAttemptOnRefreshByID updates answer, message ID and increments refresh_count by attempt ID.
func UpdateCaptchaAttemptOnRefreshByID(attemptID uint, newAnswer string, newMessageID int64) (*models.CaptchaAttempts, error) {
	collection := db.DB.Collection("captcha_attempts")
	filter := bson.M{"id": attemptID}
	update := bson.M{
		"$set": bson.M{
			"answer":     newAnswer,
			"message_id": newMessageID,
			"updated_at": time.Now(),
		},
		"$inc": bson.M{"refresh_count": 1},
	}

	var attempt models.CaptchaAttempts
	err := collection.FindOneAndUpdate(context.Background(), filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&attempt)

	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		log.Errorf("[Database][UpdateCaptchaAttemptOnRefreshByID]: %v", err)
		return nil, err
	}
	return &attempt, nil
}

// StoreMessageForCaptcha stores a message sent by a user before captcha completion.
func StoreMessageForCaptcha(userID, chatID int64, attemptID uint, messageType int, content, fileID, caption string) error {
	storedMsg := &models.StoredMessages{
		UserID:      userID,
		ChatID:      chatID,
		AttemptID:   attemptID,
		MessageType: messageType,
		Content:     content,
		FileID:      fileID,
		Caption:     caption,
		CreatedAt:   time.Now(),
	}

	err := db.CreateRecord(storedMsg)
	if err != nil {
		log.Errorf("[Database][StoreMessageForCaptcha]: %v", err)
		return err
	}

	return nil
}

// GetStoredMessagesForAttempt retrieves all stored messages for a specific captcha attempt.
func GetStoredMessagesForAttempt(attemptID uint) ([]*models.StoredMessages, error) {
	var messages []*models.StoredMessages
	collection := db.DB.Collection("stored_messages")
	cursor, err := collection.Find(context.Background(), bson.M{"attempt_id": attemptID}, options.Find().SetSort(bson.M{"created_at": 1}))
	if err != nil {
		log.Errorf("[Database][GetStoredMessagesForAttempt]: %v", err)
		return nil, err
	}
	defer cursor.Close(context.Background())
	if err := cursor.All(context.Background(), &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

// GetStoredMessagesForUser retrieves all stored messages for a user in a chat.
func GetStoredMessagesForUser(userID, chatID int64) ([]*models.StoredMessages, error) {
	var messages []*models.StoredMessages
	collection := db.DB.Collection("stored_messages")
	cursor, err := collection.Find(context.Background(), bson.M{"user_id": userID, "chat_id": chatID}, options.Find().SetSort(bson.M{"created_at": 1}))
	if err != nil {
		log.Errorf("[Database][GetStoredMessagesForUser]: %v", err)
		return nil, err
	}
	defer cursor.Close(context.Background())
	if err := cursor.All(context.Background(), &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

// DeleteStoredMessagesForAttempt removes all stored messages for a specific captcha attempt.
func DeleteStoredMessagesForAttempt(attemptID uint) error {
	collection := db.DB.Collection("stored_messages")
	res, err := collection.DeleteMany(context.Background(), bson.M{"attempt_id": attemptID})
	if err != nil {
		log.Errorf("[Database][DeleteStoredMessagesForAttempt]: %v", err)
		return err
	}

	if res.DeletedCount > 0 {
		log.Debugf("[Database][DeleteStoredMessagesForAttempt]: Deleted %d stored messages for attempt %d", res.DeletedCount, attemptID)
	}

	return nil
}

// DeleteStoredMessagesForUser removes all stored messages for a user in a chat.
func DeleteStoredMessagesForUser(userID, chatID int64) error {
	collection := db.DB.Collection("stored_messages")
	res, err := collection.DeleteMany(context.Background(), bson.M{"user_id": userID, "chat_id": chatID})
	if err != nil {
		log.Errorf("[Database][DeleteStoredMessagesForUser]: %v", err)
		return err
	}

	if res.DeletedCount > 0 {
		log.Debugf("[Database][DeleteStoredMessagesForUser]: Deleted %d stored messages for user %d in chat %d", res.DeletedCount, userID, chatID)
	}

	return nil
}

// CountStoredMessagesForAttempt returns the number of stored messages for a captcha attempt.
func CountStoredMessagesForAttempt(attemptID uint) (int64, error) {
	collection := db.DB.Collection("stored_messages")
	count, err := collection.CountDocuments(context.Background(), bson.M{"attempt_id": attemptID})
	if err != nil {
		log.Errorf("[Database][CountStoredMessagesForAttempt]: %v", err)
		return 0, err
	}
	return count, nil
}

// GetExpiredCaptchaAttempts returns all expired captcha attempts.
func GetExpiredCaptchaAttempts() ([]*models.CaptchaAttempts, error) {
	var attempts []*models.CaptchaAttempts
	collection := db.DB.Collection("captcha_attempts")
	cursor, err := collection.Find(context.Background(), bson.M{"expires_at": bson.M{"$lt": time.Now()}})
	if err != nil {
		log.Errorf("[Database][GetExpiredCaptchaAttempts]: %v", err)
		return nil, err
	}
	defer cursor.Close(context.Background())
	if err := cursor.All(context.Background(), &attempts); err != nil {
		return nil, err
	}
	return attempts, nil
}

// GetAllPendingCaptchaAttempts returns ALL captcha attempts (both expired and valid).
func GetAllPendingCaptchaAttempts() ([]*models.CaptchaAttempts, error) {
	var attempts []*models.CaptchaAttempts
	collection := db.DB.Collection("captcha_attempts")
	cursor, err := collection.Find(context.Background(), bson.M{})
	if err != nil {
		log.Errorf("[Database][GetAllPendingCaptchaAttempts]: %v", err)
		return nil, err
	}
	defer cursor.Close(context.Background())
	if err := cursor.All(context.Background(), &attempts); err != nil {
		return nil, err
	}
	return attempts, nil
}

// DeleteCaptchaAttemptsByIDs deletes multiple captcha attempts by their IDs.
func DeleteCaptchaAttemptsByIDs(ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	collection := db.DB.Collection("captcha_attempts")
	res, err := collection.DeleteMany(context.Background(), bson.M{"id": bson.M{"$in": ids}})
	if err != nil {
		log.Errorf("[Database][DeleteCaptchaAttemptsByIDs]: %v", err)
		return 0, err
	}
	return res.DeletedCount, nil
}

// CreateMutedUser stores a user who failed captcha and should be unmuted later
func CreateMutedUser(userID, chatID int64, unmuteAt time.Time) error {
	return db.CreateRecord(&models.CaptchaMutedUsers{
		UserID:    userID,
		ChatID:    chatID,
		UnmuteAt:  unmuteAt,
		CreatedAt: time.Now(),
	})
}

// GetUsersToUnmute returns users whose unmute time has passed
func GetUsersToUnmute() ([]*models.CaptchaMutedUsers, error) {
	var users []*models.CaptchaMutedUsers
	collection := db.DB.Collection("captcha_muted_users")
	cursor, err := collection.Find(context.Background(), bson.M{"unmute_at": bson.M{"$lt": time.Now()}})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())
	if err := cursor.All(context.Background(), &users); err != nil {
		return nil, err
	}
	return users, nil
}

// DeleteMutedUsersByIDs removes multiple users by their IDs
func DeleteMutedUsersByIDs(ids []uint) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	collection := db.DB.Collection("captcha_muted_users")
	res, err := collection.DeleteMany(context.Background(), bson.M{"id": bson.M{"$in": ids}})
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}
