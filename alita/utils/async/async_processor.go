package async

import (
	"context"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/divideprojects/Alita_Robot/alita/config"
)

// AsyncProcessor handles asynchronous processing of non-critical operations
// This is a minimal stub to satisfy main.go requirements
type AsyncProcessor struct {
	enabled bool
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// GlobalAsyncProcessor is the singleton instance
var GlobalAsyncProcessor *AsyncProcessor

// InitializeAsyncProcessor creates and starts the global async processor
// This is a minimal implementation to satisfy main.go requirements
func InitializeAsyncProcessor() {
	ctx, cancel := context.WithCancel(context.Background())
	GlobalAsyncProcessor = &AsyncProcessor{
		enabled: config.EnableAsyncProcessing,
		ctx:     ctx,
		cancel:  cancel,
	}

	if GlobalAsyncProcessor.enabled {
		log.Info("[AsyncProcessor] Initialized (minimal mode)")
	}
}

// StopAsyncProcessor stops the global async processor
// This is a minimal implementation to satisfy main.go requirements
func StopAsyncProcessor() {
	if GlobalAsyncProcessor != nil {
		if GlobalAsyncProcessor.cancel != nil {
			GlobalAsyncProcessor.cancel()
		}
		GlobalAsyncProcessor.wg.Wait()
		log.Info("[AsyncProcessor] Stopped")
	}
}
