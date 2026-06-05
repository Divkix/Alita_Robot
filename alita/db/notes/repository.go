package notes

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

// getNotesSettings retrieves or creates default notes settings for a chat.
func getNotesSettings(chatID int64) *models.NotesSettings {
	settings, err := cache.GetFromCacheOrLoad(cache.CacheKey("notes_settings", chatID), cache.CacheTTLNotesSettings, func() (*models.NotesSettings, error) {
		noteSrc := &models.NotesSettings{}
		err := db.GetRecord(noteSrc, bson.M{"chat_id": chatID})
		if err == db.ErrRecordNotFound {
			// Ensure chat exists before creating notes settings
			if !db.ChatExists(chatID) {
				// Chat doesn't exist, return default settings without creating record
				log.Warnf("[Database][getNotesSettings]: Chat %d doesn't exist, returning default settings", chatID)
				return &models.NotesSettings{ChatId: chatID, Private: false}, nil
			}

			// Create default settings only if chat exists
			noteSrc = &models.NotesSettings{ChatId: chatID, Private: false, CreatedAt: time.Now(), UpdatedAt: time.Now()}
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
func getAllChatNotes(chatId int64) (notes []*models.Notes) {
	err := db.GetRecords(&notes, bson.M{"chat_id": chatId})
	if err != nil {
		log.Errorf("[Database] getAllChatNotes: %v - %d", err, chatId)
		return []*models.Notes{}
	}
	return
}

// GetNotes returns the notes settings for the specified chat ID.
func GetNotes(chatID int64) *models.NotesSettings {
	return getNotesSettings(chatID)
}

// GetNote retrieves a specific note by chat ID and note name from the database.
func GetNote(chatID int64, keyword string) (noteSrc *models.Notes) {
	noteSrc = &models.Notes{}
	err := db.GetRecord(noteSrc, bson.M{"chat_id": chatID, "note_name": keyword})
	if err == db.ErrRecordNotFound {
		return nil
	} else if err != nil {
		log.Errorf("[Database] GetNote: %v - %d", err, chatID)
		return nil
	}

	return
}

// GetNotesList retrieves a list of all note names for a specific chat ID.
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
func DoesNoteExists(chatID int64, noteName string) bool {
	var note models.Notes
	collection := db.DB.Collection("notes")
	err := collection.FindOne(context.Background(), bson.M{"chat_id": chatID, "note_name": noteName}).Decode(&note)
	if err != nil {
		if err == db.ErrRecordNotFound {
			return false
		}
		log.Errorf("[Database] DoesNoteExists: %v - %d", err, chatID)
		return false
	}
	return true
}

// AddNote creates a new note in the database for the specified chat.
func AddNote(chatID int64, noteName, replyText, fileID string, buttons models.ButtonArray, filtType int, pvtOnly, grpOnly, adminOnly, webPrev, isProtected, noNotif bool) error {
	// Check if note already exists
	var existingNote models.Notes
	collection := db.DB.Collection("notes")
	err := collection.FindOne(context.Background(), bson.M{"chat_id": chatID, "note_name": noteName}).Decode(&existingNote)
	if err == nil {
		return nil // Note already exists
	} else if err != db.ErrRecordNotFound {
		log.Errorf("[Database][AddNote] checking existence: %d - %v", chatID, err)
		return err
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
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err = db.CreateRecord(&noterc)
	if err != nil {
		log.Errorf("[Database][AddNotes]: %d - %v", chatID, err)
		return err
	}
	return nil
}

// RemoveNote deletes a note with the specified name from the chat.
func RemoveNote(chatID int64, noteName string) error {
	collection := db.DB.Collection("notes")
	_, err := collection.DeleteOne(context.Background(), bson.M{"chat_id": chatID, "note_name": noteName})
	if err != nil {
		log.Errorf("[Database][RemoveNote]: %d - %v", chatID, err)
		return err
	}
	return nil
}

// RemoveAllNotes deletes all notes for the specified chat ID from the database.
func RemoveAllNotes(chatID int64) error {
	collection := db.DB.Collection("notes")
	_, err := collection.DeleteMany(context.Background(), bson.M{"chat_id": chatID})
	if err != nil {
		log.Errorf("[Database][RemoveAllNotes]: %d - %v", chatID, err)
		return err
	}
	return nil
}

// ensureNotesSettingsRecord ensures a notes_settings row exists for the chat.
func ensureNotesSettingsRecord(chatID int64) error {
	noteSrc := &models.NotesSettings{}
	err := db.GetRecord(noteSrc, bson.M{"chat_id": chatID})
	if err == nil {
		return nil
	}
	if err != db.ErrRecordNotFound {
		return err
	}
	if err := chats.EnsureChatInDb(chatID, ""); err != nil {
		return err
	}
	return db.CreateRecord(&models.NotesSettings{ChatId: chatID, Private: false, CreatedAt: time.Now(), UpdatedAt: time.Now()})
}

// TooglePrivateNote toggles the private notes setting for the specified chat.
func TooglePrivateNote(chatID int64, pref bool) error {
	if err := ensureNotesSettingsRecord(chatID); err != nil {
		log.Errorf("[Database][TooglePrivateNote]: ensure settings %d - %v", chatID, err)
		return err
	}
	err := db.UpdateRecordWithZeroValues(
		&models.NotesSettings{},
		bson.M{"chat_id": chatID},
		map[string]any{"private": pref, "updated_at": time.Now()},
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
func LoadNotesStats() (notesNum, notesUsingChats int64) {
	collection := db.DB.Collection("notes")

	// Count total notes
	var err error
	notesNum, err = collection.CountDocuments(context.Background(), bson.M{})
	if err != nil {
		log.Errorf("[Database] LoadNotesStats (notes): %v", err)
		return 0, 0
	}

	// Count distinct chats with notes
	distinctChats, err := collection.Distinct(context.Background(), "chat_id", bson.M{})
	if err != nil {
		log.Errorf("[Database] LoadNotesStats (chats): %v", err)
		return notesNum, 0
	}
	notesUsingChats = int64(len(distinctChats))

	return
}
