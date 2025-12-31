package db

import (
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// getNotesSettings retrieves or creates default notes settings for a chat.
// Used internally before performing any notes-related operation.
// Returns default settings if the chat doesn't exist in the database.
func getNotesSettings(chatID int64) (noteSrc *NotesSettings) {
	noteSrc = &NotesSettings{}
	err := GetRecord(noteSrc, NotesSettings{ChatId: chatID})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Ensure chat exists before creating notes settings
		if !ChatExists(chatID) {
			// Chat doesn't exist, return default settings without creating record
			log.Warnf("[Database][getNotesSettings]: Chat %d doesn't exist, returning default settings", chatID)
			return &NotesSettings{ChatId: chatID, Private: false}
		}

		// Create default settings only if chat exists
		noteSrc = &NotesSettings{ChatId: chatID, Private: false}
		err := CreateRecord(noteSrc)
		if err != nil {
			log.Errorf("[Database][getNotesSettings]: %d - %v", chatID, err)
		}
	} else if err != nil {
		// Return default on error
		noteSrc = &NotesSettings{ChatId: chatID, Private: false}
		log.Errorf("[Database] getNotesSettings: %v - %d", err, chatID)
	}
	return
}

// getAllChatNotes retrieves all notes for a specific chat ID from the database.
// Returns an empty slice if no notes are found or an error occurs.
func getAllChatNotes(chatId int64) (notes []*Notes) {
	err := GetRecords(&notes, Notes{ChatId: chatId})
	if err != nil {
		log.Errorf("[Database] getAllChatNotes: %v - %d", err, chatId)
		return []*Notes{}
	}
	return
}

// GetNotes returns the notes settings for the specified chat ID.
// This is the public interface to access notes settings.
func GetNotes(chatID int64) *NotesSettings {
	return getNotesSettings(chatID)
}

// GetNote retrieves a specific note by chat ID and note name from the database.
// Returns nil if the note is not found or an error occurs.
func GetNote(chatID int64, keyword string) (noteSrc *Notes) {
	noteSrc = &Notes{}
	err := GetRecord(noteSrc, Notes{ChatId: chatID, NoteName: keyword})
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
	var note Notes
	err := DB.Where("chat_id = ? AND note_name = ?", chatID, noteName).Take(&note).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
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
func AddNote(chatID int64, noteName, replyText, fileID string, buttons ButtonArray, filtType int, pvtOnly, grpOnly, adminOnly, webPrev, isProtected, noNotif bool) error {
	// Check if note already exists using optimized query
	var existingNote Notes
	err := DB.Where("chat_id = ? AND note_name = ?", chatID, noteName).Take(&existingNote).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			log.Errorf("[Database][AddNote] checking existence: %d - %v", chatID, err)
			return err
		}
		// Note doesn't exist, continue with creation
	} else {
		return nil // Note already exists
	}

	noterc := Notes{
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

	err = CreateRecord(&noterc)
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
	result := DB.Where("chat_id = ? AND note_name = ?", chatID, noteName).Delete(&Notes{})
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
	err := DB.Where("chat_id = ?", chatID).Delete(&Notes{}).Error
	if err != nil {
		log.Errorf("[Database][RemoveAllNotes]: %d - %v", chatID, err)
		return err
	}
	return nil
}

// TooglePrivateNote toggles the private notes setting for the specified chat.
// When enabled, notes are sent privately to users instead of in the group.
// Returns an error if the operation fails.
func TooglePrivateNote(chatID int64, pref bool) error {
	err := UpdateRecordWithZeroValues(&NotesSettings{}, NotesSettings{ChatId: chatID}, NotesSettings{Private: pref})
	if err != nil {
		log.Errorf("[Database][TooglePrivateNote]: %d - %v", chatID, err)
		return err
	}
	return nil
}

// LoadNotesStats returns statistics about notes across the entire system.
// Returns the total number of notes and the number of distinct chats using notes.
func LoadNotesStats() (notesNum, notesUsingChats int64) {
	// Count total notes
	err := DB.Model(&Notes{}).Count(&notesNum).Error
	if err != nil {
		log.Errorf("[Database] LoadNotesStats (notes): %v", err)
		return 0, 0
	}

	// Count distinct chats with notes
	err = DB.Model(&Notes{}).Distinct("chat_id").Count(&notesUsingChats).Error
	if err != nil {
		log.Errorf("[Database] LoadNotesStats (chats): %v", err)
		return notesNum, 0
	}

	return
}
