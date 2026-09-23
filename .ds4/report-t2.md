# Task agent-20260919T183731Z-085022e9 — trig:Milled / trig:MilledAll

## Status: DONE

Registered the two mill-event trigger modes and made a mill distinguishable
from an ordinary library→graveyard move. `make report` no longer lists either
primitive as missing; the real-corpus tests on The Wise Mothman (`MilledAll`)
and Glowing One (`Milled`) pass, verified again AFTER the rebase onto main.

## Round 2 — the rebase (controller directive 2026-09-23T03:20:03Z)

`findings-t2.md` said only that the earlier rebase failed because
`.ds4/report-t1.md` was unstaged. There were no prior MAJORs to answer; the
round-1 code commit had been APPROVED (`verdict-t1.md`).

What I did:

1. The only uncommitted path was the tracked-but-gitignored rolling docs slot
   `.ds4/report-t1.md`. Committed it with `git add -f` (no `git add -A`).
2. `git rebase main` from `5c84527e` onto `8cac5583` (80 commits). The CODE
   commit `ae64998d` replayed with **no conflict**. The only conflict was
   `.ds4/report-t1.md`; I kept both sides (main's DestroyAll/Reveal reports,
   then this mill report), `git add -f`, `git rebase --continue`.
3. Re-ran every gate on the rebased tree; all pasted below are round-2 runs.

The mill code itself is unchanged by the rebase (it is the same commit,
re-parented onto current main).

## What changed (per file)

- **`events/actions.go`** — the mill action marker: `milledText = "milled"`,
  `Mill(obj, player)` (a `MoveZone` library→graveyard carrying
  `Text: "milled"`) and `IsMill(ev)`. Same marker discipline
  `IsDiscard`/`IsSacrifice` use; no new event Kind, so the append-only,
  hash-chained schema is untouched. `Text` was otherwise unused on a
  library→graveyard move.
- **`effects/cardflow.go` (`effMill`)** — emits `events.Mill(id, p)` instead
  of a bare `MoveZone`, and brackets one `api:Mill` resolution in a mill batch
  via an optional interface (`BeginMillBatch`/`EndMillBatch`, the
  `zoneBatcher` shape) so `MilledAll` can fire once for the whole call.
  `defer` closes the bracket; `effMill` has no decision ask, so it cannot
  suspend mid-body.
- **`rules/trigmatch_mill.go`** (new) — `milledMatches` for `Mode$ Milled` and
  `Mode$ MilledAll`: fires only on `events.IsMill`, then matches `ValidPlayer$`
  against the milled player (`ev.Player`, "you" = trigger source's controller)
  and `ValidCard$` against the milled card (`ev.Obj`). Registers both modes and
  `effects.RegisterNonAPI("trig:Milled")` / `("trig:MilledAll")`.
- **`rules/engine.go`** — the mill batch fields (`millBatchOpen/Depth/Idx/Log`).
- **`rules/trigger_match.go`** — `millBatchEntry`; the `checkFaceTriggers`
  latch for `MilledAll` (keyed on the trigger line, the `DamageAll` "one or
  more" shape: first matching card queues, later cards accumulate the count);
  `openMillBatch`/`closeMillBatch` (patch `TriggerAmount` = count and
  `Remembered`/`Captured` = the matching-card set); `BeginMillBatch` /
  `EndMillBatch`; and `Milled`/`MilledAll` in `actionTriggerModes` so
  `ActivationLimit$` (Mirelurk Queen) applies.
- **`rules/trigger_referents.go`** — `case "Milled", "MilledAll"`:
  `TriggerCard = ev.Obj`, `TriggerPlayer = ev.Player`, `TriggerAmount = 1` per
  event (the batch close overrides the count). This is what
  `TriggerCount$Amount` reads (The Wise Mothman's X).
- **`rules/trigger_eligibility.go`**, **`cards/compiled_codes.go`** —
  `triggerModeEvents` / `triggerInterestForMode` map both modes to
  `MoveZone`/`TriggerInterestZoneChange`.
- **`rules/trigmatch_registry_test.go`** — `Milled`/`MilledAll` added to
  `addedAfterTheSplit` with the ticket comment (the registry ratchet).
- **`rules/mill_trigger_test.go`** (new, separate file) — the tests below.

## Tests (all new, in a new file `rules/mill_trigger_test.go`)

- `TestMillTriggerGlowingOneGainsLifePerNonlandMill` — `Milled` (per-card):
  one mill of one nonland gains 1 life, with a `TriggerPush` naming Glowing
  One. Asserts the preconditions (Glowing One on the battlefield, the milled
  card a nonland).
- `TestMillTriggerWiseMothmanBatchesAndCounts` — `MilledAll` (batch): one mill
  of THREE nonlands fires ONCE and the target ask has `Max == 3`; answering
  all three places three +1/+1 counters. Without the batch latch the engine
  queues THREE triggers (a `KTriggerOrder` over three), which the test catches.
- `TestMillTriggerMirelurkQueenActivationLimitOncePerTurn` — a second mill in
  the same turn does not draw/counter again.
- `TestMillTriggerModesAreRegistered` — both primitives are in
  `effects.Supported()` (guards a registration revert, not just the matcher).

## Gates (round-2 commands, exact output)

```
$ go test -run 'TestMillTrigger|TestEveryDispatchedTriggerModeHasAMatcher|TestNoTriggerModeIsRegistered' ./rules/
ok  	github.com/adams-shaun/gorge/rules	1.106s

$ go test ./events/ ./cards/ ./effects/
ok  	github.com/adams-shaun/gorge/events	(cached)
ok  	github.com/adams-shaun/gorge/cards	(cached)
ok  	github.com/adams-shaun/gorge/effects	2.674s

$ go build ./...                 # exit 0, no output
$ go run ./cmd/gentypes -check   # exit 0, no output
$ gofmt -l <changed .go files>   # no output (all formatted)

$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.647s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.327s
```

`cmd/botbench` did NOT move: no repo deck carries any of the five carriers
(`grep -rl "Glowing One\|The Wise Mothman\|Mirelurk Queen\|Screeching
Scorchbeast\|Infesting Radroach" internal/testutil/decks/` → no hits), so no
botbench game can see the new triggers. TestHeads is a daemon gate and was not
run. No `heads_test.go` edit.

### `make report` and the missing set

```
$ make report            # exit 0
... (header printed)
$ grep -i mill .ds4/scratch/report.log
no mill in report        # trig:Milled / trig:MilledAll are NOT missing
```

`make report` recompiles the corpus in-memory and prints the top missing
primitives; neither mill mode appears anywhere in its output.

### Corpus prevalence (the brief's numbers, re-measured)

```
$ /usr/bin/grep -rlE 'Mode\$ MilledAll' .cards/cardsfolder | wc -l
3
$ /usr/bin/grep -rlE 'Mode\$ Milled ' .cards/cardsfolder | wc -l
2
```

Both brief claims held exactly (3 files / 2 files).

## Fails without the fix (proof each test can fail)

Round-2 proof. `rules/trigmatch_mill.go` and `rules/trigger_match.go` were
copied to `.ds4/scratch/`, the hunk reverted in the real file, the targeted
test run, then the file restored byte-identically (`cmp` against the scratch
copy: `RESTORED_CMP=0`).

### 1. Registration reverted (`"Milled"→"MilledX"`, `"MilledAll"→"MilledAllX"`)

```
--- FAIL: TestMillTriggerGlowingOneGainsLifePerNonlandMill (0.74s)
    mill_trigger_test.go:149: seat 0 life = 20, want 21 after one nonland mill (trigger did not fire)
--- FAIL: TestMillTriggerWiseMothmanBatchesAndCounts (0.00s)
    mill_trigger_test.go:217: pending = &{... Kind:attackers ...}, want the MilledAll target ask (KTarget)
--- FAIL: TestMillTriggerMirelurkQueenActivationLimitOncePerTurn (0.00s)
    mill_trigger_test.go:296: hand after first mill = 7, want 8 (the trigger did not draw)
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.765s
```

### 2. Batch latch reverted (`t.Mode == "MilledAll"` → `"MilledAllX"` at
`rules/trigger_match.go:1357`)

```
--- FAIL: TestMillTriggerWiseMothmanBatchesAndCounts (0.68s)
    mill_trigger_test.go:217: pending = &{... Kind:trigger_order ... Min:3 Max:3 Options:[The Wise Mothman ...] x3}, want the MilledAll target ask (KTarget)
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.697s
```

i.e. without the batch the engine queues THREE separate MilledAll triggers —
exactly the per-card bug. Restored (`RESTORED_CMP=0`).

## Ratchets / goldens

- `knownUnsupported` (`rules/acceptance_test.go`): none of the five carriers is
  a repo-deck card, so no entry exists to delete and none was touched. Measured
  by grep: no carrier name appears in `rules/acceptance_test.go` or
  `rules/paramcensus_test.go`.
- `knownUnsupportedParams` / `knownUnmodelledCountHeads`: untouched.
- `addedAfterTheSplit` (trigger registry ratchet): both modes added.
- AGENTS.md "Known approximations": no row added or removed (the table is
  frozen; this task closes no listed row).

## Issues

- **Cost mills do not fire these triggers.** `rules/cast.go` (the
  `Text: "mill cost"` move) emits an unmarked library→graveyard move, so a mill
  paid as a cost fires neither `Milled` nor `MilledAll` (CR 701.17a says
  milling is milling however paid). Left out of this ticket because the brief
  scoped it to the `api:Mill` primitive and named the `cost:Mill` ticket
  (agent-20260918T230554Z-2b1a0e21) as the cost-path owner. Already filed as
  `.ds4/new-tickets/cost-mill-milled-triggers.md`.
- **Infesting Radroach (Milled, `TriggerZones$ Graveyard` +
  `PresentZone$ Graveyard`/`IsPresent$ Card.StrictlySelf`) and Screeching
  Scorchbeast (MilledAll, `ResolvedLimit$ 1` + `OptionalDecider$ You`) are not
  covered by a test.** Their gates ride the shared `zoneGate` /
  `triggerConditionHolds` / `resolvedLimitValue` machinery, which this ticket
  did not change, but the two carriers are only verified by the code paths, not
  by a test. The brief's Done only named Glowing One and The Wise Mothman, and
  `make report` shows all five carriers unlocked (neither primitive missing).
- **`TriggerInterestZoneChange` narrowing for the two modes does not take
  effect until the IR cache is regenerated** (`cards/compiled_codes.go`'s
  `triggerInterestForMode` is baked into `.cards/ir.gob.gz` at compile time).
  Behaviour is unaffected either way: an unrecognised mode currently ORs in
  `TriggerInterestAny`, the fail-open default, so the compiled prefilter never
  drops a mill event; the change only narrows a mill-only face's scan set once
  the cache is rebuilt. `make report` recompiles fresh in-memory but
  deliberately never writes the shared cache.
- Round-1 review (`verdict-t1.md`) carried one MINOR: cost-mill producers must
  emit the canonical mill event once the cost:Mill ticket lands — captured
  above.
