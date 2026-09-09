#!/usr/bin/env bash
#
# scripts/smoke.sh — the browser smoke gate (Task SG1, extended by ui19).
#
# Builds the REAL client and the REAL binary, starts THREE `gorged` servers on
# smoke ports (8090-8099) — one `-spectator public`, one `-spectator
# omniscient`, and one SEATED (1v1 play-vs-bot: `-vsbot -humans 1`) — drives
# the headless-browser smoke test in web/e2e against all three, and tears
# every server down (and removes its temp dir) whether the gate passes or
# fails.
#
# The public spectator server is non-negotiable: it is the mode that was
# broken (a literal JSON-null hand spread into the board), and the
# omniscient-only path is exactly what hid the regression from every unit
# test. The SEATED server is the ui19 extension: the earlier public/omni
# gates drive only the two SPECTATOR modes, so two real regressions — a
# hand fan that sized itself from its own output (running a big hand off the
# board) and a seated player's identity bar drawn on top of their own first
# card — sailed through the whole green unit suite to a human screenshot.
# Only a real seated 1v1 client can see them, so the gate now drives one.
# "GET the real ids from /api/tables", "POST /api/games for the join" and
# "fail on any browser error, any stuck loading state, a blank page that never
# mounts" are all in the Playwright spec, not here — this script only builds,
# serves and cleans up.
#
# The servers are started here rather than via Playwright's `webServer`
# because the gate's teardown is a SET-level concern: every server must die
# and every temp dir must go even when the test fails, and a leaked gorged on
# a smoke port poisons the next run.
#
# Run through `make smoke`, or directly: scripts/smoke.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

# vitest/playwright need Node >=24 (the system node is v20). Prefix the v24
# toolchain so the target works from a plain shell with no nvm shim loaded.
export PATH="$HOME/.nvm/versions/node/v24.15.0/bin:$PATH"
if ! node --version 2>/dev/null | grep -q '^v24'; then
  echo "smoke: need Node 24 at $HOME/.nvm/versions/node/v24.15.0/bin (not found on PATH)" >&2
  exit 1
fi

# Vite writes a mutable cache; point it at our own scratch dir rather than
# through web/node_modules, so the build never touches the shared install.
export VITE_CACHE_DIR="${VITE_CACHE_DIR:-/tmp/gorge-smoke-vite-$$}"

echo "== smoke: building client =="
( cd web && npm run build )
echo "== smoke: building gorged =="
mkdir -p bin
CGO_ENABLED=0 go build -o bin/gorged ./cmd/gorged

# ---- allocate three free smoke ports (8090-8099); NEVER 8080/8081 (demo) ----
taken=$(ss -lptn 2>/dev/null | grep -oE ':[0-9]{4}\b' | tr -d ':' | sort -u)
ports=()
for p in $(seq 8090 8099); do
  if ! grep -qx "$p" <<<"$taken"; then
    ports+=("$p")
  fi
  if [ "${#ports[@]}" -ge 3 ]; then break; fi
done
if [ "${#ports[@]}" -lt 3 ]; then
  echo "smoke: need three free ports in 8090-8099 (none available)" >&2
  exit 1
fi
PUBPORT="${ports[0]}"
OMNPORT="${ports[1]}"
SEATPORT="${ports[2]}"

PUBDIR="$(mktemp -d /tmp/gorge-smoke-public-XXXXXX)"
OMNDIR="$(mktemp -d /tmp/gorge-smoke-omni-XXXXXX)"
SEATDIR="$(mktemp -d /tmp/gorge-smoke-seat-XXXXXX)"
SERVER_PIDS=()

cleanup() {
  # Kill by recorded PID (never pkill -f / bare pgrep -f: the pattern matches
  # our own cmdline and has killed a session here). Find by socket if needed.
  for pid in "${SERVER_PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
  # gorged flushes its persistence directory to disk on the way out, so a
  # server that is still dying re-creates files racing `rm -rf` below and the
  # temp dir survives as a half-flushed tables.json. Wait for every recorded
  # pid to actually exit (bounded) before removing the dirs.
  for _ in $(seq 1 50); do
    alive=0
    for pid in "${SERVER_PIDS[@]:-}"; do
      if kill -0 "$pid" 2>/dev/null; then alive=1; fi
    done
    [ "$alive" -eq 0 ] && break
    sleep 0.1
  done
  rm -rf "$PUBDIR" "$OMNDIR" "$SEATDIR" "$VITE_CACHE_DIR"
}
trap cleanup EXIT

wait_ready() {
  local port="$1"
  for _ in $(seq 1 120); do
    if curl -sf -o /dev/null "http://127.0.0.1:$port/api/tables"; then
      return 0
    fi
    sleep 0.5
  done
  return 1
}

start_server() {
  local port="$1" dir="$2" spec="$3" log="$4"
  ./bin/gorged -addr "127.0.0.1:$port" -dir "$dir" -spectator "$spec" \
    -decks internal/testutil/decks -tables 4 -seats 4 -pace 1.5s >"$log" 2>&1 &
  SERVER_PIDS+=("$!")
}

# The SEATED server (ui19) is a 1v1 play-vs-bot server: `-vsbot` arms
# POST /api/games (which returns a join URL seating the client at seat 0 of
# a fresh 2-seat table), and `-humans 1` additionally seats a real human at
# seat 1 of startup table t1 (the -vsbot flow always seats at seat 0, so the
# two together let the gate assert the 1v1 top/bottom mapping from EACH seat).
# `-seat-token` fixes the seat-1 token so the spec can build the join path
# without scraping stderr. `-tables 2 -seats 2` keeps it a real 1v1 table.
start_seated_server() {
  local port="$1" dir="$2" log="$3"
  ./bin/gorged -addr "127.0.0.1:$port" -dir "$dir" -spectator omniscient \
    -decks internal/testutil/decks -tables 2 -seats 2 -pace 1.5s -seed 1 \
    -format constructed -vsbot -humans 1 -seat-token ui19seat1 >"$log" 2>&1 &
  SERVER_PIDS+=("$!")
}

start_server "$PUBPORT" "$PUBDIR" public  "$PUBDIR/server.log"
start_server "$OMNPORT" "$OMNDIR" omniscient "$OMNDIR/server.log"
start_seated_server "$SEATPORT" "$SEATDIR" "$SEATDIR/server.log"

echo "== smoke: starting gorged (public :$PUBPORT, omniscient :$OMNPORT, seated :$SEATPORT) =="
if ! wait_ready "$PUBPORT"; then
  echo "smoke: public gorged on :$PUBPORT never became ready:" >&2
  sed -n '1,60p' "$PUBDIR/server.log" >&2 || true
  exit 1
fi
if ! wait_ready "$OMNPORT"; then
  echo "smoke: omniscient gorged on :$OMNPORT never became ready:" >&2
  sed -n '1,60p' "$OMNDIR/server.log" >&2 || true
  exit 1
fi
if ! wait_ready "$SEATPORT"; then
  echo "smoke: seated gorged on :$SEATPORT never became ready:" >&2
  sed -n '1,60p' "$SEATDIR/server.log" >&2 || true
  exit 1
fi

echo "== smoke: driving the browser gate =="
set +e
( cd web && SMOKE_PUBLIC="http://127.0.0.1:$PUBPORT" SMOKE_OMNI="http://127.0.0.1:$OMNPORT" SMOKE_SEATED="http://127.0.0.1:$SEATPORT" npx playwright test )
status=$?
set -e

if [ "$status" -eq 0 ]; then
  echo "== smoke: PASS (public :$PUBPORT, omniscient :$OMNPORT, seated :$SEATPORT) =="
else
  echo "== smoke: FAIL (public :$PUBPORT, omniscient :$OMNPORT, seated :$SEATPORT) =="
fi
exit "$status"
