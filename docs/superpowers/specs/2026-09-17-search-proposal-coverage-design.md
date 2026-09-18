# History-Conditioned Proposal Coverage Design

## Outcome

Improve the runnable search probe's world coverage without weakening its
information boundary or full-prefix validation. The sampler will condition
library permutations at genesis and later shuffles on facts the acting seat
actually observed, including public cards opponents later play from hand and
the actor's ordered library-arrangement windows.

This remains a calibration probe, not production MCTS, a trainer, or evidence
that a search policy is stronger. Every root remains in the population and a
root without four positive proposals and ESS >= 4 still uses the baseline.

## Why this design

The fixed 500-root run accepted 164 of 32,000 proposals. A diagnostic rerun
showed direct opponent hand-exit identity mismatches in 81.2% of rejected
Death-n-taxes-actor attempts and 87.3% of rejected Dimir-actor attempts. The
dominant transitions were hand-to-battlefield land plays, hand-to-stack spell
casts, and public hand-to-exile alternate costs. Most remaining early event
mismatches are different consequences of different opponent holdings.

MageZero does not provide a hidden-information solution to copy. Its current
MCTS clones the actual XMage game state, its default configuration enables
opponent-hand visibility, and its documentation explicitly acknowledges leaked
hands, future draws, and random outcomes. Its unused `shuffleUnknowns` helper
directly swaps hand/library contents and is followed by a TODO for true
stochastic MCTS. Gorge must instead sample histories consistent with the
observer's information.

## Information and mutation boundaries

- Constraint compilation consumes only `PublicGame` and the owned `History`
  representation. It never receives the source engine, original seed, hidden
  zones/hash, raw decision indices, or opponent-private intent details.
- A proposal hook may inspect the current *hypothetical* world's library and
  hand. Those values never enter actor decisions or emitted diagnostics.
- The proposal changes only chance outcomes at real shuffle calls. Resulting
  library state is still installed by the ordinary Shuffle event through
  `events.Apply`; there is no state surgery or artificial zone movement.
- Ordinary games and `NewHypothetical` without a proposal hook remain byte-,
  RNG-, head-, and clone-compatible.
- A selected world is replayed without the hook from Config, its complete
  checked chance transcript, and its intent log.

## Observable constraint model

Compile history into player-specific library epochs. Epoch zero begins at that
player's genesis shuffle; every later Shuffle event begins another epoch.

Within an epoch the compiler may emit only these constraints:

1. An actor-visible Draw fixes the next shuffled position to the observed card
   name. If the card was already public before entering the library, its known
   observer identity fixes the physical hypothetical object instead.
2. An actor-visible `KArrange` decision fixes the ordered top-card window shown
   by its options. The recorded answer remains an intervention; `LibraryOrder`
   is never treated as random.
3. The first public appearance of an opponent-owned card moving from Hand to a
   public zone adds a cumulative name-count deadline. By the number of draws
   observed for that player at that point, enough copies of that name must have
   appeared in the opening/drawn prefix. Replays of an already-known bounced
   object add no new hidden-hand requirement.
4. A later shuffle resets positional tracking and creates a new independently
   conditionable epoch. At that shuffle, matching cards already present in the
   hypothetical opponent hand satisfy public-play requirements first; only the
   remaining deficit constrains post-shuffle draws.

Draw positions are tracked only while their relationship to the epoch's
shuffle is exact. An unmodelled library mutation, opponent-private arrangement,
or decision whose library ordering cannot be inferred marks subsequent
constraints unguided until the next Shuffle. Those histories remain in the
population and continue through prior sampling plus full-prefix rejection.

## Exact proposal distribution

At a guided shuffle with `n` distinct physical cards, let `C` be the set of
physical permutations satisfying all supported fixed-position and cumulative
deadline constraints after accounting for the hypothetical hand. The target
chance model is uniform over all `n!` permutations. The proposal is uniform
over `C`, so its importance factor is:

```text
target / proposal = |C| / n!
```

The sampler computes `|C|` exactly with memoized dynamic programming over the
relevant prefix positions and remaining physical copy counts. Sampling walks
the same DP: a candidate physical card is selected in proportion to the number
of valid completions beneath it. Once all constraints are discharged, the
remaining distinct physical cards are shuffled uniformly.

Observed requirements with the same card name are cumulative counts rather
than labeled copies. This avoids overcounting permutations when identical card
definitions can satisfy interchangeable observations. Physical object IDs
remain distinct inside the DP and output permutation. Exact known-object
constraints are never collapsed by name.

Log factors accumulate over every guided shuffle. An unguided shuffle uses the
ordinary prior and contributes zero. A constraint set with no compatible
permutation rejects that proposal with zero weight; a contradiction provable
from public deck composition before sampling is a typed contradictory-input
failure. No approximate count receives an invented weight.

## Hypothetical shuffle boundary

Add an opt-in planner to the hypothetical engine. Each real library shuffle,
including genesis, mulligan, search, and Shuffle effects, passes a context with
player, per-player shuffle ordinal, and owned descriptions of the hypothetical
hand/library cards. The planner either returns nil for ordinary prior sampling
or a complete desired physical permutation.

The engine validates that the result is exactly a permutation of the current
library, translates it to Fisher-Yates values, advances the independent PCG for
every forced value, and records the values in the normal chance transcript.
Planner errors poison only that hypothetical attempt through the existing typed
chance-failure boundary. The planner is detached at the root so search
continuations use the ordinary hypothetical RNG.

The effects package routes library randomization through its Host rather than
mutating state. Test hosts retain a deterministic fallback. The ordinary host
path remains the same Fisher-Yates loop and emits the same Shuffle event.

## Determinism and diagnostics

Engine seed, proposal choices, opponent policy RNGs, and final weighted
resampling use separate deterministic streams derived from the experiment
seed, history digest, attempt ordinal, player, and shuffle ordinal. Worker
identity and wall-clock values never enter those streams.

Replace the single first-rejection-only diagnostic with a bounded sorted
histogram. A bucket contains frame index, comparison component, and a normalized
shape such as `hand_to_battlefield`, `hand_to_stack`, `hand_to_exile`,
`identity_other`, `event_kind`, `board`, or `action`. It contains no hidden card
identity. The histogram is deterministic and does not affect proposal control
flow.

Also report guided genesis/later shuffles, actor arrange windows, unguided epoch
constraints, compatible-permutation failures, positive proposals, ESS,
duplicates, submissions, and fallback causes.

## Failure behavior

- Invalid public configuration or internal accounting mismatch: invariant
  error; the command fails.
- Publicly contradictory constraints: typed contradiction; sampling stops for
  the root and baseline fallback remains explicit.
- No compatible completion for one hypothetical past: reject that attempt,
  not the whole history.
- Unsupported epoch shape: use the prior for that epoch and record it.
- Prefix mismatch after proposal: reject normally and record its histogram
  bucket.
- Submit-budget exhaustion: preserve the distinct budget category.

Full-prefix observation comparison remains the final correctness oracle.

## Verification and calibration

- Exhaustively enumerate tiny physical decks with duplicate names and verify
  proposal support, uniformity over `C`, and `|C|/n!` weights for exact slots,
  cumulative deadlines, and their combination.
- Test contradictory deadlines and exact-object/name conflicts.
- Test a real-engine opponent public land/spell history that fails under the
  prior but reconstructs under the guide.
- Test a later Shuffle followed by actor draws, and an actor-visible arrange
  window followed by its recorded reorder and draw.
- Replay every selected world from Config + full chance transcript + intents;
  verify duplicate resamples own independent mutable engines.
- Repeat noninterference tests while changing source secrets, source RNG,
  private intent indices, and hidden-inclusive hashes.
- Verify ordinary heads, RNG draws, clones, repo-deck replay, and architecture
  import rules. Do not run race tests or regenerate goldens.
- Run the fixed 500-game calibration and a one-worker deterministic subset with
  `GOMAXPROCS=5 GOMEMLIMIT=5GiB`. Compare coverage by actor and later epoch,
  acceptance, ESS, duplicate worlds, rejection classes, time, and allocations
  to the 17/500 checkpoint. Do not interpret outcomes as strength evidence.

## Scope limit

This work improves construction of information-consistent root worlds. It does
not add a trainer, learned belief model, opponent-policy inference, live MCTS,
or production integration. It does not force observed opponent choices; they
remain generated by the frozen policy in each hypothetical view.
