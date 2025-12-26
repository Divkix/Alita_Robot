// Package main provides a documentation generator for Alita Robot.
// It parses the codebase and generates Markdown documentation for Astro/Starlight.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Config holds the generator configuration
type Config struct {
	// Source paths (relative to project root)
	LocalesPath    string
	ModulesPath    string
	ConfigPath     string
	MigrationsPath string
	SampleEnvPath  string
	ChatStatusPath string // Path to chat_status.go for permission parsing

	// Output path
	DocsOutputPath string

	// Options
	Verbose bool
	DryRun  bool
}

// Module represents a bot module with its commands and help text
type Module struct {
	Name        string
	DisplayName string
	HelpText    string
	Commands    []Command
	Aliases     []string
}

// Command represents a bot command
type Command struct {
	Name        string
	Handler     string
	Module      string
	Disableable bool
	Aliases     []string
}

// EnvVar represents an environment variable
type EnvVar struct {
	Name        string
	Type        string
	Required    bool
	Default     string
	Description string
	Validation  string
	Category    string
}

// DBTable represents a database table
type DBTable struct {
	Name        string
	Description string
	Columns     []DBColumn
	Indexes     []string
	ForeignKeys []string
}

// DBColumn represents a database column
type DBColumn struct {
	Name       string
	Type       string
	Nullable   bool
	Default    string
	PrimaryKey bool
	Unique     bool
}

// Callback represents a callback query handler registration
type Callback struct {
	Prefix     string // e.g., "restrict."
	Handler    string // e.g., "restrictButtonHandler"
	Module     string // e.g., "bans"
	SourceFile string // e.g., "bans.go"
}

// PermissionFunc represents a permission checking function
type PermissionFunc struct {
	Name        string   // e.g., "CanUserRestrict"
	Signature   string   // Full function signature
	Parameters  []string // Parameter list
	ReturnType  string   // e.g., "bool"
	Category    string   // e.g., "User Permission Checks"
	Description string   // From comment above function
}

var config Config

func main() {
	// Parse flags
	flag.StringVar(&config.LocalesPath, "locales", "locales", "Path to locales directory")
	flag.StringVar(&config.ModulesPath, "modules", "alita/modules", "Path to modules directory")
	flag.StringVar(&config.ConfigPath, "config", "alita/config/config.go", "Path to config.go")
	flag.StringVar(&config.MigrationsPath, "migrations", "migrations", "Path to migrations directory")
	flag.StringVar(&config.SampleEnvPath, "sample-env", "sample.env", "Path to sample.env")
	flag.StringVar(&config.ChatStatusPath, "chat-status", "alita/utils/chat_status/chat_status.go", "Path to chat_status.go")
	flag.StringVar(&config.DocsOutputPath, "output", "docs/src/content/docs", "Output path for generated docs")
	flag.BoolVar(&config.Verbose, "verbose", false, "Enable verbose logging")
	flag.BoolVar(&config.DryRun, "dry-run", false, "Print what would be generated without writing files")
	flag.Parse()

	if config.Verbose {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	log.Info("🚀 Starting Alita Robot Documentation Generator")

	// Find project root
	projectRoot, err := findProjectRoot()
	if err != nil {
		log.Fatalf("Failed to find project root: %v", err)
	}
	log.Debugf("Project root: %s", projectRoot)

	// Parse all sources
	log.Info("📖 Parsing translations...")
	translations, moduleAliases, err := parseTranslations(filepath.Join(projectRoot, config.LocalesPath))
	if err != nil {
		log.Fatalf("Failed to parse translations: %v", err)
	}
	log.Infof("   Found %d module help texts", len(translations))

	log.Info("🔍 Parsing commands from source...")
	commands, err := parseCommands(filepath.Join(projectRoot, config.ModulesPath))
	if err != nil {
		log.Fatalf("Failed to parse commands: %v", err)
	}
	log.Infof("   Found %d commands", len(commands))

	log.Info("⚙️  Parsing environment variables...")
	envVars, err := parseConfigStruct(filepath.Join(projectRoot, config.ConfigPath))
	if err != nil {
		log.Fatalf("Failed to parse config: %v", err)
	}
	log.Infof("   Found %d environment variables", len(envVars))

	log.Info("🗄️  Parsing database schema...")
	tables, err := parseMigrations(filepath.Join(projectRoot, config.MigrationsPath))
	if err != nil {
		log.Fatalf("Failed to parse migrations: %v", err)
	}
	log.Infof("   Found %d database tables", len(tables))

	// Parse callbacks
	log.Info("🔔 Parsing callbacks from modules...")
	callbacks, err := parseCallbacks(filepath.Join(projectRoot, config.ModulesPath))
	if err != nil {
		log.Fatalf("Failed to parse callbacks: %v", err)
	}
	log.Infof("   Found %d callback handlers", len(callbacks))

	// Parse permissions
	log.Info("🔐 Parsing permission functions...")
	permissions, err := parsePermissions(filepath.Join(projectRoot, config.ChatStatusPath))
	if err != nil {
		log.Fatalf("Failed to parse permissions: %v", err)
	}
	log.Infof("   Found %d permission functions", len(permissions))

	// Build modules with their commands
	modules := buildModules(translations, commands, moduleAliases)
	log.Infof("📦 Built %d modules with commands", len(modules))

	// Generate documentation
	outputPath := filepath.Join(projectRoot, config.DocsOutputPath)

	log.Info("📝 Generating documentation...")

	if err := generateModuleDocs(modules, outputPath); err != nil {
		log.Fatalf("Failed to generate module docs: %v", err)
	}

	if err := generateCommandReference(modules, outputPath); err != nil {
		log.Fatalf("Failed to generate command reference: %v", err)
	}

	if err := generateEnvReference(envVars, outputPath); err != nil {
		log.Fatalf("Failed to generate env reference: %v", err)
	}

	if err := generateSchemaReference(tables, outputPath); err != nil {
		log.Fatalf("Failed to generate schema reference: %v", err)
	}

	if err := generateCommandsOverview(modules, outputPath); err != nil {
		log.Fatalf("Failed to generate commands overview: %v", err)
	}

	// Generate callbacks reference
	if err := generateCallbacksReference(callbacks, outputPath); err != nil {
		log.Fatalf("Failed to generate callbacks reference: %v", err)
	}
	log.Info("Generated: api-reference/callbacks.md")

	// Generate permissions reference
	if err := generatePermissionsReference(permissions, outputPath); err != nil {
		log.Fatalf("Failed to generate permissions reference: %v", err)
	}
	log.Info("Generated: api-reference/permissions.md")

	log.Info("✅ Documentation generation complete!")
	log.Infof("   Output: %s", outputPath)
}

// findProjectRoot finds the project root by looking for go.mod
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find project root (go.mod)")
		}
		dir = parent
	}
}

// buildModules combines translations and commands into Module structs
func buildModules(translations map[string]string, commands []Command, aliases map[string][]string) []Module {
	// Group commands by module
	commandsByModule := make(map[string][]Command)
	for _, cmd := range commands {
		commandsByModule[cmd.Module] = append(commandsByModule[cmd.Module], cmd)
	}

	var modules []Module
	for name, helpText := range translations {
		// Extract module name from key (e.g., "admin_help_msg" -> "Admin")
		moduleName := strings.TrimSuffix(name, "_help_msg")
		displayName := cases.Title(language.English).String(moduleName)

		// Normalize module name for lookup
		normalizedName := strings.ToLower(moduleName)

		// Find commands for this module
		var moduleCommands []Command
		for modName, cmds := range commandsByModule {
			if strings.ToLower(modName) == normalizedName ||
				strings.ToLower(modName) == normalizedName+"s" ||
				strings.ToLower(modName)+"s" == normalizedName {
				moduleCommands = append(moduleCommands, cmds...)
			}
		}

		// Sort commands by name
		sort.Slice(moduleCommands, func(i, j int) bool {
			return moduleCommands[i].Name < moduleCommands[j].Name
		})

		modules = append(modules, Module{
			Name:        normalizedName,
			DisplayName: displayName,
			HelpText:    helpText,
			Commands:    moduleCommands,
			Aliases:     aliases[displayName],
		})
	}

	// Sort modules by name
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Name < modules[j].Name
	})

	return modules
}
