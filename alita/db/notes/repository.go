package notes

import (
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// getNotesSettings retrieves or creates default notes settings for a chat.
// Used internally before performing any notes-related operation.
// Returns default settings if the chat doesn't exist in the database.
// Results are cached with stampede protection for performance.
func getNotesSettings(chatID int64) *models.NotesSettings {
	settings, err := cache.GetFromCacheOrLoad(cache.CacheKey("notes_settings", chatID), cache.CacheTTLNotesSettings, func() (*models.NotesSettings, error) {
		noteSrc := &models.NotesSettings{}
		err := db.GetRecord(noteSrc, models.NotesSettings{ChatId: chatID})
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Ensure chat exists before creating notes settings
			if !db.ChatExists(chatID) {
				// Chat doesn't exist, return default settings without creating record
				log.Warnf("[Database][getNotesSettings]: Chat %d doesn't exist, returning default settings", chatID)
				return &models.NotesSettings{ChatId: chatID, Private: false}, nil
			}

			// Create default settings only if chat exists
			noteSrc = &models.NotesSettings{ChatId: chatID, Private: false}
			err := db.CreateRecord(noteSrc)
			if err != nil {
				log.Errorf("[Database][getNotesSettings]: %d - %v", chatID, err)
			}
		} else if err != nil {
			// Return default on error
			log.Errorf("[Database] getNotesSettings: %v - %d", err, chatID)
			return &models.NotesSettings{ChatId: chatID, Private: false}, nil
		}
		return noteSrc, nil
	})
	if err != nil {
		log.Errorf("[Database][getNotesSettings]: cache load error %d - %v", chatID, err)
		return &models.NotesSettings{ChatId: chatID, Private: false}
	}
	return settings
}

// getAllChatNotes retrieves all notes for a specific chat ID from the database.
// Returns an empty slice if no notes are found or an error occurs.
func getAllChatNotes(chatId int64) (notes []*models.Notes) {
	err := db.GetRecords(&notes, models.Notes{ChatId: chatId})
	if err != nil {
		log.Errorf("[Database] getAllChatNotes: %v - %d", err, chatId)
		return []*models.Notes{}
	}
	return
}

// GetNotes returns the notes settings for the specified chat ID.
// This is the public interface to access notes settings.
func GetNotes(chatID int64) *models.NotesSettings {
	return getNotesSettings(chatID)
}

// GetNote retrieves a specific note by chat ID and note name from the database.
// Returns nil if the note is not found or an error occurs.
func GetNote(chatID int64, keyword string) (noteSrc *models.Notes) {
	noteSrc = &models.Notes{}
	err := db.GetRecord(noteSrc, models.Notes{ChatId: chatID, NoteName: keyword})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	} else if err != nil {
		log.Errorf("[Database] GetNote: %v - %d", err, chatID)
		return nil
	}

	return
}

// GetNotesList retrieves a list of all note names for a specific chat ID.
// The admin parameter determines whether to include admin-only notes.
// Returns an empty slice if no notes are found.
func GetNotesList(chatID int64, admin bool) (allNotes []string) {
	noteSrc := getAllChatNotes(chatID)
	for _, note := range noteSrc {
		if admin {
			// Admin sees all notes
			allNotes = append(allNotes, note.NoteName)
		} else {
			// Non-admin only sees non-admin notes
			if !note.AdminOnly {
				allNotes = append(allNotes, note.NoteName)
			}
		}
	}

	return
}

// DoesNoteExists checks whether a note with the given name exists in the specified chat.
// Returns false if the note doesn't exist or an error occurs.
// Uses LIMIT 1 optimization for better performance than COUNT.
func DoesNoteExists(chatID int64, noteName string) bool {
	var note models.Notes
	err := db.DB.Where("chat_id = ? AND note_name = ?", chatID, noteName).Take(&note).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false
		}
		log.Errorf("[Database] DoesNoteExists: %v - %d", err, chatID)
		return false
	}
	return true
}

// AddNote creates a new note in the database for the specified chat.
// Returns an error if the operation fails.
// Supports various note types including text, media, and custom buttons.
func AddNote(chatID int64, noteName, replyText, fileID string, buttons models.ButtonArray, filtType int, pvtOnly, grpOnly, adminOnly, webPrev, isProtected, noNotif bool) error {
	// Check if note already exists using optimized query
	var existingNote models.Notes
	err := db.DB.Where("chat_id = ? AND note_name = ?", chatID, noteName).Take(&existingNote).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Errorf("[Database][AddNote] checking existence: %d - %v", chatID, err)
			return err
		}
		// Note doesn't exist, continue with creation
	} else {
		return nil // Note already exists
	}

	noterc := models.Notes{
		ChatId:      chatID,
		NoteName:    noteName,
		NoteContent: replyText,
		MsgType:     filtType,
		FileID:      fileID,
		Buttons:     buttons,
		AdminOnly:   adminOnly,
		PrivateOnly: pvtOnly,
		GroupOnly:   grpOnly,
		WebPreview:  webPrev,
		IsProtected: isProtected,
		NoNotif:     noNotif,
	}

	err = db.CreateRecord(&noterc)
	if err != nil {
		log.Errorf("[Database][AddNotes]: %d - %v", chatID, err)
		return err
	}
	return nil
}

// RemoveNote deletes a note with the specified name from the chat.
// Returns an error if the operation fails.
func RemoveNote(chatID int64, noteName string) error {
	// Directly attempt to delete the note without checking existence first
	result := db.DB.Where("chat_id = ? AND note_name = ?", chatID, noteName).Delete(&models.Notes{})
	if result.Error != nil {
		log.Errorf("[Database][RemoveNote]: %d - %v", chatID, result.Error)
		return result.Error
	}
	// result.RowsAffected will be 0 if no note was found, which is fine
	return nil
}

// RemoveAllNotes deletes all notes for the specified chat ID from the database.
// Returns an error if the operation fails.
func RemoveAllNotes(chatID int64) error {
	err := db.DB.Where("chat_id = ?", chatID).Delete(&models.Notes{}).Error
	if err != nil {
		log.Errorf("[Database][RemoveAllNotes]: %d - %v", chatID, err)
		return err
	}
	return nil
}

// ensureNotesSettingsRecord ensures a notes_settings row exists for the chat.
func ensureNotesSettingsRecord(chatID int64) error {
	noteSrc := &models.NotesSettings{}
	err := db.GetRecord(noteSrc, models.NotesSettings{ChatId: chatID})
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err := chats.EnsureChatInDb(chatID, ""); err != nil {
		return err
	}
	return db.CreateRecord(&models.NotesSettings{ChatId: chatID, Private: false})
}

// TooglePrivateNote toggles the private notes setting for the specified chat.
// When enabled, notes are sent privately to users instead of in the group.
// Returns an error if the operation fails.
func TooglePrivateNote(chatID int64, pref bool) error {
	if err := ensureNotesSettingsRecord(chatID); err != nil {
		log.Errorf("[Database][TooglePrivateNote]: ensure settings %d - %v", chatID, err)
		return err
	}
	err := db.UpdateRecordWithZeroValues(
		&models.NotesSettings{},
		models.NotesSettings{ChatId: chatID},
		map[string]any{"private": pref},
	)
	if err != nil {
		log.Errorf("[Database][TooglePrivateNote]: %d - %v", chatID, err)
		return err
	}

	// Invalidate cache after update
	cache.DeleteCache(cache.CacheKey("notes_settings", chatID))
	return nil
}

// LoadNotesStats returns statistics about notes across the entire system.
// Returns the total number of notes and the number of distinct chats using notes.
func LoadNotesStats() (notesNum, notesUsingChats int64) {
	// Count total notes
	err := db.DB.Model(&models.Notes{}).Count(&notesNum).Error
	if err != nil {
		log.Errorf("[Database] LoadNotesStats (notes): %v", err)
		return 0, 0
	}

	// Count distinct chats with notes
	err = db.DB.Model(&models.Notes{}).Distinct("chat_id").Count(&notesUsingChats).Error
	if err != nil {
		log.Errorf("[Database] LoadNotesStats (chats): %v", err)
		return notesNum, 0
	}

	return
}
