package chat_status

import (
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

// respondCfg holds per-call options for PermissionResponder.Respond.
type respondCfg struct {
	useReply              bool
	fallbackToSendMessage bool
}

// respondOpt configures a single Respond call.
type respondOpt func(*respondCfg)

// WithReply makes the responder use msg.Reply instead of b.SendMessage.
// This preserves the exact behaviour of functions like RequireBotAdmin.
func WithReply() respondOpt {
	return func(c *respondCfg) {
		c.useReply = true
	}
}

// WithReplyFallback makes the responder try msg.Reply first and fall back to
// b.SendMessage with ReplyParameters if Reply fails. Used by RequireUserAdmin.
func WithReplyFallback() respondOpt {
	return func(c *respondCfg) {
		c.useReply = true
		c.fallbackToSendMessage = true
	}
}

// PermissionResponder centralises the response choreography for failed
// permission checks. Callers perform the pure check; when it fails they
// delegate error messaging to Respond.
type PermissionResponder struct {
	bot *gotgbot.Bot
}

// NewPermissionResponder creates a PermissionResponder for the given bot.
func NewPermissionResponder(b *gotgbot.Bot) *PermissionResponder {
	return &PermissionResponder{bot: b}
}

// Respond sends the correct error response for a failed permission check.
//
//   - If btnKey is non-empty and the update is a callback query, the text is
//     answered via query.Answer with btnKey translation.
//   - Otherwise the text is sent as a chat message using cmdKey translation.
//     By default SendMessage with ReplyParameters{AllowSendingWithoutReply:true}
//     is used. Callers that previously used msg.Reply can pass WithReply().
//     RequireUserAdmin passes WithReplyFallback().
//
// It always returns false so the permission check function can return it
// directly.
func (r *PermissionResponder) Respond(ctx *ext.Context, cmdKey, btnKey string, opts ...respondOpt) bool {
	cfg := respondCfg{}
	for _, o := range opts {
		o(&cfg)
	}

	chat := extractChatFromContext(ctx, nil)
	if chat == nil || ctx == nil || ctx.EffectiveMessage == nil {
		return false
	}

	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	// Callback query path.
	if btnKey != "" {
		query, _ := callbackQueryFromContext(ctx)
		if query != nil {
			text, _ := tr.GetString(btnKey)
			_, err := query.Answer(r.bot, &gotgbot.AnswerCallbackQueryOpts{Text: text})
			if err != nil {
				log.WithFields(log.Fields{
					"chatId": chat.Id,
					"btnKey": btnKey,
				}).Errorf("callback answer failed: %v", err)
			}
			return false
		}
	}

	text, _ := tr.GetString(cmdKey)

	if cfg.useReply {
		msg := ctx.EffectiveMessage
		_, err := msg.Reply(r.bot, text, nil)
		if err != nil {
			log.WithFields(log.Fields{
				"chatId": chat.Id,
				"cmdKey": cmdKey,
			}).Warningf("reply failed: %v", err)

			if cfg.fallbackToSendMessage {
				_, fallbackErr := r.bot.SendMessage(chat.Id, text, &gotgbot.SendMessageOpts{
					ReplyParameters: &gotgbot.ReplyParameters{
						MessageId:                msg.MessageId,
						AllowSendingWithoutReply: true,
					},
				})
				if fallbackErr != nil {
					log.WithFields(log.Fields{
						"chatId":        chat.Id,
						"cmdKey":        cmdKey,
						"replyError":    err,
						"fallbackError": fallbackErr,
					}).Errorf("sendMessage fallback also failed: %v", fallbackErr)
				}
			}
		}
		return false
	}

	_, err := r.bot.SendMessage(chat.Id, text, &gotgbot.SendMessageOpts{
		ReplyParameters: &gotgbot.ReplyParameters{
			MessageId:                ctx.EffectiveMessage.MessageId,
			AllowSendingWithoutReply: true,
		},
	})
	if err != nil {
		log.WithFields(log.Fields{
			"chatId": chat.Id,
			"cmdKey": cmdKey,
		}).Errorf("sendMessage failed: %v", err)
	}
	return false
}
