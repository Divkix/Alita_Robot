package modules

import (
	"fmt"
	"html"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	log "github.com/sirupsen/logrus"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/locks"
	"github.com/divkix/Alita_Robot/alita/utils/actionlog"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/updatememo"

	"github.com/divkix/Alita_Robot/alita/i18n"
)

var (
	locksModule = moduleStruct{
		moduleName:        "Locks",
		permHandlerGroup:  5,
		restrHandlerGroup: 6,
	}
	arabmatch, _                 = regexp.Compile("[\u0600-\u06FF]")
	OTHER        filters.Message = func(msg *gotgbot.Message) bool {
		return msg.Game != nil || msg.Sticker != nil || message.Animation(msg)
	}
	MEDIA filters.Message = func(msg *gotgbot.Message) bool {
		return msg.Audio != nil || msg.Document != nil || msg.VideoNote != nil || msg.Video != nil || msg.Voice != nil || msg.Photo != nil
	}
	MESSAGES filters.Message = func(msg *gotgbot.Message) bool {
		return msg.Text != "" || msg.Contact != nil || msg.Location != nil || msg.Venue != nil || MEDIA(msg) || OTHER(msg)
	}
	hasURLEntity = func(msg *gotgbot.Message) bool {
		for _, entity := range msg.Entities {
			if entity.Type == "url" || entity.Type == "text_link" || entity.Url != "" {
				return true
			}
		}
		for _, entity := range msg.CaptionEntities {
			if entity.Type == "url" || entity.Type == "text_link" || entity.Url != "" {
				return true
			}
		}
		return false
	}
	PREVIEW filters.Message = hasURLEntity

	lockMap = map[string]filters.Message{
		"sticker": message.Sticker,
		"audio":   message.Audio,
		"voice":   message.Voice,
		"document": func(msg *gotgbot.Message) bool {
			return msg.Document != nil && msg.Animation == nil
		},
		"video":     message.Video,
		"videonote": message.VideoNote,
		"contact":   message.Contact,
		"photo":     message.Photo,
		"gif":       message.Animation,
		"url":       hasURLEntity,
		"bots":      message.NewChatMembers,
		"forward":   message.Forwarded,
		"game":      message.Game,
		"location":  message.Location,
		"rtl": func(msg *gotgbot.Message) bool {
			return arabmatch.MatchString(msg.Text) || arabmatch.MatchString(msg.Caption)
		},
		"anonchannel": func(msg *gotgbot.Message) bool {
			sender := msg.GetSender()
			return sender.IsAnonymousChannel() || sender.IsLinkedChannel()
		},
	}

	restrMap = map[string]filters.Message{
		"messages": MESSAGES,
		"comments": MESSAGES,
		"media":    MEDIA,
		"other":    OTHER,
		"previews": PREVIEW,
		"all":      message.All,
	}

	cachedLockTypes     []string
	cachedLockTypesOnce sync.Once
)

func (moduleStruct) getLockMapAsArray() []string {
	cachedLockTypesOnce.Do(func() {
		tmpMap := make(map[string]filters.Message, len(lockMap)+len(restrMap))

		for r, rk := range restrMap {
			tmpMap[r] = rk
		}
		for l, lk := range lockMap {
			tmpMap[l] = lk
		}

		lockTypes := make([]string, 0, len(tmpMap))
		for k := range tmpMap {
			lockTypes = append(lockTypes, k)
		}
		slices.Sort(lockTypes)
		cachedLockTypes = lockTypes
	})

	return cachedLockTypes
}

func (moduleStruct) buildLockTypesMessage(ctx *ext.Context, chatID int64) (res string) {
	chatLocks := locks.GetChatLocks(chatID)

	newMapLocks := chatLocks
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	res, _ = tr.GetString("locks_current_locks_header")

	keys := make([]string, 0, len(newMapLocks))
	for k := range newMapLocks {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var sb strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&sb, "\n - %s = %v", k, newMapLocks[k])
	}
	res += sb.String()

	return
}

func (m moduleStruct) locktypes(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	if chat_status.CheckDisabledCmd(b, msg, "locktypes") {
		return ext.EndGroups
	}
	if resolveMemberChat(b, ctx, true) == nil {
		return ext.EndGroups
	}
	_locktypes := m.getLockMapAsArray()

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	header, _ := tr.GetString("locks_locktypes_header")
	_, err := msg.Reply(b, header+strings.Join(_locktypes, "\n - "), formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

func (m moduleStruct) locks(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	if chat_status.CheckDisabledCmd(b, msg, "locks") {
		return ext.EndGroups
	}
	chat := resolveAdminChat(b, ctx, true)
	if chat == nil {
		return ext.EndGroups
	}

	_, err := msg.Reply(b, m.buildLockTypesMessage(ctx, chat.Id), formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// setLocksKeys are the replies of /lock or /unlock.
type setLocksKeys struct {
	whatKey, failedKey, doneKey, verb string
}

var (
	lockKeys   = setLocksKeys{"locks_what_to_lock", "locks_lock_failed", "locks_locked_successfully", "lock"}
	unlockKeys = setLocksKeys{"locks_what_to_unlock", "locks_unlock_failed", "locks_unlocked_successfully", "unlock"}
)

func (m moduleStruct) lockPerm(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.setLocks(b, ctx, true, lockKeys)
}

func (m moduleStruct) unlockPerm(b *gotgbot.Bot, ctx *ext.Context) error {
	return m.setLocks(b, ctx, false, unlockKeys)
}

func (m moduleStruct) setLocks(b *gotgbot.Bot, ctx *ext.Context, lock bool, keys setLocksKeys) error {
	msg := ctx.EffectiveMessage
	connectedChat := resolveAdminChat(b, ctx, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	chat := ctx.EffectiveChat
	args := ctx.Args()[1:]

	sender := ctx.EffectiveSender
	if sender == nil {
		return ext.EndGroups
	}

	if !chat_status.RequireBotAdmin(b, ctx, nil) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_not_admin", "", chat_status.WithReply())
		return ext.EndGroups
	}
	if !chat_status.RequireUserAdmin(b, ctx, nil, sender.Id()) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_user_admin_cmd_error", "chat_status_user_admin_button_error", chat_status.WithReplyFallback())
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	reply := func(text string) error {
		if _, err := msg.Reply(b, text, formatting.Shtml()); err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	if len(args) == 0 {
		text, _ := tr.GetString(keys.whatKey)
		return reply(text)
	}

	lockTypes := m.getLockMapAsArray()
	for _, perm := range args {
		if !slices.Contains(lockTypes, perm) {
			temp, _ := tr.GetString("locks_invalid_lock_type")
			return reply(fmt.Sprintf(temp, html.EscapeString(perm)))
		}
	}

	failedLocks := make([]string, 0, len(args))
	for _, perm := range args {
		if err := locks.UpdateLock(chat.Id, perm, lock); err != nil {
			log.Warnf("[Locks] Failed to %s %s in chat %d: %v", keys.verb, perm, chat.Id, err)
			failedLocks = append(failedLocks, perm)
		}
	}

	if len(failedLocks) > 0 {
		text, _ := tr.GetString(keys.failedKey)
		return reply(fmt.Sprintf(text, strings.Join(failedLocks, ", ")))
	}
	temp, _ := tr.GetString(keys.doneKey)
	return reply(fmt.Sprintf(temp, strings.Join(args, "\n - ")))
}

type chatLocksMemoKey struct{ chatID int64 }

// chatLocksForUpdate is locks.GetChatLocks memoised for the current update, so
// permHandler and restHandler share one lookup. The returned map is shared
// between them and must be treated as read-only.
func chatLocksForUpdate(ctx *ext.Context, chatID int64) map[string]bool {
	return updatememo.Get(updatememo.From(ctx), chatLocksMemoKey{chatID}, func() map[string]bool {
		return locks.GetChatLocks(chatID)
	})
}

// anyLockHit reports whether msg trips any lock that is enabled in chatLocks.
// It only inspects the message, so it never calls Telegram or the database.
// The lock named skip is ignored.
func anyLockHit(chatLocks map[string]bool, lockFilters map[string]filters.Message, msg *gotgbot.Message, skip string) bool {
	for key, filter := range lockFilters {
		if key != skip && chatLocks[key] && filter(msg) {
			return true
		}
	}
	return false
}

func (moduleStruct) restHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	sender := ctx.EffectiveSender

	if sender == nil {
		return ext.ContinueGroups
	}

	chatLocks := chatLocksForUpdate(ctx, chat.Id)
	// Pure in-memory check first so unlocked content never pays for the
	// admin/approval/bot-permission lookups below.
	if !anyLockHit(chatLocks, restrMap, msg, "") {
		return ext.ContinueGroups
	}

	// Anonymous admins post as the group itself (sender.Id() is the chat ID),
	// which IsUserAdmin rejects; they are admins and exempt from locks.
	if sender.IsAnonymousAdmin() {
		return ext.ContinueGroups
	}

	senderID := sender.Id()

	if chat_status.IsUserAdminForUpdate(b, ctx, chat.Id, senderID) {
		return ext.ContinueGroups
	}
	if senderID > 0 && chat_status.IsApprovedForUpdate(b, ctx, chat.Id, senderID) {
		return ext.ContinueGroups
	}

	if !chat_status.CanBotDeleteForUpdate(b, ctx, nil) {
		return ext.ContinueGroups
	}

	for restr, filter := range restrMap {
		if !filter(msg) || !chatLocks[restr] {
			continue
		}

		// but skip Telegram's system account, which forwards channel posts
		if restr == "comments" {
			if msg.From != nil && msg.From.Id == chat_status.TelegramUserID {
				continue
			}
			if chat_status.IsUserInChat(b, chat, senderID) {
				continue
			}
		}

		_ = helpers.DeleteMessageWithErrorHandling(b, chat.Id, msg.MessageId)
		break
	}

	return ext.ContinueGroups
}

func (moduleStruct) permHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	sender := ctx.EffectiveSender

	if sender == nil {
		return ext.ContinueGroups
	}

	chatLocks := chatLocksForUpdate(ctx, chat.Id)
	// Pure in-memory check first so unlocked content never pays for the
	// admin/approval/bot-permission lookups below.
	if !anyLockHit(chatLocks, lockMap, msg, "bots") {
		return ext.ContinueGroups
	}

	// Anonymous admins post as the group itself (sender.Id() is the chat ID),
	// which IsUserAdmin rejects; they are admins and exempt from locks.
	if sender.IsAnonymousAdmin() {
		return ext.ContinueGroups
	}

	senderID := sender.Id()

	if chat_status.IsUserAdminForUpdate(b, ctx, chat.Id, senderID) {
		return ext.ContinueGroups
	}
	if senderID > 0 && chat_status.IsApprovedForUpdate(b, ctx, chat.Id, senderID) {
		return ext.ContinueGroups
	}

	if !chat_status.CanBotDeleteForUpdate(b, ctx, nil) {
		return ext.ContinueGroups
	}

	for perm, filter := range lockMap {
		if !filter(msg) || !chatLocks[perm] {
			continue
		}

		if perm == "bots" {
			continue
		}

		_ = helpers.DeleteMessageWithErrorHandling(b, chat.Id, msg.MessageId)
		break
	}

	return ext.ContinueGroups
}

func (moduleStruct) botLockHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	sender := ctx.EffectiveSender
	mem := ctx.ChatMember.NewChatMember.MergeChatMember().User

	if !locks.IsPermLocked(chat.Id, "bots") {
		return ext.ContinueGroups
	}

	var senderID int64
	if sender != nil {
		senderID = sender.Id()
	}

	if senderID > 0 && chat_status.IsUserAdmin(b, chat.Id, senderID) {
		return ext.ContinueGroups
	}
	if senderID > 0 && chat_status.IsApproved(b, chat.Id, senderID) {
		return ext.ContinueGroups
	}

	if !chat_status.IsBotAdmin(b, ctx, nil) {
		tr := i18n.MustNewTranslator(lang.GetLanguage(&ext.Context{EffectiveChat: chat}))
		text, _ := tr.GetString("locks_bot_lock_no_permission")
		_, err := b.SendMessage(chat.Id, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.ContinueGroups
	}
	if !chat_status.CanBotRestrict(b, ctx, nil) {
		// A ChatMember update has no message to reply to, so send to the chat.
		tr := i18n.MustNewTranslator(lang.GetLanguage(&ext.Context{EffectiveChat: chat}))
		text, _ := tr.GetString("locks_bot_lock_no_ban_permission")
		if _, err := b.SendMessage(chat.Id, text, formatting.Shtml()); err != nil {
			log.Error(err)
			return err
		}
		return ext.ContinueGroups
	}

	_, err := chat.BanMember(b, mem.Id, nil)
	if err != nil {
		log.Error(err)
		return err
	}
	var addedBy string
	if sender != nil {
		addedBy = fmt.Sprintf("%s (%d)", sender.Name(), senderID)
	}
	actionlog.Automated(b, chat, "LOCK_BOTS", mem.Id, mem.FirstName, actionlog.Field{Name: "Added by", Value: addedBy})

	tr := i18n.MustNewTranslator(lang.GetLanguage(&ext.Context{EffectiveChat: chat}))
	text, _ := tr.GetString("locks_bot_only_admins")
	_, err = b.SendMessage(chat.Id, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.ContinueGroups
}

func LoadLocks(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(locksModule.moduleName, true)

	dispatcher.AddHandler(handlers.NewCommand("lock", withPMGroupArg("lock", helpers.PMGroupArgOwn, locksModule.lockPerm)))
	dispatcher.AddHandler(handlers.NewCommand("unlock", withPMGroupArg("unlock", helpers.PMGroupArgOwn, locksModule.unlockPerm)))
	dispatcher.AddHandler(handlers.NewCommand("locktypes", withPMGroupArg("locktypes", helpers.PMGroupArgNone, locksModule.locktypes)))
	helpers.AddCmdToDisableable("locktypes")
	dispatcher.AddHandler(handlers.NewCommand("locks", withPMGroupArg("locks", helpers.PMGroupArgNone, locksModule.locks)))
	helpers.AddCmdToDisableable("locks")
	dispatcher.AddHandlerToGroup(handlers.NewMessage(message.All, locksModule.permHandler), locksModule.permHandlerGroup)
	dispatcher.AddHandlerToGroup(handlers.NewMessage(message.All, locksModule.restHandler), locksModule.restrHandlerGroup)
	dispatcher.AddHandler(
		handlers.NewChatMember(
			func(u *gotgbot.ChatMemberUpdated) bool {
				mem := u.NewChatMember.MergeChatMember()
				oldMem := u.OldChatMember.MergeChatMember()
				return mem.User.IsBot && mem.Status == "member" && oldMem.Status == "left"
			},
			locksModule.botLockHandler,
		),
	)
}

func init() {
	RegisterLegacyModule("Locks", 130, LoadLocks)
}
