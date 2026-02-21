package i18n

import (
	"errors"
	"testing"
)

func TestI18nError_Error_WithoutUnderlyingError(t *testing.T) {
	t.Parallel()
	e := &I18nError{
		Op:      "load",
		Lang:    "en",
		Key:     "some.key",
		Message: "not found",
		Err:     nil,
	}
	got := e.Error()
	want := "i18n load failed for lang=en key=some.key: not found"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestI18nError_Error_WithUnderlyingError(t *testing.T) {
	t.Parallel()
	underlying := errors.New("disk error")
	e := &I18nError{
		Op:      "read",
		Lang:    "fr",
		Key:     "greeting",
		Message: "file read failed",
		Err:     underlying,
	}
	got := e.Error()
	want := "i18n read failed for lang=fr key=greeting: file read failed: disk error"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestI18nError_Unwrap(t *testing.T) {
	t.Parallel()
	underlying := errors.New("wrapped error")
	e := &I18nError{Err: underlying}
	if e.Unwrap() != underlying {
		t.Errorf("Unwrap() = %v, want %v", e.Unwrap(), underlying)
	}
}

func TestI18nError_Unwrap_Nil(t *testing.T) {
	t.Parallel()
	e := &I18nError{Err: nil}
	if e.Unwrap() != nil {
		t.Errorf("Unwrap() should return nil for no underlying error")
	}
}

func TestNewI18nError(t *testing.T) {
	t.Parallel()
	underlying := errors.New("root cause")
	e := NewI18nError("op", "es", "key.sub", "message text", underlying)

	if e.Op != "op" {
		t.Errorf("Op = %q, want %q", e.Op, "op")
	}
	if e.Lang != "es" {
		t.Errorf("Lang = %q, want %q", e.Lang, "es")
	}
	if e.Key != "key.sub" {
		t.Errorf("Key = %q, want %q", e.Key, "key.sub")
	}
	if e.Message != "message text" {
		t.Errorf("Message = %q, want %q", e.Message, "message text")
	}
	if e.Err != underlying {
		t.Errorf("Err = %v, want %v", e.Err, underlying)
	}
}

func TestNewI18nError_ErrorsIs(t *testing.T) {
	t.Parallel()
	underlying := ErrKeyNotFound
	e := NewI18nError("get_string", "en", "missing", "not found", underlying)
	if !errors.Is(e, ErrKeyNotFound) {
		t.Errorf("errors.Is should find ErrKeyNotFound through wrapped error")
	}
}
