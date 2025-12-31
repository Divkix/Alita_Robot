package db

import (
	"errors"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// OptimizedLockQueries provides optimized queries for lock operations
type OptimizedLockQueries struct {
	db *gorm.DB
}

// NewOptimizedLockQueries creates a new instance of OptimizedLockQueries.
// Returns an instance with nil database if DB is not initialized.
func NewOptimizedLockQueries() *OptimizedLockQueries {
	if DB == nil {
		log.Error("[OptimizedLockQueries] Database not initialized")
		return &OptimizedLockQueries{db: nil}
	}
	return &OptimizedLockQueries{db: DB}
}

// GetLockStatus retrieves only the lock status for a specific lock type.
// Optimized for high-frequency calls (319K+ calls) by selecting only the locked column.
// Returns false by default if no record is found.
func (o *OptimizedLockQueries) GetLockStatus(chatID int64, lockType string) (bool, error) {
	if o.db == nil {
		return false, errors.New("database not initialized")
	}

	var locked bool
	err := o.db.Model(&LockSettings{}).
		Select("locked").
		Where("chat_id = ? AND lock_type = ?", chatID, lockType).
		Scan(&locked).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil // Default to unlocked
	}
	if err != nil {
		log.Errorf("[OptimizedLockQueries] GetLockStatus: %v", err)
		return false, err
	}

	return locked, nil
}

// GetChatLocksOptimized retrieves all locks for a chat with minimal column selection.
// Returns a map of lock types to their boolean status for improved performance.
func (o *OptimizedLockQueries) GetChatLocksOptimized(chatID int64) (map[string]bool, error) {
	if o.db == nil {
		return nil, errors.New("database not initialized")
	}

	type LockResult struct {
		LockType string
		Locked   bool
	}

	var locks []LockResult
	err := o.db.Model(&LockSettings{}).
		Select("lock_type, locked").
		Where("chat_id = ?", chatID).
		Find(&locks).Error
	if err != nil {
		log.Errorf("[OptimizedLockQueries] GetChatLocksOptimized: %v", err)
		return nil, err
	}

	result := make(map[string]bool)
	for _, lock := range locks {
		result[lock.LockType] = lock.Locked
	}

	return result, nil
}

// OptimizedUserQueries provides optimized queries for user operations
type OptimizedUserQueries struct {
	db *gorm.DB
}

// NewOptimizedUserQueries creates a new instance of OptimizedUserQueries.
// Returns an instance with nil database if DB is not initialized.
func NewOptimizedUserQueries() *OptimizedUserQueries {
	if DB == nil {
		log.Error("[OptimizedUserQueries] Database not initialized")
		return &OptimizedUserQueries{db: nil}
	}
	return &OptimizedUserQueries{db: DB}
}

// GetUserBasicInfo retrieves only essential user information with minimal column selection.
// Optimized for high-frequency calls (61K+ calls) by selecting only necessary fields.
func (o *OptimizedUserQueries) GetUserBasicInfo(userID int64) (*User, error) {
	if o.db == nil {
		return nil, errors.New("database not initialized")
	}

	var user User
	err := o.db.Model(&User{}).
		Select("id, user_id, username, name, language").
		Where("user_id = ?", userID).
		First(&user).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Errorf("[OptimizedUserQueries] GetUserBasicInfo: %v", err)
	}

	return &user, err
}

// OptimizedChatQueries provides optimized queries for chat operations
type OptimizedChatQueries struct {
	db *gorm.DB
}

// NewOptimizedChatQueries creates a new instance of OptimizedChatQueries.
// Returns an instance with nil database if DB is not initialized.
func NewOptimizedChatQueries() *OptimizedChatQueries {
	if DB == nil {
		log.Error("[OptimizedChatQueries] Database not initialized")
		return &OptimizedChatQueries{db: nil}
	}
	return &OptimizedChatQueries{db: DB}
}

// GetChatBasicInfo retrieves only essential chat information with minimal column selection.
// Optimized for high-frequency calls (123K+ calls) by selecting only necessary fields.
func (o *OptimizedChatQueries) GetChatBasicInfo(chatID int64) (*Chat, error) {
	if o.db == nil {
		return nil, errors.New("database not initialized")
	}

	var chat Chat
	err := o.db.Model(&Chat{}).
		Select("id, chat_id, chat_name, language, users, is_inactive").
		Where("chat_id = ?", chatID).
		First(&chat).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Errorf("[OptimizedChatQueries] GetChatBasicInfo: %v", err)
	}

	return &chat, err
}

// OptimizedAntifloodQueries provides optimized queries for antiflood operations
type OptimizedAntifloodQueries struct {
	db *gorm.DB
}

// NewOptimizedAntifloodQueries creates a new instance of OptimizedAntifloodQueries.
// Returns an instance with nil database if DB is not initialized.
func NewOptimizedAntifloodQueries() *OptimizedAntifloodQueries {
	if DB == nil {
		log.Error("[OptimizedAntifloodQueries] Database not initialized")
		return &OptimizedAntifloodQueries{db: nil}
	}
	return &OptimizedAntifloodQueries{db: DB}
}

// GetAntifloodSettings retrieves antiflood settings with minimal column selection.
// Optimized for high-frequency calls (58K+ calls) and returns default settings if none exist.
func (o *OptimizedAntifloodQueries) GetAntifloodSettings(chatID int64) (*AntifloodSettings, error) {
	if o.db == nil {
		return &AntifloodSettings{
			ChatId: chatID,
			Limit:  0, // Changed from 5 - disabled by default
			Action: "mute",
		}, errors.New("database not initialized")
	}

	var settings AntifloodSettings
	err := o.db.Model(&AntifloodSettings{}).
		Select("id, chat_id, flood_limit, action, mode, delete_antiflood_message").
		Where("chat_id = ?", chatID).
		First(&settings).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Return default settings - disabled by default
		return &AntifloodSettings{
			ChatId: chatID,
			Limit:  0, // Changed from 5 - disabled by default
			Action: "mute",
		}, nil
	}
	if err != nil {
		log.Errorf("[OptimizedAntifloodQueries] GetAntifloodSettings: %v", err)
		return nil, err
	}

	return &settings, nil
}

// OptimizedFilterQueries provides optimized queries for filter operations
type OptimizedFilterQueries struct {
	db *gorm.DB
}

// NewOptimizedFilterQueries creates a new instance of OptimizedFilterQueries.
// Returns an instance with nil database if DB is not initialized.
func NewOptimizedFilterQueries() *OptimizedFilterQueries {
	if DB == nil {
		log.Error("[OptimizedFilterQueries] Database not initialized")
		return &OptimizedFilterQueries{db: nil}
	}
	return &OptimizedFilterQueries{db: DB}
}

// GetChatFiltersOptimized retrieves filters with minimal column selection.
// Optimized for high-frequency calls (34K+ calls) by selecting only essential filter fields.
// Includes all fields needed by filtersWatcher: keyword, filter_reply, msgtype, fileid, filter_buttons, nonotif.
func (o *OptimizedFilterQueries) GetChatFiltersOptimized(chatID int64) ([]*ChatFilters, error) {
	if o.db == nil {
		return nil, errors.New("database not initialized")
	}

	var filters []*ChatFilters
	err := o.db.Model(&ChatFilters{}).
		Select("id, chat_id, keyword, filter_reply, msgtype, fileid, filter_buttons, nonotif").
		Where("chat_id = ?", chatID).
		Find(&filters).Error
	if err != nil {
		log.Errorf("[OptimizedFilterQueries] GetChatFiltersOptimized: %v", err)
		return nil, err
	}

	return filters, nil
}

// OptimizedBlacklistQueries provides optimized queries for blacklist operations
type OptimizedBlacklistQueries struct {
	db *gorm.DB
}

// NewOptimizedBlacklistQueries creates a new instance of OptimizedBlacklistQueries.
// Returns an instance with nil database if DB is not initialized.
func NewOptimizedBlacklistQueries() *OptimizedBlacklistQueries {
	if DB == nil {
		log.Error("[OptimizedBlacklistQueries] Database not initialized")
		return &OptimizedBlacklistQueries{db: nil}
	}
	return &OptimizedBlacklistQueries{db: DB}
}

// OptimizedChannelQueries provides optimized queries for channel operations
type OptimizedChannelQueries struct {
	db *gorm.DB
}

// NewOptimizedChannelQueries creates a new instance of OptimizedChannelQueries.
// Returns an instance with nil database if DB is not initialized.
func NewOptimizedChannelQueries() *OptimizedChannelQueries {
	if DB == nil {
		log.Error("[OptimizedChannelQueries] Database not initialized")
		return &OptimizedChannelQueries{db: nil}
	}
	return &OptimizedChannelQueries{db: DB}
}

// GetChannelSettings retrieves channel settings with all relevant columns.
// Returns channel settings for the specified chat or nil if not found.
func (o *OptimizedChannelQueries) GetChannelSettings(chatID int64) (*ChannelSettings, error) {
	if o.db == nil {
		return nil, errors.New("database not initialized")
	}

	var settings ChannelSettings
	err := o.db.Model(&ChannelSettings{}).
		Select("id, chat_id, channel_id, channel_name, username").
		Where("chat_id = ?", chatID).
		First(&settings).Error

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		log.Errorf("[OptimizedChannelQueries] GetChannelSettings: %v", err)
	}

	return &settings, err
}

// CachedOptimizedQueries provides caching layer for optimized queries
type CachedOptimizedQueries struct {
	lockQueries      *OptimizedLockQueries
	userQueries      *OptimizedUserQueries
	chatQueries      *OptimizedChatQueries
	antifloodQueries *OptimizedAntifloodQueries
	filterQueries    *OptimizedFilterQueries
	blacklistQueries *OptimizedBlacklistQueries
	channelQueries   *OptimizedChannelQueries
}

// NewCachedOptimizedQueries creates a new instance with all optimized query types.
// Initializes all the different query optimizers for various database entities.
func NewCachedOptimizedQueries() *CachedOptimizedQueries {
	return &CachedOptimizedQueries{
		lockQueries:      NewOptimizedLockQueries(),
		userQueries:      NewOptimizedUserQueries(),
		chatQueries:      NewOptimizedChatQueries(),
		antifloodQueries: NewOptimizedAntifloodQueries(),
		filterQueries:    NewOptimizedFilterQueries(),
		blacklistQueries: NewOptimizedBlacklistQueries(),
		channelQueries:   NewOptimizedChannelQueries(),
	}
}

// GetLockStatusCached retrieves lock status with caching layer for improved performance.
// Uses 1-hour cache TTL and falls back to direct query if cache fails.
func (c *CachedOptimizedQueries) GetLockStatusCached(chatID int64, lockType string) (bool, error) {
	if c == nil || c.lockQueries == nil {
		return false, errors.New("lock queries not initialized")
	}

	cacheKey := lockCacheKey(chatID, lockType)

	// Try to get from cache first
	cached, err := getFromCacheOrLoad(cacheKey, 1*time.Hour, func() (bool, error) {
		return c.lockQueries.GetLockStatus(chatID, lockType)
	})
	if err != nil {
		// Fallback to direct query on cache error
		return c.lockQueries.GetLockStatus(chatID, lockType)
	}

	return cached, nil
}

// GetUserBasicInfoCached retrieves user information with caching layer for improved performance.
// Uses 1-hour cache TTL and falls back to direct query if cache fails.
func (c *CachedOptimizedQueries) GetUserBasicInfoCached(userID int64) (*User, error) {
	if c == nil || c.userQueries == nil {
		return nil, errors.New("user queries not initialized")
	}

	cacheKey := userCacheKey(userID)

	cached, err := getFromCacheOrLoad(cacheKey, 1*time.Hour, func() (*User, error) {
		return c.userQueries.GetUserBasicInfo(userID)
	})
	if err != nil {
		return c.userQueries.GetUserBasicInfo(userID)
	}

	return cached, nil
}

// GetChatBasicInfoCached retrieves chat information with caching layer for improved performance.
// Uses 30-minute cache TTL and falls back to direct query if cache fails.
func (c *CachedOptimizedQueries) GetChatBasicInfoCached(chatID int64) (*Chat, error) {
	if c == nil || c.chatQueries == nil {
		return nil, errors.New("chat queries not initialized")
	}

	cacheKey := chatCacheKey(chatID)

	cached, err := getFromCacheOrLoad(cacheKey, 30*time.Minute, func() (*Chat, error) {
		return c.chatQueries.GetChatBasicInfo(chatID)
	})
	if err != nil {
		return c.chatQueries.GetChatBasicInfo(chatID)
	}

	return cached, nil
}

// GetAntifloodSettingsCached retrieves antiflood settings with caching layer for improved performance.
// Uses 1-hour cache TTL and falls back to direct query if cache fails.
func (c *CachedOptimizedQueries) GetAntifloodSettingsCached(chatID int64) (*AntifloodSettings, error) {
	if c == nil || c.antifloodQueries == nil {
		return nil, errors.New("antiflood queries not initialized")
	}

	cacheKey := optimizedAntifloodCacheKey(chatID)

	cached, err := getFromCacheOrLoad(cacheKey, 1*time.Hour, func() (*AntifloodSettings, error) {
		return c.antifloodQueries.GetAntifloodSettings(chatID)
	})
	if err != nil {
		return c.antifloodQueries.GetAntifloodSettings(chatID)
	}

	return cached, nil
}

// GetChatFiltersCached retrieves filters with caching layer for improved performance.
// Uses 15-minute cache TTL and falls back to direct query if cache fails.
func (c *CachedOptimizedQueries) GetChatFiltersCached(chatID int64) ([]*ChatFilters, error) {
	if c == nil || c.filterQueries == nil {
		return nil, errors.New("filter queries not initialized")
	}

	cacheKey := filterListCacheKey(chatID)

	cached, err := getFromCacheOrLoad(cacheKey, 15*time.Minute, func() ([]*ChatFilters, error) {
		return c.filterQueries.GetChatFiltersOptimized(chatID)
	})
	if err != nil {
		return c.filterQueries.GetChatFiltersOptimized(chatID)
	}

	return cached, nil
}

// GetChannelSettingsCached retrieves channel settings with caching layer for improved performance.
// Uses 30-minute cache TTL and falls back to direct query if cache fails.
func (c *CachedOptimizedQueries) GetChannelSettingsCached(chatID int64) (*ChannelSettings, error) {
	if c == nil || c.channelQueries == nil {
		return nil, errors.New("channel queries not initialized")
	}

	cacheKey := channelCacheKey(chatID)

	cached, err := getFromCacheOrLoad(cacheKey, 30*time.Minute, func() (*ChannelSettings, error) {
		return c.channelQueries.GetChannelSettings(chatID)
	})
	if err != nil {
		return c.channelQueries.GetChannelSettings(chatID)
	}

	return cached, nil
}

// lockCacheKey generates a cache key for lock status.
// Returns a formatted string for use in caching lock status by chat ID and lock type.
func lockCacheKey(chatID int64, lockType string) string {
	return fmt.Sprintf("alita:lock:%d:%s", chatID, lockType)
}

// userCacheKey generates a cache key for user information.
// Returns a formatted string for use in caching user data by user ID.
func userCacheKey(userID int64) string {
	return fmt.Sprintf("alita:user:%d", userID)
}

// chatCacheKey generates a cache key for chat information.
// Returns a formatted string for use in caching chat data by chat ID.
func chatCacheKey(chatID int64) string {
	return fmt.Sprintf("alita:chat:%d", chatID)
}

// optimizedAntifloodCacheKey generates a cache key for antiflood settings.
// Returns a formatted string for use in caching antiflood settings by chat ID.
func optimizedAntifloodCacheKey(chatID int64) string {
	return fmt.Sprintf("alita:antiflood:%d", chatID)
}

// channelCacheKey generates a cache key for channel settings.
// Returns a formatted string for use in caching channel settings by chat ID.
func channelCacheKey(chatID int64) string {
	return fmt.Sprintf("alita:channel:%d", chatID)
}

// Global instance for optimized queries with thread-safe reinitialization support.
// Uses RWMutex with double-checked locking instead of sync.Once to handle database
// reconnection scenarios safely without race conditions.
var (
	optimizedQueries   *CachedOptimizedQueries
	optimizedQueriesMu sync.RWMutex
)

// isOptimizedQueriesValid checks if the current instance is valid and connected to the current DB.
// Must be called with at least a read lock held.
func isOptimizedQueriesValid() bool {
	if optimizedQueries == nil {
		return false
	}
	if DB == nil {
		return false
	}
	if optimizedQueries.userQueries == nil || optimizedQueries.userQueries.db == nil {
		return false
	}
	// Check if the DB reference matches the current global DB
	return optimizedQueries.userQueries.db == DB
}

// GetOptimizedQueries returns the singleton instance of CachedOptimizedQueries.
// Uses double-checked locking with RWMutex for thread-safe lazy initialization
// and safe reinitialization on database reconnection.
func GetOptimizedQueries() *CachedOptimizedQueries {
	// Fast path: check with read lock
	optimizedQueriesMu.RLock()
	if isOptimizedQueriesValid() {
		result := optimizedQueries
		optimizedQueriesMu.RUnlock()
		return result
	}
	optimizedQueriesMu.RUnlock()

	// Slow path: acquire write lock for initialization
	optimizedQueriesMu.Lock()
	defer optimizedQueriesMu.Unlock()

	// Double-check after acquiring write lock (another goroutine may have initialized)
	if isOptimizedQueriesValid() {
		return optimizedQueries
	}

	// Initialize or reinitialize
	if DB == nil {
		log.Warn("[GetOptimizedQueries] Database not initialized yet, queries will fail")
		// Return a properly initialized empty instance that will return errors
		optimizedQueries = &CachedOptimizedQueries{
			lockQueries:      &OptimizedLockQueries{db: nil},
			userQueries:      &OptimizedUserQueries{db: nil},
			chatQueries:      &OptimizedChatQueries{db: nil},
			antifloodQueries: &OptimizedAntifloodQueries{db: nil},
			filterQueries:    &OptimizedFilterQueries{db: nil},
			blacklistQueries: &OptimizedBlacklistQueries{db: nil},
			channelQueries:   &OptimizedChannelQueries{db: nil},
		}
		return optimizedQueries
	}

	log.Debug("[GetOptimizedQueries] Initializing optimized queries with valid DB")
	optimizedQueries = NewCachedOptimizedQueries()
	return optimizedQueries
}
