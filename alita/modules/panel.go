package modules

import (
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/panel"
)

func init() {
	RegisterDeepLinkHandler("panel_", panelDeepLinkHandler)
}

// panelDeepLinkHandler answers /start panel_<chatID> in PM with an inline
// web_app button. The button is sent to any caller; the panel itself refuses a
// chat the caller is not a member of. With the panel disabled the payload is
// unknown, so it falls back to the same help every other unknown deep link
// sends.
func panelDeepLinkHandler(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
	if !config.AppConfig.PanelEnabled {
		return sendDefaultHelp(b, ctx, user)
	}

	chatID, err := strconv.ParseInt(strings.TrimPrefix(arg, "panel_"), 10, 64)
	if err != nil || chatID == 0 {
		return sendDefaultHelp(b, ctx, user)
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	text, _ := tr.GetString("panel_open_message")
	buttonText, _ := tr.GetString("panel_open_button")

	keyboard := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{
			{
				Text:   buttonText,
				WebApp: &gotgbot.WebAppInfo{Url: panel.WebAppURL(config.AppConfig.PanelPublicURL, chatID)},
			},
		}},
	}
	if _, err = b.SendMessage(ctx.EffectiveChat.Id, text, &gotgbot.SendMessageOpts{
		ReplyMarkup: keyboard,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
			IsDisabled: true,
		},
	}); err != nil {
		return err
	}
	return ext.EndGroups
}
