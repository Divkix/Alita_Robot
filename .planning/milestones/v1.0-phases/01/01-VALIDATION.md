---
phase: 1
slug: ground-truth-tooling
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-02-27
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) |
| **Config file** | none — existing `go test ./...` covers project |
| **Quick run command** | `go test ./scripts/...` |
| **Full suite command** | `go test ./scripts/... && make check-translations` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./scripts/...`
- **After every plan wave:** Run `go test ./scripts/... && make check-translations`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 01-01-01 | 01 | 1 | TOOL-01 | unit + integration | `go test ./scripts/check_translations/...` | ❌ W0 | ⬜ pending |
| 01-02-01 | 02 | 1 | TOOL-02 | unit | `go test ./scripts/generate_docs/...` | ❌ W0 | ⬜ pending |
| 01-03-01 | 03 | 2 | TOOL-03 | integration | `go run ./scripts/generate_docs/ -inventory` | N/A | ⬜ pending |
| 01-04-01 | 04 | 2 | TOOL-04 | build | `cd docs && bun run build` | N/A | ⬜ pending |
| 01-05-01 | 05 | 2 | TOOL-05 | manual | Verify `.planning/INVENTORY.md` has all 22 modules | N/A | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `scripts/check_translations/main_test.go` — test stubs for path resolution fix (TOOL-01)
- [ ] `scripts/generate_docs/parsers_test.go` — test stubs for MultiCommand regex (TOOL-02)

*Existing `go test` infrastructure covers all phase requirements. No framework install needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| starlight-links-validator catches broken links | TOOL-04 | Requires Astro build with bun | Run `cd docs && bun run build` and verify link validator output |
| Module-to-docs mapping completeness | TOOL-05 | Subjective completeness check | Compare `.planning/INVENTORY.md` module list against `ls alita/modules/*.go` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
