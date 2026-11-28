#!/usr/bin/env bash
# pr-submit.sh [--draft] [--title T] [--body B | --body-file F] [--force]: the one way a hand or subagent branch opens
# (or refreshes) its pull request to main. Run it from the branch's own worktree, never the main checkout. It does
# what agentctl open-pr does before a push, in the same order and with the same scripts:
#   1. fetch REMOTE (PR_SUBMIT_REMOTE, default origin) and merge REMOTE/main into the branch. A conflict only in the
#      generated set (scripts/autoresolve-paths.tsv) takes main's copy and rebuilds it (scripts/pr-regen.sh, with the
#      branch's side as PR_REGEN_HEAD); a conflict anywhere else aborts the merge, prints the files and exits 4.
#   2. scripts/pr-derived.sh BASE: a derived record the branch only raised (support.md, kgen's pins, remspan, the
#      function ceilings) goes back to main's copy, committed on its own, so the PR stops conflicting every time main
#      regenerates them (scripts/regen-main.sh carries the rise).
#   3. scripts/pr-precheck.sh BASE must print "pr-precheck: ok" on the exact head pushed; else its output and exit 1.
#   4. push the branch (no force: --force is a lease on the fetched remote branch) and open the PR to main, not a
#      draft unless --draft, its body ending in the swarm signature. An open PR is only re-synced and pushed.
# Why: 11-26, 9 of 10 red gorge PRs were wt/* branches subagents opened by hand, skipping all three scripts and
# carrying derived files that went DIRTY on every main merge. Last line: "pr-submit: ok #N" or the failure.
# Exit: 0 ok (or nothing to submit); 1 a failed step; 2 usage or the wrong checkout; 4 a conflict outside the set.
set -uo pipefail
SIGNATURE="This Robot (gorge agentctl swarm) Shan't Be Blamed For Stupid Humans"
remote=${PR_SUBMIT_REMOTE:-origin}
draft= title= body= force=
while [ $# -gt 0 ]; do
  case $1 in
  --draft) draft=1 ;;
  --force) force=1 ;;
  --title) title=${2:?--title needs a value}; shift ;;
  --body) body=${2:?--body needs a value}; shift ;;
  --body-file) body=$(cat "${2:?--body-file needs a path}") || exit 2; shift ;;
  *) echo "usage: pr-submit.sh [--draft] [--title T] [--body B | --body-file F] [--force]" >&2; exit 2 ;;
  esac
  shift
done
fail() { echo "pr-submit: $*"; exit "${rc:-1}"; }

top=$(git rev-parse --show-toplevel 2>/dev/null) || { rc=2 fail "not in a git checkout"; }
common=$(git rev-parse --path-format=absolute --git-common-dir) || exit 2
# The main checkout is the tree that owns the common git dir; other sessions share it (AGENTS.md, Working in a worktree).
[ "$top" != "$(dirname "$common")" ] || rc=2 fail "$top is the main checkout: run me from the branch's own worktree"
cd "$top" || exit 2
branch=$(git symbolic-ref -q --short HEAD) || rc=2 fail "detached HEAD: check out the PR branch"
case $branch in main | master | develop) rc=2 fail "on $branch: run me on a PR branch" ;; esac
git diff --quiet HEAD -- && [ -z "$(git ls-files -u)" ] || rc=2 fail "uncommitted tracked changes: commit them first"

git fetch -q "$remote" main || fail "fetch $remote main failed"
base=$(git rev-parse --verify -q "$remote/main^{commit}") || fail "no $remote/main"
lease=()
if git ls-remote --exit-code -q --heads "$remote" "$branch" >/dev/null 2>&1; then
  git fetch -q "$remote" "+refs/heads/$branch:refs/remotes/$remote/$branch" || fail "fetch $remote $branch failed"
  theirs=$(git rev-parse "$remote/$branch")
  if ! git merge-base --is-ancestor "$theirs" HEAD; then
    # The remote holds commits this branch lacks: pr-autoresolve's merges, or the branch was rebased here.
    [ -n "$force" ] || fail "$remote/$branch has commits HEAD lacks: git merge $remote/$branch, or pass --force to replace them"
    lease=(--force-with-lease="refs/heads/$branch:$theirs")
  fi
fi

# 1. Merge main.
pre=$(git rev-parse HEAD)
if ! git merge -q --no-edit -m "Merge $remote/main into $branch" "$base" >/dev/null 2>&1; then
  mapfile -t conflicted < <(git diff --name-only --diff-filter=U)
  [ ${#conflicted[@]} -gt 0 ] || { git merge --abort 2>/dev/null; fail "merging $remote/main failed with no conflicted path"; }
  mapfile -t other < <(bash scripts/pr-autoresolve.sh --match "${conflicted[@]}")
  if [ ${#other[@]} -gt 0 ]; then
    git merge --abort
    echo "pr-submit: merging $remote/main conflicts outside the generated files; resolve these yourself, then rerun:"
    printf '  %s\n' "${other[@]}"
    rc=4 fail "conflict: ${other[*]}"
  fi
  regen=()
  for f in "${conflicted[@]}"; do
    if git cat-file -e ":3:$f" 2>/dev/null; then git checkout -q --theirs -- "$f"; else git rm -q -- "$f"; continue; fi
    case $f in
    dirac/docs/support.md | internal/codeshape/testdata/func_ceilings.tsv | dirac/oracle/testdata/repro_table_names.txt | \
      dirac/kgen/testdata/acceptance.tsv | dirac/kgen/testdata/build_acceptance.tsv | dirac/kgen/testdata/remspan.tsv)
      regen+=("$f") ;;
    *) echo "pr-submit: took $remote/main's $f (pr-precheck judges it; scripts/pr-autoresolve.sh documents its rebuild)" ;;
    esac
  done
  if [ ${#regen[@]} -gt 0 ]; then
    echo "pr-submit: rebuilding ${regen[*]} over the merged tree (scripts/pr-regen.sh)"
    PR_REGEN_HEAD=$pre bash scripts/pr-regen.sh "${regen[@]}" || { git merge --abort; fail "pr-regen.sh failed on ${regen[*]}"; }
  fi
  git add -- "${conflicted[@]}" 2>/dev/null
  [ -z "$(git ls-files -u)" ] || { git merge --abort; fail "paths still unmerged after the rebuild"; }
  git commit -q --no-edit || { git merge --abort; fail "merge commit failed"; }
fi

# 2. Derived records the branch only raised go back to main's copy.
derived=(dirac/docs/support.md dirac/kgen/testdata/acceptance.tsv dirac/kgen/testdata/build_acceptance.tsv
  dirac/kgen/testdata/remspan.tsv internal/codeshape/testdata/func_ceilings.tsv)
bash scripts/pr-derived.sh "$base" || { git checkout -q HEAD -- .; fail "pr-derived.sh failed (no corpus? scripts/link-cards.sh)"; }
if ! git diff --quiet HEAD --; then
  mapfile -t reset < <(git diff --name-only HEAD --)
  stray=$(printf '%s\n' "${reset[@]}" | grep -v -x -F "$(printf '%s\n' "${derived[@]}")")
  [ -z "$stray" ] || { git checkout -q HEAD -- .; fail "pr-derived.sh changed files outside the derived set: $stray"; }
  git add -- "${reset[@]}" && git commit -q -m "Leave derived records at $remote/main's copy (scripts/pr-derived.sh)" ||
    fail "committing pr-derived.sh's reset failed"
fi
git diff --quiet "$base" HEAD -- && { echo "pr-submit: the branch adds nothing to $remote/main; nothing pushed"; exit 0; }

# 3. pr-precheck on the exact head. timeout signals its whole process group.
head=$(git rev-parse HEAD)
log=$(mktemp); trap 'rm -f "$log"' EXIT
precheck() {
  timeout -k 10 "${PR_SUBMIT_PRECHECK_TIMEOUT:-900}" bash scripts/pr-precheck.sh "$base" 2>&1 | tee "$log"
  prc=${PIPESTATUS[0]}
  [ "$prc" -eq 0 ] && [ "$(tail -n 1 "$log")" = "pr-precheck: ok" ] ||
    fail "pr-precheck failed on $head (exit $prc): $(tail -n 1 "$log"); nothing pushed"
}
# refusals_only FROM TO: TO descends from FROM by refuse-on-red.sh's apply-mode commits alone (its subject, only dirac/
# paths). That is the one precheck step that moves HEAD (REFUSE_ON_RED=apply); a merge or any other commit is not one.
refusals_only() {
  git merge-base --is-ancestor "$1" "$2" || return 1
  [ -z "$(git log --format=%s "$1..$2" | grep -v -E '^dirac: refuse [0-9]+ cards the rules fix diverges on$')" ] || return 1
  [ -z "$(git diff --name-only "$1" "$2" -- | grep -v '^dirac/')" ]
}
precheck
# A refusal commit is checked by one more pr-precheck run on it (its dirac/ diff runs the support ledger, and
# refuse-on-red is n/a on it), then pushed: the first pr-submit after an apply-mode refusal succeeds.
now=$(git rev-parse HEAD)
if [ "$now" != "$head" ] && git diff --quiet HEAD -- && refusals_only "$head" "$now"; then
  echo "pr-submit: pr-precheck committed refusals ($(git rev-parse --short=10 "$now")); checking that head too"
  head=$now
  precheck
fi
[ "$(git rev-parse HEAD)" = "$head" ] && git diff --quiet HEAD -- || fail "the tree moved during pr-precheck; nothing pushed"

# 4. Push the checked head, then open the PR or reuse the open one.
git push -q "${lease[@]}" "$remote" "$head:refs/heads/$branch" || fail "push to $remote $branch failed"
num=$(gh pr list --head "$branch" --base main --state open --json number --jq '.[0].number // empty') || fail "gh pr list failed"
if [ -z "$num" ]; then
  [ -n "$title" ] || title=$(git log --reverse --no-merges --format=%s "$base..$head" | head -n 1)
  [ -n "$body" ] || body="Commits:"$'\n'"$(git log --reverse --no-merges --format='- %s' "$base..$head")"
  case $body in *"$SIGNATURE"*) ;; *) body+=$'\n\n---\n'"$SIGNATURE"$'\n' ;; esac
  url=$(gh pr create --base main --head "$branch" --title "$title" --body "$body" ${draft:+--draft}) || fail "gh pr create failed"
  num=${url##*/}
  echo "pr-submit: opened $url"
else
  echo "pr-submit: pushed $(git rev-parse --short=10 "$head") to the open #$num"
fi
echo "pr-submit: ok #$num"
