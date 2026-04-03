#!/bin/bash
# Idempotent environment setup for Alita Robot bug fixing

set -e

echo "Setting up Alita Robot development environment..."

# Check Go version
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
REQUIRED_VERSION="1.22"

if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$REQUIRED_VERSION" ]; then
    echo "Warning: Go version $GO_VERSION may be too old. Recommended: 1.25+"
fi

# Install dependencies
echo "Installing Go dependencies..."
go mod download

# Verify tools
echo "Verifying development tools..."

if ! command -v golangci-lint &> /dev/null; then
    echo "Warning: golangci-lint not found. Install with:"
    echo "  brew install golangci-lint  # macOS"
    echo "  or see: https://golangci-lint.run/usage/install/"
fi

echo "Environment setup complete!"
echo ""
echo "Available commands:"
echo "  make test    - Run all tests with race detection"
echo "  make lint    - Run golangci-lint"
echo "  make build   - Build the project"
echo "  make tidy    - Run go mod tidy"
