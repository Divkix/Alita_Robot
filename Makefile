.PHONY: run tidy vendor build lint test check-translations check-duplicates validate-db release

GO_CMD = go
GOLANGCI_LINT_CMD = golangci-lint

run:
	$(GO_CMD) run main.go

tidy:
	$(GO_CMD) mod tidy

vendor:
	$(GO_CMD) mod vendor

build:
	$(GO_CMD) build -trimpath -ldflags="-s -w" .

lint:
	@which $(GOLANGCI_LINT_CMD) > /dev/null || (echo "golangci-lint not found, install it from https://golangci-lint.run/usage/install/" && exit 1)
	$(GOLANGCI_LINT_CMD) run

test:
	$(GO_CMD) test -tags testtools -v -race -coverprofile=coverage.out -coverpkg=$$(go list ./... | grep -v -E '(^github.com/divkix/Alita_Robot$$|scripts/)' | paste -sd, -) -count=1 -timeout 10m ./...

check-translations:
	@echo "🔍 Checking for missing translations..."
	@cd scripts/check_translations && $(GO_CMD) mod tidy && $(GO_CMD) run main.go

check-duplicates:
	@echo "🔍 Checking for duplicate code..."
	@which $(GOLANGCI_LINT_CMD) > /dev/null || (echo "golangci-lint not found, install it from https://golangci-lint.run/usage/install/" && exit 1)
	$(GOLANGCI_LINT_CMD) run --enable dupl

# Database validation
validate-db:
	@echo "🔍 Validating database for orphaned records..."
	@go run scripts/validate_orphaned_data.go

# Release
release:
	@./scripts/release.sh $(TYPE)
