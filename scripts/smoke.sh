#!/usr/bin/env bash
#
# scripts/smoke.sh — the browser smoke gate (Task SG1).
#
# Builds the REAL client and the REAL binary, starts TWO `gorged` servers on
# smoke ports (8090-8099) — one `-spectator public`, one `-spectator
# omniscient` — drives the headless-browser smoke test in web/e2e against
# both, and tears both servers down (and removes their temp dirs) whether the
# gate passes or fails.
#
# The public spectator server is non-negotiable: it is the mode that was
# broken (a literal JSON-null hand spread into the board), and the
# omniscient-only path is exactly what hid the regression from every unit
# test. "GET the real ids from /api/tables" and "fail on any browser error,
# any stuck loading state, a blank page that never mounts" are all in the
# Playwright spec, not here — this script only builds, serves and cleans up.
#
# The two servers are started here rather than via Playwright's `webServer`
# because the gate's teardown is a PAIR-level concern: both servers must die
# and both temp dirs must go even when the test fails, and a leaked gorged on
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

# ---- allocate two free smoke ports (8090-8099); NEVER 8080/8081 (demo) ----
taken=$(ss -lptn 2>/dev/null | grep -oE ':[0-9]{4}\b' | tr -d ':' | sort -u)
ports=()
for p in $(seq 8090 8099); do
  if ! grep -qx "$p" <<<"$taken"; then
    ports+=("$p")
  fi
  if [ "${#ports[@]}" -ge 2 ]; then break; fi
done
if [ "${#ports[@]}" -lt 2 ]; then
  echo "smoke: need two free ports in 8090-8099 (none available)" >&2
  exit 1
fi
PUBPORT="${ports[0]}"
OMNPORT="${ports[1]}"

PUBDIR="$(mktemp -d /tmp/gorge-smoke-public-XXXXXX)"
OMNDIR="$(mktemp -d /tmp/gorge-smoke-omni-XXXXXX)"
SERVER_PIDS=()

cleanup() {
  # Kill by recorded PID (never pkill -f / bare pgrep -f: the pattern matches
  # our own cmdline and has killed a session here). Find by socket if needed.
  for pid in "${SERVER_PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
  rm -rf "$PUBDIR" "$OMNDIR" "$VITE_CACHE_DIR"
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

start_server "$PUBPORT" "$PUBDIR" public  "$PUBDIR/server.log"
start_server "$OMNPORT" "$OMNDIR" omniscient "$OMNDIR/server.log"

echo "== smoke: starting gorged (public :$PUBPORT, omniscient :$OMNPORT) =="
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

echo "== smoke: driving the browser gate =="
set +e
( cd web && SMOKE_PUBLIC="http://127.0.0.1:$PUBPORT" SMOKE_OMNI="http://127.0.0.1:$OMNPORT" npx playwright test )
status=$?
set -e

if [ "$status" -eq 0 ]; then
  echo "== smoke: PASS (public :$PUBPORT, omniscient :$OMNPORT) =="
else
  echo "== smoke: FAIL (public :$PUBPORT, omniscient :$OMNPORT) =="
fi
exit "$status"
