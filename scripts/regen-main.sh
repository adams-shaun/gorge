#!/usr/bin/env bash
# regen-main.sh [REMOTE]: keep one pull request open that brings main's derived records up to main's tree: the
# support page and kgen's acceptance pins, which tickets no longer carry for a rise (scripts/pr-derived.sh). Run it
# from a checkout with the corpus (it builds a worktree off REMOTE/main), as the operator's gh user: a pull request
# pushed with a workflow's GITHUB_TOKEN would start no CI. A regen PR in the merge queue is left alone, since a push
# would reset its batch; the next run after it merges carries what moved since.
set -uo pipefail
remote=${1:-origin}
branch=regen/main
repo=$(git rev-parse --show-toplevel)
main=$(cd "$(git rev-parse --git-common-dir)/.." && pwd)
cap=(systemd-run --user --scope -q -p MemoryMax=4G -p CPUQuota=400% env GOMAXPROCS=4 GOMEMLIMIT=3GiB)
pins='^(TestCorpusAcceptance|TestSetAcceptance|TestCorpusBuildAcceptance|TestPinTablesParse|TestRememberedSpanCensus|TestStdMetaPoolBuilds)$'
files=(dirac/docs/support.md dirac/kgen/testdata/acceptance.tsv dirac/kgen/testdata/build_acceptance.tsv dirac/kgen/testdata/remspan.tsv dirac/kgen/stdmeta_test.go)

pr=$(gh pr list --head "$branch" --state open --json number,labels,mergeable,statusCheckRollup --jq '.[0] | select(.) | "\(.number) \(if any(.statusCheckRollup[]?; .name == "fast" and .conclusion == "FAILURE") then "RED" else .mergeable end) \([.labels[].name] | join(","))"')
# An open regen PR that still merges is left alone: a push restarts its checks, and main moves more often than they
# finish (11-17: #749 re-pushed every tick, its fast never ended). It carries a rise up to its own base; the next run
# after it merges carries the rest. Only a conflicting one, or one whose fast is red, is rebuilt (11-24: #4249 was cut
# over 9499e39379, main then re-pinned remspan up, and the open PR sat red lowering it back).
case " $pr " in "  ") ;; *" CONFLICTING "*|*" RED "*) ;; *) echo "regen-main: #${pr%% *} is open and merges; left alone"; exit 0 ;; esac

git -C "$repo" fetch -q "$remote" main || { echo "regen-main: fetch failed"; exit 1; }
wt=$main/.worktrees/regen-main
git -C "$repo" worktree remove --force "$wt" 2>/dev/null; rm -rf "$wt"
git -C "$repo" worktree add -q --detach "$wt" "$remote/main" || { echo "regen-main: worktree add failed"; exit 1; }
trap 'git -C "$repo" worktree remove --force "$wt" 2>/dev/null' EXIT
cd "$wt" || exit 1
source scripts/pr-cards.sh
"${cap[@]}" env KGEN_REPIN=1 go test -timeout 5m -run "$pins" ./dirac/kgen >/dev/null 2>&1
"${cap[@]}" env DIRAC_UPDATE_SUPPORT=1 go test -timeout 5m -run '^TestSupportDocIsCurrent$' ./dirac/kgen >/dev/null 2>&1
"${cap[@]}" env DIRAC_PIN_STRICT=1 DIRAC_SUPPORT_STRICT=1 go test -timeout 5m -run "${pins%)\$}|TestSupportDocIsCurrent)\$" ./dirac/kgen >/dev/null 2>&1 ||
  { echo "regen-main: main's records do not hold exact after a regen (a fall: KGEN_REPIN refuses it)"; exit 1; }
if git diff --quiet -- "${files[@]}"; then echo "regen-main: main's derived records are current"; exit 0; fi
git add -- "${files[@]}"
git commit -q -m "dirac: regenerate the support page and kgen's acceptance pins over main ($(git rev-parse --short=10 HEAD))" ||
  { echo "regen-main: commit failed"; exit 1; }
git push -q -f "$remote" "HEAD:refs/heads/$branch" || { echo "regen-main: push failed"; exit 1; }
if [ -z "$pr" ]; then
  body=$'Tickets leave a rise in these derived records to this one PR (scripts/pr-derived.sh), so they stop conflicting on them. Regenerated over main by scripts/regen-main.sh.\n\n---\nThis Robot (gorge agentctl swarm) Shan\'t Be Blamed For Stupid Humans\n'
  url=$(gh pr create --base main --head "$branch" --title "dirac: regenerate main's support page and acceptance pins" --body "$body") ||
    { echo "regen-main: gh pr create failed"; exit 1; }
  pr=${url##*/}
fi
gh api -X POST "repos/{owner}/{repo}/issues/${pr%% *}/labels" -f 'labels[]=queue' >/dev/null || { echo "regen-main: label failed"; exit 1; }
echo "regen-main: pushed $(git diff --stat HEAD~1 | tail -1) to #${pr%% *}"
