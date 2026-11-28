#!/usr/bin/env bash
# ci-corpus.sh: on a self-hosted runner, point .cards at a directory kept on the box, so the corpus and dirac's tables
# are not restored and rebuilt by every job (11-16: 1-2 min of every job on the owned box). GitHub-hosted runners restore
# .cards from the Actions cache instead (ci.yml).
#
# The box's runners share these directories, so each is keyed by everything that can change its contents:
#   base/<ref>-<cards>            the fetched and compiled corpus (FORGE_REF, engine/cards sources); never written after
#   tree/<ref>-<cards>-<tables>-<rule>  a copy of base for one set of table sources (every file kgen builds from:
#                                 scripts/table-key-files.sh), where make dirac-tables writes its tables; <rule>
#                                 is the reuse rule the tree's .tables-ok was marked under (bump it to orphan every
#                                 tree marked earlier)
# A PR that changes dirac gets its own tree (a local copy of base, seconds), so it never rewrites the tables another
# job is reading. Trees unused for 2 days are removed.
set -euo pipefail
# The keys (ref, ck, dk, RULE, root, base, tree) come from scripts/ci-corpus-keys.sh, the one definition ci.yml's
# prepare job also evals to key the Actions cache (cardstree-<ck>-<dk>-<RULE>). That script's comment carries the
# dk and RULE rationale; sourcing it keeps the box's directories and the cache key from drifting.
. "$(dirname "$0")/ci-corpus-keys.sh"
mkdir -p "$root/base" "$root/tree"
# Orphaned temp dirs (11-20: the owned box's / sat at 80% for hours with ~1.1 TB in this user's /tmp: go-build work dirs and
# test temp dirs from 11-17 on, left by jobs the merge queue cancelled; go removes its work dir only on a clean exit).
# Every job removes this user's top-level temp directories whose newest entry is older than CI_TMP_TRIM_MIN (720)
# minutes, twice GitHub's 6 h job cap, so no live job's dir qualifies. One job at a time, and before the corpus lock,
# so a long first pass never holds up another job's corpus.
(
  exec 8>"$root/.tmp-trim.lock"; flock -n 8 || exit 0
  age=${CI_TMP_TRIM_MIN:-720}; me=$(id -u); n=0
  for d in "${CI_TMP_DIR:-/tmp}"/*; do
    [ -d "$d" ] && [ ! -L "$d" ] && [ "$(stat -c %u "$d")" = "$me" ] || continue
    [ -n "$(find "$d" -mmin -"$age" -print -quit 2>/dev/null)" ] && continue
    rm -rf -- "$d" 2>/dev/null || true; n=$((n + 1))
  done
  [ "$n" -eq 0 ] || echo "ci-corpus: removed $n orphaned temp dirs (older than $age min)"
) || true
exec 9>"$root/.lock"; flock 9
# A complete corpus tree restored from the Actions cache (ci.yml's prepare, self-hosted) arrives in
# $CI_CORPUS_RESTORE: a workspace-local directory with .tables-ok and a cards.lock at FORGE_REF. Install it as the
# box's tree; the key cardstree-<ck>-<dk>-<RULE> is this tree's every input, so a hit cannot carry a tree built
# from different sources (the 11-21 failure mode has no differing input to hide behind). The restore step finishes
# (or fails, and the workflow deletes its directory) before this runs, so a partial restore never reaches here.
# No other caller sets CI_CORPUS_RESTORE.
if [ -n "${CI_CORPUS_RESTORE:-}" ] && [ ! -e "$tree/.tables-ok" ] && [ -e "$CI_CORPUS_RESTORE/.tables-ok" ] && grep -qs "$ref" "$CI_CORPUS_RESTORE/cards.lock"; then
  rm -rf "$tree"; cp -a "$CI_CORPUS_RESTORE" "$tree"; touch "$tree"
  echo "ci-corpus: installed the restored corpus tree $tree"
fi
# base is only ever a source for trees: a complete tree on the box (built here, or installed above) makes fetching
# and compiling a missing base pointless work.
if ! grep -qs "$ref" "$base/cards.lock" && [ ! -e "$tree/.tables-ok" ]; then
  rm -rf "$base.tmp"; mkdir -p "$base.tmp"; rm -rf .cards; ln -sfn "$base.tmp" .cards
  make fetch-cards compile-cards
  rm -rf "$base"; mv "$base.tmp" "$base"
fi
[ -d "$tree" ] || cp -a "$base" "$tree"
# Touched before the box lock is released: cp -a keeps base's old mtime, and the trims below (another job's, under
# the box lock) remove trees by age.
touch "$tree"
# dirac's tables for this tree, built once (the first job of the tree: the prepare stage), under the tree's own
# lock, not the box's: a job waits only for a build of its own tables. 11-22: queue-precheck's attribution builds a
# tree per queue prefix, 36-62 s each, and under the box lock they ran one at a time, behind every other job's.
#
# A tree is marked .tables-ok (reusable by later checkouts) only by a build that left the checkout's committed bytes
# intact. When kgen -tables finds a committed ids.tsv its own sources no longer produce, it rewrites the file IN THE
# CHECKOUT (its exit 3) and make rebuilds kgen around the rewritten bytes: the tables then carry a published key
# (gorge.TableSourceHash over the embeds) that exists only in that mutated workspace, and marking the tree would
# serve every later fresh checkout of the same stack tables its own binary asks for under another key — every dirac
# test red with "no card tables for this binary" (11-26: 703 reds, 10 merge-queue batches bisected ~2 h, #5256 and
# #4754 dequeued; the run that built the tree passed green, its tests compiled after the rewrite). The Makefile
# target leaves bin/.ids-rewritten behind when it did that; such a build leaves the tree unmarked, so every checkout
# rebuilds for itself (~52 s) and runs green on its own tables until the stale ids.tsv is repinned, and reuse
# resumes by itself.
exec 9>&-
exec 7>"$tree.lock"; flock 7
if [ ! -e "$tree/.tables-ok" ] && [ -z "${CI_CORPUS_NO_TABLES:-}" ]; then
  rm -rf .cards; ln -sfn "$tree" .cards
  make dirac-tables
  if [ -e bin/.ids-rewritten ]; then
    rm -f bin/.ids-rewritten
    echo "ci-corpus: kgen rewrote ids.tsv in this checkout; $tree left unmarked (its tables match no committed tree)" >&2
  else
    touch "$tree/.tables-ok"
  fi
fi
exec 7>&-
exec 9>"$root/.lock"; flock 9
touch "$tree"
find "$root/tree" -mindepth 1 -maxdepth 1 -type d -mtime +2 -exec rm -rf {} + 2>/dev/null || true
find "$root/tree" -mindepth 1 -maxdepth 1 -type f -name '*.lock' -mtime +2 -delete 2>/dev/null || true
# Disk pressure (11-17 22:50: metal's / hit 100%, and runners died "No space left on device" mid-step). Nothing else
# trims what this box's runners write: the trees above (a full copy per table key, dozens a day) and the shared Go
# cache. At CI_DISK_TRIM_PCT (65) percent used, remove trees not used for 3 h and Go cache files not used for 6 h.
# Both are safe while jobs run: a job's .cards is hard links (it keeps its data if its tree goes), and go refreshes
# a cache entry's mtime at most hourly when it uses it, so a 6 h idle entry is not live work.
# 65, not 85: the owned box's cross-engine farm stops launching at 70% used, so a trim that waits for 85% starves it.
used=$(df --output=pcent / | tail -1 | tr -dc 0-9)
if [ "${used:-0}" -ge "${CI_DISK_TRIM_PCT:-65}" ]; then
  echo "ci-corpus: / is ${used}% used; trimming idle corpus trees and Go cache"
  find "$root/tree" -mindepth 1 -maxdepth 1 -type d ! -path "$tree" -mmin +180 -exec rm -rf {} + 2>/dev/null || true
  gc=$(go env GOCACHE 2>/dev/null || true)
  [ -n "$gc" ] && [ -d "$gc" ] && find "$gc" -type f -mmin +360 -delete 2>/dev/null || true
  df -h / | tail -1
fi
# .cards is a real directory of hard links to the tree, not a symlink: filepath.WalkDir does not follow a symlink at
# its root, so a corpus walk over a symlinked .cards finds nothing (11-16: TestNumericPredicateCorpusCensus "found
# only 0 numeric tokens"). Hard links take seconds, share the files and keep their mtimes (the Go test cache).
rm -rf .cards; cp -al "$tree" .cards 2>/dev/null || { rm -rf .cards; cp -a "$tree" .cards; }
echo "ci-corpus: .cards = hard links to $tree"
