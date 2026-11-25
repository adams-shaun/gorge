#!/usr/bin/env bash
# pr-autoresolve.sh [--dry-run] [--keep] PR: merge REMOTE/main (AUTORESOLVE_REMOTE, default origin) into pull request
# PR and push the merge to its branch with a plain push, when every conflicted path is a generated file listed in
# scripts/autoresolve-paths.tsv. Those take main's side and are rebuilt over the merged tree with the make targets.
# A conflict in any other path prints the list, exits 4 and pushes nothing.
# pr-autoresolve.sh --match PATH...: print each PATH no row of autoresolve-paths.tsv matches; exit 1 if any.
#
# Why: main regenerates support.md, kgen's acceptance pins, the kop ordinals and the ratchet files many times a day,
# and almost every open dirac PR then conflicts on them alone (.github/workflows/pr-autoresolve.yml runs this for each
# CONFLICTING PR on every push to main). Recipes, each checked by its own test after the rebuild:
#   support        DIRAC_UPDATE_SUPPORT=1 TestSupportDocIsCurrent ./dirac/kgen (also after kgen-pins: it reads them)
#   kgen-pins      main's rows plus the PR's added ones (--union), then KGEN_REPIN=1 then DIRAC_PIN_STRICT=1 over the pin tests ./dirac/kgen (a fall is refused)
#   kop-ordinals   KOP_ORDINALS_REGEN=1 TestKopOrdinalsUnchanged ./dirac/kop: rows re-derived from the merged constants
#   dirac-layout   DIRAC_UPDATE_LAYOUT=1 over the layout pin tests ./dirac
#   func-ceilings  a keyed 3-way row merge (main's rows, plus each row the PR added, changed or deleted); never lowered
#                  here (cmd/checkratchet refuses a PR that lowers one), then the shrink-only ratchets must pass
#   repro-names    append-only: main's rows, then the PR's new ones; then TestReproGlobMatchesTable
# Exit: 0 pushed (or would push, --dry-run) or nothing to do; 1 a rebuild or push failed; 2 usage; 3 no corpus;
# 4 a conflict outside the generated set (left for a human or agent).
set -uo pipefail
here=$(cd "$(dirname "$0")" && pwd)
paths_file=$here/autoresolve-paths.tsv

# recipe_for PATH: the recipe of the first row whose glob matches PATH, or nothing.
recipe_for() {
  local glob recipe
  while IFS=$'\t' read -r glob recipe; do
    case $glob in '' | '#'*) continue ;; esac
    # A glob's * stays within one directory (case's * would cross a /).
    [ "${1//[^\/]/}" = "${glob//[^\/]/}" ] || continue
    # shellcheck disable=SC2254 # the row is a glob on purpose
    case $1 in $glob) echo "$recipe"; return 0 ;; esac
  done <"$paths_file"
  return 1
}

if [ "${1:-}" = --match ]; then
  shift; bad=0
  for p; do recipe_for "$p" >/dev/null || { echo "$p"; bad=1; }; done
  exit $bad
fi

# --union MAIN PR [BASE]: MAIN's file, then each data row of PR whose key (first column) MAIN lacks and BASE (the merge
# base, optional) did not have: a row the PR added, e.g. the acceptance row of a new set list. Main's deletions stay.
if [ "${1:-}" = --union ]; then
  [ $# -ge 3 ] || { echo "usage: pr-autoresolve.sh --union MAIN PR [BASE]" >&2; exit 2; }
  cat "$2"
  awk -F'\t' 'FILENAME == ARGV[3] { if (!/^#/ && NF && !($1 in seen)) print; next }
    !/^#/ && NF { seen[$1] = 1 }' "$2" "${4:-/dev/null}" "$3"
  exit 0
fi

dry= keep=
while [ $# -gt 0 ]; do
  case $1 in --dry-run) dry=1 ;; --keep) keep=1 ;; -*) echo "usage: pr-autoresolve.sh [--dry-run] [--keep] PR" >&2; exit 2 ;; *) break ;; esac
  shift
done
pr=${1:-}
[[ $pr =~ ^[0-9]+$ ]] || { echo "usage: pr-autoresolve.sh [--dry-run] [--keep] PR | --match PATH..." >&2; exit 2; }
remote=${AUTORESOLVE_REMOTE:-origin}
say() { echo "pr-autoresolve #$pr: $*"; }

# PR facts: gh when it is on PATH and GH_TOKEN is unset (local use), else the REST API (the box runners have no gh).
if command -v gh >/dev/null 2>&1 && [ -z "${GH_TOKEN:-}" ]; then
  info=$(gh pr view "$pr" --json headRefName,baseRefName,isCrossRepository,state \
    --jq '"\(.headRefName) \(.baseRefName) \(.isCrossRepository) \(.state)"') || { say "gh pr view failed"; exit 1; }
else
  : "${GH_TOKEN:?GH_TOKEN is required without gh}" "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required without gh}"
  info=$(curl -fsS -H "Authorization: Bearer $GH_TOKEN" -H "Accept: application/vnd.github+json" \
    "https://api.github.com/repos/$GITHUB_REPOSITORY/pulls/$pr" | python3 -c '
import json, sys
d = json.load(sys.stdin)
head = d["head"]
cross = "true" if head.get("repo") is None or head["repo"]["full_name"] != d["base"]["repo"]["full_name"] else "false"
print(head["ref"], d["base"]["ref"], cross, d["state"].upper())') || { say "PR lookup failed"; exit 1; }
fi
read -r branch base cross state <<<"$info"
[ "$state" = OPEN ] || { say "is $state; left alone"; exit 0; }
[ "$base" = main ] || { say "targets $base, not main; left alone"; exit 0; }
[ "$cross" = false ] || { say "comes from a fork; it cannot be pushed to"; exit 0; }

repo=$(git rev-parse --show-toplevel) || exit 1
main=$(cd "$(git rev-parse --git-common-dir)/.." && pwd)
git -C "$repo" fetch -q "$remote" main "+refs/heads/$branch:refs/remotes/$remote/$branch" || { say "fetch failed"; exit 1; }
wt=$main/.worktrees/pr-autoresolve-$pr
git -C "$repo" worktree remove --force "$wt" 2>/dev/null; rm -rf "$wt"
git -C "$repo" worktree add -q --detach "$wt" "$remote/$branch" || { say "worktree add failed"; exit 1; }
[ -n "$keep" ] || trap 'git -C "$repo" worktree remove --force "$wt" 2>/dev/null' EXIT
cd "$wt" || exit 1
head=$(git rev-parse HEAD)

# The committer: the caller's git identity, else the Actions bot.
ident=()
git config user.email >/dev/null || ident+=(-c "user.email=41898282+github-actions[bot]@users.noreply.github.com")
git config user.name >/dev/null || ident+=(-c "user.name=github-actions[bot]")

if git "${ident[@]}" merge -q --no-edit "$remote/main" >/dev/null 2>&1; then
  say "merges $remote/main cleanly; nothing to resolve"; exit 0
fi
mapfile -t conflicted < <(git diff --name-only --diff-filter=U)
[ ${#conflicted[@]} -gt 0 ] || { say "merge failed with no conflicted path"; git merge --abort; exit 1; }
mapfile -t other < <(bash "$here/pr-autoresolve.sh" --match "${conflicted[@]}")
if [ ${#other[@]} -gt 0 ]; then
  say "conflicts outside the generated set; not pushed:"
  printf '  %s\n' "${other[@]}"
  git merge --abort; exit 4
fi
say "resolving ${conflicted[*]}"

declare -A recipes=()
for f in "${conflicted[@]}"; do
  r=$(recipe_for "$f"); recipes[$r]=1
  case $r in
  repro-names)
    { git show ":3:$f" 2>/dev/null; git show ":2:$f" 2>/dev/null; } | awk 'NF && !seen[$0]++' >"$f" ;;
  func-ceilings)
    # Main's rows (:3), with each row the PR (:2) added or changed against the merge base (:1) and without each it
    # deleted; main's header; rows sorted by key (bytewise, as sort.Strings).
    t=$(mktemp -d)
    git show ":1:$f" >"$t/b" 2>/dev/null; git show ":2:$f" >"$t/o" 2>/dev/null; git show ":3:$f" >"$t/m" 2>/dev/null
    awk -F'\t' -v hdr="$t/h" -v rows="$t/r" '
      /^#/ || !NF { if (FILENAME == ARGV[3]) print > hdr; next }
      FILENAME == ARGV[1] { b[$1] = $0; next }
      FILENAME == ARGV[2] { o[$1] = $0; next }
      { r[$1] = $0 }
      END {
        for (k in o) if (!(k in b) || b[k] != o[k]) r[k] = o[k]
        for (k in b) if (!(k in o)) delete r[k]
        for (k in r) print r[k] > rows
      }' "$t/b" "$t/o" "$t/m"
    { cat "$t/h"; LC_ALL=C sort "$t/r"; } >"$f"; rm -rf "$t" ;;
  kgen-pins)
    # Main's side, plus each row the PR added (a new list's acceptance row): KGEN_REPIN only revises rows that exist.
    t=$(mktemp -d)
    git show ":1:$f" >"$t/b" 2>/dev/null; git show ":2:$f" >"$t/o" 2>/dev/null
    if git show ":3:$f" >"$t/m" 2>/dev/null; then
      bash "$here/pr-autoresolve.sh" --union "$t/m" "$t/o" "$t/b" >"$f"
    else git rm -q -- "$f"; rm -rf "$t"; continue; fi
    rm -rf "$t" ;;
  *)
    # Main's side; the recipe rebuilds it below.
    if git cat-file -e ":3:$f" 2>/dev/null; then git checkout -q --theirs -- "$f"; else git rm -q -- "$f"; continue; fi ;;
  esac
  git add -- "$f"
done
[ -z "${recipes[kgen-pins]:-}" ] || recipes[support]=1

# The corpus: the box's keyed store on a runner (ci-corpus.sh), else this tree's FORGE_REF (pr-cards-link.sh).
needs_corpus=
for r in support kgen-pins dirac-layout kop-ordinals; do [ -z "${recipes[$r]:-}" ] || needs_corpus=1; done
if [ -n "$needs_corpus" ]; then
  if [ "${GITHUB_ACTIONS:-}" = true ]; then bash scripts/ci-corpus.sh >/dev/null || { say "ci-corpus.sh failed"; exit 3; }
  else bash scripts/pr-cards-link.sh || { say "no corpus at this tree's FORGE_REF"; exit 3; }; fi
  [ -d .cards/cardsfolder ] || { say "no .cards corpus (the corpus tests would skip)"; exit 3; }
  make -s dirac-tables >/dev/null 2>&1 || { say "make dirac-tables failed"; exit 1; }
fi

# t PKG RUN [TESTENV]: one make test-one; its verdict line is printed, a non-PASS fails.
t() {
  local out
  out=$(make -s test-one PKG="$1" RUN="$2" TESTENV="${3:-}" 2>&1)
  local rc=$?
  echo "$out" | tail -1
  [ $rc -eq 0 ] || { echo "$out" | tail -40; return 1; }
}
pins='^TestCorpusAcceptance$|^TestSetAcceptance$|^TestCorpusBuildAcceptance$|^TestPinTablesParse$|^TestRememberedSpanCensus$|^TestStdMetaPoolBuilds$'
layout='^TestStateLayoutPinned$|^TestNoNewFixedStateFields$|^TestCapacityHashPins$'
fail() { say "$1"; exit 1; }
if [ -n "${recipes[kop-ordinals]:-}" ]; then
  t ./dirac/kop '^TestKopOrdinalsUnchanged$' KOP_ORDINALS_REGEN=1 >/dev/null
  t ./dirac/kop '^TestKopOrdinalsUnchanged$|^TestOrdinalsHoldNoManifestRow$' || fail "ordinals.tsv did not regenerate"
fi
if [ -n "${recipes[dirac-layout]:-}" ]; then
  t ./dirac "$layout" DIRAC_UPDATE_LAYOUT=1 >/dev/null
  t ./dirac "$layout" || fail "layout.tsv did not regenerate"
fi
if [ -n "${recipes[func-ceilings]:-}" ]; then
  t ./internal/codeshape '^TestFuncCeilingsLowerOnly$|^TestLongFunctionsOnlyShrink$|^TestComplexFunctionsOnlyShrink$' ||
    fail "func_ceilings.tsv merged, but a ratchet fails"
fi
if [ -n "${recipes[repro-names]:-}" ]; then
  t ./dirac/oracle '^TestReproGlobMatchesTable$' || fail "repro_table_names.txt merged, but TestReproGlobMatchesTable fails"
fi
if [ -n "${recipes[kgen-pins]:-}" ]; then
  t ./dirac/kgen "$pins" KGEN_REPIN=1 >/dev/null
  t ./dirac/kgen "$pins" DIRAC_PIN_STRICT=1 || fail "kgen acceptance pins did not re-pin (a fall needs KGEN_REPIN_FALL)"
fi
if [ -n "${recipes[support]:-}" ]; then
  t ./dirac/kgen '^TestSupportDocIsCurrent$' DIRAC_UPDATE_SUPPORT=1 >/dev/null
  t ./dirac/kgen '^TestSupportDocIsCurrent$' || fail "support.md did not regenerate"
fi

# Stage every listed generated file a recipe rewrote, and nothing else (a stray build output stays out).
while IFS= read -r f; do recipe_for "$f" >/dev/null && git add -- "$f"; done < <(git diff --name-only)
[ -z "$(git diff --name-only --diff-filter=U)" ] || fail "paths still unmerged after the rebuild"
git "${ident[@]}" commit -q --no-edit || fail "commit failed"
say "merge $(git rev-parse --short=10 HEAD) over $(git rev-parse --short=10 "$head"):"
git diff --stat "$head" HEAD -- "${conflicted[@]}" | tail -n 20
if [ -n "$dry" ]; then say "--dry-run: not pushed${keep:+ (worktree kept: $wt)}"; exit 0; fi
git push -q "$remote" "HEAD:refs/heads/$branch" || fail "push failed (the branch moved? the next run retries)"
say "pushed to $branch"
