package modules

import (
	log "github.com/sirupsen/logrus"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
)

// trS returns the translation for key, discarding the lookup error (which only
// signals a missing key; GetString already falls back to English). It replaces
// the repeated `func() string { t, _ := tr.GetString(key); return t }()` inline
// closures used to set single button labels.
func trS(tr *i18n.Translator, key string) string {
	s, _ := tr.GetString(key)
	return s
}

func ctxTr(ctx *ext.Context) *i18n.Translator {
	return i18n.MustNewTranslator(lang.GetLanguage(ctx))
}

func replyHTML(b *gotgbot.Bot, msg *gotgbot.Message, text string) {
	if msg == nil || text == "" {
		return
	}
	if _, err := msg.Reply(b, text, formatting.Shtml()); err != nil {
		log.Error(err)
	}
}

func requireUser(b *gotgbot.Bot, ctx *ext.Context) *gotgbot.User {
	u := chat_status.RequireUser(b, ctx)
	if u == nil {
		chat_status.NewPermissionResponder(b).Respond(ctx, "common_cannot_identify_user", "", chat_status.WithReply())
	}
	return u
}
