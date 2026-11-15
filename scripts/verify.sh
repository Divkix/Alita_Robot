#!/usr/bin/env bash
# Local equivalent of ci.yml, in job declaration order (excluding deployment).
# VERIFY_USE_LOCAL_SERVICES=1 permits local services: alita_test must be a fresh,
# empty, throwaway database; it will be migrated and written to.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

# GitHub Actions sets CI for every step and does not set GOFLAGS.
export CI=true
unset GOFLAGS

shard_dir=''
results=()
containers=()
cleanup() {
    local status=$?
    trap - EXIT
    if ((${#containers[@]})); then
        docker rm -f "${containers[@]}" >/dev/null 2>&1 || true
    fi
    if [[ -n $shard_dir ]]; then
        rm -rf "$shard_dir"
    fi
    printf '\nVerification summary:\n'
    if ((${#results[@]})); then
        printf '  %s\n' "${results[@]}"
    fi
    exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

run() {
    local step=$1
    shift
    printf '\n==> [%s] %s\n' "$step" "$*"
    "$@"
    results+=("PASS: $step")
}
skip() {
    printf 'SKIPPED: %s, needs %s\n' "$1" "$2"
    results+=("SKIPPED: $1 (needs $2)")
}
has() { command -v "$1" >/dev/null 2>&1; }
docker_ready() { has docker && docker info >/dev/null 2>&1; }

# stable means the Go selected by PATH, as requested; never install Go here.
if has go; then
    go version
else
    echo 'Go is unavailable; Go checks will be skipped.'
fi

# security: the action installs @latest, then uses these default arguments.
# No remote vulnerability-DB requests: provide a pre-populated file:// DB.
if ! has go; then
    skip security 'Go on PATH'
elif [[ ${GOVULNDB:-} != file://* || ! -d ${GOVULNDB#file://} ]]; then
    skip security 'a local vulnerability database (GOVULNDB=file:///path/to/db); network DB access is disabled'
else
    if ! has govulncheck; then
        run security-install env GOOS= GOARCH= go install golang.org/x/vuln/cmd/govulncheck@latest
        tool_bin=$(go env GOBIN)
        export PATH="${tool_bin:-$(go env GOPATH)/bin}:$PATH"
    fi
    run security govulncheck -C . -format text ./...
fi

# lint: binary installation is used in CI, so do not substitute go install.
if has go; then
    run lint-tidy go mod tidy -diff
else
    skip lint-tidy 'Go on PATH'
fi
lint_available=false
if has golangci-lint && lint_version=$(golangci-lint version); then
    if [[ $lint_version =~ version[[:space:]]v?2\.15\.0([[:space:]]|$) ]]; then
        lint_available=true
    fi
fi
if ! "$lint_available"; then
    skip lint 'golangci-lint v2.15.0'
elif has go; then
    run lint golangci-lint run --timeout 10m
else
    skip lint 'Go on PATH'
fi
if has go && has make; then
    run lint-nilaway make nilaway
else
    skip lint-nilaway 'Go and make'
fi
echo 'n/a: Technical Debt Tracking (GitHub step-summary report only)'
base=''
if has git; then
    base=$(git merge-base HEAD origin/main 2>/dev/null || git merge-base HEAD main 2>/dev/null || true)
fi
if ! "$lint_available"; then
    skip lint-new-issues 'golangci-lint v2.15.0'
elif [[ -z $base ]]; then
    skip lint-new-issues 'a local merge base with origin/main or main'
elif has go && "$lint_available"; then
    run lint-new-issues golangci-lint run --enable-only dupl,gocyclo --timeout 10m --new-from-rev "$base"
else
    skip lint-new-issues 'Go on PATH'
fi

echo 'n/a: Cyclomatic Complexity Analysis (GitHub step-summary report only)'

build() {
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /tmp/alita_robot .
    (cd /tmp && ./alita_robot --version)
}
if has go; then
    run build build
else
    skip build 'Go on PATH'
fi

# Share CI services between the two jobs. Only containers created here are removed.
port_ready() { (exec 3<>"/dev/tcp/localhost/$1") 2>/dev/null; }
postgres_ready() {
    if has psql; then
        PGPASSWORD=postgres PGCONNECT_TIMEOUT=2 psql \
            'postgres://postgres@localhost:5432/alita_test?sslmode=disable' \
            -Atqc 'SELECT 1' >/dev/null 2>&1
    elif [[ -n ${pg_name:-} ]]; then
        docker exec "$pg_name" pg_isready -h 127.0.0.1 -U postgres -d alita_test >/dev/null 2>&1
    else
        port_ready 5432
    fi
}
redis_ready() {
    local reply
    (exec 3<>/dev/tcp/localhost/6379
        printf '*1\r\n$4\r\nPING\r\n' >&3
        IFS= read -r -t 2 reply <&3
        [[ $reply == $'+PONG\r' ]]) 2>/dev/null
}
services=false
if docker_ready; then
    pg_name="alita-verify-postgres-$$"
    if ! docker run -d --name "$pg_name" -p 5432:5432 \
        -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres \
        -e POSTGRES_DB=alita_test postgres:18; then
        echo 'ERROR: could not start fresh PostgreSQL container; localhost:5432 must be free' >&2
        exit 1
    fi
    containers+=("$pg_name")
    redis_name="alita-verify-redis-$$"
    if ! docker run -d --name "$redis_name" -p 6379:6379 redis:8; then
        echo 'ERROR: could not start fresh Redis container; localhost:6379 must be free' >&2
        exit 1
    fi
    containers+=("$redis_name")
    for _ in {1..30}; do
        if postgres_ready && redis_ready; then
            services=true
            break
        fi
        sleep 2
    done
    if ! "$services"; then
        echo 'ERROR: PostgreSQL/Redis did not become ready in time' >&2
        exit 1
    fi
elif [[ ${VERIFY_USE_LOCAL_SERVICES:-} == 1 ]]; then
    if postgres_ready && redis_ready; then
        services=true
    else
        echo 'ERROR: opted-in local PostgreSQL/Redis services are not ready at CI addresses' >&2
        exit 1
    fi
fi
service_need='Docker, or VERIFY_USE_LOCAL_SERVICES=1 with a throwaway PostgreSQL 18 (alita_test) and Redis on localhost'
# Apply job env in a subshell so it cannot leak into later jobs. Clear inherited
# test selectors: CI enables each only in its corresponding step.
ci_test() (
    export BOT_TOKEN=test-token OWNER_ID=1 MESSAGE_DUMP=1
    export REDIS_ADDRESS=localhost:6379
    export DATABASE_URL='postgres://postgres:postgres@localhost:5432/alita_test?sslmode=disable'
    export AUTO_MIGRATE=false USE_WEBHOOKS=false DEBUG=false CGO_ENABLED=1
    unset ALITA_TEST_DATABASE ALITA_TEST_MIGRATION_CHAIN TEST_ARGS TEST_OUTPUT
    "$@"
)
if ! "$services"; then
    skip postgres-migration-chain "$service_need"
    skip postgres-integrity "$service_need"
elif ! has go || ! has make || ! has gcc; then
    skip postgres-migration-chain 'Go, make and a C compiler (CGO_ENABLED=1)'
    skip postgres-integrity 'Go, make and a C compiler (CGO_ENABLED=1)'
else
    run postgres-migration-chain ci_test env ALITA_TEST_MIGRATION_CHAIN=true \
        go test -tags testtools -v -count=1 -run '^TestRepositoryMigrationChain$' ./alita/db/migrations
    run postgres-integrity ci_test env ALITA_TEST_DATABASE=true make test-postgres-integrity
fi

# test-shards: keep ALITA_TEST_DATABASE unset as in CI.
tests_ran=false
if ! "$services"; then
    skip test-shards "$service_need"
elif has go && has make && has gcc; then
    shard_dir=$(mktemp -d)
    for i in {0..3}; do
        run "test-shard-$i" ci_test scripts/test_shard.sh "$i" 4
        mkdir "$shard_dir/coverage-shard-$i"
        mv coverage.out test-results.json "$shard_dir/coverage-shard-$i/"
    done
    tests_ran=true
else
    skip test-shards 'Go, make and a C compiler (CGO_ENABLED=1)'
fi
merge_coverage() {
    local profiles=("$shard_dir"/coverage-shard-*/coverage.out)
    local profile mode
    test "${#profiles[@]}" -eq 4
    for profile in "${profiles[@]}"; do
        test -s "$profile"
        IFS= read -r mode < "$profile"
        test "$mode" = "mode: atomic"
    done
    {
        echo "mode: atomic"
        for profile in "${profiles[@]}"; do
            tail -n +2 "$profile"
        done
    } > coverage.out
}
coverage() {
    if [[ ! -f coverage.out ]]; then
        echo 'ERROR: coverage.out not found'
        exit 1
    fi
    local THRESHOLD=78 COVERAGE BELOW mode
    IFS= read -r mode < coverage.out
    test "$mode" = 'mode: atomic'
    COVERAGE=$(go tool cover -func=coverage.out | grep '^total:' | awk '{print $3}' | tr -d '%')
    BELOW=$(awk "BEGIN {print ($COVERAGE < $THRESHOLD) ? 1 : 0}")
    if [[ $BELOW -eq 1 ]]; then
        echo "Coverage ${COVERAGE}% is below threshold ${THRESHOLD}%"
        exit 1
    fi
    echo "PASS: Coverage ${COVERAGE}% meets threshold ${THRESHOLD}%"
}
if "$tests_ran"; then
    run test-coverage-merge merge_coverage
    run test-coverage coverage
else
    skip test-coverage 'coverage output from this run of test-shards'
fi

if has go && has make; then
    run docs-translations make check-translations
    run docs-drift make check-docs
else
    skip docs-translations 'Go and make'
    skip docs-drift 'Go and make'
fi
docs_links() { (cd docs && bun run check:links); }
if has bun; then
    run docs-links docs_links
else
    skip docs-links 'Bun on PATH (CI uses setup-bun without a version pin)'
fi

# check-formal downloads TLC if missing. Require its verified cache instead,
# preserving the no-network rule; the target owns the pinned image and flags.
jar="${XDG_CACHE_HOME:-$HOME/.cache}/alita/tla2tools-1.7.4.jar"
jar_sha='936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88'
sha256() {
    if has sha256sum; then sha256sum "$1"; else shasum -a 256 "$1"; fi
}
if ! docker_ready || ! has make; then
    skip formal 'Docker and make'
elif [[ ! -f $jar ]] || { ! has sha256sum && ! has shasum; }; then
    skip formal 'cached TLC 1.7.4 jar and sha256sum or shasum (network download disabled)'
elif [[ $(sha256 "$jar" | cut -d' ' -f1) != "$jar_sha" ]]; then
    skip formal 'cached TLC 1.7.4 jar with the CI SHA-256 (network download disabled)'
else
    run formal make check-formal
fi
if docker_ready && docker buildx version >/dev/null 2>&1; then
    run docker-build docker buildx build --file docker/alpine --platform linux/amd64 --load --tag test-build:latest .
    run docker-verify docker image inspect test-build:latest
else
    skip docker-build 'Docker with Buildx'
    skip docker-verify 'Docker with Buildx'
fi
