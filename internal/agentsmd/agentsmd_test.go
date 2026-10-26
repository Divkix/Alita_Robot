// Package agentsmd checks that AGENTS.md stays short and names only things the
// repository still has. Rules that belong to one package live in its doc
// comments, where they change with the code; this file is what keeps them
// from drifting back.
package agentsmd

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// maxLines is the AGENTS.md budget. Past it, move a rule into the doc comment
// of the code it governs instead of raising the number.
const maxLines = 175

const root = "../.."

var (
	codeSpan   = regexp.MustCompile("`([^`\n]+)`")
	identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*(\(\))?$`)
	word       = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
	path       = regexp.MustCompile(`^[A-Za-z0-9_.-]+(/[A-Za-z0-9_.*-]*)+$`)
)

func readAgentsMd(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	return string(data)
}

func TestAgentsMdFitsBudget(t *testing.T) {
	if n := strings.Count(readAgentsMd(t), "\n"); n > maxLines {
		t.Errorf("AGENTS.md has %d lines, budget %d: move package-specific rules into that package's doc comments", n, maxLines)
	}
}

// TestAgentsMdNamesExist fails when an inline code span in AGENTS.md names an
// identifier no source file contains, or a path that does not exist.
func TestAgentsMdNamesExist(t *testing.T) {
	words := sourceWords(t)
	for _, m := range codeSpan.FindAllStringSubmatch(withoutFences(readAgentsMd(t)), -1) {
		span := m[1]
		switch {
		case strings.HasPrefix(span, "github.com/"):
			// The module path, not a file.
		case path.MatchString(span):
			if !pathExists(span, words) {
				t.Errorf("AGENTS.md names path %q, which does not exist", span)
			}
		case identifier.MatchString(span):
			name := strings.TrimSuffix(span, "()")
			name = name[strings.LastIndex(name, ".")+1:]
			if !words[name] {
				t.Errorf("AGENTS.md names %q, which no source file contains", span)
			}
		}
	}
}

// pathExists reports whether span names a file or directory, glob patterns
// allowed, or a package directory followed by one of its identifiers
// (`internal/testdb.Run`).
func pathExists(span string, words map[string]bool) bool {
	if matches, err := filepath.Glob(filepath.Join(root, span)); err == nil && len(matches) != 0 {
		return true
	}
	dot := strings.LastIndex(span, ".")
	if dot < strings.LastIndex(span, "/") {
		return false
	}
	info, err := os.Stat(filepath.Join(root, span[:dot]))
	return err == nil && info.IsDir() && words[span[dot+1:]]
}

// withoutFences drops fenced code blocks: they hold shell commands, not names.
func withoutFences(doc string) string {
	var b strings.Builder
	fenced := false
	for line := range strings.SplitSeq(doc, "\n") {
		if strings.HasPrefix(line, "```") {
			fenced = !fenced
			continue
		}
		if !fenced {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// sourceWords collects every identifier-shaped word in the repository's Go,
// SQL, YAML and shell files.
func sourceWords(t *testing.T) map[string]bool {
	t.Helper()
	words := map[string]bool{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		switch filepath.Ext(p) {
		case ".go", ".sql", ".yml", ".yaml", ".sh":
		default:
			if d.Name() != "Makefile" {
				return nil
			}
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, w := range word.FindAll(data, -1) {
			words[string(w)] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repository: %v", err)
	}
	return words
}
