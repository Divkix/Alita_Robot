package errors

import (
	stderrors "errors"
	"fmt"
	"strings"
	"testing"
)

func TestWrapNilError(t *testing.T) {
	t.Parallel()

	result := Wrap(nil, "msg")
	if result != nil {
		t.Fatalf("Wrap(nil, ...) expected nil, got %v", result)
	}
}

func TestWrapNonNilError(t *testing.T) {
	t.Parallel()

	base := fmt.Errorf("base error")
	result := Wrap(base, "operation failed")
	if result == nil {
		t.Fatalf("Wrap(err, ...) expected non-nil, got nil")
	}

	we, ok := result.(*WrappedError)
	if !ok {
		t.Fatalf("expected *WrappedError, got %T", result)
	}

	if we.Message != "operation failed" {
		t.Fatalf("expected Message %q, got %q", "operation failed", we.Message)
	}
	if we.Err != base {
		t.Fatalf("expected Err to be base error, got %v", we.Err)
	}
	if we.File == "" {
		t.Fatalf("expected non-empty File")
	}
	if strings.Count(we.File, "/") > 1 {
		t.Fatalf("expected File to have at most 1 slash, got %q", we.File)
	}
	if we.Line <= 0 {
		t.Fatalf("expected Line > 0, got %d", we.Line)
	}
	if we.Function == "" {
		t.Fatalf("expected non-empty Function")
	}
}

func TestWrapfNilError(t *testing.T) {
	t.Parallel()

	result := Wrapf(nil, "op %s", "save")
	if result != nil {
		t.Fatalf("Wrapf(nil, ...) expected nil, got %v", result)
	}
}

func TestWrapfFormatsMessage(t *testing.T) {
	t.Parallel()

	base := fmt.Errorf("base")
	result := Wrapf(base, "op %s id %d", "save", 42)
	if result == nil {
		t.Fatalf("Wrapf(err, ...) expected non-nil, got nil")
	}

	we, ok := result.(*WrappedError)
	if !ok {
		t.Fatalf("expected *WrappedError, got %T", result)
	}

	expected := "op save id 42"
	if we.Message != expected {
		t.Fatalf("expected Message %q, got %q", expected, we.Message)
	}
}

func TestWrappedErrorFormat(t *testing.T) {
	t.Parallel()

	base := fmt.Errorf("original error")
	result := Wrap(base, "context message")
	if result == nil {
		t.Fatalf("Wrap returned nil")
	}

	we := result.(*WrappedError)
	errStr := we.Error()

	if !strings.Contains(errStr, "at") {
		t.Fatalf("Error() missing 'at', got %q", errStr)
	}
	if !strings.Contains(errStr, we.File) {
		t.Fatalf("Error() missing file %q, got %q", we.File, errStr)
	}
	if !strings.Contains(errStr, ":") {
		t.Fatalf("Error() missing ':', got %q", errStr)
	}
	if !strings.Contains(errStr, "in") {
		t.Fatalf("Error() missing 'in', got %q", errStr)
	}
	if !strings.Contains(errStr, we.Function) {
		t.Fatalf("Error() missing function %q, got %q", we.Function, errStr)
	}
	if !strings.Contains(errStr, base.Error()) {
		t.Fatalf("Error() missing original error text %q, got %q", base.Error(), errStr)
	}
}

func TestUnwrapChain(t *testing.T) {
	t.Parallel()

	baseErr := fmt.Errorf("base error")
	wrapped := Wrap(baseErr, "outer")
	if wrapped == nil {
		t.Fatalf("Wrap returned nil")
	}

	if !stderrors.Is(wrapped, baseErr) {
		t.Fatalf("stderrors.Is(wrapped, baseErr) expected true")
	}

	unwrapped := stderrors.Unwrap(wrapped)
	if unwrapped != baseErr {
		t.Fatalf("stderrors.Unwrap expected baseErr, got %v", unwrapped)
	}
}

func TestDoubleWrap(t *testing.T) {
	t.Parallel()

	baseErr := fmt.Errorf("root cause")
	inner := Wrap(baseErr, "inner")
	outer := Wrap(inner, "outer")

	if !stderrors.Is(outer, baseErr) {
		t.Fatalf("stderrors.Is(outer, baseErr) expected true after double wrap")
	}
}

func TestWrapEmptyMessage(t *testing.T) {
	t.Parallel()

	base := fmt.Errorf("base")
	result := Wrap(base, "")
	if result == nil {
		t.Fatalf("Wrap(err, \"\") expected non-nil, got nil")
	}

	we, ok := result.(*WrappedError)
	if !ok {
		t.Fatalf("expected *WrappedError, got %T", result)
	}
	if we.Message != "" {
		t.Fatalf("expected empty Message, got %q", we.Message)
	}
	if we.Err != base {
		t.Fatalf("expected Err to be base, got %v", we.Err)
	}
}

func TestFilePathTruncation(t *testing.T) {
	t.Parallel()

	base := fmt.Errorf("err")
	result := Wrap(base, "msg")
	if result == nil {
		t.Fatalf("Wrap returned nil")
	}

	we := result.(*WrappedError)
	slashCount := strings.Count(we.File, "/")
	if slashCount > 1 {
		t.Fatalf("File %q has %d slashes, expected at most 1", we.File, slashCount)
	}
}
