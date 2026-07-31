package modules

import (
	"fmt"
	"html"
	"slices"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/message"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/reactions"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
)

var reactionsModule = moduleStruct{
	moduleName:   "Reactions",
	handlerGroup: 8,
}

var supportedReactionEmoji = []string{
	"❤", "👍", "👎", "🔥", "🥰", "👏", "😁", "🤔", "🤯", "😱", "🤬", "😢",
	"🎉", "🤩", "🤮", "💩", "🙏", "👌", "🕊", "🤡", "🥱", "🥴", "😍", "🐳",
	"❤‍🔥", "🌚", "🌭", "💯", "🤣", "⚡", "🍌", "🏆", "💔", "🤨", "😐", "🍓",
	"🍾", "💋", "🖕", "😈", "😴", "😭", "🤓", "👻", "👨‍💻", "👀", "🎃", "🙈",
	"😇", "😨", "🤝", "✍", "🤗", "🫡", "🎅", "🎄", "☃", "💅", "🤪", "🗿",
	"🆒", "💘", "🙉", "🦄", "😘", "💊", "🙊", "😎", "👾", "🤷‍♂", "🤷",
	"🤷‍♀", "😡",
}

// LoadReactions loads the reactions module with all command handlers
func LoadReactions(dispatcher *ext.Dispatcher) {
	// Admin commands
	dispatcher.AddHandler(handlers.NewCommand("addreaction", reactionsModule.addReaction))
	dispatcher.AddHandler(handlers.NewCommand("removereaction", reactionsModule.removeReaction))
	dispatcher.AddHandler(handlers.NewCommand("reactions", reactionsModule.listReactions))
	dispatcher.AddHandler(handlers.NewCommand("resetreactions", reactionsModule.resetReactions))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("reactions_help"), reactionsModule.reactionsHelpHandler))

	// Message watcher for reactions (positive handler group for monitoring)
	dispatcher.AddHandlerToGroup(handlers.NewMessage(message.All, reactionsModule.checkReactions), reactionsModule.handlerGroup)

	// Register module as disableable
	DefaultHelpRegistry().AbleMap[reactionsModule.moduleName] = true

	// Add help text
	DefaultHelpRegistry().AltHelpOptions["Reactions"] = []string{"reaction"}
	DefaultHelpRegistry().helpableKb["Reactions"] = [][]gotgbot.InlineKeyboardButton{
		{
			{
				Text:         "Add Reaction",
				CallbackData: encodeCallbackData("reactions_help", map[string]string{"action": "add"}),
			},
			{
				Text:         "Remove Reaction",
				CallbackData: encodeCallbackData("reactions_help", map[string]string{"action": "remove"}),
			},
		},
	}

	log.Info("[Modules] Reactions module loaded")
}

// reactionsHelpHandler handles inline help callbacks for reaction commands.
func (m moduleStruct) reactionsHelpHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	if query == nil {
		return ext.EndGroups
	}

	action := ""
	if decoded, ok := decodeCallbackData(query.Data, "reactions_help"); ok {
		action, _ = decoded.Field("action")
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	if action == "" {
		text, _ := tr.GetString("common_callback_invalid_request")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return ext.EndGroups
	}

	var helpText string
	switch action {
	case "add":
		helpText, _ = tr.GetString("reactions_add_usage")
	case "remove":
		helpText, _ = tr.GetString("reactions_remove_usage")
	default:
		text, _ := tr.GetString("common_callback_invalid_request")
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: text})
		return ext.EndGroups
	}

	if query.Message == nil {
		_, _ = query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: helpText})
		return ext.EndGroups
	}

	backText, _ := tr.GetString("common_back")
	_, _, err := query.Message.EditText(
		b,
		helpText,
		&gotgbot.EditMessageTextOpts{
			ParseMode: formatting.HTML,
			ReplyMarkup: gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{
							Text:         backText,
							CallbackData: encodeCallbackData("helpq", map[string]string{"m": "Reactions"}),
						},
					},
				},
			},
		},
	)
	if err != nil {
		log.Error(err)
		return err
	}

	_, err = query.Answer(b, nil)
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// addReaction handles /addreaction <keyword> <emoji> command
func (m moduleStruct) addReaction(b *gotgbot.Bot, ctx *ext.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Reactions][addReaction] Recovered from panic: %v", r)
		}
	}()

	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}

	// Check permission - only admins can add reactions
	if !chat_status.CanUserChangeInfo(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_change_info_cmd_error", "chat_status_change_info_button_error")
		return ext.EndGroups
	}

	args := ctx.Args()
	if len(args) < 3 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reactions_add_usage")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	keyword := strings.ToLower(strings.TrimSpace(args[1]))
	emoji := strings.ReplaceAll(strings.TrimSpace(args[2]), "\ufe0f", "")

	// ReactionTypeEmoji accepts only Telegram's documented reaction set.
	if !slices.Contains(supportedReactionEmoji, emoji) {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reactions_invalid_emoji")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	// Store in DB (cache is invalidated by the repository).
	if err := reactions.AddReaction(chat.Id, keyword, emoji); err != nil {
		log.Errorf("[Reactions] Failed to save reaction: %v", err)
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reactions_add_error")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	text, _ := tr.GetString("reactions_add_success", i18n.TranslationParams{
		"keyword": html.EscapeString(keyword),
		"emoji":   html.EscapeString(emoji),
	})
	_, err := msg.Reply(b, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// removeReaction handles /removereaction <keyword> command
func (m moduleStruct) removeReaction(b *gotgbot.Bot, ctx *ext.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Reactions][removeReaction] Recovered from panic: %v", r)
		}
	}()

	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}

	// Check permission
	if !chat_status.CanUserChangeInfo(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_change_info_cmd_error", "chat_status_change_info_button_error")
		return ext.EndGroups
	}

	args := ctx.Args()
	if len(args) < 2 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reactions_remove_usage")
		_, err := msg.Reply(b, text, formatting.Shtml())
		if err != nil {
			log.Error(err)
			return err
		}
		return ext.EndGroups
	}

	keyword := strings.ToLower(strings.TrimSpace(args[1]))

	reactionsMap := reactions.GetReactions(chat.Id)

	// Check if keyword exists
	if _, exists := reactionsMap[keyword]; !exists {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reactions_keyword_not_found", i18n.TranslationParams{
			"keyword": html.EscapeString(keyword),
		})
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	// Remove reaction from DB (cache is invalidated by the repository).
	if err := reactions.RemoveReaction(chat.Id, keyword); err != nil {
		log.Errorf("[Reactions] Failed to update reactions: %v", err)
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reactions_remove_error")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	text, _ := tr.GetString("reactions_remove_success", i18n.TranslationParams{
		"keyword": html.EscapeString(keyword),
	})
	_, err := msg.Reply(b, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// listReactions handles /reactions command
func (m moduleStruct) listReactions(b *gotgbot.Bot, ctx *ext.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Reactions][listReactions] Recovered from panic: %v", r)
		}
	}()

	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat

	reactionsMap := reactions.GetReactions(chat.Id)
	if len(reactionsMap) == 0 {
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reactions_none")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	// Build list
	var sb strings.Builder
	for keyword, emoji := range reactionsMap {
		fmt.Fprintf(&sb, "• %s → %s\n", html.EscapeString(keyword), html.EscapeString(emoji))
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	text, _ := tr.GetString("reactions_list_header", i18n.TranslationParams{
		"list": sb.String(),
	})
	_, err := msg.Reply(b, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// resetReactions handles /resetreactions command
func (m moduleStruct) resetReactions(b *gotgbot.Bot, ctx *ext.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Reactions][resetReactions] Recovered from panic: %v", r)
		}
	}()

	msg := ctx.EffectiveMessage
	chat := ctx.EffectiveChat
	user := chat_status.RequireUser(b, ctx)
	if user == nil {
		return ext.EndGroups
	}

	// Check permission
	if !chat_status.CanUserChangeInfo(b, ctx, chat, user.Id) {
		chat_status.NewPermissionResponder(b).Respond(ctx, "chat_status_change_info_cmd_error", "chat_status_change_info_button_error")
		return ext.EndGroups
	}

	// Delete all reactions from DB (cache is invalidated by the repository).
	if err := reactions.ResetReactions(chat.Id); err != nil {
		log.Errorf("[Reactions] Failed to reset reactions: %v", err)
		tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
		text, _ := tr.GetString("reactions_remove_error")
		_, _ = msg.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}

	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	text, _ := tr.GetString("reactions_reset_success")
	_, err := msg.Reply(b, text, formatting.Shtml())
	if err != nil {
		log.Error(err)
		return err
	}

	return ext.EndGroups
}

// checkReactions checks incoming messages and reacts with emojis when keywords match
func (m moduleStruct) checkReactions(b *gotgbot.Bot, ctx *ext.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("[Reactions][checkReactions] Recovered from panic: %v", r)
		}
	}()

	msg := ctx.EffectiveMessage
	if msg == nil || msg.Text == "" {
		return ext.ContinueGroups
	}

	chat := ctx.EffectiveChat
	if chat == nil {
		return ext.ContinueGroups
	}

	// Get reactions for this chat (read-through cache backed by the DB).
	reactionsMap := reactions.GetReactions(chat.Id)
	if len(reactionsMap) == 0 {
		return ext.ContinueGroups
	}

	// Check if message text contains any keywords (case-insensitive).
	// Collect all matching keywords first, then sort them for deterministic selection.
	lowerText := strings.ToLower(msg.Text)
	var matchedKeywords []string
	for keyword := range reactionsMap {
		if strings.Contains(lowerText, keyword) {
			matchedKeywords = append(matchedKeywords, keyword)
		}
	}
	if len(matchedKeywords) == 0 {
		return ext.ContinueGroups
	}
	slices.Sort(matchedKeywords)
	emoji := reactionsMap[matchedKeywords[0]]

	_, err := b.SetMessageReaction(
		chat.Id,
		msg.MessageId,
		&gotgbot.SetMessageReactionOpts{
			Reaction: []gotgbot.ReactionType{
				gotgbot.ReactionTypeEmoji{
					Emoji: emoji,
				},
			},
		},
	)
	if err != nil {
		log.Debugf("[Reactions] Failed to set reaction: %v", err)
	}

	return ext.ContinueGroups
}

func init() {
	RegisterLegacyModule("Reactions", 250, LoadReactions)
}
