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

## Testing Checklist

Before considering a bug fix complete:
- [ ] Test added/updated for the bug scenario
- [ ] `go test -race ./package` passes
- [ ] `make lint` passes
- [ ] `go build ./...` succeeds
- [ ] No regressions in existing tests
