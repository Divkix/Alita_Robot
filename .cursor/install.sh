#!/usr/bin/env bash
# Idempotent repository bootstrap for the Alita Robot Cloud Agent environment.
# Installs the system services (PostgreSQL, Redis), the pinned linter, and Go
# module dependencies. Services themselves are started per-boot in start.sh.
set -euo pipefail

GOLANGCI_LINT_VERSION="v2.11.4" # keep in sync with .pre-commit-config.yaml / CI

echo "==> [install] Installing system packages (PostgreSQL, Redis, tooling)"
sudo DEBIAN_FRONTEND=noninteractive apt-get update -y
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
  postgresql \
  postgresql-client \
  redis-server \
  ca-certificates \
  perl \
  git

echo "==> [install] Installing golangci-lint ${GOLANGCI_LINT_VERSION}"
if ! golangci-lint version 2>/dev/null | grep -q "${GOLANGCI_LINT_VERSION#v}"; then
  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
    | sudo sh -s -- -b /usr/local/bin "${GOLANGCI_LINT_VERSION}"
fi

echo "==> [install] Downloading Go modules"
go mod download

echo "==> [install] Warming the build cache (go build ./...)"
go build ./...

echo "==> [install] Done"
