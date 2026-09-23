# Report — DestroyAll.Zone

Implemented and committed as `67b6fc8c` (`fix(effects): honor DestroyAll Zone parameter`).

## Changes

- `effects/zone.go`: `effDestroyAll` now reads `Zone$`, defaults to `Battlefield`, and fails closed for an unknown zone word. Victim collection, zone recheck, and the `MoveZone` event use the selected zone. Battlefield-only indestructibility, regeneration, Umbra Armor, and batch departure handling remain limited to the battlefield.
- `effects/destroyall_zone_test.go`: added an end-to-end primitive test proving an Instant in an opponent's exile moves to the graveyard, while a matching card outside that zone and a nonmatching battlefield creature remain in place.

The worktree already had `.cards` as a symlink to `/home/sadams/projects/gorge/.cards`; corpus-backed tests were not skipped. The supplied prevalence claim held: `/usr/bin/grep -rlE 'DB\\$ DestroyAll.*Zone\\$' .cards/cardsfolder | wc -l` returned `1`.

## Verification

- `go test -run '^TestDestroyAllUsesNamedZone$' ./effects/`
  ```
  ok  github.com/adams-shaun/gorge/effects  0.002s
  ```
- Proved the new test fails without the fix by temporarily restoring battlefield-only zone selection and restoring `effects/zone.go` byte-identically afterward (`cmp` passed):
  ```
  --- FAIL: TestDestroyAllUsesNamedZone (0.00s)
      destroyall_zone_test.go:22: named-zone card moved to exile, want graveyard
  FAIL
  FAIL github.com/adams-shaun/gorge/effects 0.002s
  FAIL
  ```
- `go test -run '^TestEveryRepoDeckParamsAreRead$' ./rules/`
  ```
  ok  github.com/adams-shaun/gorge/rules  0.756s
  ```
- `go test ./internal/archtest/ 2>&1 | tail -15`
  ```
  ok  github.com/adams-shaun/gorge/internal/archtest  3.915s
  ```
- `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -5`
  ```
  ok  github.com/adams-shaun/gorge/cmd/botbench  1.398s
  ```
- `go run ./cmd/gentypes -check` passed (no output); `gofmt -l effects/zone.go effects/destroyall_zone_test.go` returned no files; `git diff --check` passed.

## Issues

None found outside the brief's scope. The parameter census test passed; no ratchet table change was needed in this worktree.

# Task rv1 — `RevealAllValid$` unread: reveal every matching hand card

## What changed and why

**`effects/cardflow.go`** (`effReveal`, one new block after the existing
`RevealValid$` filter and the `n := amt` count resolution, before the pickable
block): when `RevealAllValid$` is non-empty, filter the pool by the spec with
`MatchesSpecCtx(g, rav, id, c.SpecContext(c.Controller))` and set
`n = int32(len(pool))`. Pre-fix the parameter was read ONLY by the `pickable`
gate, so the spec was never fed to the filter and the emit took `pool[:1]` —
the FIRST card of the whole hand whether or not it matched. This is the single
choke point the brief named; the `pickable` gate is untouched.

Everything downstream then works unchanged: `pool[:n]` emits the full matching
set, `RememberRevealed$` captures all of it, and a zero-match pool hits the
existing `if n == 0 { continue }` skip — no Note, no capture, the same
fail-closed convention `RevealType$`/`RevealValid$` already apply.

**`effects/revealallvalid_test.go`** (new file): three tests driving the REAL
corpus scripts (style: `infernal_tutor_test.go`).

**`AGENTS.md`**: deleted the `RevealAllValid$` Known-approximations row.

**`internal/testutil/agentsdoc_test.go`**: `knownApproximationRows` 20 → 19 and
`knownOversizeRows` 8 → 7, with the measurement comment updated.

### Brief premise corrections (both re-measured)

1. The brief's expected Break Expectations result `[Bolt, Bears]` for a hand
   `[Plains, Lightning Bolt, Grizzly Bears]` is **wrong**: Lightning Bolt is
   `ManaCost:R` → cmc 1, and the spec is `Card.cmcGE2+…`, so Bolt does NOT
   match. My test uses `[Plains(0), Lightning Bolt(1), Grizzly Bears(2), Hill
   Giant(4)]` and asserts exactly `[Grizzly Bears, Hill Giant]` in hand order.
2. The brief said lower `knownApproximationRows` "from 23 to 22". The merged
   constant measured 20 (and `approximationRows()` counted 20 data rows at
   HEAD), so I lowered it to 19. I also lowered `knownOversizeRows` 8 → 7: the
   deleted row's Stand-in cell was 809 bytes (over the 600 cap), and the
   constant's own doc says "Lower it when you delete one of them". The
   zero-match test's `n == 0` path is the fail-closed convention; the "assert
   the handler ran" requirement is satisfied by asserting no Note carries ids
   (with the fix reverted this same test emits ids `[1]`).

## Gate commands and real output

Targeted run (the brief's permitted invocation):

```
$ go test -run 'TestRevealAllValid' ./effects/ 2>&1 | tail -5
ok  	github.com/adams-shaun/gorge/effects	(cached)
```

Full targeted output on the first uncached pass:

```
$ go test -run 'TestRevealAllValid' ./effects/ > .ds4/scratch/t.log 2>&1; tail -40 .ds4/scratch/t.log
ok  	github.com/adams-shaun/gorge/effects	0.619s
```

Ratchet test (the row deletion):

```
$ go test -run 'TestKnownApproximation' ./internal/testutil/ 2>&1 | tail -10
ok  	github.com/adams-shaun/gorge/internal/testutil	0.001s
```

```
$ go test ./internal/archtest/ 2>&1 | tail -10
ok  	github.com/adams-shaun/gorge/internal/archtest	3.402s
```

```
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -8
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.262s
```

```
$ gofmt -l effects/cardflow.go effects/revealallvalid_test.go internal/testutil/agentsdoc_test.go
(empty)
$ go run ./cmd/gentypes -check
(empty)
```

`.cards` was **present** in this worktree (symlink to the shared corpus), so
the corpus-backed tests ran, not skipped.

## Fails without the fix

Saved the fixed `effects/cardflow.go` to `.ds4/scratch/cardflow.go.fixed`,
removed the new block from the real file, ran the one targeted command, then
restored byte-identically (`cmp` clean):

```
$ go test -run 'TestRevealAllValid' ./effects/
--- FAIL: TestRevealAllValidBreakExpectationsRevealsEveryMatch (0.56s)
    revealallvalid_test.go:128: reveal ids = [1], want exactly the two cmc>=2 hand cards [3 4]
--- FAIL: TestRevealAllValidMindSpikeRemembersEveryMatch (0.00s)
    revealallvalid_test.go:174: reveal ids = [1], want exactly the two matching cards [1 4]
--- FAIL: TestRevealAllValidZeroMatchSkipsCleanly (0.00s)
    revealallvalid_test.go:208: a zero-match reveal emitted ids [1], want none
FAIL
FAIL	github.com/adams-shaun/gorge/effects	0.574s
FAIL
```

`ids = [1]` is the first hand card (Plains) in every case — the exact defect.
Each test asserts its own precondition (the four cmc values 0/1/2/4 actually
differ; the creature/land/instant classes actually differ; every zero-match
hand card is cmc < 2; the object is in the hand zone), so a vacuous setup fails
loudly.

## Head / ratchet movement

None. None of the 9 `RevealAllValid$` cards is in any repo deck (re-measured:
`/usr/bin/grep -rl 'RevealAllValid\$' internal/testutil/decks/ | wc -l` → 0),
so `TestHeads` did not move and `botbench`'s pinned split is unchanged (gate
above passed with no re-pin). Daemon gates (TestHeads, `make sim`, `go vet`,
CR conformance) skipped per `gorge-context.md`.

## Issues

- **Residual deviation (round-t1 review MINOR, now disclosed): 3 of the 9
  `RevealAllValid$` carriers use `TargetedPlayerOwn` in the spec —
  wingbright_thief, boareskyr_tollkeeper, phantasmal_extraction — and
  `TargetedPlayerOwn` is NOT a registered matcher word
  (`effects/filter.go` registers only `TargetedPlayerCtrl`; measured corpus
  prevalence: `/usr/bin/grep -rlE 'TargetedPlayerOwn' .cards/cardsfolder |
  wc -l` → 23 files). Those three cards now reveal NOTHING (fail-closed,
  `n == 0` skip) where pre-fix they revealed one arbitrary card — both wrong,
  the fail-closed direction is the convention the brief mandates, and none of
  the 9 is in any repo deck, so no golden moved. Filed for the operator as
  `.ds4/new-tickets/effects-targeted-player-own-matcher.md` in this worktree.
- **Comma-union coverage gap (round-t1 review MINOR):** no test drives the one
  comma-union carrier (Boareskyr Tollkeeper,
  `Creature.TargetedPlayerOwn,Land.TargetedPlayerOwn`) because both of its
  alternatives are `TargetedPlayerOwn`, which fails closed until the matcher
  word is registered (previous bullet). The union plumbing itself is
  byte-identical to the sibling `RevealValid$`/`RevealType$` blocks; a union
  test should be added when the `TargetedPlayerOwn` ticket lands.
- **`AlreadyRevealed$` is unread anywhere in the Go tree** (confirmed: zero
  grep hits). It appears on the chained ChangeZone of Break Expectations and
  the 8 siblings. Today the sub re-walks the hand with its own
  `ChangeType$ …+TargetedPlayerCtrl` filter, so behaviour is correct without
  it; the flag is a cosmetic no-op. Out of scope per the brief. No CR-lane
  test needed (not a visible defect).
- **Lightning Bolt cmc is 1, not ≥2** — a premise error in the filed brief, not
  a code defect. Noted so a future reader doesn't trust the brief's example.
- **`RevealAllValid$` on `PeekAndReveal`** has zero corpus lines (verified
  `Random$`/`Look$`/`Optional$`/`NumCards$`/`PeekAmount$` collisions likewise
  zero across the 9 cards), so the new block cannot collide with those arms.
  No issue.

STATUS=DONE
COMMITS=3a8dc7122025f479d3d84b26325741e0e07c9116
TESTS=go test -run 'TestRevealAllValid' ./effects/ → ok; TestKnownApproximation* → ok; archtest → ok; botbench TestConstructedDefaultIsByteIdentical → ok; gofmt/gentypes clean
# Report — task agent-20260918T230554Z-74976c7c (kw:Backup)


## Status: DONE

Registered the two mill-event trigger modes and made a mill distinguishable
from an ordinary library→graveyard move. `make report` no longer lists either
primitive as missing; the real-corpus tests on The Wise Mothman (`MilledAll`)
and Glowing One (`Milled`) pass.

Rebase: `git status` was clean at start (no uncommitted work to preserve), and
`git log -1` was already `5c84527e` (a merge of main). No rebase was needed.

## What changed (per file)

- **`events/actions.go`** — added the mill action marker: `milledText =
  "milled"`, `Mill(obj, player)` (a `MoveZone` library→graveyard carrying
  `Text: "milled"`) and `IsMill(ev)`. This is the same marker discipline
  `IsDiscard`/`IsSacrifice` use, and it needs no new event Kind (append-only
  schema untouched). `Text` was otherwise unused on a library→graveyard move.
- **`effects/cardflow.go` (`effMill`)** — emits `events.Mill(id, p)` instead
  of a bare `MoveZone`, and brackets one `api:Mill` resolution in a mill batch
  via an optional interface (`BeginMillBatch`/`EndMillBatch`, the
  `zoneBatcher` shape) so `MilledAll` can fire once for the whole call.
  `defer` closes the bracket; `effMill` has no decision ask, so it cannot
  suspend mid-body.
- **`rules/trigmatch_mill.go`** (new) — `milledMatches` for `Mode$ Milled` and
  `Mode$ MilledAll`: fires only on `events.IsMill`, then matches `ValidPlayer$`
  against the milled player (`ev.Player`, "you" = trigger source's controller)
  and `ValidCard$` against the milled card (`ev.Obj`). Registers both modes
  and `effects.RegisterNonAPI("trig:Milled")` / `("trig:MilledAll")`.
- **`rules/engine.go`** — the mill batch fields (`millBatchOpen/Depth/Idx/Log`).
- **`rules/trigger_match.go`** — `millBatchEntry`; the `checkFaceTriggers`
  latch for `MilledAll` (keyed on the trigger line, the `DamageAll` "one or
  more" shape: first matching card queues, later cards accumulate the count);
  `openMillBatch`/`closeMillBatch` (patch `TriggerAmount` = count, and
  `Remembered`/`Captured` = the matching-card set); `BeginMillBatch` /
  `EndMillBatch`; and `Milled`/`MilledAll` added to `actionTriggerModes` so
  `ActivationLimit$` (Mirelurk Queen) applies.
- **`rules/trigger_referents.go`** — `case "Milled", "MilledAll"`: `TriggerCard
  = ev.Obj`, `TriggerPlayer = ev.Player`, `TriggerAmount = 1` per event (the
  batch close overrides the count). This is what `TriggerCount$Amount` reads.
- **`rules/trigger_eligibility.go`** and **`cards/compiled_codes.go`** —
  `triggerModeEvents` / `triggerInterestForMode` map both modes to
  `MoveZone`/`TriggerInterestZoneChange`, keeping a mill-only face's
  compiled scan set narrow.
- **`rules/trigmatch_registry_test.go`** — `Milled`/`MilledAll` added to
  `addedAfterTheSplit` with the ticket comment (the registry ratchet).
- **`rules/mill_trigger_test.go`** (new) — the tests (below).

## Tests (all new, in a new file)

- `TestMillTriggerGlowingOneGainsLifePerNonlandMill` — `Milled` (per-card):
  one mill of one nonland gains 1 life, with a `TriggerPush` naming Glowing
  One. Asserts the preconditions (Glowing One on the battlefield, the milled
  card a nonland).
- `TestMillTriggerWiseMothmanBatchesAndCounts` — `MilledAll` (batch): one mill
  of THREE nonlands fires ONCE (`TriggerPush` count == 1) and the target ask
  has `Max == 3`; answering all three places three +1/+1 counters. If the
  batch latch is absent the engine queues THREE triggers (a `KTriggerOrder`
  over three), which the test catches.
- `TestMillTriggerMirelurkQueenActivationLimitOncePerTurn` — a second mill in
  the same turn does not draw/counter again.
- `TestMillTriggerModesAreRegistered` — both primitives are in
  `effects.Supported()` (guards a registration revert, not just the matcher).

## Gates (exact commands and real output)

```
$ go test -run 'TestMillTrigger|TestEveryDispatchedTriggerModeHasAMatcher|TestNoTriggerModeIsRegistered' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.695s

$ go test ./events/
ok  	github.com/adams-shaun/gorge/events	5.616s

$ go test ./cards/
ok  	github.com/adams-shaun/gorge/cards	7.193s

$ go test ./effects/
ok  	github.com/adams-shaun/gorge/effects	2.570s

$ go build ./...            # no output (success)
$ go vet ./rules/           # exit 0, no output
$ go run ./cmd/gentypes -check   # exit 0, no output
$ gofmt -l <changed .go files>   # no output (formatted)

$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.325s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.229s
```

`cmd/botbench` did NOT move: no repo deck carries any of the five carriers
(`grep -rl "Glowing One\|The Wise Mothman\|Mirelurk Queen\|Screeching
Scorchbeast\|Infesting Radroach" internal/testutil/decks/` → no hits), so no
botbench game can see the new triggers. TestHeads is a daemon gate and was not
run.

### `make report` and the missing set

```
$ make report
cards: 33667  playable: 29775 (88.4%)
tokens: 839
top missing primitives (cards unlocked):
  ... (trig:Milled / trig:MilledAll do NOT appear)
```

Because the report only prints the top 25, I also queried coverage directly
(a temporary `cards.LoadRegistry`+`effects.Supported()` program with a blank
`rules` import, so `RegisterNonAPI` runs):

```
trig:Milled not in missing set
trig:MilledAll not in missing set
cards=33667 playable=29775
```

(The brief's AGENTS.md baseline figure of 24727/73.4% is stale — it predates
the version-5 IR cache and the primitives merged since. The brief's measured
CLAIM that these modes each block 2 and 3 cards held: with the registration
reverted the coverage query reports exactly `trig:Milled` blocks 2 and
`trig:MilledAll` blocks 3.)

## Fails without the fix (proof each test can fail)

`rules/trigmatch_mill.go` and `rules/trigger_match.go` were copied to
`.ds4/scratch/`, the hunk reverted in the real file, the test run, then the
file restored byte-identically (`cmp` against the scratch copy — restored-ok).

### 1. Registration reverted (`"Milled"→"MilledX"`, `"MilledAll"→"MilledAllX"`)

```
--- FAIL: TestMillTriggerGlowingOneGainsLifePerNonlandMill (0.64s)
    mill_trigger_test.go:149: seat 0 life = 20, want 21 after one nonland mill (trigger did not fire)
--- FAIL: TestMillTriggerWiseMothmanBatchesAndCounts (0.00s)
    mill_trigger_test.go:217: pending = &{... Kind:attackers ...}, want the MilledAll target ask (KTarget)
FAIL	github.com/adams-shaun/gorge/rules	0.652s
```

### 2. Batch latch reverted (`t.Mode == "MilledAll"` → `"MilledAllX"` in the latch)

```
--- FAIL: TestMillTriggerWiseMothmanBatchesAndCounts (0.00s)
    mill_trigger_test.go:217: pending = &{... Kind:trigger_order ... Min:3 Max:3 Options:[The Wise Mothman ...] x3}, want the MilledAll target ask (KTarget)
FAIL	github.com/adams-shaun/gorge/rules	0.581s
```

i.e. without the batch the engine queues THREE separate MilledAll triggers —
exactly the per-card bug. (Glowing One, the per-card mode, still passed.)

### 3. `actionTriggerModes` membership reverted

```
--- FAIL: TestMillTriggerMirelurkQueenActivationLimitOncePerTurn (0.64s)
    mill_trigger_test.go:306: hand after second mill = 9, want 8 (ActivationLimit$ 1 not enforced)
FAIL	github.com/adams-shaun/gorge/rules	0.648s
```

## Ratchets / goldens

- Chain heads (`rules/heads_test.go`): not run in-seat (daemon gate). No
  `heads_test.go` edit.
- `knownUnsupported` (`rules/acceptance_test.go`): none of the five carriers is
  a repo-deck card, so no entry exists to delete and none was touched.
- `knownUnsupportedParams` / `knownUnmodelledCountHeads`: untouched.
- `addedAfterTheSplit` (trigger registry ratchet): both modes added.
- AGENTS.md "Known approximations": no row added or removed (the table is
  frozen; this task closes no listed row).

## Issues

- **Cost mills do not fire these triggers.** `rules/cast.go` (the
  `Text: "mill cost"` move, ~line 1465) emits an unmarked library→graveyard
  move, so a mill paid as a cost fires neither `Milled` nor `MilledAll`
  (CR 701.17a says milling is milling however paid). Left out of this ticket
  because the brief scoped it to the `api:Mill` primitive and named the
  `cost:Mill` ticket (agent-20260918T230554Z-2b1a0e21) as the cost-path owner.
  Filed as `.ds4/new-tickets/cost-mill-milled-triggers.md`.
- **Infesting Radroach (Milled, `TriggerZones$ Graveyard` +
  `PresentZone$ Graveyard`/`IsPresent$ Card.StrictlySelf`) and Screeching
  Scorchbeast (MilledAll, `ResolvedLimit$ 1` + `OptionalDecider$ You`) are not
  covered by a test.** Their gates ride the shared `zoneGate` /
  `triggerConditionHolds` / `resolvedLimitValue` machinery, which this ticket
  did not change, but the two carriers are only verified by the code paths,
  not by a test in this round. Both are real corpus cards; the brief's Done
  only named Glowing One and The Wise Mothman.
- **`TriggerInterestZoneChange` narrowing for the two modes does not take
  effect until the IR cache is regenerated** (`cards/compiled_codes.go`'s
  `triggerInterestForMode` is baked into `.cards/ir.gob.gz` at compile time).
  Behaviour is unaffected either way: an unrecognised mode currently ORs in
  `TriggerInterestAny`, the fail-open default, so the compiled prefilter never
  drops a mill event; the change only narrows a mill-only face's scan set once
  the cache is rebuilt. `make report` recompiles fresh in-memory but
  deliberately never writes the shared cache.
