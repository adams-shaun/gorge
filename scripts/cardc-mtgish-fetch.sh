#!/usr/bin/env bash
# cardc-mtgish-fetch.sh -- download the pinned mtgish data for cardc and verify
# it against engine/cardc/MTGISH.lock.
#
# The files land in ${CARDS_DIR:-.cards}/mtgish/<ref>/ (gitignored):
# mtgish.lines.json carries Oracle text (Wizards' copyright) and is never
# committed. Idempotent: a file already present with the locked sha256 is not
# downloaded again. Exit 1 on a sha256 mismatch, 2 on a ref that differs from
# the lock.
#
#   scripts/cardc-mtgish-fetch.sh      # MTGISH_REF from the environment, else cardc.mk
set -euo pipefail

repo=$(git rev-parse --show-toplevel)
lock="$repo/engine/cardc/MTGISH.lock"
ref=${MTGISH_REF:-$(sed -n 's/^MTGISH_REF *?= *//p' "$repo/cardc.mk")}
lock_ref=$(awk '$1 == "ref" {print $2}' "$lock")
[ -n "$ref" ] || { echo "mtgish-fetch: MTGISH_REF unset" >&2; exit 2; }
[ "$ref" = "$lock_ref" ] || { echo "mtgish-fetch: MTGISH_REF $ref != $lock ref $lock_ref (bump both together)" >&2; exit 2; }
lock_repo=$(awk '$1 == "repo" {print $2}' "$lock")
base=${MTGISH_BASE_URL:-$(echo "$lock_repo" | sed 's#^https://github.com/#https://raw.githubusercontent.com/#')}
dest="${CARDS_DIR:-$repo/.cards}/mtgish/$ref"
mkdir -p "$dest"

status=0
while read -r kind sum path; do
	[ "$kind" = sha256 ] || continue
	out="$dest/$(basename "$path")"
	if [ -f "$out" ] && [ "$(sha256sum "$out" | cut -d' ' -f1)" = "$sum" ]; then
		echo "mtgish-fetch: $out ok"
		continue
	fi
	tmp="$out.part"
	curl -sSfL --retry 3 -o "$tmp" "$base/$ref/$path"
	got=$(sha256sum "$tmp" | cut -d' ' -f1)
	if [ "$got" != "$sum" ]; then
		echo "mtgish-fetch: $path: sha256 $got, lock says $sum" >&2
		rm -f "$tmp"
		status=1
		continue
	fi
	mv "$tmp" "$out"
	echo "mtgish-fetch: $out fetched"
done <"$lock"
exit $status
