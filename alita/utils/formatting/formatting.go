package formatting

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/rules"
	"github.com/divkix/Alita_Robot/alita/i18n"
)

const (
	Markdown             = "Markdown"
	HTML                 = "HTML"
	MaxMessageLength int = 4096
)

var (
	linkRegex        = regexp.MustCompile(`<a href="(.*?)">(.*?)</a>`)
	rulesBtnRegex    = regexp.MustCompile(`(?s){rules(:(same|up))?}`)
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
	formatPlaceholderTokens = [...]string{
		"{first}", "{last}", "{fullname}", "{username}", "{mention}", "{count}", "{chatname}", "{id}",
	}
)

type memberCountEntry struct {
	count int
	at    time.Time
}

const defaultMemberCountCacheTTL = 60 * time.Second

var (
	memberCountCache    sync.Map
	memberCountCacheTTL = defaultMemberCountCacheTTL
)

func cachedMemberCount(b *gotgbot.Bot, chat *gotgbot.Chat) string {
	if v, ok := memberCountCache.Load(chat.Id); ok {
		if e, ok := v.(memberCountEntry); ok && time.Since(e.at) < memberCountCacheTTL {
			return strconv.Itoa(e.count)
		}
		memberCountCache.Delete(chat.Id)
	}
	count, err := chat.GetMemberCount(b, nil)
	if err != nil {
		return "0"
	}
	entry := memberCountEntry{count: int(count), at: time.Now()}
	memberCountCache.Store(chat.Id, entry)
	expireMemberCount(chat.Id, entry)
	return strconv.Itoa(int(count))
}

func expireMemberCount(chatID int64, entry memberCountEntry) {
	time.AfterFunc(memberCountCacheTTL, func() {
		memberCountCache.CompareAndDelete(chatID, entry)
	})
}

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

// UTF16Len returns s in Telegram's length unit: UTF-16 code units.
func UTF16Len(s string) int {
	n := 0
	for _, r := range s {
		if r > 0xFFFF {
			n += 2
			continue
		}
		n++
	}
	return n
}

// SplitByUTF16Limit cuts s into pieces of at most limit UTF-16 units. It cuts
// between runes, so a surrogate pair is never split. limit must be at least 2.
func SplitByUTF16Limit(s string, limit int) []string {
	var pieces []string
	start, units := 0, 0
	for i, r := range s {
		w := 1
		if r > 0xFFFF {
			w = 2
		}
		if units+w > limit {
			pieces = append(pieces, s[start:i])
			start, units = i, 0
		}
		units += w
	}
	return append(pieces, s[start:])
}

func SplitMessage(msg string) []string {
	if UTF16Len(msg) <= MaxMessageLength {
		return []string{msg}
	}

	lines := strings.Split(msg, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	result := make([]string, 0, len(lines))

	smallMsg := ""
	smallMsgUnits := 0
	flush := func() {
		if smallMsg != "" {
			result = append(result, smallMsg)
			smallMsg = ""
			smallMsgUnits = 0
		}
	}

	for _, line := range lines {
		lineUnits := UTF16Len(line)

		if smallMsgUnits+lineUnits+1 <= MaxMessageLength {
			smallMsg += line + "\n"
			smallMsgUnits += lineUnits + 1
			continue
		}

		flush()

		if lineUnits < MaxMessageLength {
			smallMsg = line + "\n"
			smallMsgUnits = lineUnits + 1
			continue
		}

		result = append(result, SplitByUTF16Limit(line, MaxMessageLength)...)
	}

	flush()

	return result
}

func MentionHtml(userId int64, name string) string {
	return MentionUrl(fmt.Sprintf("tg://user?id=%d", userId), name)
}

func MentionUrl(url, name string) string {
	return fmt.Sprintf("<a href=\"%s\">%s</a>", html.EscapeString(url), html.EscapeString(name))
}

func HtmlEscape(s string) string {
	return html.EscapeString(s)
}

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

// replaceFormatPlaceholders substitutes formatPlaceholderTokens with values in a
// single left-to-right pass; inserted values are never rescanned, matching the
// strings.Replacer semantics it replaces.
func replaceFormatPlaceholders(s string, values [len(formatPlaceholderTokens)]string) string {
	var b strings.Builder
	b.Grow(len(s))

	for {
		i := strings.IndexByte(s, '{')
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i])
		s = s[i:]

		matched := false
		for j := range len(formatPlaceholderTokens) {
			token := formatPlaceholderTokens[j]
			if strings.HasPrefix(s, token) {
				b.WriteString(values[j])
				s = s[len(token):]
				matched = true
				break
			}
		}
		if !matched {
			b.WriteByte('{')
			s = s[1:]
		}
	}
}

// chatLanguage is the language of the chat passed to FormattingReplacer: the
// user's own language when that chat is a PM, the group's otherwise. A note or
// greeting preview of a connected group, shown in the admin's PM, passes the
// group, so it renders in the group's language. It is only read when a
// fallback label is needed.
func chatLanguage(chat *gotgbot.Chat) string {
	if chat == nil {
		return "en"
	}
	if chat.Type == "private" {
		return lang.GetUserLanguage(context.Background(), chat.Id)
	}
	return lang.GetGroupLanguage(context.Background(), chat.Id)
}

func FormattingReplacer(b *gotgbot.Bot, chat *gotgbot.Chat, user *gotgbot.User, oldMsg string, buttons []db.Button) (res string, btns []db.Button) {
	if strings.IndexByte(oldMsg, '{') < 0 {
		// No '{' means no placeholder can match: the body is returned untouched.
		return oldMsg, buttons
	}

	var (
		firstName string
		lastName  string
		fullName  string
		username  string
		userId    int64
	)

	if user == nil {
		tr := i18n.MustNewTranslator(chatLanguage(chat))
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
			tr := i18n.MustNewTranslator(chatLanguage(chat))
			personNoName, _ := tr.GetString("helpers_person_no_name")
			if personNoName == "" {
				personNoName = "PersonWithNoName"
			}
			firstName = personNoName
		}

		lastName = user.LastName
		fullName = GetFullName(firstName, user.LastName)
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
		countStr = cachedMemberCount(b, chat)
	}

	values := [len(formatPlaceholderTokens)]string{
		html.EscapeString(firstName),
		html.EscapeString(lastName),
		html.EscapeString(fullName),
		username,
		username,
		countStr,
		html.EscapeString(chat.Title),
		strconv.Itoa(int(userId)),
	}

	response := rulesBtnRegex.FindStringSubmatch(oldMsg)
	if response == nil {
		return replaceFormatPlaceholders(oldMsg, values), buttons
	}

	res = replaceFormatPlaceholders(rulesBtnRegex.ReplaceAllString(oldMsg, ""), values)
	btns = append([]db.Button(nil), buttons...)

	rulesDb := rules.GetChatRulesInfo(chat.Id)
	if rulesDb.Rules == "" {
		return res, btns
	}

	rulesBtnText := rulesDb.RulesBtn
	if rulesBtnText == "" {
		tr := i18n.MustNewTranslator(chatLanguage(chat))
		defaultRulesText, _ := tr.GetString("button_rules_default")
		if defaultRulesText == "" {
			defaultRulesText = "Rules"
		}
		rulesBtnText = defaultRulesText
	}

	sameline := response[2] == "same"
	rulesButton := db.Button{
		Name:     rulesBtnText,
		Url:      fmt.Sprintf("https://t.me/%s?start=rules_%d", b.Username, chat.Id),
		SameLine: sameline,
	}

	if response[2] == "up" {
		btns = append([]db.Button{rulesButton}, buttons...)
	} else {
		btns = append(btns, rulesButton)
	}

	return res, btns
}

func GetFullName(firstName, lastName string) string {
	if lastName != "" {
		return firstName + " " + lastName
	}
	return firstName
}
