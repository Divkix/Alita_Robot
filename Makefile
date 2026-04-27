.PHONY: run tidy vendor build lint test check-translations check-duplicates psql-prepare psql-migrate psql-status psql-rollback psql-reset psql-verify generate-docs check-docs inventory docs-dev validate-db backup-db

GO_CMD = go
GORELEASER_CMD = goreleaser
GOLANGCI_LINT_CMD = golangci-lint

# PostgreSQL Migration Variables
PSQL_SCRIPT = scripts/migrate_psql.sh
PSQL_MIGRATIONS_DIR ?= tmp/migrations_cleaned
MIGRATIONS_DIR ?= migrations

run:
	$(GO_CMD) run main.go

tidy:
	$(GO_CMD) mod tidy

vendor:
	$(GO_CMD) mod vendor

build:
	$(GORELEASER_CMD) release --snapshot --skip=publish --clean --skip=sign

lint:
	@which $(GOLANGCI_LINT_CMD) > /dev/null || (echo "golangci-lint not found, install it from https://golangci-lint.run/usage/install/" && exit 1)
	$(GOLANGCI_LINT_CMD) run

test:
	$(GO_CMD) test -v -race -coverprofile=coverage.out -coverpkg=$$(go list ./... | grep -v 'scripts/' | paste -sd, -) -count=1 -timeout 10m ./...

check-translations:
	@echo "🔍 Checking for missing translations..."
	@cd scripts/check_translations && $(GO_CMD) mod tidy && $(GO_CMD) run main.go

check-duplicates:
	@echo "🔍 Checking for duplicate code..."
	@which $(GOLANGCI_LINT_CMD) > /dev/null || (echo "golangci-lint not found, install it from https://golangci-lint.run/usage/install/" && exit 1)
	$(GOLANGCI_LINT_CMD) run --enable dupl

# PostgreSQL Migration Targets
psql-prepare:
	@echo "🔧 Preparing PostgreSQL migrations (cleaning SQL)..."
	@mkdir -p $(PSQL_MIGRATIONS_DIR)
	@for file in $(MIGRATIONS_DIR)/*.sql; do \
		filename=$$(basename "$$file"); \
		echo "  Processing $$filename..."; \
		sed -E '/(grant|GRANT).*(anon|authenticated|service_role)/d' "$$file" | \
		sed 's/ with schema "extensions"//g' | \
		sed 's/create extension if not exists/CREATE EXTENSION IF NOT EXISTS/g' | \
		sed 's/create extension/CREATE EXTENSION IF NOT EXISTS/g' > "$(PSQL_MIGRATIONS_DIR)/$$filename"; \
	done
	@echo "✅ PostgreSQL migrations prepared in $(PSQL_MIGRATIONS_DIR)"
	@echo "📋 Found $$(ls -1 $(PSQL_MIGRATIONS_DIR)/*.sql 2>/dev/null | wc -l) migration files"

psql-migrate:
	@echo "🚀 Applying PostgreSQL migrations..."
	@if [ -z "$(PSQL_DB_HOST)" ] || [ -z "$(PSQL_DB_NAME)" ] || [ -z "$(PSQL_DB_USER)" ]; then \
		echo "❌ Error: Required environment variables not set"; \
		echo "   Please set: PSQL_DB_HOST, PSQL_DB_NAME, PSQL_DB_USER, PSQL_DB_PASSWORD"; \
		exit 1; \
	fi
	@chmod +x $(PSQL_SCRIPT) 2>/dev/null || true
	@bash $(PSQL_SCRIPT)

psql-status:
	@echo "📊 PostgreSQL Migration Status"
	@if [ -z "$(PSQL_DB_HOST)" ] || [ -z "$(PSQL_DB_NAME)" ] || [ -z "$(PSQL_DB_USER)" ]; then \
		echo "❌ Error: Required environment variables not set"; \
		echo "   Please set: PSQL_DB_HOST, PSQL_DB_NAME, PSQL_DB_USER, PSQL_DB_PASSWORD"; \
		exit 1; \
	fi
	@echo "🔍 Checking migration status..."
	@PGPASSWORD=$(PSQL_DB_PASSWORD) psql -h $(PSQL_DB_HOST) -p $${PSQL_DB_PORT:-5432} -U $(PSQL_DB_USER) -d $(PSQL_DB_NAME) -c \
		"SELECT version, executed_at FROM schema_migrations ORDER BY executed_at DESC;" 2>/dev/null || \
		echo "⚠️  No migrations table found. Run 'make psql-migrate' to initialize."

psql-rollback:
	@echo "⏪ Rolling back last PostgreSQL migration..."
	@if [ -z "$(PSQL_DB_HOST)" ] || [ -z "$(PSQL_DB_NAME)" ] || [ -z "$(PSQL_DB_USER)" ]; then \
		echo "❌ Error: Required environment variables not set"; \
		exit 1; \
	fi
	@echo "⚠️  Rollback functionality requires manual intervention"
	@echo "   Last applied migration:"
	@PGPASSWORD=$(PSQL_DB_PASSWORD) psql -h $(PSQL_DB_HOST) -p $${PSQL_DB_PORT:-5432} -U $(PSQL_DB_USER) -d $(PSQL_DB_NAME) -t -c \
		"SELECT version FROM schema_migrations ORDER BY executed_at DESC LIMIT 1;" 2>/dev/null

psql-reset:
	@echo "🔥 WARNING: This will DROP ALL TABLES in the database!"
	@echo "   Database: $(PSQL_DB_NAME) on $(PSQL_DB_HOST)"
	@echo "   Type 'yes' to confirm: " && read confirm && [ "$$confirm" = "yes" ] || (echo "Cancelled" && exit 1)
	@echo "💣 Resetting database..."
	@PGPASSWORD=$(PSQL_DB_PASSWORD) psql -h $(PSQL_DB_HOST) -p $${PSQL_DB_PORT:-5432} -U $(PSQL_DB_USER) -d $(PSQL_DB_NAME) -c \
		"DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
	@echo "✅ Database reset complete"

psql-verify:
	@echo "🔎 Verifying cleaned migrations are in sync"
	@TMP=$$(mktemp -d); \
	echo "Using temp dir: $$TMP"; \
	$(MAKE) --no-print-directory psql-prepare PSQL_MIGRATIONS_DIR="$$TMP"; \
	git diff --no-index --exit-code $(PSQL_MIGRATIONS_DIR) "$$TMP" || (echo "❌ Drift detected between migrations and $(PSQL_MIGRATIONS_DIR)" && exit 1)

# Documentation Targets
generate-docs:
	@echo "📚 Generating documentation..."
	@cd scripts/generate_docs && $(GO_CMD) run .
	@echo "✅ Documentation generated in docs/src/content/docs/"

check-docs:
	@echo "🔍 Checking docs generation for drift..."
	@TMP=$$(mktemp -d /tmp/alita-docs-check.XXXXXX); \
	ROOT=$$(pwd); \
	echo "  Generating docs to temp directory..."; \
	cd scripts/generate_docs && $(GO_CMD) run . -output "$$TMP"; \
	cd "$$ROOT"; \
	echo "  Comparing generated docs against current docs..."; \
	diff -rq "$$TMP" docs/src/content/docs/ > /dev/null 2>&1; \
	EXIT_CODE=$$?; \
	if [ $$EXIT_CODE -eq 0 ]; then \
		echo "✅ No drift detected — generated docs match current docs."; \
	else \
		echo "❌ Drift detected! Generated docs differ from current docs:"; \
		diff -r "$$TMP" docs/src/content/docs/ || true; \
		echo ""; \
		echo "Run 'make generate-docs' to sync."; \
	fi; \
	rm -rf "$$TMP"; \
	exit $$EXIT_CODE

inventory:
	@echo "Generating canonical command inventory..."
	@cd scripts/generate_docs && $(GO_CMD) run . -inventory
	@echo "Inventory written to .planning/INVENTORY.json and .planning/INVENTORY.md"

docs-dev:
	@echo "🚀 Starting Astro dev server..."
	@cd docs && bun run dev

# Database validation and backup
validate-db:
	@echo "🔍 Validating database for orphaned records..."
	@go run scripts/validate_orphaned_data.go

backup-db:
	@echo "💾 Creating database backup..."
	@./scripts/backup_database.sh
