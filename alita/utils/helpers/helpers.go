package helpers

import (
	"fmt"
	"html"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	tgmd2html "github.com/PaulSonOfLars/gotg_md2html"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/callbackcodec"
	"github.com/divkix/Alita_Robot/alita/utils/chat_status"
	"github.com/divkix/Alita_Robot/alita/utils/media"
	"github.com/divkix/Alita_Robot/alita/utils/string_handling"
)

// NOTE: small helper functions
// constants
const (
	Markdown             = "Markdown"
	HTML                 = "HTML"
	None                 = "None"
	MaxMessageLength int = 4096
)

// Precompiled regexes for ReverseHTML2MD function
var (
	linkRegex = regexp.MustCompile(`<a href="(.*?)">(.*?)</a>`)
	// Map of HTML tags to their compiled regexes
	htmlTagRegexes = map[string]*regexp.Regexp{
		"i":    regexp.MustCompile(`<i>(.*)</i>`),
		"u":    regexp.MustCompile(`<u>(.*)</u>`),
		"b":    regexp.MustCompile(`<b>(.*)</b>`),
		"s":    regexp.MustCompile(`<s>(.*)</s>`),
		"code": regexp.MustCompile(`<code>(.*)</code>`),
		"pre":  regexp.MustCompile(`<pre>(.*)</pre>`),
	}
)

// Shtml returns SendMessageOpts configured with HTML parse mode, disabled link preview,
// and reply parameters that allow sending without reply.
func Shtml() *gotgbot.SendMessageOpts {
	return &gotgbot.SendMessageOpts{
		ParseMode: HTML,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
			IsDisabled: true,
		},
		ReplyParameters: &gotgbot.ReplyParameters{
			AllowSendingWithoutReply: true,
		},
	}
}

// Smarkdown returns SendMessageOpts configured with Markdown parse mode, disabled link preview,
// and reply parameters that allow sending without reply.
func Smarkdown() *gotgbot.SendMessageOpts {
	return &gotgbot.SendMessageOpts{
		ParseMode: Markdown,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
			IsDisabled: true,
		},
		ReplyParameters: &gotgbot.ReplyParameters{
			AllowSendingWithoutReply: true,
		},
	}
}

// SplitMessage splits a message into multiple messages if it exceeds MaxMessageLength.
// It splits on newlines to preserve message structure when possible.
// Uses utf8.RuneCountInString to correctly count UTF-8 characters instead of bytes.
func SplitMessage(msg string) []string {
	// Count UTF-8 characters (runes), not bytes
	if utf8.RuneCountInString(msg) <= MaxMessageLength {
		return []string{msg}
	}

	lines := strings.Split(msg, "\n")
	smallMsg := ""
	result := make([]string, 0)

	for _, line := range lines {
		// Check if adding this line would exceed the limit (in runes, not bytes)
		potentialMsg := smallMsg + line + "\n"
		if utf8.RuneCountInString(potentialMsg) <= MaxMessageLength {
			smallMsg = potentialMsg
		} else {
			// If current line itself is too long, we need to split it
			if utf8.RuneCountInString(line) > MaxMessageLength {
				// Split the long line into chunks
				if smallMsg != "" {
					result = append(result, smallMsg)
					smallMsg = ""
				}
				// Split line by runes, not bytes
				runes := []rune(line)
				for len(runes) > 0 {
					chunkSize := min(MaxMessageLength, len(runes))
					result = append(result, string(runes[:chunkSize]))
					runes = runes[chunkSize:]
				}
			} else {
				// Current accumulated message is full, save it and start new
				if smallMsg != "" {
					result = append(result, smallMsg)
				}
				smallMsg = line + "\n"
			}
		}
	}

	// Don't forget the last accumulated message
	if smallMsg != "" {
		result = append(result, smallMsg)
	}

	return result
}

// MentionHtml creates an HTML mention link for a user using their Telegram user ID.
func MentionHtml(userId int64, name string) string {
	return MentionUrl(fmt.Sprintf("tg://user?id=%d", userId), name)
}

// MentionUrl creates an HTML link with the given URL and display name.
// The name is HTML-escaped for safety.
func MentionUrl(url, name string) string {
	return fmt.Sprintf("<a href=\"%s\">%s</a>", url, html.EscapeString(name))
}

// HtmlEscape escapes special HTML characters in a string to prevent injection.
// Used when inserting untrusted content into HTML-formatted messages.
func HtmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// GetFullName combines first name and last name into a full name.
// If last name is empty, returns only the first name.
func GetFullName(FirstName, LastName string) string {
	var name string
	if LastName != "" {
		name = FirstName + " " + LastName
	} else {
		name = FirstName
	}
	return name
}

// InitButtons creates an inline keyboard markup for the connection menu.
// Shows admin commands button if the user is an admin, otherwise shows only user commands.
func InitButtons(b *gotgbot.Bot, chatId, userId int64) gotgbot.InlineKeyboardMarkup {
	return InitButtonsWithLanguage(b, chatId, userId, "en")
}

// InitButtonsWithLanguage creates an inline keyboard markup with localized button text.
func InitButtonsWithLanguage(b *gotgbot.Bot, chatId, userId int64, language string) gotgbot.InlineKeyboardMarkup {
	tr := i18n.MustNewTranslator(language)
	adminText, _ := tr.GetString("helpers_admin_commands")
	if adminText == "" {
		adminText = "Admin commands" // fallback
	}
	userText, _ := tr.GetString("helpers_user_commands")
	if userText == "" {
		userText = "User commands" // fallback
	}

	var connButtons [][]gotgbot.InlineKeyboardButton
	if chat_status.IsUserAdmin(b, chatId, userId) {
		connButtons = [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         adminText,
					CallbackData: callbackcodec.EncodeOrFallback("connbtns", map[string]string{"t": "Admin"}, "connbtns.Admin"),
				},
			},
			{
				{
					Text:         userText,
					CallbackData: callbackcodec.EncodeOrFallback("connbtns", map[string]string{"t": "User"}, "connbtns.User"),
				},
			},
		}
	} else {
		connButtons = [][]gotgbot.InlineKeyboardButton{
			{
				{
					Text:         userText,
					CallbackData: callbackcodec.EncodeOrFallback("connbtns", map[string]string{"t": "User"}, "connbtns.User"),
				},
			},
		}
	}
	connKeyboard := gotgbot.InlineKeyboardMarkup{InlineKeyboard: connButtons}
	return connKeyboard
}

// GetMessageLinkFromMessageId generates a Telegram message link from chat and message ID.
// Handles both public groups (with username) and private groups (without username).
// NOTE: msg.GetLink() only works for supergroups/channels. This custom implementation
// also handles private groups and non-supergroups by constructing the link manually.
func GetMessageLinkFromMessageId(chat *gotgbot.Chat, messageId int64) (messageLink string) {
	messageLink = "https://t.me/"
	chatIdStr := fmt.Sprint(chat.Id)
	if chat.Username == "" {
		var linkId string
		if IsChannelID(chat.Id) {
			linkId = strings.ReplaceAll(chatIdStr, "-100", "")
		} else if strings.HasPrefix(chatIdStr, "-") && !IsChannelID(chat.Id) {
			// this is for non-supergroups
			linkId = strings.ReplaceAll(chatIdStr, "-", "")
		}
		messageLink += fmt.Sprintf("c/%s/%d", linkId, messageId)
	} else {
		messageLink += fmt.Sprintf("%s/%d", chat.Username, messageId)
	}
	return
}

// NOTE: connection helper functions

// IsUserConnected checks if a user is connected to a chat and validates permissions.
// Handles both private messages (with connection system) and group messages.
// Returns the effective chat if all checks pass, nil otherwise.
func IsUserConnected(b *gotgbot.Bot, ctx *ext.Context, chatAdmin, botAdmin bool) (chat *gotgbot.Chat) {
	msg := ctx.EffectiveMessage
	user := ctx.EffectiveUser
	tr := i18n.MustNewTranslator(db.GetLanguage(ctx))

	if msg.Chat.Type == "private" {
		conn := db.Connection(user.Id)
		if conn.Connected && conn.ChatId != 0 {
			chatFullInfo, err := b.GetChat(conn.ChatId, nil)
			if err != nil {
				log.WithFields(log.Fields{
					"userId": user.Id,
					"chatId": conn.ChatId,
					"error":  err,
				}).Warn("Stale connection detected - chat no longer accessible")
				// Provide user feedback about stale connection
				text, _ := tr.GetString("connections_stale_connection")
				_, _ = msg.Reply(b, text, nil)
				return nil
			}
			_chat := chatFullInfo.ToChat() // need to convert to Chat type
			chat = &_chat
		} else {
			text, _ := tr.GetString("connections_is_user_connected_need_group")
			_, err := msg.Reply(b,
				text,
				&gotgbot.SendMessageOpts{
					ReplyParameters: &gotgbot.ReplyParameters{
						MessageId:                msg.MessageId,
						AllowSendingWithoutReply: true,
					},
				},
			)
			if err != nil {
				log.Error(err)
				return nil
			}

			return nil
		}
	} else {
		chat = ctx.EffectiveChat
	}
	if botAdmin {
		if !chat_status.IsUserAdmin(b, chat.Id, b.Id) {
			text, _ := tr.GetString("connections_is_user_connected_bot_not_admin")
			_, err := msg.Reply(b, text, Shtml())
			if err != nil {
				log.Error(err)
				return nil
			}

			return nil
		}
	}
	if chatAdmin {
		if !chat_status.IsUserAdmin(b, chat.Id, user.Id) {
			text, _ := tr.GetString("connections_is_user_connected_user_not_admin")
			_, err := msg.Reply(b, text, Shtml())
			if err != nil {
				log.Error(err)
				return nil
			}

			return nil
		}
	}
	ctx.EffectiveChat = chat
	return chat
}

// NOTE: keyboard helper functions

// BuildKeyboard constructs an inline keyboard from a slice of database button objects.
// Handles button grouping based on the SameLine property for proper layout.
func BuildKeyboard(buttons []db.Button) [][]gotgbot.InlineKeyboardButton {
	keyb := make([][]gotgbot.InlineKeyboardButton, 0)
	for _, btn := range buttons {
		if btn.SameLine && len(keyb) > 0 {
			keyb[len(keyb)-1] = append(keyb[len(keyb)-1], gotgbot.InlineKeyboardButton{Text: btn.Name, Url: btn.Url})
		} else {
			k := make([]gotgbot.InlineKeyboardButton, 1)
			k[0] = gotgbot.InlineKeyboardButton{Text: btn.Name, Url: btn.Url}
			keyb = append(keyb, k)
		}
	}
	return keyb
}

// ConvertButtonV2ToDbButton converts markdown parser button format to database button format.
// Maps ButtonV2 fields to corresponding db.Button fields.
func ConvertButtonV2ToDbButton(buttons []tgmd2html.ButtonV2) (btns []db.Button) {
	btns = make([]db.Button, len(buttons))
	for i, btn := range buttons {
		btns[i] = db.Button{
			Name:     btn.Name,
			Url:      btn.Content,
			SameLine: btn.SameLine,
		}
	}
	return
}

// RevertButtons converts database button format back to markdown button string format.
// Generates markdown buttonurl syntax for each button with proper same-line handling.
func RevertButtons(buttons []db.Button) string {
	res := ""
	for _, btn := range buttons {
		if btn.SameLine {
			res += fmt.Sprintf("\n[%s](buttonurl://%s:same)", btn.Name, btn.Url)
		} else {
			res += fmt.Sprintf("\n[%s](buttonurl://%s)", btn.Name, btn.Url)
		}
	}
	return res
}

// InlineKeyboardMarkupToTgmd2htmlButtonV2 converts Telegram inline keyboard to markdown button format.
// Filters out non-URL buttons and handles same-line button positioning.
func InlineKeyboardMarkupToTgmd2htmlButtonV2(replyMarkup *gotgbot.InlineKeyboardMarkup) (btns []tgmd2html.ButtonV2) {
	btns = make([]tgmd2html.ButtonV2, 0)
	for _, inlineKeyboard := range replyMarkup.InlineKeyboard {
		if len(inlineKeyboard) > 1 {
			for i, button := range inlineKeyboard {
				// if any button has anything other than url, it's not a valid button
				// skip options such as CallbackData, CallbackUrl, etc.
				if button.Url == "" {
					continue
				}

				sameline := true
				if i == 0 {
					sameline = false
				}
				btns = append(
					btns,
					tgmd2html.ButtonV2{
						Name:     button.Text,
						Content:  button.Url,
						SameLine: sameline,
					},
				)
			}
		} else {
			btns = append(btns,
				tgmd2html.ButtonV2{
					Name:     inlineKeyboard[0].Text,
					Content:  inlineKeyboard[0].Url,
					SameLine: false,
				},
			)
		}
	}
	return
}

// ChunkKeyboardSlices splits a slice of inline keyboard buttons into chunks of specified size.
// Used for creating organized help menu keyboards with consistent row layouts.
func ChunkKeyboardSlices(slice []gotgbot.InlineKeyboardButton, chunkSize int) (chunks [][]gotgbot.InlineKeyboardButton) {
	for len(slice) > 0 {
		if len(slice) < chunkSize {
			chunkSize = len(slice)
		}

		chunks = append(chunks, slice[0:chunkSize])
		slice = slice[chunkSize:]
	}
	return
}

// NOTE: language helper functions

// MakeLanguageKeyboard creates an inline keyboard with all available language options.
// Uses valid language codes from config and chunks them into 2-column layout.
func MakeLanguageKeyboard() [][]gotgbot.InlineKeyboardButton {
	var kb []gotgbot.InlineKeyboardButton

	for _, langCode := range config.AppConfig.ValidLangCodes {
		properLang := GetLangFormat(langCode)
		if properLang == "" || properLang == " " {
			continue
		}

		kb = append(
			kb,
			gotgbot.InlineKeyboardButton{
				Text:         properLang,
				CallbackData: callbackcodec.EncodeOrFallback("change_language", map[string]string{"l": langCode}, fmt.Sprintf("change_language.%s", langCode)),
			},
		)
	}

	return ChunkKeyboardSlices(kb, 2)
}

// GetLangFormat returns a formatted language display string with name and flag emoji.
// Uses i18n system to get localized language name and flag for the given language code.
func GetLangFormat(langCode string) string {
	tr := i18n.MustNewTranslator(langCode)
	langName, _ := tr.GetString("language_name")
	langFlag, _ := tr.GetString("language_flag")
	return langName + " " + langFlag
}

// NOTE: tgmd2html helper functions

// ReverseHTML2MD converts HTML-formatted text back to markdown format.
// Handles common HTML tags like bold, italic, underline, strikethrough, code, pre, and links.
func ReverseHTML2MD(text string) string {
	Html2MdMap := map[string]string{
		"i":    "_%s_",
		"u":    "__%s__",
		"b":    "*%s*",
		"s":    "~%s~",
		"code": "`%s`",
		"pre":  "```%s```",
		"a":    "[%s](%s)",
	}

	for i := range strings.SplitSeq(text, " ") {
		for htmlTag, keyValue := range Html2MdMap {
			k := ""
			// using this because <a> uses <href> tag
			if htmlTag == "a" {
				if linkRegex.MatchString(i) {
					matches := linkRegex.FindStringSubmatch(i)
					if len(matches) >= 3 {
						k = fmt.Sprintf(keyValue, matches[2], matches[1])
					}
				} else {
					continue
				}
			} else if regex, exists := htmlTagRegexes[htmlTag]; exists {
				if regex.MatchString(i) {
					k = fmt.Sprintf(keyValue, regex.ReplaceAllString(i, "$1"))
				} else {
					continue
				}
			}
			if k != "" {
				text = strings.ReplaceAll(text, i, k)
			}
		}
	}

	return text
}

// NOTE: formatting helper functions

// FormattingReplacer processes message text and replaces placeholders with actual user/chat data.
// Handles variables like {first}, {last}, {username}, {mention}, {count}, {chatname}, {id}.
// Also processes rules button insertion with various positioning options.
func FormattingReplacer(b *gotgbot.Bot, chat *gotgbot.Chat, user *gotgbot.User, oldMsg string, buttons []db.Button) (res string, btns []db.Button) {
	return FormattingReplacerWithLanguage(b, chat, user, oldMsg, buttons, "en")
}

// FormattingReplacerWithLanguage is like FormattingReplacer but accepts a language parameter for localization.
func FormattingReplacerWithLanguage(b *gotgbot.Bot, chat *gotgbot.Chat, user *gotgbot.User, oldMsg string, buttons []db.Button, language string) (res string, btns []db.Button) {
	var (
		firstName     string
		fullName      string
		username      string
		rulesBtnRegex = `(?s){rules(:(same|up))?}`
	)

	firstName = user.FirstName
	if len(user.FirstName) <= 0 {
		tr := i18n.MustNewTranslator(language)
		personNoName, _ := tr.GetString("helpers_person_no_name")
		if personNoName == "" {
			personNoName = "PersonWithNoName" // fallback
		}
		firstName = personNoName
	}

	if user.LastName != "" {
		fullName = firstName + " " + user.LastName
	} else {
		fullName = firstName
	}
	mention := MentionHtml(user.Id, firstName)

	if user.Username != "" {
		username = "@" + html.EscapeString(user.Username)
	} else {
		username = mention
	}

	// Only fetch member count if {count} placeholder is present
	countStr := "0" // Default value to avoid empty replacement
	if strings.Contains(oldMsg, "{count}") {
		if count, err := chat.GetMemberCount(b, nil); err == nil {
			countStr = strconv.Itoa(int(count))
		}
	}

	r := strings.NewReplacer(
		"{first}", html.EscapeString(firstName),
		"{last}", html.EscapeString(user.LastName),
		"{fullname}", html.EscapeString(fullName),
		"{username}", username,
		"{mention}", mention,
		"{count}", countStr,
		"{chatname}", html.EscapeString(chat.Title),
		"{id}", strconv.Itoa(int(user.Id)),
	)
	res = r.Replace(oldMsg)
	btns = buttons // copies the buttons over to format rules btn

	rulesDb := db.GetChatRulesInfo(chat.Id)
	rulesBtnText := rulesDb.RulesBtn
	if rulesBtnText == "" {
		tr := i18n.MustNewTranslator(language)
		defaultRulesText, _ := tr.GetString("button_rules_default")
		if defaultRulesText == "" {
			defaultRulesText = "Rules" // fallback
		}
		rulesBtnText = defaultRulesText
	}

	// only add rules btn when rules are added in chat
	if rulesDb.Rules != "" {
		pattern, err := regexp.Compile(rulesBtnRegex)
		if err != nil {
			log.Error(err)
		}
		if pattern.MatchString(res) {
			response := pattern.FindStringSubmatch(res)

			sameline := false
			if response[2] == "same" {
				sameline = true
			}

			rulesButton := db.Button{
				Name:     rulesBtnText,
				Url:      fmt.Sprintf("https://t.me/%s?start=rules_%d", b.Username, chat.Id),
				SameLine: sameline,
			}

			if response[2] == "up" {
				// this adds the button on top of all buttons
				btns = []db.Button{rulesButton}
				btns = append(btns, buttons...)
			} else {
				// this adds the button to bottom (default behaviour)
				btns = buttons
				btns = append(btns, rulesButton)

			}
			res = pattern.ReplaceAllString(res, "")
		}
	}

	return res, btns
}

// NOTE: extract status helper functions

// ExtractJoinLeftStatusChange analyzes ChatMemberUpdated events to detect join/leave status changes.
// Returns (was_member, is_member) booleans indicating membership status transition.
// Returns (false, false) for channels or if no status change occurred.
func ExtractJoinLeftStatusChange(u *gotgbot.ChatMemberUpdated) (bool, bool) {
	// return false for channels
	if u.Chat.Type == "channel" {
		return false, false
	}

	oldMemberStatus := u.OldChatMember.MergeChatMember().Status
	newMemberStatus := u.NewChatMember.MergeChatMember().Status
	oldIsMember := u.OldChatMember.MergeChatMember().IsMember
	newIsMember := u.NewChatMember.MergeChatMember().IsMember

	if oldMemberStatus == newMemberStatus {
		return false, false
	}

	wasMember := string_handling.FindInStringSlice(
		[]string{"member", "administrator", "creator"},
		oldMemberStatus,
	) || (oldMemberStatus == "restricted" && oldIsMember)

	isMember := string_handling.FindInStringSlice(
		[]string{"member", "administrator", "creator"},
		newMemberStatus,
	) || (newMemberStatus == "restricted" && newIsMember)

	return wasMember, isMember
}

// ExtractAdminUpdateStatusChange detects admin status changes from ChatMemberUpdated events.
// Returns true if there was a transition to/from administrator or creator status.
// Returns false for channels or if no admin status change occurred.
func ExtractAdminUpdateStatusChange(u *gotgbot.ChatMemberUpdated) bool {
	// return false for channels
	if u.Chat.Type == "channel" {
		return false
	}

	oldMemberStatus := u.OldChatMember.MergeChatMember().Status
	newMemberStatus := u.NewChatMember.MergeChatMember().Status

	// status remains same
	if oldMemberStatus == newMemberStatus {
		return false
	}

	adminStatusChanged := (string_handling.FindInStringSlice(
		[]string{"administrator", "creator"},
		oldMemberStatus,
	) && !string_handling.FindInStringSlice(
		[]string{"administrator", "creator"},
		newMemberStatus,
	)) ||
		(string_handling.FindInStringSlice(
			[]string{"administrator", "creator"},
			newMemberStatus,
		) && !string_handling.FindInStringSlice(
			[]string{"administrator", "creator"},
			oldMemberStatus,
		))

	return adminStatusChanged
}

// NOTE: NoteWelcomeFilter helper functions

// GetNoteAndFilterType extracts and processes note or filter content from a Telegram message.
// Handles text, media files, and reply messages with button parsing and content validation.
// Returns parsed content with metadata like data type, buttons, and special options.
func GetNoteAndFilterType(msg *gotgbot.Message, isFilter bool, language string) (keyWord, fileid, text string, dataType int, buttons []db.Button, pvtOnly, grpOnly, adminOnly, webPrev, isProtected, noNotif bool, errorMsg string) {
	dataType = -1 // not defined datatype; invalid note
	tr := i18n.MustNewTranslator(language)

	if isFilter {
		errorMsg, _ = tr.GetString("helpers_need_filter_content")
		if errorMsg == "" {
			errorMsg = "You need to give the filter some content!" // fallback
		}
	} else {
		errorMsg, _ = tr.GetString("helpers_need_note_content")
		if errorMsg == "" {
			errorMsg = "You need to give the note some content!" // fallback
		}
	}

	var (
		rawText string
		args    = strings.Fields(msg.Text)[1:]
	)
	_buttons := make([]tgmd2html.ButtonV2, 0) // make a slice for buttons
	replyMsg := msg.ReplyToMessage

	// set rawText from helper function
	setRawText(msg, args, &rawText)

	// extract the noteword
	if len(args) >= 2 && replyMsg == nil {
		// Uses inline extraction to avoid circular dependency with the extraction package.
		if len(args) > 0 {
			keyWord = args[0]
			if len(args) > 1 {
				text = strings.Join(args[1:], " ")
			}
		}
		text, _buttons = tgmd2html.MD2HTMLButtonsV2(text)
		dataType = db.TEXT
	} else if replyMsg != nil && len(args) >= 1 {
		// Uses inline extraction to avoid circular dependency with the extraction package.
		keyWord = strings.Join(args, " ")

		if replyMsg.ReplyMarkup == nil {
			text, _buttons = tgmd2html.MD2HTMLButtonsV2(rawText)
		} else {
			text, _ = tgmd2html.MD2HTMLButtonsV2(rawText)
			_buttons = InlineKeyboardMarkupToTgmd2htmlButtonV2(replyMsg.ReplyMarkup)
		}

		if replyMsg.Text != "" {
			dataType = db.TEXT
		} else if replyMsg.Sticker != nil {
			fileid = replyMsg.Sticker.FileId
			dataType = db.STICKER
		} else if replyMsg.Document != nil {
			fileid = replyMsg.Document.FileId
			dataType = db.DOCUMENT
		} else if len(replyMsg.Photo) > 0 {
			fileid = replyMsg.Photo[len(replyMsg.Photo)-1].FileId // using -1 index to get best photo quality
			dataType = db.PHOTO
		} else if replyMsg.Audio != nil {
			fileid = replyMsg.Audio.FileId
			dataType = db.AUDIO
		} else if replyMsg.Voice != nil {
			fileid = replyMsg.Voice.FileId
			dataType = db.VOICE
		} else if replyMsg.Video != nil {
			fileid = replyMsg.Video.FileId
			dataType = db.VIDEO
		} else if replyMsg.VideoNote != nil {
			fileid = replyMsg.VideoNote.FileId
			dataType = db.VideoNote
		}
	}

	// pre-fix the data before sending it back
	preFixes(_buttons, keyWord, &text, &dataType, fileid, &buttons, &errorMsg, language)

	// return if datatype is invalid
	if dataType != -1 && !isFilter {
		// parse options such as pvtOnly, adminOnly, webPrev and replace them
		pvtOnly, grpOnly, adminOnly, webPrev, isProtected, noNotif, _ = notesParser(text)
	}

	return
}

// GetWelcomeType extracts and processes welcome/greeting content from a Telegram message.
// Similar to GetNoteAndFilterType but specifically for greeting messages.
// Returns processed content with data type, file ID, and buttons for the greeting.
func GetWelcomeType(msg *gotgbot.Message, greetingType string, language string) (text string, dataType int, fileid string, buttons []db.Button, errorMsg string) {
	dataType = -1
	tr := i18n.MustNewTranslator(language)
	template, _ := tr.GetString("helpers_need_content")
	if template == "" {
		template = "You need to give me some content to %s users!" // fallback
	}
	errorMsg = fmt.Sprintf(template, greetingType)
	var (
		rawText string
		args    = strings.Fields(msg.Text)[1:]
	)
	_buttons := make([]tgmd2html.ButtonV2, 0)
	replyMsg := msg.ReplyToMessage

	// set rawText from helper function
	setRawText(msg, args, &rawText)

	if len(args) >= 1 && msg.ReplyToMessage == nil {
		fileid = ""
		text, _buttons = tgmd2html.MD2HTMLButtonsV2(rawText)
		dataType = db.TEXT
	} else if msg.ReplyToMessage != nil {
		if replyMsg.ReplyMarkup == nil {
			text, _buttons = tgmd2html.MD2HTMLButtonsV2(rawText)
		} else {
			text, _ = tgmd2html.MD2HTMLButtonsV2(rawText)
			_buttons = InlineKeyboardMarkupToTgmd2htmlButtonV2(replyMsg.ReplyMarkup)
		}
		if len(args) == 0 && replyMsg.Text != "" {
			dataType = db.TEXT
		} else if replyMsg.Sticker != nil {
			fileid = replyMsg.Sticker.FileId
			dataType = db.STICKER
		} else if replyMsg.Document != nil {
			fileid = replyMsg.Document.FileId
			dataType = db.DOCUMENT
		} else if len(replyMsg.Photo) > 0 {
			fileid = replyMsg.Photo[len(replyMsg.Photo)-1].FileId
			dataType = db.PHOTO
		} else if replyMsg.Audio != nil {
			fileid = replyMsg.Audio.FileId
			dataType = db.AUDIO
		} else if replyMsg.Voice != nil {
			fileid = replyMsg.Voice.FileId
			dataType = db.VOICE
		} else if replyMsg.Video != nil {
			fileid = replyMsg.Video.FileId
			dataType = db.VIDEO
		} else if replyMsg.VideoNote != nil {
			fileid = replyMsg.VideoNote.FileId
			dataType = db.VideoNote
		}
	}

	// pre-fix the data before sending it back
	preFixes(_buttons, "Greeting", &text, &dataType, fileid, &buttons, &errorMsg, language)

	return
}

// SendFilter sends a filter message using the media package.
// Handles random message selection, formatting replacement, and keyboard building.
// Returns the sent message or an error.
func SendFilter(b *gotgbot.Bot, ctx *ext.Context, filterData *db.ChatFilters, replyMsgId int64) (*gotgbot.Message, error) {
	if filterData == nil {
		return nil, fmt.Errorf("filter data is nil")
	}
	chat := ctx.EffectiveChat

	var (
		buttons       []db.Button
		sent          string
		tmpfilterData db.ChatFilters
	)
	tmpfilterData = *filterData
	buttons = tmpfilterData.Buttons

	// Random data goes there
	rstrings := strings.Split(tmpfilterData.FilterReply, "%%%")
	if len(rstrings) == 1 {
		sent = rstrings[0]
	} else {
		n := rand.Intn(len(rstrings)) // #nosec G404 - Non-cryptographic random is sufficient for selecting messages
		sent = rstrings[n]
	}

	tmpfilterData.FilterReply, buttons = FormattingReplacer(b, chat, ctx.EffectiveUser, sent, buttons)
	keyb := BuildKeyboard(buttons)
	keyboard := gotgbot.InlineKeyboardMarkup{InlineKeyboard: keyb}

	return media.SendFilter(b, ctx.Message.Chat.Id, &tmpfilterData, &keyboard, replyMsgId, ctx.Message.MessageThreadId)
}

// notesParser parses special note options from message text using regex patterns.
// Detects {private}, {noprivate}, {admin}, {preview}, {protect}, {nonotif} tags.
// Returns boolean flags for each option and the text with tags removed.
func notesParser(sent string) (pvtOnly, grpOnly, adminOnly, webPrev, protectedContent, noNotif bool, sentBack string) {
	pvtOnly, err := regexp.MatchString(`({private})`, sent)
	if err != nil {
		log.Error(err)
		return
	}

	grpOnly, err = regexp.MatchString(`({noprivate})`, sent)
	if err != nil {
		log.Error(err)
		return
	}

	adminOnly, err = regexp.MatchString(`({admin})`, sent)
	if err != nil {
		log.Error(err)
		return
	}

	webPrev, err = regexp.MatchString(`({preview})`, sent)
	if err != nil {
		log.Error(err)
		return
	}

	protectedContent, err = regexp.MatchString(`({protect})`, sent)
	if err != nil {
		log.Error(err)
		return
	}

	noNotif, err = regexp.MatchString(`({nonotif})`, sent)
	if err != nil {
		log.Error(err)
		return
	}

	sent = strings.NewReplacer(
		"{private}", "",
		"{admin}", "",
		"{preview}", "",
		"{noprivate}", "",
		"{protect}", "",
		"{nonotif}", "",
	).Replace(sent)

	return pvtOnly, grpOnly, adminOnly, webPrev, protectedContent, noNotif, sent
}

// SendNote sends a note message using the media package.
// Handles random message selection, formatting replacement, option parsing, and keyboard building.
// Returns the sent message or an error.
func SendNote(b *gotgbot.Bot, chat *gotgbot.Chat, ctx *ext.Context, noteData *db.Notes, replyMsgId int64) (*gotgbot.Message, error) {
	var (
		buttons []db.Button
		sent    string
	)

	// copy just in case
	buttons = noteData.Buttons

	// Random data goes there
	rstrings := strings.Split(noteData.NoteContent, "%%%")
	if len(rstrings) == 1 {
		sent = rstrings[0]
	} else {
		n := rand.Intn(len(rstrings)) // #nosec G404 - Non-cryptographic random is sufficient for selecting messages
		sent = rstrings[n]
	}

	noteData.NoteContent, buttons = FormattingReplacer(b, chat, ctx.EffectiveUser, sent, buttons)
	// below is an additional step, need to remove it
	_, _, _, _, _, _, noteData.NoteContent = notesParser(noteData.NoteContent) // replaces the text
	keyb := BuildKeyboard(buttons)
	keyboard := gotgbot.InlineKeyboardMarkup{InlineKeyboard: keyb}
	// using false as last arg to format the note
	return media.SendNote(b, ctx.Message.Chat.Id, noteData, &keyboard, replyMsgId, ctx.Message.MessageThreadId)
}

// preFixes validates and preprocesses message content before database storage.
// Checks message length limits using UTF-8 character count (not bytes), validates button URLs,
// sets default button names, and filters invalid content. Modifies parameters by reference.
func preFixes(buttons []tgmd2html.ButtonV2, defaultNameButton string, text *string, dataType *int, fileid string, dbButtons *[]db.Button, errorMsg *string, language string) {
	tr := i18n.MustNewTranslator(language)

	// Use utf8.RuneCountInString to count UTF-8 characters instead of len() for bytes
	textRuneCount := utf8.RuneCountInString(*text)

	if *dataType == db.TEXT && textRuneCount > 4096 {
		*dataType = -1
		template, _ := tr.GetString("helpers_text_too_long")
		*errorMsg = fmt.Sprintf(template, textRuneCount)
	} else if *dataType != db.TEXT && textRuneCount > 1024 {
		*dataType = -1
		template, _ := tr.GetString("helpers_caption_too_long")
		*errorMsg = fmt.Sprintf(template, textRuneCount)
	} else {
		for i, button := range buttons {
			if button.Name == "" {
				buttons[i].Name = defaultNameButton
			}
		}

		// buttonUrlFixer filters out non-URL buttons from the keyboard, keeping only valid URL buttons.
		buttonUrlFixer := func(_buttons *[]tgmd2html.ButtonV2) {
			// regex taken from https://regexr.com/39nr7
			buttonUrlPattern, _ := regexp.Compile(`[(htps)?:/w.a-zA-Z\d@%_+~#=]{2,256}\.[a-z]{2,6}\b([-a-zA-Z\d@:%_+.~#?&/=]*)`)
			// Build a new slice instead of modifying during iteration
			validButtons := make([]tgmd2html.ButtonV2, 0, len(*_buttons))
			for _, btn := range *_buttons {
				if buttonUrlPattern.MatchString(btn.Content) {
					validButtons = append(validButtons, btn)
				}
			}
			*_buttons = validButtons
		}

		buttonUrlFixer(&buttons)
		*dbButtons = ConvertButtonV2ToDbButton(buttons)

		// trim the characters \n, \t, \r and space from the text
		// also, set the dataType to -1 to make note invalid
		*text = strings.Trim(*text, "\n\t\r ")
		if *text == "" && fileid == "" {
			*dataType = -1
		}
	}
}

// setRawText extracts raw markdown text from a Telegram message.
// Handles both direct message text/caption and replied message content.
// Sets rawText parameter by reference with the extracted content.
func setRawText(msg *gotgbot.Message, args []string, rawText *string) {
	replyMsg := msg.ReplyToMessage
	if replyMsg == nil {
		if msg.Text == "" && msg.Caption != "" {
			parts := strings.SplitN(msg.OriginalCaptionMDV2(), " ", 2)
			if len(parts) >= 2 {
				*rawText = parts[1] // remove the command
			}
		} else if msg.Text != "" && msg.Caption == "" {
			parts := strings.SplitN(msg.OriginalMDV2(), " ", 2)
			if len(parts) >= 2 {
				*rawText = parts[1] // remove the command
			}
		}
	} else {
		if replyMsg.Text == "" && replyMsg.Caption != "" {
			*rawText = replyMsg.OriginalCaptionMDV2()
		} else if replyMsg.Caption == "" && len(args) >= 2 {
			parts := strings.SplitN(msg.OriginalMDV2(), " ", 3)
			if len(parts) >= 3 {
				*rawText = parts[2] // remove the command and first arg
			}
		} else {
			*rawText = replyMsg.OriginalMDV2()
		}
	}
}
