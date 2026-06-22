#!/usr/bin/env bash
# Patch the bot's internal version to a release tag:
#   scripts/bump_version.sh v<MAJOR>.<MINOR>.<PATCH>[-<prerelease>]
#   - alita/config/config.go  BotVersion: "X.Y.Z"   (canonical, no v)
#   - main.go                 version = "vX.Y.Z"     (CLI fallback)
# Idempotent: a no-op leaves the working tree clean so callers can skip the
# commit. Portable across BSD (macOS) and GNU sed.
set -euo pipefail
cd "$(dirname "$0")/.."

raw="${1:-}"
tag="v${raw#v}"
if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]]; then
  echo "usage: scripts/bump_version.sh v<MAJOR>.<MINOR>.<PATCH>[-<prerelease>]" >&2
  exit 1
fi

ver="${tag#v}"
cfg="alita/config/config.go"
mn="main.go"

sed -i.bak -E "s|BotVersion:[[:space:]]*\"[^\"]*\"|BotVersion:  \"${ver}\"|" "$cfg"
rm -f "${cfg}.bak"
sed -i.bak -E "s|version = \"v[0-9][^\"]*\"|version = \"${tag}\"|" "$mn"
rm -f "${mn}.bak"

grep -qF "BotVersion:  \"${ver}\"" "$cfg" || { echo "error: failed to patch ${cfg}" >&2; exit 1; }
grep -qF "version = \"${tag}\"" "$mn" || { echo "error: failed to patch ${mn}" >&2; exit 1; }

echo "BotVersion set to ${ver} (CLI fallback ${tag})"
