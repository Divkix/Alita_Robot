package error_handling

import (
	"errors"
	"testing"

	log "github.com/sirupsen/logrus"
)

func init() {
	// Suppress log output during tests
	log.SetLevel(log.PanicLevel)
}

func TestHandleErr_Nil(t *testing.T) {
	// Should not panic on nil
	HandleErr(nil)
}

func TestHandleErr_NonNil(t *testing.T) {
	// Should not panic on real error
	HandleErr(errors.New("something failed"))
}

func TestRecoverFromPanic_NoPanic(t *testing.T) {
	// When there's no panic, RecoverFromPanic should be a no-op
	defer RecoverFromPanic("TestFunc", "TestMod")
	// no panic occurs — function completes normally
}

func TestRecoverFromPanic_WithPanic(t *testing.T) {
	// Verify that RecoverFromPanic actually catches a panic
	panicked := true
	func() {
		defer RecoverFromPanic("TestFunc", "TestMod")
		panicked = false // won't be set if panic aborts before this
		panic("test panic")
	}()
	// If we reach here, RecoverFromPanic caught the panic
	if panicked {
		t.Fatal("expected panicked to be false after RecoverFromPanic caught the panic")
	}
}

func TestRecoverFromPanic_WithErrorValue(t *testing.T) {
	// recover() only works when RecoverFromPanic is directly deferred.
	func() {
		defer RecoverFromPanic("TestFunc", "TestMod")
		panic(errors.New("error panic"))
	}()
}

func TestRecoverFromPanic_WithIntValue(t *testing.T) {
	func() {
		defer RecoverFromPanic("F", "M")
		panic(42)
	}()
}

func TestCaptureError_Nil(t *testing.T) {
	// Should not panic on nil error
	CaptureError(nil, map[string]string{"key": "val"})
}

func TestCaptureError_NilTags(t *testing.T) {
	// Should not panic with nil tags
	CaptureError(errors.New("err"), nil)
}

func TestCaptureError_WithTags(t *testing.T) {
	CaptureError(errors.New("something"), map[string]string{
		"module": "test",
		"user":   "123",
	})
}

func TestCaptureError_EmptyTags(t *testing.T) {
	CaptureError(errors.New("err"), map[string]string{})
}

func TestCaptureError_MultipleTags(t *testing.T) {
	tags := map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
	}
	// Should not panic
	CaptureError(errors.New("multi-tag error"), tags)
}
