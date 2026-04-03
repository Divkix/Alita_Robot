# Validation Surface

**What belongs here:** Testing approach for bug fixes.

---

## Testing Strategy for Bug Fixes

### Unit Testing
Each bug fix should have corresponding test coverage:
- Add test case that reproduces the bug
- Verify fix with `go test -run TestXxx ./package`
- Run with race detector: `go test -race ./package`

### Static Analysis
- Run `make lint` (golangci-lint)
- Run `go vet ./...`
- Ensure gofmt formatting

### Race Detection
Critical for concurrency fixes:
- Run `go test -race ./...` after all fixes
- Must show 0 races detected

### Regression Testing
- Run full test suite: `make test`
- All original tests must pass
- Coverage should not decrease

## Test Commands by Package

```bash
# Modules
go test -v ./alita/modules
go test -race ./alita/modules

# Database
go test -v ./alita/db
go test -race ./alita/db

# Utils
go test -v ./alita/utils/...
go test -race ./alita/utils/...

# Config
go test -v ./alita/config
go test -race ./alita/config

# i18n
go test -v ./alita/i18n
go test -race ./alita/i18n

# All
go test -race ./...
make test
```

## Resource Cost Classification

Bug fixes are lightweight:
- Each test run: ~1-2 minutes
- Memory usage: <1GB
- CPU: Single core sufficient
- Race detector adds ~2x time

## Flow Validator Guidance: Main-Fixes Code Review Surface

For main-fixes validation, the testing approach is code review + static analysis.

### Testing Approach
1. **Code Review**: Read source files to verify initialization order, goroutine handling, and error checking
2. **Static Analysis**: Run `make lint` and `go vet` on modified packages
3. **Build Verification**: Run `go build ./...` to ensure no compilation errors

### Assertion Groups

**Group 1: Main Initialization (main.go)**
- VAL-HIGH-002: Activity monitor starts AFTER database init
- VAL-HIGH-004: i18n cache uses validated manager (or proper nil check)
- VAL-MED-005: Health check uses default port fallback

**Group 2: Webhook & HTTP Server (httpserver/)**
- VAL-HIGH-003: Webhook goroutines have timeout context
- VAL-LOW-002: Webhook secret handling consistent between main.go and httpserver
- VAL-MED-006: HTTP server start properly handles errors after timeout

**Group 3: Config (alita/config/)**
- VAL-HIGH-012: Type conversion errors are logged, not silently ignored

### Verification Pattern
For each assertion:
1. Read the source file(s) mentioned in validation-contract.md
2. Locate the specific code pattern (e.g., activityMonitor.Start() call)
3. Verify it matches the expected behavior
4. Record: PASS if pattern found, FAIL if missing or wrong

### Isolation
Each validator reads from the codebase (read-only)
No shared state between validators - safe to run concurrently

For module-fixes validation, the "user surface" is the Go test suite and code verification.

### Testing Approach
1. **Code Review**: Read the relevant source files to verify fixes are applied
2. **Test Execution**: Run `go test -race` and `make test` to verify behavior
3. **Static Analysis**: Run `make lint` to ensure code quality

### Isolation Strategy
- Each validator operates on read-only codebase analysis
- Tests can run concurrently (different packages don't interfere)
- No shared mutable state between validators

### Assertion Testing Pattern
For each assertion:
1. Read the relevant source file(s)
2. Verify the fix pattern matches validation-contract.md specification
3. Run targeted tests: `go test -v -race ./alita/modules` or specific package
4. Record evidence: test output, code snippets showing fix
5. Report: PASS if fix verified, FAIL if not found, BLOCKED if prerequisite broken

### Commands
```bash
# Module tests
go test -v -race ./alita/modules

# Specific test patterns
go test -v -race -run "TestFilter|TestBlacklist|TestReport" ./alita/modules

# Lint check
make lint

# Full test suite
make test
```

### Output Location
## Flow Validator Guidance: Utils-Fixes Code Review Surface

For utils-fixes validation, the testing approach combines code review with targeted race detector tests.

### Testing Approach
1. **Code Review**: Read the relevant source files to verify fixes are applied
2. **Test Execution**: Run `go test -race` on specific packages
3. **Static Analysis**: Run `make lint` to ensure code quality

### Isolation Strategy
- Each validator operates on read-only codebase analysis
- Tests can run concurrently (different packages don't interfere)
- Race detector tests run independently per package

### Assertion Testing Pattern
For each assertion:
1. Read the relevant source file(s)
2. Verify the fix pattern matches validation-contract.md specification
3. Run targeted tests: `go test -v -race ./alita/utils/...`
4. Record evidence: test output, code snippets showing fix
5. Report: PASS if fix verified, FAIL if not found, BLOCKED if prerequisite broken

### Commands
```bash
# Utils tests
go test -v -race ./alita/utils/monitoring

# Helpers tests

# Keyword matcher tests

# Extraction tests

# Tracing tests

# Chat status tests

# Lint check
make lint

# Full test suite
make test
```

### Output Location
Write JSON report to: `.factory/validation/utils-fixes/user-testing/flows/<group-id>.json`
Save evidence: Code snippets in report, test output captured
