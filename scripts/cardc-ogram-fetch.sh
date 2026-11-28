#!/usr/bin/env bash
# cardc-ogram-fetch.sh -- download the pinned oracle-grammar release assets for
# cardc and verify them against engine/cardc/OGRAM.lock.
#
# The files land in ${CARDS_DIR:-.cards}/ogram/<tag>/ (gitignored): the
# .jsonl.gz assets carry Oracle text (Wizards' copyright) and are never
# committed. `sha256` lines are release assets; a `schema` line is a file of the
# repository at the lock's commit, landed under its base name. Idempotent: a file already present with the locked sha256 is not
# downloaded again. Exit 1 on a sha256 mismatch, 2 on a tag that differs from
# the lock.
#
#   scripts/cardc-ogram-fetch.sh      # OGRAM_TAG from the environment, else cardc.mk
set -euo pipefail

repo=$(git rev-parse --show-toplevel)
lock="$repo/engine/cardc/OGRAM.lock"
tag=${OGRAM_TAG:-$(sed -n 's/^OGRAM_TAG *?= *//p' "$repo/cardc.mk")}
lock_tag=$(awk '$1 == "tag" {print $2}' "$lock")
[ -n "$tag" ] || { echo "ogram-fetch: OGRAM_TAG unset" >&2; exit 2; }
[ "$tag" = "$lock_tag" ] || { echo "ogram-fetch: OGRAM_TAG $tag != $lock tag $lock_tag (bump both together)" >&2; exit 2; }
lock_repo=$(awk '$1 == "repo" {print $2}' "$lock")
lock_commit=$(awk '$1 == "commit" {print $2}' "$lock")
base=${OGRAM_BASE_URL:-$lock_repo/releases/download/$tag}
raw=${OGRAM_RAW_URL:-$(echo "$lock_repo" | sed 's#^https://github.com/#https://raw.githubusercontent.com/#')/$lock_commit}
dest="${CARDS_DIR:-$repo/.cards}/ogram/$tag"
mkdir -p "$dest"

status=0
while read -r kind sum name; do
	case "$kind" in
	sha256) url="$base/$name" ;;
	schema) url="$raw/$name" ;;
	*) continue ;;
	esac
	out="$dest/$(basename "$name")"
	if [ -f "$out" ] && [ "$(sha256sum "$out" | cut -d' ' -f1)" = "$sum" ]; then
		echo "ogram-fetch: $out ok"
		continue
	fi
	tmp="$out.part"
	curl -sSfL --retry 3 -o "$tmp" "$url"
	got=$(sha256sum "$tmp" | cut -d' ' -f1)
	if [ "$got" != "$sum" ]; then
		echo "ogram-fetch: $name: sha256 $got, lock says $sum" >&2
		rm -f "$tmp"
		status=1
		continue
	fi
	mv "$tmp" "$out"
	echo "ogram-fetch: $out fetched"
done <"$lock"
exit $status
