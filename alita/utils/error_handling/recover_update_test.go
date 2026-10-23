package error_handling

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/divkix/Alita_Robot/alita/utils/metrics"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

func TestRecoverFromUpdatePanicLabelsTheUpdate(t *testing.T) {
	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{MaxRoutines: -1})
	dispatcher.AddHandler(handlers.NewMessage(nil, func(_ *gotgbot.Bot, ctx *ext.Context) error {
		defer RecoverFromUpdatePanic(ctx, "handler", "test")
		panic("boom")
	}))
	update := ext.NewContext(&gotgbot.Bot{}, &gotgbot.Update{Message: &gotgbot.Message{Text: "hi"}}, nil)
	panics := metrics.UpdatesProcessed.WithLabelValues("message", metrics.OutcomePanic.String(), "")

	before := testutil.ToFloat64(panics)
	if err := (tracing.TracingProcessor{}).ProcessUpdate(dispatcher, &gotgbot.Bot{}, update); err != nil {
		t.Fatalf("ProcessUpdate() error = %v", err)
	}
	if got := testutil.ToFloat64(panics) - before; got != 1 {
		t.Fatalf("panic updates delta = %v, want 1", got)
	}
}
