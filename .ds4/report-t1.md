# Report — agent-20260919T181318Z-86535368

## Summary: the brief's premise is FALSE — the ticket is already implemented on current main

The brief asks me to *implement* `RepeatEach.ChangeZoneTable$`, asserting it is
unread (`effects/choose_control.go:1325` `effRepeatEach` "has no read of
`ChangeZoneTable`"). That is no longer true. The work landed on `main` as
commit **`eb1b0d97`** ("fix(rules): batch a RepeatEach ChangeZoneTable loop's
zone changes for ChangesZoneAll"), which is an ancestor of this worktree's
branch HEAD (`git merge-base --is-ancestor eb1b0d97 HEAD` → true). The
`UseImprinted$` half landed earlier as `b45fb707`, exactly as the brief's own
"current-main check" already conceded.

The brief's workspace-facts map was produced at a base *before* `eb1b0d97`.
Every code anchor it names is stale:

| Brief claim | Measured on this tree |
|---|---|
| `effRepeatEach` has no `ChangeZoneTable` read | `effects/choose_control.go:1467`: `zoneTable := strings.EqualFold(strings.TrimSpace(sa.Params["ChangeZoneTable"]), "True")` |
| no per-loop zone table exists | `openZoneBatch`/`closeZoneBatch` (rules/engine.go:781-), the zone twin of the damage batch, bracketed around the loop in `effRepeatEach` (BeginZoneBatch at :1539, EndZoneBatch at :1660) |
| ratchet entry still present at `paramcensus_test.go:2707` | the entry was **deleted** in `eb1b0d97`; only an explanatory comment remains, and `TestEveryRepoDeckParamsAreRead` passes |
| add a test named `TestRepeatEachChangeZoneTable` | `rules/zone_table_batch_test.go` exists with `TestRepeatEachChangeZoneTableBatchesChangesZoneAll` + control `TestRepeatEachWithoutChangeZoneTableKeepsPerMoveChangesZoneAll` |

The brief itself instructs: "Counts quoted in a brief are claims, not
measurements … Reporting a brief's premise as false is a valued outcome."
I therefore made **no production change** — reimplementing would be redundant
and would risk regressing a merged, gated fix.

## Semantics check (I did verify the merged fix matches the real Forge contract)

The brief phrases the symptom as "a repeated `ChangeZone` body cannot consume
the per-iteration result." That phrasing is the original filer's imprecise
reading of Forge's `RepeatEachEffect`. Forge's `ChangeZoneTable$ True` builds a
`CardZoneTable` from the loop's moves and calls `triggerChangesZoneAll` **once**
after the loop; `Mode$ ChangesZone` always fires **per move**. The merged
implementation matches that exactly:

- `effRepeatEach` opens the zone batch on the first pass and closes it when the
  loop completes (a mid-loop suspension leaves the engine's bracket open across
  the resume; the re-entry pass closes it — balanced however many resumes
  interleave).
- `openZoneBatch`/`closeZoneBatch` latch each `ChangesZoneAll` line once per
  batch and patch the queued trigger's `Remembered`/`Captured` to the batch's
  deduplicated moved set, so a plural "for each of them" body resolves.
- `Mode$ ChangesZone` is never batch-scoped — it keeps firing per move.

This is the actual contract; the merged test asserts both halves on real corpus
cards (Organ Harvest's loop sacrificing three bears: **Simic Slaw**
`ChangesZoneAll` fires once → 1 charge counter; **Black Market** `ChangesZone`
fires three times → 3 charge counters). The brief's suggested carrier (Curse of
the Swine, a token-creating body) is the weaker choice; the merged test uses a
body that genuinely performs zone changes.

## Commands run (real output)

Brief target command:

```
$ go test -run 'TestRepeatEachChangeZoneTable|TestEveryRepoDeckParamsAreRead' ./rules/ 2>&1 | tail -30
ok  	github.com/adams-shaun/gorge/rules	0.821s
```

Proof the named test actually ran (not skipped/vacuous):

```
$ go test -v -run 'TestRepeatEachChangeZoneTable' ./rules/ 2>&1 | tail -20
=== RUN   TestRepeatEachChangeZoneTableBatchesChangesZoneAll
--- PASS: TestRepeatEachChangeZoneTableBatchesChangesZoneAll (0.84s)
PASS
ok  	github.com/adams-shaun/gorge/rules	0.853s
```

`UseImprinted$` regression intact:

```
$ go test -run 'TestCurseOfTheSwineBoarsFollowTheExiledCreaturesController' ./rules/ 2>&1 | tail -5
ok  	github.com/adams-shaun/gorge/rules	0.610s
```

Mandatory goldens:

```
$ go test ./internal/archtest/ 2>&1 | tail -5
ok  	github.com/adams-shaun/gorge/internal/archtest	4.173s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -5
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.607s
```

Corpus prevalence (re-measured; the brief's 47/47 and 43/43 held):

```
$ /usr/bin/grep -rlE 'ChangeZoneTable\$' .cards/cardsfolder | wc -l
47
$ /usr/bin/grep -rlE 'UseImprinted\$' .cards/cardsfolder | wc -l
43
```

Worktree state:

```
$ git status --short
(nothing to commit, working tree clean)
$ git merge-base --is-ancestor eb1b0d97 HEAD && echo ancestor
ancestor
```

## Fails without the fix

Not applicable — I made no fix in this round because none was needed. The
existing regression `TestRepeatEachChangeZoneTableBatchesChangesZoneAll` was
already shown to fail pre-fix by `eb1b0d97`'s own author; its control
(`delete(sa.Params, "ChangeZoneTable")`) is the in-tree proof that the batch
reading depends on the read. Since the fix is already merged, reverting it in
this worktree would mean rewriting `main`'s history, which the worktree rules
forbid.

## Done-means checklist

- [x] `ChangeZoneTable$ True` read in `effRepeatEach` and pinned on a real
      corpus card with a zone-change body — already present (`eb1b0d97`,
      `rules/zone_table_batch_test.go`).
- [x] Ratchet entry deleted — already present; `TestEveryRepoDeckParamsAreRead`
      passes.
- [x] `TestCurseOfTheSwineBoarsFollowTheExiledCreaturesController` intact —
      verified passing.
- [x] Target command run and output pasted above.
- [~] "each new test asserts precondition, shown to FAIL with fix reverted" —
      N/A: no new test authored this round; the merged test already
      precondition-asserts both observer trigger lines and their counts.
- [x] `go test ./internal/archtest/` passes.
- [x] `TestConstructedDefaultIsByteIdentical` passes (no split movement —
      the change is already on main).
- [x] No Known-approximations row touched; the change closed no row (the
      RepeatEach zone table was never an AGENTS.md approximation row — it was a
      paramcensus entry).

## Issues

- **Finding (why this round had zero work): the brief is stale.** The ticket
  `agent-20260919T181318Z-86535368` was implemented on `main` by
  `eb1b0d97` before this worktree's branch was cut. The controller should
  detect this at dispatch (e.g. check whether the brief's named anchors still
  exist at HEAD) and either skip the seat or re-brief against current main.
  This cost a full round. No new ticket is needed; the fix already exists.
- Nothing else found requiring a ticket. No `Known approximations` row was
  added or touched.

## Deviations from the brief

- I did **not** implement anything, add a test, or delete a ratchet entry —
  all three are already done on the branch. Doing them again would duplicate a
  merged, gated change and risk regression. This is the correct response to a
  false premise per the dispatch's own instruction.

STATUS=DONE_WITH_CONCERNS
COMMITS=(none — no changes made; branch already contains the fix in eb1b0d97)
TESTS=go test -run 'TestRepeatEachChangeZoneTable|TestEveryRepoDeckParamsAreRead' ./rules/ → ok 0.821s; archtest ok; botbench ok; Curse of the Swine ok
