package helpers_test

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"reflect"
	"testing"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

type replySafeTestBotClient struct {
	request func(context.Context, string, string, map[string]any, *gotgbot.RequestOpts) (json.RawMessage, error)
}

func (c replySafeTestBotClient) RequestWithContext(ctx context.Context, token string, method string, params map[string]any, opts *gotgbot.RequestOpts) (json.RawMessage, error) {
	return c.request(ctx, token, method, params, opts)
}

func (replySafeTestBotClient) GetAPIURL(*gotgbot.RequestOpts) string {
	return gotgbot.DefaultAPIURL
}

func (replySafeTestBotClient) FileURL(token string, path string, _ *gotgbot.RequestOpts) string {
	return gotgbot.DefaultAPIURL + "/file/bot" + token + "/" + path
}

func TestReplySafeBotClientSendsDeletedTargetReply(t *testing.T) {
	for _, tc := range []struct {
		name  string
		reply bool
		opts  *gotgbot.SendMessageOpts
	}{
		{name: "reply nil opts", reply: true},
		{
			name:  "reply custom opts",
			reply: true,
			opts: &gotgbot.SendMessageOpts{
				ParseMode:           "HTML",
				DisableNotification: true,
				ReplyMarkup:         gotgbot.ForceReply{ForceReply: true},
			},
		},
		{
			name:  "reply custom target",
			reply: true,
			opts: &gotgbot.SendMessageOpts{
				ReplyParameters: &gotgbot.ReplyParameters{MessageId: 99, Quote: "quoted text", QuotePosition: 3},
			},
		},
		{
			name: "direct sendMessage",
			opts: &gotgbot.SendMessageOpts{
				ParseMode:       "HTML",
				ReplyParameters: &gotgbot.ReplyParameters{MessageId: 42, Quote: "quoted text", QuoteParseMode: "HTML"},
				RequestOpts:     &gotgbot.RequestOpts{Timeout: 3 * time.Second},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var wantOpts *gotgbot.SendMessageOpts
			if tc.opts != nil {
				copyOpts := *tc.opts
				if copyOpts.ReplyParameters != nil {
					copyReply := *copyOpts.ReplyParameters
					copyOpts.ReplyParameters = &copyReply
				} else if tc.reply {
					// gotgbot.Message.Reply fills its target before calling the client.
					copyOpts.ReplyParameters = &gotgbot.ReplyParameters{MessageId: 42}
				}
				wantOpts = &copyOpts
			}
			calls := 0
			var sent map[string]any
			client := replySafeTestBotClient{request: func(_ context.Context, token string, method string, params map[string]any, opts *gotgbot.RequestOpts) (json.RawMessage, error) {
				calls++
				sent = params
				if token != "999:test" || method != "sendMessage" {
					t.Fatalf("request = %q %q, want original token and sendMessage", token, method)
				}
				if tc.opts != nil && opts != tc.opts.RequestOpts {
					t.Fatal("request options were replaced")
				}
				reply, ok := params["reply_parameters"].(*gotgbot.ReplyParameters)
				if !ok || reply == nil || !reply.AllowSendingWithoutReply {
					return nil, &gotgbot.TelegramError{Code: 400, Description: "Bad Request: message to be replied not found"}
				}
				return json.RawMessage(`{"message_id":100,"date":1,"chat":{"id":-1001,"type":"supergroup"}}`), nil
			}}
			bot := &gotgbot.Bot{Token: "999:test", BotClient: helpers.ReplySafeBotClient{BotClient: client}}
			var msg *gotgbot.Message
			var err error
			if tc.reply {
				target := gotgbot.Message{MessageId: 42, Chat: gotgbot.Chat{Id: -1001, Type: "supergroup"}}
				msg, err = target.Reply(bot, "reply text", tc.opts)
			} else {
				msg, err = bot.SendMessage(-1001, "reply text", tc.opts)
			}
			if err != nil || msg == nil || msg.MessageId != 100 {
				t.Fatalf("send deleted-target reply = %#v, %v, want sent message", msg, err)
			}
			if calls != 1 {
				t.Fatalf("API sends = %d, want 1", calls)
			}
			if !reflect.DeepEqual(tc.opts, wantOpts) {
				t.Fatalf("caller opts changed: got %#v, want %#v", tc.opts, wantOpts)
			}
			wantReply := gotgbot.ReplyParameters{MessageId: 42}
			if wantOpts != nil {
				wantReply = *wantOpts.ReplyParameters
				for key, want := range map[string]any{
					"parse_mode":           wantOpts.ParseMode,
					"disable_notification": wantOpts.DisableNotification,
					"reply_markup":         wantOpts.ReplyMarkup,
				} {
					if want == "" || want == false || want == nil {
						continue
					}
					if !reflect.DeepEqual(sent[key], want) {
						t.Errorf("%s = %#v, want %#v", key, sent[key], want)
					}
				}
			}
			wantReply.AllowSendingWithoutReply = true
			if !reflect.DeepEqual(sent["reply_parameters"], &wantReply) || sent["chat_id"] != int64(-1001) || sent["text"] != "reply text" {
				t.Fatalf("sent params = %#v, want original target, text and reply with safe fallback", sent)
			}
		})
	}
}

func TestReplySafeBotClientPreservesOtherRequests(t *testing.T) {
	for _, tc := range []struct {
		name   string
		method string
		reply  any
	}{
		{name: "no reply", method: "sendMessage"},
		{name: "typed nil reply", method: "sendMessage", reply: (*gotgbot.ReplyParameters)(nil)},
		{name: "raw reply", method: "sendMessage", reply: `{"message_id":42}`},
		{name: "zero target", method: "sendMessage", reply: &gotgbot.ReplyParameters{}},
		{name: "negative target", method: "sendMessage", reply: &gotgbot.ReplyParameters{MessageId: -1}},
		{name: "cross-chat reply", method: "sendMessage", reply: &gotgbot.ReplyParameters{MessageId: 42, ChatId: -1002}},
		{name: "ephemeral reply", method: "sendMessage", reply: &gotgbot.ReplyParameters{MessageId: 42, EphemeralMessageId: 7}},
		{name: "ephemeral-only reply", method: "sendMessage", reply: &gotgbot.ReplyParameters{EphemeralMessageId: 7}},
		{name: "already safe reply", method: "sendMessage", reply: &gotgbot.ReplyParameters{MessageId: 42, AllowSendingWithoutReply: true}},
		{name: "other method", method: "sendPhoto", reply: &gotgbot.ReplyParameters{MessageId: 42}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := map[string]any{"chat_id": int64(-1001)}
			if tc.reply != nil {
				params["reply_parameters"] = tc.reply
			}
			wantParams := maps.Clone(params)
			var wantReply gotgbot.ReplyParameters
			if reply, ok := tc.reply.(*gotgbot.ReplyParameters); ok && reply != nil {
				wantReply = *reply
			}
			calls := 0
			client := helpers.ReplySafeBotClient{BotClient: replySafeTestBotClient{request: func(_ context.Context, _ string, method string, got map[string]any, _ *gotgbot.RequestOpts) (json.RawMessage, error) {
				calls++
				if method != tc.method || !reflect.DeepEqual(got, wantParams) || got["reply_parameters"] != tc.reply {
					t.Fatalf("request changed: %s %#v", method, got)
				}
				return json.RawMessage(`true`), nil
			}}}
			result, err := client.RequestWithContext(context.Background(), "999:test", tc.method, params, nil)
			if err != nil || string(result) != "true" || calls != 1 {
				t.Fatalf("request = %s, %v, calls %d, want true, nil, 1", result, err, calls)
			}
			if reply, ok := tc.reply.(*gotgbot.ReplyParameters); ok && reply != nil && !reflect.DeepEqual(*reply, wantReply) {
				t.Fatal("caller reply parameters changed")
			}
		})
	}
}

func TestReplySafeBotClientPreservesRequestOverrides(t *testing.T) {
	for _, override := range []any{
		&gotgbot.ReplyParameters{MessageId: 99},
		&gotgbot.ReplyParameters{MessageId: 99, ChatId: -1002},
		`{"message_id":99,"allow_sending_without_reply":false}`,
		nil,
	} {
		params := map[string]any{"reply_parameters": &gotgbot.ReplyParameters{MessageId: 42}}
		opts := &gotgbot.RequestOpts{Timeout: time.Second, OverrideParams: map[string]any{"reply_parameters": override, "text": "override text"}}
		wantOverrides, err := json.Marshal(opts.OverrideParams)
		if err != nil {
			t.Fatal(err)
		}
		calls := 0
		client := helpers.ReplySafeBotClient{BotClient: replySafeTestBotClient{request: func(_ context.Context, _ string, _ string, got map[string]any, gotOpts *gotgbot.RequestOpts) (json.RawMessage, error) {
			calls++
			if gotOpts != opts || !reflect.DeepEqual(got, params) {
				t.Fatal("explicit reply override changed the original request")
			}
			return json.RawMessage(`true`), nil
		}}}
		if _, err := client.RequestWithContext(context.Background(), "999:test", "sendMessage", params, opts); err != nil {
			t.Fatal(err)
		}
		gotOverrides, err := json.Marshal(opts.OverrideParams)
		if err != nil || string(gotOverrides) != string(wantOverrides) || calls != 1 {
			t.Fatalf("overrides = %s, %v, calls %d, want unchanged overrides and one request", gotOverrides, err, calls)
		}
	}
}

func TestReplySafeBotClientReturnsErrorsWithoutRetry(t *testing.T) {
	reply := &gotgbot.ReplyParameters{MessageId: 42}
	params := map[string]any{"reply_parameters": reply, "text": "original text"}
	opts := &gotgbot.RequestOpts{OverrideParams: map[string]any{"text": "override text"}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wantErr := errors.New("send failed")
	calls := 0
	client := helpers.ReplySafeBotClient{BotClient: replySafeTestBotClient{request: func(gotCtx context.Context, token string, method string, got map[string]any, gotOpts *gotgbot.RequestOpts) (json.RawMessage, error) {
		calls++
		if gotCtx != ctx || token != "999:test" || method != "sendMessage" || gotOpts != opts {
			t.Fatal("request context, token, method or options changed")
		}
		if !got["reply_parameters"].(*gotgbot.ReplyParameters).AllowSendingWithoutReply {
			t.Fatal("unrelated overrides disabled safe reply defaults")
		}
		return nil, wantErr
	}}}
	result, err := client.RequestWithContext(ctx, "999:test", "sendMessage", params, opts)
	if result != nil || err != wantErr || calls != 1 {
		t.Fatalf("request = %s, %v, calls %d, want nil, original error, 1", result, err, calls)
	}
	if params["reply_parameters"] != reply || reply.AllowSendingWithoutReply || params["text"] != "original text" || opts.OverrideParams["text"] != "override text" {
		t.Fatal("caller parameters or overrides changed")
	}
}
