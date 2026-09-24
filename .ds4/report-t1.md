# CR 702.168 Gift — implementation report (agent-20260919T183016Z-886b1a86)

## Summary

Implemented Bloomburrow's Gift mechanic end to end: the free cast-time promise
election, the promise carried as a CastFlag (`state.FlagPromisedGift`) with the
receiver in `state.Object.GiftPromisedTo`, the `GiftAbility` SVar body resolved
before the spell's other effects, the new `events.GiftPromise` and
`events.GiveGift` Kinds, the `PromisedGift` filter predicate, the
`Count$PromisedGift.<a>.<b>` head, `Defined$ Promised` /
`TokenOwner$ Promised`, and the `trig:GiveGift` family. `kw:Gift` and
`trig:GiveGift` register via `RegisterNonAPI` with real behaviour (not inert
markers). Two commits: `c2dc5503` (feature) and `20646d96` (flag refactor +
Kitnap tests).

`.cards` was present in the worktree (symlink to `/home/sadams/projects/gorge/.cards`,
verified); corpus-backed tests really ran (rules suite ~0.5 s for the targeted
`-run` set, and the acceptance run 1.8 s with corpus).

## What changed, per file

### state
- **`state/object.go`** — added `FlagPromisedGift` (appended after
  `FlagDisguised`, per the enum's append-only precedent) and added it to
  `CastProvenanceFlags`, so `events.Apply`'s StackCopy strip has one home and a
  copy of a promised spell (never cast, CR 707.10) cannot inherit the promise.
  Added `Object.GiftPromisedTo PlayerID`, the promised opponent; it resets with
  the CastFlags window (leaving the battlefield or the stack to a
  non-battlefield zone).

### events
- **`events/event.go`** — appended `GiftPromise` (the election record: Obj the
  spell, Player the promised opponent, Amount 1/0) and `GiveGift` (the
  completed-gift marker) at the end of `Kind`, after `CloneStatic`, with
  append-only comments; `NumKinds = int(GiveGift) + 1`. Added the
  `"promisedgift"` name to `flagNames`, appended at the end of the table.
- **`events/apply.go`** — `GiftPromise` sets/clears `FlagPromisedGift` and
  `GiftPromisedTo`; `GiveGift` is an Apply no-op marker (the Investigate
  shape). The Move resets clear `GiftPromisedTo` with the CastFlags window.
- **`events/event.go` `eventTriggerInterest`** — named `GiftPromise`/`GiveGift`
  in the zero-interest audit list (both ordinals are past the 64-bit mask, so
  both classifiers fail open).

### effects
- **`effects/gift.go`** (new) — `RegisterNonAPI("kw:Gift", "trig:GiveGift")`
  with a doc explaining the casting-option (kw:MayFlashSac) shape.
- **`effects/filter.go`** — the `PromisedGift` predicate reading
  `FlagPromisedGift` (one home); `recognisedPredicate` picks it up, so
  `Card.Self+PromisedGift`, `Card.PromisedGift` and the `!` form all resolve.
- **`effects/count.go`** — the `PromisedGift.<yes>.<no>` head in
  `evalCountBody`, reading the same flag.
- **`effects/context.go`** — the `Defined$ Promised` case, reading the flag +
  `GiftPromisedTo` off `c.Source`; `TokenOwner$ Promised` reaches it through the
  existing default `tokenOwnerPlayers` → `definedSpec` path.
- **`effects/value_heads.go`** — added `"PromisedGift"` to
  `modelledValueHeads`, so `Registry.Unsupported` stops reporting
  `count:PromisedGift`.

### rules
- **`rules/cast.go`** — `pendingCast` gains `giftDone/giftPromise/giftTo`;
  `giftAsk` (a Min 1 / Max 1 `KChoose` offering a decline plus one option per
  legal opponent) slots into `continueCast` right after `altAddAsk`, before the
  X/target stages; `castAnswer` handles `gift_decline`/`gift_promise`;
  `pushCast` folds the answer as `events.GiftPromise` onto the stack object;
  `payCast` re-states the flag on the pay-time CastInfo (whose wholesale
  `CastFlags` assignment would otherwise clear the push-time fold).
- **`rules/stack.go`** — `resolveTop` splices the `GiftAbility` body as the HEAD
  of the spell's own chain (a shallow copy, never a mutation of the parsed
  table) when the object is promised, so a mid-gift ask suspends through the
  ordinary continuation machinery; emits `events.GiveGift` just before the gift
  body runs.
- **`rules/trigmatch_cards.go`** — `giveGiftMatches` (mode `GiveGift`) and its
  registration.
- **`rules/trigger_eligibility.go`** — `GiveGift` mode returns 0 (past the mask,
  fail-open) and is named in the interest audit.
- **`rules/trigmatch_registry_test.go`** — `"GiveGift"` added to
  `addedAfterTheSplit` with the ticket-and-why comment.

### bot
- **`botpolicy/policy.go`** — a `gift_decline`/`gift_promise` KChoose arm that
  takes the decline **by Kind**, not index (the deterministic plain-cast
  direction); the one shared identifier the engine's `castAnswer` also
  dispatches on, so the bot can never re-submit a differently-routed answer.
- **`cmd/botbench/actioncoverage.go`** — `gift_decline`/`gift_promise` added to
  the KChoose option-kind seed.

### tests (new files)
- **`rules/gift_test.go`** — Wear Down (destroy 2 / draw vs destroy 1 / no
  draw), Valley Rally (`TokenOwner$ Promised` Food for the opponent), Octomancer
  (permanent gift token), Kitnap (promise survives to ETB; `Card.PromisedGift`
  gates the stun counters), Jolly Gerbils (`trig:GiveGift` fires on a given gift
  and not on a declined one), and the primitive-registration census.
- **`rules/gift_counter_test.go`** — Long River's Pull promised (X=0/Y=1: the
  chained counter names a NON-creature spell) and declined (X=1/Y=0: the main
  counter names the creature spell, no draw), plus the stack-copy strip pin.
- **`botpolicy/gift_test.go`** — the bot's decline answer run back through
  `Decision.Validate` (one-home rule).

## Gates run (real output)

```
$ go test -run 'TestGift' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.635s

$ go test -run 'TestGiftPromiseBotAnswerValidates' ./botpolicy/
ok  	github.com/adams-shaun/gorge/botpolicy	0.002s

$ go test -run 'TestValueHeadRegistryMatchesEvaluator|TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched|TestTriggerEligibility|TestCompiledTriggerInterestParity' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.635s

$ go test -run 'TestKnownUnmodelledCountHeads|TestRepoDeckParamsAreRead|TestEveryRepoDeckParamsAreRead' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.663s

$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	4.086s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	0.727s

$ go test -run 'TestHeads|TestEveryRepoDeck|TestRepoDecks|TestRepoDeckGames' ./rules/
ok  	github.com/adams-shaun/gorge/rules	1.786s

$ gofmt -l <changed .go files>
(no output)

$ go run ./cmd/gentypes -check
(no output)

$ go build ./...
(no output)
```

## Head / ratchet movement

**None.** No repo deck carries a `K:Gift` carrier. Re-measured: the two
`Gift` substring hits in `internal/testutil/decks/*.json` are
`"Gift of Immortality"` (avengers-assemble) and `"Gift of Estates"`
(foundations-calling-all-angels) — neither has `K:Gift`. `TestHeads`,
`TestEveryRepoDeckIsFullySupported`, `TestEveryRepoDeckParamsAreRead` and the
botbench split are all byte-identical; `knownUnsupported`,
`knownUnsupportedParams` and `knownUnmodelledCountHeads` were not edited.
(botbench's `TestConstructedDefaultIsByteIdentical` passed unchanged, as
expected.)

Corpus prevalence re-measured and the brief's numbers HELD: 27 `K:Gift` files,
26 `PromisedGift` files, 1 `T:Mode$ GiveGift`, 27 `GiftAbility` files.

## Fails without the fix

Scripted: each non-test hunk reverted, the named test run, the file restored and
`cmp`-verified byte-identical. Real output (`.ds4/scratch/fails.log`):

```
### giftAsk dispatch (rules/cast.go)
restored byte-identical: yes
test TestGiftWearDownPromisedDestroysTwoAndDrawsForPromised: FAILED
  gift_test.go:87: gift ask pending = ... Kind:target ..., want a KChoose

### GiftAbility resolution splice (rules/stack.go)
restored byte-identical: yes
test TestGiftValleyRallyCreatesFoodForPromisedOpponent: FAILED
  gift_test.go:232: promised opponent Food tokens = 0, want 1 (TokenOwner$ Promised)

### Count$PromisedGift head (effects/count.go)
restored byte-identical: yes
test TestGiftWearDownPromisedDestroysTwoAndDrawsForPromised: FAILED
  gift_test.go:95: promised Wear Down target bound = 1..1, want 2..2 (Count$PromisedGift.2.1 unread)

### CastProvenanceFlags strip (state/object.go)
restored byte-identical: yes
test TestGiftCopyCarriesNoPromise: FAILED
  gift_counter_test.go:154: stack copy inherited the gift promise: flags=promisedgift GiftPromisedTo=0, want no promisedgift flag / 0

### trig:GiveGift registration (rules/trigmatch_cards.go)
restored byte-identical: yes
test TestGiftJollyGerbilsTriggersOnAGivenGift: FAILED
  gift_test.go:291: Jolly Gerbils hand = 7, want 8 (cast rally -1, gerbils draw +1)

### Defined$ Promised (effects/context.go)
restored byte-identical: yes
test TestGiftValleyRallyCreatesFoodForPromisedOpponent: FAILED
  gift_test.go:232: promised opponent Food tokens = 0, want 1 (TokenOwner$ Promised)

### PromisedGift predicate (effects/filter.go)
restored byte-identical: yes
test TestGiftKitnapPromisedDrawsAndSkipsStunCounters: FAILED
  gift_test.go:374: promised Kitnap put 3 stun counters on the bear, want 0 (Card.PromisedGift read true)
```

## Deviations from the brief

1. **Perch Protection's unsupported set.** The brief says "names exactly
   `api:Phases`". Measured: `[api:Phases, stat:CantChangeLife]`.
   `stat:CantChangeLife` is Perch Protection's chained `DB$ Effect |
   StaticAbilities$ STCantChange` life-total static — the pre-existing Effect
   static gap (AGENTS.md's "Effect registers real continuous effects only for
   ..." row), not a Gift gap. The test asserts the measured pair, not the
   brief's expectation.
2. **Permanent gifts resolve during the spell's resolution, just before the
   permanent enters, not as a separate "when it enters" triggered ability.**
   The gift body is spliced as the head of the spell's chain (one code path for
   instants/sorceries/permanents). The observable results the brief names are
   correct (Octomancer creates the promised opponent's 8/8; Kitnap's promise
   survives to its ETB trigger and drives `Card.PromisedGift`), but the gift
   action's position relative to other ETB triggers differs from a true ETB
   trigger. See Issues #2.
3. **Long River's Pull's offer gate.** Blue's offer gate evaluates the
   UNPROMISED X=1 bound at cast-offer time (before the election), so the spell
   is offered only when a creature spell is on the stack. The promised test
   therefore has a creature spell (which admits the offer) plus a seat-1
   non-creature instant; the promised DBCounter names the non-creature spell,
   proving the "any spell" (Y=1) half. See Issues #3.

## Issues (found, not fixed)

1. **Perch Protection's `stat:CantChangeLife` is unimplemented** (`S:Mode$
   CantChangeLife` under a `DB$ Effect | StaticAbilities$`), so only the
   phase-out rider (`api:Phases`) is missing of the brief's named set. The
   Gift half works. This is inside AGENTS.md's existing Effect static row; do
   NOT add a row. A CR-lane candidate: CR 614.1a's "life total can't change".
2. **Permanent gifts are not modeled as triggered abilities (CR 702.168c).**
   `rules/stack.go`'s `resolveTop` runs the `GiftAbility` body before the
   spell's other effects for every spell type, so a permanent's gift fires
   during resolution rather than as an ETB trigger. A true triggered-ability
   model would queue it in `rules/altcast.go`'s entry hook like Evoke. The
   named test cards behave correctly; a `GiveGift` marker fires during
   resolution. Precise enough to ticket: `rules/stack.go resolveTop` gift
   splice; corpus 27 carriers, of which Kitnap and Octomancer are permanent
   carriers.
3. **The cast-offer gate for `Count$PromisedGift`-bounded targeting uses the
   unpromised branch.** `rules/cast.go`'s offer/legality evaluation resolves
   `TargetMin$/Max$ X` (X=`Count$PromisedGift.0.1`) before the election, so a
   spell whose promise would widen its target set is not offered when only a
   non-promised-legal target exists. Long River's Pull is the only measured
   carrier of this shape (`X:Count$PromisedGift.0.1`); a structural fix would
   evaluate the union of both branches in the offer gate. Worth a CR-lane test
   citing CR 601.2c + 702.168a.
4. **`Defined$ Promised` reads the live receiver; a promised opponent who has
   left the game is not re-checked.** `effects/context.go`'s `Promised` case
   returns the stored seat (bounds-checked but not aliveness-checked), so a
   departed opponent's gift resolves against a lost seat. Low impact (the
   engine's `effDraw`/`effToken` guard lost seats); noted for completeness.
