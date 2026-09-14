#!/usr/bin/env bash
# Suggest the next tag from the conventional commits since the last one.
#
#   next-version.sh [major|minor|patch]
#
# With no argument the bump is derived: a `!`/BREAKING CHANGE commit bumps major,
# any `feat` bumps minor, anything else patch. It only prints a suggestion — the
# release skill still asks before tagging.
set -euo pipefail

LAST=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
BUMP="${1:-}"

if [ -z "$BUMP" ]; then
  BUMP="patch"
  while IFS= read -r -d $'\x1e' record; do
    subject="${record%%$'\x1f'*}"
    body="${record#*$'\x1f'}"
    if [[ "$subject" =~ ^[a-zA-Z]+(\([^\)]*\))?!: ]] || [[ "$body" == *"BREAKING CHANGE"* ]]; then
      BUMP="major"; break
    fi
    [[ "$subject" =~ ^feat(\([^\)]*\))?: ]] && BUMP="minor"
  done < <(git log "${LAST}..HEAD" --no-merges --pretty=format:'%s%x1f%b%x1e')
fi

IFS='.' read -r major minor patch <<<"${LAST#v}"
case "$BUMP" in
  major) major=$((major + 1)); minor=0; patch=0 ;;
  minor) minor=$((minor + 1)); patch=0 ;;
  patch) patch=$((patch + 1)) ;;
  *) echo "unknown bump: $BUMP" >&2; exit 1 ;;
esac

echo "v${major}.${minor}.${patch}"
