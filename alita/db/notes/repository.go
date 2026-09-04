package notes

import (
	"errors"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/cache"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
)

func getNotesSettings(chatID int64) *models.NotesSettings {
	settingsVal, err := cache.GetFromCacheOrLoad(cache.CacheKey("notes_settings", chatID), cache.CacheTTLNotesSettings, func() (models.NotesSettings, error) {
		noteSrc := &models.NotesSettings{}
		err := db.GetRecord(noteSrc, models.NotesSettings{ChatId: chatID})
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if !db.ChatExists(chatID) {
				log.Warnf("[Database][getNotesSettings]: Chat %d doesn't exist, returning default settings", chatID)
				return models.NotesSettings{ChatId: chatID, Private: false}, nil
			}

			noteSrc = &models.NotesSettings{ChatId: chatID, Private: false}
			err := db.CreateRecord(noteSrc)
			if err != nil {
				log.Errorf("[Database][getNotesSettings]: %d - %v", chatID, err)
			}
		} else if err != nil {
			log.Errorf("[Database] getNotesSettings: %v - %d", err, chatID)
			return models.NotesSettings{ChatId: chatID, Private: false}, nil
		}
		return *noteSrc, nil
	})
	if err != nil {
		log.Errorf("[Database][getNotesSettings]: cache load error %d - %v", chatID, err)
		return &models.NotesSettings{ChatId: chatID, Private: false}
	}
	return &settingsVal
}

func getAllChatNotes(chatId int64) (notes []*models.Notes) {
	err := db.GetRecords(&notes, models.Notes{ChatId: chatId})
	if err != nil {
		log.Errorf("[Database] getAllChatNotes: %v - %d", err, chatId)
		return []*models.Notes{}
	}
	return
}

func GetNotes(chatID int64) *models.NotesSettings {
	return getNotesSettings(chatID)
}

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

type cachedNoteInfo struct {
	Name      string
	AdminOnly bool
}

func notesListCacheKey(chatID int64) string {
	return cache.CacheKey("notes_list", chatID)
}

func invalidateNotesCache(chatID int64) {
	cache.DeleteCache(notesListCacheKey(chatID))
}

func GetNotesList(chatID int64, admin bool) []string {
	cacheKey := notesListCacheKey(chatID)
	entries, err := cache.GetFromCacheOrLoad(cacheKey, cache.CacheTTLNotesList, func() ([]cachedNoteInfo, error) {
		notes := getAllChatNotes(chatID)
		infos := make([]cachedNoteInfo, 0, len(notes))
		for _, n := range notes {
			infos = append(infos, cachedNoteInfo{Name: n.NoteName, AdminOnly: n.AdminOnly})
		}
		return infos, nil
	})
	if err != nil {
		noteSrc := getAllChatNotes(chatID)
		var out []string
		for _, note := range noteSrc {
			if admin || !note.AdminOnly {
				out = append(out, note.NoteName)
			}
		}
		return out
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if admin || !e.AdminOnly {
			out = append(out, e.Name)
		}
	}
	return out
}

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

func AddNote(chatID int64, noteName, replyText, fileID string, buttons models.ButtonArray, filtType int, pvtOnly, grpOnly, adminOnly, webPrev, isProtected, noNotif bool) error {
	now := time.Now().UTC()
	noterc := map[string]any{
		"chat_id":      chatID,
		"note_name":    noteName,
		"note_content": replyText,
		"msg_type":     filtType,
		"file_id":      fileID,
		"buttons":      buttons,
		"admin_only":   adminOnly,
		"private_only": pvtOnly,
		"group_only":   grpOnly,
		"web_preview":  webPrev,
		"is_protected": isProtected,
		"no_notif":     noNotif,
		"created_at":   now,
		"updated_at":   now,
	}

	result := db.DB.Model(&models.Notes{}).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_id"}, {Name: "note_name"}},
		DoNothing: true,
	}).Create(noterc)
	if result.Error != nil {
		log.Errorf("[Database][AddNote]: %d - %v", chatID, result.Error)
		return result.Error
	}
	if result.RowsAffected > 0 {
		invalidateNotesCache(chatID)
	}
	return nil
}

func UpdateNote(chatID int64, noteName, replyText, fileID string, buttons models.ButtonArray, filtType int, pvtOnly, grpOnly, adminOnly, webPrev, isProtected, noNotif bool) (bool, error) {
	result := db.DB.Model(&models.Notes{}).
		Where("chat_id = ? AND note_name = ?", chatID, noteName).
		Updates(map[string]any{
			"note_content": replyText,
			"msg_type":     filtType,
			"file_id":      fileID,
			"buttons":      buttons,
			"admin_only":   adminOnly,
			"private_only": pvtOnly,
			"group_only":   grpOnly,
			"web_preview":  webPrev,
			"is_protected": isProtected,
			"no_notif":     noNotif,
			"updated_at":   time.Now().UTC(),
		})
	if result.Error != nil {
		log.Errorf("[Database][UpdateNote]: %d - %v", chatID, result.Error)
		return false, result.Error
	}
	if result.RowsAffected > 0 {
		invalidateNotesCache(chatID)
	}
	return result.RowsAffected > 0, nil
}

func RemoveNote(chatID int64, noteName string) error {
	result := db.DB.Where("chat_id = ? AND note_name = ?", chatID, noteName).Delete(&models.Notes{})
	if result.Error != nil {
		log.Errorf("[Database][RemoveNote]: %d - %v", chatID, result.Error)
		return result.Error
	}
	if result.RowsAffected > 0 {
		invalidateNotesCache(chatID)
	}
	return nil
}

func RemoveAllNotes(chatID int64) error {
	err := db.DB.Where("chat_id = ?", chatID).Delete(&models.Notes{}).Error
	if err != nil {
		log.Errorf("[Database][RemoveAllNotes]: %d - %v", chatID, err)
		return err
	}
	invalidateNotesCache(chatID)
	return nil
}

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

	cache.DeleteCache(cache.CacheKey("notes_settings", chatID))
	return nil
}

func LoadNotesStats() (notesNum, notesUsingChats int64) {
	err := db.DB.Model(&models.Notes{}).Count(&notesNum).Error
	if err != nil {
		log.Errorf("[Database] LoadNotesStats (notes): %v", err)
		return 0, 0
	}

	err = db.DB.Model(&models.Notes{}).Distinct("chat_id").Count(&notesUsingChats).Error
	if err != nil {
		log.Errorf("[Database] LoadNotesStats (chats): %v", err)
		return notesNum, 0
	}

	return
}
