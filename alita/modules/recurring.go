package modules

import (
	"errors"
	"html"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	log "github.com/sirupsen/logrus"

	"github.com/divkix/Alita_Robot/alita/db"
	"github.com/divkix/Alita_Robot/alita/db/lang"
	"github.com/divkix/Alita_Robot/alita/db/models"
	"github.com/divkix/Alita_Robot/alita/db/recurringmessages"
	"github.com/divkix/Alita_Robot/alita/i18n"
	"github.com/divkix/Alita_Robot/alita/utils/content"
	"github.com/divkix/Alita_Robot/alita/utils/extraction"
	"github.com/divkix/Alita_Robot/alita/utils/formatting"
	"github.com/divkix/Alita_Robot/alita/utils/helpers"
)

var recurringModule = moduleStruct{
	moduleName: "Recurring",
}

// /recurring changes what the chat sees, so it takes the right /save and
// /setwelcome take. It is group-only: the message goes to the chat (and topic)
// the command is sent in.
var recurringDesc = helpers.CommandDescriptor{
	Name:        "recurring",
	Disableable: false,
	RequiredChecks: []helpers.CheckFunc{
		helpers.RequireGroup(),
		helpers.RequireUserAdmin(),
		helpers.CanUserChangeInfo(),
	},
}

// recurringNow is the clock /recurring add reads. Tests replace it; production
// never writes it after startup.
var recurringNow = time.Now

const (
	recurringTimeLayout = "2006-01-02 15:04"
	recurringPreviewLen = 40
)

// recurring handles /recurring: add, list (also the bare command) and rm.
func (moduleStruct) recurring(c *helpers.CommandContext) error {
	args := c.Ctx.Args()[1:]
	if len(args) == 0 {
		return recurringList(c)
	}
	switch strings.ToLower(args[0]) {
	case "list":
		return recurringList(c)
	case "add":
		return recurringAdd(c)
	case "rm", "remove", "del", "delete":
		if len(args) != 2 {
			return recurringUsage(c)
		}
		return recurringRemove(c, args[1])
	default:
		return recurringUsage(c)
	}
}

// recurringOptions are the settings given between the interval and the content.
type recurringOptions struct {
	start          *time.Time // only the clock time is used
	end            *time.Time
	deletePrevious bool
	threadID       int64
	// forum is whether the chat has topics; topic= is refused without them.
	forum bool
}

// recurringWord is one whitespace-delimited word of the command text, with its
// byte span.
type recurringWord struct {
	raw        string
	start, end int
}

func recurringWords(text string) []recurringWord {
	var words []recurringWord
	start := -1
	for i, r := range text {
		switch {
		case unicode.IsSpace(r) && start >= 0:
			words = append(words, recurringWord{raw: text[start:i], start: start, end: i})
			start = -1
		case !unicode.IsSpace(r) && start < 0:
			start = i
		}
	}
	if start >= 0 {
		words = append(words, recurringWord{raw: text[start:], start: start, end: len(text)})
	}
	return words
}

// recurringAdd handles /recurring add <interval> [options] <content>, where
// the content is a note's: text with buttons and fillings, or the replied
// message.
func recurringAdd(c *helpers.CommandContext) error {
	msg := c.Msg
	words := recurringWords(msg.Text)
	// words: /recurring add <interval> [options...] [content...]
	if len(words) < 3 {
		return recurringUsage(c)
	}

	seconds := extraction.ExtractDuration(c.Bot, c.Ctx, words[2].raw)
	if seconds < 0 {
		return ext.EndGroups // ExtractDuration replied
	}
	interval := time.Duration(seconds) * time.Second
	if interval < recurringmessages.MinInterval {
		return recurringReply(c, "recurring_interval_too_short", nil)
	}

	opts := recurringOptions{forum: c.Chat.IsForum}
	if msg.IsTopicMessage {
		opts.threadID = msg.MessageThreadId
	}
	next := 3
	for ; next < len(words); next++ {
		known, ok := opts.parse(words[next].raw)
		if !known {
			break
		}
		if !ok {
			return recurringReply(c, "recurring_invalid_option", i18n.TranslationParams{"option": html.EscapeString(words[next].raw)})
		}
	}

	// Cut "add" and the options so the notes parser sees
	// "/recurring <interval> <content>", the interval in the keyword's place.
	// Later spans first, so the earlier ones keep their offsets.
	stripped := msg
	for i := next - 1; i >= 3; i-- {
		stripped = stripPMGroupToken(stripped, pmGroupToken{start: words[i-1].end, end: words[i].end})
	}
	stripped = stripPMGroupToken(stripped, pmGroupToken{start: words[0].end, end: words[1].end})

	if msg.ReplyToMessage == nil && next >= len(words) {
		return recurringReply(c, "recurring_need_content", nil)
	}
	language := lang.GetLanguage(c.Ctx)
	extracted := content.ExtractNoteAndFilter(stripped, false, language)
	if extracted.DataType == -1 {
		noContent, _ := i18n.MustNewTranslator(language).GetString("content_need_note_content")
		if extracted.ErrorMsg == "" || extracted.ErrorMsg == noContent {
			return recurringReply(c, "recurring_need_content", nil)
		}
		_, err := msg.Reply(c.Bot, extracted.ErrorMsg, formatting.Shtml())
		return err
	}

	now := recurringNow().UTC()
	firstRun := now.Add(interval)
	if opts.start != nil {
		firstRun = time.Date(now.Year(), now.Month(), now.Day(), opts.start.Hour(), opts.start.Minute(), 0, 0, time.UTC)
		if !firstRun.After(now) {
			firstRun = firstRun.AddDate(0, 0, 1)
		}
	}
	if opts.end != nil && opts.end.Before(firstRun) {
		return recurringReply(c, "recurring_end_before_start", nil)
	}

	row := &models.RecurringMessage{
		ChatID:         c.Chat.Id,
		ThreadID:       opts.threadID,
		Text:           extracted.Text,
		FileID:         extracted.FileID,
		MsgType:        extracted.DataType,
		Buttons:        extracted.Buttons,
		IntervalSecs:   seconds,
		NextRunAt:      firstRun,
		EndsAt:         opts.end,
		DeletePrevious: opts.deletePrevious,
		CreatedBy:      c.User.Id,
	}
	if err := recurringmessages.Add(row); err != nil {
		if errors.Is(err, recurringmessages.ErrLimitReached) {
			return recurringReply(c, "recurring_limit_reached", i18n.TranslationParams{"max": recurringmessages.MaxPerChat})
		}
		log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).Error("[Recurring] could not save a recurring message")
		return recurringReply(c, "error_generic", nil)
	}

	lines := []string{
		recurringString(c.Tr, "recurring_added", i18n.TranslationParams{"id": row.ID}),
		recurringItemLine(c.Tr, row),
	}
	_, err := msg.Reply(c.Bot, strings.Join(lines, "\n"), formatting.Shtml())
	return err
}

// parse applies one option word. known is false for a word that is not an
// option (the content starts there); ok is false for an option with a bad
// value.
func (o *recurringOptions) parse(word string) (known, ok bool) {
	key, value, found := strings.Cut(word, "=")
	if !found {
		return false, false
	}
	switch strings.ToLower(key) {
	case "start":
		t, err := time.Parse("15:04", value)
		if err != nil {
			return true, false
		}
		o.start = &t
	case "end":
		end, err := parseRecurringEnd(value)
		if err != nil {
			return true, false
		}
		o.end = &end
	case "delete":
		on, valid := parseToggleArg(value)
		if !valid {
			return true, false
		}
		o.deletePrevious = on
	case "topic":
		id, err := strconv.ParseInt(value, 10, 64)
		if err != nil || id <= 0 || !o.forum {
			return true, false
		}
		o.threadID = id
	default:
		return false, false
	}
	return true, true
}

// parseRecurringEnd reads end=YYYY-MM-DD (the end of that day) or
// end=YYYY-MM-DDTHH:MM, both UTC.
func parseRecurringEnd(value string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02T15:04", value); err == nil {
		return t.UTC(), nil
	}
	day, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, err
	}
	return day.AddDate(0, 0, 1).Add(-time.Second).UTC(), nil
}

func recurringList(c *helpers.CommandContext) error {
	rows, err := recurringmessages.List(c.Chat.Id)
	if err != nil {
		log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).Error("[Recurring] could not list recurring messages")
		return recurringReply(c, "error_generic", nil)
	}
	if len(rows) == 0 {
		return recurringReply(c, "recurring_list_empty", nil)
	}

	lines := []string{recurringString(c.Tr, "recurring_list_title", i18n.TranslationParams{
		"count": len(rows),
		"max":   recurringmessages.MaxPerChat,
	})}
	for i := range rows {
		lines = append(lines, "", recurringItemLine(c.Tr, &rows[i]), recurringPreview(c.Tr, &rows[i]))
	}
	lines = append(lines, "", recurringString(c.Tr, "recurring_list_hint", nil))
	_, err = c.Msg.Reply(c.Bot, strings.Join(lines, "\n"), formatting.Shtml())
	return err
}

// recurringItemLine describes a message's schedule: its ID, interval, next
// run and options.
func recurringItemLine(tr *i18n.Translator, row *models.RecurringMessage) string {
	parts := []string{recurringString(tr, "recurring_list_item", i18n.TranslationParams{
		"id":       row.ID,
		"interval": formatDuration(int(row.IntervalSecs)),
		"next":     row.NextRunAt.UTC().Format(recurringTimeLayout),
	})}
	if row.EndsAt != nil {
		parts = append(parts, recurringString(tr, "recurring_list_item_until", i18n.TranslationParams{
			"end": row.EndsAt.UTC().Format(recurringTimeLayout),
		}))
	}
	if row.ThreadID != 0 {
		parts = append(parts, recurringString(tr, "recurring_list_item_topic", i18n.TranslationParams{"topic": row.ThreadID}))
	}
	if row.DeletePrevious {
		parts = append(parts, recurringString(tr, "recurring_list_item_delete_previous", nil))
	}
	if !row.Enabled {
		parts = append(parts, recurringString(tr, "recurring_list_item_stopped", nil))
	}
	return strings.Join(parts, " · ")
}

var recurringTagPattern = regexp.MustCompile(`<[^>]*>`)

// recurringPreview is the start of the message's text without formatting, or
// the media label for media without a caption.
func recurringPreview(tr *i18n.Translator, row *models.RecurringMessage) string {
	text := strings.Join(strings.Fields(html.UnescapeString(recurringTagPattern.ReplaceAllString(row.Text, ""))), " ")
	if runes := []rune(text); len(runes) > recurringPreviewLen {
		text = string(runes[:recurringPreviewLen]) + "…"
	}
	if row.MsgType != db.TEXT {
		media := recurringString(tr, "recurring_list_media", nil)
		if text == "" {
			return media
		}
		return media + " " + html.EscapeString(text)
	}
	return "<i>" + html.EscapeString(text) + "</i>"
}

func recurringRemove(c *helpers.CommandContext, arg string) error {
	id, err := strconv.ParseInt(strings.TrimPrefix(arg, "#"), 10, 64)
	if err != nil || id <= 0 {
		return recurringUsage(c)
	}
	removed, err := recurringmessages.Remove(c.Chat.Id, id)
	if err != nil {
		log.WithFields(log.Fields{"chat_id": c.Chat.Id, "error": err}).Error("[Recurring] could not remove a recurring message")
		return recurringReply(c, "error_generic", nil)
	}
	if !removed {
		return recurringReply(c, "recurring_not_found", i18n.TranslationParams{"id": id})
	}
	return recurringReply(c, "recurring_removed", i18n.TranslationParams{"id": id})
}

func recurringUsage(c *helpers.CommandContext) error {
	usage, _ := c.Tr.GetString("recurring_usage")
	return replyCommandHelp(c.Bot, c.Msg, c.Tr, "recurring", usage)
}

func recurringString(tr *i18n.Translator, key string, params i18n.TranslationParams) string {
	text, _ := tr.GetString(key, params)
	return text
}

func recurringReply(c *helpers.CommandContext, key string, params i18n.TranslationParams) error {
	_, err := c.Msg.Reply(c.Bot, recurringString(c.Tr, key, params), formatting.Shtml())
	return err
}

func LoadRecurring(dispatcher *ext.Dispatcher) {
	SetModuleEnabled(recurringModule.moduleName, true)

	helpers.WrapCommand(dispatcher, recurringDesc, recurringModule.recurring)
	RegisterAnonymousAdminHandler("recurring", anonCommandContextHandler(recurringDesc, recurringModule.recurring))
}

func init() {
	RegisterLegacyModule("Recurring", 226, LoadRecurring)
}
