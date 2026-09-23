# fb-20260923T005857Z-c1a24352 — Count$ResolvedThisTurn (Sephiroth transform)

## Summary

Two reported symptoms, one root cause. `Count$ResolvedThisTurn` — the SVar body
behind Sephiroth, Fabled SOLDIER's "If this is the fourth time this ability has
resolved this turn, transform Sephiroth" — was unmodelled, so the SVar gate
failed OPEN and `DB$ SetState | Mode$ Transform` flipped the creature on the
FIRST resolution. The back face prints `ManaCost:no cost`, so `Rakdos, the
Muscle`'s `TriggeredCard$CardManaCost` read 0 and exiled nothing (symptom 2,
downstream of symptom 1).

The head is now modelled; the tally is a per-ability, per-turn count folded
from the existing `Resolve` event and bound onto `effects.Ctx` by `rules`.

## What changed, per file

- `state/game.go` — new `Game.ResolvedThisTurn map[string]int32`. Keyed by
  `events.ResolvedAbilityKey` (source ObjID + root `Ability$` body content).
  The `CombatsThisTurn` shape: folded from events, cleared at `TurnChange`.
  `Game.Clone` deep-copies it (the `ExtraTurns` contract) because `Apply`
  increments it in place — a shared map would let a clone corrupt the original.
- `events/abilitykey.go` (new) — `ResolvedAbilityKey(source, sa)`: a
  deterministic pre-order walk of the root `*cards.SA`'s Kind/API/params (sorted
  keys) plus its `Sub` chain. Content, NOT pointer: `cards.Link`/`ResolveSVar`
  parse the `Execute$` SVar text fresh, so one card's two T: lines that share an
  `Execute$` SVar carry pointer-distinct but structurally-equal SAs (Victor,
  Valgavoth's Seneschal's `ChangesZone` and `FullyUnlock` both `Execute$
  TrigSurveil`). Pointer identity would split their tally; content merges it.
- `events/apply.go` — split `Resolve` out of the inert marker group into its own
  `case`, which increments the tally for the resolving ability stack object
  (`o.Source` + `o.Ability`; a spell carries no `Ability`). Reset added at the
  `TurnChange` boundary next to `CombatsThisTurn`. No new `Kind` or `Event`
  field: the tally is folded from the existing `Resolve` event.
- `effects/registry.go` — `Ctx.ResolvedThisTurn int32` (bound data; effects
  cannot import rules).
- `effects/count.go` — `case "ResolvedThisTurn": return c.ResolvedThisTurn, true`
  in `evalCountBody`, so `EvalCountOK` reports it modelled. An unbound Ctx reads
  a legitimate zero (fail CLOSED), not the unresolvable verdict.
- `rules/stack.go` — `resolvedAbilityTally(o)` / `resolvedAbilityTallyFor(source, sa)`
  (one home) read the tally; bound in `resolveTop`'s ability branch and in
  `resolveAbility` (the direct-resolution path the brief names).
- `rules/resolution.go` — `resumeResolution`'s Ctx rebuild re-binds the tally,
  so a chain suspended at a mid-resolution ask reads the same ordinal on re-entry.
- `rules/count_head_ratchet_test.go` — deleted the `"Count$ResolvedThisTurn"`
  map entry and its 3-line comment; comment now says "five bodies remain".
- `rules/paramcensus_gates_test.go` — retargeted `TestGateChainWiring`'s
  fail-open assertion from the now-modelled `Count$ResolvedThisTurn` to the
  still-unmodelled `Count$CardNumAttacksThisTurn`, so the contract stays pinned.
- `rules/replacement_turn_mana_test.go` — `TestSephirothTransformRunsTheDestinationFaceReplacement`
  drove `DBTransform` directly and relied on the old fail-open. It now presents
  the fourth-resolution context (seeds `ResolvedThisTurn` for the body's key)
  before calling `resolveAbility`, which is what the leaf is reached under. Its
  focus (the destination face's `repl:Transform` body runs) is unchanged, and
  its `replayCheck` still passes (`diffGames` does not compare the tally, which
  is derived from the log anyway).
- `effects/resolved_this_turn_test.go`, `events/abilitykey_test.go`,
  `rules/sephiroth_resolved_test.go`, `state/clone_resolved_test.go` (new tests).

## Structural fix, not a card allowlist

The fix is generic: the head is modelled for every carrier, and the identity is
derived from `(source, body content)`, so the next `Count$ResolvedThisTurn` card
is covered without touching code. The two known hazards are covered
structurally: **repeated resolutions of one ability accumulate** (Sephiroth test:
1,2,3,4) and **two T: lines sharing one `Execute$` SVar merge into one tally**
(`TestResolvedAbilityKeyMergesContentEqualBodies`, which proves the two SAs are
pointer-distinct first). Prowl's back face (`DB$ SetState | Mode$ Transform |
ConditionCheckSVar$ TrigAmount | ConditionSVarCompare$ EQ2`) carries the
identical shape and is fixed by the same head.

## Gates run (real output)

Corpus present as a symlink to `/home/sadams/projects/gorge/.cards` (verified);
`TestEveryRepoDeckCountHeadResolves` ran 0.84s, so corpus tests executed rather
than skipped.

Targeted command (brief's "Done means"; `TestParamcensus` does not exist as a
test name — the fail-open assertion lives in `TestGateChainWiring`), plus the
events key test:

```
$ go test -run 'TestSephiroth|TestEveryRepoDeckCountHeadResolves|TestParamcensus|TestRefPropertyCounts|TestResolvedThisTurn|TestResolvedAbilityKey|TestGateChainWiring|TestRakdosMuscle' ./rules/ ./effects/ ./events/
ok  	github.com/adams-shaun/gorge/rules	0.640s
ok  	github.com/adams-shaun/gorge/effects	0.014s
ok  	github.com/adams-shaun/gorge/events	0.003s
```

Verbose confirmation the corpus-backed tests ran (not skipped):

```
--- PASS: TestEveryRepoDeckCountHeadResolves (0.84s)
--- PASS: TestGateChainWiring (0.00s)
--- PASS: TestRakdosMuscleSacTriggerExilesAndMayPlaysWithAnyTypeMana (0.00s)
--- PASS: TestSephirothTransformsOnlyOnFourthResolution (0.00s)
--- PASS: TestRakdosExilesUntransformedSephirothManaValue (0.00s)
--- PASS: TestRefPropertyCounts (0.00s)
--- PASS: TestResolvedThisTurnCountHead (0.00s)
```

Behaviour goldens:

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	2.440s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.207s
```

The botbench split did NOT move — the repo-deck behaviour change (Sephiroth,
plus Nissa/Tannuk in `pro-shaper.json`) did not flip any of the 20 games, so no
re-pin and no attribution was needed.

Packages edited with no `-run` target in the brief, run once each:

```
$ go test ./events/ ./state/
ok  	github.com/adams-shaun/gorge/events	4.553s
ok  	github.com/adams-shaun/gorge/state	(cached)
```

Formatting / generated types (the Go half of `make lint`):

```
$ gofmt -l cards/ state/ events/ effects/ rules/     # (no output)
$ go run ./cmd/gentypes -check                        # (no output)
```

## Fails without the fix

Reverted ONLY the `case "ResolvedThisTurn"` hunk in `effects/count.go` (scratch
copy taken first, file restored byte-identically — `cmp` confirmed), then:

```
$ go test -run 'TestSephirothTransformsOnlyOnFourthResolution|TestRakdosExilesUntransformed|TestResolvedThisTurnCountHead|TestEveryRepoDeckCountHeadResolves' ./rules/ ./effects/

--- FAIL: TestEveryRepoDeckCountHeadResolves (0.58s)
    count_head_ratchet_test.go:103: "Count$ResolvedThisTurn" evaluates unresolvable from a repo deck (carried by [Nissa, Resurgent Animist Sephiroth, Fabled SOLDIER Tannuk, Memorial Ensign]), which is not in knownUnmodelledCountHeads -- new gap, add it to the table
--- FAIL: TestSephirothTransformsOnlyOnFourthResolution (0.00s)
    sephiroth_resolved_test.go:72: after 1 resolution(s) face = "Sephiroth, One-Winged Angel", want the front face (transformed too early)
--- FAIL: TestRakdosExilesUntransformedSephirothManaValue (0.00s)
    sephiroth_resolved_test.go:112: after one resolution face = &{Sephiroth, One-Winged Angel no cost [...]}, want the front face with mana value 3
--- FAIL: TestResolvedThisTurnCountHead (0.00s)
    resolved_this_turn_test.go:17: EvalCountOK(bound) = (0, false), want (4, true)
FAIL	github.com/adams-shaun/gorge/rules	0.615s
FAIL	github.com/adams-shaun/gorge/effects	0.010s
```

Every new test fails with the fix reverted (the ratchet test because the head
reverts to unmodelled; the two rules tests because the first resolution
transforms; the effects test because the head is unresolvable; the clone test
because the shared map leaks). Preconditions
asserted in each test: Sephiroth is on the battlefield showing the front face
with mana value 3; the library holds ≥4 cards with the top three distinguishable
from the fourth; the trigger actually queued (`observedTriggerCount > 0`); the
two content-equal SAs are pointer-distinct.

The clone fix has its own revert proof — removing ONLY the `ResolvedThisTurn`
deep-copy from `state/game.go` (scratch copy restored byte-identically):

```
$ go test -run TestCloneOwnsResolvedThisTurn ./state/
--- FAIL: TestCloneOwnsResolvedThisTurn (0.00s)
    clone_resolved_test.go:22: writing the clone changed the original tally to 4, want 3 -- Clone shares the map
FAIL
```

## Precondition / vacuity notes

- `TestSephirothTransformsOnlyOnFourthResolution` asserts the front-face mana
  value is 3 before the deaths, so the face check is not vacuous.
- `TestRakdosExilesUntransformedSephirothManaValue` asserts the sacrificed
  object is the front face with mana value 3 *after one resolution* and that the
  fourth library card stays in the library (proving exactly 3 exiled, not more).
  It fires one real death trigger first, matching the player's sequence.
- `TestSephirothResolvedTallyReplaysExactly` is event-driven and calls
  `replayCheck` (log-only replay must rebuild the tally and transform on the
  fourth). Sephiroth's own ETB trigger is cleared before the deaths so it is not
  a confound; the trigger count is asserted per death.

## Scope / deviations

- `AGENTS.md` is NOT touched; `knownApproximationRows` is NOT changed. The
  `(devthr1)` row at `AGENTS.md:231` still names `Count$ResolvedThisTurn` among
  its six bodies — correctly, since the row closes only when ALL six resolve and
  this ticket closes one. It is already stale on `YourStartingLife` (closed by
  `827ca863`), which is not this ticket's business.
- The other five ratchet bodies (`MaxOppDamageThisTurn`, `CardNumAttacksThisTurn`,
  `NonCombatDamageThisTurn`, `ChosenNumber`, and the stale `YourStartingLife`)
  are out of scope and untouched.
- `rules/replacement_turn_mana_test.go` was modified out of necessity: its
  direct `DBTransform` invocation relied on the head failing open. The change is
  minimal and preserves its intent.

## Issues (found, not fixed)

1. **Five `Count$ResolvedThisTurn`-sibling heads remain unmodelled** in
   `rules/count_head_ratchet_test.go`'s `knownUnmodelledCountHeads`
   (`Count$MaxOppDamageThisTurn`, `Count$CardNumAttacksThisTurn`,
   `Count$NonCombatDamageThisTurn`, and the unbound-context `Count$ChosenNumber`;
   `Count$YourStartingLife` is already modelled but stale in the AGENTS.md row).
   Each is a separate ticket.
2. **`resolveAbility`'s direct-resolution path reads the tally as 0** unless a
   map entry exists, because only a stack `Resolve` event increments it. This is
   correct (a synthetic direct resolution never "resolved this turn"), but it is
   the reason `TestSephirothTransformRunsTheDestinationFaceReplacement` needed
   the tally presented explicitly. Worth a note if a future engine flow resolves
   a real repeatable ability through `resolveAbility` rather than `resolveTop`.
3. **`Count$ResolvedThisTurn` on an ACTIVATED ability** (Ashling the Pilgrim,
   Bronze Cudgels, Inner-Flame Igniter, Soulbright Seeker/Flamekin, Temporal
   Aperture) is now supported by the same content key, but only the triggered
   path is covered by a test in this repo's decks (none of those activated
   carriers are in a repo deck). If one is added, the activated path should be
   pinned end to end.

## Ironies / ledger

The defect was invisible to `.ds4/ledger.json` (no CR-lane test named the
transform gate). A CR-lane test citing CR 608.2m / CR 603.4 for
"an ability that has resolved this turn" would surface the head's family; not
written here (not in brief).

---

# Report — stat:CountersRemain

Implemented `S:Mode$ CountersRemain` for the two corpus carriers. The worktree was rebased onto `main` before implementation (it reported up to date), and `.cards` was already present. Measured prevalence: 2 files, `Me, the Immortal` and `Skullbriar, the Walking Grave`.

## What changed

- `rules/statics.go`: registered `stat:CountersRemain`; added `countersRemainApplies`, using the canonical active-static walk and `ValidCard$` filter (missing filters fail closed).
- `rules/engine.go`: tags a final, replacement-adjusted battlefield departure when its own active static matches, except moves to hand/library.
- `events/actions.go`, `events/apply.go`: carries the preservation marker in the existing MoveZone `Counter` payload while retaining any existing payload; replay decodes it and preserves counters during the Move fold. Ordinary moves and moves to hand/library still clear counters. No event kind/field or encoding changed.
- `rules/counters_remain_test.go`: real-corpus test on Me, the Immortal; asserts static/preconditions, adds counters, moves to exile and back to the battlefield, then verifies a hand move clears them.

## Gates and measurements

Corpus check:

```text
$ grep -rlE '^S:Mode\$ CountersRemain' .cards/cardsfolder | sort
.cards/cardsfolder/m/me_the_immortal.txt
.cards/cardsfolder/s/skullbriar_the_walking_grave.txt
$ grep -rlE '^S:Mode\$ CountersRemain' .cards/cardsfolder | wc -l
2
```

Targeted test:

```text
$ go test -run 'TestCountersRemainPreservesCountersExceptHandAndLibrary$' ./rules/
ok  github.com/adams-shaun/gorge/rules  0.593s
```

The test was also run before the final guard-only refinement:

```text
$ go test -run 'TestCountersRemainPreservesCountersExceptHandAndLibrary$' ./rules/
ok  github.com/adams-shaun/gorge/rules  0.595s
```

`make report` (first run rebuilt the stale IR cache):

```text
CGO_ENABLED=0 go build -o bin/forgec ./cmd/forgec
bin/forgec report -dir .cards
forgec: IR cache unusable (IR cache version 3, want 5 — run `make compile-cards`); compiling fresh from cardsfolder
corpus: 95f04e8a04c8925fa97cb226fc3341cabcc90a53 @ 95f04e8a04c8925fa97cb226fc3341cabcc90a53 (GPL-3.0, 33669 files)
cards: 33667  playable: 29738 (88.3%)
tokens: 839
```
The primitive is registered and report completed against the full corpus.

Required behaviour goldens:

```text
$ go test ./internal/archtest/
ok  github.com/adams-shaun/gorge/internal/archtest  3.574s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  github.com/adams-shaun/gorge/cmd/botbench  1.333s
```

Formatting/type check and whitespace:

```text
$ gofmt -l events/actions.go events/apply.go rules/engine.go rules/statics.go rules/counters_remain_test.go
(no output)
$ go run ./cmd/gentypes -check
(no output, exit 0)
$ git diff --check
(no output, exit 0)
```

## Fails without the fix

Copied `rules/engine.go` to `.ds4/scratch/`, removed the event-tagging hunk, and ran the targeted test. It failed on the actual exile move. Restored the file and verified it byte-identically with `cmp`.

```text
$ go test -run 'TestCountersRemainPreservesCountersExceptHandAndLibrary$' ./rules/
--- FAIL: TestCountersRemainPreservesCountersExceptHandAndLibrary (0.58s)
    counters_remain_test.go:28: P1P1 counters after battlefield-to-exile move = 0, want 2
FAIL
FAIL  github.com/adams-shaun/gorge/rules  0.593s
FAIL
RESTORED_BYTE_IDENTICAL
```

## Issues

No additional unfixed defects found in this scope. No AGENTS.md approximation row was present for this primitive to delete. No CR-lane test was added: the requested behaviour is directly pinned by the real-carrier engine test.

Commit: `c1b64881 feat(rules): preserve counters for CountersRemain statics`

---

# Report — TriggerController$ on ChangesZone

## Changes

- `rules/trigger_match.go`: when a `ChangesZone`/`ChangesZoneAll` trigger explicitly names `TriggerController$ TriggeredCardController`, the queued ability's controller (and matching `Ctx.Controller`) now comes from the moved permanent's LKI on a battlefield departure. This keeps APNAP grouping, stack control and resolution under that controller, without changing default trigger controller behavior or interpreting other selector values.
- `rules/trigger_controller_test.go`: added a focused integration test using an inline watcher script. A seat-1 watcher sees a creature stolen by seat 0 die; it asserts the creature is on the battlefield before departure, the two controllers differ, its live controller has reset to its owner after moving, the pending/stack trigger belongs to seat 0 rather than the watcher, and resolution affects only seat 0.

The parser/read census needed no special table change. The ordinary `t.Params["TriggerController"]` access is included by the static trigger-parameter census; `TestEveryRepoDeckParamsAreRead` passes with no new unsupported entry. GNU grep measured 42 corpus files with `TriggerController$`.

A direct Junji corpus test was not added: Junji's trigger is on the same card that dies, so the existing default leaves-the-battlefield LKI controller path already assigns it to the departing card's last controller. Such a test would pass with this fix reverted and violate the required fail-without-fix proof. The new inline watcher test isolates and proves the selector's distinct behavior. Junji's compiled script is present in `.cards`.

## Fails without the fix

Saved the fixed production file, removed only the new controller-selection hunk, ran the focused test, and restored the file byte-identically (`cmp` exit 0):

```text
go test -run '^TestChangesZoneTriggeredCardControllerUsesDepartingCardLKI$' ./rules/
--- FAIL: TestChangesZoneTriggeredCardControllerUsesDepartingCardLKI (0.00s)
    trigger_controller_test.go:32: trigger controller = 1, want departing card's last controller seat 0 (witness controller is seat 1)
FAIL
FAIL github.com/adams-shaun/gorge/rules 0.003s
```

## Gates and output

Corpus present: `.cards` is a symlink to `/home/sadams/projects/gorge/.cards`.

```text
/usr/bin/grep -rlE 'TriggerController\\$' .cards/cardsfolder | wc -l
42

gofmt -l rules/trigger_match.go rules/trigger_controller_test.go
[no output]
go run ./cmd/gentypes -check
[no output]

go test -run 'TestChangesZoneTriggeredCardControllerUsesDepartingCardLKI|TestEveryRepoDeckParamsAreRead' ./rules/
ok github.com/adams-shaun/gorge/rules 0.750s

go test ./internal/archtest/
ok github.com/adams-shaun/gorge/internal/archtest 3.489s

go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok github.com/adams-shaun/gorge/cmd/botbench 1.334s

git diff --check
[no output]
```

Botbench split remains byte-identical; no golden update was needed. No Known-approximations row is closed by this change.

## Issues

No unfixed behavior identified within the requested `TriggeredCardController` selector for battlefield departures. Other `TriggerController$` spellings and ChangesZone events that are not battlefield departures remain outside this implementation's scope; the selector is intentionally limited to the substantiated form and event/LKI boundary.

---

# Reports merged in from main (integration of main into wt/agent-20260919T194848Z-9bc258a5)

# Report — cli-20260923T060000Z-trig-attackerblocked

---

# Report — api:Attach Optional$ / Yuffie object-choice attach

## Summary

Closed the ticket's one sub-shape: `Mode$ AttackerBlocked` (and its sibling
`Mode$ AttackerBlockedByCreature`) now fire `AddTrigger$`-granted instances, not
only printed face triggers. Because the row's other three sub-shapes
(AttackersDeclared, Cycled, CounterAdded) had all landed on this base, this
commit also deletes the "Four trigger modes carry limits" row from AGENTS.md and
lowers `knownApproximationRows` 21 → 20.

Commit: `76014dd4`

## Workspace facts used

- `.cards` was already a symlink in the worktree (`ls -la .cards` →
  `.cards -> /home/sadams/projects/gorge/.cards`); corpus-backed tests really
  ran (the new tests parse real corpus cards, and a missing corpus would have
  `t.Fatalf`'d on the parse). No skips.
- Sibling landing check on the branch: `git log --oneline` shows
  `b5f1a0d9 merge(...trig-attackersdeclared)`, `c37f9f03 merge(...trig-cycled)`,
  `db800365 merge(...trig-counteradded)`. All three sub-shapes are on the base,
  so the row deletion is authorised.

## What changed, per file

### `rules/trigmatch_combat.go`

The root cause: `AttackerBlocked` and `AttackerBlockedByCreature` are dedicated
hooks, not `trigMatchers` entries (`rules/trigmatch_combat.go`'s `init()` does
not register them). The ordinary granted-trigger walk
(`checkGrantedStaticTriggersUsing`) queues a grant only after
`triggerMatches(...)` returns true, and `triggerMatches` returns false when
`trigMatchers[t.Mode] == nil`. So every `AddTrigger$` grant of these two modes
was rejected and never fired.

- Extracted the per-trigger queue body of `checkAttackerBlockedTriggers` into a
  new shared helper `queueAttackerBlockedTrigger(t, source, controller, idx,
  granted, grantor, ev)`. Printed triggers and granted instances now share the
  zone/phase gates, the read-only-then-reserve limit discipline, the two
  `ValidCard$`/`ValidBlocker$` candidate walks and the reference capture. The
  helper carries `Granted`/`Grantor`/`Execute` onto the `pendingTrigger` so
  `pushTrigger` routes it through `events.GrantTriggerPush` (the replayable-grant
  path) and `events.Apply` rebuilds the `Execute$` body from the grantor's SVar
  table. This is exactly the `queueAttackerUnblockedTrigger` /
  `checkGrantedAttackerUnblockedTriggers` shape already used for the sibling
  `AttackerUnblocked` mode.
- Added `checkGrantedAttackerBlockedTriggers(ev)`, called from
  `checkAttackerBlockedTriggers` after the printed walk. It iterates the
  deterministic `e.active()` slice, selects live grants whose
  `ce.AddTrigger.Mode` is one of the two modes, resolves the grantor
  (`ce.Source`, or `ce.TriggerGrantor` for the Animate route), links the body
  with `grantedTriggerExecute`, and for each object matching `ce.Affects` queues
  through the same helper with `idx = -1` and the grant provenance. A grant
  whose body cannot be resolved queues nothing (the live==replay gate).
- The extracted helper adds an early `t.Effect == nil` return. The printed path
  previously broke out of the candidate walk on a nil effect; behaviour is
  identical (nothing queued, no limit consumed).

`attackerBlockedCandidates` itself was already correct (it reads `ValidCard$`
against each attacker) and needed no change; the fix is that granted instances
now reach it.

### `rules/attacker_blocked_grants_test.go` (new)

Two tests, both driven by real corpus cards:

- `TestGrantedAttackerBlockedByCreaturePumps` — Retaliation
  (`AddTrigger$ TrigBlocked`, `Mode$ AttackerBlockedByCreature | ValidCard$
  Card.Self | ValidBlocker$ Creature`). A granted 2/2 becomes blocked and pumps
  to 3/3.
- `TestGrantedAttackerBlockedDraws` — Stormsurge Kraken
  (`AddTrigger$ TrigBlocked`, `Mode$ AttackerBlocked | ValidCard$ Card.Self`,
  `OptionalDecider$ You` draw two). The Kraken, with a commander in play so the
  Lieutenant static is live, becomes blocked and draws two.

Each asserts its own preconditions: the recipient is on the battlefield; the
compared values differ (2/2 → 3/3; hand 0 → 2); the recipient prints NO
become-blocked trigger (so the path under test is the granted one, not a
printed line); and `GrantTriggerPush == 1` (the granted handler actually ran).
The Kraken test also asserts the static is live via 7/7 (printed 5/5 + granted
2/2), so the `IsPresent$`-gated grant is proven before the block. Both build a
`Clone()` and compare in the Retaliation case (the granted body is an SVar
resolved from the grantor's table during Apply).

### `AGENTS.md`

Deleted the row (found by its text):

> Four trigger modes carry limits. ... **AttackerBlocked** misses
> `AddTrigger$`-granted instances. | `rules/trigger_match.go` (...) | M4 (...)

No new row, no other row touched.

### `internal/testutil/agentsdoc_test.go`

`knownApproximationRows` 21 → 20, with a comment noting this ticket's deletion.

## Gates run (real output pasted)

Environment: `.cards` symlink present; `GOFLAGS=-p=2` and `GOMEMLIMIT` left at
their defaults; no `-p`/`-parallel` override.

Targeted tests (the brief's one gated command plus the fix's siblings):

```
$ go test -count=1 -run 'TestGrantedAttackerBlocked' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.007s
```

```
$ go test -run 'Afflict|Flanking|AttackerBlocked|AttackerUnblocked|AttackerUnblockedOnce|BlocksTrigger|BlockerDeclaration|MinMaxBlocker|MustBlock|Menace' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.793s
```

Ratchets that a merge with main newly enforces:

```
$ go test -run 'TestParamCensus|TestEveryRepoDeckParamsAreRead|TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched|TestEveryDispatchedTriggerModeHasAMatcher' ./rules/
ok  	github.com/adams-shaun/gorge/rules	1.035s
```

```
$ go test -run 'TestKnownApproximation|TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' ./internal/testutil/
ok  	github.com/adams-shaun/gorge/internal/testutil	0.001s
```

Behaviour goldens outside `rules/` (run once, before DONE):

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.637s
```

```
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.305s
```

```
$ gofmt -l rules/trigmatch_combat.go rules/attacker_blocked_grants_test.go internal/testutil/agentsdoc_test.go
(no output)
$ go build ./...
(no output)
$ go run ./cmd/gentypes -check
(no output)
```

## Fails without the fix

Restored the pre-fix `rules/trigmatch_combat.go` (saved to
`.ds4/scratch/trigmatch_combat.go.orig`), ran the new tests, then restored the
fixed file byte-identically (`cmp` against `.ds4/scratch/trigmatch_combat.go.fixed`
printed `RESTORED_BYTE_IDENTICAL`):

```
$ go test -run 'TestGrantedAttackerBlocked' ./rules/
--- FAIL: TestGrantedAttackerBlockedByCreaturePumps (0.00s)
    attacker_blocked_grants_test.go:58: Retaliation granted AttackerBlockedByCreature GrantTriggerPush events = 0, want 1
--- FAIL: TestGrantedAttackerBlockedDraws (0.00s)
    attacker_blocked_grants_test.go:117: Stormsurge Kraken granted AttackerBlocked GrantTriggerPush events = 0, want 1
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.006s
FAIL
```

Both fail at the granted-handler assertion, i.e. with the fix reverted the
granted walk does not exist and no `GrantTriggerPush` is emitted.

## Head / ratchet movement

None measured:

- `TestConstructedDefaultIsByteIdentical` (`cmd/botbench`) passed unchanged, so
  the 20-game bot win split did not move → **no botbench re-pin**.
- Confirmed no repo-deck card carries any of the three affected corpus carriers:
  `grep -rl 'Stormsurge Kraken'|'Retaliation'|'Mirror Shield'` over
  `internal/testutil/decks/` returns nothing, so `TestHeads` and the deck
  acceptance/replay goldens cannot move from this change.
- No `knownUnsupported`, `knownUnsupportedParams`, `knownUnmodelledCountHeads`
  or `registeredModes` entry changed; the four ratchet scans above pass.
- No new `Mode$` was registered (the two modes stay dedicated hooks), so the
  registry ratchet is untouched.
- No `events.Kind` change; the granted instance uses the existing
  `GrantTriggerPush` path.

## Structural fix (not the instance)

The brief names "expand the granted triggers … so a granted instance is a
candidate like a printed one". I chose the shared-helper shape: one
`queueAttackerBlockedTrigger` that both the printed walk and the granted walk
call, and one granted walk that selects grants by MODE (not by a hard-coded
list of cards or by parsing printed faces). The next sibling carrier of either
mode is covered automatically — any live `AddTrigger$` whose `Mode$` is
`AttackerBlocked`/`AttackerBlockedByCreature` flows through the same path, and
the two modes cannot drift in gates/referents because they share the helper.
A corpus carrier list is deliberately not encoded.

I did NOT touch the AttackersDeclared, Cycled or CounterAdded code (sibling
sub-shapes, out of scope).

## Deviations from the brief

None. The brief's `## Workspace facts`/row quote differ slightly from the actual
AGENTS.md text ("per declare step" vs actual wording); I deleted the row by its
actual text.

## Open concerns / caveats

1. `rules/paramcensus_test.go`'s comment above the `trig:` read-root list still
   says a granted trigger of ANY mode "matches through triggerMatches' own
   dispatch". That was already false for these two dedicated-hook modes and is
   the very defect fixed here; the granted half now lives in
   `checkGrantedAttackerBlockedTriggers`. Comment-only drift; the census tests
   pass. I left it unchanged to stay inside the brief.
2. Mirror Shield's `AddTrigger$ TrigBlocks & TrigBecomeBlocked` still does not
   fire — but for a DIFFERENT, pre-existing reason (the `&`-joined multi-name
   value is never split, so no grant registers at all). Filed as a new ticket;
   see `## Issues`.

## Issues

- **`AddTrigger$` with `&`-joined SVar names never registers** (out of scope;
  filed as `.ds4/new-tickets/addtrigger-multiname-ampersand.md`,
  Priority 2). `rules/layers.go` (`staticEffects`, AddTrigger branch) looks up
  the WHOLE `st.Params["AddTrigger"]` string in `fc.SVars`; `cards/parse.go`
  does not split a parameter value on `&`. Measured: 5 corpus files use the
  shape (`mirror_shield`, `veterans_armaments`, `astrologians_planisphere`,
  `candlekeep_sage`, `noble_heritage`); a throwaway test confirmed Mirror Shield
  registers **0** AddTrigger grants. This is the multi-name grammar shared by
  every granted mode, not the AttackerBlocked sub-shape this ticket closed
  (which is about making a REGISTERED grant fire). Fixing it changes granted
  behaviour for all modes, so it is a separate ticket.
- **`paramcensus_test.go` stale comment** (comment-only): the `trig:` read-root
  preamble claims all granted triggers dispatch through `triggerMatches`; the
  two become-blocked modes now have a dedicated granted walk. See concern 1.
- No CR-lane test was added or is proposed. The defect is a trigger-dispatch
  gap, not a CR-rule conformance gap, and the ticket brief asked for a card-level
  test only.

---

- `effects/attach.go`: Generalized destination validation to accept the actual object being attached, rather than always using the resolving source. For a `Choices$` object-side pool, each candidate is now checked against its possible destinations; the offered candidates are restricted to attachable objects and the destination list saved over suspension is the intersection valid for every offered object. This fixes Yuffie's real `Choices$ Equipment.YouCtrl | Defined$ Self` ETB: formerly the resolver treated Yuffie as the attached object, filtered Yuffie itself as an illegal self-destination, and emitted `cannot attach: no legal target` without posing the optional election. The existing Optional$ ask/decline flow had already landed in commit `744f665c`; this change corrects the Choices$ candidate/destination interaction surfaced by this card.
- `rules/yuffie_attach_optional_test.go`: Added a real-corpus Yuffie ETB integration test. It confirms the Equipment is offered in a Min-0/Max-1 choice, the preceding gain-control rider resolves, and declining produces no Attach event and leaves the Equipment unattached. The test also replay-checks the resulting event stream.

The implementation is role-based rather than Yuffie-specific: other object-side Choices$ Attach effects receive destination validation against their offered attaching objects too.

## Workspace and controller directive

- `.cards` existed in this worktree before testing.
- The worktree started clean. Main advanced after the initial inspection: current `HEAD` is `db5952897d663ab39f3d0ee0b560cbc6700634e1`, while current `main` is `9b8072c6966fe7839a7ae7719a92763c97865c6c`.
- I did not run `git rebase main`: the repo worktree instructions explicitly prohibit running `git rebase`. This work is committed on its task branch for the controller's integration/rebase handling.

## Fails without the fix

Saved `effects/attach.go` to `.ds4/scratch/attach.go.fixed`, then temporarily changed object-side candidate validation back to validate against the resolving source. Ran the new test, restored the source file, and confirmed byte identity with `cmp` (`restore_cmp=0`). The negative run failed as intended:

```text
--- FAIL: TestYuffieMayDeclineHerETBAttach (0.63s)
    yuffie_attach_optional_test.go:91: Yuffie's ETB never posed its Optional$ attach choice; events=[...]
FAIL
```

# Report: PlayerCountPropertyYou per-turn counts

## Changes

- `effects/count.go`: resolves `SacrificedThisTurn`, `CardsDiscardedThisTurn`, `LifeLostThisTurn`, and `LandsPlayed` for the `PlayerCountPropertyYou$` head. The event-backed tallies are attributed to the resolving controller; `LandsPlayed` reads the event-mutated per-player state and invalid controller indices remain unresolved. Other property/group spellings remain fail-closed.
- `effects/registry.go`, `rules/stack.go`: added the Host bridge for `SacrificesThisTurn`, folded from sacrifice events since the latest `TurnChange`.
- `effects/context_test.go`, `effects/playercount_property_you_test.go`: fake-host setup and a new effects regression asserting all four supported properties resolve to distinct nonzero controller-0 values and controller-1 values (including its real zero land count).
- `effects/count_compare_test.go`: changed the former unsupported-property assertions to retain only unsupported forms.
- `rules/sacrifices_this_turn_test.go`: new test checks owner attribution for each of two battlefield sacrifice candidates and the `TurnChange` reset.

The structural approach uses the existing replay-derived host event folds for sacrifice, discard, and life loss, plus the existing event-updated `LandsPlayed` state; no parallel effects-side event interpretation was introduced. This covers future cards using these exact You-group properties.

The Evendo Brushrazer end-to-end may-play assertion is deferred exactly as the brief conditions it: `Card.ExiledWithSource` does not have a working matcher in this branch, so fixing this count alone would not enable the card. The new effects test pins the previously unresolved count behavior; the deck ratchet and full Evendo gate were not changed.

Corpus measurements from `.cards/`: the You-group frequencies include 17 `SacrificedThisTurn`, 17 `CardsDiscardedThisTurn`, 15 `HasPropertyBeenAttackedThisCombat`, 6 `LifeLostThisTurn`, 5 `OpponentsAttackedThisCombat`, 4 `AttractionsVisitedThisTurn`, 3 `LandsPlayed`, and 3 `DamageThisTurn` occurrences. Literal `Card.ExiledWithSource` appears in 100 script files (`grep -rlE`), rather than the brief's stated 72; this is a raw-text match count, not a claim that all 100 have the identical Affected context.

The recorded events included `cannot attach: no legal target` after the gain-control rider, confirming the setup reached the actual Yuffie trigger and failed specifically at attachment destination validation.

## Gates run

```text
go test -run '^(TestYuffieMayDeclineHerETBAttach|TestEveryRepoDeckParamsAreRead|TestAjanisChosenMayAttachAskPosesAndYesAttachesTheAura|TestAjanisChosenMayAttachDeclineLeavesTheAuraAndRunsTheChain|TestCoriSteelCutterOptionalAttachAttachesTheEquipment)$' ./rules/
ok   github.com/adams-shaun/gorge/rules  1.101s
```

This includes the parameter census gate (`TestEveryRepoDeckParamsAreRead`) and the Yuffie, Ajani's Chosen and Cori-Steel Cutter attach coverage.

```text
go test ./internal/archtest/
ok   github.com/adams-shaun/gorge/internal/archtest  3.553s

go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok   github.com/adams-shaun/gorge/cmd/botbench  1.339s

gofmt -l effects/attach.go rules/yuffie_attach_optional_test.go
[no output]
go run ./cmd/gentypes -check
[no output]
git diff --check
[no output]
```

No chain-head or botbench golden movement was observed; `TestHeads` was not run because it is a daemon-only gate outside the task's named test.

## Issues

No separate unfixed issue was found. The earlier attached optional-choice implementation was already present in this branch; the Yuffie object-side Choices$ destination mismatch was fixed here.

---

# Report — fb-20260922T145544Z

Restricted floating mana now projects onto the pool readout, so a seat that
holds mana the engine will not spend on the cast they are looking at can see
why.

Commit: `19a2b712 feat(view): project restricted mana onto the pool readout`

- `.cards` in this worktree was already a symlink to
  `/home/sadams/projects/gorge/.cards` (found present, not created).
- `.ds4/` was copied by the controller; `issue.md`, `ledger.json` and prior
  reports were present.

## What changed, per file

- **`view/view.go`**
  - New `PlayerView.PoolRestrictions []PoolRestrictionView` field,
    `json:"pool_restrictions,omitempty"`, inserted after `Pool`, with the
    public-information CR 106.4a/106.4b rationale and the byte-identity
    contract (omitempty, the `Available` convention).
  - New `PoolRestrictionView{Color string; Amount int32; Text string}` (json
    `color`, `amount`, `text`).
  - Projection at the existing site next to `pv.Pool = poolView(p.Pool)`:
    `pv.PoolRestrictions = poolRestrictions(g, p.RestrictedMana)` — filled for
    every seat under every visibility, exactly like `Pool`.
- **`view/poolrestriction.go`** (new) — `poolRestrictions` projects each batch
  (skipping `Valid == ""` batches, which impose no spend limit), and
  `humanizeRestrictValid`/`humanizeRestrictTerm`/`humanizeSpec` render the
  common shapes: `Spell.<spec>` → "spend only to cast <spec>", `Activated.<spec>`
  → "spend only to activate <spec>", bare `Spell`/`Activated`/`nonSpell`, with
  card types/supertypes/colours recognised. `ChosenType` resolves against
  `g.Obj(batch.Source).ChosenType`. Anything unrecognised (e.g. `MultiColor`,
  `wasCastFromYourHand`) makes the WHOLE Valid$ fall back to the raw string
  rather than a half-prosed mixture. Multi-term Valid$ (OR semantics) joins
  with " or ". `view/` does not import `rules/`.
- **`view/pool_restriction_test.go`** (new) — the projection pins (see below).
- **`view/projection_closure_test.go`** — added `pool_restrictions: true` to
  the `PlayerView` JSON-key allowlist with the public-fact rationale. This is
  the D6 god-view gate firing correctly on a new key; the field is public, like
  `available` and `pool`, so it belongs in the allowlist, not behind a gate
  exemption.
- **`web/src/components/ManaPool.svelte`** — new optional
  `poolRestrictions` prop, rendered as persistent text (`data-mana-restrictions`,
  `data-mana-restriction="<sym>"`) inside the pool group, beside the chips, per
  the component's own B1 persistent-text contract. Outer and pool-group
  conditions extended to draw the annotation even when the pool map is empty.
  Absent/empty/null draws exactly the old markup.
- **`web/src/components/{IdentityBar,Rail,SeatPanel}.svelte`** — thread
  `player.pool_restrictions` / `focused.pool_restrictions` / `mine.pool_restrictions`.
- **`web/src/components/ManaPool.svelte.test.ts`** — rendering pins (below).
- **`web/src/protocol.ts`** — regenerated via `go run ./cmd/gentypes` (not
  hand-edited); `PoolRestrictionView` and `pool_restrictions?` appear.

Engine (`rules/`), `state/`, `events/`, heads and goldens are untouched.

## Required tests added

- `TestRestrictedManaIsProjected` (`view/`) — the replayed snapshot shape: one
  `{B}` batch `Valid:Spell.Creature+ChosenType` with source `ChosenType:"Demon"`
  plus one bare `{B}` (the control). Asserts preconditions first: the batches
  really sit in `g.Players[0].RestrictedMana`, and the produced text really
  differs from the raw Valid$ fallback (so a formatter that fell through
  fails). Expects "spend only to cast a Demon creature spell" for the
  restricted batch, nothing for the bare one, and a second `Spell.Creature`
  seat expecting "spend only to cast a creature spell".
- `TestRestrictedManaExoticValidFallsBackRaw` — raw fallback floor and the
  `Spell.Instant,Spell.Sorcery` "or" join.
- `TestPoolRestrictionsOmittedWhenEmpty` — a seat with no restricted mana
  serialises no `pool_restrictions` key and carries a nil slice.
- `TestPoolRestrictionIsPublicForEveryViewer` — projected for every seat under
  Seat/Public/Omniscient and every viewer, like `Pool`.
- `ManaPool.svelte.test.ts` — annotation is persistent markup keyed by colour;
  one annotation per batch not per symbol; absent/null/empty renders
  **byte-identically** to the pre-change output; a blank-text entry draws
  nothing.

## Gates run (real output)

Required view command:

```
$ go test -run 'TestRestrictedManaIsProjected|TestCR106ManaPoolIsPublicForEveryPlayer' ./view/ 2>&1 | tail -30
ok  	github.com/adams-shaun/gorge/view	(cached)
```

Full `view` package (I edited it; run once):

```
$ go test ./view/
ok  	github.com/adams-shaun/gorge/view	0.807s
```

Web rendering pin:

```
$ cd web && npm test -- src/components/ManaPool.svelte.test.ts
 Test Files  1 passed (1)
      Tests  18 passed (18)
```

Wire types:

```
$ go run ./cmd/gentypes && go run ./cmd/gentypes -check
OK
```

Format:

```
$ gofmt -l view/*.go
(no output)
```

Behaviour goldens outside `rules/`:

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	(cached)

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.155s
```

Extra (not required by the brief; done because I threaded three components):
`cd web && npm run check` → `0 errors and 1 warning` (the warning is a
pre-existing `ResolvedCard.svelte` state-capture warning, unrelated).

## Fails without the fix

Each new test was shown to fail with the non-test change reverted; the file was
then restored and `cmp`-verified byte-identical.

(1) Projection call removed from `view/view.go` (`pv.PoolRestrictions = …`
deleted):

```
--- FAIL: TestRestrictedManaIsProjected (0.00s)
    pool_restriction_test.go:83: seat 0 projects 0 pool restrictions, want 1 (the bare {B} carries no limit): []
--- FAIL: TestRestrictedManaExoticValidFallsBackRaw (0.00s)
    pool_restriction_test.go:159: projects 0 restrictions, want 1
--- FAIL: TestPoolRestrictionIsPublicForEveryViewer (0.00s)
    pool_restriction_test.go:203: visibility seat viewer 0: seat 0 projects 0 restrictions, want 1
FAIL
FAIL	github.com/adams-shaun/gorge/view	0.002s
```

(2) Humanizer forced to return the raw Valid$ (`return valid` at the top of
`humanizeRestrictValid`):

```
--- FAIL: TestRestrictedManaIsProjected (0.00s)
    pool_restriction_test.go:90: precondition: the projection fell through to the raw Valid$ string instead of humanising it
--- FAIL: TestRestrictedManaExoticValidFallsBackRaw (0.00s)
    pool_restriction_test.go:179: two-alternative text = "Spell.Instant,Spell.Sorcery", want "spend only to cast an instant spell or cast a sorcery spell"
FAIL
FAIL	github.com/adams-shaun/gorge/view	0.002s
```

(3) Restriction-rendering block removed from `ManaPool.svelte`:

```
FAIL  src/components/ManaPool.svelte.test.ts > ManaPool > renders one annotation per restricted batch, not one per pool symbol
AssertionError: expected '...data-mana-pool="" ...' to contain 'data-mana-restriction="B"'
 Test Files  1 failed (1)
      Tests  2 failed | 16 passed (18)
```

Restoration was `cmp`-verified (`RESTORED`, `RESTORED2`,
`RESTORED-BYTE-IDENTICAL`).

## Brief premise check

The brief's corpus prevalence was re-measured and held exactly:

```
$ /usr/bin/grep -rlE 'AB\$ Mana.*RestrictValid\$' .cards/cardsfolder | wc -l
167
$ /usr/bin/grep -rhE 'AB\$ Mana.*RestrictValid\$' .cards/cardsfolder | wc -l
170
```

The feedback snapshot directory exists at the path the brief gave.

## Deviations from the brief

- **`view/projection_closure_test.go` allowlist edit.** The brief did not
  mention this file, but `TestViewMarshalsClosed` fails the build on any new
  `PlayerView` JSON key. `pool_restrictions` is a public fact (same class as
  `available`/`library_top`), so I added it to the allowlist with a rationale
  comment rather than exempting the test. This is the gate doing its job, not a
  widened condition.
- **Batches with an empty `Valid` are omitted.** The brief says "one entry per
  batch". A batch with `Valid == ""` (an `AddsNoCounter$`-only batch, e.g.
  Boseiju) carries pool provenance but no spend limit; annotating it would
  mislead. It is skipped, so `PoolRestrictions` is one entry per *restricted*
  batch. Recorded in the field doc and here.
- **Raw-string fallback is all-or-nothing per Valid$.** A mixed Valid$ where
  one alternative is exotic returns the raw whole string, not a partial
  prose/raw join. Named in the humanizer doc.
- **`ManaPool.svelte` outer condition widened** to `|| restrictions.length > 0`
  so the annotation draws if restriction state ever outlives its pool chips.
  With an empty/absent restriction list the markup is byte-identical to before.

## Issues

1. **Follow-up (ticket-worthy): annotate the any-colour decision prompt at
   production time.** The restriction is only visible AFTER the mana is
   produced. `rules/mana.go`'s `askManaColor` path poses "Choose a colour of
   mana" / Cavern's "Add one mana of any colour" with no indication that the
   result will be restricted. Annotating that prompt (engine-side decision
   text) would tell the player before they commit, but it changes decision text
   mid-chain and moves goldens, so it is out of scope here. This is the
   follow-up the brief's scope boundary names.
2. **`ChosenType` resolution is live, not LKI.** If the producing permanent has
   left the battlefield or recorded no `ChosenType` by the time the view
   projects, the text degrades to "a creature spell of the chosen type"
   (`humanizeSpec`). The restriction itself also fails closed in that case
   (rules-side), so the annotation is honest but cannot name the type. Not a
   defect in this change; noting it.
3. **No CR-lane test proposed.** This is a `view/` projection with no CR-rule
   behaviour change; the engine admission semantics are already pinned in
   `rules/paramcensus_targeting_type_choice_test.go`. No new conformance-lane
   test is warranted.

No Known-approximations row was added, grown, or closed (this is a new surface,
not a row's remainder); `knownApproximationRows` is unchanged.

---

# Merged concurrent report: cli-20260923T060000Z-rv2b-countheads

# Report — cli-20260923T060000Z-rv2b-countheads

## What changed and why

Ticket scope: the `<Ref>$<Property>` count heads the rv2b row named as
evaluating to zero — `CastTotalManaSpent`, `LifeTotal`, `CardCounters.ALL`,
`CardCounters.AGE`, `CardNumColors` — plus (this ticket owns it) the AGENTS.md
row deletion.

### Brief premise re-measured first (a claim, not a measurement)

The dispatch's system notes say a brief's counts are claims. I measured every
named head against the current tree before writing code, with a probe test
(`go test -run 'TestRefProperty' ./effects/` against unmodified `count.go`):

| named head | state at HEAD | evidence |
|---|---|---|
| `CastTotalManaSpent` (ref-property) | **already reads real state** | the neighbouring ref-head ticket landed it (`840adbc6`, an ancestor of HEAD) |
| `CardCounters.ALL` (ref-property) | **already reads real state** | `441f7af7` ("read the Count$Valid $CardCounters.<KIND> summed property"), ancestor |
| `CardCounters.AGE` (ref-property) | **already reads real state** | AGE is a REAL counter kind: `rules/cumulative.go:268` emits `CounterChange Counter:"AGE"`, so `o.Counter("AGE")` answers |
| `CardNumColors` (ref-property) | **zero — genuinely missing** | no case in `evalRefProperty` |
| `LifeTotal` (player ref) | **zero — genuinely missing** | `evalPlayerRefProperty`'s ref switch does not know `TriggeredTarget`/`TriggeredPlayer`/`TriggeredDefendingPlayer` |

So the brief's list was stale for three of five. The genuinely-open work was
`CardNumColors` and `LifeTotal`, and that is what I implemented. The probe
test for the already-working three is kept as coverage (it passes both with
and without my change; it is evidence they were never the gap).

### `effects/count.go`

1. **`evalRefProperty`, new `case prop == "CardNumColors"`** — adds
   `int32(len(h.ObjectColors(o)))` per referenced object. `h.ObjectColors` is
   the SAME read the plain `Count$CardNumColors` head uses (live layer-5
   colours on the battlefield, the printed face elsewhere, including the LKI
   snapshot the loop already swaps in), so the two spellings cannot disagree.
   Corpus carriers: `Lurking Spinecrawler`, `Moonveil Regent`, `Mana Cannons`
   (`TriggeredCard$CardNumColors` / `Targeted$CardNumColors`).

2. **`evalPlayerRefProperty` default ref case** — the hand-list ref switch now
   falls through to `effects/context.go`'s shared `definedSpec` resolver (the
   same resolver a `Defined$` spelling goes through) and keeps its player
   entries. This is the structural fix the dispatch asks for: the next
   `Defined$`-resolved player ref is covered without editing a second list.
   **The property is confined to `LifeTotal`** (the one player head this
   ticket names), so the wider ref set cannot silently widen the family's
   other player count semantics. The `/Op` suffix is cut before the switch so
   the gate can inspect the bare property.
   Corpus carriers now correct: `TriggeredTarget$LifeTotal` (13 files —
   Quietus Spike, Ebonblade Reaper), `TriggeredPlayer$LifeTotal` (1),
   `TriggeredDefendingPlayer$LifeTotal` (1).

### `internal/testutil/agentsdoc_test.go`

`knownApproximationRows` **22 → 21**, comment updated, because this ticket
deletes the drained rv2b row (see below).

### `AGENTS.md`

Deleted the `(rv2b)` "Three fail-closed damage remainders" row by its text
(NOT renumbered). Both siblings are ancestors of HEAD on this branch:
`cli-20260923T060000Z-rv2b-damagesource` (`2acd1d4d`) and
`cli-20260923T060000Z-rv2b-validplayers` (`dbe11453`), and their commit
subjects name their sub-shapes, so the whole row is closed. Verified with
`git merge-base --is-ancestor`.

### `effects/count_ref_property_rv2b_test.go` (new)

All new tests live in a NEW ticket-named file (per the "new tests go in a new
file" rule). Covers: the object `CardNumColors` head (`Remembered$`,
`Targeted$` spellings), the player `LifeTotal` head (plain and `/HalfUp`,
`TriggeredDefendingPlayer`), plus the three already-working heads
(`CardCounters.AGE`/`.ALL`, `CastTotalManaSpent`) as regression coverage, and
two REAL-corpus tests reading the compiled SVars of **Moonveil Regent** and
**Quietus Spike**.

## Gates run (real output pasted)

Targeted acceptance test (brief's acceptance shape):

```
$ go test -run 'TestRefProperty' ./effects/
ok  	github.com/adams-shaun/gorge/effects	0.605s
```

Full package I edited (once, at the end):

```
$ go test ./effects/
ok  	github.com/adams-shaun/gorge/effects	3.412s
```

agentsdoc ratchet (row count now 21):

```
$ go test -run 'TestKnownApproximation' ./internal/testutil/
ok  	github.com/adams-shaun/gorge/internal/testutil	(cached)
```

Behaviour goldens outside `rules/` (run once, before reporting):

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.437s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.161s
```

Lint Go half:

```
$ gofmt -l effects/count.go effects/count_ref_property_rv2b_test.go internal/testutil/agentsdoc_test.go
(no output)
$ go run ./cmd/gentypes -check
(no output, exit 0)
```

Corpus present (a green run is not a skipped one): `ls .cards | head` is
non-empty and the corpus-backed tests took 0.6s (a skipped run is ~2ms), so
the corpus was really exercised.

Heads / ratchets: the botbench 20-game split is byte-identical, so no re-pin
is needed; **TestHeads was NOT run in-seat** (daemon gate — the brief says
skip it), so this report does not claim chain-head movement in either
direction. One ratchet moved: `knownApproximationRows` 22 → 21, the rv2b row
deletion.

## Fails without the fix

I backed `effects/count.go` up (`cp`), restored HEAD's version over it
(`git show HEAD:effects/count.go`), ran the one test, and restored the fixed
file byte-identically (`cmp` confirmed). No `git stash`, no
`git checkout <path>`.

```
$ go test -run 'TestRefProperty' ./effects/
--- FAIL: TestRefPropertyCardNumColorsReadsTheReferencedObject (0.00s)
    count_ref_property_rv2b_test.go:32: Remembered$CardNumColors = 0, want 3 (the referenced card's colours)
    count_ref_property_rv2b_test.go:38: Targeted$CardNumColors = 0, want 3
--- FAIL: TestRefPropertyLifeTotalReadsTheReferencedPlayer (0.00s)
    count_ref_property_rv2b_test.go:60: TriggeredTarget$LifeTotal = 0, want 13
    count_ref_property_rv2b_test.go:63: TriggeredTarget$LifeTotal/HalfUp = 0, want 7
    count_ref_property_rv2b_test.go:70: TriggeredDefendingPlayer$LifeTotal = 0, want 4
--- FAIL: TestRefPropertyCardNumColorsReadsTheRealCorpusSVar (0.57s)
    count_ref_property_rv2b_test.go:151: real Moonveil Regent SVar = 0, want 3
--- FAIL: TestRefPropertyLifeTotalReadsTheRealCorpusSVar (0.00s)
    count_ref_property_rv2b_test.go:172: real Quietus Spike SVar = 0, want 7 (half of 13, rounded up)
FAIL
FAIL	github.com/adams-shaun/gorge/effects	0.588s
FAIL
```

The `CardCounters` / `CastTotalManaSpent` tests in the same file PASS with the
fix reverted — they are regression coverage for already-working heads, not the
fix, and are labelled as such.

Each new test asserts its own precondition: the referenced card is
three-colour (and the source is not), the two players' life totals differ,
the fixture carries 3 AGE + 2 P1P1 counters, the other fixture carries 0, the
referred object carries no cast spend. A vacuous setup fails loudly.

## Deviations from the brief, with reasons

1. **`LifeTotal` is implemented in `evalPlayerRefProperty`, not literally in
   `evalRefProperty`.** `evalRefProperty`'s object loop `continue`s every
   `IsPlayer` target and its property switch is object-only, so a player
   `LifeTotal` structurally cannot live there. `evalPlayerRefProperty` is the
   family's designated player-valued half; the brief's "in `evalRefProperty`"
   names the family, and the code comment says where it actually landed.

2. **The player-ref property is confined to `LifeTotal`.** I resolved the REF
   structurally (shared `definedSpec`, so no second hand-list) but gated the
   property, because activating the other already-implemented player heads
   (`TriggeredTarget$CardsInHand`, `$Valid`, `$Counters.Poison`,
   `$LifeLostThisTurn`, …) would be "a global count semantic" change beyond
   the brief's named heads. See Issues for the residual.

3. **I initially overwrote the tracked shared test file
   `effects/count_ref_property_test.go`** (its `TestRefPropertyCounts` asserts
   `TriggeredTarget$LifeTotal == 0`, pinned by an earlier ticket). I caught
   this, restored the file byte-for-byte from HEAD, and moved all my tests to
   the new `effects/count_ref_property_rv2b_test.go`. The final diff touches
   NO shared test file (`git status` shows only the new file, untracked at
   first, plus the four changed files). The existing `TestRefPropertyCounts`
   still passes unchanged — its fixture binds `TriggeredTarget` to object
   targets, so `LifeTotal` is legitimately 0 there. Its comment ("An unknown
   ref or property stays zero") is now slightly imprecise for that one line,
   but the value is unchanged and I did not edit a shared file to reword it.

## Open concerns

- The `LifeTotal` gate (`prop != "LifeTotal"`) is a deliberate scope fence.
  If a future ticket drains the rest of the player-ref property family it
  should replace the gate with the full table, not add another fence.
- Botbench is byte-identical, so no re-pin is needed; TestHeads is unmeasured
  in-seat (daemon gate).

## Issues (defects found, not fixed)

1. **The rest of the player-ref `<Ref>$<Property>` family is still zero for
   the `Defined$`-resolved refs.** `TriggeredPlayer$CardsInHand` (10 corpus
   files), `TriggeredTarget$CardsInHand` (4), `TriggeredTarget$Valid` (3),
   `TriggeredDefendingPlayer$CardsInHand` (2), `TriggeredDefendingPlayer$CardsInLibrary` (2),
   `TriggeredCardController$ValidGraveyard` (2), `TriggeredTarget$Counters.Poison` (2),
   `TriggeredTarget$LifeLostThisTurn`, `TriggeredTarget$CardsInLibrary`,
   `TriggeredDefendingPlayer$ValidExile`, `TriggeredCardController$Valid`,
   `TriggeredTarget$Counters.RAD` (1 each). The readers all exist in
   `evalPlayerRefProperty`; they are unreachable only because the ref gate is
   confined to `LifeTotal` (deviation 2). A one-line lift of the gate would
   close them, but that is a wider count-semantic change than this ticket
   authorizes and needs its own ticket + botbench measurement. File/func:
   `effects/count.go` `evalPlayerRefProperty` (the `default:` ref case).
   A CR-lane test citing CR 107.3 / 608.2 (a count over a referenced player)
   would make this visible to the ledger.

2. **`PlayerCountRemembered$LifeTotal` (14 corpus files) is entirely
   unimplemented.** `PlayerCountRemembered` appears nowhere outside tests in
   `effects/`/`rules/` (`grep -rn 'PlayerCountRemembered' effects rules` →
   none). Same for `PlayerCountDefinedTriggeredSourceController$LifeTotal`,
   `PlayerCountDefinedTriggeredCardOwner$LifeTotal`,
   `PlayerCountDefinedActivePlayer$LifeTotal` (1 file each). This is the
   `PlayerCount<group>$<property>` head family, a different head from this
   ticket's `<Ref>$<Property>`, so out of scope. It deserves its own ticket;
   a CR-lane test citing CR 107.3 would surface it.

3. **`effects/count_ref_property_test.go`'s comment** labels
   `TriggeredTarget$LifeTotal` under "An unknown ref or property stays zero".
   The ref/property is no longer unknown; the value is 0 only because that
   fixture binds `TriggeredTarget` to object targets. Cosmetic; left as-is to
   avoid editing a shared file.

## Commit

`a62d152c` — `fix(effects): resolve the <Ref>$<Property> CardNumColors and LifeTotal count heads`

---

# Report — Vote.StoreVoteNum

Implemented fixed-choice `StoreVoteNum$` outcomes and pinned Fateful Tempest against the real corpus.

- `effects/misc.go`: fixed-list Vote now uses a shared outcome resolver. Without `StoreVoteNum$`, it preserves the prior winner/tie behavior. With `StoreVoteNum$ True`, each choice body runs with its own `VoteNum` binding in a private copy of the source SVar table; this avoids mutating the card face's shared SVar map and lets each body consume its tally.
- `rules/fateful_tempest_vote_test.go`: added an end-to-end real-corpus test with two votes for each option. It verifies two Mountains are milled and two exiled, proving both SVar bodies read their own count, and checks replay.

`.cards/` was present as a symlink to `/home/sadams/projects/gorge/.cards`; the corpus test did not skip. The measured `StoreVoteNum` prevalence is **13 files**, matching the brief. The worktree was clean before the required `git rebase main`, which reported up to date.

## Verification

Targeted real-corpus regression:

```text
$ go test -run '^TestFatefulTempestStoresEachOptionVoteCount$' ./rules/ > .ds4/scratch/t.log 2>&1; rc=$?; tail -40 .ds4/scratch/t.log; exit $rc
ok   github.com/adams-shaun/gorge/rules  0.603s
```

Architecture golden:

```text
$ go test ./internal/archtest/ 2>&1 | tail -15
ok   github.com/adams-shaun/gorge/internal/archtest  3.864s
```

Constructed-default golden:

```text
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -5
ok   github.com/adams-shaun/gorge/cmd/botbench  1.364s
```

Formatting and generated types:

```text
$ gofmt -l effects/misc.go rules/fateful_tempest_vote_test.go; go run ./cmd/gentypes -check
[no output; exit 0]
```

Diff check and corpus measurement:

```text
$ git diff --check; grep -rlE 'StoreVoteNum' .cards/cardsfolder | wc -l
13
```

## Fails without the fix

Copied `effects/misc.go` to `.ds4/scratch/misc.go.fixed`, disabled only the `StoreVoteNum$` branch in `resolveVoteOutcomes`, and ran the new test. It failed on the first observable tally-dependent effect; restored the source from the copy and verified byte identity with `cmp` (`cmp=0`).

```text
$ go test -run '^TestFatefulTempestStoresEachOptionVoteCount$' ./rules/ > .ds4/scratch/t-no-fix.log 2>&1; rc=$?; tail -30 .ds4/scratch/t-no-fix.log; test $rc -ne 0
--- FAIL: TestFatefulTempestStoresEachOptionVoteCount (0.59s)
    fateful_tempest_vote_test.go:90: precondition/result: two past votes must mill two Mountains, got 0
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.608s
FAIL
```

## Issues

The existing no-host R-9 fallback still resolves a Vote decision with the deterministic first ballot option; changing that host-degradation behavior was outside this StoreVoteNum task. No new CR-lane finding or Known approximations row was added.

`.cards` existed as `/home/sadams/projects/gorge/.cards` (resolved target `/home/sadams/projects/gorge/.cards`).

- `git status --short && git log -1 --oneline && git rebase main`
  ```text
  c5669fdf merge(cli-20260923T060000Z-choose-number): approx: effChooseNumber never asks mid-resolution and always chooses 0
  Current branch wt/agent-20260918T233200Z-b598c141 is up to date.
  ```
- `go test -run 'TestPlayerCountPropertyYou' ./effects/`
  ```text
  ok   github.com/adams-shaun/gorge/effects  0.003s
  ```
- `go test -run '^TestSacrificesThisTurnCountsOwnedPermanentsAndResets$' ./rules/`
  ```text
  ok   github.com/adams-shaun/gorge/rules  0.003s
  ```
- `go test ./internal/archtest/`
  ```text
  ok   github.com/adams-shaun/gorge/internal/archtest  3.478s
  ```
- `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/`
  ```text
  ok   github.com/adams-shaun/gorge/cmd/botbench  1.281s
  ```
- `go run ./cmd/gentypes -check`: passed, no output.
- `gofmt -l effects/count.go effects/registry.go effects/context_test.go effects/count_compare_test.go effects/playercount_property_you_test.go rules/stack.go rules/sacrifices_this_turn_test.go`: passed, no output.
- `git diff --check`: passed, no output.

## Fails without the fix

For each new test, I copied its non-test implementation file to `.ds4/scratch`, removed the implementation, ran the targeted test, restored the file from the copy, and confirmed it byte-identical with `cmp`.

- Effects test, with the four dispatch cases removed from `effects/count.go`:
  ```text
  exit=1
  --- FAIL: TestPlayerCountPropertyYouPerTurnLedgerCounts (0.00s)
      playercount_property_you_test.go:29: SacrificedThisTurn = (0, false), want (1, true)
      playercount_property_you_test.go:29: LifeLostThisTurn = (0, false), want (3, true)
      playercount_property_you_test.go:29: LandsPlayed = (0, false), want (2, true)
      playercount_property_you_test.go:47: controller 1 SacrificedThisTurn = (0, false), want (3, true)
      playercount_property_you_test.go:47: controller 1 LifeLostThisTurn = (0, false), want (7, true)
      playercount_property_you_test.go:47: controller 1 LandsPlayed = (0, false), want (0, true)
  FAIL
  ```
  (The pre-existing `CardsDiscardedThisTurn` dispatch remained in place, so its assertion correctly continued to pass.)
- Rules test, with `Engine.SacrificesThisTurn` removed from `rules/stack.go`:
  ```text
  # github.com/adams-shaun/gorge/rules [github.com/adams-shaun/gorge/rules.test]
  rules/attack_cost.go:106:30: cannot use e (variable of type *Engine) as effects.Host value in argument to effects.EvalCountOK: *Engine does not implement effects.Host (missing method SacrificesThisTurn)
  rules/attack_cost.go:109:30: cannot use e (variable of type *Engine) as effects.Host value in argument to effects.EvalCountOK: *Engine does not implement effects.Host (missing method SacrificesThisTurn)
  rules/attack_cost.go:205:32: cannot use e (variable of type *Engine) as effects.Host value in argument to effects.EvalCountOK: *Engine does not implement effects.Host (missing method SacrificesThisTurn)
  rules/attack_cost.go:212:33: cannot use e (variable of type *Engine) as effects.Host value in argument to effects.EvalCountOK: *Engine does not implement effects.Host (missing method SacrificesThisTurn)
  rules/cast.go:3248:35: cannot use e (variable of type *Engine) as effects.Host value in argument to effects.CharmRandomChosen: *Engine does not implement effects.Host (missing method SacrificesThisTurn)
  rules/cast.go:3304:37: cannot use e (variable of type *Engine) as effects.Host value in argument to effects.CharmEligibleModes: *Engine does not implement effects.Host (missing method SacrificesThisTurn)
  rules/cast.go:3305:46: cannot use e (variable of type *Engine) as effects.Host value in argument to effects.CharmModeBounds: *Engine does not implement effects.Host (missing method SacrificesThisTurn)
  rules/cast.go:7286:35: cannot use e (variable of type *Engine) as effects.Host value in argument to effects.EvalCountOK: *Engine does not implement effects.Host (missing method SacrificesThisTurn)
  rules/control.go:107:35: cannot use e (variable of type *Engine) as effects.Host value in argument to effects.ControlGrantEnded: *Engine does not implement effects.Host (missing method SacrificesThisTurn)
  rules/control_static.go:194:32: cannot use e (variable of type *Engine) as effects.Host value in argument to effects.ControlGrantEnded: *Engine does not implement effects.Host (missing method SacrificesThisTurn)
  rules/control_static.go:194:32: too many errors
  FAIL	github.com/adams-shaun/gorge/rules [build failed]
  FAIL
  ```

## Issues

- Sacrifice events do not carry an actor/player field. `rules/stack.go` therefore attributes `SacrificesThisTurn` by the permanent's owner; this is replay-stable but does not distinguish a permanent sacrificed by a different controller. The event shape or a separate provenance mechanism would be needed for exact actor attribution. Report this limitation for follow-up; the known-approximation register was not changed.
- The full Evendo Brushrazer may-play path remains inert because `Affected$ Card.ExiledWithSource` is not handled here. Raw `Card.ExiledWithSource` text occurs in 100 `.cards/cardsfolder` scripts; this ticket does not implement that predicate, add the Evendo static-gate test, or move the deck ratchet.
- The remaining unimplemented `PlayerCountPropertyYou$` shapes include `HasPropertyBeenAttackedThisCombat` (15 occurrences), `OpponentsAttackedThisCombat` (5), `AttractionsVisitedThisTurn` (4), and `DamageThisTurn` (3), plus the rarer `Valid` (2), `OpponentsAttackedThisTurn` (2), `LifeLostLastTurn` (2), `SacrificedPermanentTypesThisTurn` (1), `PlaneswalkedToThisTurn` (1), `HasPropertyNotedForBattleUgin` (1), `HasPropertyNotedForBattleBolas` (1), `HasPropertyMaxSpeed` (1), `ExploredThisTurn` (1), `DomainPlayer` (1), `DamageToOppsThisTurn` (1), and `BeenDealtCombatDamageSinceLastTurn` (1). Combat-history and other state-specific semantics were outside this ticket.

## Commit

`66ae9f21 fix(count): resolve per-turn player property counts`

---

# Report — NonRememberedController selectors


## What changed and why

- `effects/context.go`: `definedSpec` now recognises `NonRememberedController` and `OppNonRememberedController`, returning living players in stable `AliveFrom` order other than the remembered card's controller. The Opp form additionally excludes the resolving controller. A missing/invalid remembered card anchor is a recognised empty set, so `Defined` does not fall back to the source.
- `effects/copypermanent.go`: `Controller$` accepts the same selectors and mints one copy per resolved player. Copy destinations are expanded in deterministic owner-then-target order; an empty set mints nothing. The existing one-copy controller path is unchanged.
- `effects/nonremembered_controller_test.go`: added real-corpus pins for Fractured Identity's CopyPermanent rider and Plaguecrafter's Discard rider. They assert the remembered-controller exclusion, the Opp-qualified set, an empty unbound set, no unsupported-selector Note, and that each expected Plaguecrafter player actually discards while the remembered card's controller does not.

The selector resolution is shared through `definedSpec`, so the next Defined$ carrier (LoseLife or another Discard card) uses the same implementation rather than a carrier-specific selector list. CopyPermanent uses that resolver for its multi-controller loop.

Closed ledger issue: `issue-agent-20260920T074357Z-b9ac41c2`.

`.cards` was present as a symlink to `/home/sadams/projects/gorge/.cards`.

## Fails without the fix

Copied the two changed non-test files to `.ds4/scratch/`, temporarily disabled both selector cases, ran the new tests, restored both files, and confirmed both `cmp` checks passed (`restore_cmp=0`). The test command failed as required:

```text
test_exit=1 restore_cmp=0
--- FAIL: TestFracturedIdentityGivesEveryOtherPlayerACopy (0.69s)
    nonremembered_controller_test.go:43: Fractured Identity copy owners = [1 0 0 0], want [1 1 0 1]; events=[{Seq:0 Kind:note Player:0 Obj:1 From:library To:library Amount:0 Step:untap Counter: Text:Controller$ NonRememberedController is not implemented; the copy is controlled by the resolving controller IDs:[]} {Seq:0 Kind:copy_token Player:0 Obj:2 From:library To:library Amount:0 Step:untap Counter: Text: IDs:[]} {Seq:0 Kind:move_zone Player:0 Obj:3 From:library To:battlefield Amount:0 Step:untap Counter: Text: IDs:[]}]
--- FAIL: TestNonRememberedControllerDefinedPlayers (0.00s)
    nonremembered_controller_test.go:77: Plaguecrafter Defined$ NonRememberedController = [{1 0 false}]; want 3 live players

FAIL
FAIL    github.com/adams-shaun/gorge/effects  0.724s
```


## Gates

Focused tests:

```text
go test -run 'TestFracturedIdentityGivesEveryOtherPlayerACopy|TestNonRememberedControllerDefinedPlayers' ./effects
ok   github.com/adams-shaun/gorge/effects  0.641s
```

Build:

```text
go build ./...
[no output; exit 0]
```

Chain-head golden:

```text
go test -run 'TestHeads' ./rules
ok   github.com/adams-shaun/gorge/rules  1.830s
```

Required behavior goldens:

```text
go test ./internal/archtest/
ok   github.com/adams-shaun/gorge/internal/archtest  (cached)

go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok   github.com/adams-shaun/gorge/cmd/botbench  1.245s
```

Formatting/type generation:

```text
gofmt -l effects/context.go effects/copypermanent.go effects/nonremembered_controller_test.go
[no output]
go run ./cmd/gentypes -check
[no output; exit 0]
```

`git diff --check` passed. `TestHeads` and the botbench golden did not move. No acceptance-ratchet or replay-head changes were made.

## Issues

None found outside the requested selector family; no unaddressed issues.

## Commit

`bfca5670 fix(effects): resolve nonremembered controller selectors`

---

# Vanishing implementation report

## Changes

- `cards/kw_vanishing.go`: registered a `Vanishing` expander for the scoped `Vanishing:<N>` form. It adds a battlefield-entry TIME-counter replacement, a controller-upkeep Phase trigger gated on at least one TIME counter, and a separate battlefield `CounterRemoved` trigger gated on the post-removal count being zero. Removal and sacrifice are ordinary triggered effects; bare `K:Vanishing` is deliberately not treated as a count-bearing form.
- `cards/kw_registry_test.go`: added `Vanishing` to the expander registry ratchet.
- `cards/kw_vanishing_test.go`: verifies expansion into entry replacement plus the upkeep and last-counter triggers.
- `rules/vanishing_test.go`: corpus-backed Deep Forest Hermit tests assert battlefield placement and its three entry counters; verify upkeep removal resolves through the stack and the last-counter sacrifice is a separate trigger; check another player's upkeep and zero-counter upkeep do not tick or queue the Vanishing trigger.

`.cards` was present (not skipped). Measured 21 corpus files containing `K:Vanishing`; of those script lines, 19 use `K:Vanishing:<N>` and two are bare `K:Vanishing` (Out of Time and Tidewalker). Repo-deck ratchets and heads were not edited. No Known approximations row was closed or changed.

## Fails without the fix

Copied `cards/kw_vanishing.go` to `.ds4/scratch/kw_vanishing.go.fixed`, removed the production expander, ran the required targeted command, then restored and verified the file byte-identically:

```text
exit=1
--- FAIL: TestEveryExpandedKeywordHasAnExpander (0.00s)
    kw_registry_test.go:84: keyword head "Vanishing" has no registered expander: it silently stops expanding
--- FAIL: TestVanishingExpansion (0.00s)
    kw_vanishing_test.go:9: Vanishing entry replacement = [], want ETB placement of 3 TIME counters
FAIL
FAIL	github.com/adams-shaun/gorge/cards	0.002s
--- FAIL: TestVanishingDeepForestHermitUpkeepAndLastCounter (0.58s)
    vanishing_test.go:38: precondition: Deep Forest Hermit entered with 0 TIME counters, want 3
--- FAIL: TestVanishingOnlyTriggersOnControllersUpkeepAndNotAtZero (0.00s)
    vanishing_test.go:75: precondition: Deep Forest Hermit entered with 0 TIME counters, want 3
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.607s
restored byte-identically
```

## Verification

Targeted command:

```text
go test -run 'TestEveryExpandedKeywordHasAnExpander|TestVanishing' ./cards/ ./rules/
exit=0
ok  github.com/adams-shaun/gorge/cards  0.002s
ok  github.com/adams-shaun/gorge/rules  0.619s
```

Architecture gate:

```text
go test ./internal/archtest/
exit=0
ok  github.com/adams-shaun/gorge/internal/archtest  (cached)
```

Botbench golden:

```text
go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
exit=0
ok  github.com/adams-shaun/gorge/cmd/botbench  1.231s
```

Formatting/type generation checks:

```text
gofmt -l cards/kw_vanishing.go cards/kw_vanishing_test.go cards/kw_registry_test.go rules/vanishing_test.go
(no output)
go run ./cmd/gentypes -check
(no output; exit 0)
```

`git diff --check` passed with no output. No botbench split movement.

## Issues

- Bare `K:Vanishing` appears on 2 corpus cards (Out of Time and Tidewalker). It has no `<N>` count and is outside this ticket's explicitly scoped `Vanishing:<N>` script shape; the expander intentionally returns without inventing behavior for it. A follow-up must define the intended bare-keyword semantics before implementing it.

## Commit

`3f4072f3 feat(cards): implement Vanishing time counters`

---

# RepeatEach honours RepeatOptionalForEachPlayer$ (rpteachopt1)

## What changed and why

`RepeatEach` ignored `RepeatOptionalForEachPlayer$` / `RepeatOptionalMessage$`
(seven corpus files: Tempt with Vengeance, Tempt with Reflections, Tempt with
Glory, Tempt with Bunnies, Tempt with Mayhem, Tempting Contract, Zagorka,
Mother of Sanctum). It selected its subjects and then ran every subject's body
unconditionally. The brief's root cause was confirmed: no production read of
either parameter existed.

The fix poses one yes/no election per subject before that subject's body runs;
a yes runs the body once, a no skips only that subject and the loop continues.

### `effects/registry.go`
- `RepeatCursor` gains `Election bool` (mark a cursor parked on an election,
  not a completed body).
- New `RepeatEachOptionalContinuation{Next int32; Accept bool}` — the scoped
  answer of one subject's offer, distinct from `RepeatOptionalContinuation`
  (the `Repeat` do/while's own election). The two cursors are never conflated.
- `Ctx.RepeatEachOptional *RepeatEachOptionalContinuation`.

### `effects/choose_control.go`
- `effRepeatEach` reads `RepeatOptionalForEachPlayer$`/`RepeatOptionalMessage$`
  and, in the subject loop, offers subjects that have not been answered yet.
- New `poseRepeatEachElection`: builds the `KChoose` yes/no decision (player =
  `PlayerOf` the subject, prompt = the message, `ResumeKind
  "repeat_each_optional"`, `ResumeSA` = the RepeatEach SA), calls the shared
  `Ask` boundary, and on a suspended ask parks the loop through the EXISTING
  `SuspendRepeat` payload (subjects, `Next`, `Outer`, `Chosen`, `VoteCounts`,
  `Election: true`). R-9: `AskNoHost`/`AskEmpty` returns false and the subject
  is declined, the loop continuing.

### `rules/resolution.go`
- `repeatCursor` gains `election bool`; `SuspendRepeat` threads
  `s.Election` onto the parked loop frame.
- New `resumeResolution` arm `"repeat_each_optional"`: consumes the parked
  loop frame (`rp.outer`, identified by `election`) so the loop is re-entered
  exactly once at the OFFERED subject (not a second time through the outer
  recursion), rebuilds `Ctx.Repeat` + `Ctx.RepeatEachOptional`, and transfers
  the loop frame's accumulated Remembered (`loopRemembered`) and vote tally
  onto the head so the re-entered loop continues from the first pass's
  bindings. `rp.outer = lf.outer` makes the enclosing chain run after.

A body that suspends on its own nested ask is untouched: its existing
`Next+1` cursor resumes into the NEXT subject's election (covered by the
inline-fixture regression).

No `events.Event` field or ordinal changed. All game-state mutation continues
to go through emitted events. No `knownUnsupportedParams` edit (none of the
seven carriers is in the ratchet, re-verified `<empty grep>`); no
Known-approximations row added (the parameter had no row).

## Exact commands and output

`.cards` was PRESENT as a symlink (`ln -sfn /home/sadams/projects/gorge/.cards
.cards`); confirmed by the corpus test RUNNING rather than skipping (verbose
run below).

### Brief's targeted test command (`rules/`)
```
$ go test -run 'TestRepeatEachOptionalForEachPlayer|TestRepeatEachOptionalForEachPlayerSuspendedBody|TestHeads' ./rules/
ok  	github.com/adams-shaun/gorge/rules	1.970s
```
Verbose confirmation that the real-corpus test RAN (not skipped):
```
$ go test -v -run 'TestRepeatEachOptionalForEachPlayerMixedAnswers' ./rules/
=== RUN   TestRepeatEachOptionalForEachPlayerMixedAnswers
--- PASS: TestRepeatEachOptionalForEachPlayerMixedAnswers (0.69s)
PASS
ok  	github.com/adams-shaun/gorge/rules	0.708s
```

### Edited non-rules packages (run once each)
```
$ go test ./effects/
ok  	github.com/adams-shaun/gorge/effects	2.630s
$ go test ./botpolicy/
ok  	github.com/adams-shaun/gorge/botpolicy	0.644s
```

### Behaviour goldens outside `rules/`
```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	(cached)
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.299s
```
No allowlist edits. No bot-split or chain-head movement (TestHeads passed in
the brief's command; `cmd/botbench` split unchanged) — expected, as no repo
deck carries these parameters.

### Format
```
$ gofmt -l effects/choose_control.go effects/registry.go rules/resolution.go \
      rules/repeat_each_optional_test.go effects/repeat_each_optional_test.go \
      effects/context_test.go botpolicy/repeat_each_optional_test.go
gofmt-clean
$ go run ./cmd/gentypes -check
(no output)
$ go build ./...
(no output)
```

## Fails without the fix

Reverted the feature by disabling the per-subject-election branch in
`effects/choose_control.go` (`if optionalForEach {` → `if false &&
optionalForEach {`), ran the one command, then restored the file
byte-identically (`cmp` against `.ds4/scratch/choose_control.go.orig` printed
`RESTORED`). Failing output:

```
--- FAIL: TestRepeatEachOptionalForEachPlayerMixedAnswers (0.59s)
    repeat_each_optional_test.go:131: precondition failed: 2 Elemental Tokens before any answer, want 0
--- FAIL: TestRepeatEachOptionalForEachPlayerDeclinesEverySubject (0.00s)
    repeat_each_optional_test.go:169: decision = kind attackers resume "", want a repeat_each_optional KChoose for player 1: ...
--- FAIL: TestRepeatEachOptionalForEachPlayerAcceptsBoth (0.00s)
    repeat_each_optional_test.go:195: decision = kind attackers resume "", want a repeat_each_optional KChoose for player 1: ...
--- FAIL: TestRepeatEachOptionalForEachPlayerSuspendedBody (0.00s)
    repeat_each_optional_test.go:264: decision = kind arrange resume "arrange", want a repeat_each_optional KChoose for player 1: ...
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.609s
```

The first failure is the cleanest evidence: without the election, both
opponents' bodies run up front (2 Elementals before any answer); the other
three show the loop finishing with no election asked at all.

## New tests (new files, per the no-shared-append rule)

- `rules/repeat_each_optional_test.go` — end-to-end on the real corpus carrier
  Tempt with Vengeance (`RepeatPlayers$ Player.Opponent`, X=1):
  `…MixedAnswers` (opponent 1 declines → 0 tokens, opponent 2 accepts → 1
  token, in loop order, prompt = the script's `RepeatOptionalMessage$`),
  `…DeclinesEverySubject` (both decline → nothing, and asserts no
  `RepeatEach selector unimplemented` Note so the handler provably ran),
  `…AcceptsBoth`, and `…SuspendedBody` (inline fixture whose per-subject body
  Scries; answering the body's `KArrange` must resume into the NEXT subject's
  election). Each asserts its own precondition (zero tokens before any answer;
  the body's `KArrange` really appeared).
- `effects/repeat_each_optional_test.go` — R-9 no-ask decline (asks once per
  opponent, runs no body, parks nothing), the election cursor payload
  (`Election`, `Next`, captured subjects, subject), and re-entry
  accept/decline (accepted subject runs its body once and the next subject is
  asked; a declined subject runs nothing and the loop still advances).
- `botpolicy/repeat_each_optional_test.go` — the distinct
  `repeat_each_optional` shape round-trips `Decision.Validate` on both option
  orders (a binding check that the offered order is read, so the bot cannot
  livelock re-submitting a rejected answer). The shared KChoose fallback
  answers option 0, which is legal and terminates.

The one pre-existing test file touched is `effects/context_test.go`, only to
record `SuspendRepeat` calls on the effects double (its `fakeHost`); no
production `events.Event` change.

## Issues (found, not fixed)

- **Post-loop accumulation still computes 0.** With the election now correct,
  the carriers' post-loop `SubAbility$` accumulation (`Tempting Contract`'s
  `X: PlayerCountRememberedOwner$Amount` then `DBToken TokenAmount$ X`;
  `Tempt with Vengeance`'s `Y` via `DB$ StoreSVar | Type$ CountSVar`) still
  yields 0 because `DB$ StoreSVar` is unregistered, so "for each opponent who
  does, create N more" creates nothing. The election and each opponent's own
  offer are correct; the shared bonus is the adjacent defect. The brief marks
  the StoreSVar issue superseded, so I did not file a ticket.
- **`ChangeZoneTable$ True` is unread** (`effects/`, `rules/`, `cards/` have
  no read; 47 corpus files carry it, all seven `RepeatOptionalForEachPlayer$`
  carriers among them). Not this feature's parameter; listed so it is visible.
- **Non-player loops with `RepeatOptionalForEachPlayer$`** are asked to the
  subject object's controller (`PlayerOf`). No corpus carrier does this
  (all seven are player loops), so the generalization is unmeasured.
- This feature is invisible to `make ledger` (no CR-lane test drives
  `RepeatOptionalForEachPlayer$`). If it should stop being invisible, a
  CR-lane test citing CR 608.2c/601.2 would fit.

## Deviations from the brief

None. The suspending-body regression was added because the continuation
transport DID change (a new resume kind consuming the parked loop frame), as
the brief conditions it.
