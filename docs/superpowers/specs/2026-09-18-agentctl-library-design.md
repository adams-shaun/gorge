# agentctl — combining the pi-agent dispatcher and the autonomous orchestrator into a reusable library

Status: **approved direction, awaiting implementation** (strangler extraction, approach B — user-approved 2026-09-18)
Parents: `~/projects/ds4-harness` (pi-agent dispatcher, `bin/pi-agent`, `bin/ds4-dash`, `ds4/` package) and `~/projects/gorge/orchestrator` (the autonomous defect pipeline daemon)
New repo: `~/projects/agentctl` — Python ≥ 3.11, standard library only, no runtime deps.

## 1. Why

Two tools have grown up side by side and duplicate each other's plumbing:

- **ds4-harness** dispatches local/paid model seats through the `pi` coding agent
  (`bin/pi-agent`: bubblewrap jail, private `~/.pi` copy, STATUS.json contract) and
  serves a fleet dashboard (`bin/ds4-dash`, port 8765, watch-registry in `~/.ds4/roots`).
- **gorge/orchestrator** is a defect-pipeline state machine (triage → dispatched →
  review → deterministic gates → merge → deploy) that drives those seats via the
  dispatch wrapper (`pi.py`), an issue ledger (`issues.py`), gates (`gates.py`),
  git mechanics (`git_ops.py`), a policy config (`config.py`) and CLI steering
  (`directive.py`, the `pause` file).

Both are gorge-shaped today: paths, seat rosters, gate commands and policies are
baked into module constants. Goal: extract the reusable core into `agentctl` so
another project can run the same pipeline with its own config, and add the
steering the operator asked for — **a login on the agents dashboard and unlocked
interactions**: stop all work now; rebase all worktrees / inject a
"rebase-first" directive; reprioritize ticket X; expose per-launch agent config;
and a harness seam for true RPC/prompt-injection into live sessions later.

## 2. Non-goals (first cut)

- No multi-user auth / RBAC. Single operator: one shared token, session cookie.
- No WAN exposure: everything stays bound to 127.0.0.1; login exists so the dash
  can be reverse-proxied later without redesign.
- No new parallelism features. The known "no semaphore" limitation in the gorge
  README stays backlog (the box-wide seat caps from `pi.running_names` carry over).
- No true in-session RPC injection in the first cut (see §4 — the interface leaves
  room; the pi implementation is kill+relaunch-with-message).
- No daemon migration before the steering features land: the gorge daemon keeps
  shipping fixes on the current code until slice 5.

## 3. Library layout

```
agentctl/
  agentctl/
    harness/
      base.py         # Harness protocol: launch / status / liveness / steer / kill
      pi_harness.py   # pi implementation — bin/pi-agent's mechanics in Python
      seat_env.py     # seat roster + provider/env resolution (from ds4/seat.py)
    ledger/
      issues.py       # .ds4/issues/<id>.md schema + CRUD (from orchestrator/issues.py)
      priority.py     # queue ordering (from daemon's priority logic + its test)
    pipeline/
      config.py       # PipelineConfig dataclass; consumers build one
      gates.py        # gate RUNNER only; gate definitions come from the consumer
      gitops.py       # worktree lifecycle, rebase, merge, push — no repo policy
      daemon.py       # the tick() loop, parameterized by PipelineConfig
    control/
      server.py       # stdlib HTTP control server the daemon runs alongside
      auth.py         # token file -> signed session cookie; middleware
      actions.py      # steering verbs as pure state operations
    dash/
      server.py       # the evolved ds4-dash: login, buttons, polls control API
  tests/
  README.md
```

**Boundary rules (inherited, load-bearing):**

1. `pipeline.gates` never calls a harness — every gate is a literal subprocess
   whose exit code is ground truth.
2. `control` never talks to a harness directly. Steering only writes state the
   daemon's next tick reads (issue files, directive blocks, override config,
   pause/stop files). The daemon is the single writer, so every steering verb
   goes through the same code path the daemon itself uses — no new write
   semantics to audit, nothing that bypasses the state machine.
3. The dashboard reads via the control/dash API and writes only through
   `control.actions` — the UI never mutates state files itself.

## 4. Harness layer (slice 1)

`Harness` protocol (one implementation now, one later):

```
launch(spec: SeatSpec) -> SeatHandle     # spec: name, cwd, brief/system/report/out
                                         # paths, provider, model, thinking, max_minutes
status(handle) -> Status | None          # STATUS.json reader (DONE/BLOCKED/CAPPED/…)
liveness(handle) -> float | None         # seconds since transcript last grew
running_names(models, repo=None) -> set  # /proc scan, box-wide or repo-scoped
steer(handle, message) -> SteerResult    # FIRST CUT: kill + relaunch with --message
kill(handle) -> None
already_launched(name) -> bool           # the launch-log marker (same-name relaunch
                                         #   must never kill a live credential store)
```

- `pi_harness.py` ports `bin/pi-agent` bash to Python, byte-for-byte in behaviour:
  the bwrap jail (worktree + git common dir + Go caches writable, / read-only,
  /tmp private), the private `~/.pi` copy per run, `--mode json --no-extensions
  --no-skills`, the events.jsonl + session transcript, the STATUS.json writer
  (status/commits/tests/tokens/steps/tool_errors), the monitor registry hook.
  The bash `bin/pi-agent` stays on disk and untouched until the gorge daemon
  itself is migrated (slice 5), then becomes a thin alias.
- `seat_env.py` is `ds4/seat.py` generalized: `~/.ds4/local-seat.env` resolution,
  `is-local`, auth preflight, the paid-seat warning.
- **RPC seam.** No mainstream harness offers clean mid-session RPC; the closest
  real path is Claude Code's `--input-format stream-json` (push a user message
  into an ongoing session over stdin). A future `claude_stream.py` harness
  implements `steer()` as true injection; pi keeps kill+relaunch. This is a
  spike AFTER the steering features land, never a slice-1 dependency.

## 5. Ledger layer (slice 2)

- `issues.py` moves over as-is: flat-frontmatter Markdown at
  `<repo>/.ds4/issues/<id>.md`, History log, status vocabulary
  (`new/briefed/dispatched/waiting/review/human_needed/merged`).
- `priority.py`: the queue ordering + `test_priority_order.py` semantics.
- Ingestion (gorge's `feedback/*/report.json` watcher, the `.ds4/issues/inbox/`
  drop) stays consumer-side: it is gorge-specific.
- New, small: `set_priority(issue_id, n)` — the write the steering layer uses.

## 6. Pipeline layer (slices 3+5)

- `PipelineConfig` dataclass: repo root, dirs, seat roster (local provider/model,
  escalated/reviewer seats, fallbacks), caps (`MAX_LOCAL_SEATS`,
  `MAX_PAID_SEATS`), ladder (`MAX_LOCAL_ROUNDS`, `MAX_ESCALATED_ROUNDS`),
  `MAX_MINUTES`, poll seconds, gate list (ordered `Gate` callables), merge/deploy
  hooks, and the policy flags gorge owns today (TestHeads auto-accept and friends).
- `gates.py`: the runner from `orchestrator/gates.py` (`GateResult`, timeouts,
  output-tail capture) — takes the gate list from config; knows nothing about Go.
- `gitops.py`: from `orchestrator/git_ops.py` — `create_worktree`,
  `create_triage_worktree`, rebase, `merge --no-ff`, push, deploy hook. Repo
  specifics (symlinking `.cards`/`.superpowers`, the ledger copy, node_modules
  link) become config-provided "worktree prep" callables.
- `daemon.py`: the `tick()` state machine, one pass per poll, identical status
  vocabulary and history format; reads the override config (§7) fresh each tick.

## 7. Control plane — auth + steering (slice 4) ← the operator-facing deliverable

**Auth (`control/auth.py`):** token file `~/.agentctl/control-token` (generated
on first run, mode 0600, root/user-only). `POST /login` exchanges the token for
an HMAC-signed session cookie (stdlib `hmac` + `secrets`, expiry ~12h). Every
endpoint except `GET /login` (the form) and `POST /login` requires the cookie.
The dash's read API moves behind login too — the operator asked for a login on
the dashboard, and read access leaks transcripts. Default bind stays
127.0.0.1:8790 (env `AGENTCTL_CONTROL_PORT`); the existing fleet dash stays on
8765 and is migrated onto the new dash module with auth in the same slice.

**Steering verbs (`control/actions.py`) — the unlocked interactions:**

| Verb | Endpoint | Mechanism | Notes |
|---|---|---|---|
| Pause all daemons | `POST /fleet/pause` | write `~/.agentctl/stop` + every registered repo's `pause` file | graceful: no new dispatches, in-flight rounds finish |
| **Stop all work now** | `POST /fleet/halt` | pause as above **+ `SIGTERM` every live seat** (the `/proc` scan; confirm button) | the "stop everything" switch |
| Resume | `POST /fleet/resume` | remove the files | |
| Rebase all worktrees | `POST /repo/<root>/rebase-all` | `gitops.rebase_all_worktrees`: fetch + rebase each `.worktrees/<id>` onto its base | conflicts: rebase aborted, worktree flagged on the dash for the existing merge-resolver path — never force |
| Rebase-first directive | `POST /repo/<root>/directive` (preset) | inject the CONTROLLER DIRECTIVE block ("rebase before continuing; the ledger at `<repo>/.ds4/ledger.json` is the queue") into matching briefs | `directive.py`'s mechanism: patches `issue.brief` + live `<worktree>/.ds4/brief.md`; an in-flight TURN cannot see it (documented; use kill/relaunch for that) |
| Arbitrary directive | same endpoint, body = text | same mechanism, `--issue`/`--status` selectors as today | |
| Reprioritize ticket X | `POST /repo/<root>/priority` `{issue_id, priority}` | `ledger.set_priority` | picked up next tick by the existing ordering |
| Launch config | `GET/POST /repo/<root>/launch-config` | per-repo override file `<repo>/.ds4/orchestrator/overrides.json`: provider, model, thinking, max-minutes, per-kind caps | daemon re-reads each tick (precedence: overrides > consumer defaults); no restart; UI form with the current effective values |

The control server is started by the daemon (or standalone for a repo whose
daemon is down — steering a stopped pipeline must still work: it writes state
the daemon reads on next start).

## 8. Consumers after migration

- **gorge/orchestrator** shrinks to: feedback ingestion, gate definitions
  (go build / go test core / CR conformance / TestHeads policy / make sim / web
  gates), brief templates, deploy + ledger hooks, and a `PipelineConfig`.
  The stale `orchestrator/dashboard.py` reference (dead `.pyc`, docstring in
  `daemon.py`) is resolved — the dash is `agentctl.dash`.
- **ds4-harness** keeps `agent.py`/`tools.py`/`review.py` (SDD specifics) and
  consumes `seat_env` + the dash; `bin/ds4-dash` becomes a wrapper.

## 9. Slice order & acceptance (each slice leaves the gorge daemon green)

1. **Harness** — port pi-agent + seat env; tests ported
   (`test_running_names_repo.py` and the STATUS-contract cases); gorge
   `orchestrator/pi.py` re-exports from agentctl; `make`-green gorge suite.
2. **Ledger** — issues + priority + ported tests; gorge re-exports.
3. **Gates + gitops** — runner parameterized; ported `test_merge_resolver.py`,
   `test_daemon_lock.py`, `test_redispatch_stale_marker.py`; gorge re-exports.
4. **Control + dash** — auth, steering verbs, dashboard buttons. **First
   operator-visible milestone.** Verify: login wall on 8765/8790, each verb
   exercised against a paused test repo, halt kills a seeded fake seat process.
5. **Daemon migration** — gorge/orchestrator/daemon.py replaced by
   `agentctl.pipeline.daemon` + gorge `PipelineConfig`; gorge's own tests plus a
   live soak (feedback → merged fix) before the old daemon code is deleted.

Rollback: every slice is a re-export in the consumer until slice 5; reverting a
slice is a revert commit in gorge, and the bash `bin/pi-agent` remains available
throughout.

## 10. Risks

- **Same-name relaunch** kills a live credential store — `already_launched`
  round-specific naming carries over verbatim, and slice 4's halt/resume verbs
  are built on the same `/proc` scan so they cannot relaunch anything.
- **Halt is sharp**: SIGTERM mid-write can corrupt a STATUS.json — the status
  reader already tolerates malformed files (returns None), and a killed seat's
  issue parks in `dispatched` with a dead-dispatch finding, the existing path.
- **Overrides file races**: the daemon reads it once per tick; two racing POSTs
  write via tempfile+rename — last write wins, single operator, acceptable.
- **The live demo deploys on every gorge merge** — docs-only commits are safe;
  code commits during slices 1–3 are re-exports covered by the gorge suite.
