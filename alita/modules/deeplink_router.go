package modules

import (
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	log "github.com/sirupsen/logrus"
)

// DeepLinkHandler processes a deep link argument.
type DeepLinkHandler func(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error

var deepLinkRegistry = make(map[string]DeepLinkHandler)
var exactDeepLinkRegistry = make(map[string]DeepLinkHandler)

// RegisterDeepLinkHandler registers a handler for a deep link prefix.
func RegisterDeepLinkHandler(prefix string, handler DeepLinkHandler) {
	deepLinkRegistry[prefix] = handler
}

// RegisterExactDeepLinkHandler registers a handler for an exact deep link match.
func RegisterExactDeepLinkHandler(arg string, handler DeepLinkHandler) {
	exactDeepLinkRegistry[arg] = handler
}

// HandleDeepLink routes a deep link argument to the appropriate handler.
func HandleDeepLink(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
	if handler, ok := exactDeepLinkRegistry[arg]; ok {
		return handler(b, ctx, user, arg)
	}

	var matchedPrefix string
	var handler DeepLinkHandler
	for prefix, h := range deepLinkRegistry {
		if strings.HasPrefix(arg, prefix) && len(prefix) > len(matchedPrefix) {
			matchedPrefix = prefix
			handler = h
		}
	}

	if handler != nil {
		return handler(b, ctx, user, arg)
	}

	return sendDefaultHelp(b, ctx, user)
}

// sendDefaultHelp sends the default start/help message.
func sendDefaultHelp(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User) error {
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	startHelpText := getStartHelp(tr)
	startMarkupKb := getStartMarkup(tr, b.Username)
	_, err := b.SendMessage(ctx.EffectiveChat.Id,
		startHelpText,
		&gotgbot.SendMessageOpts{
			ParseMode: formatting.HTML,
			LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
				IsDisabled: true,
			},
			ReplyMarkup: &startMarkupKb,
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}
