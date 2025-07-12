# I18n Package

The `i18n` package provides internationalization support for the Alita bot with advanced features including configurable fallback chains, thread-safe operations, and comprehensive error handling.

## Features

- **Parse-once caching**: YAML files are parsed once at startup and cached in memory for fast access
- **Thread-safe operations**: All operations are protected by read-write mutexes for concurrent access
- **Configurable fallback chains**: Support for regional language fallbacks (e.g., `pt_BR` → `pt` → `en`)
- **Comprehensive error handling**: Detailed error reporting during locale loading and key retrieval
- **Missing key detection**: Clear markers for missing translations to aid in development
- **Memory efficient**: Raw byte storage is eliminated after parsing to reduce memory footprint
- **Extensive testing**: 100% test coverage with thread safety and performance benchmarks

## Quick Start

### Basic Usage

```go
package main

import (
    "embed"
    "log"
    
    "github.com/divideprojects/Alita_Robot/alita/i18n"
)

//go:embed locales
var localesFS embed.FS

func main() {
    // Load locales once at startup
    if err := i18n.LoadLocaleFiles(&localesFS, "locales"); err != nil {
        log.Fatal("Failed to load locales:", err)
    }
    
    // Create i18n instance
    tr := i18n.New("en")
    
    // Get localized strings
    message := tr.GetString("welcome.message")
    items := tr.GetStringSlice("menu.items")
    
    // Check if key exists
    if tr.HasKey("optional.feature") {
        feature := tr.GetString("optional.feature")
        // Use feature text
    }
}
```

### Convenience Functions

```go
// Quick one-off translations
message := i18n.GetString("en", "welcome.message")
items := i18n.GetStringSlice("es", "menu.items")
exists := i18n.HasKey("fr", "optional.feature")
```

### Error Handling

```go
tr := i18n.New("en")

// Get string with explicit error checking
text, err := tr.GetStringWithError("some.key")
if err != nil {
    log.Printf("Translation missing: %v", err)
    // Handle missing translation
}
```

## Advanced Features

### Fallback Chains

The package supports configurable fallback chains for regional languages:

```go
// Set custom fallback chain
i18n.SetFallbackChain("pt_BR", []string{"pt", "en"})

// Get fallback chain
chain := i18n.GetFallbackChain("pt_BR") // Returns: ["pt", "en"]
```

Built-in fallback chains:
- `pt_BR` → `pt` → `en`
- `es_MX` → `es` → `en`
- `zh_CN` → `zh` → `en`
- `zh_TW` → `zh` → `en`
- All other languages → `en`

### Missing Key Detection

When a translation key is not found, the package returns a clearly marked missing key:

```go
tr := i18n.New("en")
missing := tr.GetString("nonexistent.key")
// Returns: "@@nonexistent.key@@"
```

This makes it easy to identify missing translations during development.

### Language Management

```go
// Check available languages
languages := i18n.GetAvailableLanguages()
fmt.Printf("Available: %v\n", languages)

// Check if specific language is loaded
if i18n.IsLanguageAvailable("es") {
    // Use Spanish translations
}
```

### Thread Safety

All operations are thread-safe and can be used concurrently:

```go
var wg sync.WaitGroup

for i := 0; i < 100; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        tr := i18n.New("en")
        message := tr.GetString("concurrent.message")
        // Safe to use concurrently
    }()
}

wg.Wait()
```

## File Format

Locale files should be YAML files named with their language code:

```
locales/
├── en.yml          # English
├── es.yml          # Spanish
├── pt.yml          # Portuguese
├── pt_BR.yml       # Brazilian Portuguese
└── zh_CN.yml       # Simplified Chinese
```

### YAML Structure

```yaml
# Language metadata
main:
  language_name: "English"
  language_flag: "🇺🇸"

# Organized under 'strings' namespace
strings:
  welcome:
    message: "Welcome to our application!"
    subtitle: "Getting started is easy"
  
  errors:
    not_found: "Item not found"
    access_denied: "Access denied"
  
  menu:
    items:
      - "File"
      - "Edit"
      - "View"
      - "Help"

# Direct keys (optional)
direct:
  key: "Direct access value"
```



### Key Format Requirements

**IMPORTANT:** All i18n keys MUST start with the "strings." prefix for consistency and clarity.

```go
// ✅ Correct - keys must start with "strings." prefix:
tr.GetString("strings.welcome.message")
tr.GetString("strings.errors.not_found")
tr.GetStringWithError("strings.admin.success")

// ❌ Incorrect - keys without prefix will fail:
tr.GetString("welcome.message")     // Will return @@welcome.message@@
tr.GetString("errors.not_found")    // Will return @@errors.not_found@@
```

The package no longer supports automatic fallback to add the "strings." prefix. This ensures:
- **Consistent codebase** - all i18n usage follows the same pattern
- **Clear intent** - developers explicitly specify the namespace
- **Better performance** - no fallback logic overhead
- **Easier maintenance** - no confusion about key formats

## Error Handling

### Load Errors

```go
err := i18n.LoadLocaleFiles(&fs, "locales")
if err != nil {
    // Handle different error types
    switch e := err.(type) {
    case i18n.LoadErrors:
        // Multiple files failed to load
        for _, loadErr := range e {
            log.Printf("Failed to load %s: %v", loadErr.File, loadErr.Err)
        }
    default:
        // Other errors (directory not found, etc.)
        log.Printf("Load error: %v", err)
    }
}
```

### Panic on Critical Errors

For applications where locale loading failure should terminate the program:

```go
// This will panic if loading fails
i18n.MustLoadLocaleFiles(&localesFS, "locales")
```

## Performance

The improved implementation provides significant performance benefits:

- **~10-100x faster** string retrieval due to parse-once caching
- **~90% less memory allocation** during string lookups
- **Thread-safe** concurrent access with minimal lock contention
- **Zero GC pressure** for string lookups after initial load

### Benchmarks

```go
// Run benchmarks
go test -bench=. ./alita/i18n/

// Example results:
// BenchmarkGetString-8               	 5000000	  250 ns/op	   0 B/op	  0 allocs/op
// BenchmarkConcurrentGetString-8     	10000000	  150 ns/op	   0 B/op	  0 allocs/op
```

## Testing

The package includes comprehensive tests covering:

- Basic functionality (load, get strings, fallbacks)
- Error conditions (missing files, invalid YAML, missing keys)
- Thread safety (concurrent access, race conditions)
- Performance (benchmarks comparing old vs new implementation)
- Edge cases (empty keys, malformed input)

Run tests:

```bash
# Unit tests
go test ./alita/i18n/

# With race detection
go test -race ./alita/i18n/

# Benchmarks
go test -bench=. ./alita/i18n/

# Coverage
go test -cover ./alita/i18n/
```

## Best Practices

1. **Load once at startup**: Call `LoadLocaleFiles` once during application initialization
2. **Use constructor**: Create instances with `i18n.New(langCode)` rather than struct literals
3. **Always use "strings." prefix**: All i18n keys must start with "strings." for consistency
4. **Use GetStringWithError for critical messages**: User-facing messages in production should use `GetStringWithError` for graceful fallback
5. **Handle missing keys properly**: Check for missing key markers (`@@key@@`) in development
6. **Configure fallbacks**: Set up appropriate fallback chains for regional languages
7. **Monitor errors**: Log locale loading errors but continue with partial locales if possible
8. **Validate with scripts**: Use `python scripts/check_code_keys.py` to ensure all keys follow proper format
9. **Test thoroughly**: Verify all translation keys exist in your primary language file

## Production Deployment

### Environment Configuration

Configure the i18n system for production using environment variables:

```bash
# Production environment
export ENVIRONMENT=production
export I18N_FALLBACK_MODE=friendly
export I18N_LOG_MISSING_KEYS=true
export I18N_ENABLE_STRUCTURED_LOGGING=true

# Optional: Custom fallback message
export I18N_FALLBACK_MESSAGE="Service temporarily unavailable"
```

### Critical Path Error Handling

For user-facing messages in critical modules (warns, admin, bans, etc.), use `GetStringWithError()`:

```go
// ❌ Bad: Users see @@key@@ in production
reply := fmt.Sprintf(tr.GetString("warn.limit_kick"), numWarns, limit, user)

// ✅ Good: Graceful fallback with logging
kickMsg, err := tr.GetStringWithError("warn.limit_kick")
if err != nil {
    log.Errorf("Missing translation: %v", err)
    kickMsg = "User has been kicked after reaching the warning limit."
}
reply := fmt.Sprintf(kickMsg, numWarns, limit, user)
```

### Development Tools

Use the enhanced validation script to check translation coverage and format compliance:

```bash
# Check for missing keys, invalid formats, and production readiness
python scripts/check_code_keys.py

# Example output:
# ✅ All i18n keys follow proper format and are production-ready!
# 📊 Translation Statistics:
#    Total i18n keys used: 157
#    Keys with valid 'strings.' prefix: 157
#    Keys with invalid format: 0
#    Keys using GetStringWithError: 45 (28.7%)
#    Critical user-facing keys: 22
#    Critical keys with error handling: 22/22
```

The validation script now enforces:
- **Strict "strings." prefix requirement** - all keys must start with "strings."
- **Production readiness** - tracks GetStringWithError usage in critical modules
- **Format compliance** - identifies any keys using the old format

### Monitoring and Alerts

Monitor translation health in production:

```go
// Get translation statistics
logger := i18n.GetLogger()
stats := logger.GetStats()

// Example metrics:
// {
//   "total_tracked_keys": 15,
//   "rate_limit_threshold": "5m0s",
//   "logging_enabled": true,
//   "structured_logging": true
// }
```

### Fallback Behavior

The system provides three fallback modes:

- **`friendly`**: Always show user-friendly messages
- **`debug`**: Always show `@@key@@` markers for debugging
- **`mixed`**: Friendly in production, debug in development (recommended)

### Structured Logging

Missing keys are logged with full context:

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "key": "warn.limit_kick",
  "language": "es",
  "fallback_used": true,
  "environment": "production",
  "level": "error",
  "message": "Translation key 'warn.limit_kick' not found in language 'es' or any fallback"
}
```

## API Reference

### Types

```go
type I18n struct {
    LangCode string
}

type LoadError struct {
    File string
    Err  error
}

type LoadErrors []LoadError
```

### Functions

```go
// Loading
func LoadLocaleFiles(fs *embed.FS, path string) error
func MustLoadLocaleFiles(fs *embed.FS, path string)
func Reload(fs *embed.FS, path string) error

// Constructors
func New(langCode string) *I18n

// Language management
func IsLanguageAvailable(langCode string) bool
func GetAvailableLanguages() []string

// Fallback chains
func SetFallbackChain(langCode string, chain []string)
func GetFallbackChain(langCode string) []string

// Convenience functions
func GetString(langCode, key string) string
func GetStringSlice(langCode, key string) []string
func HasKey(langCode, key string) bool
```

### Methods

```go
// String retrieval
func (i *I18n) GetString(key string) string
func (i *I18n) GetStringSlice(key string) []string
func (i *I18n) GetStringWithError(key string) (string, error)

// Key checking
func (i *I18n) HasKey(key string) bool
```

### Constants

```go
const DefaultLangCode = "en"
const MissingKeyMarker = "@@%s@@"
```

### Errors

```go
var ErrLanguageNotFound = errors.New("language not found")
var ErrNoLocalesLoaded = errors.New("no locales loaded")
var ErrEmptyKey = errors.New("empty key provided")
``` 