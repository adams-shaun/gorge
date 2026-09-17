# Fresh-session resume prompt

Paste the following into a fresh session with `/tmp/gorge` as the workspace.

---

Resume profiling-led optimization of gorge, the pure-Go Magic rules engine.
The objective is to loop and iterate on the largest measured hotspots, with
bot self-play and eventual MCTS/I-MCTS throughput as the motivation.

Start with a fresh brainstorming session for the next optimization. I will
approve the chosen design before implementation; no next-stage trigger index
or cost cache has been approved or implemented yet. Do not redo the first batch.

## Read first and establish current state

1. Read `AGENTS.md` and applicable skills.
2. Inspect git status, branch, HEAD and origin rather than assuming the state
   below is still current.
3. Read `docs/superpowers/reports/2026-09-17-hotspot-optimization.md` for the
   measured changes, methodology, caveats, verification and artifact locations.

The checkpoint branch is `perf/hotspot-optimization-2026-09-17`. Optimization
commit `e169cd2` was rebased cleanly onto fetched `origin/main` at `57867a7`.
The branch was kept local; no push or PR was performed. A later documentation
commit may be HEAD. Preserve any subsequent user changes. Do not commit,
push, merge into main or deploy without a new request.

## Already implemented

- Reject impossible granted-Ward events/targets before expensive discovery.
- Return trigger/replacement faces without per-object singleton allocations.
- Iterate filter alternatives and predicates lazily, preserving raw-name commas
  and fail-closed semantics.
- Reuse immutable normalizers in ParseCost and ProducedCounts.
- Cache pure Phase syntax, separately from replay-visible diagnostic state;
  clones own their writable cache.
- Use the existing reusable BoardSeat adapter in botbench, with View fallback
  and full-game intent/outcome parity tests.

On the pre-sync revision, repeated warm direct-Board comparisons reduced
allocated bytes/game by 82.5% (duel), 89.0% (four seats), and 90.8% (aggro).
All paired game summaries and final chain heads matched. Allocation object
counts fell 95.6–97.8%. These are allocated bytes, not retained memory.

The machine is very busy: expect variance. Do not present observed wall-time
ratios as portable speedups. Run benchmark workers one at a time with
`GOMAXPROCS=1`, fixed seeds/decks/toolchain, corpus load and warm-up excluded,
and separate allocation evidence from sampled CPU attribution. CPU affinity
does not make the machine idle.

## Next design: follow the largest remaining hotspot

Longer final-only profiles on the first batch found these inclusive CPU shares:

| Path | Duel, 250 games | Four seats, 50 games | Aggro, 150 games |
|---|---:|---:|---:|
| Trigger scan | 32.9% | 46.3% | 30.4% |
| Legal actions | 33.7% | 23.6% | 23.4% |
| Cost-static collection | 9.5% | 7.4% | 3.2% |
| Board projection | 8.6% | 5.5% | 16.4% |

These overlap; cost collection is inside legal-action generation. The wider
workload evidence moves conservative trigger-scan pruning ahead of cost caching.
However, upstream `57867a7` landed substantial sacrifice/unless-cost rules,
new Draw replacements and bot-policy changes after those profiles. Establish a
fresh baseline on the synchronized tree before treating the old percentages or
game outcomes as current. Do not mistake upstream game changes for regressions
from the optimization patch; compare against the matching origin revision.
Inspect `rules/trigger_match.go`, especially `checkFaceTriggers`,
`roomTriggerFaces`, `triggerMatches`, `zoneGate` and the look-back observer flow.
Compare conservative event-kind/immutable-face eligibility with maintaining
an object index; explain the invalidation and ordering tradeoffs before coding.

Do not simply skip bookkeeping events or hidden zones. `Always` state triggers,
once-per-spec Phase diagnostics, Room alternate faces, live versus look-back
matching, life-loss/damage batching and granted Ward all need their current
semantics and deterministic trigger order. Diagnostics are events too.

Smaller follow-ups: `costStatics` scans every zone three times per pricing
evaluation; remaining normalizer construction, especially `effects.effMana`,
still allocates; `staticEffects` and event-log growth are allocation targets.
If discussing cost caching, cache only ordered membership, never dynamic
amounts, targets, X or evaluated costs. Audit re-entry and clone ownership.
`handEngine` and `onBoard` mutate test setup without events after New; a durable
log-length-only cache can retain stale pre-fixture membership. Evaluate
call-scoped reuse as an alternative before changing fixtures.

For MCTS, the original probe found a different bottleneck: the first append
to a cloned log copies historical events. At one turn-11 position this cost
about 385 KB, versus 55 KB for Engine.Clone alone. A persistent-prefix design
needs separate approval and history-reader/hash/clone tests. It is not fixed.
No CUDA backend or GPU benchmark exists from this work; GPU neural inference
and GPU-resident rules simulation are distinct proposals.

## Verification and guardrails

Keep all state mutation through `events.Apply`, stable event ordinals and
ordering, exact replay hashes, no ambient randomness or event-visible map
iteration, no cgo/third-party core dependencies, and no committed Forge scripts.
The gitignored `.cards/` corpus exists at Makefile pin
`95f04e8a04c8925fa97cb226fc3341cabcc90a53`; use make targets if missing.

Use test-first regressions for each change, benchmark after each iteration,
and compare exact game results/heads. Run clone/race and Room/Ward/Phase/
look-back tests appropriate to the change, plus replay/golden-head checks.
The first batch already had independent review with no important findings.

The full pre-sync backend checks passed in changed packages and make sim
verified 20/20 replays. The full repository gates were not green: pre-existing
repro/feedback fixtures diverged at event 1, an overshoot fixture at event 76,
architecture checks rejected existing time/resume-writer cases, and web lint
had three existing errors. Baseline overlays independently reproduced the
replay failures. Do not regenerate goldens/fixtures or fix unrelated failures
just to claim green. Read the report's synchronization checkpoint for newer
post-rebase verification; revalidate if upstream has moved. The opt-in
conformance lane is deliberately known-red.

On the final synchronized tree, the updated host fixture instead fails at
event 802 (`move_zone` versus `choose`); exact unmodified origin `57867a7`
reproduces it. The old overshoot timeout tests no longer failed. Final
targeted rules, full effects/botbench/replay/seat/state/view checks, 20/20 sim
replays, Go vet and generated-type checks passed. The complete rules suite
was run at the first sync, not rerun in full after the second; the report
records the final rules-test selection. Repro/feedback and architecture
failures and the three web lint errors remain.

## Local evidence, if still present

- `/tmp/gorge-profile-deep.FZ9z7k/`: original diagnostic `main.go`, `go.mod`,
  investigation report and clone measurements.
- `/tmp/gorge-hotspots.pfkHCN/`: stage binaries/profiles, repeated comparison
  JSON, measurement scripts, baseline overlay and verification logs.
- Latest attribution: `final-{duel,four,aggro}-long` CPU/heap/JSON files.
- Repeated pairs: `repeat-*` and `isolated-*`; the latter name does NOT mean
  isolated hardware. `comparison.json` covers only the first series.
- Historical probe binaries use Go 1.26.4; native botbench A/B and ordinary
  tests used Go 1.25.11. Match toolchains for comparisons.
- `/tmp/gorge-sync-baseline.IenA7V/overlay.json` replaces all seven modified
  production files with exact origin `57867a7` copies. It established the
  final-sync fixture failure baseline; reassess its coverage before using it
  after further source changes. Final-sync verification logs are beside the
  earlier profiles, named `final-sync-*.log`.

These paths are temporary and may disappear. Historical binaries measure the
pre-sync first batch, not arbitrary future source. Rebuild a probe against the
current branch before attributing a new result to it. No old benchmark process
should be assumed live; inspect actual tool/process state.

Begin by summarizing the evidence, checking whether fresh profiling changes
the priority, and proposing the next design and its verification strategy.
Wait for my approval before implementing it.
