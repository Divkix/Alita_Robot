package modules

import (
	"errors"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/opstate"
)

const overwriteCacheTTL = 5 * time.Minute

var errOverwriteCacheMiss = errors.New("overwrite cache miss")

// overwriteBase holds common fields for temporary state storage during command flows.
type overwriteBase struct {
	ChatID   int64
	UserID   int64
	ItemName string // filterWord or noteWord
	Text     string
	FileID   string
	Buttons  []db.Button
	DataType int
}

// struct for filters module
type overwriteFilter struct {
	overwriteBase
}

// struct for notes module
type overwriteNote struct {
	overwriteBase
	PvtOnly     bool
	GrpOnly     bool
	AdminOnly   bool
	WebPrev     bool
	IsProtected bool
	NoNotif     bool
}

func overwriteCacheKey(kind, token string) string {
	return opstate.Key(kind+"_overwrite", token)
}

func setOverwriteCache(key string, data any) error {
	return opstate.Stash(key, data, overwriteCacheTTL)
}

func getOverwriteCache[T any](key string) (*T, error) {
	data, err := opstate.Peek[T](key)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, errOverwriteCacheMiss
	}
	return data, nil
}

func consumeOverwriteCache[T any](key string) (*T, error) {
	data, err := opstate.Consume[T](key)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, errOverwriteCacheMiss
	}
	return data, nil
}

func deleteOverwriteCache(key string) {
	opstate.Delete(key)
}

// overwriteGroup returns the group the overwrite stored under key acts on: the
// one /save or /filter ran in, never the callback data or the chat the button
// is pressed in, which is the PM when the command came from there. It reads
// the overwrite without consuming it, so a refused press leaves it to its
// admin, and returns 0 when nothing is stored. ok is false, with the query
// answered, when the presser is no admin of that group.
func overwriteGroup(b *gotgbot.Bot, ctx *ext.Context, key string, userID int64) (chatID int64, ok bool) {
	data, err := getOverwriteCache[overwriteBase](key)
	if err != nil {
		return 0, true
	}
	if !chat_status.RequireUserAdmin(b, ctx, &gotgbot.Chat{Id: data.ChatID}, userID) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return 0, false
	}
	return data.ChatID, true
}

// editOverwriteAnswer replaces the overwrite question in message with text.
// Pressed in PM, the text names group chatID (withButtonGroupLine) and is
// sent as HTML; in the group it stays plain text, as it always was.
func editOverwriteAnswer(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator, message gotgbot.MaybeInaccessibleMessage, chatID int64, text string) error {
	opts := &gotgbot.EditMessageTextOpts{Text: text}
	if chatID != 0 && inPM(ctx.EffectiveMessage) {
		opts.Text, opts.ParseMode = withButtonGroupLine(b, ctx, tr, chatID, text), formatting.HTML
	}
	_, _, err := message.EditText(b, opts)
	return err
}
