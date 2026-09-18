# Compiled face metadata completed checkpoint

Resume in `/tmp/gorge` on branch
`perf/hotspot-optimization-2026-09-17`.

## Completed work

The approved compiled `cards.Face` metadata plan is implemented through Task
8. The implementation HEAD before the report-only checkpoint was `dea13a5`.
The retained commits are:

```text
1b703c0 fix(events): preserve activation count across arena growth
4352c4a test: benchmark face metadata baseline
41e1c41 feat: define compiled card metadata codes
a70ba6f feat: compile flat face metadata catalog
ffd3961 feat: bind compiled metadata during corpus load
6524e52 perf: use compiled face query metadata
63767ef perf: use compiled trigger interest masks
dea13a5 perf: dispatch compiled effect APIs by opcode
```

The catalog uses one-based IDs, sorted source-map and token-key traversal, and
conservative unknown fallbacks. Runtime IDs do not enter events or protocols.
Face queries preserve textual fallback and pointer/order equality, trigger
interests are conservative prefilters, and the effect registry atomically
publishes its name and opcode views.

Read the final evidence in
`docs/superpowers/reports/2026-09-17-compiled-face-metadata-running.md` before
starting new work.

## Verification checkpoint

Focused tests, vet, and whitespace checks passed:

```sh
go test ./cards ./effects ./rules ./state ./internal/searchprobe ./cmd/searchprobe -count=1
go vet ./...
git diff --check
```

The full suite is not green because of failures reproduced at the `1b703c0`
control: botbench deck ordering/golden expectations, gorged deck listing, five
host overshoot-tail fixtures, the stall-guard timeout, and the architecture
resume-writer allowlist. Do not alter host behavior or regenerate goldens to
hide them.

The original pre-task search artifact is not a valid exact oracle after the
required `AbilityPush` replay correction. The preserved post-fix control and
compiled results compare exactly equal after removing only timing/memory
fields: 500 games, 498 eligible roots, two no-root games, 100 covered roots,
and zero errors.

```text
/tmp/gorge-searchprobe-post-ability-fix-500-20260918.json
/tmp/gorge-searchprobe-post-ability-fix-cpu-500-20260918.pprof
/tmp/gorge-searchprobe-post-ability-fix-heap-500-20260918.pprof
/tmp/gorge-searchprobe-post-ability-fix-bin-20260918
/tmp/gorge-compiled-face-500-20260918.json
/tmp/gorge-compiled-face-cpu-500-20260918.pprof
/tmp/gorge-compiled-face-heap-500-20260918.pprof
/tmp/gorge-compiled-face-bin-20260918
```

End-to-end performance is neutral: wall time improved 0.6%, profiled CPU rose
0.35%, runtime allocation rose 0.12%, and sampled allocation space rose 0.04%.
The focused type, keyword, ability, and trigger paths improved without new
steady-state allocations; registry load and retained heap increased.

## Next task: design predicate and cost compilation

Do not rerun the completed face-metadata plan. Start a new design and approval
cycle for the next immutable compilation boundary. Candidate scope is
selector/filter predicates, condition expressions, cost tokens, and mana
feasibility, prioritized by a fresh profile.

The design must preserve these constraints:

- Pure Go only; no cgo or third-party core dependencies.
- Never commit Forge scripts or `.cards` content.
- Keep textual IR as the authoritative diagnostic and fallback form.
- Compiled predicates use `yes`, `no`, and `maybe`; only `no` may eliminate a
  candidate, while `maybe` runs the textual oracle.
- Sort all source maps before assigning IDs or emitting bytes.
- Runtime catalog IDs never enter events, replays, decisions, or existing
  protocols.
- Preserve face-local `TriggerPush.Amount` and `AbilityPush.Amount` ordinals.
- Preserve all mutation through `events.Apply`, deterministic ordering,
  replay bytes, decisions, RNG, and clone independence.
- Benchmark filters and cost/mana feasibility before migrating a consumer.
- Use the post-`AbilityPush` 500-game control above as the semantic oracle.
- Do not run race tests, regenerate goldens, push, merge, rebase, or create a
  PR without fresh authorization.

Before editing, inspect HEAD, upstream, worktree status, and the complete diff.
Use `superpowers:brainstorming` for the new design, then write and approve a
spec before producing an implementation plan.
