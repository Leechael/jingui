#!/usr/bin/env bash
set -euo pipefail

# Usage: bump-version.sh <major|minor|patch> [prerelease]
#
# Determines the next version by bumping the latest v* git tag.
#   bump-version.sh patch              → v0.1.0 → v0.1.1
#   bump-version.sh minor              → v0.1.1 → v0.2.0
#   bump-version.sh major              → v0.2.0 → v1.0.0
#   bump-version.sh minor beta         → v0.2.0 → v0.3.0-beta.1
#   bump-version.sh patch rc           → v0.3.0-beta.1 → v0.3.0-rc.1
#
# If the current tag already has the same prerelease label, the counter
# is incremented (e.g. v1.0.0-beta.1 → v1.0.0-beta.2).
# A stable bump from a prerelease strips the suffix (v1.0.0-beta.2 → v1.0.0).

TYPE="${1:?Usage: bump-version.sh <major|minor|patch> [prerelease]}"
PRERELEASE="${2:-}"

case "$TYPE" in
  major|minor|patch) ;;
  *) echo "Error: type must be major, minor, or patch (got '$TYPE')" >&2; exit 1 ;;
esac

if [[ -n "$PRERELEASE" ]]; then
  case "$PRERELEASE" in
    alpha|beta|rc) ;;
    *) echo "Error: prerelease must be alpha, beta, or rc (got '$PRERELEASE')" >&2; exit 1 ;;
  esac
fi

# Fetch the latest v* tag (sorted by semver). Default to v0.0.0 if none exist.
# versionsort.suffix=- ensures prerelease tags (v1.0.0-beta.1) sort before
# their stable counterpart (v1.0.0), matching semver ordering.
LATEST=$(git -c 'versionsort.suffix=-' tag --list 'v*' --sort=-version:refname | head -n1)
LATEST="${LATEST:-v0.0.0}"

# Strip leading 'v'
VERSION="${LATEST#v}"

# Split off prerelease suffix if present (e.g. "1.0.0-beta.1" → "1.0.0" + "beta.1")
CURRENT_PRE=""
if [[ "$VERSION" == *-* ]]; then
  CURRENT_PRE="${VERSION#*-}"
  VERSION="${VERSION%%-*}"
fi

# Parse major.minor.patch
IFS='.' read -r MAJOR MINOR PATCH <<< "$VERSION"
MAJOR="${MAJOR:-0}"
MINOR="${MINOR:-0}"
PATCH="${PATCH:-0}"

# If currently on a prerelease and doing a stable bump (no prerelease requested),
# just strip the suffix — the base version is already the target.
if [[ -n "$CURRENT_PRE" && -z "$PRERELEASE" ]]; then
  echo "v${MAJOR}.${MINOR}.${PATCH}"
  exit 0
fi

# If already on the same prerelease label, just increment the counter.
if [[ -n "$PRERELEASE" && -n "$CURRENT_PRE" ]]; then
  CURRENT_LABEL="${CURRENT_PRE%%.*}"
  CURRENT_COUNT="${CURRENT_PRE#*.}"
  if [[ "$CURRENT_COUNT" == "$CURRENT_PRE" ]]; then
    CURRENT_COUNT=0
  fi
  if [[ "$PRERELEASE" == "$CURRENT_LABEL" ]]; then
    echo "v${MAJOR}.${MINOR}.${PATCH}-${PRERELEASE}.$((CURRENT_COUNT + 1))"
    exit 0
  fi
fi

# Bump the base version
case "$TYPE" in
  major)
    MAJOR=$((MAJOR + 1))
    MINOR=0
    PATCH=0
    ;;
  minor)
    MINOR=$((MINOR + 1))
    PATCH=0
    ;;
  patch)
    PATCH=$((PATCH + 1))
    ;;
esac

# Append prerelease suffix if requested
if [[ -n "$PRERELEASE" ]]; then
  echo "v${MAJOR}.${MINOR}.${PATCH}-${PRERELEASE}.1"
else
  echo "v${MAJOR}.${MINOR}.${PATCH}"
fi
