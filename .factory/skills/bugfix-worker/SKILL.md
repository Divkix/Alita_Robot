# Bug Fix Worker Skill

This skill is for fixing bugs in the Alita Robot Go codebase.

---
name: bugfix-worker
description: Fix specific bugs in the Alita Robot Go Telegram bot codebase following CLAUDE.md patterns and guidelines.
---

## When to Use This Skill

Use this skill when:
- Fixing nil pointer dereferences
- Adding error handling to goroutines
- Fixing race conditions
- Correcting error checking patterns
- Adding missing nil checks
- Fixing cache invalidation issues
- Any bug fix task in the codebase

## Required Skills

None - this is a code analysis and editing skill.

## Work Procedure

### 1. Read and Understand
- Read the feature description carefully
- Read the relevant source files mentioned
- Understand the bug and its context
- Check CLAUDE.md for relevant patterns

### 2. Locate the Bug
- Use Grep to find specific code patterns
- Read the exact lines that need fixing
- Understand surrounding code for context

### 3. Implement the Fix
- Make minimal changes to fix the bug
- Follow existing code style and patterns
- Add nil checks where needed
- Add error handling where needed
- Ensure cache invalidation patterns are correct

### 4. Add or Update Tests
- If test exists for this scenario, update it
- If no test exists, add a test case
- Ensure test covers the bug scenario
- Run the specific test to verify

### 5. Verify the Fix
- Run: `go test -v -run TestXxx ./package` for affected package
- Run: `go test -race ./package` for race detection
- Run: `go build ./...` to ensure compiles
- Run: `make lint` if available

### 6. Document Changes
- Comment complex fixes
- Ensure commit message describes the bug and fix

## Example Handoff

```json
{
  "salientSummary": "Fixed nil pointer dereference in admin.go demote() by adding nil check after GetMember(). Added test case to verify graceful handling of nil member returns.",
  "whatWasImplemented": "Added nil check for userMember after chat.GetMember() call in admin.go demote() function. When GetMember returns nil, the function now returns an error instead of panicking. Added TestDemoteNilMember test case to verify the fix.",
  "whatWasLeftUndone": "",
  "verification": {
    "commandsRun": [
      {"command": "go test -v -run TestDemote ./alita/modules", "exitCode": 0, "observation": "Test passed including new nil member test"},
      {"command": "go test -race ./alita/modules", "exitCode": 0, "observation": "No race conditions detected"},
      {"command": "go build ./...", "exitCode": 0, "observation": "Build successful"}
    ]
  },
  "discoveredIssues": []
}
```

## When to Return to Orchestrator

- The bug is more complex than described and requires architectural changes
- The fix would break existing functionality
- Multiple files need coordinated changes that span multiple areas
- The bug reveals deeper issues that need mission-level decisions
- You discover additional related bugs that should be tracked separately

## Common Bug Patterns in This Codebase

### Nil Pointer Patterns
- Always check `ctx.EffectiveSender` before accessing `.User`
- Check `msg.GetChat()` before accessing `.Id`
- Check DB results before accessing fields
- Check callback query message before accessing

### Error Handling Patterns
- Never ignore DB errors with `_`
- Use `error_handling.RecoverFromPanic()` in goroutines
- Check `result.Error` not `err.Error` for GORM operations
- Log errors in goroutines, don't silently discard

### Cache Patterns
- Cache key format: `alita:{module}:{identifier}`
- Always invalidate cache after successful DB writes
- Invalidate after error check, not before
- Use `deleteCache()` helper

### Integer Handling
- Use `strconv.ParseInt(str, 10, 64)` for user IDs, not `Atoi`
- Telegram user IDs can exceed 32-bit range
- Check for parsing errors

### Goroutine Patterns
- Always use `defer error_handling.RecoverFromPanic()`
- Handle errors, don't fire-and-forget
- Consider cleanup/stop mechanisms
- Be careful with variable capture in closures
