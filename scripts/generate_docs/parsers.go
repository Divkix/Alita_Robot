package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// parseTranslations parses locale YAML files and extracts help messages
func parseTranslations(localesPath string) (map[string]string, map[string][]string, error) {
	helpTexts := make(map[string]string)
	aliases := make(map[string][]string)

	// Parse en.yml for help messages
	enPath := filepath.Join(localesPath, "en.yml")
	data, err := os.ReadFile(enPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read %s: %w", enPath, err)
	}

	var translations map[string]interface{}
	if err := yaml.Unmarshal(data, &translations); err != nil {
		return nil, nil, fmt.Errorf("failed to parse %s: %w", enPath, err)
	}

	// Extract help messages
	for key, value := range translations {
		if strings.HasSuffix(key, "_help_msg") {
			if str, ok := value.(string); ok {
				helpTexts[key] = str
			}
		}
	}

	// Parse config.yml for aliases
	configPath := filepath.Join(localesPath, "config.yml")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		log.Warnf("Could not read config.yml: %v", err)
		return helpTexts, aliases, nil
	}

	var configYaml map[string]interface{}
	if err := yaml.Unmarshal(configData, &configYaml); err != nil {
		log.Warnf("Could not parse config.yml: %v", err)
		return helpTexts, aliases, nil
	}

	// Extract alt_names
	if altNames, ok := configYaml["alt_names"].(map[string]interface{}); ok {
		for module, aliasList := range altNames {
			switch v := aliasList.(type) {
			case []interface{}:
				for _, alias := range v {
					if str, ok := alias.(string); ok {
						aliases[module] = append(aliases[module], str)
					}
				}
			case string:
				aliases[module] = []string{v}
			}
		}
	}

	return helpTexts, aliases, nil
}

// parseCommands parses Go source files to extract command registrations
func parseCommands(modulesPath string) ([]Command, error) {
	var commands []Command

	// Regex patterns for command extraction
	newCommandPattern := regexp.MustCompile(`handlers\.NewCommand\s*\(\s*"([^"]+)"\s*,\s*(\w+)\.(\w+)\s*\)`)
	disableablePattern := regexp.MustCompile(`misc\.AddCmdToDisableable\s*\(\s*"([^"]+)"\s*\)`)
	moduleNamePattern := regexp.MustCompile(`(\w+)Module\s*=\s*moduleStruct\s*\{\s*moduleName:\s*"([^"]+)"`)

	// Track disableable commands
	disableableCmds := make(map[string]bool)

	// Track module names from struct declarations
	moduleNames := make(map[string]string)

	files, err := filepath.Glob(filepath.Join(modulesPath, "*.go"))
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			log.Warnf("Could not read %s: %v", file, err)
			continue
		}

		content := string(data)
		fileName := filepath.Base(file)
		moduleName := strings.TrimSuffix(fileName, ".go")

		// Find module name declarations
		moduleMatches := moduleNamePattern.FindAllStringSubmatch(content, -1)
		for _, match := range moduleMatches {
			if len(match) >= 3 {
				moduleNames[match[1]+"Module"] = match[2]
			}
		}

		// Find disableable commands
		disableMatches := disableablePattern.FindAllStringSubmatch(content, -1)
		for _, match := range disableMatches {
			if len(match) >= 2 {
				disableableCmds[match[1]] = true
			}
		}

		// Find command registrations
		cmdMatches := newCommandPattern.FindAllStringSubmatch(content, -1)
		for _, match := range cmdMatches {
			if len(match) >= 4 {
				cmdName := match[1]
				moduleVar := match[2]

				// Try to get module name from struct declaration
				modName := moduleName
				if name, ok := moduleNames[moduleVar]; ok {
					modName = name
				}

				commands = append(commands, Command{
					Name:        cmdName,
					Handler:     match[3],
					Module:      modName,
					Disableable: disableableCmds[cmdName],
				})
			}
		}
	}

	// Sort commands
	sort.Slice(commands, func(i, j int) bool {
		if commands[i].Module != commands[j].Module {
			return commands[i].Module < commands[j].Module
		}
		return commands[i].Name < commands[j].Name
	})

	return commands, nil
}

// parseConfigStruct parses the Config struct to extract environment variables
func parseConfigStruct(configPath string) ([]EnvVar, error) {
	var envVars []EnvVar

	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Patterns
	structFieldPattern := regexp.MustCompile(`^\s+(\w+)\s+(\S+)(?:\s+` + "`" + `([^` + "`" + `]+)` + "`" + `)?\s*(?://\s*(.*))?$`)
	commentPattern := regexp.MustCompile(`^\s*//\s*(.*)$`)
	categoryPattern := regexp.MustCompile(`^\s*//\s*([A-Z][a-zA-Z\s]+)\s*(configuration|settings)?$`)

	inStruct := false
	currentCategory := "General"
	pendingComment := ""

	for scanner.Scan() {
		line := scanner.Text()

		// Detect struct start
		if strings.Contains(line, "type Config struct") {
			inStruct = true
			continue
		}

		// Detect struct end
		if inStruct && strings.TrimSpace(line) == "}" {
			inStruct = false
			continue
		}

		if !inStruct {
			continue
		}

		// Check for category comment
		if match := categoryPattern.FindStringSubmatch(line); match != nil {
			currentCategory = strings.TrimSpace(match[1])
			pendingComment = ""
			continue
		}

		// Check for regular comment
		if match := commentPattern.FindStringSubmatch(line); match != nil {
			if pendingComment != "" {
				pendingComment += " "
			}
			pendingComment += match[1]
			continue
		}

		// Check for struct field
		if match := structFieldPattern.FindStringSubmatch(line); match != nil {
			fieldName := match[1]
			fieldType := match[2]
			tags := match[3]
			inlineComment := match[4]

			// Skip unexported fields
			if fieldName[0] >= 'a' && fieldName[0] <= 'z' {
				pendingComment = ""
				continue
			}

			// Convert CamelCase to SCREAMING_SNAKE_CASE for env var name
			envName := camelToScreamingSnake(fieldName)

			// Determine if required
			required := strings.Contains(tags, `validate:"required`)

			// Extract validation rules
			validation := ""
			if strings.Contains(tags, "validate:") {
				validationPattern := regexp.MustCompile(`validate:"([^"]+)"`)
				if vm := validationPattern.FindStringSubmatch(tags); vm != nil {
					validation = vm[1]
				}
			}

			// Use inline comment or pending comment
			description := inlineComment
			if description == "" {
				description = pendingComment
			}

			envVars = append(envVars, EnvVar{
				Name:        envName,
				Type:        goTypeToSimple(fieldType),
				Required:    required,
				Description: description,
				Validation:  validation,
				Category:    currentCategory,
			})

			pendingComment = ""
		}
	}

	return envVars, scanner.Err()
}

// parseMigrations parses SQL migration files to extract table definitions
func parseMigrations(migrationsPath string) ([]DBTable, error) {
	tables := make(map[string]*DBTable)

	files, err := filepath.Glob(filepath.Join(migrationsPath, "*.sql"))
	if err != nil {
		return nil, err
	}

	// Sort files to process in order
	sort.Strings(files)

	// Patterns - handle both standard SQL and Supabase-style quoted identifiers
	// Matches: CREATE TABLE tablename, create table "public"."tablename", etc.
	createTablePattern := regexp.MustCompile(`(?i)create\s+table\s+(?:if\s+not\s+exists\s+)?(?:"[^"]+"\s*\.\s*)?(?:"([^"]+)"|(\w+))\s*\(`)
	// Matches column definitions with quoted identifiers: "column_name" type
	columnPattern := regexp.MustCompile(`(?i)^\s*"?(\w+)"?\s+(\S+)(?:\s+(.*))?$`)
	primaryKeyPattern := regexp.MustCompile(`(?i)PRIMARY\s+KEY`)
	notNullPattern := regexp.MustCompile(`(?i)NOT\s+NULL`)
	defaultPattern := regexp.MustCompile(`(?i)DEFAULT\s+([^\s,]+(?:\([^)]*\))?)`)
	uniquePattern := regexp.MustCompile(`(?i)UNIQUE`)
	// Handle quoted table names in CREATE INDEX
	createIndexPattern := regexp.MustCompile(`(?i)CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:"([^"]+)"|(\w+))\s+ON\s+(?:"[^"]+"\s*\.\s*)?(?:"([^"]+)"|(\w+))`)
	foreignKeyPattern := regexp.MustCompile(`(?i)FOREIGN\s+KEY\s*\((\w+)\)\s*REFERENCES\s+(\w+)\s*\((\w+)\)`)

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			log.Warnf("Could not read %s: %v", file, err)
			continue
		}

		content := string(data)

		// Find CREATE TABLE statements
		tableMatches := createTablePattern.FindAllStringSubmatchIndex(content, -1)
		for _, matchIdx := range tableMatches {
			// Extract table name from the match
			fullMatch := content[matchIdx[0]:matchIdx[1]]

			// Get table name - try quoted first (group 1), then unquoted (group 2)
			var tableName string
			if matchIdx[2] != -1 && matchIdx[3] != -1 {
				tableName = strings.ToLower(content[matchIdx[2]:matchIdx[3]])
			} else if matchIdx[4] != -1 && matchIdx[5] != -1 {
				tableName = strings.ToLower(content[matchIdx[4]:matchIdx[5]])
			}

			if tableName == "" {
				continue
			}

			// Find the closing parenthesis for this CREATE TABLE
			startIdx := matchIdx[1] - 1 // Start from the opening paren
			depth := 1
			endIdx := startIdx + 1
			for endIdx < len(content) && depth > 0 {
				if content[endIdx] == '(' {
					depth++
				} else if content[endIdx] == ')' {
					depth--
				}
				endIdx++
			}

			if depth != 0 {
				log.Warnf("Could not find closing paren for table %s in %s", tableName, fullMatch)
				continue
			}

			columnDefs := content[startIdx+1 : endIdx-1]

			if tables[tableName] == nil {
				tables[tableName] = &DBTable{
					Name: tableName,
				}
			}

			// Parse columns - split by newlines for cleaner parsing
			lines := strings.Split(columnDefs, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				line = strings.TrimSuffix(line, ",")
				line = strings.TrimSpace(line)

				if line == "" {
					continue
				}

				// Skip constraints
				lineUpper := strings.ToUpper(line)
				if strings.HasPrefix(lineUpper, "CONSTRAINT") ||
					strings.HasPrefix(lineUpper, "PRIMARY KEY") ||
					strings.HasPrefix(lineUpper, "FOREIGN KEY") ||
					strings.HasPrefix(lineUpper, "UNIQUE") ||
					strings.HasPrefix(lineUpper, "CHECK") {
					continue
				}

				if colMatch := columnPattern.FindStringSubmatch(line); colMatch != nil {
					colName := colMatch[1]
					colType := colMatch[2]
					rest := ""
					if len(colMatch) > 3 {
						rest = colMatch[3]
					}

					// Skip reserved words that might match
					colNameUpper := strings.ToUpper(colName)
					if colNameUpper == "PRIMARY" || colNameUpper == "FOREIGN" ||
						colNameUpper == "CONSTRAINT" || colNameUpper == "UNIQUE" ||
						colNameUpper == "CREATE" || colNameUpper == "ALTER" {
						continue
					}

					col := DBColumn{
						Name:       colName,
						Type:       strings.ToUpper(colType),
						Nullable:   !notNullPattern.MatchString(rest) && !strings.Contains(lineUpper, "NOT NULL"),
						PrimaryKey: primaryKeyPattern.MatchString(rest) || primaryKeyPattern.MatchString(line),
						Unique:     uniquePattern.MatchString(rest),
					}

					if defMatch := defaultPattern.FindStringSubmatch(rest); defMatch != nil {
						col.Default = defMatch[1]
					} else if defMatch := defaultPattern.FindStringSubmatch(line); defMatch != nil {
						col.Default = defMatch[1]
					}

					tables[tableName].Columns = append(tables[tableName].Columns, col)
				}
			}
		}

		// Find CREATE INDEX statements
		indexMatches := createIndexPattern.FindAllStringSubmatch(content, -1)
		for _, match := range indexMatches {
			// Index name can be in group 1 (quoted) or group 2 (unquoted)
			indexName := match[1]
			if indexName == "" {
				indexName = match[2]
			}

			// Table name can be in group 3 (quoted) or group 4 (unquoted)
			tableName := match[3]
			if tableName == "" {
				tableName = match[4]
			}
			tableName = strings.ToLower(tableName)

			if tableName != "" && tables[tableName] != nil {
				tables[tableName].Indexes = append(tables[tableName].Indexes, indexName)
			}
		}

		// Find FOREIGN KEY constraints
		fkMatches := foreignKeyPattern.FindAllStringSubmatch(content, -1)
		for _, match := range fkMatches {
			if len(match) >= 4 {
				// Try to find which table this belongs to by context
				// This is a simplified approach
				for tableName, table := range tables {
					if strings.Contains(strings.ToLower(content), tableName) {
						fk := fmt.Sprintf("%s -> %s(%s)", match[1], match[2], match[3])
						table.ForeignKeys = append(table.ForeignKeys, fk)
						break
					}
				}
			}
		}
	}

	// Convert map to slice
	var result []DBTable
	for _, table := range tables {
		result = append(result, *table)
	}

	// Sort tables by name
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// camelToScreamingSnake converts CamelCase to SCREAMING_SNAKE_CASE
func camelToScreamingSnake(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteRune('_')
			}
			result.WriteRune(r)
		} else {
			result.WriteRune(r - 32) // Convert to uppercase
		}
	}
	return result.String()
}

// goTypeToSimple converts Go types to simple type names
func goTypeToSimple(goType string) string {
	switch goType {
	case "string":
		return "string"
	case "int", "int64", "int32":
		return "integer"
	case "bool":
		return "boolean"
	case "float64", "float32":
		return "float"
	case "[]string":
		return "string[]"
	case "time.Duration":
		return "duration"
	default:
		return goType
	}
}
