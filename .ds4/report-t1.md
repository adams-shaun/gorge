# cli-20260924T034908Z-be6139b0 — K:etbCounter / Updated replacement-body counters fold into the entry move (CR 614.12)

## What changed and why (per file)

**`rules/entry_counters.go`** (the bulk). Entry-characteristic counting was
preflighted for intrinsic grants only (loyalty, Riot, Unleash, lore, defense).
It now also preflights the body-defined grant a `DB$ PutCounter | ETB$ True`
Updated `Moved` replacement places on the entering object itself:

- `entryBodyCandidates(ev)` — the cheap pre-preview gate. Scans the entering
  face's own `Repls` for the `PutCounter | ETB$ True` shape, and checks the
  derived `Bloodthirst`/`Sunburst` keywords (their bodies are synthesised by
  `replacement.go`, not expanded onto the face). An ordinary entry with no such
  body pays nothing.
- `entryBodyKindEncodable(sa)` / `entryBodyAbsorbable(sa)` — the shape gates.
  Only a bare single-kind, `Defined$ Self`, `Sub`-less body with a resolvable
  `CounterNum$` is absorbed; a rider or an asking modifier fails closed and
  stays on the ordinary body path.
- `entryBodyCounterGrants(ev, entrant)` — runs on the isolated post-entry
  preview. Gates each body with the ordinary `replacementMatches` (so a
  `CheckSVar$`/`SVarCompare$` gate fails closed exactly as the live dispatch
  would) and resolves the count with `effects.NumResolved` under the body's own
  `replCtx`. Returns the grants plus each absorbed body's `replIdentity`.
- `entryGrantPlan(ev, preview, entrant)` — intrinsic grants (origin board) plus
  body grants (preview board).
- `entryGrant` (new type) — a planned grant carries the source id it came from
  when it is body-defined. `settleEntryGrants` sets
  `applyingReplacement`/`replacingSource` to that source while applying the
  AddCounter class to it, so `EffectOnly$` (Doubling Season) and `ValidSource$`
  (Vorinclex) read the same replacement-body provenance the live body path
  carries (`Engine.replacementBodyCounterAdder`).
- `entryCounterStage.bodyIDs` + `replChoice.absorbed` — the absorbed-body set
  travels with the stage and with a re-parked Updated composition.
- `foldEntryMove` now returns `(events.Event, []string)`: the absorbed body
  identities, so the Updated dispatch does not run a body whose placement the
  move's Pairs payload already carries.

**`rules/replacement.go`**. `applyReplacement`, `composeUpdatedReplacements`
and `resumeUpdatedComposition` consult the absorbed set and skip those bodies.
`replChoice.absorbed` captures the set on the composition's preamble pass so a
later-answered absorbed body in a re-parked continuation is never run twice.

**`events/entry_counters.go`**. `EntryCounterKindEncodable` is the one shared
predicate over the encodable-kind table. **This is a correctness gate, not
polish** — see the regression below.

**`rules/heads_test.go`**. Re-pinned 4/6/8 seats with a measured attribution
comment (below). The 2-seat golden is unchanged.

**`rules/etb_counter_entry_fold_test.go`** (new). The acceptance test plus a
regression test for the encodability gate.

**`rules/engine.go`**. One call site adapted to `foldEntryMove`'s new signature.

## The regression the gate prevents (measured)

My first cut absorbed *every* body-defined grant into the MoveZone Pairs
payload. That payload is a fixed-index tag — `events.entryCounterKinds` names
only `LOYALTY/P1P1/LORE/DEFENSE`, and `EntryCounterPair` emits any other kind
as a zero tag `applyEntryCounterPairs` skips. So Class cards (whose level
counter is `LEVEL`) and Chalice of the Void (`CHARGE`) entered with **zero**
counters. Measured: `TestClassBandBandGatesGrantedReplacement` failed at
`precondition: Class at level 0, want 1`. The fix gates absorption on
`EntryCounterKindEncodable`; non-encodable kinds stay on their own body path
(unchanged behaviour, no silent drop). `TestEtbCounterNonEncodableKindStaysOnBodyPath`
locks it in.

## Commands run (real output)

`.cards` is a symlink to the shared corpus (`lrwxrwxrwx .cards -> /home/sadams/projects/gorge/.cards`), so corpus-backed tests RAN, not skipped. Measured 475 `K:etbCounter:` lines (brief's ~475 held).

```
$ go test -count=1 -run 'EtbCounter|EntryCounter' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.868s
```
19 tests pass, including every pre-existing entry-counter/staging test
(Doubling Season, Vorinclex, Solemnity, token entry counters, the CR 616.1
staging family) and the new `TestEtbCounterEntryFoldsIntoMove` (plain,
hardened-scales, scales-first, evolution-first, solemnity-blocks) and
`TestEtbCounterNonEncodableKindStaysOnBodyPath`.

```
$ go test -count=1 ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	1.389s

$ go test -count=1 -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	0.663s

$ go test -count=1 -run 'TestHeads$' ./rules/
ok  	github.com/adams-shaun/gorge/rules	1.584s

$ gofmt -l rules/ events/     # (empty)
$ go run ./cmd/gentypes -check # (empty)
$ go test -run 'Replacement' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.565s
$ go test -run 'Class' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.422s
```

## Fails without the fix

I saved the three touched non-test files to `.ds4/scratch/fixbackup/`,
temporarily made `entryBodyCandidates` return `false` (disabling the whole
body-grant path), ran the one test, then restored the file and `cmp`-verified
it byte-identical (`RESTORED_OK`). Output:

```
--- FAIL: TestEtbCounterEntryFoldsIntoMove (0.40s)
    --- FAIL: TestEtbCounterEntryFoldsIntoMove/plain (0.00s)
        etb_counter_entry_fold_test.go:189: entry move carries no counter grant: {Seq:43 Kind:move_zone Player:0 Obj:1 From:hand To:battlefield Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    --- FAIL: TestEtbCounterEntryFoldsIntoMove/hardened-scales (0.00s)
        etb_counter_entry_fold_test.go:189: entry move carries no counter grant: {Seq:44 Kind:move_zone Player:0 Obj:2 From:hand To:battlefield Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    --- FAIL: TestEtbCounterEntryFoldsIntoMove/scales-first (0.00s)
        etb_counter_entry_fold_test.go:167: entry folded before the CR 616.1 order answer
    --- FAIL: TestEtbCounterEntryFoldsIntoMove/evolution-first (0.00s)
        etb_counter_entry_fold_test.go:167: entry folded before the CR 616.1 order answer
FAIL
```

The `solemnity-blocks` subtest passes with or without the fix by design (the
placement is 0 either way); it asserts the trigger FIRED and that no real
`CounterChange` was emitted, so it cannot pass with the feature unregistered.
The precondition assertions in every subtest (creature on battlefield, trigger
pushed, ETB observer) are what keep the passing subtests from being vacuous.

## Head / ratchet movement (measured, met cause)

`TestHeads` moved **4, 6 and 8 seats**; **2 seats unchanged**.

| seats | old golden | new golden |
|---|---|---|
| 2 | `3ddcc4e3ba5bb799` | unchanged |
| 4 | `b3bf75f0c56512ae` | `1bb310282d7dd96b` |
| 6 | `a93593d452866261` | `93fbe0e6cfac99d6` |
| 8 | `beac5729b0e63dcc` | `5b6fa6b65b319120` |

Measured cause, not guessed. I ran the acceptance games and logged the entry
`MoveZone` of every repo-deck card carrying `K:etbCounter`:

```
seats=4 ENTRY seq=472  name="Endless One"        pairs=[[2147483650 2147483650]]
seats=4 ENTRY seq=498  name="Endless One"        pairs=[]
seats=4 ENTRY seq=524  name="Endless One"        pairs=[]
seats=4 ENTRY seq=2359 name="Chalice of the Void" pairs=[]
seats=6 ENTRY seq=182  name="Endless One"        pairs=[[2147483650 2147483650]]
seats=8 ENTRY seq=795  name="Endless One"        pairs=[[2147483650 2147483650]]
```

`[[2147483650 2147483650]]` is the Pairs tag for kind index 2 = `P1P1`, amount
1. So **eldrazi-stompy's Endless One** (`K:etbCounter:P1P1:X`, `X:Count$xPaid`)
now enters with its X +1/+1 counters folded into the entry move, and its body's
`CounterChange` becomes the notification-only `EntryCounterNotice` — every
later event payload differs. The 2-seat game is unchanged because neither
death-n-taxes (deck index 0) nor dimir-tempo (index 1) carries an
`K:etbCounter` card; eldrazi-stompy (index 2) only joins the rotation at 4
seats. Chalice of the Void carries CHARGE, a kind the Pairs tag cannot encode,
so it stays on the body path. The acceptance ratchets (`knownUnsupported`,
`knownUnsupportedParams`) are unchanged: my change reads the same `CounterNum$`/
`CounterType$` params the body already read. Botbench and archtest are
unchanged.

## Deviations from the brief / decisions

- **Absorption is limited to `P1P1`/`LOYALTY`/`LORE`/`DEFENSE`** (the kinds the
  frozen `Pairs` payload can encode). The brief's target shape and acceptance
  test are `P1P1`; other kinds (CHARGE, LEVEL, OIL, M1M1, …) keep their
  existing, correct body placement. Folding them would require a different
  carrier than the fixed-index `Pairs` tag. Filed as a follow-up (see
  `## Issues`).
- The brief says "a real `K:etbCounter` card". The committed test authors its
  card inline (the licensing rule forbids committing corpus `.txt` files) with
  a real `K:etbCounter:P1P1:2` line, so the real
  `cards/kw_etbcounter.go` expander produces the body under test. The head
  attribution above is measured on real corpus cards (Endless One).
- The brief's "Done means" said TestHeads may be re-pinned "only with
  attribution"; done, with the measurement pasted above.

## Issues

- **Non-encodable counter kinds are not folded into the entry move.** A
  body-defined entry grant whose kind the MoveZone `Pairs` tag cannot encode
  (`CHARGE`, `LEVEL`, `OIL`, `M1M1`, `SHIELD`, `ICE`, `TIME`, … — measured
  distinct `K:etbCounter` kinds: 311 P1P1, 34 M1M1, 23 OIL, 22 CHARGE, 11
  SHIELD, 7 Indestructible, 6 P1P0, 5 DIVINITY, and ~18 singletons) stays on
  its body's own placement path. That path is correct when the placement's
  AddCounter competition does not park, but in the parked CR 616.1 case the
  entry still folds before the answer and the body's counters land after it —
  the same atomicity gap this ticket closes for `P1P1`. Symptom: a Chalice of
  the Void (X≥1) entering under Hardened Scales + Branching Evolution would
  show the ETB observer 0 charge counters while the order ask is outstanding.
  `rules/entry_counters.go` (`entryBodyKindEncodable`), `events/entry_counters.go`
  (`entryCounterKinds`). Closing it needs a carrier other than the fixed-index
  `EntryCounterPairs` tag (the `events.Event` struct is frozen). Filed as
  `.ds4/new-tickets/entry-counter-arbitrary-kinds.md`.
- A CR-lane test would make that remainder visible: CR 614.12 + CR 616.1, "a
  permanent's enters-with-counters body of a non-P1P1 kind under competing
  AddCounter replacements". I did not write it (out of the brief's scope).
- Not a defect, a note: the emit pre-pass and `foldEntryMove` each build the
  isolated preview for an entry with a body candidate (the existing pattern for
  intrinsic grants), so a `K:etbCounter` entry clones the game twice. Correct
  and bounded, but a future ticket could share one preview.
