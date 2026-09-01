package federations

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/user"
)

const (
	maxFedNameLen       = 64
	maxFedSubs          = 5
	cachePrefixFed      = "fed"
	cachePrefixFedChat  = "fed_chat"
	cachePrefixFedAdmin = "fed_admins"
	cachePrefixFedBan   = "fed_ban"
	cachePrefixFedSubs  = "fed_subs"
	missingBanUserID    = -9999
)

var (
	ErrAlreadyOwnsFed = errors.New("user already owns a federation")
	ErrAlreadyJoined  = errors.New("chat already joined to a federation")
	ErrAlreadyAdmin   = errors.New("user is already a federation admin")
	ErrOwnerIsAdmin   = errors.New("owner is already a federation admin")
	ErrSubSelf        = errors.New("cannot subscribe a federation to itself")
	ErrAlreadySub     = errors.New("already subscribed")
	ErrSubLimit       = errors.New("subscription limit reached")
)

// MaxSubscriptions is the Rose-compatible per-federation subscription cap.
const MaxSubscriptions = maxFedSubs

func invalidateFed(fedID string, extra ...string) {
	keys := []string{
		cache.CacheKey(cachePrefixFed, fedID),
		cache.CacheKey(cachePrefixFedAdmin, fedID),
		cache.CacheKey(cachePrefixFedSubs, fedID),
	}
	keys = append(keys, extra...)
	for _, key := range keys {
		cache.DeleteCache(key)
	}
}

func invalidateChat(chatID int64) {
	cache.DeleteCache(cache.CacheKey(cachePrefixFedChat, chatID))
}

func invalidateBan(fedID string, userID int64) {
	cache.DeleteCache(cache.CacheKey(cachePrefixFedBan, fedID, userID))
}

func trimFedName(name string) string {
	name = strings.TrimSpace(name)
	if len(name) > maxFedNameLen {
		name = name[:maxFedNameLen]
	}
	return name
}

// CreateFederation creates a federation owned by userID. One federation per
// owner. The name is trimmed to 64 characters.
func CreateFederation(ownerID int64, name string) (*models.Federation, error) {
	name = trimFedName(name)
	if name == "" {
		return nil, fmt.Errorf("federation name is required")
	}
	if err := user.EnsureUserInDb(ownerID, "", ""); err != nil {
		return nil, err
	}
	if existing := GetFedByOwner(ownerID); existing != nil {
		return nil, ErrAlreadyOwnsFed
	}

	fed := &models.Federation{
		FedID:       uuid.NewString(),
		OwnerID:     ownerID,
		Name:        name,
		NotifyOwner: true,
	}
	if err := db.CreateRecord(fed); err != nil {
		log.Errorf("[Federations] CreateFederation: %v", err)
		return nil, err
	}
	invalidateFed(fed.FedID)
	return fed, nil
}

// RenameFederation updates the owner's federation name.
func RenameFederation(ownerID int64, name string) (*models.Federation, error) {
	name = trimFedName(name)
	if name == "" {
		return nil, fmt.Errorf("federation name is required")
	}
	fed := GetFedByOwner(ownerID)
	if fed == nil {
		return nil, gorm.ErrRecordNotFound
	}
	err := db.UpdateRecordWithZeroValues(
		&models.Federation{},
		models.Federation{FedID: fed.FedID},
		map[string]any{"name": name},
	)
	if err != nil {
		log.Errorf("[Federations] RenameFederation: %v", err)
		return nil, err
	}
	invalidateFed(fed.FedID)
	fed.Name = name
	return fed, nil
}

// DeleteFederation removes a federation and all related rows.
func DeleteFederation(fedID string) error {
	fedID = strings.TrimSpace(fedID)
	if fedID == "" {
		return fmt.Errorf("federation id is required")
	}
	var chatIDs []int64
	var bans []models.FederationBan
	var subscribers []models.FederationSub
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		// Lock the federation row so concurrent JoinFed/Fban/SubscribeFed cannot
		// commit children that the later DELETE would remove without invalidation.
		var fed models.Federation
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("fed_id = ?", fedID).
			Take(&fed).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Model(&models.FederationChat{}).
			Where("fed_id = ?", fedID).
			Pluck("chat_id", &chatIDs).Error; err != nil {
			return err
		}
		if err := tx.Where("fed_id = ?", fedID).Find(&bans).Error; err != nil {
			return err
		}
		if err := tx.Where("subscribed_fed_id = ?", fedID).Find(&subscribers).Error; err != nil {
			return err
		}
		if err := tx.Where("fed_id = ?", fedID).Delete(&models.FederationBan{}).Error; err != nil {
			return err
		}
		if err := tx.Where("fed_id = ?", fedID).Delete(&models.FederationAdmin{}).Error; err != nil {
			return err
		}
		if err := tx.Where("fed_id = ?", fedID).Delete(&models.FederationChat{}).Error; err != nil {
			return err
		}
		if err := tx.Where("fed_id = ? OR subscribed_fed_id = ?", fedID, fedID).
			Delete(&models.FederationSub{}).Error; err != nil {
			return err
		}
		return tx.Where("fed_id = ?", fedID).Delete(&models.Federation{}).Error
	})
	if err != nil {
		log.Errorf("[Federations] DeleteFederation: %v", err)
		return err
	}
	invalidateFed(fedID)
	for _, chatID := range chatIDs {
		invalidateChat(chatID)
	}
	for _, ban := range bans {
		invalidateBan(fedID, ban.UserID)
	}
	for _, sub := range subscribers {
		cache.DeleteCache(cache.CacheKey(cachePrefixFedSubs, sub.FedID))
	}
	return nil
}

// GetFed returns a federation by FedID.
func GetFed(fedID string) *models.Federation {
	fedID = strings.TrimSpace(fedID)
	if fedID == "" {
		return nil
	}
	result, err := cache.GetFromCacheOrLoad(cache.CacheKey(cachePrefixFed, fedID), cache.CacheTTLFederation, func() (models.Federation, error) {
		var fed models.Federation
		err := db.GetRecord(&fed, models.Federation{FedID: fedID})
		if err != nil {
			return models.Federation{}, err
		}
		return fed, nil
	})
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Errorf("[Federations] GetFed: %v", err)
		}
		return nil
	}
	return &result
}

// GetFedByOwner returns the federation owned by userID, if any.
func GetFedByOwner(ownerID int64) *models.Federation {
	var fed models.Federation
	err := db.GetRecord(&fed, models.Federation{OwnerID: ownerID})
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Errorf("[Federations] GetFedByOwner: %v", err)
		}
		return nil
	}
	return &fed
}

// GetChatFed returns the federation membership for a chat.
func GetChatFed(chatID int64) *models.FederationChat {
	result, err := cache.GetFromCacheOrLoad(cache.CacheKey(cachePrefixFedChat, chatID), cache.CacheTTLFederation, func() (models.FederationChat, error) {
		var row models.FederationChat
		err := db.GetRecord(&row, models.FederationChat{ChatID: chatID})
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.FederationChat{}, nil
		}
		if err != nil {
			return models.FederationChat{}, err
		}
		return row, nil
	})
	if err != nil {
		log.Errorf("[Federations] GetChatFed: %v", err)
		return nil
	}
	if result.ChatID == 0 {
		return nil
	}
	return &result
}

// JoinFed attaches a chat to a federation. A chat can only belong to one fed.
func JoinFed(chatID int64, chatName, fedID string) error {
	fed := GetFed(fedID)
	if fed == nil {
		return gorm.ErrRecordNotFound
	}
	if err := chats.EnsureChatInDb(chatID, chatName); err != nil {
		return err
	}
	existing := GetChatFed(chatID)
	if existing != nil {
		if existing.FedID == fed.FedID {
			return nil
		}
		return ErrAlreadyJoined
	}
	row := &models.FederationChat{FedID: fed.FedID, ChatID: chatID}
	if err := db.CreateRecord(row); err != nil {
		log.Errorf("[Federations] JoinFed: %v", err)
		return err
	}
	invalidateChat(chatID)
	return nil
}

// LeaveFed removes a chat from its federation.
func LeaveFed(chatID int64) error {
	existing := GetChatFed(chatID)
	if existing == nil {
		return gorm.ErrRecordNotFound
	}
	err := db.DB.Where("chat_id = ?", chatID).Delete(&models.FederationChat{}).Error
	if err != nil {
		log.Errorf("[Federations] LeaveFed: %v", err)
		return err
	}
	invalidateChat(chatID)
	return nil
}

// SetQuietFed toggles the per-chat quietfed announcement setting.
func SetQuietFed(chatID int64, quiet bool) error {
	existing := GetChatFed(chatID)
	if existing == nil {
		return gorm.ErrRecordNotFound
	}
	err := db.UpdateRecordWithZeroValues(
		&models.FederationChat{},
		models.FederationChat{ChatID: chatID},
		map[string]any{"quiet": quiet},
	)
	if err != nil {
		log.Errorf("[Federations] SetQuietFed: %v", err)
		return err
	}
	invalidateChat(chatID)
	return nil
}

// ListFedChatIDs returns every chat currently joined to a federation.
func ListFedChatIDs(fedID string) ([]int64, error) {
	var rows []models.FederationChat
	if err := db.GetRecords(&rows, models.FederationChat{FedID: fedID}); err != nil {
		log.Errorf("[Federations] ListFedChatIDs: %v", err)
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ChatID)
	}
	return ids, nil
}

// CountFedChats returns the number of chats joined to a federation.
func CountFedChats(fedID string) int {
	var count int64
	if err := db.DB.Model(&models.FederationChat{}).Where("fed_id = ?", fedID).Count(&count).Error; err != nil {
		log.Errorf("[Federations] CountFedChats: %v", err)
		return 0
	}
	return int(count)
}

// CountFedBans returns the number of bans in a federation.
func CountFedBans(fedID string) int {
	var count int64
	if err := db.DB.Model(&models.FederationBan{}).Where("fed_id = ?", fedID).Count(&count).Error; err != nil {
		log.Errorf("[Federations] CountFedBans: %v", err)
		return 0
	}
	return int(count)
}

func countModel(model any, op string) int64 {
	var count int64
	if err := db.DB.Model(model).Count(&count).Error; err != nil {
		log.Errorf("[Federations] %s: %v", op, err)
		return 0
	}
	return count
}

// LoadFederationStats returns bot-wide federation totals for /stats.
// Counts match the per-fed numbers already used by /fedinfo:
// federations, joined chats, promoted admins, bans, and subscriptions.
func LoadFederationStats() (feds, chats, admins, bans, subs int64) {
	feds = countModel(&models.Federation{}, "LoadFederationStats (feds)")
	chats = countModel(&models.FederationChat{}, "LoadFederationStats (chats)")
	admins = countModel(&models.FederationAdmin{}, "LoadFederationStats (admins)")
	bans = countModel(&models.FederationBan{}, "LoadFederationStats (bans)")
	subs = countModel(&models.FederationSub{}, "LoadFederationStats (subs)")
	return
}

// IsFedOwner reports whether userID owns the federation.
func IsFedOwner(fedID string, userID int64) bool {
	fed := GetFed(fedID)
	return fed != nil && fed.OwnerID == userID
}

// IsFedAdmin reports whether userID is the owner or a promoted admin.
func IsFedAdmin(fedID string, userID int64) bool {
	if IsFedOwner(fedID, userID) {
		return true
	}
	for _, id := range ListFedAdmins(fedID) {
		if id == userID {
			return true
		}
	}
	return false
}

// listCachedColumn loads rows matching filter through the read-through cache
// and returns the values selected by pick.
func listCachedColumn[Row any, ID any](cacheKey, op string, filter Row, pick func(Row) ID) []ID {
	result, err := cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLFederation, func() ([]ID, error) {
		var rows []Row
		if err := db.GetRecords(&rows, filter); err != nil {
			return nil, err
		}
		ids := make([]ID, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, pick(row))
		}
		return ids, nil
	})
	if err != nil {
		log.Errorf("[Federations] %s: %v", op, err)
		return nil
	}
	return result
}

// ListFedAdmins returns promoted admin user IDs (not including the owner).
func ListFedAdmins(fedID string) []int64 {
	return listCachedColumn(
		cache.CacheKey(cachePrefixFedAdmin, fedID),
		"ListFedAdmins",
		models.FederationAdmin{FedID: fedID},
		func(row models.FederationAdmin) int64 { return row.UserID },
	)
}

// PromoteFedAdmin adds a federation admin. Only the owner should call this.
func PromoteFedAdmin(fedID string, userID int64) error {
	if IsFedOwner(fedID, userID) {
		return ErrOwnerIsAdmin
	}
	if IsFedAdmin(fedID, userID) {
		return ErrAlreadyAdmin
	}
	if err := user.EnsureUserInDb(userID, "", ""); err != nil {
		return err
	}
	err := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "fed_id"}, {Name: "user_id"}},
		DoNothing: true,
	}).Create(&models.FederationAdmin{FedID: fedID, UserID: userID}).Error
	if err != nil {
		log.Errorf("[Federations] PromoteFedAdmin: %v", err)
		return err
	}
	cache.DeleteCache(cache.CacheKey(cachePrefixFedAdmin, fedID))
	return nil
}

// DemoteFedAdmin removes a federation admin.
func DemoteFedAdmin(fedID string, userID int64) error {
	result := db.DB.Where("fed_id = ? AND user_id = ?", fedID, userID).Delete(&models.FederationAdmin{})
	if result.Error != nil {
		log.Errorf("[Federations] DemoteFedAdmin: %v", result.Error)
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	cache.DeleteCache(cache.CacheKey(cachePrefixFedAdmin, fedID))
	return nil
}

// ListFedsForAdmin returns federations the user owns or is an admin of.
func ListFedsForAdmin(userID int64) []models.Federation {
	var owned []models.Federation
	if err := db.GetRecords(&owned, models.Federation{OwnerID: userID}); err != nil {
		log.Errorf("[Federations] ListFedsForAdmin owned: %v", err)
	}
	var adminRows []models.FederationAdmin
	if err := db.GetRecords(&adminRows, models.FederationAdmin{UserID: userID}); err != nil {
		log.Errorf("[Federations] ListFedsForAdmin admin: %v", err)
	}
	seen := make(map[string]struct{}, len(owned)+len(adminRows))
	out := make([]models.Federation, 0, len(owned)+len(adminRows))
	for _, fed := range owned {
		seen[fed.FedID] = struct{}{}
		out = append(out, fed)
	}
	for _, row := range adminRows {
		if _, ok := seen[row.FedID]; ok {
			continue
		}
		if fed := GetFed(row.FedID); fed != nil {
			out = append(out, *fed)
		}
	}
	return out
}

func updateFedField(fedID string, fields map[string]any) error {
	if GetFed(fedID) == nil {
		return gorm.ErrRecordNotFound
	}
	err := db.UpdateRecordWithZeroValues(
		&models.Federation{},
		models.Federation{FedID: fedID},
		fields,
	)
	if err != nil {
		log.Errorf("[Federations] updateFedField: %v", err)
		return err
	}
	invalidateFed(fedID)
	return nil
}

// SetRequireReason toggles the fedreason enforcement flag.
func SetRequireReason(fedID string, required bool) error {
	return updateFedField(fedID, map[string]any{"require_reason": required})
}

// SetNotifyOwner toggles owner PM notifications.
func SetNotifyOwner(fedID string, notify bool) error {
	return updateFedField(fedID, map[string]any{"notify_owner": notify})
}

// SetFedLogChat stores the federation log destination. Zero clears it.
func SetFedLogChat(fedID string, logChatID int64) error {
	return updateFedField(fedID, map[string]any{"log_chat_id": logChatID})
}

// Fban upserts a federation ban.
func Fban(fedID string, userID, bannedBy int64, reason string) (*models.FederationBan, bool, error) {
	if GetFed(fedID) == nil {
		return nil, false, gorm.ErrRecordNotFound
	}
	if err := user.EnsureUserInDb(userID, "", ""); err != nil {
		return nil, false, err
	}
	reason = strings.TrimSpace(reason)
	existing := GetFedBan(fedID, userID)
	row := models.FederationBan{
		FedID:    fedID,
		UserID:   userID,
		Reason:   reason,
		BannedBy: bannedBy,
	}
	err := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "fed_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"reason", "banned_by", "updated_at"}),
	}).Create(&row).Error
	if err != nil {
		log.Errorf("[Federations] Fban: %v", err)
		return nil, false, err
	}
	invalidateBan(fedID, userID)
	created := existing == nil
	return &row, created, nil
}

// Unfban removes a federation ban.
func Unfban(fedID string, userID int64) error {
	result := db.DB.Where("fed_id = ? AND user_id = ?", fedID, userID).Delete(&models.FederationBan{})
	if result.Error != nil {
		log.Errorf("[Federations] Unfban: %v", result.Error)
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	invalidateBan(fedID, userID)
	return nil
}

// GetFedBan returns a ban in a specific federation.
func GetFedBan(fedID string, userID int64) *models.FederationBan {
	result, err := cache.GetFromCacheOrLoad(
		cache.CacheKey(cachePrefixFedBan, fedID, userID),
		cache.CacheTTLFederation,
		func() (models.FederationBan, error) {
			var row models.FederationBan
			err := db.GetRecord(&row, models.FederationBan{FedID: fedID, UserID: userID})
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return models.FederationBan{UserID: missingBanUserID}, nil
			}
			if err != nil {
				return models.FederationBan{}, err
			}
			return row, nil
		},
	)
	if err != nil {
		log.Errorf("[Federations] GetFedBan: %v", err)
		return nil
	}
	if result.UserID == missingBanUserID {
		return nil
	}
	return &result
}

// FindBanInFedTree checks the federation and its subscriptions for a ban.
// The returned string is the fed that actually holds the ban.
func FindBanInFedTree(fedID string, userID int64) (*models.FederationBan, string) {
	if ban := GetFedBan(fedID, userID); ban != nil {
		return ban, fedID
	}
	for _, subID := range ListSubscribedFedIDs(fedID) {
		if ban := GetFedBan(subID, userID); ban != nil {
			return ban, subID
		}
	}
	return nil, ""
}

// ListFedBans returns every ban in a federation.
func ListFedBans(fedID string) ([]models.FederationBan, error) {
	var rows []models.FederationBan
	if err := db.GetRecords(&rows, models.FederationBan{FedID: fedID}); err != nil {
		log.Errorf("[Federations] ListFedBans: %v", err)
		return nil, err
	}
	return rows, nil
}

// ListUserFedBans returns every federation ban targeting userID.
func ListUserFedBans(userID int64) ([]models.FederationBan, error) {
	var rows []models.FederationBan
	if err := db.GetRecords(&rows, models.FederationBan{UserID: userID}); err != nil {
		log.Errorf("[Federations] ListUserFedBans: %v", err)
		return nil, err
	}
	return rows, nil
}

// SubscribeFed subscribes subscriberFedID to targetFedID. Cap is 5.
func SubscribeFed(subscriberFedID, targetFedID string) error {
	if subscriberFedID == targetFedID {
		return ErrSubSelf
	}
	if GetFed(subscriberFedID) == nil || GetFed(targetFedID) == nil {
		return gorm.ErrRecordNotFound
	}
	subs := ListSubscribedFedIDs(subscriberFedID)
	for _, id := range subs {
		if id == targetFedID {
			return ErrAlreadySub
		}
	}
	if len(subs) >= maxFedSubs {
		return ErrSubLimit
	}
	err := db.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "fed_id"}, {Name: "subscribed_fed_id"}},
		DoNothing: true,
	}).Create(&models.FederationSub{FedID: subscriberFedID, SubscribedFedID: targetFedID}).Error
	if err != nil {
		log.Errorf("[Federations] SubscribeFed: %v", err)
		return err
	}
	cache.DeleteCache(cache.CacheKey(cachePrefixFedSubs, subscriberFedID))
	return nil
}

// UnsubscribeFed removes a subscription.
func UnsubscribeFed(subscriberFedID, targetFedID string) error {
	result := db.DB.Where("fed_id = ? AND subscribed_fed_id = ?", subscriberFedID, targetFedID).
		Delete(&models.FederationSub{})
	if result.Error != nil {
		log.Errorf("[Federations] UnsubscribeFed: %v", result.Error)
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	cache.DeleteCache(cache.CacheKey(cachePrefixFedSubs, subscriberFedID))
	return nil
}

// ListSubscribedFedIDs returns federations subscriberFedID is subscribed to.
func ListSubscribedFedIDs(subscriberFedID string) []string {
	return listCachedColumn(
		cache.CacheKey(cachePrefixFedSubs, subscriberFedID),
		"ListSubscribedFedIDs",
		models.FederationSub{FedID: subscriberFedID},
		func(row models.FederationSub) string { return row.SubscribedFedID },
	)
}

// ChatsContainingUser returns the subset of chatIDs whose stored members
// include userID. Membership is the chats.users JSONB array.
func ChatsContainingUser(userID int64, chatIDs []int64) []int64 {
	if len(chatIDs) == 0 {
		return nil
	}
	var rows []models.Chat
	if err := db.DB.Model(&models.Chat{}).Select("chat_id, users").Where("chat_id IN ?", chatIDs).Find(&rows).Error; err != nil {
		log.Errorf("[Federations] ChatsContainingUser: %v", err)
		return nil
	}
	out := make([]int64, 0)
	for _, chat := range rows {
		for _, member := range chat.Users {
			if member == userID {
				out = append(out, chat.ChatId)
				break
			}
		}
	}
	return out
}

// ImportBans upserts bans from an external list. Returns how many were written.
func ImportBans(fedID string, bans []models.FederationBan) (int, error) {
	if GetFed(fedID) == nil {
		return 0, gorm.ErrRecordNotFound
	}
	written := 0
	for _, ban := range bans {
		if ban.UserID <= 0 {
			continue
		}
		if _, _, err := Fban(fedID, ban.UserID, ban.BannedBy, ban.Reason); err != nil {
			return written, err
		}
		written++
	}
	return written, nil
}
