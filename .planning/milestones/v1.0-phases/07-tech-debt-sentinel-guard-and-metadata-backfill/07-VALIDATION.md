---
phase: 7
slug: tech-debt-sentinel-guard-and-metadata-backfill
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-02-28
completed: 2026-02-28
---

# Phase 7 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go built-in `testing` package |
| **Config file** | None — standard `go test` |
| **Quick run command** | `cd scripts/generate_docs && go test ./...` |
| **Full suite command** | `cd scripts/generate_docs && go test -v -race ./...` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd scripts/generate_docs && go test ./...`
- **After every plan wave:** Run `cd scripts/generate_docs && go test -v -race ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 07-01-01 | 01 | 1 | SC-1 | unit | `cd scripts/generate_docs && go test -run TestGenerateModuleDocs_SkipsManuallyMaintainedFiles ./...` | generators_test.go | ✅ green |
| 07-01-02 | 01 | 1 | SC-1 | unit | `cd scripts/generate_docs && go test -run TestGenerateModuleDocs_WritesNonSentinelFiles ./...` | generators_test.go | ✅ green |
| 07-01-03 | 01 | 1 | SC-2/3 | integration | `make generate-docs && git status --short docs/src/content/docs/commands/*/index.md` | N/A | ✅ green |
| 07-02-01 | 02 | 2 | SC-4 | smoke | `grep -r "^requirements-completed:" .planning/phases/ && echo FAIL \|\| echo PASS` | N/A | ✅ green |
| 07-02-02 | 02 | 2 | SC-5 | smoke | `git status --short "docs/src/content/docs/commands/*/index.md" \| grep "^.M" && echo FAIL \|\| echo PASS` | N/A | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `scripts/generate_docs/generators_test.go` — stubs for SC-1 (sentinel guard skip/write tests)

*Existing `parsers_test.go` pattern applies directly — same package, tmpDir approach.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| All 21 module pages contain sentinel comment | SC-2 | File content check across 21 files | `grep -rL "MANUALLY MAINTAINED: do not regenerate" docs/src/content/docs/commands/*/index.md \| grep -v -E "(devs\|help\|users)"` (expect: no output) |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 5s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** complete
