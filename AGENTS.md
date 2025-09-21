# Repository Guidelines

## Project Structure & Module Organization
Core source lives in `alita/`, split into `modules` for bot features, `utils` for shared helpers, `config` for runtime settings, `health`/`metrics` for probes, and `db` for repositories. `main.go` wires the bot entrypoint. Localized strings sit in `locales/` (YAML) and should be updated with every user-facing change. Database schema changes go in `supabase/migrations`, while automation scripts reside in `scripts/`. Build artifacts end up in `dist/`, and supplemental docs live under `docs/`.

## Build, Test, and Development Commands
- `make run`: start the bot locally using Go modules from `main.go`.
- `make lint`: run `golangci-lint`; required before every PR.
- `make build`: produce a snapshot release via GoReleaser, matching the CI pipeline.
- `go test ./...`: execute unit tests across all packages.
- `make check-translations`: ensure new keys exist across locale bundles.
For container workflows, use `docker-compose -f local.docker-compose.yml up` to launch Postgres, Redis, and the bot together.

## Coding Style & Naming Conventions
Format Go code with `gofmt` (tabs, exported identifiers in PascalCase, package-scope helpers in lowerCamelCase). Keep module files focused: each feature lives in a dedicated file under `alita/modules`. Reuse constants from `alita/utils/constants` and prefer structured errors from `alita/utils/errors`. Run `golangci-lint` locally; extend its configuration rather than silencing warnings inline.

## Testing Guidelines
Tests belong next to their packages as `*_test.go` files using Go’s standard testing toolkit. Target new logic with table-driven cases and add mocks under `alita/utils` only when shared. Aim for meaningful coverage on critical paths (moderation flows, cache layers). Combine `go test ./...` with `make lint` in CI; keep tests hermetic so they run without external services unless flagged with build tags.

## Commit & Pull Request Guidelines
Follow Conventional Commits (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`) as already used in history. Commits should be small, focused, and include locale updates or migrations when applicable. PRs must describe intent, list validation commands (`make lint`, `go test ./...`), and link issues or spec discussions. Attach screenshots or Telegram command transcripts when altering user-visible behavior, and note any config or migration steps in the PR body.

## Security & Configuration Tips
Never commit real tokens; base your `.env` on `sample.env`. When touching database logic, run `make psql-prepare` to sync cleaned migrations and document secrets required for Supabase. Review `docker/` templates before deploying and ensure webhook URLs or API keys are provided through environment variables.
