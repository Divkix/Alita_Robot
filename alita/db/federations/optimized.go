package federations

import (
	"time"

	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// GetFederationByIDCached retrieves a federation by fed_id with caching.
func GetFederationByIDCached(fedID string) (*models.Federation, error) {
	cacheKey := cache.CacheKey("federation", "fed_id", fedID)
	return cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLFederation, func() (*models.Federation, error) {
		return GetFederationByID(fedID)
	})
}

// GetFederationByOwnerCached retrieves a federation by owner with caching.
func GetFederationByOwnerCached(ownerID int64) (*models.Federation, error) {
	cacheKey := cache.CacheKey("federation", "owner", ownerID)
	return cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLFederation, func() (*models.Federation, error) {
		return GetFederationByOwner(ownerID)
	})
}

// GetChatFederationCached retrieves the federation a chat is joined to with caching.
func GetChatFederationCached(chatID int64) (*models.Federation, error) {
	cacheKey := cache.CacheKey("federation", "chat", chatID)
	return cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLFederation, func() (*models.Federation, error) {
		return GetChatFederation(chatID)
	})
}

// IsAdminCached reports whether a user is a federation admin or owner with caching.
func IsAdminCached(fedID string, userID int64) (bool, error) {
	cacheKey := cache.CacheKey("federation", "is_admin", fedID, userID)
	return cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLFederation, func() (bool, error) {
		return IsAdmin(fedID, userID)
	})
}

// ListAdminsCached returns cached federation admins.
func ListAdminsCached(fedID string) ([]models.FederationAdmin, error) {
	cacheKey := cache.CacheKey("federation", "admins", fedID)
	return cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLFederation, func() ([]models.FederationAdmin, error) {
		return ListAdmins(fedID)
	})
}

// ListFederationChatsCached returns cached chat IDs for a federation.
func ListFederationChatsCached(fedID string) ([]int64, error) {
	cacheKey := cache.CacheKey("federation", "chats", fedID)
	return cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLFederation, func() ([]int64, error) {
		return ListFederationChats(fedID)
	})
}

// IsQuietCached reports whether a chat has quiet mode enabled with caching.
func IsQuietCached(chatID int64) (bool, error) {
	cacheKey := cache.CacheKey("federation", "quiet", chatID)
	return cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLFederation, func() (bool, error) {
		return IsQuiet(chatID)
	})
}

// BanInfo represents a single ban entry with federation metadata.
type BanInfo struct {
	UserID   int64
	Reason   string
	BannedAt time.Time
}

// GetSettingsCached returns cached federation settings.
func GetSettingsCached(fedID string) (*models.FederationSettings, error) {
	cacheKey := cache.CacheKey("federation", "settings", fedID)
	return cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLFederation, func() (*models.FederationSettings, error) {
		return GetSettings(fedID)
	})
}

// ListSubscriptionsCached returns cached subscriptions for a federation.
func ListSubscriptionsCached(fedID string) ([]models.FederationSubscription, error) {
	cacheKey := cache.CacheKey("federation", "subs", fedID)
	return cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLFederation, func() ([]models.FederationSubscription, error) {
		return ListSubscriptions(fedID)
	})
}
