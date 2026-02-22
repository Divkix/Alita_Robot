package error_handling

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestHandleErr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{
			name: "nil error is no-op",
			err:  nil,
		},
		{
			name: "non-nil error is logged without panic",
			err:  errors.New("test error"),
		},
		{
			name: "wrapped error is logged without panic",
			err:  fmt.Errorf("outer: %w", errors.New("inner error")),
		},
		{
			name: "empty message error",
			err:  errors.New(""),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// HandleErr must not panic under any circumstances
			HandleErr(tc.err)
		})
	}
}

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
		// Should not panic itself and should complete normally
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

func TestCaptureError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		tags map[string]string
	}{
		{
			name: "nil error is no-op",
			err:  nil,
			tags: map[string]string{"key": "value"},
		},
		{
			name: "non-nil error with tags",
			err:  errors.New("something went wrong"),
			tags: map[string]string{"module": "test", "user": "123"},
		},
		{
			name: "non-nil error with nil tags",
			err:  errors.New("error with nil tags"),
			tags: nil,
		},
		{
			name: "non-nil error with empty tags map",
			err:  errors.New("error with empty tags"),
			tags: map[string]string{},
		},
		{
			name: "wrapped error with tags",
			err:  fmt.Errorf("outer: %w", errors.New("inner")),
			tags: map[string]string{"component": "db", "op": "insert"},
		},
		{
			name: "nil error with nil tags is no-op",
			err:  nil,
			tags: nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// CaptureError must not panic under any input combination
			CaptureError(tc.err, tc.tags)
		})
	}
}

func TestHandleErrConcurrent(t *testing.T) {
	t.Parallel()

	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			if i%2 == 0 {
				HandleErr(nil)
			} else {
				HandleErr(fmt.Errorf("concurrent error %d", i))
			}
		}()
	}

	wg.Wait()
}

func TestCaptureErrorConcurrent(t *testing.T) {
	t.Parallel()

	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			if i%2 == 0 {
				CaptureError(nil, map[string]string{"i": fmt.Sprintf("%d", i)})
			} else {
				CaptureError(
					fmt.Errorf("concurrent capture error %d", i),
					map[string]string{"goroutine": fmt.Sprintf("%d", i)},
				)
			}
		}()
	}

	wg.Wait()
}
