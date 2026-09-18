# Compiled face metadata execution resume

Resume work in `/tmp/gorge` on branch
`perf/hotspot-optimization-2026-09-17`.

## Objective

Execute the approved compiled `cards.Face` metadata plan. Build a deterministic
registry-owned flat catalog, migrate selected CPU hot paths to masks/opcodes,
and preserve textual IR and exact replay/search behavior. The catalog is the
immutable half of a possible future CUDA boundary; do not add CUDA, cgo,
plugins, or an external worker in this task.

Read in order:

1. `AGENTS.md`
2. `docs/superpowers/specs/2026-09-17-compiled-face-metadata-design.md`
3. `docs/superpowers/plans/2026-09-17-compiled-face-metadata.md`
4. `docs/superpowers/reports/2026-09-17-engine-profile-optimization-resume.md`
5. this resume

Use `superpowers:using-git-worktrees` as required by the execution workflow,
but detect the environment first. This branch may already be the intended
workspace; do not create an isolated worktree that omits required local state.
Then use `superpowers:executing-plans`, or
`superpowers:subagent-driven-development` if agent-per-task execution is
explicitly selected. Follow the plan in order with test-driven development.

## Checkpoint

The architectural design and previous engine optimization work were committed
and pushed on 2026-09-17. Confirm actual HEAD and upstream before editing; do
not assume a recorded hash remains current after this resume is committed.

The worktree should be clean. If it is not, inspect and preserve every existing
change. Do not reset, overwrite, or absorb unrelated work.

## Non-negotiable constraints

- Pure Go only; no cgo or third-party core dependencies.
- Never commit Forge card scripts or anything under `.cards`.
- All game-state mutation continues through `events.Apply`.
- Sort source maps before assigning IDs or emitting catalog bytes.
- Runtime catalog IDs never enter events, replays, decisions, or existing
  external protocols.
- `TriggerPush.Amount` and `AbilityPush.Amount` remain face-local ordinals.
- Unknown syntax retains text and uses a conservative fallback; it never
  aliases a known opcode or becomes a false negative.
- Do not regenerate goldens or run race tests without fresh authorization.

## Baseline evidence

The corpus census is 33,669 registry cards, 839 token scripts, 35,385 faces,
and 54,671 linked SA nodes. Loading the 8.6 MB gob cache measured approximately
0.87 seconds and retained approximately 93 MB / 1.28 million heap objects in a
one-process probe.

The immutable end-to-end baseline is:

```text
/tmp/gorge-searchprobe-engine-final-500-20260917.json
/tmp/gorge-searchprobe-engine-final-bin-20260917
/tmp/gorge-searchprobe-engine-final-cpu-500-20260917.pprof
/tmp/gorge-searchprobe-engine-final-heap-500-20260917.pprof
```

It contains 500 games, 499 eligible roots, one no-root game, 114 covered roots,
and zero errors. Its measured totals were 69.463003930 seconds wall, 338.66 CPU
seconds, 49,603,764,312 runtime-allocated bytes, 46.36 GiB sampled allocation
space, and 317,072,050 sampled allocated objects.

The relevant pre-catalog costs include `checkFaceTriggers.func1` at 22.06 CPU-s
flat / 59.32 s cumulative, `Object.Face` at approximately 14.5 s,
`forEachObject` at 72.73 s cumulative, and small-string map lookup at 6.46 s
flat / 13.29 s cumulative. `BenchmarkFaceTriggerScanDistinctFaces` scans 240
faces in approximately 3.2–4.5 microseconds with zero allocations.

## Execution rules

Complete one plan task at a time. For every behavior change, write the failing
test first, observe the expected failure, implement the smallest passing
change, run the focused suite, measure the named benchmark, and commit the
task. Do not claim a performance win from a microbenchmark alone; the final
500-game run is the semantic and end-to-end gate.

The full suite has known baseline failures:

- `cmd/botbench.TestConstructedDefaultIsByteIdentical`: 18/2 versus 16/4
- `host.TestStallGuardSetToZeroDoesNotHalt`: timing failure
- an occasional asynchronous host undo-stream assertion; focused reruns have
  passed 20/20

Do not change host behavior or regenerate goldens to hide these failures.

The task-level commits in the plan are authorized. Do not push implementation
commits without fresh user authorization in the execution session.
