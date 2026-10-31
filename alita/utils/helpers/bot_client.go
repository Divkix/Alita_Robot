package helpers

import (
	"context"
	"encoding/json"
	"maps"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// ReplySafeBotClient lets same-chat sendMessage replies survive a deleted target.
// Cross-chat and ephemeral replies, other methods, and explicit reply_parameters
// request overrides retain their original semantics.
type ReplySafeBotClient struct {
	gotgbot.BotClient
}

// RequestWithContext defaults eligible replies to allow_sending_without_reply
// without changing caller-owned reply parameters, maps, or request options.
func (c ReplySafeBotClient) RequestWithContext(ctx context.Context, token string, method string, params map[string]any, opts *gotgbot.RequestOpts) (json.RawMessage, error) {
	var replyOverridden bool
	if opts != nil {
		_, replyOverridden = opts.OverrideParams["reply_parameters"]
	}
	if method == "sendMessage" && !replyOverridden {
		reply, ok := params["reply_parameters"].(*gotgbot.ReplyParameters)
		if ok && reply != nil && reply.MessageId > 0 && reply.ChatId == 0 && reply.EphemeralMessageId == 0 && !reply.AllowSendingWithoutReply {
			safeReply := *reply
			safeReply.AllowSendingWithoutReply = true
			params = maps.Clone(params)
			params["reply_parameters"] = &safeReply
		}
	}
	return c.BotClient.RequestWithContext(ctx, token, method, params, opts)
}
