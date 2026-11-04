#!/usr/bin/env bash
# demo-ports.sh — the one home for "which ports must the demo port sweep
# cover". Sourced by scripts/deploy-demo.sh.
#
# deploy-demo.sh retires a listener by emptying its *_PORT variable. If the
# sweep were built only from the current *_PORT values, emptying one would
# also drop that port from the sweep, and the server a previous layout left
# bound there would never be stopped: a second standing gorged nobody owns.
#
# So the sweep is the union of every port this demo has ever bound
# (DEMO_PORT_HISTORY, append-only) and every port this run binds. When a
# layout adds a listener on a new port, append that port to the history in
# deploy/demo.env in the same change, so retiring it later cannot take a
# previous layout's server off the sweep.
#
# The history is a property of the box the demo runs on, so it lives in the
# untracked deploy/demo.env, not here; it defaults to empty. An explicitly
# empty value clears it, which is what a test deploy sweeping only its own
# ports relies on (cmd/gorged/deploy_test.go).
DEMO_PORT_HISTORY=${DEMO_PORT_HISTORY-}

# demo_sweep_ports <bind-port>... — every port the sweep must match, one per
# line, deduped and sorted. Blank or empty arguments are ignored, so a retired
# *_PORT (empty) contributes nothing instead of poisoning the regex with `|`.
demo_sweep_ports() {
	# Word splitting of the history is the point: it is a space-separated list.
	# shellcheck disable=SC2086
	printf '%s\n' $DEMO_PORT_HISTORY "$@" |
		grep -E '^[0-9]+$' | sort -u
}
