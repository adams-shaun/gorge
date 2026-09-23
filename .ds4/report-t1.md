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
