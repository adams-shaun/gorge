#!/usr/bin/env bash
# Smoke-test the bot examples the docs site publishes (docs/site/bots/):
# build and test the native Go example module, then play one complete game
# each with the Python and the Java HTTP agents against a throwaway gorged.
#
# Needs the corpus (.cards), Go, python3 and a JDK 21 (java Agent.java uses
# the source launcher, so jdk.compiler must be present).
#
#   scripts/examples-smoke.sh [port]     # default 8097
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
port=${1:-8097}
work=$(mktemp -d "${TMPDIR:-/tmp}/gorge-examples.XXXXXX")
pid=
cleanup() {
	if [ -n "$pid" ]; then kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true; fi
	rm -rf "$work"
}
trap cleanup EXIT

test -s "$root/.cards/cards.lock" || { echo "no .cards corpus: run make fetch-cards compile-cards" >&2; exit 1; }

echo "== examples/bots/go: vet, test, build"
(cd "$root/examples/bots/go" && go vet ./... && go test -timeout 5m ./... && go build ./...)

echo "== gorged on 127.0.0.1:$port"
go build -o "$work/gorged" "$root/cmd/gorged"
(cd "$root" && exec "$work/gorged" -addr "127.0.0.1:$port" -dir "$work/data" -feedback "$work/feedback" \
	-tables 0 -vsbot -max-on-demand-tables 0 -pace 0 -prewarm=false -bot-policy bot \
	>"$work/gorged.log" 2>&1) &
pid=$!
for _ in $(seq 1 120); do
	curl -fsS "http://127.0.0.1:$port/api/tables" >/dev/null 2>&1 && break
	kill -0 "$pid" 2>/dev/null || { cat "$work/gorged.log"; exit 1; }
	sleep 0.5
done
curl -fsS "http://127.0.0.1:$port/api/tables" >/dev/null || { cat "$work/gorged.log"; exit 1; }

play() { # name, command...
	local name=$1; shift
	echo "== $name"
	local out="$work/$name.out"
	if ! timeout 300 "$@" >"$out" 2>&1; then
		tail -40 "$out"; echo "$name: agent failed" >&2; exit 1
	fi
	tail -3 "$out"
	grep -q '^match finished' "$out" || { echo "$name: no finished match" >&2; exit 1; }
}

server="http://127.0.0.1:$port"
play python python3 "$root/examples/bots/python/gorge_agent.py" --server "$server" --policy simple --seed 1
play java java "$root/examples/bots/java/Agent.java" --server "$server" --policy simple --seed 1
echo "examples smoke: OK"
