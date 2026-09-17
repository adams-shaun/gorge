# History-conditioned search probe design

## Authorization and outcome

The architecture was approved with “go”; “get it running” requests execution
of the sampling/validation proposal. Implement a runnable, non-production
search probe, not a trainer or live MCTS. Keep the full 500-game population
and `GOMAXPROCS=5`. A run with no usable sampled worlds is diagnostic evidence,
not a completed action-label comparison or strength result.

## Constraints

- No commits, pushes, merges, rebases, PR changes, deployments, race tests, or
  golden regeneration. Preserve the existing hotspot-resume edit and pilot.
- Pure Go, no new dependencies. Never serialize Forge/token source into tracked
  artifacts. All post-genesis game mutation remains through `events.Apply`.
- Ordinary Config-based games retain event bytes, ordinals, hashes, RNG draws,
  legal actions, and clone independence. Hypothetical games have separate logs.
- Experimental seeds are independent of the actual game seed. No wall-clock
  stopping decisions or order-dependent aggregation across workers.

These were implementation-session constraints. After implementation and
calibration, the user explicitly authorized a checkpoint commit and push. That
publication request does not broaden any engine, information, measurement,
merge, PR, deployment, race-test or golden-regeneration boundary.

## Components and information boundary

1. A collector records allowed seat observations at each actual burst boundary.
   It is the only component permitted to inspect the source engine. Its output
   contains public deck definitions, visible card identities/counts, the actor's
   own decision/answer history, and public action effects through the root.
2. The sampler accepts only that value representation. It cannot accept an
   actual Engine, Log, original Config.Seed, opponent-private intent, hidden
   hash, or callback into the source game. Copy only explicitly allowed decision
   fields; in-memory continuation fields are not observations.
3. A hypothetical constructor runs normal engine logic with independent,
   replayable chance outcomes. No fake present-day zone moves, state surgery,
   or replacement of Engine.G. Resume state and caches come from execution.
4. A prefix validator compares permitted observations at each boundary, not
   only the final board. World-local identities map to observed identities;
   action keys identify source, ability, mode, alternative cost, and targets.
   Duplicate copies are distinct objects. Raw option indices are never keys.
5. A probe evaluates the same candidate actions/worlds with fixed continuation
   budgets and keeps actual hidden state only in the external outcome harness.

The collector strips Shuffle and complete LibraryOrder payloads even for their
owner. Known order comes from legitimate look/arrange decisions and answers.
Strip DecisionMade's raw index text and private decision payloads. Public Notes
are interpreted by their supported reveal/look semantics, never all treated
as hand reveals. Unknown identity-bearing observation semantics return an
unsupported result; text-only public Notes are compared literally and imply no
hidden-zone membership. The supported identity channels are plain reveal/look
(the engine's empty-Text, IDs-carrying form), library look, and reveal-as-cost.
Public deck composition does not license guessing unseen object identities.

## Chance and replay

A hypothetical engine uses a checked sequence of (bound, value) chance
draws, followed by values from its independently seeded generator. Each request
also advances that generator, including requests overridden by the prefix: a
complete transcript can then reconstruct the same generator position and keep
producing the same continuation after replay. All chance draws are recorded.
An out-of-range value or bound mismatch fails explicitly. An exhausted *prefix*
uses the independent generator at its advanced position; exact replay requires the complete transcript
and matching draw count. The normal constructor does not enable recording.
Clones deep-copy mutable chance state and copy the generator's exact position.

The tape is a research replay artifact, not a claim that ordinary Config.Seed
alone can reproduce a conditioned game. Both initial shuffles and later
effects-driven random calls must consume the same abstraction. Sampling may
condition a shuffle by translating a valid permutation to Fisher–Yates draws;
it may not append an artificial shuffle at a different point in history.

## Belief model and guided proposals

The first model is specific to the frozen current bot. Chance outcomes are
uniform at their semantic chance sites; hypothetical opponent choices are made
by that policy with independent bot randomness and its own hypothetical view.
The actor's actual observed actions are interventions, not likelihood factors.
Condition on the entire allowed observation prefix. This is not a claim about
human opponents, optimal play, or an unbiased learned teacher.

Use guided proposals only where their probability can be computed. For example,
fixing k distinct positions of an n-card uniform permutation leaves (n-k)!
completions, and its importance factor is (n-k)!/n!. Work at distinct physical
copy level; account for multiple assignments when observations identify only
a name. Unsupported guidance falls back to prior sampling with prefix rejection,
not approximate assignments with an invented weight. All positive target paths
must retain proposal support. Accumulate weights in log space.

Retain reveals and visible returns as identity constraints. A shuffle erases
position knowledge, not already known library membership. Private searches,
looks and reorders belong only to the seat that saw them; opponent knowledge in
a hypothetical world is reconstructed, never copied. Later observations before
the root can constrain earlier chance outcomes. Nothing after the root can.

## Budgets, results, and cost reporting

Default calibration: 64 attempts per root, at most 5,000 submitted intents per
attempt, four output worlds. Require at least four positive proposals and
effective sample size `(sum w)^2/sum(w*w) >= 4` before seeded weighted resampling.
Report output duplicates. These starting budgets are proposals to measure, not
an established latency bound; changes must be recorded before strength runs.

Separate contradictory input (proved), unsupported semantics, exhausted work
budget, prefix rejection, and internal invariant failure. Exhaustion is not
proof that a history is impossible. No-sample roots remain in the denominator;
the eventual policy falls back to the baseline action and records the fallback.

Record attempts, submitted intents, acceptance, ESS, duplicate worlds, elapsed
time, allocation/memory measurements, prefix/replay failures and root coverage.
Timers are diagnostics only. Compare repeated runs with different worker counts
after removing timing fields. Never label true-state calibration as sampled
search, or successful execution as evidence of strength.

## Action probe and experimental boundary

Retain the pilot's population: current/current, death-n-taxes vs dimir-tempo,
seeds 10000–10499, no mulligans, first eligible turn>=5 root for seed%2, one
intervention per game. Keep baseline and pass, then fill in structured action
order to eight candidates; preserve baseline mana and land handling.

Compare current, a frozen static-only challenger, one-world shallow search,
and four-world root aggregation. Search continuations use observation-only
bots, not world-aware action maximization at every future node. Evaluate through
turn end (or terminal), with fixed submit caps and an explicitly frozen leaf
scorer. Match root actions in every world before rollouts. All variants use the
same candidate sets and continuation policy where applicable.

Paired actual outcome branches restart a common independent continuation-policy
RNG at the root, including the current-policy branch. The uninterrupted original
current/current game is separately replay-verified; its terminal head is not
mistaken for that paired branch's head.

The 500-game run is a correctness/coverage/cost calibration. It cannot replace
the original paired strength gate: N=4000, predeclared expansion to 9000 when
needed to resolve 3 percentage points, superiority over current and static-only
with a confidence interval excluding zero, and independent breadth confirmation
across the 66 explicit Legacy deck pairs before any teacher recommendation.
There is no trainer, model inference implementation, or production integration.

## Verification

- Test chance transcript validation, replay, clone divergence independence,
  normal RNG/heads parity, and effects-driven random calls.
- Exhaustively enumerate tiny decks/histories to check guided proposal weights
  against the target distribution, not just whether samples are possible.
- Test noninterference: identical allowed history and experiment seed yields
  identical output despite changing actual secrets, source RNG, private intent
  indices, and hidden-inclusive hashes.
- Test draws, public reveals, bounce, search, look/reorder, return, shuffle,
  duplicate copies, and public/private knowledge separation.
- Validate every accepted full prefix and root action set, then replay every
  selected world from its own construction artifact; test source immutability.
- Run 500 games with five workers and GOMAXPROCS=5, then a deterministic repeat
  and one-worker subset. Report every failure/fallback and make no strength claim
  until the separately specified efficacy gate actually runs.

## Implementation tracking

The companion plan records task completion and deviations. “Get it running” is
not satisfied by this document, a constructor alone, or a passing toy fixture.
