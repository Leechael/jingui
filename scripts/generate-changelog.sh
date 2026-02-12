#!/usr/bin/env bash
set -euo pipefail

# Usage: generate-changelog.sh <version>
#
# Generates a changelog entry from git log between the previous tag and HEAD,
# then prepends it to CHANGELOG.md.

VERSION="${1:?Usage: generate-changelog.sh <version>}"
FILE="CHANGELOG.md"

# Find previous tag (exclude the current version if it already exists)
PREV_TAG=$(git tag --list 'v*' --sort=-version:refname | grep -v "^${VERSION}$" | head -n1)

if [[ -n "$PREV_TAG" ]]; then
  RANGE="${PREV_TAG}..HEAD"
else
  RANGE="HEAD"
fi

DATE=$(date -u +%Y-%m-%d)
ENTRIES=$(git log "$RANGE" --pretty=format:"- %s" --no-merges)

SECTION="## ${VERSION} (${DATE})

${ENTRIES}"

if [[ -f "$FILE" ]]; then
  EXISTING=$(cat "$FILE")
  printf '%s\n\n%s\n' "$SECTION" "$EXISTING" > "$FILE"
else
  printf '# Changelog\n\n%s\n' "$SECTION" > "$FILE"
fi
