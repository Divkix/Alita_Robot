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

# Static discovery avoids compiling/linking every test binary just to list names.
# Missed names are safe: the last shard runs everything earlier shards did not select.
discovery_start=$SECONDS
listed=$(LC_ALL=C grep -rhE --include='*_test.go' \
    --exclude-dir=vendor --exclude-dir=node_modules --exclude-dir=.worktrees \
    '^[[:space:]]*func[[:space:]]+(Test|Example|Fuzz)[[:alnum:]_]*[[:space:]]*\(' . ||
    [[ $? -eq 1 ]])
names=$(printf '%s\n' "$listed" |
    LC_ALL=C sed -E 's/^[[:space:]]*func[[:space:]]+((Test|Example|Fuzz)[[:alnum:]_]*)[[:space:]]*\(.*/\1/' |
    LC_ALL=C sort -u)
echo "Static test discovery: $((SECONDS - discovery_start))s"

selected=''
flag=-run
if (( index == count - 1 )); then
    flag=-skip
fi
position=0
while IFS= read -r name; do
    if (( (index < count - 1 && position % count == index) ||
        (index == count - 1 && position % count != index) )); then
        selected+="${selected:+|}${name}"
    fi
    position=$((position + 1))
done <<< "$names"

# Make consumes one dollar sign; the resulting Go regexp matches whole top-level names.
make --no-print-directory test TEST_ARGS="${flag} '^(${selected})\$\$'" TEST_OUTPUT=test-results.json
