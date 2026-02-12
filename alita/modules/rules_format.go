package modules

import (
	"regexp"
	"strings"

	tgmd2html "github.com/PaulSonOfLars/gotg_md2html"
)

var htmlTagPattern = regexp.MustCompile(`(?i)<\/?[a-z][^>]*>`)

// normalizeRulesForHTML ensures legacy markdown rules render correctly in HTML mode.
func normalizeRulesForHTML(rawRules string) string {
	trimmed := strings.TrimSpace(rawRules)
	if trimmed == "" {
		return ""
	}
	if htmlTagPattern.MatchString(trimmed) {
		return rawRules
	}
	return tgmd2html.MD2HTMLV2(rawRules)
}
