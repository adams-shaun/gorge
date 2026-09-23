# Report — DestroyAll.Zone (fix round 2: restore accumulated report history)

## What this round changed and why

The only MAJOR finding was that round 1's commit `6ef01416` had replaced the
1,565-line accumulated `.ds4/report-t1.md` with this ticket's 42-line report,
deleting unrelated durable review history. This round restores it and records
this ticket narrowly.

- `.ds4/report-t1.md` — restored. The file is now **main's current 1,951-line
  accumulated report history with this ticket's 43-line report prepended at the
  top**. Verified that main's entire file survives below the prepend:
  `diff <(git show main:.ds4/report-t1.md) <(tail -n +44 .ds4/report-t1.md)`
  prints nothing ("MAIN CONTENT FULLY PRESERVED"), all 17 top-level report
  headings are present, and no conflict markers remain. This mirrors the
  precedent from commit `00118ef5` ("restore historical report after MustBlock
  verification"), which undid the same class of destructive rewrite the same
  way.
- `effects/zone.go`, `effects/destroyall_zone_test.go` — unchanged from the
  already-reviewed round-1 code fix (`Zone$` read, defaults to `Battlefield`,
  fails closed on an unknown zone word; victims collected and rechecked in the
  selected zone; battlefield-only indestructibility/regeneration/Umbra/batch
  handling left battlefield-scoped).
- `.ds4/report-t2.md` — this round's report is prepended at the top; the prior
  MustBlock verification report already in the file is **preserved below it**
  rather than overwritten, so this round's diff deletes no durable report
  either.

Rebase directive (2026-09-23T03:20:03Z) was followed: work was committed first,
then `git rebase main` was run. The code commit applied cleanly; the only
conflict was in `.ds4/report-t1.md`, resolved by keeping both main's accumulated
history and this ticket's report (the exact remedy the directive and the finding
name). The rebase completed and the branch is now based on `main @ 19b8fb3a`.

## Fails without the fix

The code fix and its failing proof are unchanged from round 1 and re-verified
here. I backed up `effects/zone.go` to `.ds4/scratch/zone.go.fixed`, removed the
`Zone$` read (restoring the battlefield-only default), ran the one test,
restored the file from the backup, and byte-compared it:

```text
$ go test -run '^TestDestroyAllUsesNamedZone$' ./effects/
--- FAIL: TestDestroyAllUsesNamedZone (0.00s)
    destroyall_zone_test.go:22: named-zone card moved to exile, want graveyard
FAIL
FAIL	github.com/adams-shaun/gorge/effects	0.002s
FAIL

$ cp .ds4/scratch/zone.go.fixed effects/zone.go
$ cmp .ds4/scratch/zone.go.fixed effects/zone.go
RESTORED_BYTE_IDENTICAL
```

## Gates run (this round, after the rebase)

```text
$ go build ./...
(no output; exit 0)

$ go test -run '^TestDestroyAllUsesNamedZone$' ./effects/
ok  	github.com/adams-shaun/gorge/effects	0.002s

$ go test ./internal/archtest/ 2>&1 | tail -5
ok  	github.com/adams-shaun/gorge/internal/archtest	3.685s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -5
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.368s

$ gofmt -l effects/zone.go effects/destroyall_zone_test.go
(no output; exit 0)
$ go run ./cmd/gentypes -check
(no output; exit 0)
$ git diff --check
(no output; exit 0)
```

`.cards` is a present symlink to `/home/sadams/projects/gorge/.cards`, so the
corpus-dependent reads were not silently skipped. The brief's prevalence claim
held:

```text
$ /usr/bin/grep -rlE 'DB\$ DestroyAll.*Zone\$' .cards/cardsfolder | wc -l
1
$ /usr/bin/grep -rlE 'DB\$ DestroyAll.*Zone\$' .cards/cardsfolder
.cards/cardsfolder/k/kindred_dominance.txt
```

## Review finding disposition

- [MAJOR] `.ds4/report-t1.md` deleted the accumulated report history — FIXED.
  Main's full 1,951-line history is restored with this ticket's report prepended
  (diff vs main is `+43` lines and no deletions), and the same preservation is
  applied to `.ds4/report-t2.md`. The 17 prior report headings, including
  "Task rv1 — RevealAllValid$", "kw:Backup" and "Count$ResolvedThisTurn", are
  all present.

## Final diff vs main

```text
 .ds4/report-t1.md               | 43 ++++++++++++++++++++++++++++++++++++++++
 .ds4/report-t2.md               |  .. (this report prepended, MustBlock report preserved)
 effects/destroyall_zone_test.go | 30 ++++++++++++++++++++++++++++++++
 effects/zone.go                 | 34 ++++++++++++++++++++------------
```

## Issues

- None new. The card Kindred Dominance still needs its other half — the
  `Creature.IsNotChosenType` filter predicate — which is the separate ticket
  `agent-20260918T201120Z-c09a9312`; the param-census row for `DestroyAll.Zone`
  cannot retire until both land. This ticket's half (reading `Zone$`) is done.
- Process note (not a code defect): `.ds4/report-t1.md` is a shared append target
  reused across tickets, and a fresh round that writes it from scratch silently
  destroys other tickets' durable reports. The structural guard would be for the
  harness to refuse a shrinking write to a tracked report file (or to route each
  ticket to its own path). I did not change the harness; I followed the
  established restore-and-prepend convention.


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

---

Historical report preserved verbatim below from the main lineage (MustBlock verification round, agent-20260923T072310Z-8affc438); it belongs to a separate task and is not a finding of the Convoked$Amount ticket.
---

# Report — Verify and pin multiple MustBlock blockers

## Changes

Restored `.ds4/report-t1.md` byte-for-byte from the parent of `80d29498`, undoing that commit's unrelated destructive rewrite (review finding). This round's report is only in `.ds4/report-t2.md`. No production files or tests changed. The existing `TestMustBlockTwoWatchdogsShareAttacker` regression test is already present in `rules/mustblock_min_team_test.go` and the fix is already landed in `4fe4eadc` (`fix(rules): satisfy MustBlock with legal whole blocking teams`). No ratchet, allowlist, or Known-approximations entry changed.

`.cards` is a present symlink to the real corpus in this worktree, so the corpus-dependent test was not silently skipped for lack of corpus. `/usr/bin/grep -rlE 'Mode\$ MustBlock' .cards/cardsfolder | wc -l` returned `27`, matching the brief.

## Gates run

Exact targeted command from the brief:

```text
$ go test -run 'TestMustBlockTwoWatchdogsShareAttacker$' ./rules/ 2>&1 | tail -30
ok   github.com/adams-shaun/gorge/rules (cached)
```

```text
$ go test ./internal/archtest/ 2>&1 | tail -15
ok   github.com/adams-shaun/gorge/internal/archtest (cached)

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -5
ok   github.com/adams-shaun/gorge/cmd/botbench (cached)
```

All three gates passed in this round. The Go test cache returned the results as shown; no code or tests were changed during this verification.

## Fails without the fix

Not applicable: no test was added. The already-existing regression test is part of the fixing commit `4fe4eadc`; this task did not revert or alter production code.

## Issues

This defect is already fixed by `4fe4eadc`. No other defect was investigated or fixed. The reported prevalence of 27 corpus files describes the mechanic, not a remaining defect; no acceptance census or approximation entry requires a change.
