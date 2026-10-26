package modules

import (
	"fmt"
	"html"
	"slices"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db/connections"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/callbackcodec"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

// /commands shows the help modules' commands by section: a first screen of
// sections, a section screen of its commands and a command screen with the
// command's help and example. A member sees only the commands a member can
// use (menuCommands), an admin all of them. The menu lives in PM: in a group
// the command links there, carrying the group.

const (
	commandsMenuNamespace  = "cmds"
	commandsDeepLinkPrefix = "commands_"
	commandsButtonsPerRow  = 3
)

// commandsView is what a menu screen depends on besides the language: whether
// admin commands are shown, and the group examples name (0 for none). Buttons
// carry it, so pressing one checks nothing again; it only selects help text.
type commandsView struct {
	admin bool
	group int64
}

func (v commandsView) callbackData(fields map[string]string) string {
	fields["v"] = "m"
	if v.admin {
		fields["v"] = "a"
	}
	if v.group != 0 {
		fields["c"] = strconv.FormatInt(v.group, 10)
	}
	return encodeCallbackData(commandsMenuNamespace, fields)
}

func commandsViewFromCallback(decoded *callbackcodec.Decoded) commandsView {
	v, _ := decoded.Field("v")
	view := commandsView{admin: v == "a"}
	if raw, ok := decoded.Field("c"); ok {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id < 0 {
			view.group = id
		}
	}
	return view
}

// commandsMenu answers /commands: in PM with the menu, in a group with a
// button that opens it in PM for that group.
func (moduleStruct) commandsMenu(c *helpers.CommandContext) error {
	if c.Chat.Type == "private" {
		return sendCommandsMenu(c.Bot, c.Ctx, c.Tr, c.User.Id, 0)
	}
	text, _ := c.Tr.GetString("commands_open_in_pm")
	button, _ := c.Tr.GetString("commands_open_in_pm_button")
	_, err := c.Msg.Reply(c.Bot, text, &gotgbot.SendMessageOpts{
		ParseMode: formatting.HTML,
		ReplyParameters: &gotgbot.ReplyParameters{
			MessageId:                c.Msg.MessageId,
			AllowSendingWithoutReply: true,
		},
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{{
			Text: button,
			Url:  fmt.Sprintf("https://t.me/%s?start=%s%d", getBotUsername(c.Bot), commandsDeepLinkPrefix, c.Chat.Id),
		}}}},
	})
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

func commandsDeepLinkHandler(b *gotgbot.Bot, ctx *ext.Context, user *gotgbot.User, arg string) error {
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	chatID, err := strconv.ParseInt(strings.TrimPrefix(arg, commandsDeepLinkPrefix), 10, 64)
	if err != nil || chatID >= 0 {
		text, _ := tr.GetString("helpers_invalid_deep_link")
		_, _ = ctx.EffectiveMessage.Reply(b, text, formatting.Shtml())
		return ext.EndGroups
	}
	return sendCommandsMenu(b, ctx, tr, user.Id, chatID)
}

func sendCommandsMenu(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator, userID, linkedGroup int64) error {
	view, notice := commandsViewFor(b, ctx, tr, userID, linkedGroup)
	text, kb := notice, gotgbot.InlineKeyboardMarkup{}
	if notice == "" {
		text, kb = commandsHomeScreen(tr, view)
	}
	_, err := ctx.EffectiveMessage.Reply(b, text, &gotgbot.SendMessageOpts{
		ParseMode:          formatting.HTML,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
		ReplyMarkup:        kb,
	})
	if err != nil {
		log.Error(err)
		return err
	}
	return ext.EndGroups
}

// commandsViewFor decides whether userID sees admin commands: an admin of the
// group the deep link names, else of the connected group, else of any group
// in the /settings picker's list (adminGroupsForUser, which shares the
// picker's pause and scan). During the pause with no list kept it returns
// the picker's wait notice instead. Examples name the linked group, else the
// connected one.
// Reuse that picker lookup rather than introducing a separate admin-group scan.
func commandsViewFor(b *gotgbot.Bot, ctx *ext.Context, tr *i18n.Translator, userID, linkedGroup int64) (commandsView, string) {
	view := commandsView{group: linkedGroup}
	if linkedGroup != 0 && chat_status.IsUserAdmin(b, linkedGroup, userID) {
		view.admin = true
		return view, ""
	}
	if conn := connections.Connection(userID); conn.Connected && conn.ChatId != 0 {
		if view.group == 0 {
			view.group = conn.ChatId
		}
		if chat_status.IsUserAdmin(b, conn.ChatId, userID) {
			view.admin = true
			return view, ""
		}
	}
	entries, wait := adminGroupsForUser(b, ctx, tr, userID)
	if wait != "" {
		return view, wait
	}
	view.admin = len(entries) > 0
	return view, ""
}

// commandsSections lists the help menu's modules (lower case) by their shown
// names, in help order.
func commandsSections() (modules []string, names map[string]string) {
	names = map[string]string{}
	for _, name := range listModulesFrom(DefaultHelpRegistry()) {
		module := strings.ToLower(name)
		modules = append(modules, module)
		names[module] = name
	}
	return modules, names
}

// sectionCommands lists module's commands the view shows, in the order of the
// English <module>_extend_msg.
func sectionCommands(module string, view commandsView) []string {
	order, _ := extendLines(i18n.MustNewTranslator("en"), module)
	var out []string
	for _, cmd := range order {
		entry, ok := menuCommands[cmd]
		if ok && entry.Module == module && (view.admin || !entry.AdminOnly) {
			out = append(out, cmd)
		}
	}
	return out
}

// extendLines splits module's <module>_extend_msg in tr into its commands, in
// order, and each command's "/cmd — description" line.
func extendLines(tr *i18n.Translator, module string) (order []string, lines map[string]string) {
	text, _ := tr.GetString(module + "_extend_msg")
	lines = map[string]string{}
	for line := range strings.SplitSeq(strings.TrimSpace(text), "\n") {
		line = strings.TrimSpace(line)
		name, _, found := strings.Cut(line, " — ")
		if !found || !strings.HasPrefix(name, "/") {
			continue
		}
		cmd := strings.TrimPrefix(name, "/")
		if _, seen := lines[cmd]; !seen {
			order = append(order, cmd)
		}
		lines[cmd] = line
	}
	return order, lines
}

func sectionHeading(tr *i18n.Translator, module, name string) string {
	heading := "<b>" + html.EscapeString(name) + "</b>"
	if desc, _ := tr.GetString(module + "_section_desc"); desc != "" {
		heading += " — " + html.EscapeString(desc)
	}
	return heading
}

func chunkButtons(buttons []gotgbot.InlineKeyboardButton) [][]gotgbot.InlineKeyboardButton {
	return slices.Collect(slices.Chunk(buttons, commandsButtonsPerRow))
}

// commandsHomeScreen lists the sections with a command the view shows.
func commandsHomeScreen(tr *i18n.Translator, view commandsView) (string, gotgbot.InlineKeyboardMarkup) {
	headerKey := "commands_menu_member"
	if view.admin {
		headerKey = "commands_menu_admin"
	}
	header, _ := tr.GetString(headerKey)
	lines := []string{header, ""}
	var buttons []gotgbot.InlineKeyboardButton
	modules, names := commandsSections()
	for _, module := range modules {
		if len(sectionCommands(module, view)) == 0 {
			continue
		}
		lines = append(lines, sectionHeading(tr, module, names[module]))
		buttons = append(buttons, gotgbot.InlineKeyboardButton{
			Text:         names[module],
			CallbackData: view.callbackData(map[string]string{"a": "s", "m": module}),
		})
	}
	return strings.Join(lines, "\n"), gotgbot.InlineKeyboardMarkup{InlineKeyboard: chunkButtons(buttons)}
}

// commandsSectionScreen lists module's commands the view shows, one
// "/cmd — description" line each from <module>_extend_msg. ok is false for a
// module outside the help menu or without such commands.
func commandsSectionScreen(tr *i18n.Translator, view commandsView, module string) (string, gotgbot.InlineKeyboardMarkup, bool) {
	_, names := commandsSections()
	name, listed := names[module]
	commands := sectionCommands(module, view)
	if !listed || len(commands) == 0 {
		return "", gotgbot.InlineKeyboardMarkup{}, false
	}
	_, localLines := extendLines(tr, module)
	_, enLines := extendLines(i18n.MustNewTranslator("en"), module)
	lines := []string{sectionHeading(tr, module, name), ""}
	buttons := make([]gotgbot.InlineKeyboardButton, 0, len(commands))
	for _, cmd := range commands {
		line, ok := localLines[cmd]
		if !ok {
			line = enLines[cmd]
		}
		lines = append(lines, line)
		buttons = append(buttons, gotgbot.InlineKeyboardButton{
			Text:         "/" + cmd,
			CallbackData: view.callbackData(map[string]string{"a": "c", "n": cmd}),
		})
	}
	if hint, _ := tr.GetString("commands_section_hint"); hint != "" {
		lines = append(lines, "", hint)
	}
	back, _ := tr.GetString("common_back_arrow")
	rows := append(chunkButtons(buttons), []gotgbot.InlineKeyboardButton{{
		Text:         back,
		CallbackData: view.callbackData(map[string]string{"a": "h"}),
	}})
	return strings.Join(lines, "\n"), gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}, true
}

// commandsCommandScreen shows cmd's help entry (or its module's help when the
// language has no entry for it), its PM example naming the view's group, a
// button that copies the example, and Back to its section.
func commandsCommandScreen(tr *i18n.Translator, view commandsView, cmd string) (string, gotgbot.InlineKeyboardMarkup, bool) {
	entry, ok := menuCommands[cmd]
	if !ok {
		return "", gotgbot.InlineKeyboardMarkup{}, false
	}
	example := commandPMExample(cmd)
	if view.group != 0 {
		example = exampleNamingGroup(example, view.group)
	}
	text := commandHelpBody(tr, entry.Module, cmd, example)
	if text == "" {
		return "", gotgbot.InlineKeyboardMarkup{}, false
	}
	var rows [][]gotgbot.InlineKeyboardButton
	if example != "" {
		copyText, _ := tr.GetString("commands_copy_example_button")
		rows = append(rows, []gotgbot.InlineKeyboardButton{{
			Text:     copyText,
			CopyText: &gotgbot.CopyTextButton{Text: example},
		}})
	}
	back, _ := tr.GetString("common_back_arrow")
	rows = append(rows, []gotgbot.InlineKeyboardButton{{
		Text:         back,
		CallbackData: view.callbackData(map[string]string{"a": "s", "m": entry.Module}),
	}})
	return text, gotgbot.InlineKeyboardMarkup{InlineKeyboard: rows}, true
}

// examplePlaceholderGroups are the ways locales/config.yml examples name a
// group.
var examplePlaceholderGroups = []string{"@mygroup", `"My Group"`, "-1001234567890"}

// exampleNamingGroup puts groupID in place of the placeholder group that
// example names first, and returns example unchanged when it names none.
func exampleNamingGroup(example string, groupID int64) string {
	cmd, rest, found := strings.Cut(example, " ")
	if !found {
		return example
	}
	for _, placeholder := range examplePlaceholderGroups {
		after, ok := strings.CutPrefix(rest, placeholder)
		if ok && (after == "" || strings.HasPrefix(after, " ")) {
			return cmd + " " + strconv.FormatInt(groupID, 10) + after
		}
	}
	return example
}

func (moduleStruct) commandsMenuCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	query, ok := callbackQueryFromContext(ctx)
	if !ok {
		return ext.EndGroups
	}
	msg := query.Message
	decoded, ok := decodeCallbackData(query.Data, commandsMenuNamespace)
	// The menu is sent only in PM; a forged button elsewhere must not edit a
	// bot message there.
	if msg == nil || !ok || ctx.EffectiveChat == nil || ctx.EffectiveChat.Type != "private" {
		return answerInvalidCallback(b, ctx, query)
	}
	tr := i18n.MustNewTranslator(lang.GetLanguage(ctx))
	view := commandsViewFromCallback(decoded)

	var (
		text  string
		kb    gotgbot.InlineKeyboardMarkup
		valid = true
	)
	switch action, _ := decoded.Field("a"); action {
	case "h":
		text, kb = commandsHomeScreen(tr, view)
	case "s":
		module, _ := decoded.Field("m")
		text, kb, valid = commandsSectionScreen(tr, view, module)
	case "c":
		cmd, _ := decoded.Field("n")
		text, kb, valid = commandsCommandScreen(tr, view, cmd)
	default:
		valid = false
	}
	if !valid {
		return answerInvalidCallback(b, ctx, query)
	}

	if _, _, err := msg.EditText(b, &gotgbot.EditMessageTextOpts{
		Text:               text,
		ParseMode:          formatting.HTML,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{IsDisabled: true},
		ReplyMarkup:        kb,
	}); err != nil {
		if strings.Contains(err.Error(), "message is not modified") {
			log.Debug(err)
		} else {
			log.Error(err)
		}
	}
	_, _ = query.Answer(b, nil)
	return ext.EndGroups
}

func loadCommandsMenu(dispatcher *ext.Dispatcher) {
	helpers.WrapCommand(dispatcher, helpers.CommandDescriptor{Name: "commands"}, DefaultHelpRegistry().commandsMenu)
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix(commandsMenuNamespace+"|"), DefaultHelpRegistry().commandsMenuCallback))
}

func init() {
	RegisterDeepLinkHandler(commandsDeepLinkPrefix, commandsDeepLinkHandler)
}
