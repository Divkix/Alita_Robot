package modules

import (
	"strings"
	"unicode/utf8"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

// buildModerationMatchText builds a unified searchable string for moderation.
// It includes message text, caption text, and URL entities from both.
func buildModerationMatchText(msg *gotgbot.Message) string {
	if msg == nil {
		return ""
	}

	// No entities means no URL can be extracted: skip the slice/map allocations
	// and reproduce the trim + dedupe of Text and Caption directly.
	if len(msg.Entities) == 0 && len(msg.CaptionEntities) == 0 {
		text := strings.TrimSpace(msg.Text)
		caption := strings.TrimSpace(msg.Caption)
		switch {
		case text == "":
			return caption
		case caption == "" || caption == text:
			return text
		default:
			return text + "\n" + caption
		}
	}

	parts := make([]string, 0, 4)
	seen := make(map[string]struct{})
	appendUnique := func(s string) {
		trimmed := strings.TrimSpace(s)
		if trimmed == "" {
			return
		}
		if _, ok := seen[trimmed]; ok {
			return
		}
		seen[trimmed] = struct{}{}
		parts = append(parts, trimmed)
	}

	appendUnique(msg.Text)
	appendUnique(msg.Caption)

	for _, url := range extractEntityURLs(msg.Text, msg.Entities) {
		appendUnique(url)
	}
	for _, url := range extractEntityURLs(msg.Caption, msg.CaptionEntities) {
		appendUnique(url)
	}

	return strings.Join(parts, "\n")
}

func extractEntityURLs(source string, entities []gotgbot.MessageEntity) []string {
	if len(entities) == 0 {
		return nil
	}

	// Allocate lazily: most messages carry entities that are not URLs.
	var urls []string
	for _, entity := range entities {
		if entity.Url != "" {
			urls = append(urls, entity.Url)
			continue
		}
		// "url" entities store raw links in the text/caption itself.
		if entity.Type == "url" {
			if extracted := extractEntityText(source, entity.Offset, entity.Length); extracted != "" {
				urls = append(urls, extracted)
			}
		}
	}
	if urls == nil {
		return []string{}
	}
	return urls
}

// extractEntityText returns the UTF-16 code-unit range [offset, offset+length)
// of source. The walk is single pass: encoding the whole source per entity is
// wasteful for messages with many links. Boundary handling matches
// utf16.Encode/Decode round tripping, including code units that split a
// surrogate pair (those decode to U+FFFD).
func extractEntityText(source string, offset, length int64) string {
	if source == "" || offset < 0 || length <= 0 {
		return ""
	}

	end := offset + length
	var (
		unit int64
		b    strings.Builder
	)
	// Code units never outnumber source bytes, so this stays bounded.
	b.Grow(int(min(length, int64(len(source)))))
	for _, r := range source {
		if unit >= end {
			return b.String()
		}

		width := int64(1)
		if r > 0xFFFF {
			width = 2
		}

		covered := min(unit+width, end) - max(unit, offset)
		switch {
		case covered == width:
			b.WriteRune(r)
		case covered > 0:
			b.WriteRune(utf8.RuneError)
		}
		unit += width
	}

	// Whole source walked: apply the same range check utf16.Encode would have.
	if offset >= unit || length > unit-offset {
		return ""
	}
	return b.String()
}
