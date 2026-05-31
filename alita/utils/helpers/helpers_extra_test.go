package helpers

import (
	"fmt"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

// ---------------------------------------------------------------------------
// AddCmdToDisableable
// ---------------------------------------------------------------------------

func TestAddCmdToDisableable(t *testing.T) {
	const testCmd = "test_disableable_cmd_42"

	cmdsMu.Lock()
	orig := make([]string, len(DisableCmds))
	copy(orig, DisableCmds)
	cmdsMu.Unlock()

	defer func() {
		cmdsMu.Lock()
		DisableCmds = orig
		cmdsMu.Unlock()
	}()

	AddCmdToDisableable(testCmd)

	cmdsMu.Lock()
	found := false
	for _, c := range DisableCmds {
		if c == testCmd {
			found = true
			break
		}
	}
	cmdsMu.Unlock()

	if !found {
		t.Fatalf("expected %q in DisableCmds", testCmd)
	}
}

func TestAddCmdToDisableableThreadSafe(t *testing.T) {
	cmdsMu.Lock()
	orig := make([]string, len(DisableCmds))
	copy(orig, DisableCmds)
	cmdsMu.Unlock()

	defer func() {
		cmdsMu.Lock()
		DisableCmds = orig
		cmdsMu.Unlock()
	}()

	const n = 100
	done := make(chan bool, n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			AddCmdToDisableable(fmt.Sprintf("concurrent_cmd_%d", idx))
			done <- true
		}(i)
	}
	for i := 0; i < n; i++ {
		<-done
	}

	cmdsMu.Lock()
	count := len(DisableCmds)
	cmdsMu.Unlock()

	if count != len(orig)+n {
		t.Fatalf("expected %d commands in DisableCmds, got %d", len(orig)+n, count)
	}
}

// ---------------------------------------------------------------------------
// MultiCommand
// ---------------------------------------------------------------------------

func TestMultiCommand(t *testing.T) {
	t.Parallel()

	b := &gotgbot.Bot{Token: "123:abc"}
	d := ext.NewDispatcher(&ext.DispatcherOpts{})

	called := make(map[string]bool)
	handler := func(cmd string) handlers.Response {
		return func(_ *gotgbot.Bot, _ *ext.Context) error {
			called[cmd] = true
			return nil
		}
	}

	MultiCommand(d, []string{"cmda", "cmdb"}, handler("multi"))

	u1 := &gotgbot.Update{
		Message: &gotgbot.Message{
			Chat:     gotgbot.Chat{Id: 1, Type: "private"},
			From:     &gotgbot.User{Id: 1, IsBot: false, FirstName: "T"},
			Text:     "/cmda",
			Entities: []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 5}},
		},
	}
	if err := d.ProcessUpdate(b, u1, nil); err != nil {
		t.Fatalf("ProcessUpdate(/cmda) unexpected error: %v", err)
	}
	if !called["multi"] {
		t.Fatal("expected handler to be called for /cmda")
	}

	called = make(map[string]bool)
	u2 := &gotgbot.Update{
		Message: &gotgbot.Message{
			Chat:     gotgbot.Chat{Id: 1, Type: "private"},
			From:     &gotgbot.User{Id: 1, IsBot: false, FirstName: "T"},
			Text:     "/cmdb",
			Entities: []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 5}},
		},
	}
	if err := d.ProcessUpdate(b, u2, nil); err != nil {
		t.Fatalf("ProcessUpdate(/cmdb) unexpected error: %v", err)
	}
	if !called["multi"] {
		t.Fatal("expected handler to be called for /cmdb")
	}

	called = make(map[string]bool)
	u3 := &gotgbot.Update{
		Message: &gotgbot.Message{
			Chat:     gotgbot.Chat{Id: 1, Type: "private"},
			From:     &gotgbot.User{Id: 1, IsBot: false, FirstName: "T"},
			Text:     "/cmdc",
			Entities: []gotgbot.MessageEntity{{Type: "bot_command", Offset: 0, Length: 5}},
		},
	}
	if err := d.ProcessUpdate(b, u3, nil); err != nil {
		t.Fatalf("ProcessUpdate(/cmdc) unexpected error: %v", err)
	}
	if called["multi"] {
		t.Fatal("expected handler NOT to be called for /cmdc")
	}
}
