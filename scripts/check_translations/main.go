package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

type TranslationKey struct {
	Key  string
	File string
	Line int
}

type MissingTranslation struct {
	Key   string
	Usage []string
}

func main() {
	fmt.Println("🔍 Checking translations...")
	fmt.Println()

	// Step 1: Extract all translation keys from Go files
	keys, err := extractTranslationKeys("../../alita")
	if err != nil {
		fmt.Printf("❌ Error extracting translation keys: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("📊 Found %d translation keys in codebase\n", len(keys))

	// Step 2: Load locale files
	locales, err := loadLocaleFiles("../../locales")
	if err != nil {
		fmt.Printf("❌ Error loading locale files: %v\n", err)
		os.Exit(1)
	}

	// Step 3: Check each locale for missing keys
	totalMissing := 0
	for localeName, locale := range locales {
		fmt.Printf("\n📁 Checking locale: %s\n", localeName)
		missing := checkMissingKeys(keys, locale)

		if len(missing) > 0 {
			fmt.Printf("  ⚠️  Missing %d translations:\n", len(missing))
			for _, m := range missing {
				fmt.Printf("    • %s\n", m.Key)
				for _, usage := range m.Usage {
					fmt.Printf("      └─ used in: %s\n", usage)
				}
			}
			totalMissing += len(missing)
		} else {
			fmt.Printf("  ✅ All translations present\n")
		}
	}

	// Step 4: Check each non-English locale for values copied from en.yml
	allow, err := loadAllowlist(identicalAllowlistFile)
	if err != nil {
		fmt.Printf("❌ Error loading %s: %v\n", identicalAllowlistFile, err)
		os.Exit(1)
	}
	totalUntranslated, err := reportUntranslatedValues(locales, allow)
	if err != nil {
		fmt.Printf("❌ Error checking values: %v\n", err)
		os.Exit(1)
	}

	// Step 5: Summary
	fmt.Printf("\n" + strings.Repeat("─", 50) + "\n")
	failed := false
	if totalMissing > 0 {
		fmt.Printf("❌ Summary: Found %d missing translations\n", totalMissing)
		failed = true
	} else {
		fmt.Printf("✅ Summary: All translations are present!\n")
	}
	if totalUntranslated > 0 {
		fmt.Printf("❌ Summary: Found %d values identical to %s or unused allowlist entries\n",
			totalUntranslated, sourceLocale)
		fmt.Printf("   Translate them; add a string that is the same in both languages to %s with a reason,\n",
			identicalAllowlistFile)
		fmt.Printf("   and remove allowlist entries that no longer match.\n")
		failed = true
	} else {
		fmt.Printf("✅ Summary: No values copied from %s!\n", sourceLocale)
	}
	if failed {
		os.Exit(1)
	}
}

// identicalAllowlistFile lists the values a locale may share with en.yml,
// relative to the directory the check runs from (scripts/check_translations).
const identicalAllowlistFile = "identical_allowlist.txt"

// sourceLocale is the file every other locale translates.
const sourceLocale = "en.yml"

// reportUntranslatedValues prints, per non-English locale, the keys whose
// value is the en.yml value, then the allowlist entries no locale needed, and
// returns how many of both it found. A missing en.yml is an error.
func reportUntranslatedValues(locales map[string]map[string]any, allow allowlist) (int, error) {
	en, ok := locales[sourceLocale]
	if !ok {
		return 0, fmt.Errorf("%s not found or not readable", sourceLocale)
	}
	enValues := make(map[string]string)
	flattenStrings("", en, enValues)

	names := make([]string, 0, len(locales))
	for name := range locales {
		if name != sourceLocale {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	total := 0
	for _, name := range names {
		fmt.Printf("\n📁 Checking values in locale: %s\n", name)
		untranslated := checkUntranslatedValues(enValues, locales[name], localeCode(name), allow)
		if len(untranslated) == 0 {
			fmt.Printf("  ✅ No values copied from %s\n", sourceLocale)
			continue
		}
		fmt.Printf("  ⚠️  %d values identical to %s:\n", len(untranslated), sourceLocale)
		for _, key := range untranslated {
			fmt.Printf("    • %s\n", key)
		}
		total += len(untranslated)
	}

	if unused := allow.unused(); len(unused) > 0 {
		fmt.Printf("\n⚠️  %d %s entries match no value identical to %s:\n",
			len(unused), identicalAllowlistFile, sourceLocale)
		for _, entry := range unused {
			fmt.Printf("    • %s\n", entry)
		}
		total += len(unused)
	}
	return total, nil
}

// checkUntranslatedValues returns, sorted, the keys whose value in locale is
// the en value and still holds words to translate, minus the allowlist.
func checkUntranslatedValues(enValues map[string]string, locale map[string]any, code string, allow allowlist) []string {
	values := make(map[string]string)
	flattenStrings("", locale, values)

	var result []string
	for key, value := range values {
		enValue, ok := enValues[key]
		if !ok || value != enValue || !hasTranslatableText(value) || allow.allows(code, key) {
			continue
		}
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

// flattenStrings collects every string leaf of v under its dotted key path;
// a list item gets its index as the last segment.
func flattenStrings(prefix string, v any, out map[string]string) {
	join := func(part string) string {
		if prefix == "" {
			return part
		}
		return prefix + "." + part
	}
	switch val := v.(type) {
	case string:
		out[prefix] = val
	case map[string]any:
		for k, child := range val {
			flattenStrings(join(k), child, out)
		}
	case []any:
		for i, child := range val {
			flattenStrings(join(strconv.Itoa(i)), child, out)
		}
	}
}

// nonTranslatableParts match the parts of a value that stay the same in
// every language.
var nonTranslatableParts = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(?:https?://|www\.|t\.me/)\S+`),                                     // URLs
	regexp.MustCompile(`<[^<>]*>`),                                                                // HTML tags
	regexp.MustCompile(`&(?:[a-zA-Z]+|#\d+|#x[0-9a-fA-F]+);`),                                     // HTML entities
	regexp.MustCompile(`\{[^{}]*\}`),                                                              // {placeholders}
	regexp.MustCompile(`%(?:\[\d+\])?[-+#0]*(?:\d+|\*)?(?:\.(?:\d+|\*))?[vTtbcdoOqxXUeEfFgGsp%]`), // format verbs
}

// hasTranslatableText reports whether s still holds a letter once URLs,
// HTML, placeholders and format verbs are removed. Emoji, digits and
// punctuation are not letters, so a value made only of them reads the same
// in every language.
func hasTranslatableText(s string) bool {
	for _, re := range nonTranslatableParts {
		s = re.ReplaceAllString(s, " ")
	}
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// localeCode turns a locale file name ("es.yml") into its code ("es").
func localeCode(filename string) string {
	return strings.TrimSuffix(strings.TrimSuffix(filename, ".yml"), ".yaml")
}

// allowlist holds the keys whose value may equal en.yml's: in one locale
// ("es:key") or in every locale ("key"). It records the entries a check
// needed, so entries that match nothing can be reported.
type allowlist struct {
	all       map[string]bool
	perLocale map[string]bool
	used      map[string]bool
}

func (a allowlist) allows(code, key string) bool {
	entry := code + ":" + key
	if !a.perLocale[entry] {
		if !a.all[key] {
			return false
		}
		entry = key
	}
	if a.used != nil {
		a.used[entry] = true
	}
	return true
}

// unused returns, sorted, the entries no allows call has matched.
func (a allowlist) unused() []string {
	var result []string
	for _, set := range []map[string]bool{a.all, a.perLocale} {
		for entry := range set {
			if !a.used[entry] {
				result = append(result, entry)
			}
		}
	}
	sort.Strings(result)
	return result
}

// loadAllowlist reads one entry per line, "key" or "locale:key", followed by
// a "# reason" comment. Blank lines and lines starting with "#" are skipped;
// an entry without a reason is an error.
func loadAllowlist(path string) (allowlist, error) {
	allow := allowlist{all: map[string]bool{}, perLocale: map[string]bool{}, used: map[string]bool{}}
	data, err := os.ReadFile(path) // #nosec G304 - fixed file name or test fixture
	if err != nil {
		return allow, err
	}

	for i, raw := range strings.Split(string(data), "\n") {
		lineNo := i + 1
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entry, reason, found := strings.Cut(line, "#")
		entry = strings.TrimSpace(entry)
		if !found || strings.TrimSpace(reason) == "" {
			return allow, fmt.Errorf("line %d: %q needs a \"# reason\" comment", lineNo, entry)
		}
		if strings.ContainsAny(entry, " \t") {
			return allow, fmt.Errorf("line %d: %q is not a single key", lineNo, entry)
		}
		if strings.Contains(entry, ":") {
			allow.perLocale[entry] = true
		} else {
			allow.all[entry] = true
		}
	}
	return allow, nil
}

func extractTranslationKeys(rootDir string) ([]TranslationKey, error) {
	var keys []TranslationKey

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		fileKeys, err := extractKeysFromFile(path)
		if err != nil {
			return fmt.Errorf("extract keys from %s: %w", path, err)
		}
		keys = append(keys, fileKeys...)

		return nil
	})

	// Note: alt_names are configuration keys, not translation keys
	// They are loaded from config.yml, not from translation files
	// So we don't need to check for them

	return keys, err
}

func extractKeysFromFile(filePath string) ([]TranslationKey, error) {
	var keys []TranslationKey

	// Resolve to absolute path to handle legitimate relative paths (e.g., ../../alita/...)
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not resolve path %s: %w", filePath, err)
	}
	filePath = absPath

	// Read file content
	content, err := os.ReadFile(filePath) // #nosec G304 - path validation performed above
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, content, parser.AllErrors)
	if err != nil {
		return nil, err
	}

	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || (sel.Sel.Name != "GetString" && sel.Sel.Name != "GetStringSlice") || len(call.Args) == 0 {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		key, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		pos := fset.Position(lit.Pos())
		keys = append(keys, TranslationKey{Key: key, File: filePath, Line: pos.Line})

		return true
	})

	return keys, nil
}

func loadLocaleFiles(localesDir string) (map[string]map[string]any, error) {
	locales := make(map[string]map[string]any)

	entries, err := os.ReadDir(localesDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if !strings.HasSuffix(filename, ".yml") && !strings.HasSuffix(filename, ".yaml") {
			continue
		}

		// Skip config.yml as it's not a translation file
		if filename == "config.yml" {
			continue
		}

		filePath := filepath.Join(localesDir, filename)

		// Resolve to absolute path to handle legitimate relative paths
		absFilePath, err := filepath.Abs(filePath)
		if err != nil {
			fmt.Printf("  Warning: Could not resolve path %s, skipping: %v\n", filename, err)
			continue
		}
		filePath = absFilePath

		// Ensure the resolved path is still within the locales directory
		absLocalesDir, _ := filepath.Abs(localesDir)
		if !strings.HasPrefix(filePath, absLocalesDir) {
			fmt.Printf("  Warning: File path %s is outside locales directory, skipping\n", filename)
			continue
		}

		data, err := os.ReadFile(filePath) // #nosec G304 - path validation performed above
		if err != nil {
			fmt.Printf("  ⚠️  Warning: Could not read %s: %v\n", filename, err)
			continue
		}

		var locale map[string]any
		if err := yaml.Unmarshal(data, &locale); err != nil {
			fmt.Printf("  ⚠️  Warning: Could not parse %s: %v\n", filename, err)
			continue
		}

		locales[filename] = locale
	}

	return locales, nil
}

func checkMissingKeys(keys []TranslationKey, locale map[string]any) []MissingTranslation {
	missing := make(map[string][]string)

	for _, key := range keys {
		// Skip alt_names keys as they're configuration, not translations
		if strings.HasPrefix(key.Key, "alt_names.") {
			continue
		}

		if !keyExists(key.Key, locale) {
			usage := fmt.Sprintf("%s:%d", filepath.Base(key.File), key.Line)
			missing[key.Key] = append(missing[key.Key], usage)
		}
	}

	// Convert to sorted list
	var result []MissingTranslation
	for key, usages := range missing {
		result = append(result, MissingTranslation{
			Key:   key,
			Usage: usages,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Key < result[j].Key
	})

	return result
}

func keyExists(key string, data map[string]any) bool {
	parts := strings.Split(key, ".")
	current := data

	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part - check if key exists
			_, exists := current[part]
			return exists
		}

		// Navigate deeper
		if next, ok := current[part].(map[string]any); ok {
			current = next
		} else {
			return false
		}
	}

	return false
}
