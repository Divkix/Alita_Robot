package chat_status

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

// sendAnonAdminKeyboard sends an inline keyboard to verify anonymous admin identity.
// Creates a callback button that anonymous admins can click to prove their admin status.
func sendAnonAdminKeyboard(b *gotgbot.Bot, msg *gotgbot.Message, chat *gotgbot.Chat) (*gotgbot.Message, error) {
	// Create a minimal context to get the language
	ctx := &ext.Context{
		EffectiveMessage: msg,
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))
	mainText, _ := tr.GetString("chat_status_anon_confirm")
	buttonText, _ := tr.GetString("chat_status_anon_prove_admin")

	return msg.Reply(b,
		mainText,
		&gotgbot.SendMessageOpts{
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{{
						Text:         buttonText,
						CallbackData: fmt.Sprintf("alita:anonAdmin:%d:%d", chat.Id, msg.MessageId),
					}},
				},
			},
		},
	)
}
