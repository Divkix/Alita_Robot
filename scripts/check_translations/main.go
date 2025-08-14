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
	"strings"

	"gopkg.in/yaml.v3"
)

type TranslationKey struct {
	Key       string
	File      string
	Line      int
	IsDynamic bool
}

type MissingTranslation struct {
	Key   string
	Usage []string
}

var (
	simpleKeyRegex  = regexp.MustCompile(`tr\.GetString\s*\(\s*"([^"]+)"`)
	simpleKeyRegex2 = regexp.MustCompile(`tr\.GetStringSlice\s*\(\s*"([^"]+)"`)
)

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
	for localeName, localeData := range locales {
		fmt.Printf("\n📁 Checking locale: %s\n", localeName)
		missing := checkMissingKeys(keys, localeData)

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

	// Step 4: Summary
	fmt.Printf("\n" + strings.Repeat("─", 50) + "\n")
	if totalMissing > 0 {
		fmt.Printf("❌ Summary: Found %d missing translations\n", totalMissing)
		os.Exit(1)
	} else {
		fmt.Printf("✅ Summary: All translations are present!\n")
	}
}

func extractTranslationKeys(rootDir string) ([]TranslationKey, error) {
	var keys []TranslationKey
	keyMap := make(map[string]bool)

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		fileKeys, err := extractKeysFromFile(path)
		if err != nil {
			fmt.Printf("  ⚠️  Warning: Could not parse %s: %v\n", path, err)
			// Continue processing other files
			return nil
		}

		for _, key := range fileKeys {
			keys = append(keys, key)
			keyMap[key.Key] = true
		}

		return nil
	})

	// Note: alt_names are configuration keys, not translation keys
	// They are loaded from config.yml, not from translation files
	// So we don't need to check for them

	return keys, err
}

func extractKeysFromFile(filePath string) ([]TranslationKey, error) {
	var keys []TranslationKey

	// Read file content
	filePath = filepath.Clean(filePath)
	content, err := os.ReadFile(filePath) //nolint:gosec // Path is cleaned and from filepath.WalkDir
	if err != nil {
		return nil, err
	}

	// Method 1: Use regex to find simple patterns
	lines := strings.Split(string(content), "\n")
	for lineNum, line := range lines {
		// Check for tr.GetString("key")
		if matches := simpleKeyRegex.FindAllStringSubmatch(line, -1); matches != nil {
			for _, match := range matches {
				if len(match) > 1 {
					keys = append(keys, TranslationKey{
						Key:  match[1],
						File: filePath,
						Line: lineNum + 1,
					})
				}
			}
		}

		// Check for tr.GetStringSlice("key")
		if matches := simpleKeyRegex2.FindAllStringSubmatch(line, -1); matches != nil {
			for _, match := range matches {
				if len(match) > 1 {
					keys = append(keys, TranslationKey{
						Key:  match[1],
						File: filePath,
						Line: lineNum + 1,
					})
				}
			}
		}
	}

	// Method 2: Use AST parsing for more complex patterns
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, content, parser.AllErrors)
	if err != nil {
		// If AST parsing fails, continue with regex results
		return keys, nil
	}

	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check for tr.GetString or tr.GetStringSlice calls
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "tr" {
				if sel.Sel.Name == "GetString" || sel.Sel.Name == "GetStringSlice" {
					if len(call.Args) > 0 {
						if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
							key := strings.Trim(lit.Value, `"`)
							pos := fset.Position(lit.Pos())

							// Check if this key is already in our list
							found := false
							for _, existingKey := range keys {
								if existingKey.Key == key && existingKey.File == filePath {
									found = true
									break
								}
							}

							if !found {
								keys = append(keys, TranslationKey{
									Key:  key,
									File: filePath,
									Line: pos.Line,
								})
							}
						}
					}
				}
			}
		}

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
		filePath = filepath.Clean(filePath)
		data, err := os.ReadFile(filePath) //nolint:gosec // Path is cleaned and from trusted source
		if err != nil {
			fmt.Printf("  ⚠️  Warning: Could not read %s: %v\n", filename, err)
			continue
		}

		var localeData map[string]any
		if err := yaml.Unmarshal(data, &localeData); err != nil {
			fmt.Printf("  ⚠️  Warning: Could not parse %s: %v\n", filename, err)
			continue
		}

		locales[filename] = localeData
	}

	return locales, nil
}

func checkMissingKeys(keys []TranslationKey, localeData map[string]any) []MissingTranslation {
	missing := make(map[string][]string)

	for _, key := range keys {
		// Skip alt_names keys as they're configuration, not translations
		if strings.HasPrefix(key.Key, "alt_names.") {
			continue
		}

		if !keyExists(key.Key, localeData) {
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
