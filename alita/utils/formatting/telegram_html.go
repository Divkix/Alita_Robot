package formatting

import (
	"html"
	"regexp"
	"strings"

	tgmd2html "github.com/PaulSonOfLars/gotg_md2html"
)

// telegramHTMLTagRe matches Telegram-supported HTML tags, including optional
// attributes. It is a closed allowlist so command placeholders such as
// <keyword> or <reply/username> are not treated as markup.
var telegramHTMLTagRe = regexp.MustCompile(
	`(?i)</?(?:b|strong|i|em|u|ins|s|strike|del|code|pre|blockquote|tg-spoiler|span|tg-emoji|a)(?:\s[^>]*)?>`,
)

// telegramHTMLOpenTagRe matches opening Telegram HTML tags. Combined with
// telegramHTMLCloseTagRe, this avoids treating a markdown code span such as
// `</b>` as an HTML-authored string.
var telegramHTMLOpenTagRe = regexp.MustCompile(
	`(?i)<(?:b|strong|i|em|u|ins|s|strike|del|code|pre|blockquote|tg-spoiler|span|tg-emoji|a)(?:\s[^>]*)?>`,
)

// telegramHTMLCloseTagRe matches closing Telegram HTML tags. Markdown help
// uses placeholders like <trigger>, which would otherwise look like opening
// tags, so a closer alone is not enough to select the HTML path.
var telegramHTMLCloseTagRe = regexp.MustCompile(
	`(?i)</(?:b|strong|i|em|u|ins|s|strike|del|code|pre|blockquote|tg-spoiler|span|tg-emoji|a)>`,
)

// htmlEntityRe matches existing HTML entities so preserveTelegramHTML does not
// double-escape &amp; / &#123; / &#x1F; into visible entity text.
var htmlEntityRe = regexp.MustCompile(`&(?:#\d+|#[xX][0-9a-fA-F]+|[a-zA-Z][a-zA-Z0-9]+);`)

// ToTelegramHTML converts locale or template text to Telegram HTML.
// Markdown is converted with MD2HTMLV2. Strings that already contain Telegram
// HTML tags keep those tags and only escape leftover <, >, and & so
// placeholders like <keyword> display as text instead of being parsed as tags.
// Leading and trailing newlines are preserved because MD2HTMLV2 strips them,
// and the help header relies on a trailing blank line before the body.
func ToTelegramHTML(s string) string {
	if s == "" {
		return ""
	}
	leading, core, trailing := splitEdgeNewlines(s)
	if core == "" {
		return s
	}
	var out string
	if looksLikeTelegramHTML(core) {
		out = preserveTelegramHTML(core)
	} else {
		out = tgmd2html.MD2HTMLV2(core)
	}
	return leading + out + trailing
}

func looksLikeTelegramHTML(s string) bool {
	return telegramHTMLOpenTagRe.MatchString(s) && telegramHTMLCloseTagRe.MatchString(s)
}

func splitEdgeNewlines(s string) (leading, core, trailing string) {
	start := 0
	for start < len(s) && s[start] == '\n' {
		start++
	}
	end := len(s)
	for end > start && s[end-1] == '\n' {
		end--
	}
	return s[:start], s[start:end], s[end:]
}

// preserveTelegramHTML keeps Telegram HTML tags intact and HTML-escapes the
// text around them, leaving pre-existing entities unchanged.
func preserveTelegramHTML(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	last := 0
	for _, loc := range telegramHTMLTagRe.FindAllStringIndex(s, -1) {
		b.WriteString(escapeHTMLTextPreservingEntities(s[last:loc[0]]))
		b.WriteString(s[loc[0]:loc[1]])
		last = loc[1]
	}
	b.WriteString(escapeHTMLTextPreservingEntities(s[last:]))
	return b.String()
}

func escapeHTMLTextPreservingEntities(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	last := 0
	for _, loc := range htmlEntityRe.FindAllStringIndex(s, -1) {
		b.WriteString(html.EscapeString(s[last:loc[0]]))
		b.WriteString(s[loc[0]:loc[1]])
		last = loc[1]
	}
	b.WriteString(html.EscapeString(s[last:]))
	return b.String()
}
