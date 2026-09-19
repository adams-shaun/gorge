# Botbench Decision Trace Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an opt-in, deterministic, privacy-safe, atomically published JSONL decision trace to `cmd/botbench`, then use it to establish the approved mono5 policy baseline.

**Architecture:** A game-local collector copies a strict `board-v1` projection immediately after policy choice and before submission. Game workers return trace buffers alongside outcomes; the existing ordered pair/game folds assemble them into a run-level document, validate every record, and atomically publish only after the whole run succeeds. Trace schema, projection, validation, JSONL writing, and diagnostics live outside the rules engine so tracing cannot mutate events or policy inputs.

**Tech Stack:** Go standard library only (`encoding/json`, `os`, `path/filepath`, `sort`); existing `botpolicy`, `decision`, `state`, and `cmd/botbench` APIs.

**Spec:** `docs/superpowers/specs/2026-09-18-botbench-decision-trace-design.md`

## Global Constraints

- Pure Go; no cgo and no third-party dependencies.
- Never commit Forge card scripts, `.cards/`, or captured decision traces.
- Trace collection is opt-in and observational: after policy choice, before `Engine.Submit`, with no policy RNG reads or second policy call.
- Workers never write trace files; output order is pair index, game index, decision sequence, then that game's terminal record.
- Do not change event kinds, engine state, replay inputs, chain heads, or goldens.
- Project only the deciding seat's own card/mana facts and public board/stack/commander facts; omit labels, prompts, source, continuation fields, rolls, compiled SAs, and opponent private zones.
- Development evaluation is all ten unordered mono5 pairs at seed 0 and 100 games per pair; held-out evaluation is seed 1,000,000 and 400 games per pair.

---

### Task 1: Versioned trace schema and immutable board projection

**Files:**
- Create: `cmd/botbench/trace.go`
- Create: `cmd/botbench/trace_test.go`

**Interfaces:**
- Consumes: `botpolicy.Board`, `decision.Decision`, `decision.Intent`, `gameOutcome`, `pairDef`.
- Produces: `traceRunV1`, `traceDecisionV1`, `traceGameV1`, `traceOptionV1`, `traceBoardV1`, `newGameTrace() *gameTrace`, `(*gameTrace).record(*decision.Decision, decision.Intent, *botpolicy.Board, traceDecisionMeta) error`, and `validateTraceRecord(any) error`.

- [ ] **Step 1: Write failing schema/projection tests**

  Build a hand-authored `botpolicy.Board` containing deliberately unsorted map keys, own card/mana facts, public creatures/life/commanders/stack, and a `decision.Decision` populated with every allowed option field plus every forbidden field (`Label`, `Source`, `SVar`, `Cost`, `Grant`, `Resume*`, `Rolls`). Assert marshalled JSON contains the declared v1 fields in sorted-ID slices and contains none of the forbidden strings or fields. Add a mutation test that refills/changes the source Board after `record` and asserts the recorded JSON is unchanged.

- [ ] **Step 2: Run tests and confirm RED**

  Run: `go test ./cmd/botbench -run 'TestTrace(BoardProjection|RedactsPrivateAndContinuationFields|SnapshotSurvivesBoardReuse|RejectsUnknownSchema)' -count=1`

  Expected: FAIL because trace types and projection do not exist.

- [ ] **Step 3: Implement the minimal schema and copying projection**

  Define explicit structs with `record_type` and `schema_version`, never embedding `decision.Option` or `botpolicy.Board`. Copy map-backed board data into slices sorted by numeric object/player ID; deep-copy keyword, mana-production, and commander-damage slices. Encode mana in fixed W/U/B/R/G/C order. Copy chosen indices in submitted order and legal options in engine-offered order. Validate record type/version, indices, ordering, and required identity fields.

- [ ] **Step 4: Run focused tests and confirm GREEN**

  Run: `go test ./cmd/botbench -run 'TestTrace(BoardProjection|RedactsPrivateAndContinuationFields|SnapshotSurvivesBoardReuse|RejectsUnknownSchema)' -count=1`

  Expected: PASS.

- [ ] **Step 5: Commit the schema milestone**

  Run: `git add cmd/botbench/trace.go cmd/botbench/trace_test.go && git commit -m 'feat(botbench): define decision trace schema'`

### Task 2: Game-local observation and deterministic worker folding

**Files:**
- Modify: `cmd/botbench/main.go`
- Modify: `cmd/botbench/main_test.go`
- Modify: `cmd/botbench/trace.go`
- Modify: `cmd/botbench/trace_test.go`

**Interfaces:**
- Consumes: Task 1's `gameTrace.record` and immutable decision records.
- Produces: trace-enabled game results returned through existing worker slots, with terminal records populated from final `gameOutcome`; a nil collector preserves the current path.

- [ ] **Step 1: Write failing observation/isolation tests**

  Add a tiny deterministic fake player and an engine-backed fixture proving: disabled collection allocates/records nothing; enabled and disabled games submit identical intents and produce identical event JSON, head, outcome, and verified replay; records are decision-sequence ordered and terminal data follows its game's decisions.

- [ ] **Step 2: Run tests and confirm RED**

  Run: `go test ./cmd/botbench -run 'TestTrace(DisabledIsNoOp|DoesNotChangeGameOrReplay|GameDecisionOrder)' -count=1`

  Expected: FAIL because the play path does not accept or return a game trace.

- [ ] **Step 3: Thread an optional game-local trace through the real play path**

  Extend the internal play result (not public engine APIs) to carry `*gameTrace`. In `playMatchOnce`, retain the current game-shaped Board long enough to call `record` once after `DecideBoard`/`Decide` and before `Submit`; for legacy/view seats, construct the same seat-private `botpolicy.Board` solely for tracing without calling the policy again. Attach outcome fields only after the game ends. Keep stats and action coverage independent.

- [ ] **Step 4: Fold traces from worker slots in stable order**

  Extend `gameResult` and pair results with per-game trace buffers. Preserve lowest-index error semantics and fold traces only after every submitted worker completes. Ensure pair coordinators write only to their own indexed result slot.

- [ ] **Step 5: Run isolation and concurrency tests**

  Run: `go test ./cmd/botbench -run 'TestTrace(DisabledIsNoOp|DoesNotChangeGameOrReplay|GameDecisionOrder|WorkerCountDeterminism)' -count=1`

  Expected: PASS, including byte-identical trace assembly at workers 1 and 8.

- [ ] **Step 6: Commit the collection milestone**

  Run: `git add cmd/botbench/main.go cmd/botbench/main_test.go cmd/botbench/trace.go cmd/botbench/trace_test.go && git commit -m 'feat(botbench): collect ordered decision traces'`

### Task 3: Atomic JSONL publication and CLI contract

**Files:**
- Modify: `cmd/botbench/main.go`
- Modify: `cmd/botbench/main_test.go`
- Modify: `cmd/botbench/trace.go`
- Modify: `cmd/botbench/trace_test.go`

**Interfaces:**
- Consumes: ordered per-game records from Task 2.
- Produces: `writeDecisionTrace(path string, run traceRunV1, games []gameTrace) error`, `-decision-trace <path>`, mono5 suite/split inference in the run header.

- [ ] **Step 1: Write failing filesystem and header tests**

  Cover empty path (no-op), existing destination, absent parent, injected write/validation/rename failures, cleanup of the temporary sibling, one run header only, exact pair manifest, actual flags/watchdogs/policies/format/seats/base seed/games per pair, `suite:"mono5"` only for the exact ten-pair manifest, and split values `development`/`heldout` only for the approved seed/game combinations.

- [ ] **Step 2: Run tests and confirm RED**

  Run: `go test ./cmd/botbench -run 'TestDecisionTrace(Atomic|RefusesExisting|RequiresParent|CleansUpOnFailure|Header|Mono5Split)' -count=1`

  Expected: FAIL because file publication and CLI wiring do not exist.

- [ ] **Step 3: Implement atomic validated JSONL output**

  Require an existing parent and non-existing destination. Create an exclusive temporary sibling, stream one compact JSON object per line through a validating encoder, `Sync`, close, and rename only after all games and records validate. On every failure, close/remove the temporary file and preserve the original run error. Do not create directories and do not overwrite a destination.

- [ ] **Step 4: Wire the flag without changing default output**

  Add `-decision-trace` to `main`, `mainExit`, and matrix execution. Reject tracing for unsupported non-matrix/grind shapes if the v1 header cannot truthfully describe them. Preserve byte-identical stdout and current allocation path when the flag is empty. Publish only after the matrix and replay/error gates succeed.

- [ ] **Step 5: Run focused and package tests**

  Run: `go test ./cmd/botbench -run 'TestDecisionTrace|TestTrace|TestMatrix' -count=1`

  Run: `go test ./cmd/botbench -count=1`

  Expected: PASS.

- [ ] **Step 6: Commit the atomic CLI milestone**

  Run: `git add cmd/botbench/main.go cmd/botbench/main_test.go cmd/botbench/trace.go cmd/botbench/trace_test.go && git commit -m 'feat(botbench): publish atomic JSONL decision traces'`

### Task 4: Diagnostic proxy report and mono5 baseline evidence

**Files:**
- Create: `cmd/botbench/trace_report.go`
- Create: `cmd/botbench/trace_report_test.go`
- Create: `docs/superpowers/reports/2026-09-18-bot-policy-baseline.md`
- Modify: `cmd/botbench/main.go`

**Interfaces:**
- Consumes: v1 JSONL files and rejects any unknown schema version.
- Produces: deterministic aggregates by policy, decision kind, pair, and starting seat: opportunities, offered counts, chosen kind/mode/position, singleton/width, terminal outcome/turn/stall association. All are labelled diagnostic proxies.

- [ ] **Step 1: Write failing analyzer tests**

  Feed a small hand-built JSONL fixture with two pairs, both policies, singleton and multi-option decisions, different chosen positions/modes, wins/draws/stalls, and starting seats. Assert exact sorted output and unknown-version rejection.

- [ ] **Step 2: Run tests and confirm RED**

  Run: `go test ./cmd/botbench -run 'TestTraceReport(AggregatesAndSorts|RejectsUnknownVersion|LabelsProxies)' -count=1`

  Expected: FAIL because the analyzer does not exist.

- [ ] **Step 3: Implement deterministic analysis**

  Decode line by line with `json.Decoder`, validate every version/type, associate each game's decisions with its terminal record, aggregate only named v1 measures, sort all output keys, and call signals “diagnostic proxies,” never regret.

- [ ] **Step 4: Run trace and full verification**

  Run: `go test ./cmd/botbench -count=1`

  Run: `go test ./... -count=1`

  Run: `go vet ./...`

  Run: `git diff --check`

  Expected: all ordinary suites PASS; do not run or reinterpret the known-red conformance lane.

- [ ] **Step 5: Capture raw baselines outside git**

  Run development candidate/control/legacy matrices using the exact pair string `mono-white-equipment:mono-blue-tempo,mono-white-equipment:mono-black-aggro,mono-white-equipment:mono-red-prowess,mono-white-equipment:mono-green-stompy,mono-blue-tempo:mono-black-aggro,mono-blue-tempo:mono-red-prowess,mono-blue-tempo:mono-green-stompy,mono-black-aggro:mono-red-prowess,mono-black-aggro:mono-green-stompy,mono-red-prowess:mono-green-stompy`, seed 0, 100 games per pair, `-seats 2`, existing watchdog defaults, JSON results under `/tmp`, and JSONL traces under `/tmp`. Run with workers 1 and the normal parallel budget once to prove trace-byte determinism. Run verified replay checks for the selected sample/engine path.

- [ ] **Step 6: Write the baseline report and choose one family**

  Record exact commands, commit SHA, pair rows with 95% intervals, pooled counts, starting-player split, stalls/errors, replay status, same-policy control, legacy comparison, and the trace diagnostics. Select the highest-frequency/high-consequence non-forced family supported by pair-level evidence; do not choose from pooled rate alone.

- [ ] **Step 7: Commit verified baseline evidence**

  Run: `git add cmd/botbench/trace_report.go cmd/botbench/trace_report_test.go docs/superpowers/reports/2026-09-18-bot-policy-baseline.md && git commit -m 'docs(botbench): record mono5 policy baseline'`

### Task 5: One narrow deterministic policy experiment

**Files:**
- Modify: exactly one policy-family file under `botpolicy/` selected by Task 4 (`cast.go`, `target.go`, or `combat.go`)
- Modify: the matching focused `botpolicy/*_test.go`
- Create: `docs/superpowers/reports/2026-09-18-bot-policy-experiment-1.md`

**Interfaces:**
- Consumes: Task 4 diagnostics and existing `botpolicy.Board` facts only.
- Produces: one deterministic heuristic with explicit stable option-index tie-breaks and no adapter changes unless separately parity-tested.

- [ ] **Step 1: State the experiment before coding**

  Add the observed family frequency, forced-choice share, pair associations, one causal hypothesis, exact proposed ranking, deterministic tie-break, and rejection threshold to the report.

- [ ] **Step 2: Write the smallest failing unit test**

  Construct two option lists that differ only in the feature the hypothesis ranks; assert the desired choice and a permutation/tie case that proves the stable tie-break. If a Board field is required, first add adapter-parity tests in both seat/game construction paths.

- [ ] **Step 3: Run the focused test and confirm RED**

  Run: `go test ./botpolicy -run '<exact new test names>' -count=1`

  Expected: FAIL on the old policy's choice.

- [ ] **Step 4: Implement the minimal heuristic**

  Change only the chosen family. Use no randomness, maps only through sorted keys when order can reach a choice, and option index as the final tie-break.

- [ ] **Step 5: Verify focused and adapter tests**

  Run: `go test ./botpolicy ./seat -count=1`

  Expected: PASS, including whole-game adapter parity and deterministic replay tests.

- [ ] **Step 6: Evaluate development seeds and retain or revert**

  Run candidate-vs-baseline, candidate-vs-legacy, and candidate-vs-candidate controls across all ten pairs at seed 0, 100 games per pair. Retain only if no stalls/errors/replay failures occur and pair-level evidence supports the hypothesis without material regressions hidden by pooling; otherwise revert only this experiment and document it as rejected.

- [ ] **Step 7: Gate a retained experiment on held-out seeds**

  Run the same three comparisons at seed 1,000,000, 400 games per pair. Report every pair's wins/rate/95% interval, pooled result, starting-player split, stalls/errors, replay status, and decision-family diagnostics. A pooled improvement alone is insufficient.

- [ ] **Step 8: Verify and commit a retained experiment**

  Run: `go test ./... -count=1`

  Run: `go vet ./...`

  Run: `git diff --check`

  If all pass and the held-out gate supports retention, run: `git add botpolicy docs/superpowers/reports/2026-09-18-bot-policy-experiment-1.md && git commit -m 'feat(botpolicy): improve <selected-family> decisions'`. If rejected, commit only the evidence report when coherent.

## Self-Review

- Spec coverage: Tasks 1-3 cover schema, privacy, snapshot lifetime, worker ordering, replay isolation, failure cleanup, atomicity, header manifest/suite/split, and default no-op behavior. Task 4 covers the specified diagnostic proxies and fixed baseline. Task 5 covers the required narrow TDD experiment and development/held-out gates.
- Placeholder scan: no TBD/TODO/fill-in implementation placeholders are present; the `<selected-family>` commit text and exact test-name shell token are execution-time values determined by measured Task 4 evidence, not missing implementation requirements.
- Type consistency: `gameTrace` owns decision records and a terminal record; it travels only through internal worker result slots. `writeDecisionTrace` consumes those ordered buffers. The analyzer consumes the same v1 records the writer emits.
