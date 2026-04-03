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

## Flow Validator Guidance: Go Test Surface

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
Write JSON report to: `.factory/validation/module-fixes/user-testing/flows/<group-id>.json`
Save evidence: Code snippets in report, test output captured
