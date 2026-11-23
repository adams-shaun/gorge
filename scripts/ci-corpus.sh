#!/usr/bin/env bash
# ci-corpus.sh: on a self-hosted runner, point .cards at a directory kept on the box, so the corpus and dirac's tables
# are not restored and rebuilt by every job (11-16: 1-2 min of every job on metal2). GitHub-hosted runners restore
# .cards from the Actions cache instead (ci.yml).
#
# The box's runners share these directories, so each is keyed by everything that can change its contents:
#   base/<ref>-<cards>        the fetched and compiled corpus (FORGE_REF, engine/cards sources); never written after
#   tree/<ref>-<cards>-<tables>  a copy of base for one set of table sources (every file kgen builds from:
#                                scripts/table-key-files.sh), where make dirac-tables writes its tables
# A PR that changes dirac gets its own tree (a local copy of base, seconds), so it never rewrites the tables another
# job is reading. Trees unused for 2 days are removed.
set -euo pipefail
ref=$(awk '/^FORGE_REF/ {print $3}' Makefile)
h() { git ls-files -z -- "$@" | sort -z | xargs -0 cat | sha256sum | cut -c1-12; }
ck=$ref-$(h 'engine/cards/*.go')
# dk covers every file whose bytes can decide the tables: the non-test Go files and embedded files of each in-module
# package kgen is built from (scripts/table-key-files.sh, from go list), not only tablesrc.go's embeds. 11-21:
# keyed by the embeds alone, a box served tables an older tree had built after engine/rules/pay changed, and batch
# #3110 passed a replay its own tables fail (main red; revert #3141). 11-17: dirac/support alone must re-key too,
# which the embeds in this list still do.
dk=$(bash scripts/table-key-files.sh | tr '\n' '\0' | xargs -0 cat | sha256sum | cut -c1-12)
root=${CI_CORPUS_ROOT:-$HOME/gorge-cards}; base=$root/base/$ck; tree=$root/tree/$ck-$dk
mkdir -p "$root/base" "$root/tree"
# Orphaned temp dirs (11-20: metal2's / sat at 80% for hours with ~1.1 TB in this user's /tmp: go-build work dirs and
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
if ! grep -qs "$ref" "$base/cards.lock"; then
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
exec 9>&-
exec 7>"$tree.lock"; flock 7
if [ ! -e "$tree/.tables-ok" ] && [ -z "${CI_CORPUS_NO_TABLES:-}" ]; then
  rm -rf .cards; ln -sfn "$tree" .cards
  make dirac-tables
  touch "$tree/.tables-ok"
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
# 65, not 85: metal2's cross-engine farm stops launching at 70% used, so a trim that waits for 85% starves it.
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
