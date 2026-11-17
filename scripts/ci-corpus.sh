#!/usr/bin/env bash
# ci-corpus.sh: on a self-hosted runner, point .cards at a directory kept on the box, so the corpus and dirac's tables
# are not restored and rebuilt by every job (11-16: 1-2 min of every job on metal2). GitHub-hosted runners restore
# .cards from the Actions cache instead (ci.yml).
#
# The box's runners share these directories, so each is keyed by everything that can change its contents:
#   base/<ref>-<cards>        the fetched and compiled corpus (FORGE_REF, engine/cards sources); never written after
#   tree/<ref>-<cards>-<dirac>  a copy of base for one set of dirac sources, where make dirac-tables writes its tables
# A PR that changes dirac gets its own tree (a local copy of base, seconds), so it never rewrites the tables another
# job is reading. Trees unused for 2 days are removed.
set -euo pipefail
ref=$(awk '/^FORGE_REF/ {print $3}' Makefile)
h() { git ls-files -z -- "$@" | sort -z | xargs -0 cat | sha256sum | cut -c1-12; }
ck=$ref-$(h 'engine/cards/*.go')
dk=$(git ls-files -- 'dirac/*.go' | grep -v '_test\.go$' | sort | tr '\n' '\0' | xargs -0 cat | sha256sum | cut -c1-12)
root=${CI_CORPUS_ROOT:-$HOME/gorge-cards}; base=$root/base/$ck; tree=$root/tree/$ck-$dk
mkdir -p "$root/base" "$root/tree"
exec 9>"$root/.lock"; flock 9
if ! grep -qs "$ref" "$base/cards.lock"; then
  rm -rf "$base.tmp"; mkdir -p "$base.tmp"; rm -rf .cards; ln -sfn "$base.tmp" .cards
  make fetch-cards compile-cards
  rm -rf "$base"; mv "$base.tmp" "$base"
fi
[ -d "$tree" ] || cp -a "$base" "$tree"
touch "$tree"
find "$root/tree" -mindepth 1 -maxdepth 1 -type d -mtime +2 -exec rm -rf {} + 2>/dev/null || true
rm -rf .cards; ln -sfn "$tree" .cards
echo "ci-corpus: .cards -> $tree"
