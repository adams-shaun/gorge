#!/usr/bin/env bash
# set-census.sh: what the engine lacks for one Forge edition, measured against
# any Forge commit without touching the pinned corpus. It fetches that commit's
# card scripts into CORPUS (keep it outside the repo: the scripts are GPL-3.0),
# reads the edition's card names, and runs TestSetCensus, which prints the cards
# that fail to compile, name unimplemented primitives or carry unread params.
#
#   scripts/set-census.sh CORPUS REF EDITION...
#   scripts/set-census.sh /tmp/forge-master master "Star Trek" "Star Trek Commander"
set -euo pipefail

if [ "$#" -lt 3 ]; then
	echo "usage: set-census.sh CORPUS REF EDITION..." >&2
	exit 2
fi
corpus=$1
ref=$2
shift 2
root=$(git rev-parse --show-toplevel)
cd "$root"

repo=https://github.com/Card-Forge/forge
go run -trimpath ./cmd/forgec fetch -dir "$corpus" -repo "$repo.git" -ref "$ref"
commit=$(sed -n 's/.*"commit": "\(.*\)".*/\1/p' "$corpus/cards.lock")
echo "corpus: $commit"

names=$(mktemp)
trap 'rm -f "$names"' EXIT
for edition in "$@"; do
	: >"$names"
	curl -fsSL --get "https://raw.githubusercontent.com/Card-Forge/forge/$commit/forge-gui/res/editions/$(printf %s "$edition" | sed 's/ /%20/g').txt" |
		awk '/^\[/ { in_cards = ($0 == "[cards]"); next }
			in_cards && NF { sub(/^[^ ]+ [^ ]+ /, ""); sub(/ @.*$/, ""); print }' |
		sort -u >"$names"
	echo "== $edition: $(wc -l <"$names") names"
	GOWORK=off GORGE_SET_CENSUS_CORPUS=$corpus GORGE_SET_CENSUS_NAMES=$names \
		go test -trimpath -timeout 5m -count=1 -v -run '^TestSetCensus$' ./engine/rules |
		sed -n 's/^ *setcensus_test.go:[0-9]*: //p'
done
