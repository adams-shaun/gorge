# The autonomous defect pipeline

Watches for defects (player feedback, hand-authored tickets), turns each
into a ledger entry under `.ds4/issues/`, and drives it through triage,
implementation, review and deterministic gates to a merged, pushed,
deployed fix — with no Claude session and no human approval in the loop
by default.

## Why this exists

Every fix shipped in this repo's first day of feedback came from the same
loop, run by hand: notice a bug, write a brief, dispatch a local model,
review the diff, run the gates myself, merge, push, redeploy. That loop is
mechanical enough to automate — the judgement calls in it (is this diff
correct, do the gates pass) are exactly the parts a model or a subprocess
exit code can make, and the orchestration around them (which seat next,
when to escalate, when to give up and ask a human) is a small state
machine, not a reasoning task.

Nothing in this package is itself an LLM agent. `daemon.py` is a plain
Python loop; the actual thinking happens in a `pi-agent`-launched model
(local by default, escalated to a paid codex seat on repeated failure),
and every gate that decides whether code merges is a literal `go
build`/`go test`/`make sim` subprocess call, never a model's opinion.

## The pipeline

```
feedback/*/report.json  ─┐
.ds4/issues/inbox/*.md  ─┴─→  .ds4/issues/<id>.md   (new)
                                    │
                          triage (local model)
                                    ▼
                                 briefed
                                    │
                    create worktree, dispatch implementer
                                    ▼
                    dispatched  ──(local round 1, 2)──┐
                                    │                  │ fail twice
                                    │ DONE              ▼
                                    │            escalate to SOL
                                    ▼                  │
                          review (terra, always)  ◄────┘
                                    │
                       ┌─── APPROVE ┴─── REQUEST_CHANGES ──► back to dispatched
                       ▼                                      (with findings)
                 deterministic gates
              (build, tests, CR lane,
               TestHeads, make sim,
               web gates if touched)
                       │
              ┌─────── pass ┴─── fail ──► back to dispatched (with findings)
              ▼
   rebase, merge --no-ff, push,
   make deploy-demo, make ledger
                       │
                       ▼
                    merged
```

`human_needed` is the one state nothing above returns from automatically:
the escalation ladder (2 local rounds, then 2 SOL rounds) ran out, or a
seat reported `NEEDS_CONTEXT` (it's genuinely blocked on a decision only a
person can make), or a merge itself failed (a conflict the rebase step
couldn't resolve). Check `.ds4/issues/*.md` for that status; the `History`
section says exactly why.

## Files

- `config.py` — every path, model name, and threshold in one place.
- `issues.py` — the issue-file schema (`.ds4/issues/<id>.md`, Markdown with
  a flat frontmatter block) and its CRUD.
- `pi.py` — a thin wrapper around the `pi-agent` CLI: launch, detach, read
  `status.json`. No policy lives here.
- `seats.py` — builds the concrete brief/system/report file set for a
  triage, implementer, or review dispatch and calls `pi.launch`.
- `gates.py` — the deterministic gate runner. The only module that must
  never call a seat.
- `git_ops.py` — worktree lifecycle, rebase, merge, push, deploy, and the
  head-golden auto-update for an accepted chain-head move.
- `daemon.py` — the state machine. `tick()` runs one pass over every open
  issue; `main()` loops it on `config.POLL_SECONDS`.

## Running it

```sh
scripts/start-autonomous.sh          # start (idempotent) — also starts the demo if it's down
scripts/start-autonomous.sh status   # what's running
scripts/start-autonomous.sh stop     # stop the daemon (leaves the demo up)
```

The supervisor restarts the daemon and the demo server independently if
either dies, and polls every 30s. Logs:

- `.ds4/orchestrator/daemon.log` — the state machine's own decisions.
- `.ds4/orchestrator/supervisor.log` — start/stop/restart events.
- `.ds4/orchestrator/launches/<name>.log` — one pi-agent seat's raw stdout.

## Pause without stopping

```sh
touch .ds4/orchestrator/pause    # the daemon keeps polling but takes no dispatch/merge action
rm .ds4/orchestrator/pause       # resume
```

## Feeding it a ticket by hand

Drop a `.md` file in `.ds4/issues/inbox/`; its content becomes the report
and its filename stem becomes the issue id (`inbox-<stem>`). Picked up on
the next tick, same pipeline as a feedback report.

## Policy notes worth knowing before you trust it further

- **TestHeads auto-accept.** A moved chain head is accepted and the golden
  rewritten automatically iff the CR conformance lane is 0 FAIL and `make
  sim` is 20/20 replay OK — the same proxy this repo used by hand for every
  head move on its first day. If either isn't clean, the task is parked for
  a human, never merged on a guess.
- **One task, one worktree, no parallelism yet.** `daemon.tick()` walks
  every open issue every poll, but nothing here caps how many implementer
  seats run at once — the agent-offload skill's 4-local/2-codex ceiling
  isn't enforced. Fine at today's issue volume; will need a semaphore
  before it isn't.
- **Escalation is currently "try again," not "try differently."** A local
  round that fails gets the exact same brief plus findings; it doesn't get
  a smarter brief. If a task's local round keeps failing for the same
  reason, that's a signal the *brief* triage wrote is wrong, not just that
  the implementer is weak — worth an eyeball on `human_needed` parks before
  assuming the ladder just needs another rung.
