#!/usr/bin/env bash
#
# scripts/smoke.sh — the browser smoke gate.
#
# Builds the REAL client and the REAL binary, starts SEVEN `gorged` servers,
# each bound to a kernel-assigned free port (127.0.0.1:0; the port is read back
# from the server's own startup line) — public and omniscient spectators, a SEATED 1v1,
# the shared mana-wheel board fixture, the Talisman two-stage continuation fixture,
# and one server per engine offering all three wires — then drives
# the headless-browser smoke test in web/e2e against all seven, and tears
# every server down (and removes its temp dir) whether the gate passes or
# fails.
#
# The public spectator server is non-negotiable: it is the mode that was
# broken (a literal JSON-null hand spread into the board), and the
# omniscient-only path is exactly what hid the regression from every unit
# test. The SEATED server exists because the earlier public/omni
# gates drive only the two SPECTATOR modes, so two real regressions — a
# hand fan that sized itself from its own output (running a big hand off the
# board) and a seated player's identity bar drawn on top of their own first
# card — sailed through the whole green unit suite to a human screenshot.
# Only a real seated 1v1 client can see them, so the gate now drives one.
# The wire servers are where the client plays a seated game over native, ManaBrew
# and SpellBench v2, on gorge and on dirac; the spec names each pair in its output.
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
# It needs the corpus (`make fetch-cards compile-cards`) and builds dirac's tables.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

# Declared up front, BEFORE any early exit: the EXIT trap below reads these,
# and under `set -u` an undeclared variable on a build-failure or mid-farm
# exit turns the trap into `VAR: unbound variable`, whose failure status then
# overrides the script's own exit status.
SERVER_PIDS=()
PUBPORT="" OMNPORT="" SEATPORT="" FIXTUREPORT="" TALISPORT="" WIREGORGEPORT="" WIREDIRACPORT=""

# Every server binds 127.0.0.1:0 and the KERNEL picks the port; the assigned
# port is read back from the server's own startup line
# ("gorged: N tables of ... on 127.0.0.1:PORT (dir ...)"). The kernel only
# hands out a port that is free at bind time and the listener holds it, so
# there is no scan-then-bind window for a leftover server or a concurrent gate
# to race. The old fixed 8090-8099 probe guessed at free ports with `ss` and
# then bound them; when that range was held on a CI runner, every retry lost
# the same race and the gate failed on batches that had nothing to do with it.
#
# A :0 bind cannot fail on an address in use, so a server that does not come up
# is a real startup failure: its own log is printed instead of retrying a port
# guess. If the log names an address in use, list the holders with `ss -lptn`
# and stop the holder BY PID (never `pkill -f` / bare `pgrep -f`: the pattern
# matches your own shell and has killed a session here).

# port_listeners prints the ss -lptn lines listening on any of the given
# ports (nothing when none is). teardown_farm uses it to name a holder that
# survived teardown on a port this run actually used.
port_listeners() {
  local wanted=" $* "
  ss -lptn 2>/dev/null | awk -v wanted="$wanted" '
    NR > 1 {
      n = split($4, a, ":"); p = a[n]
      if (p ~ /^[0-9]+$/ && index(wanted, " " p " ") > 0) print
    }'
}

# reported_port prints the port gorged reported in its startup line, or
# nothing while that line is not in the log yet. It anchors on the trailing
# " (dir ...)" so the seated server's join-URL lines (also "gorged: ..."
# lines carrying a port) cannot match.
reported_port() {
  sed -n 's/^gorged: .* on 127\.0\.0\.1:\([0-9][0-9]*\) (dir .*/\1/p' "$1" 2>/dev/null | head -n1 || true
}

# wait_bound NAME LOG PID waits (bounded, 30 s) for one server to report its
# kernel-assigned port and prints it. A server that exits first is a hard
# failure whose log is shown here, with the `ss -lptn` hint, so the CI log
# names the cause instead of a retry storm.
wait_bound() {
  local name="$1" log="$2" pid="$3" port=""
  for _ in $(seq 1 120); do
    port=$(reported_port "$log")
    if [ -n "$port" ]; then printf '%s' "$port"; return 0; fi
    if ! kill -0 "$pid" 2>/dev/null; then
      echo "smoke: $name gorged (pid $pid) exited before reporting a bound port; its log follows:" >&2
      sed -n '1,60p' "$log" >&2 || true
      echo "smoke: the log above names the cause; if it says 'address already in use', list holders with 'ss -lptn' and stop the holder by pid (never pkill -f / bare pgrep -f)" >&2
      return 1
    fi
    sleep 0.25
  done
  echo "smoke: $name gorged (pid $pid) did not report a bound port within 30s; its log follows:" >&2
  sed -n '1,60p' "$log" >&2 || true
  return 1
}

# teardown_farm is the ONE teardown path for every recorded smoke pid: the
# EXIT trap (cleanup) calls it, so no second kill/rm block can drift from this
# one. Bounded-but-strict:
#   1. SIGTERM every recorded pid (never pkill -f / bare pgrep -f: the
#      pattern matches our own cmdline and has killed a session here).
#   2. Bounded wait (~5 s of 0.1 s polls). gorged's graceful path is
#      srv.Shutdown under a 5 s deadline then Registry.Close; measured
#      mid-match exit is ≤15 ms, so TERM must stay the FIRST signal — its
#      flush-on-exit writes the persistence dir.
#   3. SIGKILL every pid still alive after that (a sleeper with `trap '' TERM`
#      is the proof this step exists); bounded wait (~2 s) again.
#   4. LOUD stderr for any pid that survived SIGKILL, and for any smoke port
#      still LISTENING, so a residual leak names its culprit by pid.
#   5. Only after all of that, rm -rf the persistence dirs: a dir removed
#      under a live server is the half-flushed tables.json race.
teardown_farm() {
  local context="${1:-the smoke run}" pid alive ports
  for pid in "${SERVER_PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
  for _ in $(seq 1 50); do
    alive=0
    for pid in "${SERVER_PIDS[@]:-}"; do
      if kill -0 "$pid" 2>/dev/null; then alive=1; fi
    done
    [ "$alive" -eq 0 ] && break
    sleep 0.1
  done
  for pid in "${SERVER_PIDS[@]:-}"; do
    if kill -0 "$pid" 2>/dev/null; then
      echo "smoke: pid $pid still alive after SIGTERM; escalating to SIGKILL" >&2
      kill -9 "$pid" 2>/dev/null || true
    fi
  done
  for _ in $(seq 1 20); do
    alive=0
    for pid in "${SERVER_PIDS[@]:-}"; do
      if kill -0 "$pid" 2>/dev/null; then alive=1; fi
    done
    [ "$alive" -eq 0 ] && break
    sleep 0.1
  done
  for pid in "${SERVER_PIDS[@]:-}"; do
    if kill -0 "$pid" 2>/dev/null; then
      echo "smoke: LEAK: smoke pid $pid SURVIVED SIGKILL and is still running — kill it by pid; it holds its smoke port until it dies" >&2
    fi
  done
  ports=$(port_listeners "$PUBPORT" "$OMNPORT" "$SEATPORT" "$FIXTUREPORT" "$TALISPORT" "$WIREGORGEPORT" "$WIREDIRACPORT")
  if [ -n "$ports" ]; then
    echo "smoke: smoke ports still LISTENING after $context:" >&2
    printf '%s\n' "$ports" | sed 's/^/smoke:   /' >&2
    echo "smoke:   (a listener with no users:(...) detail belongs to another user; a listener pid that is NOT one of the farm pids above is a peer gate or another agent's server, not a leak from this run)" >&2
  fi
  rm -rf "${PUBDIR:-}" "${OMNDIR:-}" "${SEATDIR:-}" "${FIXTUREDIR:-}" "${TALISDIR:-}" "${WIREGORGEDIR:-}" "${WIREDIRACDIR:-}" "${VITE_CACHE_DIR:-}"
}

cleanup() {
  if [ "${#SERVER_PIDS[@]}" -gt 0 ]; then
    teardown_farm "the smoke run"
  else
    # No servers were recorded (a probe mode or a build failure): nothing to
    # kill and nothing to re-report — but the temp dirs the run created must
    # STILL go. A teardown that leaks on a failure path would keep a port busy.
    # The selftest sets every dir to "" so this is a silent no-op there.
    rm -rf "${PUBDIR:-}" "${OMNDIR:-}" "${SEATDIR:-}" "${FIXTUREDIR:-}" "${TALISDIR:-}" "${WIREGORGEDIR:-}" "${WIREDIRACDIR:-}" "${VITE_CACHE_DIR:-}"
  fi
}
trap cleanup EXIT

if [[ "${SMOKE_TEARDOWN_SELFTEST:-}" == 1 ]]; then
  # Regression gate for the teardown itself (no build, no browser): record a
  # sleeper that IGNORES SIGTERM in SERVER_PIDS, run the
  # real teardown helper, and fail loudly unless the escalation killed it.
  # Before the bounded-strict teardown this mode is the "fails without the
  # fix" witness: the old TERM-and-wait-only cleanup left the sleeper alive
  # after PASS/FAIL was already reported, which is exactly how a leaked farm
  # blocked every later run.
  echo "== smoke: teardown selftest =="
  PUBDIR="" OMNDIR="" SEATDIR="" FIXTUREDIR="" TALISDIR="" WIREGORGEDIR="" WIREDIRACDIR="" VITE_CACHE_DIR=""
  SERVER_PIDS=()
  ( trap '' TERM; exec sleep 30 ) &
  sleeper=$!
  SERVER_PIDS+=("$sleeper")
  if ! kill -0 "$sleeper" 2>/dev/null; then
    echo "smoke: SELFTEST FAIL: sleeper pid $sleeper was not alive before teardown (bad precondition)" >&2
    SERVER_PIDS=()
    exit 1
  fi
  echo "smoke: selftest sleeper pid $sleeper recorded (ignores SIGTERM, dies to SIGKILL)"
  teardown_farm "the teardown selftest"
  if kill -0 "$sleeper" 2>/dev/null; then
    echo "smoke: SELFTEST FAIL: sleeper pid $sleeper SURVIVED teardown — SIGKILL escalation missing or ineffective" >&2
    kill -9 "$sleeper" 2>/dev/null || true
    SERVER_PIDS=()
    exit 1
  fi
  echo "smoke: SELFTEST PASS: sleeper pid $sleeper survived SIGTERM and died to the SIGKILL escalation"
  SERVER_PIDS=()
  exit 0
fi

# vitest/playwright need Node >=24. A node on PATH serves if it is v24; otherwise
# take the nvm toolchain, so the target works from a plain shell with no shim loaded.
if ! node --version 2>/dev/null | grep -q '^v24'; then
  export PATH="$HOME/.nvm/versions/node/v24.15.0/bin:$PATH"
fi
if ! node --version 2>/dev/null | grep -q '^v24'; then
  echo "smoke: need Node 24 on PATH or at $HOME/.nvm/versions/node/v24.15.0/bin" >&2
  exit 1
fi

# Pin the browser cache in this shell too: the helper installs in its own
# process, and the launches below must resolve the same path even when the
# harness points XDG_CACHE_HOME at a per-run temp dir.
: "${PLAYWRIGHT_BROWSERS_PATH:=$HOME/.cache/ms-playwright}"
export PLAYWRIGHT_BROWSERS_PATH
echo "== smoke: ensuring Playwright Chromium =="
bash "$REPO_ROOT/scripts/ensure-playwright.sh"

# Vite writes a mutable cache; point it at our own scratch dir rather than
# through web/node_modules, so the build never touches the shared install.
export VITE_CACHE_DIR="${VITE_CACHE_DIR:-/tmp/gorge-smoke-vite-$$}"

echo "== smoke: building client =="
( cd web && npm run build )
# dirac's tables MUST be built BEFORE gorged: the binary embeds the pool
# ids.tsv bytes, and kgen rewrites a pool's ids.tsv (exit 3, then the make
# target rebuilds kgen and reruns) the first time it is built in a checkout
# that lacks one. Building gorged first leaves it expecting the OLD table key,
# and the wire-dirac server then exits at startup with "no card tables for
# this binary" — which the old farm loop reported as a port bind race.
echo "== smoke: building dirac's card tables =="
make dirac-tables
echo "== smoke: building gorged =="
mkdir -p bin
CGO_ENABLED=0 go build -trimpath -o bin/gorged ./cmd/gorged

# ---- start the farm; the kernel assigns each port ----
# Every gorged is launched with -addr 127.0.0.1:0 and reports the port the
# kernel gave it on its startup line (see the note at the top). This removes
# the scan window entirely: two gates on one box can never pick the same port,
# and a leftover server holding a fixed port cannot starve the gate at all.

PUBDIR="$(mktemp -d /tmp/gorge-smoke-public-XXXXXX)"
OMNDIR="$(mktemp -d /tmp/gorge-smoke-omni-XXXXXX)"
SEATDIR="$(mktemp -d /tmp/gorge-smoke-seat-XXXXXX)"
FIXTUREDIR="$(mktemp -d /tmp/gorge-smoke-board-XXXXXX)"
TALISDIR="$(mktemp -d /tmp/gorge-smoke-talisman-XXXXXX)"
WIREGORGEDIR="$(mktemp -d /tmp/gorge-smoke-wire-gorge-XXXXXX)"
WIREDIRACDIR="$(mktemp -d /tmp/gorge-smoke-wire-dirac-XXXXXX)"
SERVER_PIDS=()

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
  local dir="$1" spec="$2" log="$3"
  ./bin/gorged -addr "127.0.0.1:0" -dir "$dir" -spectator "$spec" \
    -decks testkit/testutil/decks -tables 4 -seats 4 -pace 1.5s >"$log" 2>&1 &
  SERVER_PIDS+=("$!")
}

# The SEATED server is a 1v1 play-vs-bot server: `-vsbot` arms
# POST /api/games (which returns a join URL seating the client at seat 0 of
# a fresh 2-seat table), and `-humans 1` additionally seats a real human at
# seat 1 of startup table t1 (the -vsbot flow always seats at seat 0, so the
# two together let the gate assert the 1v1 top/bottom mapping from EACH seat).
# `-seat-token` fixes the seat-1 token so the spec can build the join path
# without scraping stderr. `-tables 2 -seats 2` keeps it a real 1v1 table.
start_seated_server() {
  local dir="$1" log="$2"
  ./bin/gorged -addr "127.0.0.1:0" -dir "$dir" -spectator omniscient \
    -decks testkit/testutil/decks -tables 2 -seats 2 -pace 1.5s -seed 1 \
    -format constructed -vsbot -humans 1 -seat-token seatedfixture >"$log" 2>&1 &
  SERVER_PIDS+=("$!")
}

# A wire server offers ManaBrew and SpellBench v2 beside native, for a play-vs-bot game on either
# engine. Its one startup table of bots stops after a game, so the browser has the machine.
start_wire_server() {
  local dir="$1"
  shift
  ./bin/gorged -addr "127.0.0.1:0" -dir "$dir" -spectator omniscient \
    -tables 1 -seats 2 -perpetual=false -seed 1 -format constructed \
    -vsbot -manabrew -sbv2 "$@" >"$dir/server.log" 2>&1 &
  SERVER_PIDS+=("$!")
}

# start_farm starts all seven servers, each on its own kernel-assigned port,
# and records every pid in launch order: public, omniscient, seated, board
# fixture, talisman fixture, wire gorge, wire dirac.
start_farm() {
  SERVER_PIDS=()

  start_server "$PUBDIR" public  "$PUBDIR/server.log"
  start_server "$OMNDIR" omniscient "$OMNDIR/server.log"
  start_seated_server "$SEATDIR" "$SEATDIR/server.log"

  # The mana-wheel board: deterministic human hands with zero-cost Memnites plus the
  # Underground Sea that exercises the two-stage mana wheel. The tracked fixture
  # is a deck list (names/counts), never Forge card text.
  ./bin/gorged -addr "127.0.0.1:0" -dir "$FIXTUREDIR" -spectator omniscient \
    -decks web/e2e/fixtures/decks -tables 1 -seats 2 -pace 0 -seed 24 \
    -mulligans 0 -perpetual=false -humans 0,1 -seat-token boardfixture >"$FIXTUREDIR/server.log" 2>&1 &
  SERVER_PIDS+=("$!")

  # The two-stage Talisman continuation. The fixture gets a
  # Talisman of Indulgence castable by turn 2 (two Mountains for its {2} cost),
  # then stops at the priority window whose Talisman activation poses the
  # stage-1 ability wheel the browser test answers THROUGH the picker — the
  # path whose stage-2 colour ask must re-open the wheel at the card.
  ./bin/gorged -addr "127.0.0.1:0" -dir "$TALISDIR" -spectator omniscient \
    -decks web/e2e/fixtures/decks-talisman -tables 1 -seats 2 -pace 0 -seed 7 \
    -mulligans 0 -perpetual=false -humans 0,1 -seat-token talismanwheel >"$TALISDIR/server.log" 2>&1 &
  SERVER_PIDS+=("$!")

  start_wire_server "$WIREGORGEDIR" -decks testkit/testutil/decks
  start_wire_server "$WIREDIRACDIR" -engine dirac -dirac-sets fdn -decks dirac/testdata/bench/fdn
}

start_farm

# Read back the kernel-assigned port from each server's own startup line,
# in the same launch order. A server that dies first fails here, printing its
# log: with -addr :0 an in-use address is not a possible cause, so this is a
# real startup failure, not a race to retry.
FARM_NAMES=(public omniscient seated fixture talisman wire-gorge wire-dirac)
FARM_LOGS=("$PUBDIR/server.log" "$OMNDIR/server.log" "$SEATDIR/server.log" \
  "$FIXTUREDIR/server.log" "$TALISDIR/server.log" \
  "$WIREGORGEDIR/server.log" "$WIREDIRACDIR/server.log")
PORTS=()
for i in "${!SERVER_PIDS[@]}"; do
  port=$(wait_bound "${FARM_NAMES[$i]}" "${FARM_LOGS[$i]}" "${SERVER_PIDS[$i]}") || exit 1
  PORTS+=("$port")
done
PUBPORT="${PORTS[0]}"
OMNPORT="${PORTS[1]}"
SEATPORT="${PORTS[2]}"
FIXTUREPORT="${PORTS[3]}"
TALISPORT="${PORTS[4]}"
WIREGORGEPORT="${PORTS[5]}"
WIREDIRACPORT="${PORTS[6]}"

# Distinctness and liveness, after every port is known. A server that reported
# a port and then died frees that port for the kernel to hand to a LATER :0
# bind, which would silently drive one server for two fixtures — exactly the
# wrong-fixture failure the old scan was trying to prevent. Both holes are
# cheap to close here: every pid must be alive, and the seven ports distinct.
for i in "${!SERVER_PIDS[@]}"; do
  if ! kill -0 "${SERVER_PIDS[$i]}" 2>/dev/null; then
    echo "smoke: ${FARM_NAMES[$i]} gorged (pid ${SERVER_PIDS[$i]}) exited after reporting port ${PORTS[$i]}; its log follows:" >&2
    sed -n '1,60p' "${FARM_LOGS[$i]}" >&2 || true
    exit 1
  fi
done
dup=$(printf '%s\n' "${PORTS[@]}" | sort | uniq -d)
if [ -n "$dup" ]; then
  echo "smoke: two servers were assigned port $dup — a reported server died and the kernel reused its port; refusing to drive an aliased fixture" >&2
  exit 1
fi

echo "== smoke: gorged farm up (public :$PUBPORT, omniscient :$OMNPORT, seated :$SEATPORT, fixture :$FIXTUREPORT, talisman :$TALISPORT, wire gorge :$WIREGORGEPORT, wire dirac :$WIREDIRACPORT) =="
echo "smoke: farm pids: public:$PUBPORT=${SERVER_PIDS[0]} omniscient:$OMNPORT=${SERVER_PIDS[1]} seated:$SEATPORT=${SERVER_PIDS[2]} fixture:$FIXTUREPORT=${SERVER_PIDS[3]} talisman:$TALISPORT=${SERVER_PIDS[4]} wire-gorge:$WIREGORGEPORT=${SERVER_PIDS[5]} wire-dirac:$WIREDIRACPORT=${SERVER_PIDS[6]}" >&2
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
if ! wait_ready "$FIXTUREPORT"; then
  echo "smoke: board fixture gorged on :$FIXTUREPORT never became ready:" >&2
  sed -n '1,60p' "$FIXTUREDIR/server.log" >&2 || true
  exit 1
fi
if ! wait_ready "$TALISPORT"; then
  echo "smoke: talisman fixture gorged on :$TALISPORT never became ready:" >&2
  sed -n '1,60p' "$TALISDIR/server.log" >&2 || true
  exit 1
fi
if ! wait_ready "$WIREGORGEPORT"; then
  echo "smoke: gorge wire gorged on :$WIREGORGEPORT never became ready:" >&2
  sed -n '1,60p' "$WIREGORGEDIR/server.log" >&2 || true
  exit 1
fi
if ! wait_ready "$WIREDIRACPORT"; then
  echo "smoke: dirac wire gorged on :$WIREDIRACPORT never became ready:" >&2
  sed -n '1,60p' "$WIREDIRACDIR/server.log" >&2 || true
  exit 1
fi
echo "== smoke: driving the browser gate =="
set +e
( cd web && SMOKE_PUBLIC="http://127.0.0.1:$PUBPORT" SMOKE_OMNI="http://127.0.0.1:$OMNPORT" SMOKE_SEATED="http://127.0.0.1:$SEATPORT" SMOKE_FIXTURE="http://127.0.0.1:$FIXTUREPORT" SMOKE_WHEEL="http://127.0.0.1:$FIXTUREPORT" SMOKE_TALISMAN="http://127.0.0.1:$TALISPORT" SMOKE_WIRE_GORGE="http://127.0.0.1:$WIREGORGEPORT" SMOKE_WIRE_DIRAC="http://127.0.0.1:$WIREDIRACPORT" npx playwright test )
status=$?
set -e

if [ "$status" -eq 0 ]; then
  echo "== smoke: PASS (public :$PUBPORT, omniscient :$OMNPORT, seated :$SEATPORT, fixture :$FIXTUREPORT, talisman :$TALISPORT, wire gorge :$WIREGORGEPORT, wire dirac :$WIREDIRACPORT) =="
else
  echo "== smoke: FAIL (public :$PUBPORT, omniscient :$OMNPORT, seated :$SEATPORT, fixture :$FIXTUREPORT, talisman :$TALISPORT, wire gorge :$WIREGORGEPORT, wire dirac :$WIREDIRACPORT) =="
fi
exit "$status"
