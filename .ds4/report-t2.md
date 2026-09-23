# Task report — model the `Convoked$Amount` Count head

Ticket: agent-20260922T200200Z-7feb602c
Commit: `9e650da1` `feat(effects): model the Convoked$Amount Count head`

## State of the round (read this first)

The implementation for this ticket was already present and committed on this
branch when the round started (`9e650da1`, the branch tip). The provided
`findings-t2.md` is **not** a review finding: it is the recorded output of a
failed `git rebase`/merge-fallback whose only blocker was an unstaged
`.ds4/report-t1.md` (an agent artifact, not product code), plus a stale
`.ds4/report-t2.md` left over from a *different* ticket
(player-count sacrifice attribution). Neither names a defect in this ticket's
work.

This round therefore did NOT rewrite the implementation. It re-verified the
committed work against every "Done means" item, proved the new tests fail with
the fix reverted, and restored the working tree to a clean, committed state
(the dirty `.ds4/report-t1.md` was restored to HEAD; no product file was
changed). The commit already on the branch satisfies the brief; the evidence
is below.

If the controller expected a fresh commit for this round: there is none, by
design, because the round changed no product code and adding a no-op commit
would only obscure `9e650da1`. The branch tip is the deliverable.

## What changed and why (per file, as committed in `9e650da1`)

### `effects/count.go`
Adds the `Convoked$Amount` dispatch to `evalCountBody`, immediately before the
`switch head`. It reads `g.Obj(c.Source).Convoked` (the same source-object
provenance `effects/context.go`'s `definedSpec` uses for `Defined$ Convoked`)
and returns a legitimate `(0, true)` when the source is absent or the
provenance is empty — a modelled head, never the unresolvable fallthrough.

The head is a `<Head>$<Property>` body, so it carries its **own** optional
`/Op`: `Count$Convoked$Amount/Twice` gets the suffix peeled upstream by
`evalCountExprOK` and applied generically, while the corpus's bare
`SVar:X:Convoked$Amount/Twice` (Ancient Imperiosaur) reaches the arm with the
suffix intact and strips it here. Both compose through the single shared
`applyCountOp`, so there is no duplicate `Twice` implementation. An unknown
`Convoked$<Property>` returns `(0, false)` (fail closed).

### `effects/filter.go`
Adds `SpecUsesConvokedAmount(spec)`, the count-head sibling of
`SpecUsesConvokedReferent`. It matches the `Convoked$` head-family marker
itself (not a `Count$` prefix, which the corpus's bare form omits), so the next
`Convoked$<Property>` head is covered without a second classifier arm.

### `rules/cast.go`
`faceWantsConvoked` and `abilityParamsUseConvoked` now also consult
`SpecUsesConvokedAmount`. This is the necessary provenance gate: without it,
neither carrier ever emitted the pay-time `FlagConvoked` CastInfo, so
`Object.Convoked` stayed empty and the reported symptom persisted even with
the count head modelled. The brief permitted this only if investigation proved
the gate was missing — it was, and the test below measures it (with the gate
reverted, `Object.Convoked = []` on the stack).

### Tests (new files, per the "new tests go in a new file" rule)
- `effects/convoked_amount_test.go` — `TestConvokedAmountReadsTheCorpusHeads`
  (both real corpus SVar bodies, plain head = 2 and `/Twice` = 4, plus the
  `Count$`-prefixed spelling composing to 4 not 8) and
  `TestConvokedAmountEmptyAndAbsentAreEvaluatedZero` (present-empty and absent
  source both `(0,true)`; unknown property fails closed).
- `rules/convoked_amount_test.go` — end-to-end
  `TestAncientImperiosaurEntersWithTwoCountersPerConvoker` (two convokers ⇒ 4
  `P1P1` counters) and `TestKnightErrantOfEosXCountsConvokers` (X = 2 on the
  stack and off the resolved permanent), both driving the real convoke
  announcement through `rules/cast.go`'s `convokeAsk`.

## Gates run (real, non-cached output)

```text
$ go test -count=1 ./internal/archtest/ 2>&1 | tail -2
ok  	github.com/adams-shaun/gorge/internal/archtest	2.165s

$ go test -count=1 -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -2
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.534s

$ go test -count=1 -run 'TestConvokedAmount|TestAncientImperiosaurEntersWithTwoCountersPerConvoker|TestKnightErrantOfEosXCountsConvokers|TestEveryRepoDeckCountHeadResolves' ./effects/ ./rules/
ok  	github.com/adams-shaun/gorge/effects	1.211s
ok  	github.com/adams-shaun/gorge/rules	1.302s

$ gofmt -l effects/count.go effects/filter.go effects/convoked_amount_test.go rules/cast.go rules/convoked_amount_test.go
(no output)

$ go run ./cmd/gentypes -check
(no output; exit 0)
```

No botbench split movement: `TestConstructedDefaultIsByteIdentical` passes on
its pinned 20-game split (neither carrier appears in the repo decks, and the
provenance gate only fires for a face whose text contains `Convoked$`). No
chain-head or ratchet movement was observed; `TestEveryRepoDeckCountHeadResolves`
is green and the ratchet has no `Convoked` entry to remove (checked:
`grep -n Convoked rules/count_head_ratchet_test.go` → no match).

Brief premises re-measured (the brief itself asks that counts be treated as
claims): the corpus census holds exactly:

```text
$ /usr/bin/grep -rlE 'Convoked\$Amount' .cards/cardsfolder | wc -l
2
$ /usr/bin/grep -rlE 'Convoked\$Amount' .cards/cardsfolder
.cards/cardsfolder/k/knight_errant_of_eos.txt
.cards/cardsfolder/a/ancient_imperiosaur.txt
```

The two carriers are exactly the reported ones. `.cards` was present in this
worktree as a symlink, so every run above exercised the corpus (not a
skipped/vacuous green).

## Fails without the fix

Production files were backed up to `.ds4/scratch/fixbak/`, the three hunks
(`effects/count.go` head, `effects/filter.go` classifier, `rules/cast.go` gate
uses) were removed, and the targeted tests were run; then the files were
restored from `HEAD` and compared byte-for-byte:

```text
$ cmp effects/count.go  .ds4/scratch/fixbak/count.go  && echo "count cmp=0"
count cmp=0
$ cmp effects/filter.go .ds4/scratch/fixbak/filter.go && echo "filter cmp=0"
filter cmp=0
$ cmp rules/cast.go     .ds4/scratch/fixbak/cast.go   && echo "cast cmp=0"
cast cmp=0
```

The pre-fix run exited non-zero with all three new tests failing at their own
preconditions (never a vacuous pass):

```text
$ go test -run 'TestConvokedAmount|TestAncientImperiosaurEntersWithTwoCountersPerConvoker|TestKnightErrantOfEosXCountsConvokers' ./effects/ ./rules/
--- FAIL: TestConvokedAmountReadsTheCorpusHeads (0.65s)
    convoked_amount_test.go:75: Num Amount$ X (Knight-Errant SVar) = 0, want 2
FAIL	github.com/adams-shaun/gorge/effects	0.666s
--- FAIL: TestAncientImperiosaurEntersWithTwoCountersPerConvoker (0.64s)
    convoked_amount_test.go:127: precondition: Object.Convoked = [], want 2 creatures
--- FAIL: TestKnightErrantOfEosXCountsConvokers (0.00s)
    convoked_amount_test.go:147: precondition: Object.Convoked on the stack = &{... Convoked:[] ...}, want 2
FAIL	github.com/adams-shaun/gorge/rules	0.682s
```

Note that both end-to-end tests fail at the *precondition* that
`Object.Convoked` actually captured the two creatures — i.e. with the gate
reverted the test cannot even reach its counter assertion, which is the
correct loud failure. The effects test fails on the value itself (0 vs 2).

## Issues

- None found that this ticket did not fix. The extended provenance classifier
  `SpecUsesConvokedAmount` matches the whole `Convoked$` head family; the only
  such token in the corpus today is `Convoked$Amount` (2 files), so the gate's
  blast radius is measured and bounded to faces that read the count. If a
  future card writes a `Convoked$<Other>` head, the gate already covers it;
  the count dispatch will fail closed for the property it does not model —
  that is intended.
- No Known-approximations row existed for `Convoked$Amount`, so none was
  deleted and `knownApproximationRows` is unchanged (checked: AGENTS.md has no
  `Convoked` row).
