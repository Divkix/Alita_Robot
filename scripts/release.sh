#!/usr/bin/env bash
#
# Release helper script for Alita Robot.
#
# Bumps semantic version ("patch" / "path", "minor", "major"),
# creates an annotated git tag, and pushes it to GitHub to trigger
# the automated release workflow.
#
# Usage:
#   scripts/release.sh [OPTIONS] <patch|path|minor|major>
#

set -euo pipefail

# Ensure we run from the repository root
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || {
  echo "Error: Not inside a git repository." >&2
  exit 1
}
cd "$REPO_ROOT"

print_usage() {
  cat <<'EOF'
Usage: scripts/release.sh [OPTIONS] <patch|path|minor|major>

Bumps the semantic version, creates an annotated git tag, and pushes to GitHub.

Inputs:
  patch (or path)   Increment patch version (e.g. v3.7.0 -> v3.7.1)
  minor             Increment minor version (e.g. v3.7.0 -> v3.8.0)
  major             Increment major version (e.g. v3.7.0 -> v4.0.0)

Options:
  -d, --dry-run         Show what would be done without tagging or pushing
  -r, --remote <name>   Git remote to push to (default: origin)
  -b, --push-branch     Also push the current branch to the remote
  -m, --message <msg>   Custom tag annotation message (default: <tag>)
  -y, --yes             Skip confirmation prompts
  -h, --help            Show this help message
EOF
}

REMOTE="origin"
DRY_RUN=false
ASSUME_YES=false
PUSH_BRANCH=false
CUSTOM_MESSAGE=""
INPUT_TYPE=""

# Parse command line options and arguments
while [ $# -gt 0 ]; do
  case "$1" in
    -h|--help)
      print_usage
      exit 0
      ;;
    -d|--dry-run)
      DRY_RUN=true
      shift
      ;;
    -y|--yes)
      ASSUME_YES=true
      shift
      ;;
    -b|--push-branch)
      PUSH_BRANCH=true
      shift
      ;;
    -r|--remote)
      if [ -z "${2:-}" ]; then
        echo "Error: --remote requires a remote name." >&2
        exit 1
      fi
      REMOTE="$2"
      shift 2
      ;;
    -m|--message)
      if [ -z "${2:-}" ]; then
        echo "Error: --message requires a message string." >&2
        exit 1
      fi
      CUSTOM_MESSAGE="$2"
      shift 2
      ;;
    -*)
      echo "Error: Unknown option '$1'." >&2
      print_usage >&2
      exit 1
      ;;
    *)
      if [ -z "$INPUT_TYPE" ]; then
        INPUT_TYPE="$1"
      else
        echo "Error: Unexpected argument '$1'." >&2
        print_usage >&2
        exit 1
      fi
      shift
      ;;
  esac
done

# If no input provided, prompt if interactive, otherwise error
if [ -z "$INPUT_TYPE" ]; then
  if [ -t 0 ]; then
    echo "No release type specified."
    echo "Select release type:"
    select choice in "patch" "minor" "major" "quit"; do
      case "$choice" in
        patch|minor|major)
          INPUT_TYPE="$choice"
          break
          ;;
        quit)
          echo "Aborted."
          exit 0
          ;;
        *)
          echo "Invalid selection. Please choose 1, 2, 3, or 4."
          ;;
      esac
    done
  else
    echo "Error: Release type required ('patch', 'path', 'minor', or 'major')." >&2
    print_usage >&2
    exit 1
  fi
fi

# Normalize input (case-insensitive)
INPUT_LOWER="$(echo "$INPUT_TYPE" | tr '[:upper:]' '[:lower:]')"
case "$INPUT_LOWER" in
  path|patch)
    BUMP_TYPE="patch"
    ;;
  minor)
    BUMP_TYPE="minor"
    ;;
  major)
    BUMP_TYPE="major"
    ;;
  *)
    echo "Error: Invalid release type '$INPUT_TYPE'. Allowed values: patch (or path), minor, major." >&2
    exit 1
    ;;
esac

# Pre-flight check: working tree must be clean (ignoring untracked files)
if [ -n "$(git status --porcelain -uno)" ]; then
  if [ "$DRY_RUN" = true ]; then
    echo "Warning: Working directory has uncommitted or staged changes (ignored in dry-run)." >&2
  else
    echo "Error: Working directory has uncommitted or staged changes." >&2
    echo "Please commit or stash your changes before releasing." >&2
    exit 1
  fi
fi

# Pre-flight check: remote must exist
if ! git remote get-url "$REMOTE" >/dev/null 2>&1; then
  echo "Error: Git remote '$REMOTE' does not exist." >&2
  exit 1
fi

# Fetch tags from remote if possible
if [ "$DRY_RUN" = false ]; then
  echo "==> Fetching tags from $REMOTE..."
  git fetch --tags "$REMOTE" >/dev/null 2>&1 || echo "Warning: Could not fetch tags from $REMOTE. Continuing with local tags." >&2
fi

# Find latest semver tag
# Matches vX.Y.Z or X.Y.Z
LATEST_TAG="$(git tag -l --sort=-v:refname | grep -E '^v?[0-9]+\.[0-9]+\.[0-9]+$' | head -n 1 || true)"

if [ -z "$LATEST_TAG" ]; then
  echo "No existing semver tag found. Defaulting base to v0.0.0."
  LATEST_TAG="v0.0.0"
fi

# Strip optional leading 'v'
VERSION_NUM="${LATEST_TAG#v}"
IFS='.' read -r MAJOR MINOR PATCH <<< "$VERSION_NUM"

# Ensure all segments are valid integers
if ! [[ "$MAJOR" =~ ^[0-9]+$ ]] || ! [[ "$MINOR" =~ ^[0-9]+$ ]] || ! [[ "$PATCH" =~ ^[0-9]+$ ]]; then
  echo "Error: Could not parse version components from latest tag '$LATEST_TAG'." >&2
  exit 1
fi

# Calculate new version
case "$BUMP_TYPE" in
  patch)
    NEW_MAJOR="$MAJOR"
    NEW_MINOR="$MINOR"
    NEW_PATCH="$((PATCH + 1))"
    ;;
  minor)
    NEW_MAJOR="$MAJOR"
    NEW_MINOR="$((MINOR + 1))"
    NEW_PATCH="0"
    ;;
  major)
    NEW_MAJOR="$((MAJOR + 1))"
    NEW_MINOR="0"
    NEW_PATCH="0"
    ;;
esac

NEW_TAG="v${NEW_MAJOR}.${NEW_MINOR}.${NEW_PATCH}"
TAG_MSG="${CUSTOM_MESSAGE:-$NEW_TAG}"

# Check if tag already exists locally
if git rev-parse "$NEW_TAG" >/dev/null 2>&1; then
  echo "Error: Tag '$NEW_TAG' already exists locally." >&2
  exit 1
fi

CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "HEAD")"
COMMIT_SHA="$(git rev-parse --short HEAD)"
COMMIT_SUBJ="$(git log -1 --pretty=format:'%s')"

echo "=========================================="
echo "  Alita Robot Release: $BUMP_TYPE"
echo "=========================================="
echo "  Current version: $LATEST_TAG"
echo "  New version:     $NEW_TAG"
echo "  Target commit:   $COMMIT_SHA ($COMMIT_SUBJ)"
echo "  Target remote:   $REMOTE"
if [ "$PUSH_BRANCH" = true ]; then
  echo "  Push branch:     $CURRENT_BRANCH"
fi
echo "=========================================="

if [ "$DRY_RUN" = true ]; then
  echo "[DRY-RUN] Would create tag: git tag -a $NEW_TAG -m \"$TAG_MSG\""
  echo "[DRY-RUN] Would push tag:   git push $REMOTE $NEW_TAG"
  if [ "$PUSH_BRANCH" = true ]; then
    echo "[DRY-RUN] Would push branch: git push $REMOTE $CURRENT_BRANCH"
  fi
  echo "[DRY-RUN] Dry run complete. No changes made."
  exit 0
fi

# Confirm with user if interactive and --yes was not passed
if [ "$ASSUME_YES" = false ] && [ -t 0 ]; then
  read -r -p "Are you sure you want to create and push $NEW_TAG? [y/N] " CONFIRM
  case "$CONFIRM" in
    [yY][eE][sS]|[yY])
      ;;
    *)
      echo "Release aborted."
      exit 0
      ;;
  esac
fi

# Create annotated tag
echo "==> Creating tag $NEW_TAG..."
git tag -a "$NEW_TAG" -m "$TAG_MSG"

# Push tag to remote
echo "==> Pushing tag $NEW_TAG to $REMOTE..."
if ! git push "$REMOTE" "$NEW_TAG"; then
  echo "Error: Failed to push tag '$NEW_TAG' to '$REMOTE'." >&2
  echo "You can remove the local tag using: git tag -d $NEW_TAG" >&2
  exit 1
fi

# Optionally push current branch
if [ "$PUSH_BRANCH" = true ]; then
  echo "==> Pushing branch $CURRENT_BRANCH to $REMOTE..."
  git push "$REMOTE" "$CURRENT_BRANCH" || echo "Warning: Failed to push branch '$CURRENT_BRANCH'." >&2
fi

echo ""
echo "✅ Successfully released $NEW_TAG!"
echo "GitHub Actions will build and publish the release image."
