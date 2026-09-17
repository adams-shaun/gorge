# History-conditioned Search Probe Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans or superpowers:subagent-driven-development to implement task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Run the history-conditioned search probe on 500 games with GOMAXPROCS=5, retaining all roots and verifying sampled worlds before interpreting outcomes.

**Architecture:** An allowlisted history collector feeds a constrained sampler. A checked chance transcript drives hypothetical engines through ordinary rules execution. Prefix validation and world-local action mapping separate the sampler from the actual outcome harness.

**Tech Stack:** Go 1.25, standard library, existing gorge packages and pinned corpus.

**Spec:** `docs/superpowers/specs/2026-09-17-search-world-sampler-design.md`

## Global Constraints

- No commits, pushes, merges, rebases, PR changes, deployments, race tests, or golden regeneration.
- Preserve the existing hotspot-resume edit and pilot.
- Pure Go, no new dependencies; never track Forge/token source.
- All post-genesis game mutation remains through events.Apply.
- Ordinary games retain event bytes, ordinals, hashes, RNG draws and clone independence.
- GOMAXPROCS=5 and GOMEMLIMIT=5GiB for verification and experiments.
- Clocks measure performance only; fixed work counts govern decisions.

## Task 1: Replayable hypothetical chance

Files: `rules/chance.go`, `rules/chance_test.go`, `rules/rng.go`, `rules/engine.go`.

Interfaces: `ChanceDraw{Bound, Value int}`, `NewHypothetical(Config, []ChanceDraw) (*Engine,error)`, `AdvanceHypothetical() error`, `SubmitHypothetical(decision.Intent) error`, `ChanceTranscript() []ChanceDraw`.

- [x] Write tests using small land decks: forced toss/shuffle draws control genesis; invalid bounds and values return errors; tape+intents reproduce heads and draws; cloning separates transcript/source state; nil prefix retains seeded outcomes.
- [x] Run `GOMAXPROCS=5 GOMEMLIMIT=5GiB go test ./rules -run 'TestHypothetical' -count=1` and confirm missing-feature failures.
- [x] Implement an optional recorded stream in `rng.IntN`; unchanged normal path, copied tape/source/transcript on clone. Factor `New` through an internal constructor so the optional stream is installed before the toss. Catch only the typed chance failure at hypothetical entry points; unrelated panics propagate. Reject hypothetical submission on a normal engine.

```go
type ChanceDraw struct { Bound, Value int }
// At IntN(n), check the prefix bound, advance the independent PCG once,
// use the prefix value when supplied, then append ChanceDraw{Bound:n,
// Value:value} and increment Draws once. This preserves continuation on replay.
```

- [x] Run new tests plus `TestHeads` and clone regression tests, no golden writes.

## Task 2: Closed observation and action representation

Files: `internal/searchprobe/observation.go`, `observation_test.go`, `action.go`, `action_test.go`.

Interfaces: value-only `History`, `Frame`, `Action`; collector `Capture` receives a burst-end engine, actor and optional actor intent; `History` contains no Engine/Log/callback. Use observer identity mapping, not hidden arena layout.

- [x] Write tests that changing hidden shuffle payload, DecisionMade indices and in-memory resume fields cannot change the collected representation.
- [x] Capture public state and actor hand plus explicitly copied wire-level decision fields; normalize visible card identity at the observation point. Strip unpermitted event data before it enters History.
- [x] Add whole-prefix comparison and semantic action matching tests, including duplicate names, changed indices, ability/mode/alternative-cost mismatches and private vs public Notes.
- [x] Verify with `go test ./internal/searchprobe -run 'TestObservation|TestAction' -count=1` under the global environment.

```go
// Semantic actions are values. Translate to local pending indices only
// after matching the source's observed identity and all action parameters.
// Return an error for missing or ambiguous matches; never choose index zero.
```

## Task 3: Constrained permutations and weighted proposal pool

Files: `internal/searchprobe/permutation.go`, `permutation_test.go`, `weights.go`, `weights_test.go`.

- [x] Enumerate 3-card permutations with zero, one and two distinct positional constraints; assert allowed support and hand-derived likelihoods 1, 1/3 and 1/6.
- [x] Generate a uniform completion of positional assignments using an explicit RNG; translate the result to checked Fisher–Yates draws. Reject duplicate positions/copies and impossible bounds.
- [x] Test log-space normalization, all-zero weights, ESS for uniform vs concentrated weights, and deterministic weighted resampling including duplicates.
- [x] Verify these tests before integrating reconstruction.

```go
// For k specified distinct positions: log weight = lgamma(n-k+1)-lgamma(n+1).
// Unspecified positions are a uniformly shuffled copy-level remainder.
// General constraints with unknown proposal probability use prior rejection.
```

## Task 4: History-conditioned engine reconstruction

Files: `internal/searchprobe/sample.go`, `sample_test.go`, `reconstruct.go`, `reconstruct_test.go`.

- [x] Build tiny real-engine histories for opening draws, reveal/bounce, private look/reorder, search and subsequent shuffle; reject deliberately contradictory prefixes.
- [x] Replay actor actions as interventions. Sample independent opponent bot RNG; drive opponent choices solely from hypothetical seat views. Compare every allowed frame and reject on the first discrepancy.
- [x] Guide only constraints with a known proposal probability and retain ordinary chance sampling for other sites. Record the complete chance tape and hypothetical intents; validate selected-world replay from that artifact.
- [x] Run exactly the configured attempts, enforce submit caps, collect log weights, acceptance and failure categories, require positive-count/ESS gates, then resample four worlds.
- [x] Add noninterference tests, source independence and repeated-run equality. Distinguish unsupported from budget exhaustion; neither proves impossible history.

```go
// Each attempt is keyed by experiment seed, allowed-history digest and
// attempt ordinal. Never use original game seed, hidden hash or worker id.
// Weight is zero on prefix rejection; valid attempts use target/proposal.
```

## Task 5: Runnable action-comparison command

Files: `cmd/searchprobe/main.go`, `main_test.go`, `internal/searchprobe/rollout.go`, `rollout_test.go`, `score.go`, `score_test.go`.

- [x] Test seed allocation, exactly one root/game, eight-candidate cap with baseline/pass retention, missing-world baseline fallback and deterministic result order.
- [x] Reuse the established pilot protocol (not its true-state branching). Load explicit decks from the pinned corpus, collect all 500 roots and baseline replay evidence.
- [x] Freeze a documented static-only scorer and the turn-end leaf scorer with literal ranking tests. Compare current/static/one-world/four-world choices under matching candidates and continuation budgets.
- [x] Keep the actual engine only in the outcome harness; candidate selection receives History/worlds, never actual hidden state. Submit selected actions once and use paired terminal outcomes.
- [x] Emit JSON diagnostics, per-root errors/fallbacks, coverage, cost and paired differences. Return nonzero on invariant errors; label small runs calibration, never promotion evidence.

```sh
GOMAXPROCS=5 GOMEMLIMIT=5GiB go run ./cmd/searchprobe -games 500 -workers 5 -seed 10000 -attempts 64 -worlds 4 -out /tmp/gorge-searchprobe-500.json
```

## Task 6: Verification, review and handoff

- [x] Run focused tests, `go vet` for changed packages, replay/clone/seat parity tests and `git diff --check`.
- [x] Run the 500-game calibration, deterministic repeat and one-worker subset. Check every selected-world replay and all roots' outcomes, failures and fallbacks.
- [x] Review full task diff for secret leakage, probability mistakes, replay changes and dishonest metrics. Fix load-bearing findings before claiming completion.
- [x] Write the measured report and reproducible command. Record whether the 500-game action comparison actually ran and what blocks the original efficacy gate.

## Progress / decisions

- 2026-09-17: Architecture approved in chat; user requested “get it running”. Documents record that request, not completion.
- Work is in the user's existing `/tmp/gorge` feature checkout; do not create or switch branches implicitly. The collector, sampler and probe are tightly coupled and will be integrated in order.
- Previous turns produced design discussion but no executable reconstruction; the implementation progress below supersedes that starting status.
- Task 1 implemented; focused tests and existing heads/clone tests passed (2.366s). Review identified a missing effects-randomness test and ambiguous fallback wording. Added real Shuffle-spell replay/bound-mismatch coverage and full-tape continuation coverage (0.010s).
- Ruling: advance the independently seeded PCG even for forced prefix draws. Reconstructing a full transcript must restore the same future stream as the original hypothesis. The original “prefix or fallback” wording was ambiguous; no original-game seed is used. Cost if wrong: explicit research replay stream semantics need migration, not ordinary game goldens.
- Task 3 probability helpers implemented independently while Task 1 was reviewed: exact three-card distribution/tape enumeration, contradictory assignment rejection, log-space weighting/ESS and deterministic resampling tests passed (0.004s).
- Tasks 2–5 implemented: observer-local IDs and source/kind-qualified actions; full-prefix hypothetical replay; name-level genesis guidance with duplicate-copy likelihood; weighted resampling; frozen static and turn-end scorers; runnable `cmd/searchprobe`. Focused package tests and vet passed. Review found and tests reproduced two defects, both fixed: action context omission and aliasing of resampled duplicate worlds.
- Task 4 coverage includes real-engine synthetic shuffle, library search, private look/reorder, public reveal, battlefield-to-hand bounce and library return; each accepted world replays from its own chance tape and intents. The fixtures are synthetic, not claims of corpus coverage. Three-card name constraints pin the 1/2 toss * 2/3 duplicate-name likelihood independently.
- Ruling: guide only actor draws before the first non-draw library mutation (conservatively either library for MoveZone), plus the public toss. Everything else uses prior sampling and full-prefix rejection. This retains later-epoch roots but has low acceptance; do not reinterpret fallback as a sampled label.
- Ruling: identity-bearing Notes are allowlisted (plain reveal/look, library look, reveal-as-cost). Unknown identity semantics return a typed unsupported failure. Ordinary text-only public Notes are compared literally, never interpreted as hand membership. Empty Text with IDs is the engine's explicit reveal/hand-look format (`effects/cardflow.go`, `effects/look.go`), not an unknown message.
- Ruling: paired actual outcome branches restart the same independent continuation-policy RNG at the intervention. All four variants use that stream; the uninterrupted original current/current game is separately replay-verified. This avoids treating a variant's differing RNG consumption as if it were the original baseline continuation.
- Task 6 first 500-game run completed: 500 roots, 500 baseline replays, 2,000 terminal outcome replays, zero invariant errors. 17/500 roots passed the four-world/ESS gate; 483 fell back. 258 later-epoch roots were retained, including 6 of the 17 covered roots. Final verification/repeat and whole-change review remain in progress.
- The expanded first-rejection diagnostic exposed source-highlighted scalar modes: pay/decline share Kind/Obj, so mode labels must be part of Action even when Obj is nonzero. Added a failing regression, then retained labels specifically for mode options. Earlier 500-game artifacts predate this fix and are not final-code repeatability evidence.
- Tasks 1–6 complete. Final-code calibration: `/tmp/gorge-searchprobe-500-v2.json` (67.20s), repeat `/tmp/gorge-searchprobe-500-v2-repeat.json` (67.32s); all 500 non-timing per-game results equal. One-worker125 `/tmp/gorge-searchprobe-125-v2-serial.json` (78.64s, still GOMAXPROCS=5) equals the first125 five-worker results. Final sampler:164 accepted/32,000 attempts,17 covered roots,483 baseline fallbacks; no invariant errors. All500 baseline and2,000 paired outcomes replay, all outcomes terminal. Whole-change review and scalar-mode re-review resolved. Measured report: `docs/superpowers/reports/2026-09-17-search-probe-running.md`. This closes “get it running,” not the explicitly future strength/teacher gates.
- Publication ruling: after implementation and calibration, the user explicitly
  requested a checkpoint commit and push. That one-time request supersedes this
  plan's historical no-commit/no-push execution constraint only for publishing
  the verified checkpoint; it grants no merge, rebase, PR, deployment,
  golden-regeneration, race-test, or future-push authority.
