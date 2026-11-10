package modules

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
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

// trSprintf is trS followed by fmt.Sprintf with args.
func trSprintf(tr *i18n.Translator, key string, args ...any) string {
	return fmt.Sprintf(trS(tr, key), args...)
}

func ctxTr(ctx *ext.Context) *i18n.Translator {
	return i18n.MustNewTranslator(lang.GetLanguage(ctx))
}

func replyHTML(b *gotgbot.Bot, msg *gotgbot.Message, text string) {
	if msg == nil || text == "" {
		return
	}
	sendHTMLReply(b, msg, text)
}

func sendHTMLReply(b *gotgbot.Bot, msg *gotgbot.Message, text string) {
	if _, err := msg.Reply(b, text, formatting.Shtml()); err != nil {
		log.Error(err)
	}
}
