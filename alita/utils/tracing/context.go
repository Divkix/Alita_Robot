package tracing

import (
	"context"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// ExtractContext safely extracts a context.Context from ext.Context.Data["context"].
// This is used to retrieve the trace context injected by the webhook handler or
// TracingProcessor for downstream propagation into DB operations and other spans.
// Returns context.Background() if no context is found.
func ExtractContext(ctx *ext.Context) context.Context {
	if ctx != nil && ctx.Data != nil {
		if c, ok := ctx.Data["context"].(context.Context); ok {
			return c
		}
	}
	return context.Background()
}
