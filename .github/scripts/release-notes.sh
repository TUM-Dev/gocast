#!/usr/bin/env bash
# Render release notes for a tag from the conventional-commit history since the
# previous tag. Used by .github/workflows/release.yml and by the `release` skill
# to preview notes before a tag exists.
#
#   release-notes.sh <tag> [previous-tag]
#
# Author handles come from the GitHub compare API when `gh` is authenticated;
# without it the notes fall back to git author names, so the script stays usable
# offline.
set -euo pipefail

TAG="${1:?usage: release-notes.sh <tag> [previous-tag]}"
PREV="${2:-}"

REPO="${GITHUB_REPOSITORY:-}"
if [ -z "$REPO" ]; then
  REPO=$(git config --get remote.origin.url | sed -E 's#(git@|https://)github\.com[:/]##; s#\.git$##')
fi

# Ancestry-based, so a patch tag cut from an older branch compares against its
# own parent rather than whatever sorts highest.
if [ -z "$PREV" ]; then
  PREV=$(git describe --tags --abbrev=0 "${TAG}^" 2>/dev/null || true)
fi

if [ -n "$PREV" ]; then RANGE="${PREV}..${TAG}"; else RANGE="$TAG"; fi

# sha -> github login, one API call. Optional: everything below degrades to the
# git author name when the map is empty.
declare -A LOGIN=()
if [ -n "$PREV" ] && command -v gh >/dev/null 2>&1; then
  while read -r sha login; do
    [ -n "${login:-}" ] && LOGIN["$sha"]="$login"
  done < <(gh api "repos/${REPO}/compare/${PREV}...${TAG}" --paginate \
             --jq '.commits[] | "\(.sha) \(.author.login // "")"' 2>/dev/null || true)
fi

# Authors seen before this range, to spot first-time contributors.
declare -A SEEN=()
if [ -n "$PREV" ]; then
  while IFS= read -r name; do SEEN["$name"]=1; done < <(git log "$PREV" --pretty=format:'%aN')
fi

BREAKING=(); FEAT=(); FIX=(); PERF=(); REFACTOR=(); DOCS=(); TEST=(); CHORE=(); DEPS=(); OTHER=()
declare -A NEW_CONTRIBUTORS=()

# %x1f between fields, %x1e between records: commit bodies are multi-line.
while IFS= read -r -d $'\x1e' record; do
  record="${record#$'\n'}"
  IFS=$'\x1f' read -r sha subject author body <<<"$record"
  [ -n "${sha:-}" ] || continue

  handle="${LOGIN[$sha]:-}"
  if [ -n "$handle" ]; then who="@${handle}"; else who="$author"; fi

  if [ -n "$PREV" ] && [ -z "${SEEN[$author]:-}" ]; then
    NEW_CONTRIBUTORS["$who"]="$sha"
  fi

  # `feat(scope)!: subject` — type, scope and the breaking marker are all optional.
  type=""; scope=""; bang=""; title="$subject"
  if [[ "$subject" =~ ^([a-zA-Z]+)(\(([^\)]*)\))?(!)?:[[:space:]]*(.*)$ ]]; then
    type="${BASH_REMATCH[1],,}"; scope="${BASH_REMATCH[3]}"; bang="${BASH_REMATCH[4]}"; title="${BASH_REMATCH[5]}"
  fi

  # Trailing (#123) becomes the link target; PRs read better than raw shas.
  link="https://github.com/${REPO}/commit/${sha}"
  ref="\`${sha:0:7}\`"
  if [[ "$title" =~ ^(.*)[[:space:]]*\(#([0-9]+)\)[[:space:]]*$ ]]; then
    title="${BASH_REMATCH[1]%"${BASH_REMATCH[1]##*[![:space:]]}"}"
    link="https://github.com/${REPO}/pull/${BASH_REMATCH[2]}"
    ref="#${BASH_REMATCH[2]}"
  fi

  prefix=""
  [ -n "$scope" ] && prefix="**${scope}:** "
  entry="- ${prefix}${title} ([${ref}](${link})) by ${who}"

  if [ -n "$bang" ] || [[ "$body" == *"BREAKING CHANGE"* ]]; then
    BREAKING+=("$entry")
    continue
  fi

  case "$type" in
    feat)              FEAT+=("$entry") ;;
    fix)               if [ "$scope" = "deps" ]; then DEPS+=("$entry"); else FIX+=("$entry"); fi ;;
    perf)              PERF+=("$entry") ;;
    refactor|style)    REFACTOR+=("$entry") ;;
    docs)              DOCS+=("$entry") ;;
    test)              TEST+=("$entry") ;;
    build|chore|ci)    if [ "$scope" = "deps" ] || [ "$scope" = "deps-dev" ]; then DEPS+=("$entry"); else CHORE+=("$entry"); fi ;;
    *)                 OTHER+=("$entry") ;;
  esac
done < <(git log "$RANGE" --no-merges --reverse --pretty=format:'%H%x1f%s%x1f%aN%x1f%b%x1e')

section() {
  local heading="$1"; shift
  # An empty array expands to one empty argument on older bash; drop those.
  local entries=() e
  for e in "$@"; do [ -n "$e" ] && entries+=("$e"); done
  [ "${#entries[@]}" -gt 0 ] || return 0
  printf '## %s\n\n' "$heading"
  printf '%s\n' "${entries[@]}"
  printf '\n'
}

section "⚠️ Breaking Changes" "${BREAKING[@]-}"
section "🚀 Features"         "${FEAT[@]-}"
section "🐛 Fixes"            "${FIX[@]-}"
section "⚡ Performance"      "${PERF[@]-}"
section "♻️ Refactoring"      "${REFACTOR[@]-}"
section "📚 Documentation"    "${DOCS[@]-}"
section "✅ Tests"            "${TEST[@]-}"
section "🧰 Maintenance"      "${CHORE[@]-}"
section "📝 Other Changes"    "${OTHER[@]-}"

# Renovate and Dependabot out-number everything else; folded so the human-written
# entries stay readable.
if [ "${#DEPS[@]}" -gt 0 ]; then
  printf '<details>\n<summary>📦 Dependencies (%d)</summary>\n\n' "${#DEPS[@]}"
  printf '%s\n' "${DEPS[@]}"
  printf '\n</details>\n\n'
fi

if [ "${#NEW_CONTRIBUTORS[@]}" -gt 0 ]; then
  printf '## 🎉 New Contributors\n\n'
  for who in "${!NEW_CONTRIBUTORS[@]}"; do
    printf -- '- %s made their first contribution in https://github.com/%s/commit/%s\n' \
      "$who" "$REPO" "${NEW_CONTRIBUTORS[$who]}"
  done
  printf '\n'
fi

if [ -n "$PREV" ]; then
  printf '**Full Changelog**: https://github.com/%s/compare/%s...%s\n' "$REPO" "$PREV" "$TAG"
else
  printf '**Full Changelog**: https://github.com/%s/commits/%s\n' "$REPO" "$TAG"
fi
