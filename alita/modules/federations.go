package modules

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"
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
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

const (
	fedHandlerGroup      = -6
	fedExportCooldown    = 30 * time.Minute
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
	chat := ctx.EffectiveChat
	from := requireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	if !chat_status.RequirePrivate(b, ctx, chat) {
		text, _ := tr.GetString("feds_pm_only")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
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
	from := requireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	name := strings.TrimSpace(strings.Join(ctx.Args()[1:], " "))
	if name == "" {
		text, _ := tr.GetString("feds_need_name")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	fed, err := federations.RenameFederation(from.Id, name)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
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
	chat := ctx.EffectiveChat
	from := requireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	if !chat_status.RequirePrivate(b, ctx, chat) {
		text, _ := tr.GetString("feds_pm_only")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	fed := federations.GetFedByOwner(from.Id)
	if fed == nil {
		text, _ := tr.GetString("feds_no_fed")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
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
	from := requireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	if chat_status.RequirePrivate(b, ctx, chat) {
		text, _ := tr.GetString("feds_group_only")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if !chat_status.RequireUserOwner(b, ctx, chat, from.Id) {
		text, _ := tr.GetString("feds_owner_only_join")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	fedID, ok := parseFedID(strings.Join(ctx.Args()[1:], " "))
	if !ok {
		text, _ := tr.GetString("feds_invalid_id")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	fed := federations.GetFed(fedID)
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
	from := requireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	if chat_status.RequirePrivate(b, ctx, chat) {
		text, _ := tr.GetString("feds_group_only")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if !chat_status.RequireUserOwner(b, ctx, chat, from.Id) {
		text, _ := tr.GetString("feds_owner_only_join")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
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
	from := requireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	if chat_status.RequirePrivate(b, ctx, chat) {
		text, _ := tr.GetString("feds_group_only")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, chat, from.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}
	if federations.GetChatFed(chat.Id) == nil {
		text, _ := tr.GetString("feds_not_in_fed")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
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
	if err := federations.SetQuietFed(chat.Id, enabled); err != nil {
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
	text, _ := tr.GetString("feds_info", i18n.TranslationParams{
		"name":   html.EscapeString(fed.Name),
		"id":     fed.FedID,
		"owner":  strconv.FormatInt(fed.OwnerID, 10),
		"chats":  federations.CountFedChats(fed.FedID),
		"bans":   federations.CountFedBans(fed.FedID),
		"admins": len(federations.ListFedAdmins(fed.FedID)),
		"subs":   len(federations.ListSubscribedFedIDs(fed.FedID)),
		"reason": strconv.FormatBool(fed.RequireReason),
		"notify": strconv.FormatBool(fed.NotifyOwner),
	})
	return text
}

func (moduleStruct) fedInfo(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	from := requireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	if chat_status.CheckDisabledCmd(b, msg, "fedinfo") {
		return ext.EndGroups
	}
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
		fed = federations.GetFed(fedID)
	} else if !chat_status.RequirePrivate(b, ctx, chat) {
		if membership := federations.GetChatFed(chat.Id); membership != nil {
			fed = federations.GetFed(membership.FedID)
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
	from := requireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	if chat_status.CheckDisabledCmd(b, msg, "fedadmins") {
		return ext.EndGroups
	}
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
		fed = federations.GetFed(fedID)
	} else {
		fed = federations.GetFedByOwner(from.Id)
		if fed == nil {
			if membership := federations.GetChatFed(ctx.EffectiveChat.Id); membership != nil {
				fed = federations.GetFed(membership.FedID)
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
	fmt.Fprintf(&bld, "• %s (<code>%d</code>)", formatting.MentionHtml(fed.OwnerID, "owner"), fed.OwnerID)
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
	chat := ctx.EffectiveChat
	if requireUser(b, ctx) == nil {
		return ext.EndGroups
	}
	if chat_status.CheckDisabledCmd(b, msg, "chatfed") {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	if chat_status.RequirePrivate(b, ctx, chat) {
		text, _ := tr.GetString("feds_group_only")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	membership := federations.GetChatFed(chat.Id)
	if membership == nil {
		text, _ := tr.GetString("feds_not_in_fed")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	fed := federations.GetFed(membership.FedID)
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
	from := requireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
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
		role := "admin"
		if fed.OwnerID == from.Id {
			role = "owner"
		}
		fmt.Fprintf(&bld, "\n• <b>%s</b> (<code>%s</code>) — %s", html.EscapeString(fed.Name), fed.FedID, role)
	}
	replyHTML(b, msg, bld.String())
	return ext.EndGroups
}

func ownedFedOrReply(b *gotgbot.Bot, ctx *ext.Context) *models.Federation {
	from := ctx.EffectiveUser
	tr := ctxTr(ctx)
	fed := federations.GetFedByOwner(from.Id)
	if fed == nil {
		text, _ := tr.GetString("feds_no_fed")
		replyHTML(b, ctx.EffectiveMessage, text)
	}
	return fed
}

func (m moduleStruct) fedPromote(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.changeFedAdmin(b, ctx, true)
}

func (m moduleStruct) fedDemote(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.changeFedAdmin(b, ctx, false)
}

func (moduleStruct) changeFedAdmin(b *gotgbot.Bot, ctx *ext.Context, promote bool) error {
	msg := ctx.EffectiveMessage
	from := requireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	fed := ownedFedOrReply(b, ctx)
	if fed == nil {
		return ext.EndGroups
	}
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
	from := requireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	fedID, ok := parseFedID(strings.Join(ctx.Args()[1:], " "))
	if !ok {
		text, _ := tr.GetString("feds_invalid_id")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if federations.IsFedOwner(fedID, from.Id) {
		text, _ := tr.GetString("feds_cannot_promote_owner")
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
	from := requireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	fed := ownedFedOrReply(b, ctx)
	if fed == nil {
		return ext.EndGroups
	}
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

func resolveFbanFed(b *gotgbot.Bot, ctx *ext.Context, from *gotgbot.User) *models.Federation {
	chat := ctx.EffectiveChat
	tr := ctxTr(ctx)
	if chat_status.RequirePrivate(b, ctx, chat) {
		fed := federations.GetFedByOwner(from.Id)
		if fed == nil {
			text, _ := tr.GetString("feds_no_fed")
			replyHTML(b, ctx.EffectiveMessage, text)
		}
		return fed
	}
	membership := federations.GetChatFed(chat.Id)
	if membership == nil {
		text, _ := tr.GetString("feds_not_in_fed")
		replyHTML(b, ctx.EffectiveMessage, text)
		return nil
	}
	if !federations.IsFedAdmin(membership.FedID, from.Id) {
		text, _ := tr.GetString("feds_not_admin")
		replyHTML(b, ctx.EffectiveMessage, text)
		return nil
	}
	return federations.GetFed(membership.FedID)
}

func (m moduleStruct) fban(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.applyFban(b, ctx, true)
}

func (m moduleStruct) unfban(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.applyFban(b, ctx, false)
}

func (moduleStruct) applyFban(b *gotgbot.Bot, ctx *ext.Context, ban bool) error {
	msg := ctx.EffectiveMessage
	from := requireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	fed := resolveFbanFed(b, ctx, from)
	if fed == nil {
		return ext.EndGroups
	}
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
		notifyFedAction(b, fed, from, fmt.Sprintf(
			"FBAN <code>%d</code> by %s\nReason: %s",
			targetID,
			formatting.MentionHtml(from.Id, from.FirstName),
			html.EscapeString(reason),
		))
		go applyActiveFban(b, fed.FedID, targetID)
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
	notifyFedAction(b, fed, from, fmt.Sprintf(
		"UNFBAN <code>%d</code> by %s",
		targetID,
		formatting.MentionHtml(from.Id, from.FirstName),
	))
	return ext.EndGroups
}

func applyActiveFban(b *gotgbot.Bot, fedID string, userID int64) {
	defer error_handling.RecoverFromPanic("applyActiveFban", "Federations")
	chatIDs, err := federations.ListFedChatIDs(fedID)
	if err != nil {
		return
	}
	for _, chatID := range federations.ChatsContainingUser(userID, chatIDs) {
		if _, err := b.BanChatMember(chatID, userID, nil); err != nil {
			log.Debugf("[Federations] active fban chat %d user %d: %v", chatID, userID, err)
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
	from := requireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	if chat_status.CheckDisabledCmd(b, msg, "fedstat") {
		return ext.EndGroups
	}
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
		ban := federations.GetFedBan(reasonFed, targetID)
		if ban == nil {
			text, _ := tr.GetString("feds_fedstat_none")
			replyHTML(b, msg, text)
			return ext.EndGroups
		}
		fed := federations.GetFed(reasonFed)
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
		if fed := federations.GetFed(ban.FedID); fed != nil {
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
	from := requireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	fed := ownedFedOrReply(b, ctx)
	if fed == nil {
		return ext.EndGroups
	}
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
	from := requireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	if chat_status.CheckDisabledCmd(b, msg, "fedsubs") {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	fed := ownedFedOrReply(b, ctx)
	if fed == nil {
		return ext.EndGroups
	}
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
		if sub := federations.GetFed(id); sub != nil {
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
	from := requireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	arg := strings.TrimSpace(strings.Join(ctx.Args()[1:], " "))
	if chat.Type == "channel" {
		fedID, ok := parseFedID(arg)
		if !ok {
			text, _ := tr.GetString("feds_setfedlog_need_id")
			replyHTML(b, msg, text)
			return ext.EndGroups
		}
		fed := federations.GetFed(fedID)
		if fed == nil || fed.OwnerID != from.Id {
			text, _ := tr.GetString("feds_not_owner")
			replyHTML(b, msg, text)
			return ext.EndGroups
		}
		if err := federations.SetFedLogChat(fed.FedID, chat.Id); err != nil {
			text, _ := tr.GetString("feds_create_failed")
			replyHTML(b, msg, text)
			return ext.EndGroups
		}
		text, _ := tr.GetString("feds_setfedlog_ok")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	if chat_status.RequirePrivate(b, ctx, chat) {
		text, _ := tr.GetString("feds_group_only")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
	fed := ownedFedOrReply(b, ctx)
	if fed == nil {
		return ext.EndGroups
	}
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
	from := requireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	fed := ownedFedOrReply(b, ctx)
	if fed == nil {
		return ext.EndGroups
	}
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
	ok, err := client.SetNX(cache.Context, "alita:fed_export:"+fedID, "1", fedExportCooldown).Result()
	if err != nil {
		return true
	}
	return ok
}

func (moduleStruct) fbanList(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	from := requireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	fed := ownedFedOrReply(b, ctx)
	if fed == nil {
		return ext.EndGroups
	}
	if !allowFedExport(fed.FedID) {
		text, _ := tr.GetString("feds_export_wait")
		replyHTML(b, msg, text)
		return ext.EndGroups
	}
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
	}
	return ext.EndGroups
}

func (moduleStruct) importFBans(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	from := requireUser(b, ctx)
	if from == nil {
		return ext.EndGroups
	}
	tr := ctxTr(ctx)
	fed := ownedFedOrReply(b, ctx)
	if fed == nil {
		return ext.EndGroups
	}
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
	membership := federations.GetChatFed(chat.Id)
	if membership == nil {
		return ext.ContinueGroups
	}

	check := func(userID int64, name string) {
		if !chat_status.IsValidUserId(userID) || userID == b.Id {
			return
		}
		ban, sourceFed := federations.FindBanInFedTree(membership.FedID, userID)
		if ban == nil {
			return
		}
		if chat_status.IsUserAdmin(b, chat.Id, userID) {
			return
		}
		if _, err := b.BanChatMember(chat.Id, userID, nil); err != nil {
			log.Debugf("[Federations] passive fban: %v", err)
			return
		}
		if membership.Quiet {
			return
		}
		tr := ctxTr(ctx)
		fedName := sourceFed
		if fed := federations.GetFed(sourceFed); fed != nil {
			fedName = fed.Name
		}
		text, _ := tr.GetString("feds_passive_ban", i18n.TranslationParams{
			"user":   formatting.MentionHtml(userID, name),
			"name":   html.EscapeString(fedName),
			"reason": html.EscapeString(ban.Reason),
		})
		_, _ = b.SendMessage(chat.Id, text, &gotgbot.SendMessageOpts{ParseMode: formatting.HTML})
	}

	if len(msg.NewChatMembers) > 0 {
		for _, member := range msg.NewChatMembers {
			check(member.Id, member.FirstName)
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
	check(sender.Id(), sender.Name())
	return ext.ContinueGroups
}

func LoadFederations(dispatcher *ext.Dispatcher) {
	DefaultHelpRegistry().AbleMap[federationsModule.moduleName] = true

	dispatcher.AddHandler(handlers.NewCommand("newfed", federationsModule.newFed))
	dispatcher.AddHandler(handlers.NewCommand("renamefed", federationsModule.renameFed))
	dispatcher.AddHandler(handlers.NewCommand("delfed", federationsModule.delFed))
	dispatcher.AddHandler(handlers.NewCommand("joinfed", federationsModule.joinFed))
	dispatcher.AddHandler(handlers.NewCommand("leavefed", federationsModule.leaveFed))
	dispatcher.AddHandler(handlers.NewCommand("quietfed", federationsModule.quietFed))
	dispatcher.AddHandler(handlers.NewCommand("fedinfo", federationsModule.fedInfo))
	helpers.AddCmdToDisableable("fedinfo")
	dispatcher.AddHandler(handlers.NewCommand("fedadmins", federationsModule.fedAdmins))
	helpers.AddCmdToDisableable("fedadmins")
	dispatcher.AddHandler(handlers.NewCommand("chatfed", federationsModule.chatFed))
	dispatcher.AddHandler(handlers.NewCommand("myfeds", federationsModule.myFeds))
	dispatcher.AddHandler(handlers.NewCommand("fedpromote", federationsModule.fedPromote))
	dispatcher.AddHandler(handlers.NewCommand("feddemote", federationsModule.fedDemote))
	dispatcher.AddHandler(handlers.NewCommand("feddemoteme", federationsModule.fedDemoteMe))
	dispatcher.AddHandler(handlers.NewCommand("fedreason", federationsModule.fedReason))
	dispatcher.AddHandler(handlers.NewCommand("fednotif", federationsModule.fedNotif))
	dispatcher.AddHandler(handlers.NewCommand("fban", federationsModule.fban))
	helpers.MultiCommand(dispatcher, []string{"unfban", "funban"}, federationsModule.unfban)
	dispatcher.AddHandler(handlers.NewCommand("fedstat", federationsModule.fedStat))
	helpers.AddCmdToDisableable("fedstat")
	dispatcher.AddHandler(handlers.NewCommand("fbanstat", federationsModule.fedStat))
	dispatcher.AddHandler(handlers.NewCommand("subfed", federationsModule.subFed))
	dispatcher.AddHandler(handlers.NewCommand("unsubfed", federationsModule.unsubFed))
	dispatcher.AddHandler(handlers.NewCommand("fedsubs", federationsModule.fedSubs))
	helpers.AddCmdToDisableable("fedsubs")
	dispatcher.AddHandler(handlers.NewCommand("setfedlog", federationsModule.setFedLog))
	dispatcher.AddHandler(handlers.NewCommand("unsetfedlog", federationsModule.unsetFedLog))
	dispatcher.AddHandler(handlers.NewCommand("fbanlist", federationsModule.fbanList))
	dispatcher.AddHandler(handlers.NewCommand("importfbans", federationsModule.importFBans))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(fedCallbackNamespace+"|"), federationsModule.fedCallback))

	dispatcher.AddHandlerToGroup(
		handlers.NewMessage(message.All, federationsModule.enforceFedBan),
		federationsModule.handlerGroup,
	)
}

func init() {
	RegisterLegacyModule("Federations", 235, LoadFederations)
}
