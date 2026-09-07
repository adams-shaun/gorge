#!/usr/bin/env bash
# agent-worktree.sh — create a task worktree an agent can actually run in.
#
# `git worktree add` alone is NOT enough here. It carries only tracked files,
# and this repo needs one untracked thing to test honestly: the `.cards`
# corpus. Without it internal/testutil's CorpusRegistry calls
#
#     t.Skip("testutil: no .cards/ corpus present -- run `make fetch-cards compile-cards`")
#
# so every corpus-dependent test SKIPS instead of failing, the package still
# prints `ok`, and the agent, the review pre-filter and the gate all read a
# green run that executed almost nothing. That has happened to four worktrees
# across both orchestrator threads, which is why this is a script and not a
# line in a checklist.
#
#   scripts/agent-worktree.sh <id> [base-ref] [--web]
#
#   --web  also link web/node_modules, so a client task can run vitest without
#          copying ~166MB. See the warning at that step before using it.
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel)
ID=${1:?usage: agent-worktree.sh <id> [base-ref] [--web]}
shift
BASE=main
WEB=0
for a in "$@"; do
	case "$a" in
		--web) WEB=1 ;;
		*) BASE=$a ;;
	esac
done

WT="$ROOT/.worktrees/$ID"
say() { printf 'agent-worktree: %s\n' "$*"; }
die() { printf 'agent-worktree: %s\n' "$*" >&2; exit 1; }

[ -e "$WT" ] && die "$WT already exists"

git -C "$ROOT" worktree add "$WT" -b "wt/$ID" "$BASE" >/dev/null
say "created $WT on wt/$ID from $BASE"

mkdir -p "$WT/.ds4"
grep -qx '.ds4' "$ROOT/.git/info/exclude" 2>/dev/null || echo '.ds4' >> "$ROOT/.git/info/exclude"

# The corpus. An absolute symlink, so it survives however the worktree is
# entered, and pointed at the main checkout's copy rather than duplicated:
# it is large, gitignored (GPL Forge scripts -- never commit one) and
# read-only in practice.
[ -d "$ROOT/.cards" ] || die "no $ROOT/.cards to link — run 'make fetch-cards compile-cards' in the main checkout first"
ln -sfn "$ROOT/.cards" "$WT/.cards"
say "linked .cards -> $ROOT/.cards"

if [ "$WEB" = 1 ]; then
	# WARNING: this is a SHARED tree, not a copy. `npm run test`/`build`
	# only read it, which is what a task agent does; `npm ci` or
	# `npm install` inside the worktree would rewrite the main checkout's
	# node_modules underneath every other worktree. Brief the agent not to
	# install, or drop --web and let it copy.
	if [ -d "$ROOT/web/node_modules" ]; then
		ln -sfn "$ROOT/web/node_modules" "$WT/web/node_modules"
		say "linked web/node_modules (SHARED — the agent must not run npm ci in this worktree)"
	else
		say "no web/node_modules in the main checkout to link; skipping"
	fi
fi

# VERIFY, do not assume. The whole point of this script is a failure mode
# that looks like success, so it ends by proving the corpus is reachable
# rather than by reporting that it made a symlink.
say "verifying the corpus is reachable (the skip this script exists to prevent)..."
out=$(cd "$WT" && GOMEMLIMIT=5GiB go test -p=2 ./rules/ -run 'TestHeads' -v 2>&1 || true)
if grep -q 'no .cards/ corpus present' <<<"$out"; then
	die "corpus STILL not reachable — tests would skip and read as green. Fix before dispatching."
fi
if ! grep -qE '^(ok|--- PASS|PASS)' <<<"$out"; then
	say "WARNING: TestHeads did not report PASS/ok in this worktree — read it before dispatching:"
	printf '%s\n' "$out" | tail -5
	exit 1
fi
say "corpus reachable, TestHeads runs. Worktree ready: $WT"
