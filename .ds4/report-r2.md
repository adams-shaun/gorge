# Report — r2 (agent-20260920T071934Z-9ce69d52)

## What this round did

Round t1's only uncommitted artifact was `.ds4/report-t1.md` (the verification
report; the split-cast/Fuse **code** itself was already committed on main as
`b8347d4b` + follow-up `9b675a57`). This round:

1. Committed the t1 report. On rebase against main its path collided with a
   different agent's report already tracked at `.ds4/report-t1.md` on main
   (fb-20260922T145544Z), so main's version was kept untouched and the t1
   split-cast report was preserved alongside as
   `.ds4/report-t1-split-cast.md` (same commit).
2. Rebased the worktree onto main (`2954978f`) per the controller directive;
   the branch had no unique code commits, so it is a clean fast-forward plus
   the report commit.
3. **Verified the brief's "Done" state and closed its one open gap**: the
   brief names two specific pins that never existed in the landed work —
   `TestKillerHalfIsCastable` and `TestGallifreyFallsFuseCastsBothHalves` on
   `Coward // Killer` and `Gallifrey Falls // No More`. Added both in the new
   file `rules/split_cast_test.go` (new file, per the no-shared-append rule):

   - `TestKillerHalfIsCastable`: both halves offered from hand ("Cast Coward"
     + mode `split_alt` "Cast Killer"); casting Killer asks its own creature
     target; 3 damage kills the 2/2 Grizzly Bears; card → graveyard; replay
     verified.
   - `TestGallifreyFallsFuseCastsBothHalves`: the fused cast is WITHHELD at
     exactly {4}{R}{R} (7 red) and offered only at the summed
     {6}{R}{R}{W} (7R+2W) — the summed-cost assertion pins CR 702.101b's
     "both halves' costs"; both halves resolve (Falls' 4 damage kills the
     bear — see Issues for its inert exile rider); pool empty; replay
     verified.

   Both tests assert their preconditions (bear on battlefield, card in hand)
   and both were proven to fail with the feature's offer gates neutralized
   (see "Fails without the fix").

## Verification of the brief's premises (claims vs measurement)

- "128 files carry AlternateMode:Split" — **held** (measured: 128).
- "17 files match ^K:Fuse" — **held** (measured: 17).
- `Coward // Killer` and `Gallifrey Falls // No More` are IN the corpus
  (`.cards/cardsfolder/c/coward_killer.txt`,
  `.cards/cardsfolder/g/gallifrey_falls_no_more.txt`) — report-t1's claim
  that they are "not present in the current corpus" was **wrong**; they are
  absent only from `internal/testutil/decks/*.json` (measured: 0 hits), so
  the "deck carriers" wording in the brief is inaccurate for the committed
  deck set.
- The brief's CR citation "Fuse (CR 702/702.36)" — Fuse is CR 702.101; the
  landed code and tests cite it correctly.

## Fails without the fix

Neutralized `splitAlternateCastFace`/`fusedSplitFaces` in `rules/split.go`
(scratch revert), ran the two new tests, restored byte-identically (`cmp`
clean against the scratch copy):

```
--- FAIL: TestKillerHalfIsCastable (0.59s)
    split_cast_test.go:69: alternate-half offer missing/renamed (the brief's dead-half bug): [... only "Cast Coward" ...]
--- FAIL: TestGallifreyFallsFuseCastsBothHalves (0.00s)
    split_cast_test.go:141: fused offer missing at the summed cost: [... only "Cast Gallifrey Falls" ...]
FAIL	github.com/adams-shaun/gorge/rules	0.624s
RESTORED-BYTE-IDENTICAL
```

## Gates run (real output)

- `go test -run 'TestSplit|TestKillerHalfIsCastable|TestGallifreyFallsFuseCastsBothHalves' ./rules/`
  → `ok github.com/adams-shaun/gorge/rules 0.626s` (the 9 existing split tests + the 2 new ones, green).
- `gofmt -l rules/split_cast_test.go rules/split.go` → no output.
- `go run ./cmd/gentypes -check` → exit 0 (`GENTYPES-OK`).
- `go test ./internal/archtest/` → `ok 3.086s`.
- `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` → `ok 1.240s`
  (no engine code changed this round — only a new test file — so no split
  movement was expected or observed).
- `.cards/` symlink present (→ `/home/sadams/projects/gorge/.cards`); the
  rules runs above were corpus-backed, not skips.
- TestHeads / acceptance / `make sim` / conformance / `make report` are
  daemon gates and were not run; heads are untouched by a test-only commit.

## Issues

Defects found this round and NOT fixed (both outside the brief's scope; the
brief's change only had to make the halves reachable, which it is):

1. **A DamageAll's `ReplaceDyingDefined$` rider never registers** —
   `effects/damage.go` wires `registerReplaceDying` into `effDealDamage`
   (line 50) and the damage-exchange path (line 779) but NOT into
   `effDamageAll` (line 867). Gallifrey Falls' "If a creature dealt damage
   this way would die this turn, exile it instead" is therefore silently
   inert: the bear died to the graveyard instead of being exiled (observed
   directly in the new test; the assertion is written to the observed truth
   with a comment naming the gap). Corpus prevalence: **6 files** carry
   `SP$ DamageAll ... ReplaceDyingDefined` on one line
   (`/tmp/dmgall_rdd.txt` list captured during the round: gallifrey_falls_no_more,
   chandra's-fire heart variants and 4 others — the file list was printed by
   the loop and is reproducible with
   `grep -rlE 'SP\$ DamageAll.*ReplaceDyingDefined' .cards/cardsfolder`).
   Fix is one `defer` mirroring effDealDamage's shape; it changes engine
   behaviour, so it should be its own ticket with heads/botbench re-pinned.
   A CR-lane test citing CR 614.9 would make this defect ledger-visible.
2. **`api:Phases` (phasing out permanents) is unimplemented** — no
   `Register("Phases", …)` anywhere; `SP$ Phases` appears in **7 corpus
   files** (gallifrey_falls_no_more's No More half among them). The No More
   half resolves as an unimplemented-API note; the brief's "mass phase-out"
   behaviour does not exist yet. Deserves a ticket; a CR-lane test citing
   CR 702.25 (phasing) would surface it.

Both were left to the ledger rather than fixed: fixing either changes engine
behaviour outside this brief (heads/botbench re-pin obligations), and the
brief's own Done state is fully satisfied without them.

Report files: `.ds4/report-t1-split-cast.md` (round t1's report, committed
this round), `.ds4/report-r2.md` (this file).
