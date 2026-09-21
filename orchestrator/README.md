# orchestrator — gate hooks for the agentctl pipeline

This package is no longer a pipeline. It holds the repo-specific gate hooks
that the pipeline calls by name, and nothing else.

The autonomous defect pipeline that used to live here — `daemon.py`,
`config.py`, `gates.py`, `git_ops.py`, `issues.py`, `pi.py`, `seats.py` — was
superseded by [agentctl](https://github.com/adams-shaun/agentctl)
(`~/projects/agentctl`) in the 2026-09-19 cutover and removed from main on
2026-09-21. Its last state is preserved on the `park/orchestrator-pre-agentctl`
branch. Nothing runs from that branch; read it for history, not for behaviour.

## What runs today

- The daemon is `python3 -m agentctl run <repo>`, launched and supervised by
  `scripts/start-autonomous.sh`, running against the pinned copy at
  `~/.agentctl/pins/current` — **not** against `~/projects/agentctl` directly.
  A change to agentctl does not take effect until the pin is updated and the
  daemon restarted.
- Per-repo configuration — seats, caps, gates, landing, intake — lives in
  `.agentctl/config.toml` at the repo root. agentctl's
  `tests/test_consumer_parity.py` pins the translation from the old
  `config.py`/`gates.py` values to that file.
- Queue and issue state stay where they were: `.ds4/issues/`,
  `.ds4/orchestrator/journal.jsonl`.
- The dashboard is http://127.0.0.1:8765/.

## What is still here

`hooks.py` — the three gate hooks `.agentctl/config.toml` names:

| Hook | Used by |
|---|---|
| `cr_no_fail_lines` | the CR conformance gate's `accept` |
| `sim_all_replayed` | the `make sim` gate's `accept` |
| `testheads_policy` | `[hooks] post_gates` |

agentctl imports these as `orchestrator.hooks` with the repo root on the path,
so **this module must stay self-contained**: standard library only, no imports
from anything else in this package, no assumption that any other module here
exists. That constraint is why hooks.py survived the removal intact.

Adding a hook means adding a function here and naming it from
`.agentctl/config.toml`. Changing pipeline *behaviour* means changing agentctl,
re-pinning, and restarting the daemon.
