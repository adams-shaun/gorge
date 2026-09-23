# Merge-conflict resolution — task cli-20260922T225140Z-c661fa12

## Situation found

`git status` was CLEAN on entry, no rebase or merge in flight, but the branch
was NOT yet integrated with the current main:

- branch tip `17ca3a16` — a merge of the reviewed fix `63c07260` with the OLD
  main `0104252d`, committed by the PRIOR resolver round (its report is what
  occupied this file before; the daemon had tracked it).
- `main` had since advanced to `e6a2a84d` (the CR 616.1 replacement-order
  work: `4dfb4d74`, `5b8e6e62`, `7a45ef9a`; the EachDamage closure `49a2fde8`,
  `0c7e5ce9`; merges `38d1a4b0`, `a57b96e6`).
- reflog confirmed the daemon's rebase onto `e6a2a84d` had been started and
  **aborted**, leaving `17ca3a16` with `e6a2a84d` NOT an ancestor.

So the integration owed was a merge of the CURRENT main `e6a2a84d` into the
branch. `git rebase` is forbidden in a seat, so I used `git merge main`, the
same method the prior round used.

## Conflicted file: `.ds4/report-mrg1.md` (only content conflict)

`git merge main` auto-merged all code and `AGENTS.md`, and conflicted ONLY on
this scratch report file — because each resolver round had previously
committed its own version of it (the branch's `17ca3a16` carried the prior
round's report; main's `e6a2a84d` carried a different ticket's report,
`62421b89`'s). Neither side is source of truth: the file records a resolution
that has already happened. **Resolution:** replaced it wholesale with THIS
round's report.

## Auto-merged files — verification of both sides' intent

The merge auto-merged without markers:

- `AGENTS.md` — main's side deleted the `(each1)` row (`49a2fde8` closed it)
  and the two CR 616.1 replacement-order rows (`4dfb4d74` closed them) and
  added the CR 704.5j legend-rule row; the branch's side had already deleted
  the "Layer-4 type grants reach only the layer walk" row (that IS the fix
  `63c07260`). Both sets of deletions survive, the legend-rule row is kept.
  Verified: `(pc1)`, `(each1)`, `all-Updated`, `Layer-4 type grants` all
  absent; `704.5j legend rule` present.
- `rules/engine.go` — main's `askNextReplacementChoice` drain in `Submit`
  (line 2560) AND the branch's `layer4InPool`/`layer4Types` fields
  (lines 328/340) both present.
- `rules/paramcensus_test.go` — main's own correction (drops the stale
  `Engine.applyAddCounterBody`/`applyAddCounterReplacements` entries and adds
  `Engine.counterReplaceOp`) survives; no stale-entry failure.
- `effects/filter.go` — the branch's `SpecContext.DerivedTypes` (lines 3186,
  3257) and the published-table guard over main's pc1 predicates compose.
- `rules/clone.go`, `rules/layers.go`, `rules/statics.go` — branch layer-4
  additions, no contradiction.

## `internal/testutil/agentsdoc_test.go` — constant lowered to the measured count

The auto-merge kept the branch's `knownApproximationRows = 48`, but the merged
table is smaller: main deleted `(each1)` + the two replacement rows (3) and
added the legend-rule row (1); the branch deleted layer-4 (1). Measured with
the test's own line logic (`## Known approximations` … next `## `, `| ` lines,
minus header):

| ref | rows |
|---|---|
| base `0104252d` | 49 |
| branch `17ca3a16` | 48 (deleted layer-4) |
| main `e6a2a84d` | 47 (deleted each1 + two replacement rows; added legend-rule) |
| merged | **46** (49 − layer-4 − each1 − 2 replacement + legend-rule) |

Set `knownApproximationRows = 46` in the same commit. The test fails only on
GROWTH and merely logs on shrinkage, so 48 would not have failed — but it would
have been a stale ratchet and the test itself asks for the measured count
(same method as main's precedent `4dcef2ea`).

## Commands run and output

- `.cards` check: **present** (real symlink → `/home/sadams/projects/gorge/.cards`),
  so the corpus-backed ratchets were real, not vacuous skips.
- `git merge main` → `CONFLICT (content): Merge conflict in .ds4/report-mrg1.md`;
  `AGENTS.md` and `rules/engine.go` auto-merged; 14 files total.
- `go test ./internal/testutil/ -run 'TestKnownApproximations' -v`
  → `--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)` / `ok`.
- Post-merge ratchets:
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  → `ok github.com/adams-shaun/gorge/rules 0.797s`.
- Branch fix still green: `go test ./rules -run 'TestLayer4'`
  → `ok github.com/adams-shaun/gorge/rules 0.626s`.
- main's closures still green:
  `go test ./effects -run 'TestPlayerSpecFx20Grammar|TestUnknownPredicates'`
  → `ok github.com/adams-shaun/gorge/effects 0.005s`.
- Final combined run on the committed merge `4cc5c20a`:
  `go test ./internal/testutil/ -run 'TestKnownApproximations'` → `ok 0.001s`;
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|TestLayer4'`
  → `ok github.com/adams-shaun/gorge/rules 0.793s`.

## Uncertainties / notes

- The only content conflict was this report file; the resolution there is a
  wholesale replacement, which is the correct behaviour for a scratch record.
- No engine behaviour beyond the reviewed fix `63c07260` was introduced; the
  resolution itself touched only `AGENTS.md` (main's own delete/add, preserved
  by auto-merge), the register constant, and this report.
- `git diff --stat main` equals the branch's own fix stat (17 code/doc files +
  the constant + this report), confirming no unintended main content was
  dropped or duplicated.

## Issues

None found during this round's resolution.
