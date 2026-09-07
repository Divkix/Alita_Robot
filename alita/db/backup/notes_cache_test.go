package backup

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	gocache "github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
	gocache_store "github.com/eko/gocache/store/redis/v4"
	"github.com/redis/go-redis/v9"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/chats"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/notes"
	utilsCache "github.com/divkix/Alita_Robot/alita/utils/cache"
)

// withMiniredisCache starts an in-process Redis server and installs it as the
// cache marshaler so cache read-through and invalidation paths are exercised
// without a live Redis server.
func withMiniredisCache(t *testing.T) {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	previousMarshal := utilsCache.GetMarshal()
	previousManager := utilsCache.Manager
	manager := gocache.New[any](gocache_store.NewRedis(client))
	utilsCache.Manager = manager
	utilsCache.SetMarshal(marshaler.New(manager))
	t.Cleanup(func() {
		utilsCache.Manager = previousManager
		utilsCache.SetMarshal(previousMarshal)
	})
}

func notesImportPayload(t *testing.T, chatID int64, name string) map[string]interface{} {
	t.Helper()

	raw, err := json.Marshal(NotesBackup{
		Notes: []models.Notes{{ChatId: chatID, NoteName: name, NoteContent: "content"}},
	})
	if err != nil {
		t.Fatalf("marshal notes backup payload: %v", err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal notes backup payload: %v", err)
	}
	return payload
}

func seedCachedNote(t *testing.T, chatID int64, name string) {
	t.Helper()

	if err := chats.EnsureChatInDb(chatID, "notes-cache-test"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	if err := notes.AddNote(chatID, name, "content", "", nil, db.TEXT,
		false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote(%q) error = %v", name, err)
	}
	// Prime notes_list in Redis; every later GetNotesList must observe
	// post-import state instead of this snapshot.
	if primed := notes.GetNotesList(chatID, true); !slices.Contains(primed, name) {
		t.Fatalf("primed GetNotesList() = %v, want it to contain %q", primed, name)
	}
}

func cleanupNotesChat(t *testing.T, chatID int64) {
	t.Helper()

	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Notes{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.NotesSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})
}

func TestImportNotesInvalidatesNotesListCache(t *testing.T) {
	skipIfNoDb(t)
	withMiniredisCache(t)

	chatID := time.Now().UnixNano()
	cleanupNotesChat(t, chatID)
	seedCachedNote(t, chatID, "stale-note")

	if err := ImportModuleData(chatID, BackupModuleNotes, notesImportPayload(t, chatID, "restored-note")); err != nil {
		t.Fatalf("ImportModuleData(notes) error = %v", err)
	}

	if got := notes.GetNotesList(chatID, true); len(got) != 1 || got[0] != "restored-note" {
		t.Fatalf("GetNotesList() after import = %v, want [restored-note]", got)
	}
}

func TestClearNotesInvalidatesNotesListCache(t *testing.T) {
	skipIfNoDb(t)
	withMiniredisCache(t)

	chatID := time.Now().UnixNano() + 1
	cleanupNotesChat(t, chatID)
	seedCachedNote(t, chatID, "doomed-note")

	if err := ClearModuleData(chatID, BackupModuleNotes); err != nil {
		t.Fatalf("ClearModuleData(notes) error = %v", err)
	}

	if got := notes.GetNotesList(chatID, true); len(got) != 0 {
		t.Fatalf("GetNotesList() after clear = %v, want empty", got)
	}
}

func TestImportDisablingPreservesExplicitFalse(t *testing.T) {
	skipIfNoDb(t)

	chatID := time.Now().UnixNano() + 2
	t.Cleanup(func() {
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.DisableSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.DisableChatSettings{}).Error
		_ = db.DB.Where("chat_id = ?", chatID).Delete(&models.Chat{}).Error
	})

	// Backup imports keep every row field, including an explicit
	// disabled=false (this is the row DisableCMD must flip to true).
	payload := map[string]interface{}{
		"commands": []interface{}{
			map[string]interface{}{"command": "ban", "disabled": false},
		},
	}
	if err := ImportModuleData(chatID, BackupModuleDisabling, payload); err != nil {
		t.Fatalf("ImportModuleData(disabling) error = %v", err)
	}

	var row models.DisableSettings
	if err := db.DB.Where("chat_id = ? AND command = ?", chatID, "ban").Take(&row).Error; err != nil {
		t.Fatalf("read back imported DisableSettings error = %v", err)
	}
	if row.Disabled {
		t.Fatal("imported DisableSettings.Disabled = true, want preserved false")
	}
}
