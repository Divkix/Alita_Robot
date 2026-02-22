package db

import (
	"testing"
	"time"
)

func TestGetNotesSettings_Defaults(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test-notes-defaults"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&NotesSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	settings := GetNotes(chatID)
	if settings == nil {
		t.Fatalf("GetNotes() returned nil")
	}
	if settings.Private {
		t.Fatalf("expected default Private=false, got true")
	}
}

func TestSaveNote(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test-save-note"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&Notes{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&NotesSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	buttons := ButtonArray{{Name: "click", Url: "https://example.com", SameLine: false}}
	err := AddNote(chatID, "testnote", "note content", "", buttons, TEXT, false, false, false, false, false, false)
	if err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}

	note := GetNote(chatID, "testnote")
	if note == nil {
		t.Fatalf("GetNote() returned nil after AddNote()")
	}
	if note.NoteName != "testnote" {
		t.Fatalf("expected NoteName='testnote', got %q", note.NoteName)
	}
	if note.NoteContent != "note content" {
		t.Fatalf("expected NoteContent='note content', got %q", note.NoteContent)
	}
	if len(note.Buttons) != 1 {
		t.Fatalf("expected 1 button, got %d", len(note.Buttons))
	}
	if note.Buttons[0].Name != "click" {
		t.Fatalf("expected button name='click', got %q", note.Buttons[0].Name)
	}
	if note.MsgType != TEXT {
		t.Fatalf("expected MsgType=%d, got %d", TEXT, note.MsgType)
	}
}

func TestGetAllNotes(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test-get-all-notes"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&Notes{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&NotesSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	noteNames := []string{"alpha", "beta", "gamma"}
	for _, name := range noteNames {
		if err := AddNote(chatID, name, "content for "+name, "", ButtonArray{}, TEXT, false, false, false, false, false, false); err != nil {
			t.Fatalf("AddNote(%q) error = %v", name, err)
		}
	}

	list := GetNotesList(chatID, false)
	if len(list) < 3 {
		t.Fatalf("expected at least 3 notes, got %d", len(list))
	}

	found := map[string]bool{}
	for _, n := range list {
		found[n] = true
	}
	for _, name := range noteNames {
		if !found[name] {
			t.Fatalf("expected note %q in list, not found", name)
		}
	}
}

func TestRemoveNote(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test-remove-note"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&Notes{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&NotesSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	if err := AddNote(chatID, "to-delete", "will be removed", "", ButtonArray{}, TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}

	note := GetNote(chatID, "to-delete")
	if note == nil {
		t.Fatalf("GetNote() returned nil before removal")
	}

	if err := RemoveNote(chatID, "to-delete"); err != nil {
		t.Fatalf("RemoveNote() error = %v", err)
	}

	note = GetNote(chatID, "to-delete")
	if note != nil {
		t.Fatalf("expected note to be nil after removal, got %+v", note)
	}
}

func TestToggleNotesPrivate(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test-toggle-notes-private"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&NotesSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	// Ensure settings record is created
	_ = GetNotes(chatID)

	if err := TooglePrivateNote(chatID, true); err != nil {
		t.Fatalf("TooglePrivateNote(true) error = %v", err)
	}
	settings := GetNotes(chatID)
	if !settings.Private {
		t.Fatalf("expected Private=true after toggle, got false")
	}

	if err := TooglePrivateNote(chatID, false); err != nil {
		t.Fatalf("TooglePrivateNote(false) error = %v", err)
	}
	settings = GetNotes(chatID)
	if settings.Private {
		t.Fatalf("expected Private=false after toggle back, got true")
	}
}

func TestNoteUpsertBehavior(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test-note-upsert"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&Notes{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&NotesSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	// First add
	if err := AddNote(chatID, "dupnote", "original content", "", ButtonArray{}, TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote() first call error = %v", err)
	}

	// Second add with same name — per AddNote implementation: returns nil (no-op) if already exists
	if err := AddNote(chatID, "dupnote", "updated content", "", ButtonArray{}, TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote() second call error = %v", err)
	}

	list := GetNotesList(chatID, false)
	count := 0
	for _, n := range list {
		if n == "dupnote" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 note with name 'dupnote', got %d", count)
	}
}

func TestGetAllNotes_EmptyChat(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test-empty-notes"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&Notes{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	list := GetNotesList(chatID, false)
	if len(list) != 0 {
		t.Fatalf("expected 0 notes for empty chat, got %d", len(list))
	}
}

func TestRemoveNonExistentNote(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test-remove-nonexistent-note"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&Notes{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	// Removing a non-existent note should not return an error
	if err := RemoveNote(chatID, "ghost-note"); err != nil {
		t.Fatalf("RemoveNote() non-existent note error = %v", err)
	}
}

func TestDoesNoteExists(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test-note-exists"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&Notes{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&NotesSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	if DoesNoteExists(chatID, "new-note") {
		t.Fatalf("expected DoesNoteExists=false before creation")
	}

	if err := AddNote(chatID, "new-note", "hello", "", ButtonArray{}, TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}

	if !DoesNoteExists(chatID, "new-note") {
		t.Fatalf("expected DoesNoteExists=true after creation")
	}
}

func TestLoadNotesStats(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test-load-notes-stats"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&Notes{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&NotesSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	notesBefore, chatsBefore := LoadNotesStats()
	if notesBefore < 0 {
		t.Fatalf("LoadNotesStats() notesNum should be >= 0, got %d", notesBefore)
	}

	if err := AddNote(chatID, "stats-note", "content", "", ButtonArray{}, TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}

	notesAfter, chatsAfter := LoadNotesStats()
	if notesAfter <= notesBefore {
		t.Fatalf("expected notesNum to increase after AddNote, before=%d after=%d", notesBefore, notesAfter)
	}
	if chatsAfter < chatsBefore {
		t.Fatalf("expected chatsUsingNotes to be >= before, before=%d after=%d", chatsBefore, chatsAfter)
	}
}

func TestRemoveAllNotes(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test-remove-all-notes"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&Notes{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&NotesSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	for _, name := range []string{"n1", "n2", "n3"} {
		if err := AddNote(chatID, name, "text", "", ButtonArray{}, TEXT, false, false, false, false, false, false); err != nil {
			t.Fatalf("AddNote(%q) error = %v", name, err)
		}
	}

	if err := RemoveAllNotes(chatID); err != nil {
		t.Fatalf("RemoveAllNotes() error = %v", err)
	}

	list := GetNotesList(chatID, false)
	if len(list) != 0 {
		t.Fatalf("expected 0 notes after RemoveAllNotes, got %d", len(list))
	}
}
