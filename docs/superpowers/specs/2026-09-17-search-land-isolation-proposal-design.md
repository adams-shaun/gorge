# Search Probe Land-Isolation Proposal Design

## Outcome

Reduce the largest remaining full-prefix rejection class by biasing sampled
opponent hands toward the observed ordinary land play while preserving the
existing target distribution, frozen-policy contract, and full-prefix oracle.
This is a bounded extension of the history-conditioned proposal, not a new
belief model, a forced opponent action, production search, or strength
evidence.

The fixed 500-root calibration recorded 11,048
`identities/hand_to_battlefield` rejections across 375 roots. It was the
largest residual bucket and the largest bucket within 198 roots; 201 affected
roots accepted no proposals. Deterministic representative probes for both
actors, before and after later shuffle epochs, showed the same leading shape:
the observed land's positive deadline was compiled and guided, but the frozen
policy publicly played a differently named card at the same transition.
Same-public-card observer-reference novelty did not explain a leading
hand-exit failure in those probes.

## Scope

This proposal applies only to an opponent's ordinary land play that is
observable as a Hand-to-Battlefield move paired with `LandPlayed`. It does not
cover spells, abilities, effects that put a nonland permanent directly onto
the battlefield, actor actions, or epochs whose relevant hand accounting is
unsupported. Those paths retain the existing proposal.

The proposal consumes only:

- the owned `History` frames, including public event shapes and identities;
- public deck definitions already present in `PublicGame`; and
- the hypothetical hand and library supplied to the existing shuffle planner.

It receives no source engine, original seed, hidden source zones or hash,
opponent-private intent index, or callback into the source game. Diagnostics
remain normalized and contain no card names or hypothetical object IDs.

## Land-isolation constraints

The current constraint set for one shuffle is `C`. It contains the existing
fixed-position and cumulative lower-deadline constraints.

For each supported observed opponent land play in that epoch, let:

- `d` be the number of cards drawn from that epoch before the play;
- `y` be the publicly observed played land name;
- `exit(x, d)` be the number of cards named `x` already observed leaving that
  player's hand earlier in the epoch; and
- `hand0(x)` be the number of cards named `x` in the hypothetical hand at the
  shuffle boundary.

For every public-deck land name `x != y`, add this cumulative upper bound to
the isolation component:

```text
drawn(x, d) <= exit(x, d) - hand0(x)
```

This states that no differently named shuffled land remains in hand when the
observed land is played. A negative right-hand side makes the isolation set
empty for that hypothetical shuffle. Earlier observed exits permit exactly
the copies already consumed. Existing lower deadlines still require the
observed land names by their public deadlines.

Applying the rule at every supported land play in an epoch handles successive
land drops without labeling physical copies. The restriction is intentionally
stronger than the actual policy condition: a different land may have been in
the real hand but ranked lower. This proposal does not claim otherwise; the
mixture's unrestricted component preserves those histories.

The compiler treats a land play as supported only when its frame contains one
unambiguous, non-secret Hand-to-Battlefield move and the matching `LandPlayed`
event for that observed object. Hand accounting remains reliable from a
shuffle through a candidate play only while every intervening membership
change is either a draw or a visible, named hand exit. Any card entering the
hand after that shuffle, any exit whose name is not visible, or any bulk hand
mutation marks later land plays in that epoch ineligible. Earlier supported
plays in the epoch remain usable. Ineligible plays add no upper bounds and use
`C` unchanged. Existing library reliability and unguided-epoch rules remain
authoritative.

## Exact mixture distribution

Let `L` be the subset of `C` satisfying the land-isolation upper bounds for the
current hypothetical hand. Physical permutations remain the sample space.

When `L` is nonempty, select one of two proposal components with a fixed
probability of one half:

1. sample uniformly from `C`;
2. sample uniformly from `L`.

For a produced physical permutation `x`, the exact proposal density is:

```text
q(x) = 1/(2|C|) + I[x in L]/(2|L|)
```

The target shuffle density is `1/n!`, so the shuffle's importance factor is:

```text
(1/n!) / q(x)
```

Its logarithm is accumulated with the factors from every other guided
shuffle. Membership in both `C` and `L` is evaluated directly from the
sampled permutation. Component selection uses the proposal RNG stream, never
the engine RNG or worker identity.

When `L` is empty or the epoch is ineligible, the effective proposal is
uniform over `C`, with the existing factor `|C|/n!`; no half-probability is
discarded. When `C` is empty, the attempt remains an incompatible completion.
This preserves positive proposal support for every permutation currently
supported by the exact proposal.

## Counting and duplicate copies

Extend the constrained-permutation counter with cumulative upper deadlines.
The memoized state and unranking path must enforce lower and upper bounds at
the same prefix boundaries and return the exact physical-permutation count.
Tiny-deck exhaustive enumeration remains the correctness oracle.

Land isolation is name-based because the observation reveals a card name, not
which previously unseen physical copy supplied it. Same-name copies are never
competitors. They remain distinct physical objects in `|C|`, `|L|`, sampling,
membership tests, Fisher-Yates translation, and replay. An already known exact
object retains the existing exact-position treatment; this proposal does not
weaken or replace it.

## Policy and replay boundary

The proposal never submits the recorded opponent action. Reconstruction still
builds the opponent's hypothetical board and calls the frozen
`botpolicy.Decide`; only the resulting action may advance the world. A land
isolation sample can therefore still reject because of timing, legality,
same-name exact-object history, another decision, or any later prefix fact.

Accepted worlds retain the complete checked chance transcript and intent log.
The planner is cleared at the root, and selected worlds must replay without a
planner. Resampled duplicates continue to own independent mutable engines and
observer maps.

## Failure and diagnostics

- Invalid public constraints or accounting mismatches remain invariant errors.
- Public contradictions retain typed baseline fallback.
- An empty `C` rejects only that hypothetical attempt where appropriate.
- An empty `L` uses `C`; it does not reject the attempt or the root.
- Unsupported land/hand shapes use `C` and increment a normalized fallback
  counter.
- Full-prefix mismatches remain ordinary rejection and keep the existing
  histogram.

Add deterministic aggregate counters for eligible isolation shuffles,
isolation-component selections, empty-isolation fallbacks, and unsupported
land-isolation shapes. They must not include names, physical IDs, or timing and
must not affect proposal control flow.

## Verification

1. Exhaustively enumerate tiny physical decks, including duplicate land names,
   and prove support, uniformity within `C` and `L`, mixture density, membership,
   and exact target/proposal factors.
2. Test multiple land-play deadlines, previous public exits, a nonempty
   pre-shuffle hand, negative upper bounds, and empty-`L` fallback.
3. Test that same-name copies remain interchangeable while exact known objects
   remain exact.
4. Add a real-engine history where the current proposal commonly offers a
   competing land and the mixture increases deterministic acceptance without
   forcing the recorded choice.
5. Repeat source-secret noninterference, worker determinism, selected-world
   replay, duplicate independence, ordinary RNG/head parity, and full package
   tests under `GOMAXPROCS=5 GOMEMLIMIT=5GiB`.
6. Only after a test-first correctness change, rerun the fixed 500-root
   calibration and 125-root one-worker prefix. Compare with 2,473/32,000
   accepted, 138/500 covered, actor coverage 31/250 and 107/250, and 41/258
   later-epoch coverage. Report coverage and cost only; make no strength claim.

Do not run race tests or regenerate goldens. Do not commit, push, merge,
rebase, edit a PR, or deploy without fresh authorization.
