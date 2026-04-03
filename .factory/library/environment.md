# Environment

**What belongs here:** Required env vars, external dependencies, and setup notes for bug fixing.
**What does NOT belong here:** Service ports/commands (use `.factory/services.yaml`).

---

## Development Environment

### Required Tools
- Go 1.25+ (as specified in go.mod)
- golangci-lint (for linting)
- make (for running commands)

### Build Commands
- `make run` - Run the bot locally
- `make build` - Multi-platform release build
- `make test` - Run all tests with race detection
- `make lint` - Run golangci-lint
- `make tidy` - Run go mod tidy

### Test Commands
- `go test -v -run TestFunctionName ./package` - Run specific test
- `go test -race ./package` - Run with race detector
- `go test -v ./...` - Run all tests verbose

## Bug Fix Specific Notes

### Before Making Changes
1. Read CLAUDE.md for code patterns
2. Read the relevant source files
3. Understand the bug context
4. Check for existing tests

### Testing Bug Fixes
1. Add or update test for the bug scenario
2. Run specific package tests first
3. Run with race detector: `go test -race ./package`
4. Run full test suite: `make test`
5. Run lint: `make lint`

### Common Files to Check
- Database: `alita/db/*_db.go`
- Modules: `alita/modules/*.go`
- Utils: `alita/utils/*/*.go`
- Main: `main.go`, `alita/main.go`
- Config: `alita/config/*.go`

## External Dependencies

None required for bug fixing - all changes are internal code fixes.
