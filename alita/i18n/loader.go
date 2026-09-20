package i18n

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func (lm *LocaleManager) loadLocaleFiles() error {
	if lm.localeFS == nil || lm.localePath == "" {
		return NewI18nError("load_files", "", "", "filesystem or path not set", fmt.Errorf("invalid configuration"))
	}

	entries, err := lm.localeFS.ReadDir(lm.localePath)
	if err != nil {
		return NewI18nError("load_files", "", "", "failed to read locale directory", err)
	}

	var loadErrors []error

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		if !isYAMLFile(fileName) {
			continue
		}

		filePath := filepath.Join(lm.localePath, fileName)
		langCode := extractLangCode(fileName)

		if err := lm.loadSingleLocaleFile(filePath, langCode); err != nil {
			loadErrors = append(loadErrors, err)
			continue
		}
	}

	if len(loadErrors) > 0 {
		return fmt.Errorf("failed to load %d locale files: %v", len(loadErrors), loadErrors)
	}

	return nil
}

func (lm *LocaleManager) loadSingleLocaleFile(filePath, langCode string) error {
	content, err := lm.localeFS.ReadFile(filePath)
	if err != nil {
		return NewI18nError("load_file", langCode, "", "failed to read file", err)
	}

	parsed, err := parseYAML(content)
	if err != nil {
		return NewI18nError("load_file", langCode, "", "invalid YAML structure", err)
	}

	lm.localeMaps[langCode] = parsed
	if lm.localeIndex == nil {
		lm.localeIndex = make(map[string]*lookupIndex)
	}
	lm.localeIndex[langCode] = buildLookupIndex(parsed)
	lm.translators.Delete(langCode)

	return nil
}

// parseYAML unmarshals a YAML mapping for key lookups. yaml.v3 decodes nested
// mappings with string keys as map[string]any, so dot-path descent is clean.
func parseYAML(content []byte) (map[string]any, error) {
	var data any
	if err := yaml.Unmarshal(content, &data); err != nil {
		return nil, NewI18nError("validate_yaml", "", "", "YAML parsing failed", err)
	}

	parsed, ok := data.(map[string]any)
	if !ok {
		return nil, NewI18nError("validate_yaml", "", "", "root element must be a map", ErrInvalidYAML)
	}

	return parsed, nil
}

func extractLangCode(fileName string) string {
	langCode := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	langCode = strings.TrimSuffix(langCode, ".yml")
	langCode = strings.TrimSuffix(langCode, ".yaml")
	return langCode
}

// buildLookupIndex flattens a parsed locale into lowercase dot-path lookup tables so a
// runtime lookup is a single map read: scalars holds the fmt.Sprint form of every leaf
// (what lookupString would return), slices the resolved []string form of sequence
// leaves. Segments containing "." are skipped because lookup splits queries on "." and
// can never resolve such a segment.
//
// A locale holding two keys that differ only in case gets no index: lowering the tables
// would merge the pair and make the exact-case key unreachable, while lookupSegment
// matches exact case first. Those locales (none shipped today) fall back to the walk.
func buildLookupIndex(data map[string]any) *lookupIndex {
	if hasCaseCollidingKeys(data) {
		return nil
	}
	index := &lookupIndex{
		scalars: make(map[string]string, len(data)),
		slices:  make(map[string][]string),
	}
	index.addMap("", data)
	return index
}

// hasCaseCollidingKeys reports whether any map in the tree holds two keys that differ
// only in case.
func hasCaseCollidingKeys(m map[string]any) bool {
	seen := make(map[string]struct{}, len(m))
	for key, value := range m {
		lowered := strings.ToLower(key)
		if _, exists := seen[lowered]; exists {
			return true
		}
		seen[lowered] = struct{}{}
		if child, ok := value.(map[string]any); ok && hasCaseCollidingKeys(child) {
			return true
		}
	}
	return false
}

func (index *lookupIndex) addMap(prefix string, m map[string]any) {
	for key, value := range m {
		if strings.Contains(key, ".") {
			continue
		}
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		index.add(path, value)
		if child, ok := value.(map[string]any); ok {
			index.addMap(path, child)
		}
	}
}

// add records one path. Nil leaves are skipped (lookupString reports them as missing).
// Paths are stored lowered; buildLookupIndex rejects locales where lowering two paths
// onto one key could let a case variant shadow a sibling.
func (index *lookupIndex) add(path string, value any) {
	if value == nil {
		return
	}
	key := strings.ToLower(path)
	index.scalars[key] = fmt.Sprint(value)
	switch value.(type) {
	case []any, []string:
		index.slices[key] = toLookupSlice(value)
	}
}

// lookup descends a parsed YAML map by a dot-separated key path and returns the
// leaf value if present. Path segments are matched case-insensitively to replicate
// viper's case-insensitive key behavior (e.g. "alt_names.Admin" against a config
// where keys may differ in case).
func lookup(data map[string]any, key string) (any, bool) {
	if data == nil {
		return nil, false
	}

	segments := strings.Split(key, ".")
	var current any = data

	for _, seg := range segments {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		value, found := lookupSegment(m, seg)
		if !found {
			return nil, false
		}
		current = value
	}

	return current, true
}

// lookupSegment resolves a single map key, preferring an exact match and falling
// back to a case-insensitive match.
func lookupSegment(m map[string]any, seg string) (any, bool) {
	if value, ok := m[seg]; ok {
		return value, true
	}
	for k, v := range m {
		if strings.EqualFold(k, seg) {
			return v, true
		}
	}
	return nil, false
}

// lookupString resolves a dot-path key to its scalar value, coercing the leaf to a
// string via fmt.Sprint (mirroring viper.GetString). Missing keys yield "". It is the
// fallback for translators without a precomputed index, see Translator.lookupString.
func lookupString(data map[string]any, key string) string {
	value, found := lookup(data, key)
	if !found || value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

// lookupStringSlice resolves a dot-path key to a []string, coercing each element of
// a YAML sequence via fmt.Sprint and splitting a scalar string on whitespace
// (mirroring viper.GetStringSlice). Missing keys yield an empty slice.
func lookupStringSlice(data map[string]any, key string) []string {
	value, found := lookup(data, key)
	if !found || value == nil {
		return nil
	}
	return toLookupSlice(value)
}

// toLookupSlice coerces a non-nil leaf into the []string form GetStringSlice yields.
func toLookupSlice(value any) []string {
	switch v := value.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, elem := range v {
			out = append(out, fmt.Sprint(elem))
		}
		return out
	case string:
		return strings.Fields(v)
	default:
		return strings.Fields(fmt.Sprint(v))
	}
}

// lookupString resolves a key through the translator's precomputed flat index when one
// was built, avoiding both the dot-path split and the case-insensitive map scan. The
// index covers every key the generic walk can reach, so a miss is a miss.
func (t *Translator) lookupString(key string) string {
	if t.index == nil {
		return lookupString(t.data, key)
	}
	return t.index.scalars[strings.ToLower(key)]
}

// lookupStringSlice is lookupString's []string counterpart. Sequence leaves come from
// the index; scalar leaves resolve through their cached scalar form, exactly as the
// generic walk does.
func (t *Translator) lookupStringSlice(key string) []string {
	if t.index == nil {
		return lookupStringSlice(t.data, key)
	}
	lowered := strings.ToLower(key)
	if fields, ok := t.index.slices[lowered]; ok {
		return fields
	}
	if scalar, ok := t.index.scalars[lowered]; ok {
		return strings.Fields(scalar)
	}
	return nil
}

func isYAMLFile(fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	return ext == ".yml" || ext == ".yaml"
}
