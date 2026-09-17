# Fresh-session resume prompt: return to the MCTS training question

Paste the following into a fresh session with `/tmp/gorge` as the workspace.

---

Return to the strategic question for gorge: **should the next bot milestone use
MCTS / information-set MCTS to create a stronger training signal, and if so,
what is the smallest credible experiment?**

Do not begin with another general engine-hotspot pass. The latest three
optimizations produced only a 10.4-18.4% reduction in profiled elapsed time.
That is useful incremental work, not a transformational answer to search or
training economics. Establish the search/training cost model directly before
proposing more optimization.

Do not commit, push, rebase, merge, create or edit a PR, regenerate goldens, or
deploy without a new explicit request. Preserve user changes. Do not run race
tests unless the user changes that instruction.

## Establish current state

1. Read `AGENTS.md` and applicable skills.
2. Inspect status, branch, HEAD, upstream, origin, and merge-base rather than
   assuming the checkpoint below is still current.
3. Read these sources before answering:
   - `docs/superpowers/specs/2026-09-06-gorge-learned-policy-research.md`
   - `docs/superpowers/reports/2026-09-17-hotspot-optimization.md`
   - `docs/superpowers/reports/2026-09-17-trigger-pruning.md`
   - `docs/superpowers/specs/2026-09-03-mtgcore-go-engine-design.md`
   - the current `cmd/botbench`, `cmd/mtgsim`, `botpolicy`, `seat`, clone, log,
     projection, and legal-action paths
4. Treat all old measurements as leads. Reproduce any number that materially
   affects the recommendation on the current checkout and label estimates as
   estimates.

Published checkpoint on 2026-09-17:

- Branch: `perf/hotspot-optimization-2026-09-17`
- Remote HEAD: `1cfef874b83d41a1229dc067bacc8d920c82d451`
- PR: https://github.com/adams-shaun/gorge/pull/1
- Base checkpoint: `origin/main` at
  `8bce166943e3d09cce81acad4797a6106630ad5c`
- Corpus pin: `95f04e8a04c8925fa97cb226fc3341cabcc90a53`
- No merge or deployment was performed.
- This resume document may be the only local modification after that push.

Relevant commits already on the PR:

```text
1cfef87 docs: refresh hotspot optimization handoff
420f7df perf: reduce board and static query overhead
79b2121 test: share corpus and parallelize rules suite
db84d5c perf: reuse cost statics during legal action scans
abe0f71 perf: prune impossible printed-trigger scans
a617c4b perf: remove rules hot-path allocation overhead
```

## The question to answer

The phrase "MCTS training" is ambiguous. Separate these proposals explicitly:

1. **Inference-time search:** MCTS/PUCT chooses live actions.
2. **Search as teacher:** determinized or information-set search generates
   improved policy/value targets offline; deployment uses only the trained
   network.
3. **Search-policy iteration:** an AlphaZero-like loop repeatedly generates
   targets with the current network plus search and promotes gated successors.
4. **No-search learning:** direct self-play PPO/self-distillation on the
   offered decision options.
5. **Non-learned gate:** improve and measure spell selection first, proving
   there is exploitable strength above the current zone-order heuristic.

The deliverable is a recommendation among these paths, not an implementation.
Lead with the answer and the decisive numbers. If the evidence is insufficient,
specify one bounded experiment that would decide it, its expected runtime, and
its pass/fail threshold. Wait for approval before writing code.

## Evidence already available

The learned-policy research reached a strong prior conclusion, but it predates
the latest engine work and must be challenged rather than repeated blindly:

- Gorge has no strong teacher. The current bot beat the legacy coin-flip
  blocker by only 53.6% at N=4000.
- Spell selection was the largest measured sensitive surface: replacing its
  choice with uniform random cost 16.8 percentage points. The size of the
  improvement available *above* the current heuristic remains unknown.
- About 64.8% of priority decisions had no sensible choice beyond pass, so
  search/training should bypass them.
- An old full playout from a mid-game state measured about 14.4 ms.
- Old 1-ply candidate expansion measured roughly 212-536 us/candidate and
  made a game 9-21x more expensive. The crude search bot won only 16.5-22.5%
  against the heuristic. It copied hidden zones and therefore was not a valid
  information-set agent; treat it only as evidence that naive leaf evaluation
  and shallow horizons fail.
- Previous MCTS/PUCT work cited by the research was 30-50x slower and weaker
  than its heuristic teacher. Search-generated training data from a weaker
  search also failed. Determine whether gorge changes that conclusion.
- The first mutation after a turn-11 engine clone previously allocated about
  385 KB because the cloned log's first append copied historical capacity;
  `Engine.Clone` alone was about 55 KB. The latest slices did not address this.
- A persistent immutable log prefix plus branch-local suffix is a plausible
  search-specific optimization, but it changes history-reader, hash, clone,
  and replay semantics and needs a separate design.
- Rules execution is branch-heavy Go traversal and is not a CUDA-shaped
  workload. Batched policy/value inference is the plausible GPU boundary;
  GPU-resident rules simulation would be a separate engine/backend.

Latest three-slice A/B results, initial baseline to current candidate:

| workload | profiled elapsed | allocated bytes | allocation objects |
|---|---:|---:|---:|
| duel, 250 games | 12.104s -> 10.840s (-10.4%) | 1.193GB -> 1.052GB (-11.8%) | 15.26M -> 14.36M (-5.9%) |
| four-seat, 50 games | 7.551s -> 6.555s (-13.2%) | 631.6MB -> 574.4MB (-9.0%) | 7.87M -> 7.52M (-4.5%) |
| aggro, 150 games | 8.146s -> 6.650s (-18.4%) | 895.2MB -> 676.2MB (-24.5%) | 8.65M -> 7.92M (-8.4%) |

Those elapsed values came from single-core **profiled** runs on a shared host.
They imply only rough observed rates of 20.7 -> 23.1 duel games/s/core,
6.6 -> 7.6 four-seat games/s/core, and 18.4 -> 22.6 aggro games/s/core. Do not
quote those as production throughput. Build an unprofiled, current-tree
throughput measurement for the MCTS calculation.

The rules test suite was also accelerated independently: the measured full
suite went from 322.99s to 22.63s by sharing the corpus and using native test
parallelism. That improves engineering iteration time, not games/s.

## Required analysis

Build a simple cost model around **meaningful branch decisions**, not whole
games alone. Measure or tightly bound:

- unprofiled games/s/core for representative duel, four-seat, and aggro games;
- meaningful decisions/game after forced priority decisions are removed;
- candidates/decision and the long tail;
- engine clone cost, first branch mutation cost, and incremental subsequent
  branch steps at early-, mid-, and late-game snapshots;
- rollout cost to useful horizons: next same-seat priority, end of step, end of
  turn, and terminal;
- memory per live node and the practical parallel-search ceiling;
- batched policy/value inference cost separately from rules simulation;
- corpus-generation cost for a proposed search budget and training generation.

For hidden information, define the algorithm precisely. `Engine.Clone` copies
the actual hidden state, so ordinary perfect-information MCTS cheats. Compare
at least:

- root determinization with opponent-consistent resampling;
- information-set aggregation across determinizations;
- re-determinizing IS-MCTS if opponent nodes otherwise act on knowledge they
  should not have.

State what information a node key contains, how actions are matched across
determinizations, how stochastic outcomes are seeded, and how the design avoids
strategy fusion and event-visible nondeterminism. Include multiplayer concerns
rather than assuming a two-player zero-sum game if four-seat training is in
scope.

## Decision gates

Do not recommend a multi-week trainer merely because it is feasible. Require a
cheap gate first. A credible recommendation should name:

- the exact first decision surface, preferably spell/ability selection at
  priority rather than all nine decision kinds;
- the baseline opponent and deck matrix;
- N and confidence interval discipline (the prior recommendation was N=4000
  for the headline pair and all 66 unordered repo-deck pairs for breadth);
- compute budget per decision and per training generation;
- promotion threshold and worst-matchup guardrail;
- an ablation proving any gain comes from search, rather than a better static
  evaluator or more favorable determinizations;
- a stop condition that prevents months of training a weaker teacher.

The default smallest experiment to evaluate is: build no trainer, implement no
full MCTS, and use a throwaway, correctness-checked search probe over recorded
mid-game snapshots. Compare a static spell scorer, shallow determinized search,
and information-set aggregation under identical candidate and compute budgets.
Bench each against the current policy across enough games to resolve a 3pp
effect. If even the search oracle cannot produce stronger action labels, stop.

## Constraints

- Preserve all deterministic engine semantics, event ordinals, replay hashes,
  LKI/APNAP behavior, hidden-information boundaries, and clone independence.
- All state mutation remains through `events.Apply`.
- No cgo or third-party dependencies in the card pipeline and rules core.
- Never commit Forge scripts or token text.
- Do not infer GPU acceleration from CPU profile percentages.
- Separate measured values from projections and training-scale extrapolation.
- Do not oversell incremental hotspot improvements as an MCTS breakthrough.
- No production implementation until the user approves a written design.

Start with a concise answer to this question:

> Given the current engine, is search best used at inference time, as an
> offline teacher, inside an iterative training loop, or not yet at all—and
> what single experiment most cheaply falsifies your recommendation?
