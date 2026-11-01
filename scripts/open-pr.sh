#!/usr/bin/env bash
# open-pr.sh — open (or refresh) a GitHub pull request for the current branch,
# with a generated overview of what changed. `make pr` is the entry point.
#
#   make pr                      # current branch -> origin's default branch
#   make pr BASE=main HEAD=develop
#   make pr TITLE="..." DRAFT=1
#   make pr DRY=1                # print the title and body, change nothing
#
# The branch is pushed to origin (never the base branch itself), then a PR is
# created; if one is already open for the branch its body is regenerated.
# Needs git and an authenticated `gh`.
set -euo pipefail

die() { echo "open-pr: $*" >&2; exit 1; }

command -v gh >/dev/null || die "gh (GitHub CLI) is not installed"
git rev-parse --git-dir >/dev/null 2>&1 || die "not inside a git repository"

REMOTE=${REMOTE:-origin}
HEAD_BRANCH=${HEAD:-$(git symbolic-ref -q --short HEAD || true)}
[ -n "$HEAD_BRANCH" ] || die "detached HEAD: pass HEAD=<branch>"
if [ -z "${BASE:-}" ]; then
  BASE=$(git symbolic-ref -q --short "refs/remotes/$REMOTE/HEAD" 2>/dev/null | sed "s|^$REMOTE/||" || true)
  BASE=${BASE:-main}
fi
[ "$HEAD_BRANCH" != "$BASE" ] || die "HEAD and BASE are both '$BASE': check out a feature branch, or pass HEAD=/BASE="

git fetch -q "$REMOTE" "$BASE" || die "cannot fetch $REMOTE/$BASE"
BASE_REF="$REMOTE/$BASE"
git rev-parse -q --verify "$HEAD_BRANCH^{commit}" >/dev/null || die "no such branch: $HEAD_BRANCH"
MB=$(git merge-base "$BASE_REF" "$HEAD_BRANCH") || die "$HEAD_BRANCH shares no history with $BASE_REF"
RANGE="$MB..$HEAD_BRANCH"

NCOMMITS=$(git rev-list --count --no-merges "$RANGE")
[ "$(git rev-list --count "$RANGE")" -gt 0 ] || die "$HEAD_BRANCH has nothing that is not already on $BASE_REF"

REPO_NAME=$(git remote get-url "$REMOTE" 2>/dev/null | sed -E 's#.*[:/]([^/]+/[^/]+)$#\1#; s#\.git$##')
read -r NFILES ADDS DELS < <(git diff --numstat "$MB" "$HEAD_BRANCH" |
  awk '{f++; if ($1 != "-") a += $1; if ($2 != "-") d += $2} END {printf "%d %d %d\n", f, a, d}')

# ---- title -----------------------------------------------------------------
if [ -z "${TITLE:-}" ]; then
  if [ "$NCOMMITS" -eq 1 ]; then
    TITLE=$(git log -1 --no-merges --format=%s "$RANGE")
  else
    TITLE="$HEAD_BRANCH → $BASE: $NCOMMITS commits"
  fi
fi

# ---- body ------------------------------------------------------------------
body() {
  local first last
  first=$(git log --reverse --no-merges --format=%cs "$RANGE" | sed -n 1p)
  last=$(git log -1 --no-merges --format=%cs "$RANGE")

  echo "## Overview"
  echo
  echo "| | |"
  echo "|---|---|"
  echo "| **Repository** | \`$REPO_NAME\` |"
  echo "| **Branch** | \`$HEAD_BRANCH\` → \`$BASE\` |"
  echo "| **Commits** | $NCOMMITS (${first:-?} … ${last:-?}) |"
  echo "| **Files changed** | $NFILES |"
  echo "| **Lines** | +$ADDS / −$DELS |"
  echo

  if [ -n "${SUMMARY:-}" ]; then
    echo "$SUMMARY"
    echo
  fi

  # Commit types (conventional-commit prefixes), as a one-line digest.
  local types
  types=$(git log --no-merges --format=%s "$RANGE" |
    sed -nE 's/^([a-z]+)(\([^)]*\))?!?:.*/\1/p' | sort | uniq -c | sort -rn |
    awk '{printf "%s%s ×%d", (NR > 1 ? " · " : ""), $2, $1}')
  if [ -n "$types" ]; then
    echo "**Kinds of change:** $types"
    echo
  fi

  echo "## Changes by area"
  echo
  echo "| Area | Files | + | − |"
  echo "|---|--:|--:|--:|"
  git diff --numstat "$MB" "$HEAD_BRANCH" | awk -F'\t' '
    {
      n = split($3, p, "/")
      area = (n >= 3) ? p[1] "/" p[2] "/" : (n == 2 ? p[1] "/" : "(root)")
      files[area]++
      if ($1 != "-") add[area] += $1
      if ($2 != "-") del[area] += $2
    }
    END { for (a in files) printf "%d\t%s\t%d\t%d\n", add[a] + del[a], a, files[a], add[a], del[a] }' |
    sort -t$'\t' -k1,1nr | sed -n "1,25p" |
    awk -F'\t' '{ printf "| `%s` | %d | %d | %d |\n", $2, $3, $4, $5 }'
  echo

  echo "## Commits"
  echo
  git log --reverse --no-merges --format='- %s (`%h`)' "$RANGE" | sed -n "1,200p"
  if [ "$NCOMMITS" -gt 200 ]; then echo "- … and $((NCOMMITS - 200)) more"; fi
  echo

  echo "<details><summary>All changed files ($NFILES)</summary>"
  echo
  echo '| Status | File |'
  echo '|---|---|'
  git diff --name-status -M "$MB" "$HEAD_BRANCH" | sed -n "1,500p" | awk -F'\t' '
    { s = substr($1, 1, 1); name = (s == "R" || s == "C") ? $2 " → " $3 : $2
      label = (s == "A") ? "added" : (s == "D") ? "deleted" : (s == "R") ? "renamed" : (s == "C") ? "copied" : "modified"
      printf "| %s | `%s` |\n", label, name }'
  echo
  echo "</details>"
  echo

  echo "## Testing"
  echo
  echo "- [ ] CI is green on this branch"
  echo "- [ ] Tested locally (say how):"
  echo
  echo "## Notes for reviewers"
  echo
  echo "_Risks, follow-ups, anything a reviewer should look at first._"
}

BODY_FILE=$(mktemp)
trap 'rm -f "$BODY_FILE"' EXIT
body > "$BODY_FILE"

if [ -n "${DRY:-}" ]; then
  echo "TITLE: $TITLE"
  echo
  cat "$BODY_FILE"
  exit 0
fi

# ---- push the branch (never the base) and open / refresh the PR -------------
case "$HEAD_BRANCH" in main|master) die "refusing to push '$HEAD_BRANCH'";; esac
git push -q -u "$REMOTE" "$HEAD_BRANCH"

existing=$(gh pr list --head "$HEAD_BRANCH" --base "$BASE" --state open --json url --jq '.[0].url' 2>/dev/null || true)
if [ -n "$existing" ]; then
  gh pr edit "$existing" --body-file "$BODY_FILE" >/dev/null
  echo "updated $existing"
else
  args=(--base "$BASE" --head "$HEAD_BRANCH" --title "$TITLE" --body-file "$BODY_FILE")
  [ -n "${DRAFT:-}" ] && args+=(--draft)
  gh pr create "${args[@]}"
fi
