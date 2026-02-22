package db

import (
	"slices"
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
	if chatsBefore < 0 {
		t.Fatalf("LoadNotesStats() chatsUsingNotes should be >= 0, got %d", chatsBefore)
	}

	if err := AddNote(chatID, "stats-note", "content", "", ButtonArray{}, TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}
	if !DoesNoteExists(chatID, "stats-note") {
		t.Fatalf("expected DoesNoteExists=true after AddNote")
	}

	var localNotes int64
	if err := DB.Model(&Notes{}).Where("chat_id = ?", chatID).Count(&localNotes).Error; err != nil {
		t.Fatalf("count local notes error = %v", err)
	}
	if localNotes != 1 {
		t.Fatalf("expected exactly 1 local note after AddNote, got %d", localNotes)
	}

	notesAfter, chatsAfter := LoadNotesStats()
	// Global note stats are shared across many t.Parallel() DB tests, so other
	// tests may add/remove notes between snapshots. Assert lower bounds instead
	// of monotonic deltas.
	if notesAfter < localNotes {
		t.Fatalf("expected notesNum to include local notes, local=%d global=%d", localNotes, notesAfter)
	}
	if chatsAfter < 1 {
		t.Fatalf("expected chatsUsingNotes to be >= 1 after AddNote, got %d", chatsAfter)
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

func TestAddNoteWithAdminOnly(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test-admin-only-note"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&Notes{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&NotesSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	// Add a note with adminOnly=true
	if err := AddNote(chatID, "admin-note", "secret content", "", ButtonArray{}, TEXT, false, false, true, false, false, false); err != nil {
		t.Fatalf("AddNote() adminOnly error = %v", err)
	}

	// Admin view (admin=true) should include admin-only notes
	adminList := GetNotesList(chatID, true)
	if !slices.Contains(adminList, "admin-note") {
		t.Fatalf("GetNotesList(chatID, true) = %v, expected to contain 'admin-note'", adminList)
	}

	// Non-admin view (admin=false) should exclude admin-only notes
	nonAdminList := GetNotesList(chatID, false)
	if slices.Contains(nonAdminList, "admin-note") {
		t.Fatalf("GetNotesList(chatID, false) = %v, must not contain 'admin-note' (admin-only)", nonAdminList)
	}
}

func TestSaveNoteTwice_Overwrites(t *testing.T) {
	t.Parallel()
	skipIfNoDb(t)

	chatID := time.Now().UnixNano()
	if err := EnsureChatInDb(chatID, "test-save-twice"); err != nil {
		t.Fatalf("EnsureChatInDb() error = %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Where("chat_id = ?", chatID).Delete(&Notes{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&NotesSettings{}).Error
		_ = DB.Where("chat_id = ?", chatID).Delete(&Chat{}).Error
	})

	// Save note v1
	if err := AddNote(chatID, "my-note", "v1 content", "", ButtonArray{}, TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote() v1 error = %v", err)
	}

	// Save note v2 (same name) -- AddNote is a no-op for duplicates (returns nil, keeps v1)
	if err := AddNote(chatID, "my-note", "v2 content", "", ButtonArray{}, TEXT, false, false, false, false, false, false); err != nil {
		t.Fatalf("AddNote() v2 error = %v", err)
	}

	// GetNote should return the existing note (v1 content, since AddNote is idempotent)
	note := GetNote(chatID, "my-note")
	if note == nil {
		t.Fatal("GetNote() returned nil")
	}
	// AddNote does not overwrite existing notes - v1 content is preserved
	if note.NoteContent != "v1 content" {
		t.Fatalf("NoteContent = %q, want %q (AddNote is idempotent, does not overwrite)", note.NoteContent, "v1 content")
	}

	// Verify no duplicate entries
	list := GetNotesList(chatID, false)
	count := 0
	for _, n := range list {
		if n == "my-note" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 entry for 'my-note', got %d; list: %v", count, list)
	}
}
