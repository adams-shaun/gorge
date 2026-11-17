#!/usr/bin/env bash
# ci-corpus.sh: on a self-hosted runner, point .cards at a directory kept on the box, so the corpus and dirac's tables
# are not restored and rebuilt by every job (11-16: 1-2 min of every job on metal2). GitHub-hosted runners restore
# .cards from the Actions cache instead (ci.yml).
#
# The box's runners share these directories, so each is keyed by everything that can change its contents:
#   base/<ref>-<cards>        the fetched and compiled corpus (FORGE_REF, engine/cards sources); never written after
#   tree/<ref>-<cards>-<tables>  a copy of base for one set of table sources (the files tablesrc.go embeds), where make dirac-tables writes its tables
# A PR that changes dirac gets its own tree (a local copy of base, seconds), so it never rewrites the tables another
# job is reading. Trees unused for 2 days are removed.
set -euo pipefail
ref=$(awk '/^FORGE_REF/ {print $3}' Makefile)
h() { git ls-files -z -- "$@" | sort -z | xargs -0 cat | sha256sum | cut -c1-12; }
ck=$ref-$(h 'engine/cards/*.go')
# dk covers exactly what the tables' key covers: the files tablesrc.go embeds (gorge.TableSourceHash), test files
# excluded as it excludes them. 11-17: hashing dirac/*.go alone reused a tree whose tables were built for another key
# when a PR changed only dirac/support (evidence), and every dirac test failed "no card tables for this binary".
# The patterns are read from tablesrc.go's go:embed lines, so the two cannot drift.
tablesrc_paths() {
  # A go:embed pattern that names a directory embeds the tree under it, so each pattern also matches as a prefix.
  sed -n 's,^//go:embed ,,p' tablesrc.go | tr ' ' '\n' | sed '/^$/d' | sed 's,^\(.*\)$,:(glob)\1\n:(glob)\1/**,'
}
dk=$(tablesrc_paths | tr '\n' '\0' | xargs -0 git ls-files -z -- | tr '\0' '\n' | grep -v '_test\.go$' | sort | tr '\n' '\0' | xargs -0 cat | sha256sum | cut -c1-12)
root=${CI_CORPUS_ROOT:-$HOME/gorge-cards}; base=$root/base/$ck; tree=$root/tree/$ck-$dk
mkdir -p "$root/base" "$root/tree"
exec 9>"$root/.lock"; flock 9
if ! grep -qs "$ref" "$base/cards.lock"; then
  rm -rf "$base.tmp"; mkdir -p "$base.tmp"; rm -rf .cards; ln -sfn "$base.tmp" .cards
  make fetch-cards compile-cards
  rm -rf "$base"; mv "$base.tmp" "$base"
fi
[ -d "$tree" ] || cp -a "$base" "$tree"
# dirac's tables for this tree, built once (the first job of the tree: the prepare stage), under the lock.
if [ ! -e "$tree/.tables-ok" ] && [ -z "${CI_CORPUS_NO_TABLES:-}" ]; then
  rm -rf .cards; ln -sfn "$tree" .cards
  make dirac-tables
  touch "$tree/.tables-ok"
fi
touch "$tree"
find "$root/tree" -mindepth 1 -maxdepth 1 -type d -mtime +2 -exec rm -rf {} + 2>/dev/null || true
# .cards is a real directory of hard links to the tree, not a symlink: filepath.WalkDir does not follow a symlink at
# its root, so a corpus walk over a symlinked .cards finds nothing (11-16: TestNumericPredicateCorpusCensus "found
# only 0 numeric tokens"). Hard links take seconds, share the files and keep their mtimes (the Go test cache).
rm -rf .cards; cp -al "$tree" .cards 2>/dev/null || { rm -rf .cards; cp -a "$tree" .cards; }
echo "ci-corpus: .cards = hard links to $tree"
