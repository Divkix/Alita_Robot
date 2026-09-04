package modules

import (
	"fmt"
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
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"

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
			return arabmatch.MatchString(msg.Text)
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

func (moduleStruct) buildLockTypesMessage(chatID int64) (res string) {
	chatLocks := locks.GetChatLocks(chatID)

	newMapLocks := chatLocks
	tr := i18n.MustNewTranslator(lang.GetLanguage(&ext.Context{EffectiveChat: &gotgbot.Chat{Id: chatID}}))
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
	connectedChat := chat_status.IsUserConnected(b, ctx, false, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
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
	chat := chat_status.IsUserConnected(b, ctx, true, true)
	if chat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = chat

	_, err := msg.Reply(b, m.buildLockTypesMessage(chat.Id), formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

//nolint:dupl // lockPerm has symmetric logic with unlockPerm
func (m moduleStruct) lockPerm(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	connectedChat := chat_status.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
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

	if len(args) == 0 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("locks_what_to_lock")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	toLock := make([]string, 0, len(args))
	for _, perm := range args {
		if !slices.Contains(m.getLockMapAsArray(), perm) {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			temp, _ := tr.GetString("locks_invalid_lock_type")
			text := fmt.Sprintf(temp, perm)
			_, err := msg.Reply(b, text, formatting.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
			return ext.EndGroups
		}
		toLock = append(toLock, perm)
	}

	failedLocks := make([]string, 0, len(toLock))
	for _, perm := range toLock {
		if err := locks.UpdateLock(chat.Id, perm, true); err != nil {
			log.Warnf("[Locks] Failed to lock %s in chat %d: %v", perm, chat.Id, err)
			failedLocks = append(failedLocks, perm)
		}
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if len(failedLocks) > 0 {
		text, _ := tr.GetString("locks_lock_failed")
		_, err := msg.Reply(b, fmt.Sprintf(text, strings.Join(failedLocks, ", ")), formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	} else {
		temp, _ := tr.GetString("locks_locked_successfully")
		text := fmt.Sprintf(temp, strings.Join(toLock, "\n - "))
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return ext.EndGroups
}

//nolint:dupl // unlockPerm has symmetric logic with lockPerm
func (m moduleStruct) unlockPerm(b *gotgbot.Bot, ctx *ext.Context) error {
	msg := ctx.EffectiveMessage
	connectedChat := chat_status.IsUserConnected(b, ctx, true, true)
	if connectedChat == nil {
		return ext.EndGroups
	}
	ctx.EffectiveChat = connectedChat
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

	if len(args) == 0 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("locks_what_to_unlock")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	toUnlock := make([]string, 0, len(args))
	for _, perm := range args {
		if !slices.Contains(m.getLockMapAsArray(), perm) {
			tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
			temp, _ := tr.GetString("locks_invalid_lock_type")
			text := fmt.Sprintf(temp, perm)
			_, err := msg.Reply(b, text, formatting.Shtml())
			if err != nil {
				log.Error(err)
				return err
			}
			return ext.EndGroups
		}
		toUnlock = append(toUnlock, perm)
	}

	failedLocks := make([]string, 0, len(toUnlock))
	for _, perm := range toUnlock {
		if err := locks.UpdateLock(chat.Id, perm, false); err != nil {
			log.Warnf("[Locks] Failed to unlock %s in chat %d: %v", perm, chat.Id, err)
			failedLocks = append(failedLocks, perm)
		}
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if len(failedLocks) > 0 {
		text, _ := tr.GetString("locks_unlock_failed")
		_, err := msg.Reply(b, fmt.Sprintf(text, strings.Join(failedLocks, ", ")), formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	} else {
		temp, _ := tr.GetString("locks_unlocked_successfully")
		text := fmt.Sprintf(temp, strings.Join(toUnlock, "\n - "))
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
	}

	return ext.EndGroups
}

func (moduleStruct) restHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	msg := ctx.EffectiveMessage
	sender := ctx.EffectiveSender

	if sender == nil {
		return ext.ContinueGroups
	}

	chatLocks := locks.GetChatLocks(chat.Id)
	hasActiveLock := false
	for restrKey := range restrMap {
		if chatLocks[restrKey] {
			hasActiveLock = true
			break
		}
	}
	if !hasActiveLock {
		return ext.ContinueGroups
	}

	senderID := sender.Id()

	if chat_status.IsUserAdmin(b, chat.Id, senderID) {
		return ext.ContinueGroups
	}
	if senderID > 0 && chat_status.IsApproved(b, chat.Id, senderID) {
		return ext.ContinueGroups
	}

	if !chat_status.CanBotDelete(b, ctx, nil) {
		return ext.ContinueGroups
	}

	for restr, filter := range restrMap {
		if !filter(msg) || !locks.IsPermLocked(chat.Id, restr) {
			continue
		}

		// but skip Telegram's system account (777000) which forwards channel posts
		if restr == "comments" {
			if msg.From != nil && msg.From.Id == 777000 {
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

	chatLocks := locks.GetChatLocks(chat.Id)
	hasActiveLock := false
	for permKey := range lockMap {
		if chatLocks[permKey] {
			hasActiveLock = true
			break
		}
	}
	if !hasActiveLock {
		return ext.ContinueGroups
	}

	senderID := sender.Id()

	if chat_status.IsUserAdmin(b, chat.Id, senderID) {
		return ext.ContinueGroups
	}
	if senderID > 0 && chat_status.IsApproved(b, chat.Id, senderID) {
		return ext.ContinueGroups
	}

	if !chat_status.CanBotDelete(b, ctx, nil) {
		return ext.ContinueGroups
	}

	for perm, filter := range lockMap {
		if !filter(msg) || !locks.IsPermLocked(chat.Id, perm) {
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
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_bot_restrict_group_error", "chat_status_bot_restrict_error")
		return ext.ContinueGroups
	}

	_, err := chat.BanMember(b, mem.Id, nil)
	if err != nil {
		log.Error(err)
		return err
	}

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
	DefaultHelpRegistry().AbleMap[locksModule.moduleName] = true

	dispatcher.AddHandler(handlers.NewCommand("lock", locksModule.lockPerm))
	dispatcher.AddHandler(handlers.NewCommand("unlock", locksModule.unlockPerm))
	dispatcher.AddHandler(handlers.NewCommand("locktypes", locksModule.locktypes))
	helpers.AddCmdToDisableable("locktypes")
	dispatcher.AddHandler(handlers.NewCommand("locks", locksModule.locks))
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
