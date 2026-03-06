package async

import (
	"testing"
)

func TestInitializeAndStop(t *testing.T) {
	t.Cleanup(func() {
		asyncProcessorMu.Lock()
		GlobalAsyncProcessor = nil
		asyncProcessorMu.Unlock()
	})

	InitializeAsyncProcessor()

	asyncProcessorMu.RLock()
	proc := GlobalAsyncProcessor
	asyncProcessorMu.RUnlock()

	if proc == nil {
		t.Fatal("GlobalAsyncProcessor should not be nil after InitializeAsyncProcessor()")
	}

	// Should not panic
	StopAsyncProcessor()
}

func TestStopBeforeInit(t *testing.T) {
	t.Cleanup(func() {
		asyncProcessorMu.Lock()
		GlobalAsyncProcessor = nil
		asyncProcessorMu.Unlock()
	})

	asyncProcessorMu.Lock()
	GlobalAsyncProcessor = nil
	asyncProcessorMu.Unlock()

	// Should not panic when processor is nil
	StopAsyncProcessor()
}

func TestDoubleStop(t *testing.T) {
	t.Cleanup(func() {
		asyncProcessorMu.Lock()
		GlobalAsyncProcessor = nil
		asyncProcessorMu.Unlock()
	})

	InitializeAsyncProcessor()

	// Both stops should be panic-free
	StopAsyncProcessor()
	StopAsyncProcessor()
}
