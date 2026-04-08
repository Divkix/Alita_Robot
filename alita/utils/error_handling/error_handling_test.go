package error_handling

import (
	"errors"
	"testing"
)

func TestRecoverFromPanic(t *testing.T) {
	t.Parallel()

	t.Run("recovers from string panic in goroutine", func(t *testing.T) {
		t.Parallel()
		done := make(chan struct{})
		go func() {
			defer close(done)
			defer RecoverFromPanic("testFunc", "testMod")
			panic("test panic string")
		}()
		<-done
	})

	t.Run("recovers from error panic in goroutine", func(t *testing.T) {
		t.Parallel()
		done := make(chan struct{})
		go func() {
			defer close(done)
			defer RecoverFromPanic("testFunc", "testMod")
			panic(errors.New("panic error"))
		}()
		<-done
	})

	t.Run("recovers from integer panic", func(t *testing.T) {
		t.Parallel()
		done := make(chan struct{})
		go func() {
			defer close(done)
			defer RecoverFromPanic("testFunc", "testMod")
			panic(42)
		}()
		<-done
	})

	t.Run("no-op when no panic occurs", func(t *testing.T) {
		t.Parallel()
		defer RecoverFromPanic("testFunc", "testMod")
	})

	t.Run("empty funcName and modName", func(t *testing.T) {
		t.Parallel()
		done := make(chan struct{})
		go func() {
			defer close(done)
			defer RecoverFromPanic("", "")
			panic("panic with empty names")
		}()
		<-done
	})

	t.Run("only modName empty", func(t *testing.T) {
		t.Parallel()
		done := make(chan struct{})
		go func() {
			defer close(done)
			defer RecoverFromPanic("myFunc", "")
			panic("panic with empty modName")
		}()
		<-done
	})

	t.Run("only funcName empty", func(t *testing.T) {
		t.Parallel()
		done := make(chan struct{})
		go func() {
			defer close(done)
			defer RecoverFromPanic("", "myMod")
			panic("panic with empty funcName")
		}()
		<-done
	})
}
