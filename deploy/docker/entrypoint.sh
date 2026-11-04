#!/bin/sh
# Fetch the Forge card scripts into /cards on first start (and whenever the
# pinned commit changes), then run the command. The scripts are GPL-3.0, so
# they live in a volume and never in the image. gorged compiles the IR cache
# into the same directory on its first open.
set -eu

cards=${CARDS_DIR:-/cards}
marker=$cards/.fetched-ref

if [ -n "${FORGE_REF:-}" ]; then
	if [ ! -d "$cards/cardsfolder" ] || [ "$(cat "$marker" 2>/dev/null || true)" != "$FORGE_REF" ]; then
		echo "entrypoint: fetching the Forge card scripts at $FORGE_REF into $cards (first start; a few minutes)" >&2
		forgec fetch -dir "$cards" -ref "$FORGE_REF"
		printf '%s\n' "$FORGE_REF" >"$marker"
	fi
elif [ ! -d "$cards/cardsfolder" ]; then
	echo "entrypoint: $cards has no cardsfolder and FORGE_REF is unset; build with --build-arg FORGE_REF=<sha> or mount a fetched corpus" >&2
	exit 1
fi

exec "$@"
