package modules

import "testing"

func TestParseNoteOverwriteCallbackDataTokenized(t *testing.T) {
	t.Parallel()

	data := encodeCallbackData("notes.overwrite", map[string]string{
		"a": "yes",
		"t": "tok._-42",
	}, "notes.overwrite.yes.123_key")

	action, token, legacy, ok := parseNoteOverwriteCallbackData(data)
	if !ok {
		t.Fatalf("expected tokenized callback data to parse")
	}
	if action != "yes" {
		t.Fatalf("unexpected action: %q", action)
	}
	if token != "tok._-42" {
		t.Fatalf("unexpected token: %q", token)
	}
	if legacy != "" {
		t.Fatalf("expected empty legacy key, got %q", legacy)
	}
}

func TestParseNoteOverwriteCallbackDataLegacyPreservesPunctuation(t *testing.T) {
	t.Parallel()

	data := "notes.overwrite.yes.123_note.key_with.parts"
	action, token, legacy, ok := parseNoteOverwriteCallbackData(data)
	if !ok {
		t.Fatalf("expected legacy callback data to parse")
	}
	if action != "yes" {
		t.Fatalf("unexpected action: %q", action)
	}
	if token != "" {
		t.Fatalf("expected empty token, got %q", token)
	}
	if legacy != "123_note.key_with.parts" {
		t.Fatalf("unexpected legacy payload: %q", legacy)
	}
}

func TestParseFilterOverwriteCallbackDataTokenized(t *testing.T) {
	t.Parallel()

	data := encodeCallbackData("filters_overwrite", map[string]string{
		"a": "yes",
		"t": "f0.token",
	}, "filters_overwrite.example")

	action, token, legacy, ok := parseFilterOverwriteCallbackData(data)
	if !ok {
		t.Fatalf("expected tokenized callback data to parse")
	}
	if action != "yes" {
		t.Fatalf("unexpected action: %q", action)
	}
	if token != "f0.token" {
		t.Fatalf("unexpected token: %q", token)
	}
	if legacy != "" {
		t.Fatalf("expected empty legacy word, got %q", legacy)
	}
}

func TestParseFilterOverwriteCallbackDataLegacyFallback(t *testing.T) {
	t.Parallel()

	data := "filters_overwrite.foo.bar_baz"
	action, token, legacy, ok := parseFilterOverwriteCallbackData(data)
	if !ok {
		t.Fatalf("expected legacy callback data to parse")
	}
	if action != "yes" {
		t.Fatalf("unexpected action: %q", action)
	}
	if token != "" {
		t.Fatalf("expected empty token, got %q", token)
	}
	if legacy != "foo.bar_baz" {
		t.Fatalf("unexpected legacy word: %q", legacy)
	}
}

func TestParseFilterOverwriteCallbackDataLegacyCancel(t *testing.T) {
	t.Parallel()

	action, token, legacy, ok := parseFilterOverwriteCallbackData("filters_overwrite.cancel")
	if !ok {
		t.Fatalf("expected cancel callback to parse")
	}
	if action != "cancel" || token != "" || legacy != "" {
		t.Fatalf("unexpected parsed values: action=%q token=%q legacy=%q", action, token, legacy)
	}
}
