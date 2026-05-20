// Package formatting provides text-formatting helpers for Telegram messages,
// including HTML / Markdown option builders, message splitting, HTML↔Markdown
// conversion, and user/chat placeholder replacement.
package formatting

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

// Parse-mode constants and the Telegram message length limit.
const (
	Markdown             = "Markdown"
	HTML                 = "HTML"
	None                 = "None"
	MaxMessageLength int = 4096
)

// precompiled regexes and replacer for ReverseHTML2MD.
var (
	linkRegex = regexp.MustCompile(`<a href="(.*?)">(.*?)</a>`)
	// htmlToMdReplacer efficiently replaces HTML tags with Markdown in a single pass.
	htmlToMdReplacer = strings.NewReplacer(
		"<b>", "*",
		"</b>", "*",
		"<i>", "_",
		"</i>", "_",
		"<u>", "__",
		"</u>", "__",
		"<s>", "~",
		"</s>", "~",
		"<code>", "`",
		"</code>", "`",
		"<pre>", "```",
		"</pre>", "```",
	)
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
	if utf8.RuneCountInString(msg) <= MaxMessageLength {
		return []string{msg}
	}

	lines := strings.Split(msg, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	smallMsg := ""
	result := make([]string, 0)

	for _, line := range lines {
		potentialMsg := smallMsg + line + "\n"
		if utf8.RuneCountInString(potentialMsg) <= MaxMessageLength {
			smallMsg = potentialMsg
			continue
		}

		if utf8.RuneCountInString(line) > MaxMessageLength {
			if smallMsg != "" {
				result = append(result, smallMsg)
				smallMsg = ""
			}
			runes := []rune(line)
			for len(runes) > 0 {
				chunkSize := min(MaxMessageLength, len(runes))
				result = append(result, string(runes[:chunkSize]))
				runes = runes[chunkSize:]
			}
		} else {
			if smallMsg != "" {
				result = append(result, smallMsg)
			}
			smallMsg = line + "\n"
		}
	}

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
// Both the URL and name are HTML-escaped for safety.
func MentionUrl(url, name string) string {
	return fmt.Sprintf("<a href=\"%s\">%s</a>", html.EscapeString(url), html.EscapeString(name))
}

// HtmlEscape escapes special HTML characters in a string to prevent injection.
// Used when inserting untrusted content into HTML-formatted messages.
func HtmlEscape(s string) string {
	return html.EscapeString(s)
}

// ReverseHTML2MD converts HTML-formatted text back to markdown format.
// Handles common HTML tags like bold, italic, underline, strikethrough, code, pre, and links.
func ReverseHTML2MD(text string) string {
	if linkRegex.MatchString(text) {
		matches := linkRegex.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) >= 3 {
				oldLink := match[0]
				newLink := fmt.Sprintf("[%s](%s)", match[2], match[1])
				text = strings.Replace(text, oldLink, newLink, 1)
			}
		}
	}

	return htmlToMdReplacer.Replace(text)
}

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
		lastName      string
		fullName      string
		username      string
		userId        int64
		rulesBtnRegex = `(?s){rules(:(same|up))?}`
	)

	if user == nil {
		tr := i18n.MustNewTranslator(language)
		personNoName, _ := tr.GetString("helpers_person_no_name")
		if personNoName == "" {
			personNoName = "PersonWithNoName"
		}
		firstName = personNoName
		fullName = personNoName
		username = personNoName
		userId = 0
	} else {
		firstName = user.FirstName
		if len(user.FirstName) <= 0 {
			tr := i18n.MustNewTranslator(language)
			personNoName, _ := tr.GetString("helpers_person_no_name")
			if personNoName == "" {
				personNoName = "PersonWithNoName"
			}
			firstName = personNoName
		}

		lastName = user.LastName
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
		userId = user.Id
	}

	countStr := "0"
	if strings.Contains(oldMsg, "{count}") {
		if count, err := chat.GetMemberCount(b, nil); err == nil {
			countStr = strconv.Itoa(int(count))
		}
	}

	r := strings.NewReplacer(
		"{first}", html.EscapeString(firstName),
		"{last}", html.EscapeString(lastName),
		"{fullname}", html.EscapeString(fullName),
		"{username}", username,
		"{mention}", username,
		"{count}", countStr,
		"{chatname}", html.EscapeString(chat.Title),
		"{id}", strconv.Itoa(int(userId)),
	)
	res = r.Replace(oldMsg)
	btns = buttons

	rulesDb := db.GetChatRulesInfo(chat.Id)
	rulesBtnText := rulesDb.RulesBtn
	if rulesBtnText == "" {
		tr := i18n.MustNewTranslator(language)
		defaultRulesText, _ := tr.GetString("button_rules_default")
		if defaultRulesText == "" {
			defaultRulesText = "Rules"
		}
		rulesBtnText = defaultRulesText
	}

	if rulesDb.Rules != "" {
		pattern, err := regexp.Compile(rulesBtnRegex)
		if err != nil {
			log.Error(err)
			return res, btns
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
				btns = []db.Button{rulesButton}
				btns = append(btns, buttons...)
			} else {
				btns = buttons
				btns = append(btns, rulesButton)
			}
			res = pattern.ReplaceAllString(res, "")
		}
	}

	return res, btns
}
