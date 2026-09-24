# cli-20260924T034908Z-be6139b0 — non-P1P1 K:etbCounter entry counters fold into the entry move (CR 614.12)

Round 2. This answers the single MAJOR in `findings-t2.md`: the consolidated
acceptance requires a **non-P1P1** `K:etbCounter` (CHARGE) to have its counters
folded into the entry move before any ETB observer, without adding an
`events.Event` field. Round 1 shipped an absorption gate
(`entryBodyKindEncodable`) that rejected CHARGE and left it on the body path.

## What changed and why (per file)

**`events/entry_counters.go`** — the carrier. `EntryCounterPairs(g)` replaces
the old table-only `EntryCounterPair(g)` and encodes a grant two ways, both
inside the existing `Pairs [][2]state.ObjID` field (so `events.Event` and its
hash-chained binary encoding are untouched):

- the historic fixed-index pair for the four table kinds
  (`LOYALTY`/`P1P1`/`LORE`/`DEFENSE`), byte-identical to before;
- for any other kind, a header pair `{tag|tag2|byteLen, tag|amount}` followed
  by `ceil(len/3)` payload pairs carrying three UTF-8 bytes each
  little-endian.

Every word keeps the top tag bit (`entryCounterMarker = 1<<31`), and the string
form additionally sets `entryCounterStringMarker = 1<<30`. Real object ids can
never carry the top bit (`state/ids.go`'s `playerRefBit`/`NextID` range), so
generic event-reference walkers (`trigMustVisit`, `trigZonesCatchUp`, the
combat matchers) still return `nil` for every tagged word and never mistake
counter metadata for an object reference.

`applyEntryCounterPairs` is now a sequential decode: a table pair is one grant;
a string header consumes exactly the payload pairs its length names, so the two
forms cannot be confused. Malformed/truncated payloads are skipped whole, not
half-decoded. Three bytes per payload pair (not four) leave the two marker bits
clear, so a byte's own high bits are never clipped by the marker OR — the first
cut packed four bytes and CHARGE's `R` (0x52) lost bit 6 on decode; the
in-package codec probe caught it before any rule test.

`EntryCounterKindEncodable` now accepts any non-empty kind within the payload
bound (`entryCounterKindBytes = 64`). The four table kinds still go by index;
everything else by the UTF-8 tail.

**`rules/entry_counters.go`** — the call site uses `EntryCounterPairs(g)...`;
`entryBodyKindEncodable` still exists as the one shared predicate (both the
cheap pre-pass and the absorption walk call it) and now admits CHARGE, OIL,
M1M1, LEVEL, etc. Only an empty kind still fails closed. The doc comment was
rewritten accordingly.

**`rules/etb_counter_entry_fold_test.go`** — `TestEtbCounterNonEncodableKindStaysOnBodyPath`
is gone (it asserted the old, now-wrong body-path behaviour). Replaced by
`TestEtbCounterNonP1P1KindFoldsIntoMove` (see below).

**`rules/heads_test.go`** — 4/6/8-seat goldens re-pinned with measured
attribution.

## The acceptance test

`TestEtbCounterNonP1P1KindFoldsIntoMove` subtests:

- **plain** — an authored-but-real `K:etbCounter:CHARGE:2` artifact with an ETB
  observer that draws one card per charge counter (`Count$CardCounters.CHARGE`).
  Asserts the entry MoveZone carries a grant, the artifact has 2 CHARGE
  counters, the trigger FIRED, and the observer drew exactly 2. This is the
  brief's "ETB trigger reading its counters sees them" clause for a non-P1P1
  kind.
- **winding-first / season-first** — the same artifact under **Winding
  Constrictor** (`AddCounter` on an artifact, +1 of each kind) and **Doubling
  Season** (`AddCounter` on a permanent, double), both kind-agnostic. The pair
  is non-commuting, so the entry is staged behind the CR 616.1 order ask:
  `etbCounterEntryAnswerOrder` asserts the entry has NOT folded while the ask
  is outstanding, then each answer finalizes 6 or 5 CHARGE counters, the move
  carries the grant, and the observer draws exactly that many. This is the
  "body suspended for a choice" clause for a non-P1P1 kind.

**Why Winding Constrictor + Doubling Season rather than Hardened
Scales + Branching Evolution.** The brief names the latter, but both cards
name `ValidCounterType$ P1P1` (Hardened Scales' R: line is "+1/+1 counters on a
creature you control"; Branching Evolution the same). They can never contest a
CHARGE placement on an artifact — there would be no competition and no
suspension to test. The kind-agnostic pair is the *real* mechanism the
consolidated acceptance describes ("its CHARGE counters folded before any ETB
observer" while a CR 616.1 choice is outstanding). The P1P1 path under
Hardened Scales + Branching Evolution is already covered by
`TestEtbCounterEntryFoldsIntoMove` (round 1). This deviation is deliberate and
tested, not a scoping-out.

## Commands run (real output)

`.cards` is present as a symlink to the shared corpus
(`lrwxrwxrwx .cards -> /home/sadams/projects/gorge/.cards`), so corpus-backed
tests RAN — the `rules` runs took 0.46–1.5 s, not the ~2 ms vacuous skip.

```
$ go test -run 'EtbCounter|EntryCounter' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.462s

$ go test -run 'TestHeads$' ./rules/
ok  	github.com/adams-shaun/gorge/rules	1.548s

$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	1.767s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	(cached)   # package/binary unchanged since its last real run

$ go test ./events/
ok  	github.com/adams-shaun/gorge/events	(cached)       # same
```

Earlier in the round, after the `events` carrier change, both goldens ran for
real: `botbench` `ok 0.710s`, `events` `ok 4.539s`. The later `rules` edits were
comment/test-only, so the cache is valid.

```
$ gofmt -l rules/entry_counters.go rules/etb_counter_entry_fold_test.go rules/heads_test.go events/entry_counters.go
          # (empty)
$ go run ./cmd/gentypes -check
GENTYPES_OK
```

Also green: `go test -run 'Class|AddCounter|CounterReplacement|CounterCounter|CantPutCounter|EnergyReplacement|HardenedScales|BranchingEvolution' ./rules/`
(the round-1 report's regression surface for the old gate) → `ok 0.528s`.

## Fails without the fix

I copied `events/entry_counters.go` and `rules/entry_counters.go` to
`.ds4/scratch/fixbackup2/`, restored `EntryCounterKindEncodable` to the old
table-only body (the exact pre-fix behaviour: CHARGE not encodable → stays on
the body path), ran the one test, and restored both files (`cmp`-verified
`RESTORED_OK`). Output:

```
--- FAIL: TestEtbCounterNonP1P1KindFoldsIntoMove (0.40s)
    --- FAIL: TestEtbCounterNonP1P1KindFoldsIntoMove/plain (0.00s)
        etb_counter_entry_fold_test.go:200: entry move carries no CHARGE counter grant: {Seq:43 Kind:move_zone Player:0 Obj:1 From:hand To:battlefield Amount:0 Step:untap Counter: Text: IDs:[] Pairs:[] Secret:false}
    --- FAIL: TestEtbCounterNonP1P1KindFoldsIntoMove/winding-first (0.00s)
        etb_counter_entry_fold_test.go:179: entry folded before the CR 616.1 order answer
    --- FAIL: TestEtbCounterNonP1P1KindFoldsIntoMove/season-first (0.00s)
        etb_counter_entry_fold_test.go:179: entry folded before the CR 616.1 order answer
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.410s
FAIL
```

The `plain` failure is the missing fold; the two competition failures are the
exact atomicity gap the ticket exists for (the entry folds before the order
answer).

## Head movement (measured, per seat)

`TestHeads` moved 4/6/8 seats; 2 seats unchanged.

| seats | before (this branch) | after |
|---|---|---|
| 2 | `3ddcc4e3ba5bb799` | unchanged |
| 4 | `1bb310282d7dd96b` | `ada5fab32de2381b` |
| 6 | `93fbe0e6cfac99d6` | `6c9ca1b054ccb266` |
| 8 | `5b6fa6b65b319120` | `82a8fd284bb1bef1` |

Attribution measured, not guessed: I dumped every event of the acceptance
games with this build and with `git show HEAD:` versions of the two changed
non-test files (both runs in this worktree), then diffed per seat ignoring seq
renumbering. The diff is exactly one removed event per affected seat:

```
seat 4: -counter seats=4 obj=170 name="Chalice of the Void" ... amount=0 counter="CHARGE"
seat 6: -counter seats=6 obj=170 name="Chalice of the Void" ... amount=0 counter="CHARGE"
seat 8: -counter seats=8 obj=170 name="Chalice of the Void" ... amount=0 counter="CHARGE"
```

eldrazi-stompy's **Chalice of the Void** (`K:etbCounter:CHARGE:X`) enters from
the library with X=0; round 1 left its CHARGE body on the body path and logged
a real zero-amount `CounterChange`; this build absorbs it (a zero placement
contributes no grant and no notification), so that event (seq 2360/5040/9081)
is gone and every later seq shifts. Endless One's P1P1 fold (the round-1
cause) is unchanged. 2 seats is unchanged because neither death-n-taxes
(index 0) nor dimir-tempo (index 1) carries a `K:etbCounter` card; eldrazi-stompy
(index 2) first joins the rotation at 4 seats.

## Deviations from the brief

- **Competing pair is Winding Constrictor + Doubling Season, not Hardened
  Scales + Branching Evolution**, for the reason above: the named pair only
  affects `P1P1` and cannot contest a CHARGE placement. The named pair is still
  exercised for P1P1 by the round-1 test.
- **The acceptance card is authored inline** (real `K:etbCounter:CHARGE:2`
  line) rather than the literal corpus Chalice, consistent with the licensing
  rule that no corpus `.txt` is committed and with every other test in this
  file. The head attribution is measured on the real corpus Chalice.
- The brief's "Done means" said TestHeads may be re-pinned "only with
  attribution"; done, with the per-seat measurement above.

## Issues

- **Entry-counter bodies that are NOT the bare self shape still fold the entry
  before their counters.** `entryBodyAbsorbable` fails closed on a body with
  `SubAbility$`, an `Optional$/Choices$/...` modifier, a per-recipient count or
  a composite kind, leaving it on the ordinary body path — correct for a
  non-contested placement, but the CR 616.1 atomicity gap this ticket closes
  for bare bodies still exists for those. `rules/entry_counters.go`
  (`entryBodyAbsorbable`), `rules/replacement.go` (`applyReplacement` /
  `composeUpdatedReplacements`). Corpus prevalence: `/usr/bin/grep -rlE
  'ETB\\$ True' .cards/cardsfolder | xargs /usr/bin/grep -lE 'DB\\$ PutCounter'
  | wc -l` measures the parent set; the non-bare subset is smaller. Needs a
  staging continuation that can carry a body which places more than one grant
  or asks mid-absorption.
- A CR-lane test would make that remainder visible: CR 614.12 + CR 616.1,
  "an enters-with-counters body with a SubAbility under competing AddCounter
  replacements". Not written (out of scope).
- **Absorbing a zero-amount body suppresses its real `CounterChange`.** This is
  the deliberate round-1 P1P1-zero behaviour extended to every kind; it is the
  entire 4/6/8-seat head movement above. Recording it here so it is not
  mistaken for an accident. If a future trigger is found that must see a
  zero-amount placement event, this is the place it breaks.
- Not a defect, a coverage note: `EntryCounterPair` (the exported symbol) was
  replaced by `EntryCounterPairs`; nothing outside `events`/`rules` referenced
  it, so no caller was left behind.

## Prior finding re-verification

`findings-t2.md` had exactly one MAJOR (`rules/entry_counters.go:143`,
CHARGE not folded). It is fixed: the carrier carries arbitrary kinds and
`TestEtbCounterNonP1P1KindFoldsIntoMove` proves the fold and the suspension,
with the "fails without the fix" output above.
