#!/usr/bin/env bash
# Run a round-robin shard of top-level tests, examples, and fuzz seed corpora.
set -euo pipefail

if [[ $# -ne 2 ]] || [[ ! $1 =~ ^(0|[1-9][0-9]*)$ || ! $2 =~ ^[1-9][0-9]*$ ]] ||
    [[ ${#1} -gt 18 || ${#2} -gt 18 ]] || (( $1 >= $2 )); then
    echo "usage: $0 <zero-based-index> <count> (decimal integers, at most 18 digits; 0 <= index < count)" >&2
    exit 1
fi
index=$1
count=$2
cd "$(dirname "${BASH_SOURCE[0]}")/.."

# Reuse make test so discovery and execution compile identical race/coverage binaries.
discovery_start=$SECONDS
if ! listed=$(make --no-print-directory test TEST_ARGS='-list .'); then
    printf '%s\n' "$listed" >&2
    exit 1
fi
echo "Test discovery (compile and TestMain): $((SECONDS - discovery_start))s"
if ! names=$(printf '%s\n' "$listed" | grep -E '^(Test|Example|Fuzz)[^[:space:]]*$' | LC_ALL=C sort -u); then
    echo "test_shard: no top-level tests found" >&2
    exit 1
fi

selected=''
position=0
while IFS= read -r name; do
    if (( position % count == index )); then
        selected+="${selected:+|}${name}"
    fi
    position=$((position + 1))
done <<< "$names"
if [[ -z $selected ]]; then
    echo "test_shard: shard $index/$count selected no tests" >&2
    exit 1
fi

# Make consumes one dollar sign; the resulting Go regexp matches whole top-level names.
make --no-print-directory test TEST_ARGS="-run '^(${selected})\$\$'" TEST_OUTPUT=test-results.json
