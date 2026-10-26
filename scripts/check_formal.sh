#!/usr/bin/env bash
# Model-check the TLA+ specs in formal/ with TLC, run in Docker:
#   scripts/check_formal.sh
# Each entry in MODELS is "<dir>|<spec>|<cfg>|<expect>". expect is "pass", or
# the name of the invariant the model must violate (a model of the old, buggy
# code). A buggy model that stops failing means the spec no longer describes
# the bug it was written for.
# Add every new TLA+ model in formal/ to MODELS; the array is not discovered automatically.
# Lean proofs in formal/ are not built in CI.
set -euo pipefail
cd "$(dirname "$0")/.."

TLA_VERSION="1.7.4"
TLA_SHA256="936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88"
# eclipse-temurin:21-jre
JAVA_IMAGE="eclipse-temurin@sha256:227118764cccd3c3294617061aaa93a08309c63a395eefa83e6b35032992ea99"

MODELS=(
  "formal/antiflood|FloodReset.tla|FloodResetFixed.cfg|pass"
  "formal/antiflood|FloodReset.tla|FloodResetBuggy.cfg|ProgressInv"
)

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  else
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
  echo "❌ sha256sum or shasum is required" >&2
  exit 1
fi

cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}/alita"
jar="$cache_dir/tla2tools-$TLA_VERSION.jar"
if [[ ! -f "$jar" ]] || [[ "$(sha256 "$jar")" != "$TLA_SHA256" ]]; then
  mkdir -p "$cache_dir"
  trap 'rm -f "$jar.tmp"' EXIT
  curl -fsSL -o "$jar.tmp" \
    "https://github.com/tlaplus/tlaplus/releases/download/v$TLA_VERSION/tla2tools.jar"
  got="$(sha256 "$jar.tmp")"
  if [[ "$got" != "$TLA_SHA256" ]]; then
    rm -f "$jar.tmp"
    echo "❌ tla2tools.jar checksum mismatch: got $got, want $TLA_SHA256" >&2
    exit 1
  fi
  mv "$jar.tmp" "$jar"
fi

failed=0
for entry in "${MODELS[@]}"; do
  IFS='|' read -r dir spec cfg expect <<<"$entry"
  echo "  TLC $dir/$spec with $cfg (expect: $expect)"
  code=0
  out="$(docker run --rm \
    -v "$jar:/opt/tla2tools.jar:ro" \
    -v "$PWD/$dir:/spec:ro" \
    -w /tmp \
    "$JAVA_IMAGE" \
    java -XX:+UseParallelGC -cp /opt/tla2tools.jar tlc2.TLC \
    -metadir /tmp/tlc -config "/spec/$cfg" "/spec/$spec" 2>&1)" || code=$?

  if [[ "$expect" == "pass" ]]; then
    if [[ $code -ne 0 ]]; then
      echo "$out" >&2
      echo "❌ $cfg: TLC exited $code, want a clean run" >&2
      failed=1
    fi
  # 12 is TLC's exit status for a safety (invariant) violation.
  elif [[ $code -ne 12 ]] || ! grep -qF "Error: Invariant $expect is violated" <<<"$out"; then
    echo "$out" >&2
    echo "❌ $cfg: TLC exited $code without violating $expect" >&2
    failed=1
  fi
done

if [[ $failed -ne 0 ]]; then
  exit 1
fi
echo "✅ All formal models check as expected."
