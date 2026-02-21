package errors

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

var errSentinel = errors.New("errSentinel error")

func TestWrap_NilErr(t *testing.T) {
	if got := Wrap(nil, "msg"); got != nil {
		t.Errorf("Wrap(nil, ...) = %v, want nil", got)
	}
}

func TestWrapf_NilErr(t *testing.T) {
	if got := Wrapf(nil, "msg %s", "x"); got != nil {
		t.Errorf("Wrapf(nil, ...) = %v, want nil", got)
	}
}

func TestWrap_ReturnsWrappedError(t *testing.T) {
	err := Wrap(errSentinel, "context")
	if err == nil {
		t.Fatal("Wrap returned nil for non-nil error")
	}
	var we *WrappedError
	if !errors.As(err, &we) {
		t.Fatalf("Wrap did not return *WrappedError, got %T", err)
	}
	if we.Message != "context" {
		t.Errorf("Message = %q, want %q", we.Message, "context")
	}
	if !errors.Is(err, errSentinel) {
		t.Error("wrapped error should unwrap to errSentinel")
	}
}

func TestWrap_ErrorString(t *testing.T) {
	err := Wrap(errSentinel, "wrapping")
	s := err.Error()
	if !strings.Contains(s, "wrapping") {
		t.Errorf("Error() %q missing message", s)
	}
	if !strings.Contains(s, errSentinel.Error()) {
		t.Errorf("Error() %q missing inner error", s)
	}
	// Should contain file and line info
	if !strings.Contains(s, ":") {
		t.Errorf("Error() %q missing file:line separator", s)
	}
}

func TestWrap_FileAndLine(t *testing.T) {
	err := Wrap(errSentinel, "ctx")
	var we *WrappedError
	errors.As(err, &we)
	if we.File == "" {
		t.Error("File should not be empty")
	}
	if we.Line <= 0 {
		t.Errorf("Line = %d, want > 0", we.Line)
	}
	if we.Function == "" {
		t.Error("Function should not be empty")
	}
	// File should be shortened (last 2 path components)
	parts := strings.Split(we.File, "/")
	if len(parts) > 2 {
		t.Errorf("File %q not shortened, has %d path components", we.File, len(parts))
	}
}

func TestWrap_Unwrap(t *testing.T) {
	inner := errors.New("inner")
	err := Wrap(inner, "outer")
	var we *WrappedError
	errors.As(err, &we)
	if we.Unwrap() != inner {
		t.Errorf("Unwrap() = %v, want %v", we.Unwrap(), inner)
	}
}

func TestWrap_WrappedError(t *testing.T) {
	inner := Wrap(errSentinel, "inner")
	outer := Wrap(inner, "outer")
	// errors.Is traverses the chain
	if !errors.Is(outer, errSentinel) {
		t.Error("errors.Is should find errSentinel through double wrap")
	}
}

func TestWrapf_FormatsMessage(t *testing.T) {
	err := Wrapf(errSentinel, "user %d not found", 42)
	var we *WrappedError
	errors.As(err, &we)
	want := "user 42 not found"
	if we.Message != want {
		t.Errorf("Message = %q, want %q", we.Message, want)
	}
}

func TestWrapf_MultipleArgs(t *testing.T) {
	err := Wrapf(errSentinel, "%s=%v", "key", true)
	var we *WrappedError
	errors.As(err, &we)
	if we.Message != "key=true" {
		t.Errorf("Message = %q, want %q", we.Message, "key=true")
	}
}

func TestWrappedError_ErrorFormat(t *testing.T) {
	we := &WrappedError{
		Err:      fmt.Errorf("root"),
		Message:  "msg",
		File:     "pkg/file.go",
		Line:     99,
		Function: "pkg.Func",
	}
	got := we.Error()
	expected := "msg at pkg/file.go:99 in pkg.Func: root"
	if got != expected {
		t.Errorf("Error() = %q, want %q", got, expected)
	}
}

func TestWrap_FunctionNameShortened(t *testing.T) {
	err := Wrap(errSentinel, "check")
	var we *WrappedError
	errors.As(err, &we)
	// Function name should not contain "/" separators (shortened)
	if strings.Contains(we.Function, "/") {
		t.Errorf("Function %q should not contain '/' after shortening", we.Function)
	}
}
