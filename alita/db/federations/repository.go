package federations

import (
	"errors"
	"strings"
	"time"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	utilsCache "github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	maxFederationNameLength = 64
	maxSubscriptions        = 5
)

// ErrFederationNotFound is returned when a federation lookup fails.
var ErrFederationNotFound = errors.New("federation not found")

// ErrAlreadyFederationOwner is returned when a user already owns a federation.
var ErrAlreadyFederationOwner = errors.New("user already owns a federation")

// ErrChatAlreadyInFederation is returned when a chat is already joined to a federation.
var ErrChatAlreadyInFederation = errors.New("chat already in a federation")

// ErrMaxSubscriptionsReached is returned when a federation already has the maximum allowed subscriptions.
var ErrMaxSubscriptionsReached = errors.New("maximum subscriptions reached")

// ErrSelfSubscription is returned when a federation tries to subscribe to itself.
var ErrSelfSubscription = errors.New("cannot subscribe to the same federation")

// normalizeFederationName trims whitespace and truncates to the max length.
func normalizeFederationName(name string) string {
	name = strings.TrimSpace(name)
	if len(name) > maxFederationNameLength {
		name = name[:maxFederationNameLength]
	}
	return name
}

// generateFedID creates a new Rose-compatible UUID federation ID.
func generateFedID() string {
	return uuid.New().String()
}

// invalidateAdminCache removes the per-user is_admin cache entry.
func invalidateAdminCache(fedID string, userID int64) {
	cache.DeleteCache(cache.CacheKey("federation", "is_admin", fedID, userID))
}

// invalidateFederationCache removes all cached entries for a federation.
func invalidateFederationCache(fedID string) {
	m := utilsCache.GetMarshal()
	if m == nil {
		return
	}

	// Also invalidate owner and chat-federation caches.
	fed, err := GetFederationByID(fedID)
	if err == nil {
		invalidateOwnerCache(fed.OwnerID)
		chatIDs, chatErr := ListFederationChats(fedID)
		if chatErr == nil {
			for _, chatID := range chatIDs {
				invalidateChatFederationCache(chatID)
			}
		}
	}

	keys := []string{
		cache.CacheKey("federation", "fed_id", fedID),
		cache.CacheKey("federation", "admins", fedID),
		cache.CacheKey("federation", "settings", fedID),
		cache.CacheKey("federation", "chats", fedID),
		cache.CacheKey("federation", "subs", fedID),
		cache.CacheKey("federation", "bans", fedID),
	}

	for _, key := range keys {
		if err := m.Delete(utilsCache.Context, key); err != nil {
			log.Debugf("[Cache] Failed to delete federation cache key %s: %v", key, err)
		}
	}
}

// invalidateOwnerCache removes the cached federation lookup by owner.
func invalidateOwnerCache(ownerID int64) {
	cache.DeleteCache(cache.CacheKey("federation", "owner", ownerID))
}

// invalidateChatFederationCache removes cached federation membership for a chat.
func invalidateChatFederationCache(chatID int64) {
	cache.DeleteCache(cache.CacheKey("federation", "chat", chatID))
}

// CreateFederation creates a new federation owned by the given user.
func CreateFederation(ownerID int64, name string) (*models.Federation, error) {
	name = normalizeFederationName(name)
	if name == "" {
		return nil, errors.New("federation name cannot be empty")
	}

	fed := models.Federation{
		FedID:   generateFedID(),
		Name:    name,
		OwnerID: ownerID,
	}

	err := db.DB.Create(&fed).Error
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrAlreadyFederationOwner
		}
		log.Errorf("[Database] CreateFederation: %v", err)
		return nil, err
	}

	// Create default settings row.
	settings := models.FederationSettings{FederationID: fed.ID}
	if err := db.DB.Create(&settings).Error; err != nil {
		log.Errorf("[Database] CreateFederation default settings: %v", err)
	}

	invalidateOwnerCache(ownerID)
	return &fed, nil
}

// RenameFederation updates the name of an existing federation.
func RenameFederation(fedID string, newName string) error {
	newName = normalizeFederationName(newName)
	if newName == "" {
		return errors.New("federation name cannot be empty")
	}

	result := db.DB.Model(&models.Federation{}).
		Where("fed_id = ?", fedID).
		Update("name", newName)
	if result.Error != nil {
		log.Errorf("[Database] RenameFederation: %v", result.Error)
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrFederationNotFound
	}

	invalidateFederationCache(fedID)
	return nil
}

// DeleteFederation removes a federation and all related data.
func DeleteFederation(fedID string) error {
	var fed models.Federation
	if err := db.DB.Where("fed_id = ?", fedID).First(&fed).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFederationNotFound
		}
		log.Errorf("[Database] DeleteFederation lookup: %v", err)
		return err
	}

	if err := db.DB.Where("fed_id = ?", fedID).Delete(&models.Federation{}).Error; err != nil {
		log.Errorf("[Database] DeleteFederation: %v", err)
		return err
	}

	invalidateFederationCache(fedID)
	invalidateOwnerCache(fed.OwnerID)
	return nil
}

// GetFederationByID retrieves a federation by its public fed_id.
func GetFederationByID(fedID string) (*models.Federation, error) {
	var fed models.Federation
	if err := db.DB.Where("fed_id = ?", fedID).First(&fed).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFederationNotFound
		}
		log.Errorf("[Database] GetFederationByID: %v", err)
		return nil, err
	}
	return &fed, nil
}

// GetFederationByOwner retrieves the federation owned by a user.
func GetFederationByOwner(ownerID int64) (*models.Federation, error) {
	var fed models.Federation
	if err := db.DB.Where("owner_id = ?", ownerID).First(&fed).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFederationNotFound
		}
		log.Errorf("[Database] GetFederationByOwner: %v", err)
		return nil, err
	}
	return &fed, nil
}

// AddAdmin promotes a user to federation admin.
func AddAdmin(fedID string, userID int64, promotedBy int64) error {
	fed, err := GetFederationByID(fedID)
	if err != nil {
		return err
	}

	record := models.FederationAdmin{
		FederationID: fed.ID,
		UserID:       userID,
		PromotedBy:   promotedBy,
	}

	err = db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "federation_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"promoted_by"}),
	}).Create(&record).Error
	if err != nil {
		log.Errorf("[Database] AddAdmin: %v", err)
		return err
	}

	invalidateFederationCache(fedID)
	invalidateAdminCache(fedID, userID)
	return nil
}

// RemoveAdmin demotes a federation admin.
func RemoveAdmin(fedID string, userID int64) error {
	fed, err := GetFederationByID(fedID)
	if err != nil {
		return err
	}

	result := db.DB.Where("federation_id = ? AND user_id = ?", fed.ID, userID).Delete(&models.FederationAdmin{})
	if result.Error != nil {
		log.Errorf("[Database] RemoveAdmin: %v", result.Error)
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrFederationNotFound
	}

	invalidateFederationCache(fedID)
	invalidateAdminCache(fedID, userID)
	return nil
}

// ListAdmins returns all admin records for a federation.
func ListAdmins(fedID string) ([]models.FederationAdmin, error) {
	fed, err := GetFederationByID(fedID)
	if err != nil {
		return nil, err
	}

	var admins []models.FederationAdmin
	if err := db.DB.Where("federation_id = ?", fed.ID).Find(&admins).Error; err != nil {
		log.Errorf("[Database] ListAdmins: %v", err)
		return nil, err
	}
	return admins, nil
}

// IsAdmin reports whether a user is a federation admin or owner.
func IsAdmin(fedID string, userID int64) (bool, error) {
	fed, err := GetFederationByID(fedID)
	if err != nil {
		return false, err
	}

	if fed.OwnerID == userID {
		return true, nil
	}

	var count int64
	if err := db.DB.Model(&models.FederationAdmin{}).
		Where("federation_id = ? AND user_id = ?", fed.ID, userID).
		Count(&count).Error; err != nil {
		log.Errorf("[Database] IsAdmin: %v", err)
		return false, err
	}
	return count > 0, nil
}

// IsOwner reports whether a user owns the federation.
func IsOwner(fedID string, userID int64) (bool, error) {
	fed, err := GetFederationByID(fedID)
	if err != nil {
		return false, err
	}
	return fed.OwnerID == userID, nil
}

// JoinChat adds a chat to a federation.
func JoinChat(fedID string, chatID int64, joinedBy int64) error {
	fed, err := GetFederationByID(fedID)
	if err != nil {
		return err
	}

	// Ensure the chat is not already in another federation.
	var existing models.FederationChat
	err = db.DB.Where("chat_id = ?", chatID).First(&existing).Error
	if err == nil {
		return ErrChatAlreadyInFederation
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Errorf("[Database] JoinChat existing lookup: %v", err)
		return err
	}

	record := models.FederationChat{
		FederationID: fed.ID,
		ChatID:       chatID,
		JoinedBy:     joinedBy,
	}

	if err := db.DB.Create(&record).Error; err != nil {
		if isUniqueViolation(err) {
			return ErrChatAlreadyInFederation
		}
		log.Errorf("[Database] JoinChat: %v", err)
		return err
	}

	invalidateFederationCache(fedID)
	invalidateChatFederationCache(chatID)
	return nil
}

// LeaveChat removes a chat from its federation.
func LeaveChat(chatID int64) error {
	var chat models.FederationChat
	if err := db.DB.Where("chat_id = ?", chatID).First(&chat).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFederationNotFound
		}
		log.Errorf("[Database] LeaveChat lookup: %v", err)
		return err
	}

	if err := db.DB.Where("chat_id = ?", chatID).Delete(&models.FederationChat{}).Error; err != nil {
		log.Errorf("[Database] LeaveChat: %v", err)
		return err
	}

	invalidateChatFederationCache(chatID)
	fed, err := GetFederationByIDFromDB(chat.ID)
	if err == nil {
		invalidateFederationCache(fed.FedID)
	}
	return nil
}

// GetChatFederation returns the federation a chat is joined to.
func GetChatFederation(chatID int64) (*models.Federation, error) {
	var chatFed models.FederationChat
	if err := db.DB.Where("chat_id = ?", chatID).First(&chatFed).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFederationNotFound
		}
		log.Errorf("[Database] GetChatFederation: %v", err)
		return nil, err
	}

	var fed models.Federation
	if err := db.DB.Where("id = ?", chatFed.FederationID).First(&fed).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFederationNotFound
		}
		log.Errorf("[Database] GetChatFederation load: %v", err)
		return nil, err
	}
	return &fed, nil
}

// ListFederationChats returns chat IDs belonging to a federation.
func ListFederationChats(fedID string) ([]int64, error) {
	fed, err := GetFederationByID(fedID)
	if err != nil {
		return nil, err
	}

	var chatIDs []int64
	if err := db.DB.Model(&models.FederationChat{}).
		Where("federation_id = ?", fed.ID).
		Pluck("chat_id", &chatIDs).Error; err != nil {
		log.Errorf("[Database] ListFederationChats: %v", err)
		return nil, err
	}
	return chatIDs, nil
}

// SetQuiet toggles quiet mode for a federation chat.
func SetQuiet(chatID int64, quiet bool) error {
	var chat models.FederationChat
	if err := db.DB.Where("chat_id = ?", chatID).First(&chat).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFederationNotFound
		}
		log.Errorf("[Database] SetQuiet lookup: %v", err)
		return err
	}

	if err := db.DB.Model(&models.FederationChat{}).
		Where("chat_id = ?", chatID).
		Update("quiet", quiet).Error; err != nil {
		log.Errorf("[Database] SetQuiet: %v", err)
		return err
	}

	invalidateChatFederationCache(chatID)
	fed, err := GetFederationByIDFromDB(chat.FederationID)
	if err == nil {
		invalidateFederationCache(fed.FedID)
	}
	return nil
}

// IsQuiet reports whether a chat has quiet mode enabled.
func IsQuiet(chatID int64) (bool, error) {
	var chat models.FederationChat
	if err := db.DB.Where("chat_id = ?", chatID).First(&chat).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, ErrFederationNotFound
		}
		log.Errorf("[Database] IsQuiet: %v", err)
		return false, err
	}
	return chat.Quiet, nil
}

// GetFederationsByAdmin returns all federations where the user is owner or admin.
func GetFederationsByAdmin(userID int64) ([]models.Federation, error) {
	var fedIDs []uint
	if err := db.DB.Model(&models.FederationAdmin{}).
		Where("user_id = ?", userID).
		Pluck("federation_id", &fedIDs).Error; err != nil {
		log.Errorf("[Database] GetFederationsByAdmin admin lookup: %v", err)
		return nil, err
	}

	var feds []models.Federation
	if err := db.DB.Where("owner_id = ? OR id IN ?", userID, fedIDs).Find(&feds).Error; err != nil {
		log.Errorf("[Database] GetFederationsByAdmin: %v", err)
		return nil, err
	}
	return feds, nil
}

// GetFederationByIDFromDB is used when only the internal database ID is available.
func GetFederationByIDFromDB(id uint) (*models.Federation, error) {
	var fed models.Federation
	if err := db.DB.Where("id = ?", id).First(&fed).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFederationNotFound
		}
		log.Errorf("[Database] GetFederationByIDFromDB: %v", err)
		return nil, err
	}
	return &fed, nil
}

// isUniqueViolation returns true if the error is a PostgreSQL unique violation.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "unique constraint") ||
		strings.Contains(err.Error(), "duplicate key")
}

// BanUser bans a user in a federation.
func BanUser(fedID string, userID int64, reason string, bannedBy int64) error {
	fed, err := GetFederationByID(fedID)
	if err != nil {
		return err
	}

	record := models.FederationBan{
		FederationID: fed.ID,
		UserID:       userID,
		Reason:       reason,
		BannedBy:     bannedBy,
	}

	err = db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "federation_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"reason", "banned_by", "updated_at"}),
	}).Create(&record).Error
	if err != nil {
		log.Errorf("[Database] BanUser: %v", err)
		return err
	}

	invalidateFederationCache(fedID)
	return nil
}

// UnbanUser removes a ban for a user in a federation.
func UnbanUser(fedID string, userID int64) error {
	fed, err := GetFederationByID(fedID)
	if err != nil {
		return err
	}

	result := db.DB.Where("federation_id = ? AND user_id = ?", fed.ID, userID).Delete(&models.FederationBan{})
	if result.Error != nil {
		log.Errorf("[Database] UnbanUser: %v", result.Error)
		return result.Error
	}

	invalidateFederationCache(fedID)
	return nil
}

// IsBanned reports whether a user is banned in a federation and returns the reason.
func IsBanned(fedID string, userID int64) (bool, string, error) {
	fed, err := GetFederationByID(fedID)
	if err != nil {
		return false, "", err
	}

	var ban models.FederationBan
	err = db.DB.Where("federation_id = ? AND user_id = ?", fed.ID, userID).First(&ban).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, "", nil
		}
		log.Errorf("[Database] IsBanned: %v", err)
		return false, "", err
	}
	return true, ban.Reason, nil
}

// GetBanReason returns the reason and ban time for a user in a federation.
func GetBanReason(fedID string, userID int64) (string, time.Time, error) {
	fed, err := GetFederationByID(fedID)
	if err != nil {
		return "", time.Time{}, err
	}

	var ban models.FederationBan
	err = db.DB.Where("federation_id = ? AND user_id = ?", fed.ID, userID).First(&ban).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", time.Time{}, ErrFederationNotFound
		}
		log.Errorf("[Database] GetBanReason: %v", err)
		return "", time.Time{}, err
	}
	return ban.Reason, ban.BannedAt, nil
}

// ListBans returns all bans in a federation.
func ListBans(fedID string) ([]models.FederationBan, error) {
	fed, err := GetFederationByID(fedID)
	if err != nil {
		return nil, err
	}

	var bans []models.FederationBan
	if err := db.DB.Where("federation_id = ?", fed.ID).Find(&bans).Error; err != nil {
		log.Errorf("[Database] ListBans: %v", err)
		return nil, err
	}
	return bans, nil
}

// CountBans returns the number of banned users in a federation.
func CountBans(fedID string) (int64, error) {
	fed, err := GetFederationByID(fedID)
	if err != nil {
		return 0, err
	}

	var count int64
	if err := db.DB.Model(&models.FederationBan{}).Where("federation_id = ?", fed.ID).Count(&count).Error; err != nil {
		log.Errorf("[Database] CountBans: %v", err)
		return 0, err
	}
	return count, nil
}

// GetUserFederationBans returns all federation bans for a user.
func GetUserFederationBans(userID int64) ([]models.FederationBanInfo, error) {
	var bans []models.FederationBan
	if err := db.DB.Where("user_id = ?", userID).Find(&bans).Error; err != nil {
		log.Errorf("[Database] GetUserFederationBans: %v", err)
		return nil, err
	}

	result := make([]models.FederationBanInfo, 0, len(bans))
	for _, ban := range bans {
		fed, err := GetFederationByIDFromDB(ban.FederationID)
		if err != nil {
			continue
		}
		result = append(result, convertFederationBanModel(ban, *fed))
	}
	return result, nil
}

// SubscribeFederation subscribes one federation to another's ban list.
func SubscribeFederation(fedID string, subscribeToFedID string) error {
	if fedID == subscribeToFedID {
		return ErrSelfSubscription
	}

	fed, err := GetFederationByID(fedID)
	if err != nil {
		return err
	}

	subFed, err := GetFederationByID(subscribeToFedID)
	if err != nil {
		return err
	}

	// Enforce maximum subscription limit.
	var count int64
	if err := db.DB.Model(&models.FederationSubscription{}).
		Where("federation_id = ?", fed.ID).
		Count(&count).Error; err != nil {
		log.Errorf("[Database] SubscribeFederation count: %v", err)
		return err
	}
	if count >= maxSubscriptions {
		return ErrMaxSubscriptionsReached
	}

	record := models.FederationSubscription{
		FederationID:             fed.ID,
		SubscribedToFederationID: subFed.ID,
	}

	if err := db.DB.Create(&record).Error; err != nil {
		if isUniqueViolation(err) {
			return nil
		}
		log.Errorf("[Database] SubscribeFederation: %v", err)
		return err
	}

	invalidateFederationCache(fedID)
	return nil
}

// UnsubscribeFederation removes a federation subscription.
func UnsubscribeFederation(fedID string, subscribedToFedID string) error {
	fed, err := GetFederationByID(fedID)
	if err != nil {
		return err
	}

	subFed, err := GetFederationByID(subscribedToFedID)
	if err != nil {
		return err
	}

	result := db.DB.Where("federation_id = ? AND subscribed_to_federation_id = ?", fed.ID, subFed.ID).Delete(&models.FederationSubscription{})
	if result.Error != nil {
		log.Errorf("[Database] UnsubscribeFederation: %v", result.Error)
		return result.Error
	}

	invalidateFederationCache(fedID)
	return nil
}

// ListSubscriptions returns all federations subscribed to by a federation.
func ListSubscriptions(fedID string) ([]models.FederationSubscription, error) {
	fed, err := GetFederationByID(fedID)
	if err != nil {
		return nil, err
	}

	var subs []models.FederationSubscription
	if err := db.DB.Where("federation_id = ?", fed.ID).Find(&subs).Error; err != nil {
		log.Errorf("[Database] ListSubscriptions: %v", err)
		return nil, err
	}
	return subs, nil
}

// ListFederationsSubscribedTo returns all federations that have subscribed TO the given federation.
// This is the reverse of ListSubscriptions — used for ban propagation.
func ListFederationsSubscribedTo(fedID string) ([]models.Federation, error) {
	fed, err := GetFederationByID(fedID)
	if err != nil {
		return nil, err
	}

	var subIDs []uint
	if err := db.DB.Model(&models.FederationSubscription{}).
		Where("subscribed_to_federation_id = ?", fed.ID).
		Pluck("federation_id", &subIDs).Error; err != nil {
		log.Errorf("[Database] ListFederationsSubscribedTo: %v", err)
		return nil, err
	}

	if len(subIDs) == 0 {
		return nil, nil
	}

	var feds []models.Federation
	if err := db.DB.Where("id IN ?", subIDs).Find(&feds).Error; err != nil {
		log.Errorf("[Database] ListFederationsSubscribedTo feds: %v", err)
		return nil, err
	}
	return feds, nil
}

// IsBannedInFederationOrSubs checks if a user is banned in a federation or any subscribed federation.
func IsBannedInFederationOrSubs(fedID string, userID int64) (bool, *models.FederationBanInfo, error) {
	fed, err := GetFederationByID(fedID)
	if err != nil {
		return false, nil, err
	}

	// Check primary federation.
	var ban models.FederationBan
	err = db.DB.Where("federation_id = ? AND user_id = ?", fed.ID, userID).First(&ban).Error
	if err == nil {
		info := convertFederationBanModel(ban, *fed)
		return true, &info, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Errorf("[Database] IsBannedInFederationOrSubs primary: %v", err)
		return false, nil, err
	}

	// Check subscriptions.
	subs, err := ListSubscriptions(fedID)
	if err != nil {
		return false, nil, err
	}

	for _, sub := range subs {
		subFed, err := GetFederationByIDFromDB(sub.SubscribedToFederationID)
		if err != nil {
			continue
		}
		err = db.DB.Where("federation_id = ? AND user_id = ?", subFed.ID, userID).First(&ban).Error
		if err == nil {
			info := convertFederationBanModel(ban, *subFed)
			return true, &info, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Errorf("[Database] IsBannedInFederationOrSubs sub: %v", err)
		}
	}

	return false, nil, nil
}

// upsertFederationSetting performs an atomic upsert on federation_settings.
func upsertFederationSetting(fedID string, record *models.FederationSettings, updateColumns []string) error {
	fed, err := GetFederationByID(fedID)
	if err != nil {
		return err
	}

	record.FederationID = fed.ID
	err = db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "federation_id"}},
		DoUpdates: clause.AssignmentColumns(updateColumns),
	}).Create(record).Error
	if err != nil {
		return err
	}

	invalidateFederationCache(fedID)
	return nil
}

// SetRequireReason toggles the require_reason setting.
func SetRequireReason(fedID string, require bool) error {
	record := &models.FederationSettings{RequireReason: require}
	if err := upsertFederationSetting(fedID, record, []string{"require_reason"}); err != nil {
		log.Errorf("[Database] SetRequireReason: %v", err)
		return err
	}
	return nil
}

// SetNotifications toggles federation notifications.
func SetNotifications(fedID string, enabled bool) error {
	record := &models.FederationSettings{NotificationsEnabled: enabled}
	if err := upsertFederationSetting(fedID, record, []string{"notifications_enabled"}); err != nil {
		log.Errorf("[Database] SetNotifications: %v", err)
		return err
	}
	return nil
}

// SetLogChat sets the log chat for a federation.
func SetLogChat(fedID string, chatID int64, chatName string) error {
	// Ensure the chat exists in the chats table for the FK constraint.
	if err := chats.EnsureChatInDb(chatID, chatName); err != nil {
		log.Errorf("[Database] SetLogChat: failed to ensure chat %d in db: %v", chatID, err)
		return err
	}

	record := &models.FederationSettings{LogChatID: chatID}
	if err := upsertFederationSetting(fedID, record, []string{"log_chat_id"}); err != nil {
		log.Errorf("[Database] SetLogChat: %v", err)
		return err
	}
	return nil
}

// ClearLogChat removes the log chat for a federation.
func ClearLogChat(fedID string) error {
	fed, err := GetFederationByID(fedID)
	if err != nil {
		return err
	}

	if err := db.DB.Model(&models.FederationSettings{}).
		Where("federation_id = ?", fed.ID).
		Update("log_chat_id", nil).Error; err != nil {
		log.Errorf("[Database] ClearLogChat: %v", err)
		return err
	}

	invalidateFederationCache(fedID)
	return nil
}

// GetSettings returns the settings for a federation.
func GetSettings(fedID string) (*models.FederationSettings, error) {
	fed, err := GetFederationByID(fedID)
	if err != nil {
		return nil, err
	}

	var settings models.FederationSettings
	err = db.DB.Where("federation_id = ?", fed.ID).First(&settings).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Return defaults.
			return &models.FederationSettings{FederationID: fed.ID}, nil
		}
		log.Errorf("[Database] GetSettings: %v", err)
		return nil, err
	}
	return &settings, nil
}

// FederationBanInfo alias for cross-package use.
type FederationBanInfo = models.FederationBanInfo

// convertFederationBanModel converts a FederationBan to a FederationBanInfo.
func convertFederationBanModel(ban models.FederationBan, fed models.Federation) models.FederationBanInfo {
	return models.FederationBanInfo{
		FedID:    fed.FedID,
		FedName:  fed.Name,
		UserID:   ban.UserID,
		Reason:   ban.Reason,
		BannedAt: ban.BannedAt,
	}
}
