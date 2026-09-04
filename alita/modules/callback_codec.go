package modules

import (
	"slices"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/callbackcodec"
	log "github.com/sirupsen/logrus"
)

func encodeCallbackData(namespace string, fields map[string]string) string {
	data, err := callbackcodec.Encode(namespace, fields)
	if err != nil {
		log.WithFields(log.Fields{
			"namespace": namespace,
			"error":     err,
		}).Warn("[CallbackCodec] Failed to encode callback data - button will be dead")
		return ""
	}
	return data
}

func mustCallbackData(namespace string, fields map[string]string) (string, bool) {
	data := encodeCallbackData(namespace, fields)
	if data == "" {
		return "", false
	}
	return data, true
}

func decodeCallbackData(data string, expectedNamespaces ...string) (*callbackcodec.Decoded, bool) {
	decoded, err := callbackcodec.Decode(data)
	if err != nil {
		return nil, false
	}
	if len(expectedNamespaces) == 0 {
		return decoded, true
	}
	if slices.ContainsFunc(expectedNamespaces, func(expected string) bool {
		return strings.EqualFold(decoded.Namespace, expected)
	}) {
		return decoded, true
	}
	return nil, false
}

func answerInvalidCallback(b *gotgbot.Bot, ctx *ext.Context, query *gotgbot.CallbackQuery) error {
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	text, _ := tr.GetString("common_callback_invalid_request")
	_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
	return ext.EndGroups
}
