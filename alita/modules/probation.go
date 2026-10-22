package modules

import (
	"context"
	"errors"
	"hash/fnv"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/probation"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/cache"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/error_handling"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
	"github.com/divkix/Alita_Robot/alita/utils/tracing"
)

const (
	// probationHandlerGroup sits after the users tracker (-1) and the
	// connected-chat guard (1), and before aispam (3), antiflood (4) and
	// locks (5/6), so a blocked message is removed before they look at it.
	probationHandlerGroup = 2

	probationRedisPrefix  = "alita:probation:"
	probationRedisTimeout = 2 * time.Second
	// probationRetention is how long a member row may outlive its probation:
	// past the longest allowed time every row has run out.
	probationRetention = models.ProbationDurationMax
)

var (
	probationModule = moduleStruct{
		moduleName:   "Probation",
		handlerGroup: probationHandlerGroup,
	}

	probationDesc = helpers.CommandDescriptor{
		Name: "probation",
		// Every /probation form is admin-only, so a chat cannot silence it:
		// /probation off already is the switch.
		Disableable: false,
		PMGroupArg:  helpers.PMGroupArgOwn,
		RequiredChecks: []helpers.CheckFunc{
			groupOnlyChatCheck,
			helpers.RequireGroup(),
			helpers.RequireBotAdmin(),
			helpers.RequireUserAdmin(),
			helpers.CanUserRestrict(),
		},
	}

	// probationNoticeLifetime is how long the notice to a member stays in the
	// group. Tests shorten it.
	probationNoticeLifetime = time.Minute

	probationRetentionOnce sync.Once
)

// probationBlockNames are the kinds /probation block takes, in the order the
// replies list them.
var probationBlockNames = []string{probation.BlockLinks, probation.BlockForwards, probation.BlockMedia}

func probationRedisKey(chatID, userID int64) string {
	return probationRedisPrefix + strconv.FormatInt(chatID, 10) + ":" + strconv.FormatInt(userID, 10)
}

// startProbation puts a member who just joined on probation when the chat has
// it on. ProcessSingleJoin calls it only for the update that won the join
// claim and only for a member who is not approved, so a member another guard
// banned on the same join never gets a row.
func startProbation(ctx context.Context, chatID int64, user gotgbot.User) {
	if user.IsBot {
		return
	}
	settings, err := probation.GetSettings(ctx, chatID)
	if err != nil {
		log.WithFields(log.Fields{"chat_id": chatID, "error": err}).Warn("[Probation] could not read the chat settings")
		return
	}
	if !settings.Enabled {
		return
	}
	if err := probation.Start(chatID, user.Id, time.Now()); err != nil {
		log.WithFields(log.Fields{"chat_id": chatID, "user_id": user.Id, "error": err}).Error("[Probation] could not start probation")
		return
	}
	// A member who joins again counts from zero.
	probationDropCount(chatID, user.Id)
}

// endProbation takes userID off probation in chatID: the row and the count.
func endProbation(chatID, userID int64) {
	if err := probation.End(chatID, userID); err != nil {
		log.WithFields(log.Fields{"chat_id": chatID, "user_id": userID, "error": err}).Error("[Probation] could not end probation")
	}
	probationDropCount(chatID, userID)
}

// endProbationOnLeave ends the probation of a member who left, when the chat
// has probation on and the member is on it. A chat with it off pays only the
// cached settings read; a row left there runs out or is swept.
func endProbationOnLeave(ctx context.Context, chatID, userID int64) {
	settings, err := probation.GetSettings(ctx, chatID)
	if err != nil || !settings.Enabled {
		return
	}
	member, err := probation.GetMember(ctx, chatID, userID)
	if err != nil {
		log.WithFields(log.Fields{"chat_id": chatID, "user_id": userID, "error": err}).Warn("[Probation] could not read the member on leave")
		return
	}
	if member.Found {
		endProbation(chatID, userID)
	}
}

func probationDropCount(chatID, userID int64) {
	rdb := cache.GetRedisClient()
	if rdb == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), probationRedisTimeout)
	defer cancel()
	if err := rdb.Del(ctx, probationRedisKey(chatID, userID)).Err(); err != nil {
		log.WithFields(log.Fields{"chat_id": chatID, "user_id": userID, "error": err}).Warn("[Probation] could not drop the message count")
	}
}

// probationCount records text as one of userID's messages and returns how
// many different messages the member has sent. The set expires with the
// probation, so nothing is left behind. ok is false without Redis, or when
// Redis fails: the message is then not counted and probation ends on time.
func probationCount(chatID, userID int64, text string, ttl time.Duration) (count int64, ok bool) {
	rdb := cache.GetRedisClient()
	if rdb == nil {
		return 0, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), probationRedisTimeout)
	defer cancel()
	key := probationRedisKey(chatID, userID)
	pipe := rdb.TxPipeline()
	pipe.SAdd(ctx, key, probationTextHash(text))
	pipe.Expire(ctx, key, max(ttl, time.Minute))
	card := pipe.SCard(ctx, key)
	if _, err := pipe.Exec(ctx); err != nil {
		log.WithFields(log.Fields{"chat_id": chatID, "user_id": userID, "error": err}).Warn("[Probation] could not count the message")
		return 0, false
	}
	return card.Val(), true
}

// probationTextHash identifies a message by its text with case and spacing
// folded, so repeating a message, or changing only its case or spacing, does
// not count twice.
func probationTextHash(text string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(strings.Join(strings.Fields(strings.ToLower(text)), " ")))
	return strconv.FormatUint(h.Sum64(), 36)
}

// probationBlocked reports whether msg holds something settings keep from a
// member on probation.
func probationBlocked(settings *models.ProbationSettings, msg *gotgbot.Message) bool {
	if settings.BlockLinks && hasURLEntity(msg) {
		return true
	}
	if settings.BlockForwards && (message.Forwarded(msg) || msg.ExternalReply != nil) {
		return true
	}
	return settings.BlockMedia && MEDIA(msg)
}

// checkProbation is the group 2 watcher. It deletes what a member on
// probation may not post, counts the member's other messages, and ends the
// probation once the member has sent enough of them or the time is up. It
// always returns ContinueGroups.
func (moduleStruct) checkProbation(b *gotgbot.Bot, ctx *ext.Context) error {
	user := probationSubject(ctx)
	if user == nil {
		return ext.ContinueGroups
	}
	chat, msg := ctx.EffectiveChat, ctx.EffectiveMessage

	reqCtx := tracing.UpdateContext(ctx)
	settings, err := probation.GetSettings(reqCtx, chat.Id)
	if err != nil {
		log.WithFields(log.Fields{"chat_id": chat.Id, "error": err}).Warn("[Probation] could not read the chat settings")
		return ext.ContinueGroups
	}
	if !settings.Enabled {
		return ext.ContinueGroups
	}
	member, err := probation.GetMember(reqCtx, chat.Id, user.Id)
	if err != nil {
		log.WithFields(log.Fields{"chat_id": chat.Id, "user_id": user.Id, "error": err}).Warn("[Probation] could not read the member")
		return ext.ContinueGroups
	}
	if !member.Found {
		return ext.ContinueGroups
	}
	left := time.Until(member.JoinedAt.Add(settings.Duration()))
	if left <= 0 {
		endProbation(chat.Id, user.Id)
		return ext.ContinueGroups
	}
	if chat_status.IsUserAdminForUpdate(b, ctx, chat.Id, user.Id) || chat_status.IsApprovedForUpdate(b, ctx, chat.Id, user.Id) {
		return ext.ContinueGroups
	}

	if probationBlocked(settings, msg) {
		deleteProbationMessage(b, ctx, settings, member, user)
		return ext.ContinueGroups
	}

	text := aispamMessageText(msg)
	if utf8.RuneCountInString(strings.TrimSpace(text)) < settings.MinLength {
		return ext.ContinueGroups
	}
	if count, ok := probationCount(chat.Id, user.Id, text, left); ok && count >= int64(settings.MessageCount) {
		endProbation(chat.Id, user.Id)
	}
	return ext.ContinueGroups
}

// probationSubject returns the user who sent the update's group message when
// probation may apply to it: a person posting as themselves (IsUser rules out
// anonymous admins and channels), not a bot, and not a join or leave notice.
func probationSubject(ctx *ext.Context) *gotgbot.User {
	chat, msg, sender := ctx.EffectiveChat, ctx.EffectiveMessage, ctx.EffectiveSender
	if chat == nil || msg == nil || sender == nil || (chat.Type != "group" && chat.Type != "supergroup") {
		return nil
	}
	if !sender.IsUser() || sender.User.IsBot || msg.NewChatMembers != nil || msg.LeftChatMember != nil {
		return nil
	}
	return sender.User
}

// deleteProbationMessage removes a blocked message, when the bot may, and
// sends the notice when the chat wants it.
func deleteProbationMessage(b *gotgbot.Bot, ctx *ext.Context, settings *models.ProbationSettings, member probation.Member, user *gotgbot.User) {
	chat, msg := ctx.EffectiveChat, ctx.EffectiveMessage
	if !chat_status.CanBotDeleteForUpdate(b, ctx, nil) {
		return
	}
	if err := helpers.DeleteMessageWithErrorHandling(b, chat.Id, msg.MessageId); err != nil {
		log.WithFields(log.Fields{"chat_id": chat.Id, "message_id": msg.MessageId, "error": err}).Warn("[Probation] could not delete a blocked message")
		return
	}
	// The cached member says whether the notice already went out, so a
	// member who keeps posting links costs no claim write.
	if settings.Notice && !member.Notified {
		sendProbationNotice(b, ctx, settings, user)
	}
}

// sendProbationNotice tells the member once per probation why a message was
// removed, in the group, and deletes the notice after probationNoticeLifetime.
func sendProbationNotice(b *gotgbot.Bot, ctx *ext.Context, settings *models.ProbationSettings, user *gotgbot.User) {
	chat := ctx.EffectiveChat
	claimed, err := probation.ClaimNotice(chat.Id, user.Id)
	if err != nil {
		log.WithFields(log.Fields{"chat_id": chat.Id, "user_id": user.Id, "error": err}).Error("[Probation] could not claim the notice")
		return
	}
	if !claimed {
		return
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	text, _ := tr.GetString("probation_notice", i18n.TranslationParams{
		"user":    formatting.MentionHtml(user.Id, formatting.GetFullName(user.FirstName, user.LastName)),
		"blocked": probationBlockedText(tr, settings),
	})
	opts := &gotgbot.SendMessageOpts{ParseMode: formatting.HTML}
	if msg := ctx.EffectiveMessage; msg != nil && msg.IsTopicMessage {
		opts.MessageThreadId = msg.MessageThreadId
	}
	sent, err := helpers.SendMessageWithErrorHandling(b, chat.Id, text, opts)
	if err != nil {
		log.WithFields(log.Fields{"chat_id": chat.Id, "user_id": user.Id, "error": err}).Warn("[Probation] could not send the notice")
		return
	}
	if sent == nil {
		return
	}
	chatID, messageID := chat.Id, sent.MessageId
	time.AfterFunc(probationNoticeLifetime, func() {
		defer error_handling.RecoverFromPanic("probationNoticeDelete", "probation")
		_ = helpers.DeleteMessageWithErrorHandling(b, chatID, messageID)
	})
}

// probationBlockedText lists what settings block, in the reader's language.
func probationBlockedText(tr *i18n.Translator, settings *models.ProbationSettings) string {
	var names []string
	for _, kind := range probationBlockedKinds(settings) {
		name, _ := tr.GetString("probation_block_" + kind)
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

// probationBlockedKinds names what settings block, as /probation block takes them.
func probationBlockedKinds(settings *models.ProbationSettings) []string {
	var kinds []string
	for _, kind := range probationBlockNames {
		if (kind == probation.BlockLinks && settings.BlockLinks) || (kind == probation.BlockForwards && settings.BlockForwards) ||
			(kind == probation.BlockMedia && settings.BlockMedia) {
			kinds = append(kinds, kind)
		}
	}
	return kinds
}

// probation handles /probation. A bare call shows the chat's settings; on,
// off, time, messages, minlen, block and notice change them.
func (moduleStruct) probation(c *helpers.CommandContext) error {
	settings, err := probation.GetSettings(tracing.UpdateContext(c.Ctx), c.Chat.Id)
	if err != nil {
		return probationSaveFailed(c, err)
	}
	args := c.Ctx.Args()[1:]
	if len(args) == 0 {
		return probationReply(c, probationStatusText(c, settings))
	}

	switch option := strings.ToLower(args[0]); option {
	case "time":
		return probationSetTime(c, args[1:])
	case "messages":
		return probationSetNumber(c, args[1:], probation.SetMessageCount,
			"probation_invalid_messages", "probation_messages_success", "messages")
	case "minlen":
		return probationSetNumber(c, args[1:], probation.SetMinLength,
			"probation_invalid_minlen", "probation_minlen_success", "minlen")
	case "block":
		return probationSetBlock(c, args[1:])
	case "notice":
		if len(args) != 2 {
			return probationUsage(c)
		}
		on, ok := parseToggleArg(args[1])
		if !ok {
			return probationUsage(c)
		}
		if err := probation.SetNotice(c.Chat.Id, on); err != nil {
			return probationSaveFailed(c, err)
		}
		if on {
			return probationReplyKey(c, "probation_notice_on_success")
		}
		return probationReplyKey(c, "probation_notice_off_success")
	default:
		on, ok := parseToggleArg(option)
		if !ok || len(args) != 1 {
			return probationUsage(c)
		}
		if err := probation.SetEnabled(c.Chat.Id, on); err != nil {
			return probationSaveFailed(c, err)
		}
		if !on {
			return probationReplyKey(c, "probation_disabled_success")
		}
		text, _ := c.Tr.GetString("probation_enabled_success")
		if warning := probationNoDeleteWarning(c); warning != "" {
			text += "\n\n" + warning
		}
		return probationReply(c, text)
	}
}

func probationSetTime(c *helpers.CommandContext, args []string) error {
	if len(args) != 1 {
		return probationReplyKey(c, "probation_invalid_time")
	}
	seconds, ok := parseDuration(args[0])
	if !ok {
		return probationReplyKey(c, "probation_invalid_time")
	}
	if err := probation.SetDuration(c.Chat.Id, time.Duration(seconds)*time.Second); errors.Is(err, probation.ErrOutOfRange) {
		return probationReplyKey(c, "probation_invalid_time")
	} else if err != nil {
		return probationSaveFailed(c, err)
	}
	return probationReplyKey(c, "probation_time_success", i18n.TranslationParams{"time": formatDuration(seconds)})
}

// probationSetNumber sets one numeric setting; set refuses a value out of
// its range with probation.ErrOutOfRange.
func probationSetNumber(c *helpers.CommandContext, args []string, set func(int64, int) error, invalidKey, successKey, param string) error {
	if len(args) != 1 {
		return probationReplyKey(c, invalidKey)
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		return probationReplyKey(c, invalidKey)
	}
	if err := set(c.Chat.Id, n); errors.Is(err, probation.ErrOutOfRange) {
		return probationReplyKey(c, invalidKey)
	} else if err != nil {
		return probationSaveFailed(c, err)
	}
	return probationReplyKey(c, successKey, i18n.TranslationParams{param: n})
}

// probationSetBlock is "/probation block links,forwards,media": the kinds
// named, separated by commas or spaces, replace the chat's choice. At least
// one is needed; turn probation off to block nothing.
func probationSetBlock(c *helpers.CommandContext, args []string) error {
	choice := map[string]bool{}
	for _, field := range strings.FieldsFunc(strings.ToLower(strings.Join(args, " ")), func(r rune) bool {
		return r == ',' || r == ' '
	}) {
		if !slices.Contains(probationBlockNames, field) {
			return probationReplyKey(c, "probation_invalid_block")
		}
		choice[field] = true
	}
	if len(choice) == 0 {
		return probationReplyKey(c, "probation_invalid_block")
	}
	links, forwards, media := choice[probation.BlockLinks], choice[probation.BlockForwards], choice[probation.BlockMedia]
	if err := probation.SetBlocked(c.Chat.Id, links, forwards, media); err != nil {
		return probationSaveFailed(c, err)
	}
	blocked := probationBlockedText(c.Tr, &models.ProbationSettings{BlockLinks: links, BlockForwards: forwards, BlockMedia: media})
	return probationReplyKey(c, "probation_block_success", i18n.TranslationParams{"blocked": blocked})
}

func probationStatusText(c *helpers.CommandContext, settings *models.ProbationSettings) string {
	tr := c.Tr
	stateKey := "probation_status_off"
	if settings.Enabled {
		stateKey = "probation_status_on"
	}
	state, _ := tr.GetString(stateKey)
	details, _ := tr.GetString("probation_status_details", i18n.TranslationParams{
		"blocked":  probationBlockedText(tr, settings),
		"time":     formatDuration(settings.DurationSeconds),
		"messages": settings.MessageCount,
		"minlen":   settings.MinLength,
	})
	noticeKey := "probation_status_notice_off"
	if settings.Notice {
		noticeKey = "probation_status_notice_on"
	}
	notice, _ := tr.GetString(noticeKey)
	parts := []string{state, details, notice}
	if cache.GetRedisClient() == nil {
		warning, _ := tr.GetString("probation_status_no_redis")
		parts = append(parts, warning)
	}
	if settings.Enabled {
		if warning := probationNoDeleteWarning(c); warning != "" {
			parts = append(parts, warning)
		}
	}
	return strings.Join(parts, "\n\n")
}

// probationNoDeleteWarning warns that probation does nothing while the bot
// cannot delete messages in the chat, or is empty when it can.
func probationNoDeleteWarning(c *helpers.CommandContext) string {
	if chat_status.CanBotDeleteForUpdate(c.Bot, c.Ctx, c.Chat) {
		return ""
	}
	warning, _ := c.Tr.GetString("probation_status_no_delete")
	return warning
}

func probationUsage(c *helpers.CommandContext) error {
	usage, _ := c.Tr.GetString("probation_usage")
	return replyCommandHelp(c.Bot, c.Msg, c.Tr, "probation", usage)
}

func probationSaveFailed(c *helpers.CommandContext, err error) error {
	log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).Error("[Probation] failed to save the setting")
	if replyErr := probationReplyKey(c, "common_settings_save_failed"); replyErr != nil {
		return replyErr
	}
	return err
}

func probationReplyKey(c *helpers.CommandContext, key string, params ...i18n.TranslationParams) error {
	text, _ := c.Tr.GetString(key, params...)
	return probationReply(c, text)
}

func probationReply(c *helpers.CommandContext, text string) error {
	_, err := c.Msg.Reply(c.Bot, withGroupLine(c.Tr, c.Msg, c.Chat, text), formatting.Shtml())
	return err
}

// sweepProbationMembers deletes the member rows older than the longest
// probation. Every replica sweeps; the delete is idempotent.
func sweepProbationMembers(now time.Time) {
	deleted, err := probation.DeleteJoinedBefore(now.Add(-probationRetention))
	if err != nil {
		log.Errorf("[Probation] retention sweep failed: %v", err)
		return
	}
	if deleted > 0 {
		log.Infof("[Probation] deleted %d member rows older than %v", deleted, probationRetention)
	}
}

func LoadProbation(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(probationModule.moduleName, true)

	helpers.WrapCommand(dispatcher, probationDesc, probationModule.probation)

	dispatcher.AddHandlerToGroup(
		handlers.NewMessage(message.All, probationModule.checkProbation),
		probationModule.handlerGroup,
	)

	startRetentionSweeper(&probationRetentionOnce, "probationRetention", time.Hour, sweepProbationMembers)
}

func init() {
	RegisterLegacyModule("Probation", 14, LoadProbation)
	RegisterAnonymousAdminHandler("probation", anonCommandContextHandler(probationDesc, probationModule.probation))
}
