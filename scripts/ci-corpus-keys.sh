#!/usr/bin/env bash
# ci-corpus-keys.sh: print, as shell assignments on stdout, the corpus cache keys everything downstream of the
# Forge corpus is keyed by. Sourced by scripts/ci-corpus.sh and eval'd by ci.yml's prepare job, so the box's
# directories and the Actions cache (cardstree-<ck>-<dk>-<RULE>) share one definition and cannot drift. Run
# standalone (`bash scripts/ci-corpus-keys.sh`) it prints the assignments; sourced, it only sets the variables.
#
#   ref    FORGE_REF, the pinned Forge commit (Makefile)
#   ck     ref + a content hash of engine/cards' Go sources — keys the fetched, compiled corpus (base)
#   dk     a content hash of every file ./dirac/cmd/kgen is built from (scripts/table-key-files.sh) — with ck,
#          and the reuse rule, keys the box tree: fetched cards, compiled IR and dirac's card tables
#   RULE   the reuse rule a tree's .tables-ok was marked under (ci-corpus.sh); bump it to orphan every tree marked
#          under an earlier rule rather than serve one
#   root   the corpus root on this machine (CI_CORPUS_ROOT or ~/gorge-cards)
#   base   $root/base/$ck, the fetched and compiled corpus; never written after
#   tree   $root/tree/$ck-$dk-$RULE, where make dirac-tables writes this tree's tables
#
# dk covers every file whose bytes and PATH can decide the tables: the non-test Go files and embedded files of
# each in-module package kgen is built from (scripts/table-key-files.sh, from go list), not only tablesrc.go's
# embeds. The published key hashes each embed's path, length and bytes (gorge.hashSources), so dk must hash the
# list of paths as well as their concatenation: a file added empty, removed or moved among the listed paths
# re-keys the tables while a content-only hash stays. 11-21: keyed by the embeds alone, a box served tables an
# older tree had built after engine/rules/pay changed, and batch #3110 passed a replay its own tables fail (main
# red; revert #3141). 11-17: dirac/support alone must re-key too, which the embeds in this list still do.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
ref=$(awk '/^FORGE_REF/ {print $3}' Makefile)
# LC_ALL=C: ck must be a function of the committed files, never of the box's locale. 2026-11-27: metal2
# (en_US.UTF-8) and spotci (C) logged different ck/dk for the same merge-queue commit because sort -z collated
# the same bytes two ways; dk's list order is pinned the same way in scripts/table-key-files.sh.
h() { git ls-files -z -- "$@" | LC_ALL=C sort -z | xargs -0 cat | sha256sum | cut -c1-12; }
ck=$ref-$(h 'engine/cards/*.go')
kl=$(bash scripts/table-key-files.sh)
dk=$( { printf '%s\n' "$kl"; printf '%s\n' "$kl" | tr '\n' '\0' | xargs -0 cat; } | sha256sum | cut -c1-12)
RULE=2
root=${CI_CORPUS_ROOT:-$HOME/gorge-cards}; base=$root/base/$ck; tree=$root/tree/$ck-$dk-$RULE
# Print only when run standalone: ci.yml's prepare evals the output, while ci-corpus.sh sources this file and the
# values are already in scope.
if [ "${BASH_SOURCE[0]}" = "$0" ]; then
  printf 'ref=%q\nck=%q\ndk=%q\nRULE=%q\nroot=%q\nbase=%q\ntree=%q\n' "$ref" "$ck" "$dk" "$RULE" "$root" "$base" "$tree"
fi
