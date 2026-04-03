package db

import (
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/constants"
	"github.com/eko/gocache/lib/v4/store"
)

// UserNote represents a personal note for a user
// These are user-specific notes (not chat notes), accessible across all chats
type UserNote struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"-"`
	UserId    int64     `gorm:"column:user_id;index;not null" json:"user_id,omitempty"`
	NoteName  string    `gorm:"column:note_name;index;not null" json:"note_name,omitempty"`
	Content   string    `gorm:"column:content" json:"content,omitempty"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName returns the database table name for the UserNote model.
func (UserNote) TableName() string {
	return "user_notes"
}

// userNoteCacheKey generates a Redis cache key for a user's note
func userNoteCacheKey(userID int64, noteName string) string {
	return fmt.Sprintf("alita:user_note:%d:%s", userID, noteName)
}

// userNotesListCacheKey generates a Redis cache key for a user's note list
func userNotesListCacheKey(userID int64) string {
	return fmt.Sprintf("alita:user_notes_list:%d", userID)
}

// invalidateUserNoteCache removes a specific note from cache
func invalidateUserNoteCache(userID int64, noteName string) {
	cacheKey := userNoteCacheKey(userID, noteName)
	if err := cache.Marshal.Delete(cache.Context, cacheKey); err != nil {
		log.Debugf("[UserNotes] Failed to invalidate note cache: %v", err)
	}
	// Also invalidate the list cache
	listCacheKey := userNotesListCacheKey(userID)
	if err := cache.Marshal.Delete(cache.Context, listCacheKey); err != nil {
		log.Debugf("[UserNotes] Failed to invalidate list cache: %v", err)
	}
}

// AddUserNote creates a new user note or updates if it exists
// Uses Redis caching for performance with automatic cache invalidation
func AddUserNote(userID int64, noteName, content string) error {
	noteName = cleanNoteName(noteName)
	if noteName == "" {
		return fmt.Errorf("note name cannot be empty")
	}

	// Check if note already exists
	var existingNote UserNote
	err := DB.Where("user_id = ? AND note_name = ?", userID, noteName).First(&existingNote).Error

	if err == nil {
		// Note exists, update it
		existingNote.Content = content
		existingNote.UpdatedAt = time.Now()
		if err := DB.Save(&existingNote).Error; err != nil {
			log.Errorf("[UserNotes] Failed to update note: %v", err)
			return err
		}
	} else if err == gorm.ErrRecordNotFound {
		// Create new note
		newNote := UserNote{
			UserId:   userID,
			NoteName: noteName,
			Content:  content,
		}
		if err := DB.Create(&newNote).Error; err != nil {
			log.Errorf("[UserNotes] Failed to create note: %v", err)
			return err
		}
	} else {
		log.Errorf("[UserNotes] Database error checking note existence: %v", err)
		return err
	}

	// Invalidate cache after write
	invalidateUserNoteCache(userID, noteName)

	return nil
}

// GetUserNote retrieves a specific user note with caching
// Returns nil if note not found
func GetUserNote(userID int64, noteName string) *UserNote {
	noteName = cleanNoteName(noteName)
	if noteName == "" {
		return nil
	}

	cacheKey := userNoteCacheKey(userID, noteName)

	// Try to get from cache first
	cached, err := cache.Marshal.Get(cache.Context, cacheKey, new(UserNote))
	if err == nil && cached != nil {
		log.Debugf("[UserNotes] Cache hit for user %d note %s", userID, noteName)
		return cached.(*UserNote)
	}

	// Cache miss, get from database
	var note UserNote
	err = DB.Where("user_id = ? AND note_name = ?", userID, noteName).First(&note).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		log.Errorf("[UserNotes] Failed to get note: %v", err)
		return nil
	}

	// Store in cache for future requests
	if err := cache.Marshal.Set(cache.Context, cacheKey, note,
		store.WithExpiration(constants.DefaultCacheTTL)); err != nil {
		log.Debugf("[UserNotes] Failed to cache note: %v", err)
	}

	return &note
}

// GetUserNotesList retrieves all note names for a user with caching
// Returns empty slice if no notes found
func GetUserNotesList(userID int64) []string {
	cacheKey := userNotesListCacheKey(userID)

	// Try cache first
	cached, err := cache.Marshal.Get(cache.Context, cacheKey, new([]string))
	if err == nil && cached != nil {
		log.Debugf("[UserNotes] Cache hit for user %d note list", userID)
		return *cached.(*[]string)
	}

	// Cache miss, query database
	var notes []UserNote
	err = DB.Where("user_id = ?", userID).Find(&notes).Error
	if err != nil {
		log.Errorf("[UserNotes] Failed to get note list: %v", err)
		return []string{}
	}

	// Extract note names
	var noteNames []string
	for _, note := range notes {
		noteNames = append(noteNames, note.NoteName)
	}

	// Cache the result
	if err := cache.Marshal.Set(cache.Context, cacheKey, noteNames,
		store.WithExpiration(constants.DefaultCacheTTL)); err != nil {
		log.Debugf("[UserNotes] Failed to cache note list: %v", err)
	}

	return noteNames
}

// DeleteUserNote deletes a user note
// Returns error if note doesn't exist or deletion fails
func DeleteUserNote(userID int64, noteName string) error {
	noteName = cleanNoteName(noteName)
	if noteName == "" {
		return fmt.Errorf("note name cannot be empty")
	}

	result := DB.Where("user_id = ? AND note_name = ?", userID, noteName).Delete(&UserNote{})
	if result.Error != nil {
		log.Errorf("[UserNotes] Failed to delete note: %v", result.Error)
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	// Invalidate cache after deletion
	invalidateUserNoteCache(userID, noteName)

	return nil
}

// GetAllUserNotes retrieves all notes for a user (full content)
// Useful for backup/export purposes
func GetAllUserNotes(userID int64) []*UserNote {
	var notes []*UserNote
	err := DB.Where("user_id = ?", userID).Find(&notes).Error
	if err != nil {
		log.Errorf("[UserNotes] Failed to get all notes: %v", err)
		return []*UserNote{}
	}
	return notes
}

// cleanNoteName sanitizes note name (lowercase, trim spaces)
func cleanNoteName(name string) string {
	// Convert to lowercase and trim spaces
	name = strings.ToLower(strings.TrimSpace(name))
	// Replace multiple spaces with single space
	name = strings.Join(strings.Fields(name), " ")
	return name
}

// LoadUserNotesStats returns statistics about user notes across the system
func LoadUserNotesStats() (totalNotes, usersWithNotes int64) {
	// Count total notes
	err := DB.Model(&UserNote{}).Count(&totalNotes).Error
	if err != nil {
		log.Errorf("[UserNotes] Failed to count total notes: %v", err)
	}

	// Count distinct users with notes
	err = DB.Model(&UserNote{}).Distinct("user_id").Count(&usersWithNotes).Error
	if err != nil {
		log.Errorf("[UserNotes] Failed to count users with notes: %v", err)
	}

	return totalNotes, usersWithNotes
}

// InitUserNotesCacheWarmer starts a background goroutine to warm up the cache
// This is called during startup to pre-populate the cache with frequently accessed data
func InitUserNotesCacheWarmer() {
	// This is a placeholder for future cache warming implementation
	// Currently, we use on-demand caching which is efficient for this use case
	log.Debug("[UserNotes] Cache warmer initialized (on-demand mode)")
}
