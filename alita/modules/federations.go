package modules

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/divkix/Alita_Robot/alita/db/federations"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/user"
	"github.com/divkix/Alita_Robot/alita/i18n"
	chatsettings "github.com/divkix/Alita_Robot/alita/settings"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/punishment"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

const (
	fedHandlerGroup      = -6
	fedExportCooldown    = 30 * time.Minute
	fedExportKeyPrefix   = "alita:fed_export:"
	fedCallbackNamespace = "fed"
)

var federationsModule = moduleStruct{
	moduleName:   "Federations",
	handlerGroup: fedHandlerGroup,
}

func parseToggleArg(arg string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "on", "yes", "true", "enable":
		return true, true
	case "off", "no", "false", "disable":
		return false, true
	default:
		return false, false
	}
}

func parseFedID(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return "", false
	}
	return id.String(), true
}

func (moduleStruct) newFed(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	from := ctx.EffectiveUser
	tr := ctxTr(ctx)
	name := strings.TrimSpace(strings.Join(ctx.Args()[1:], " "))
	if name == "" {
		text, _ := tr.GetString("feds_need_name")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	fed, err := federations.CreateFederation(from.Id, name)
	if err != nil {
		if errors.Is(err, federations.ErrAlreadyOwnsFed) {
			text, _ := tr.GetString("feds_already_own")
			replyHTML(b, msg, text)
			return ext.EndGroups
		}
		log.Errorf("[Federations] newFed: %v", err)
		text, _ := tr.GetString("feds_create_failed")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	text, _ := tr.GetString("feds_created", i18n.TranslationParams{
		"name": html.EscapeString(fed.Name),
		"id":   fed.FedID,
	})
	replyHTML(b, msg, text)
	return ext.EndGroups
}

func (moduleStruct) renameFed(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	from := ctx.EffectiveUser
	tr := ctxTr(ctx)
	name := strings.TrimSpace(strings.Join(ctx.Args()[1:], " "))
	if name == "" {
		text, _ := tr.GetString("feds_need_name")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	fed, err := federations.RenameFederation(from.Id, name)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			text, _ := tr.GetString("feds_no_fed")
			replyHTML(b, msg, text)
			return ext.EndGroups
		}
		text, _ := tr.GetString("feds_rename_failed")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	text, _ := tr.GetString("feds_renamed", i18n.TranslationParams{
		"name": html.EscapeString(fed.Name),
	})
	replyHTML(b, msg, text)
	return ext.EndGroups
}

func (moduleStruct) delFed(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	tr := ctxTr(ctx)
	fed := ctx.Data["federationCommandFed"].(*models.Federation)
	data := encodeCallbackData(fedCallbackNamespace, map[string]string{"a": "del"})
	cancel := encodeCallbackData(fedCallbackNamespace, map[string]string{"a": "noop"})
	confirm, _ := tr.GetString("feds_delfed_confirm", i18n.TranslationParams{
		"name": html.EscapeString(fed.Name),
	})
	yes, _ := tr.GetString("feds_delfed_confirm_btn")
	no, _ := tr.GetString("feds_delfed_cancel_btn")
	_, err := msg.Reply(b, confirm, &gotgbot.SendMessageOpts{
		ParseMode: formatting.HTML,
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{
			InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
				{
					{Text: yes, CallbackData: data},
					{Text: no, CallbackData: cancel},
				},
			},
		},
	})
	if err != nil {
		log.Error(err)
	}
	return ext.EndGroups
}

func (moduleStruct) joinFed(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	tr := ctxTr(ctx)
	fedID, ok := parseFedID(strings.Join(ctx.Args()[1:], " "))
	if !ok {
		text, _ := tr.GetString("feds_invalid_id")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	fed := federations.GetFedContext(tracing.UpdateContext(ctx), fedID)
	if fed == nil {
		text, _ := tr.GetString("feds_not_found")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	err := federations.JoinFed(chat.Id, chat.Title, fedID)
	if err != nil {
		if errors.Is(err, federations.ErrAlreadyJoined) {
			text, _ := tr.GetString("feds_already_joined")
			replyHTML(b, msg, text)
			return ext.EndGroups
		}
		text, _ := tr.GetString("feds_join_failed")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	text, _ := tr.GetString("feds_joined", i18n.TranslationParams{
		"name": html.EscapeString(fed.Name),
		"id":   fed.FedID,
	})
	replyHTML(b, msg, text)
	return ext.EndGroups
}

func (moduleStruct) leaveFed(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	tr := ctxTr(ctx)
	if err := federations.LeaveFed(chat.Id); err != nil {
		text, _ := tr.GetString("feds_not_in_fed")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	text, _ := tr.GetString("feds_left")
	replyHTML(b, msg, text)
	return ext.EndGroups
}

func (moduleStruct) quietFed(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	tr := ctxTr(ctx)
	args := ctx.Args()[1:]
	if len(args) == 0 {
		text, _ := tr.GetString("feds_invalid_toggle")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	enabled, ok := parseToggleArg(args[0])
	if !ok {
		text, _ := tr.GetString("feds_invalid_toggle")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if err := chatsettings.QuietFed.Write(chatsettings.WriteContext{Context: tracing.UpdateContext(ctx), ChatID: chat.Id}, enabled); err != nil {
		text, _ := tr.GetString("feds_not_in_fed")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	key := "feds_quiet_off"
	if enabled {
		key = "feds_quiet_on"
	}
	text, _ := tr.GetString(key)
	replyHTML(b, msg, text)
	return ext.EndGroups
}

func formatFedInfo(tr *i18n.Translator, fed *models.Federation) string {
	yesNo := func(v bool) string {
		if v {
			return trS(tr, "common_yes")
		}
		return trS(tr, "common_no")
	}
	text, _ := tr.GetString("feds_info", i18n.TranslationParams{
		"name":   html.EscapeString(fed.Name),
		"id":     fed.FedID,
		"owner":  strconv.FormatInt(fed.OwnerID, 10),
		"chats":  federations.CountFedChats(fed.FedID),
		"bans":   federations.CountFedBans(fed.FedID),
		"admins": len(federations.ListFedAdmins(fed.FedID)),
		"subs":   len(federations.ListSubscribedFedIDs(fed.FedID)),
		"reason": yesNo(fed.RequireReason),
		"notify": yesNo(fed.NotifyOwner),
	})
	return text
}

func (moduleStruct) fedInfo(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	from := ctx.EffectiveUser
	tr := ctxTr(ctx)
	arg := strings.TrimSpace(strings.Join(ctx.Args()[1:], " "))
	var fed *models.Federation
	if arg != "" {
		fedID, ok := parseFedID(arg)
		if !ok {
			text, _ := tr.GetString("feds_invalid_id")
			replyHTML(b, msg, text)
			return ext.EndGroups
		}
		fed = federations.GetFedContext(tracing.UpdateContext(ctx), fedID)
	} else if !chat_status.RequirePrivate(b, ctx, chat) {
		if membership := federations.GetChatFedContext(tracing.UpdateContext(ctx), chat.Id); membership != nil {
			fed = federations.GetFedContext(tracing.UpdateContext(ctx), membership.FedID)
		}
	}
	if fed == nil {
		fed = federations.GetFedByOwner(from.Id)
	}
	if fed == nil {
		text, _ := tr.GetString("feds_not_found")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	replyHTML(b, msg, formatFedInfo(tr, fed))
	return ext.EndGroups
}

func (moduleStruct) fedAdmins(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	from := ctx.EffectiveUser
	tr := ctxTr(ctx)
	arg := strings.TrimSpace(strings.Join(ctx.Args()[1:], " "))
	var fed *models.Federation
	if arg != "" {
		fedID, ok := parseFedID(arg)
		if !ok {
			text, _ := tr.GetString("feds_invalid_id")
			replyHTML(b, msg, text)
			return ext.EndGroups
		}
		fed = federations.GetFedContext(tracing.UpdateContext(ctx), fedID)
	} else {
		fed = federations.GetFedByOwner(from.Id)
		if fed == nil {
			if membership := federations.GetChatFedContext(tracing.UpdateContext(ctx), ctx.EffectiveChat.Id); membership != nil {
				fed = federations.GetFedContext(tracing.UpdateContext(ctx), membership.FedID)
			}
		}
	}
	if fed == nil {
		text, _ := tr.GetString("feds_not_found")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	var bld strings.Builder
	header, _ := tr.GetString("feds_admins_header", i18n.TranslationParams{
		"name": html.EscapeString(fed.Name),
	})
	bld.WriteString(header)
	bld.WriteString("\n")
	fmt.Fprintf(&bld, "• %s (<code>%d</code>)", formatting.MentionHtml(fed.OwnerID, trS(tr, "feds_role_owner")), fed.OwnerID)
	for _, adminID := range federations.ListFedAdmins(fed.FedID) {
		_, name, found := user.GetUserInfoById(adminID)
		if !found || name == "" {
			name = strconv.FormatInt(adminID, 10)
		}
		fmt.Fprintf(&bld, "\n• %s (<code>%d</code>)", formatting.MentionHtml(adminID, name), adminID)
	}
	replyHTML(b, msg, bld.String())
	return ext.EndGroups
}

func (moduleStruct) chatFed(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	tr := ctxTr(ctx)
	membership := ctx.Data["federationCommandChat"].(*models.FederationChat)
	fed := federations.GetFedContext(tracing.UpdateContext(ctx), membership.FedID)
	if fed == nil {
		text, _ := tr.GetString("feds_not_in_fed")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	text, _ := tr.GetString("feds_chatfed", i18n.TranslationParams{
		"name": html.EscapeString(fed.Name),
		"id":   fed.FedID,
	})
	replyHTML(b, msg, text)
	return ext.EndGroups
}

func (moduleStruct) myFeds(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	from := ctx.EffectiveUser
	tr := ctxTr(ctx)
	feds := federations.ListFedsForAdmin(from.Id)
	if len(feds) == 0 {
		text, _ := tr.GetString("feds_myfeds_none")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	header, _ := tr.GetString("feds_myfeds_header")
	var bld strings.Builder
	bld.WriteString(header)
	for _, fed := range feds {
		role := trS(tr, "feds_role_admin")
		if fed.OwnerID == from.Id {
			role = trS(tr, "feds_role_owner")
		}
		fmt.Fprintf(&bld, "\n• <b>%s</b> (<code>%s</code>) — %s", html.EscapeString(fed.Name), fed.FedID, role)
	}
	replyHTML(b, msg, bld.String())
	return ext.EndGroups
}

func (m moduleStruct) fedPromote(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.changeFedAdmin(b, ctx, true)
}

func (m moduleStruct) fedDemote(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.changeFedAdmin(b, ctx, false)
}

func (moduleStruct) changeFedAdmin(b *gotgbot.Bot, ctx *ext.Context, promote bool) error {
	msg := ctx.EffectiveMessage
	from := ctx.EffectiveUser
	tr := ctxTr(ctx)
	fed := ctx.Data["federationCommandFed"].(*models.Federation)
	targetID := extraction.ExtractUser(b, ctx)
	if targetID == -1 {
		return ext.EndGroups
	}
	if targetID == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if targetID == from.Id {
		text, _ := tr.GetString("feds_cannot_promote_self")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if targetID == fed.OwnerID {
		text, _ := tr.GetString("feds_cannot_promote_owner")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if promote {
		err := federations.PromoteFedAdmin(fed.FedID, targetID)
		if err != nil {
			if errors.Is(err, federations.ErrAlreadyAdmin) || errors.Is(err, federations.ErrOwnerIsAdmin) {
				text, _ := tr.GetString("feds_already_admin")
				replyHTML(b, msg, text)
				return ext.EndGroups
			}
			text, _ := tr.GetString("feds_create_failed")
			replyHTML(b, msg, text)
			return ext.EndGroups
		}
		text, _ := tr.GetString("feds_promoted", i18n.TranslationParams{
			"user": formatting.MentionHtml(targetID, strconv.FormatInt(targetID, 10)),
		})
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if err := federations.DemoteFedAdmin(fed.FedID, targetID); err != nil {
		text, _ := tr.GetString("feds_not_fed_admin")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	text, _ := tr.GetString("feds_demoted", i18n.TranslationParams{
		"user": formatting.MentionHtml(targetID, strconv.FormatInt(targetID, 10)),
	})
	replyHTML(b, msg, text)
	return ext.EndGroups
}

func (moduleStruct) fedDemoteMe(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	from := ctx.EffectiveUser
	tr := ctxTr(ctx)
	fedID, ok := parseFedID(strings.Join(ctx.Args()[1:], " "))
	if !ok {
		text, _ := tr.GetString("feds_invalid_id")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if federations.IsFedOwner(fedID, from.Id) {
		text, _ := tr.GetString("feds_demoteme_owner")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if err := federations.DemoteFedAdmin(fedID, from.Id); err != nil {
		text, _ := tr.GetString("feds_demoteme_not_admin")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	text, _ := tr.GetString("feds_demoteme_ok")
	replyHTML(b, msg, text)
	return ext.EndGroups
}

func (moduleStruct) fedReason(b *gotgbot.Bot, ctx *ext.Context) error {
	return toggleOwnedFed(b, ctx, func(fedID string, enabled bool) error {
		return federations.SetRequireReason(fedID, enabled)
	}, "feds_fedreason_on", "feds_fedreason_off")
}

func (moduleStruct) fedNotif(b *gotgbot.Bot, ctx *ext.Context) error {
	return toggleOwnedFed(b, ctx, func(fedID string, enabled bool) error {
		return federations.SetNotifyOwner(fedID, enabled)
	}, "feds_fednotif_on", "feds_fednotif_off")
}

func toggleOwnedFed(
	b *gotgbot.Bot,
	ctx *ext.Context,
	set func(string, bool) error,
	onKey, offKey string,
) error {
	msg := ctx.EffectiveMessage
	tr := ctxTr(ctx)
	fed := ctx.Data["federationCommandFed"].(*models.Federation)
	args := ctx.Args()[1:]
	if len(args) == 0 {
		text, _ := tr.GetString("feds_invalid_toggle")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	enabled, ok := parseToggleArg(args[0])
	if !ok {
		text, _ := tr.GetString("feds_invalid_toggle")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if err := set(fed.FedID, enabled); err != nil {
		text, _ := tr.GetString("feds_create_failed")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	key := offKey
	if enabled {
		key = onKey
	}
	text, _ := tr.GetString(key)
	replyHTML(b, msg, text)
	return ext.EndGroups
}

func (m moduleStruct) fban(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.applyFban(b, ctx, true)
}

func (m moduleStruct) unfban(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.applyFban(b, ctx, false)
}

func (moduleStruct) applyFban(b *gotgbot.Bot, ctx *ext.Context, ban bool) error {
	msg := ctx.EffectiveMessage
	from := ctx.EffectiveUser
	tr := ctxTr(ctx)
	fed := ctx.Data["federationCommandFed"].(*models.Federation)
	targetID, reason := extraction.ExtractUserAndText(b, ctx)
	if targetID == -1 {
		return ext.EndGroups
	}
	if targetID == 0 {
		text, _ := tr.GetString("common_no_user_specified")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if !chat_status.IsValidUserId(targetID) {
		text, _ := tr.GetString("common_anonymous_user_error")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if targetID == from.Id || targetID == b.Id {
		text, _ := tr.GetString("feds_cannot_fban_self")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if targetID == fed.OwnerID {
		text, _ := tr.GetString("feds_cannot_fban_owner")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if federations.IsFedAdmin(fed.FedID, targetID) {
		text, _ := tr.GetString("feds_cannot_fban_admin")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if ban {
		if fed.RequireReason && strings.TrimSpace(reason) == "" {
			text, _ := tr.GetString("feds_need_reason")
			replyHTML(b, msg, text)
			return ext.EndGroups
		}
		_, created, err := federations.Fban(fed.FedID, targetID, from.Id, reason)
		if err != nil {
			text, _ := tr.GetString("feds_fban_failed")
			replyHTML(b, msg, text)
			return ext.EndGroups
		}
		if !created {
			text, _ := tr.GetString("feds_fban_already")
			replyHTML(b, msg, text)
		} else {
			text, _ := tr.GetString("feds_fban_ok", i18n.TranslationParams{
				"user":   formatting.MentionHtml(targetID, strconv.FormatInt(targetID, 10)),
				"name":   html.EscapeString(fed.Name),
				"reason": html.EscapeString(reason),
			})
			replyHTML(b, msg, text)
		}
		logText, _ := tr.GetString("feds_log_fban", i18n.TranslationParams{
			"user_id": targetID,
			"actor":   formatting.MentionHtml(from.Id, from.FirstName),
			"reason":  html.EscapeString(reason),
		})
		if logText != "" {
			notifyFedAction(b, fed, from, logText)
		}
		go applyActiveFban(b, fed.FedID, targetID, from.Id)
		return ext.EndGroups
	}
	if err := federations.Unfban(fed.FedID, targetID); err != nil {
		text, _ := tr.GetString("feds_unfban_not_banned")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	text, _ := tr.GetString("feds_unfban_ok", i18n.TranslationParams{
		"user": formatting.MentionHtml(targetID, strconv.FormatInt(targetID, 10)),
		"name": html.EscapeString(fed.Name),
	})
	replyHTML(b, msg, text)
	logText, _ := tr.GetString("feds_log_unfban", i18n.TranslationParams{
		"user_id": targetID,
		"actor":   formatting.MentionHtml(from.Id, from.FirstName),
	})
	if logText != "" {
		notifyFedAction(b, fed, from, logText)
	}
	go applyActiveUnfban(b, fed.FedID, targetID, from.Id)
	return ext.EndGroups
}

// fedWalkMaxFloodWait caps how long a federation walk waits on Telegram's
// retry_after before giving up on one chat.
const fedWalkMaxFloodWait = 30 * time.Second

// fedBanOnUpdateMaxFloodWait caps the flood wait of a ban made while handling
// an update: the wait holds a dispatcher routine and must end well inside the
// join claim's recentJoinProcessTTL.
const fedBanOnUpdateMaxFloodWait = 3 * time.Second

type fedWalkKey struct {
	fedID  string
	userID int64
}

type fedWalkLock struct {
	sync.Mutex
	waiters int
}

// fedWalkLocks queues the Telegram ban and unban walks of one (federation, user)
// pair inside this process. Each walk re-reads the ban after taking the lock, so
// the last walk to run applies the latest state and a quick fban -> unfban
// cannot end with the earlier walk overtaking the later one.
var fedWalkLocks = struct {
	sync.Mutex
	byKey map[fedWalkKey]*fedWalkLock
}{byKey: make(map[fedWalkKey]*fedWalkLock)}

func lockFedWalk(fedID string, userID int64) (unlock func()) {
	key := fedWalkKey{fedID: fedID, userID: userID}
	fedWalkLocks.Lock()
	lock := fedWalkLocks.byKey[key]
	if lock == nil {
		lock = &fedWalkLock{}
		fedWalkLocks.byKey[key] = lock
	}
	lock.waiters++
	fedWalkLocks.Unlock()

	lock.Lock()
	return func() {
		lock.Unlock()
		fedWalkLocks.Lock()
		lock.waiters--
		if lock.waiters == 0 {
			delete(fedWalkLocks.byKey, key)
		}
		fedWalkLocks.Unlock()
	}
}

// retryAfterFlood runs call once more when Telegram answers 429, after the
// retry_after it asks for, unless that wait exceeds fedWalkMaxFloodWait.
func retryAfterFlood(call func() error) error {
	return retryAfterFloodWithin(fedWalkMaxFloodWait, call)
}

// retryAfterFloodWithin is retryAfterFlood with its own cap on the wait.
func retryAfterFloodWithin(maxWait time.Duration, call func() error) error {
	err := call()
	var tgErr *gotgbot.TelegramError
	if !errors.As(err, &tgErr) || tgErr.Code != http.StatusTooManyRequests || tgErr.ResponseParams == nil {
		return err
	}
	wait := time.Duration(tgErr.ResponseParams.RetryAfter) * time.Second
	if wait > maxWait {
		return err
	}
	time.Sleep(wait)
	return call()
}

// applyActiveFban bans userID, fbanned by actorID, in the federation's chats
// that know the user.
func applyActiveFban(b *gotgbot.Bot, fedID string, userID, actorID int64) {
	defer error_handling.RecoverFromPanic("applyActiveFban", "Federations")
	defer lockFedWalk(fedID, userID)()
	// Only a confirmed missing ban stops the walk; a failed lookup bans anyway.
	ban, lookupErr := federations.GetFedBanE(context.Background(), fedID, userID)
	if lookupErr != nil {
		log.Warnf("[Federations] active fban fed %s user %d: ban lookup failed, banning anyway: %v", fedID, userID, lookupErr)
	} else if ban == nil {
		return
	}
	chatIDs, err := federations.ListFedChatIDs(fedID)
	if err != nil {
		return
	}
	chatIDs = federations.ChatsContainingUser(userID, chatIDs)
	for _, chatID := range chatIDs {
		// Federation bans are permanent and carry no reason on record.
		err := retryAfterFlood(func() error {
			_, err := punishment.Apply(context.Background(), b, punishment.Request{
				ChatID: chatID, Target: punishment.Target{ID: userID}, Mode: punishment.Ban,
				Source: models.BanSourceFed, Detail: fedID, ActorID: actorID,
			})
			return err
		})
		if err != nil {
			log.Debugf("[Federations] active fban chat %d user %d: %v", chatID, userID, err)
		}
	}
	if lookupErr == nil {
		return
	}
	// The walk banned without knowing the ban still exists. An unban walk that ran
	// before it will not come again, so undo the bans if a re-read confirms that
	// no federation in the tree bans the user, as applyActiveUnfban would.
	ban, _, err = federations.FindBanInFedTreeE(context.Background(), fedID, userID)
	if err != nil {
		log.Warnf("[Federations] active fban fed %s user %d: keeping bans, ban re-read failed: %v", fedID, userID, err)
		return
	}
	if ban != nil {
		return
	}
	for _, chatID := range chatIDs {
		err := retryAfterFlood(func() error {
			_, err := punishment.Lift(context.Background(), b, punishment.LiftRequest{
				ChatID: chatID, Target: punishment.Target{ID: userID}, Mode: punishment.Ban,
			})
			return err
		})
		if err != nil {
			log.Debugf("[Federations] active fban undo chat %d user %d: %v", chatID, userID, err)
		}
	}
}

// applyActiveUnfban lifts the Telegram ban in the federation's chats that know the
// user, mirroring applyActiveFban. It does nothing while a subscribed federation
// still bans the user, since enforceFedBan would ban them again on their next
// message, or when that cannot be checked. OnlyIfBanned keeps members who are not
// banned from being kicked.
func applyActiveUnfban(b *gotgbot.Bot, fedID string, userID, actorID int64) {
	defer error_handling.RecoverFromPanic("applyActiveUnfban", "Federations")
	defer lockFedWalk(fedID, userID)()
	ban, _, err := federations.FindBanInFedTreeE(context.Background(), fedID, userID)
	if err != nil {
		log.Warnf("[Federations] active unfban fed %s user %d: keeping bans, ban lookup failed: %v", fedID, userID, err)
		return
	}
	if ban != nil {
		return
	}
	chatIDs, err := federations.ListFedChatIDs(fedID)
	if err != nil {
		return
	}
	for _, chatID := range federations.ChatsContainingUser(userID, chatIDs) {
		err := retryAfterFlood(func() error {
			_, err := punishment.Lift(context.Background(), b, punishment.LiftRequest{
				ChatID: chatID, Target: punishment.Target{ID: userID}, Mode: punishment.Ban, ActorID: actorID,
			})
			return err
		})
		if err != nil {
			log.Debugf("[Federations] active unfban chat %d user %d: %v", chatID, userID, err)
		}
	}
}

func notifyFedAction(b *gotgbot.Bot, fed *models.Federation, actor *gotgbot.User, htmlText string) {
	if fed == nil {
		return
	}
	if fed.NotifyOwner && (actor == nil || actor.Id != fed.OwnerID) {
		if _, err := b.SendMessage(fed.OwnerID, htmlText, &gotgbot.SendMessageOpts{ParseMode: formatting.HTML}); err != nil {
			log.Debugf("[Federations] owner notify: %v", err)
		}
	}
	if fed.LogChatID != 0 {
		if _, err := b.SendMessage(fed.LogChatID, htmlText, &gotgbot.SendMessageOpts{ParseMode: formatting.HTML}); err != nil {
			log.Debugf("[Federations] fed log: %v", err)
		}
	}
}

func (moduleStruct) fedStat(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	from := ctx.EffectiveUser
	tr := ctxTr(ctx)
	args := ctx.Args()[1:]
	targetID := from.Id
	var reasonFed string
	if len(args) > 0 {
		if fedID, ok := parseFedID(args[len(args)-1]); ok {
			reasonFed = fedID
			args = args[:len(args)-1]
		}
	}
	if len(args) > 0 || msg.ReplyToMessage != nil {
		extracted := extraction.ExtractUser(b, ctx)
		if extracted == -1 {
			return ext.EndGroups
		}
		if extracted != 0 {
			targetID = extracted
		}
	}
	if reasonFed != "" {
		ban := federations.GetFedBanContext(tracing.UpdateContext(ctx), reasonFed, targetID)
		if ban == nil {
			text, _ := tr.GetString("feds_fedstat_none")
			replyHTML(b, msg, text)
			return ext.EndGroups
		}
		fed := federations.GetFedContext(tracing.UpdateContext(ctx), reasonFed)
		name := reasonFed
		if fed != nil {
			name = fed.Name
		}
		text, _ := tr.GetString("feds_fedstat_reason", i18n.TranslationParams{
			"name":   html.EscapeString(name),
			"id":     reasonFed,
			"reason": html.EscapeString(ban.Reason),
			"date":   ban.CreatedAt.UTC().Format(time.RFC3339),
		})
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	bans, err := federations.ListUserFedBans(targetID)
	if err != nil || len(bans) == 0 {
		text, _ := tr.GetString("feds_fedstat_none")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	header, _ := tr.GetString("feds_fedstat_header", i18n.TranslationParams{
		"user": formatting.MentionHtml(targetID, strconv.FormatInt(targetID, 10)),
	})
	var bld strings.Builder
	bld.WriteString(header)
	for _, ban := range bans {
		name := ban.FedID
		if fed := federations.GetFedContext(tracing.UpdateContext(ctx), ban.FedID); fed != nil {
			name = fed.Name
		}
		fmt.Fprintf(&bld, "\n• <b>%s</b> (<code>%s</code>)", html.EscapeString(name), ban.FedID)
	}
	replyHTML(b, msg, bld.String())
	return ext.EndGroups
}

func (moduleStruct) subFed(b *gotgbot.Bot, ctx *ext.Context) error {
	return changeSub(b, ctx, true)
}

func (moduleStruct) unsubFed(b *gotgbot.Bot, ctx *ext.Context) error {
	return changeSub(b, ctx, false)
}

func changeSub(b *gotgbot.Bot, ctx *ext.Context, subscribe bool) error {
	msg := ctx.EffectiveMessage
	tr := ctxTr(ctx)
	fed := ctx.Data["federationCommandFed"].(*models.Federation)
	targetID, ok := parseFedID(strings.Join(ctx.Args()[1:], " "))
	if !ok {
		text, _ := tr.GetString("feds_invalid_id")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if subscribe {
		err := federations.SubscribeFed(fed.FedID, targetID)
		if err != nil {
			switch {
			case errors.Is(err, federations.ErrSubSelf):
				text, _ := tr.GetString("feds_sub_self")
				replyHTML(b, msg, text)
			case errors.Is(err, federations.ErrAlreadySub):
				text, _ := tr.GetString("feds_already_sub")
				replyHTML(b, msg, text)
			case errors.Is(err, federations.ErrSubLimit):
				text, _ := tr.GetString("feds_sub_max")
				replyHTML(b, msg, text)
			default:
				text, _ := tr.GetString("feds_not_found")
				replyHTML(b, msg, text)
			}
			return ext.EndGroups
		}
		text, _ := tr.GetString("feds_sub_ok")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if err := federations.UnsubscribeFed(fed.FedID, targetID); err != nil {
		text, _ := tr.GetString("feds_not_sub")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	text, _ := tr.GetString("feds_unsub_ok")
	replyHTML(b, msg, text)
	return ext.EndGroups
}

func (moduleStruct) fedSubs(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	tr := ctxTr(ctx)
	fed := ctx.Data["federationCommandFed"].(*models.Federation)
	subs := federations.ListSubscribedFedIDs(fed.FedID)
	if len(subs) == 0 {
		text, _ := tr.GetString("feds_subs_none")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	header, _ := tr.GetString("feds_subs_header")
	var bld strings.Builder
	bld.WriteString(header)
	for _, id := range subs {
		name := id
		if sub := federations.GetFedContext(tracing.UpdateContext(ctx), id); sub != nil {
			name = sub.Name
		}
		fmt.Fprintf(&bld, "\n• <b>%s</b> (<code>%s</code>)", html.EscapeString(name), id)
	}
	replyHTML(b, msg, bld.String())
	return ext.EndGroups
}

func (moduleStruct) setFedLog(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	tr := ctxTr(ctx)
	fed := ctx.Data["federationCommandFed"].(*models.Federation)
	if err := federations.SetFedLogChat(fed.FedID, chat.Id); err != nil {
		text, _ := tr.GetString("feds_create_failed")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	text, _ := tr.GetString("feds_setfedlog_ok")
	replyHTML(b, msg, text)
	return ext.EndGroups
}

func (moduleStruct) unsetFedLog(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	tr := ctxTr(ctx)
	fed := ctx.Data["federationCommandFed"].(*models.Federation)
	if err := federations.SetFedLogChat(fed.FedID, 0); err != nil {
		text, _ := tr.GetString("feds_create_failed")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	text, _ := tr.GetString("feds_unsetfedlog_ok")
	replyHTML(b, msg, text)
	return ext.EndGroups
}

func allowFedExport(fedID string) bool {
	client := cache.GetRedisClient()
	if client == nil {
		return true
	}
	ok, err := client.SetNX(cache.Context, fedExportKeyPrefix+fedID, "1", fedExportCooldown).Result()
	if err != nil {
		return true
	}
	return ok
}

// releaseFedExport frees the export cooldown claimed by allowFedExport, for an
// export that delivered no file.
func releaseFedExport(fedID string) {
	if client := cache.GetRedisClient(); client != nil {
		if err := client.Del(cache.Context, fedExportKeyPrefix+fedID).Err(); err != nil {
			log.Debugf("[Federations] release export cooldown: %v", err)
		}
	}
}

func (moduleStruct) fbanList(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	tr := ctxTr(ctx)
	fed := ctx.Data["federationCommandFed"].(*models.Federation)
	if !allowFedExport(fed.FedID) {
		text, _ := tr.GetString("feds_export_wait")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	delivered := false
	defer func() {
		if !delivered {
			releaseFedExport(fed.FedID)
		}
	}()
	bans, err := federations.ListFedBans(fed.FedID)
	if err != nil {
		text, _ := tr.GetString("feds_create_failed")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if len(bans) == 0 {
		text, _ := tr.GetString("feds_export_empty")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	format := "csv"
	if len(ctx.Args()) > 1 {
		format = strings.ToLower(strings.TrimSpace(ctx.Args()[1]))
	}
	var (
		payload  []byte
		fileName string
	)
	switch format {
	case "json", "jsonl", "ndjson":
		payload, err = formatFedBanJSONL(bans)
		fileName = "fbanlist.json"
	case "minicsv":
		payload, err = formatFedBanCSV(bans, true)
		fileName = "fbanlist.csv"
	default:
		payload, err = formatFedBanCSV(bans, false)
		fileName = "fbanlist.csv"
	}
	if err != nil {
		text, _ := tr.GetString("feds_create_failed")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	_, err = b.SendDocument(ctx.EffectiveChat.Id, gotgbot.InputFileByReader(fileName, bytes.NewReader(payload)), &gotgbot.SendDocumentOpts{
		ReplyParameters: &gotgbot.ReplyParameters{MessageId: msg.MessageId},
	})
	if err != nil {
		log.Error(err)
		text, _ := tr.GetString("feds_create_failed")
		replyHTML(b, msg, text)
	} else {
		delivered = true
	}
	return ext.EndGroups
}

func (moduleStruct) importFBans(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	from := ctx.EffectiveUser
	tr := ctxTr(ctx)
	fed := ctx.Data["federationCommandFed"].(*models.Federation)
	var doc *gotgbot.Document
	if msg.ReplyToMessage != nil {
		doc = msg.ReplyToMessage.Document
	}
	if doc == nil {
		text, _ := tr.GetString("feds_import_need_file")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	data, failText := downloadFedBanDocument(b, doc, tr)
	if data == nil {
		if failText == "" {
			failText, _ = tr.GetString("feds_import_failed")
		}
		replyHTML(b, msg, failText)
		return ext.EndGroups
	}
	bans, err := parseFedBanFile(doc.FileName, data)
	if err != nil || len(bans) == 0 {
		text, _ := tr.GetString("feds_import_failed")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	// Drop the rows /fban would refuse: the bot, the owner and fed admins.
	// An imported ban on them would be enforced but could not be lifted.
	kept := bans[:0]
	for _, ban := range bans {
		if ban.UserID == b.Id || federations.IsFedAdmin(fed.FedID, ban.UserID) {
			continue
		}
		kept = append(kept, ban)
	}
	bans = kept
	for i := range bans {
		if bans[i].BannedBy == 0 {
			bans[i].BannedBy = from.Id
		}
	}
	written, err := federations.ImportBans(fed.FedID, bans)
	if err != nil {
		log.Errorf("[Federations] importFBans: %v", err)
		text, _ := tr.GetString("feds_import_failed")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	text, _ := tr.GetString("feds_import_ok", i18n.TranslationParams{"count": written})
	replyHTML(b, msg, text)
	return ext.EndGroups
}

func (moduleStruct) fedCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	decoded, ok := decodeCallbackData(query.Data, fedCallbackNamespace)
	if !ok {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	action := decoded.Fields["a"]
	switch action {
	case "noop":
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: trS(tr, "feds_delfed_cancel_btn")})
		return ext.EndGroups
	case "del":
		fed := federations.GetFedByOwner(query.From.Id)
		if fed == nil {
			_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: trS(tr, "feds_callback_denied"), ShowAlert: true})
			return ext.EndGroups
		}
		if err := federations.DeleteFederation(fed.FedID); err != nil {
			_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: trS(tr, "feds_delete_failed"), ShowAlert: true})
			return ext.EndGroups
		}
		text, _ := tr.GetString("feds_deleted")
		if query.Message != nil {
			_, _, _ = query.Message.EditText(b, &gotgbot.EditMessageTextOpts{Text: text, ParseMode: formatting.HTML})
		}
		_, _ = query.Answer(b, nil)
		return ext.EndGroups
	default:
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: trS(tr, "feds_callback_stale")})
		return ext.EndGroups
	}
}

func (moduleStruct) enforceFedBan(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	if chat == nil || chat_status.RequirePrivate(b, ctx, chat) || msg == nil {
		return ext.ContinueGroups
	}
	membership := federations.GetChatFedContext(tracing.UpdateContext(ctx), chat.Id)
	if membership == nil {
		return ext.ContinueGroups
	}

	if len(msg.NewChatMembers) > 0 {
		for _, member := range msg.NewChatMembers {
			fedBanMember(b, ctx, chat, membership, member.Id, member.FirstName, true)
		}
		return ext.ContinueGroups
	}
	sender := ctx.EffectiveSender
	if sender == nil || sender.IsAnonymousChannel() || !sender.IsUser() {
		return ext.ContinueGroups
	}
	if ctx.EffectiveUser != nil && ctx.EffectiveUser.IsBot {
		return ext.ContinueGroups
	}
	fedBanMember(b, ctx, chat, membership, sender.Id(), sender.Name(), false)
	return ext.ContinueGroups
}

// enforceFedBanOnJoin bans a fbanned newcomer from the ChatMemberUpdated of
// the join, which large groups send without a service message.
func (moduleStruct) enforceFedBanOnJoin(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	if chat == nil || ctx.ChatMember == nil || chat_status.RequirePrivate(b, ctx, chat) {
		return ext.ContinueGroups
	}
	membership := federations.GetChatFedContext(tracing.UpdateContext(ctx), chat.Id)
	if membership == nil {
		return ext.ContinueGroups
	}
	member := ctx.ChatMember.NewChatMember.GetUser()
	fedBanMember(b, ctx, chat, membership, member.Id, member.FirstName, true)
	return ext.ContinueGroups
}

// fedBanMember bans userID when the chat's federation tree has a ban for them
// and posts the notice unless the federation is quiet.
//
// A join reaches the bot as a service message and a ChatMemberUpdated, in
// either order and possibly at once. For a join the ban first takes the join
// claim greetings uses, so only one of the two updates bans and posts, and the
// claim keeps the banned newcomer from getting a captcha or welcome. A failed
// ban gives the claim back, so greetings still challenges the newcomer. While
// Redis is down the claim is this replica's, so the ban still happens.
func fedBanMember(b *gotgbot.Bot, ctx *ext.Context, chat *gotgbot.Chat, membership *models.FederationChat, userID int64, name string, join bool) {
	if !chat_status.IsValidUserId(userID) || userID == b.Id {
		return
	}
	ban, sourceFed := federations.FindBanInFedTree(membership.FedID, userID)
	if ban == nil {
		return
	}
	if chat_status.IsUserAdminForUpdate(b, ctx, chat.Id, userID) {
		return
	}
	if join && !claimRecentJoinForBan(chat.Id, userID) {
		return
	}
	// Antiraid leaves fbanned newcomers to this ban, so wait out a short flood.
	if err := retryAfterFloodWithin(fedBanOnUpdateMaxFloodWait, func() error {
		_, err := punishment.Apply(tracing.UpdateContext(ctx), b, punishment.Request{
			ChatID: chat.Id, Target: punishment.Target{ID: userID}, Mode: punishment.Ban,
			Source: models.BanSourceFed, Detail: sourceFed, ActorID: ban.BannedBy,
		})
		return err
	}); err != nil {
		log.Debugf("[Federations] passive fban: %v", err)
		if join {
			clearRecentJoinProcessing(chat.Id, userID)
		}
		return
	}
	if join {
		// The join claim is this ban's, so the intake never sees the join:
		// count it towards the raid threshold here.
		antiRaidModule.countJoin(b, ctx, userID)
	} else {
		// The sender may have joined unseen; keep greetings from challenging
		// them if that join's update comes later.
		claimRecentJoinProcessing(chat.Id, userID)
	}
	if membership.Quiet {
		return
	}
	tr := ctxTr(ctx)
	fedName := sourceFed
	if fed := federations.GetFedContext(tracing.UpdateContext(ctx), sourceFed); fed != nil {
		fedName = fed.Name
	}
	text, _ := tr.GetString("feds_passive_ban", i18n.TranslationParams{
		"user":   formatting.MentionHtml(userID, name),
		"name":   html.EscapeString(fedName),
		"reason": html.EscapeString(ban.Reason),
	})
	_, _ = b.SendMessage(chat.Id, text, &gotgbot.SendMessageOpts{ParseMode: formatting.HTML})
}

// Federation checks retain their resolved input on this update so the domain
// handler uses exactly the federation authorised for the effective actor.
func federationPrivateCheck(c *helpers.CommandContext) bool {
	if chat_status.RequirePrivate(c.Bot, c.Ctx, c.Chat) {
		return true
	}
	replyHTML(c.Bot, c.Msg, trS(c.Tr, "feds_pm_only"))
	return false
}

func federationGroupCheck(c *helpers.CommandContext) bool {
	if !chat_status.RequirePrivate(c.Bot, c.Ctx, c.Chat) {
		return true
	}
	replyHTML(c.Bot, c.Msg, trS(c.Tr, "feds_group_only"))
	return false
}

func federationChatOwnerCheck(c *helpers.CommandContext) bool {
	if chat_status.RequireUserOwner(c.Bot, c.Ctx, c.Chat, c.User.Id) {
		return true
	}
	replyHTML(c.Bot, c.Msg, trS(c.Tr, "feds_owner_only_join"))
	return false
}

func federationOwnerCheck(c *helpers.CommandContext) bool {
	fed := federations.GetFedByOwner(c.User.Id)
	if fed == nil {
		replyHTML(c.Bot, c.Msg, trS(c.Tr, "feds_no_fed"))
		return false
	}
	c.Ctx.Data["federationCommandFed"] = fed
	return true
}

func federationChatCheck(c *helpers.CommandContext) bool {
	membership := federations.GetChatFedContext(tracing.UpdateContext(c.Ctx), c.Chat.Id)
	if membership == nil {
		replyHTML(c.Bot, c.Msg, trS(c.Tr, "feds_not_in_fed"))
		return false
	}
	c.Ctx.Data["federationCommandChat"] = membership
	return true
}

func federationBanCheck(c *helpers.CommandContext) bool {
	if chat_status.RequirePrivate(c.Bot, c.Ctx, c.Chat) {
		return federationOwnerCheck(c)
	}
	if !federationChatCheck(c) {
		return false
	}
	membership := c.Ctx.Data["federationCommandChat"].(*models.FederationChat)
	if !federations.IsFedAdmin(membership.FedID, c.User.Id) {
		replyHTML(c.Bot, c.Msg, trS(c.Tr, "feds_not_admin"))
		return false
	}
	fed := federations.GetFedContext(tracing.UpdateContext(c.Ctx), membership.FedID)
	if fed == nil {
		return false
	}
	c.Ctx.Data["federationCommandFed"] = fed
	return true
}

func LoadFederations(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(federationsModule.moduleName, true)

	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "newfed", RequiredChecks: []helpers.CheckFunc{federationPrivateCheck},
	}, pipelineHandler(federationsModule.newFed))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "renamefed", RequiredChecks: []helpers.CheckFunc{helpers.RequireIdentifiedUser()},
	}, pipelineHandler(federationsModule.renameFed))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "delfed", RequiredChecks: []helpers.CheckFunc{federationPrivateCheck, federationOwnerCheck},
	}, pipelineHandler(federationsModule.delFed))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "joinfed", RequiredChecks: []helpers.CheckFunc{federationGroupCheck, federationChatOwnerCheck},
	}, pipelineHandler(federationsModule.joinFed))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "leavefed", RequiredChecks: []helpers.CheckFunc{federationGroupCheck, federationChatOwnerCheck},
	}, pipelineHandler(federationsModule.leaveFed))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "quietfed", RequiredChecks: append([]helpers.CheckFunc{federationGroupCheck}, declaredGroupSettingChecks(chatsettings.QuietFed)...),
	}, pipelineHandler(federationsModule.quietFed))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "fedinfo", Disableable: true, RequiredChecks: []helpers.CheckFunc{helpers.RequireIdentifiedUser()},
	}, pipelineHandler(federationsModule.fedInfo))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "fedadmins", Disableable: true, RequiredChecks: []helpers.CheckFunc{helpers.RequireIdentifiedUser()},
	}, pipelineHandler(federationsModule.fedAdmins))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "chatfed", Disableable: true, RequiredChecks: []helpers.CheckFunc{federationGroupCheck, federationChatCheck},
	}, pipelineHandler(federationsModule.chatFed))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "myfeds", RequiredChecks: []helpers.CheckFunc{helpers.RequireIdentifiedUser()},
	}, pipelineHandler(federationsModule.myFeds))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "fedpromote", RequiredChecks: []helpers.CheckFunc{federationOwnerCheck},
	}, pipelineHandler(federationsModule.fedPromote))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "feddemote", RequiredChecks: []helpers.CheckFunc{federationOwnerCheck},
	}, pipelineHandler(federationsModule.fedDemote))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "feddemoteme", RequiredChecks: []helpers.CheckFunc{helpers.RequireIdentifiedUser()},
	}, pipelineHandler(federationsModule.fedDemoteMe))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "fedreason", RequiredChecks: []helpers.CheckFunc{federationOwnerCheck},
	}, pipelineHandler(federationsModule.fedReason))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "fednotif", RequiredChecks: []helpers.CheckFunc{federationOwnerCheck},
	}, pipelineHandler(federationsModule.fedNotif))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "fban", RequiredChecks: []helpers.CheckFunc{federationBanCheck},
	}, pipelineHandler(federationsModule.fban))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "unfban", Aliases: []string{"funban"}, RequiredChecks: []helpers.CheckFunc{federationBanCheck},
	}, pipelineHandler(federationsModule.unfban))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "fedstat", Aliases: []string{"fbanstat"}, Disableable: true,
		RequiredChecks: []helpers.CheckFunc{helpers.RequireIdentifiedUser()},
	}, pipelineHandler(federationsModule.fedStat))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "subfed", RequiredChecks: []helpers.CheckFunc{federationOwnerCheck},
	}, pipelineHandler(federationsModule.subFed))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "unsubfed", RequiredChecks: []helpers.CheckFunc{federationOwnerCheck},
	}, pipelineHandler(federationsModule.unsubFed))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "fedsubs", Disableable: true, RequiredChecks: []helpers.CheckFunc{federationOwnerCheck},
	}, pipelineHandler(federationsModule.fedSubs))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "setfedlog", RequiredChecks: []helpers.CheckFunc{federationGroupCheck, federationOwnerCheck},
	}, pipelineHandler(federationsModule.setFedLog))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "unsetfedlog", RequiredChecks: []helpers.CheckFunc{federationOwnerCheck},
	}, pipelineHandler(federationsModule.unsetFedLog))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "fbanlist", RequiredChecks: []helpers.CheckFunc{federationOwnerCheck},
	}, pipelineHandler(federationsModule.fbanList))
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{
		Name: "importfbans", RequiredChecks: []helpers.CheckFunc{federationOwnerCheck},
	}, pipelineHandler(federationsModule.importFBans))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(fedCallbackNamespace+"|"), federationsModule.fedCallback))

	dispatcher.AddHandlerToGroup(
		handlers.NewMessage(message.All, federationsModule.enforceFedBan),
		federationsModule.handlerGroup,
	)
	dispatcher.AddHandlerToGroup(
		handlers.NewChatMember(
			func(u *gotgbot.ChatMemberUpdated) bool {
				wasMember, isMember := chat_status.ExtractJoinLeftStatusChange(u)
				return !wasMember && isMember
			},
			federationsModule.enforceFedBanOnJoin,
		),
		federationsModule.handlerGroup,
	)
}

func init() {
	RegisterLegacyModule("Federations", 235, LoadFederations)
}

// fedBannedInChat reports whether the chat's federation tree has a ban for
// userID, which fed-ban enforces on join before the other join guards run.
func fedBannedInChat(ctx *ext.Context, chatID, userID int64) bool {
	membership := federations.GetChatFedContext(tracing.UpdateContext(ctx), chatID)
	if membership == nil {
		return false
	}
	ban, _ := federations.FindBanInFedTree(membership.FedID, userID)
	return ban != nil
}
