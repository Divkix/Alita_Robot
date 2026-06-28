package modules

import "testing"

func TestParseNoteOverwriteCallbackDataTokenized(t *testing.T) {
	t.Parallel()

	data := encodeCallbackData("notes.overwrite", map[string]string{
		"a": "yes",
		"t": "tok._-42",
	})

	action, token, ok := parseNoteOverwriteCallbackData(data)
	if !ok {
		t.Fatalf("expected tokenized callback data to parse")
	}
	if action != "yes" {
		t.Fatalf("unexpected action: %q", action)
	}
	if token != "tok._-42" {
		t.Fatalf("unexpected token: %q", token)
	}
}

func TestParseNoteOverwriteCallbackDataLegacyRejected(t *testing.T) {
	t.Parallel()

	data := "notes.overwrite.yes.123_note.key_with.parts"
	if _, _, ok := parseNoteOverwriteCallbackData(data); ok {
		t.Fatalf("expected legacy callback data to be rejected")
	}
}

func TestParseFilterOverwriteCallbackDataTokenized(t *testing.T) {
	t.Parallel()

	data := encodeCallbackData("filters_overwrite", map[string]string{
		"a": "yes",
		"t": "f0.token",
	})

	action, token, ok := parseFilterOverwriteCallbackData(data)
	if !ok {
		t.Fatalf("expected tokenized callback data to parse")
	}
	if action != "yes" {
		t.Fatalf("unexpected action: %q", action)
	}
	if token != "f0.token" {
		t.Fatalf("unexpected token: %q", token)
	}
}

func TestParseFilterOverwriteCallbackDataLegacyRejected(t *testing.T) {
	t.Parallel()

	if _, _, ok := parseFilterOverwriteCallbackData("filters_overwrite.foo.bar_baz"); ok {
		t.Fatalf("expected legacy callback data to be rejected")
	}
	if _, _, ok := parseFilterOverwriteCallbackData("filters_overwrite.cancel"); ok {
		t.Fatalf("expected legacy cancel callback data to be rejected")
	}
}
