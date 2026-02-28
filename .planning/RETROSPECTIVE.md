# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.0 — Documentation & Command Consistency Audit

**Shipped:** 2026-02-28
**Phases:** 7 | **Plans:** 18 | **Sessions:** ~4

### What Was Built
- Canonical command inventory: 142 commands across 25 modules (INVENTORY.json/md)
- Full docs coverage: 4 new module pages (devs, help, users, languages), api-reference rewrite
- Locale remediation: all 4 locale files (EN/ES/FR/HI) at 838 keys, zero orphans
- Operator documentation: handler group precedence, anonymous admin flow, dev access tiers
- Sentinel-protected docs generator: 23 files protected from generator overwrites
- README alignment: project structure, command counts, env vars all corrected

### What Worked
- **TDD-first approach**: Every tooling fix (check-translations, parsers.go, camelToScreamingSnake) started with a failing test — caught regressions early
- **Canonical inventory as ground truth**: Building INVENTORY.json first gave every subsequent phase a single source to validate against
- **Sentinel pattern**: Simple `<!-- MANUALLY MAINTAINED -->` comment solved the generator overwrite problem elegantly without config files
- **3-audit cycle**: The milestone audit process caught the Phase 5 regression (generate-docs overwrote Phase 2 edits) and drove Phase 6 and 7 gap closure
- **Strict dependency ordering**: Phases 2+3 depended on Phase 1 tooling fixes; this prevented wasted work

### What Was Inefficient
- **Phase 5 regression**: `make generate-docs` in Phase 5 silently overwrote Phase 2's manual api-reference edits — required Phase 6 to restore. Should have added sentinel protection earlier
- **SUMMARY metadata inconsistency**: Used `requirements-completed` (hyphen) initially, had to rename to `requirements_completed` (underscore) across 14 files in Phase 7
- **Duplicate work on commands.md**: Written manually in Phase 2, overwritten in Phase 5, restored in Phase 6 — touched 3 times

### Patterns Established
- **Sentinel comment protection**: `<!-- MANUALLY MAINTAINED — DO NOT OVERWRITE -->` in generated files; generators check and skip
- **3-source requirements verification**: VERIFICATION.md + SUMMARY frontmatter + REQUIREMENTS.md traceability table
- **Canonical inventory-first**: Build the ground truth data file before touching any documentation surface
- **Gap closure as dedicated phases**: When audit finds regressions, create explicit phases (6, 7) rather than patching inline

### Key Lessons
1. **Generators and manual edits don't coexist without guards** — any pipeline that writes files must check for hand-maintained content before overwriting
2. **Audit early, audit often** — the 3-audit cycle (gaps_found → tech_debt → passed) was expensive but caught real regressions that would have shipped broken
3. **YAML frontmatter keys should use underscores** — Go and YAML ecosystems use snake_case; hyphens cause inconsistency
4. **Fix tooling before content** — the check-translations path bug would have invalidated all Phase 3 locale verification if not caught first

### Cost Observations
- Model mix: ~80% opus, ~15% sonnet, ~5% haiku (balanced profile)
- Sessions: ~4 (one long session covered phases 1-7)
- Notable: ~9 hours wall clock for 18 plans across 7 phases — ~30 min/plan average including research, planning, execution, and verification

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Sessions | Phases | Key Change |
|-----------|----------|--------|------------|
| v1.0 | ~4 | 7 | Established TDD + sentinel + 3-source verification pattern |

### Cumulative Quality

| Milestone | Tests Added | Docs Pages | Files Protected |
|-----------|-------------|------------|-----------------|
| v1.0 | 5 | 8 new pages | 23 sentinel-guarded |

### Top Lessons (Verified Across Milestones)

1. Generators must respect manual edits — sentinel guard pattern is the solution
2. Canonical data files before documentation surfaces — prevents drift from the start
