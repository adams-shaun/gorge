# Round record — merge resolver, this worktree (ticket cli-20260923T060000Z-hlcz-imprint), 2026-09-23

## Entry state

`git status` found the tree CLEAN at the completed merge `519f86ff` (branch fix
`417cb9aa` + main `e1829bf9`): no rebase or merge in flight. The dispatch's
conflict set (`effects/zone.go`, `internal/testutil/agentsdoc_test.go`) was
already resolved there — verified, not redone:

- `effects/zone.go` keeps the branch's `Imprint$` retention in
  `applyLibrarySearch` (the `var imprinted []state.ObjID` accumulation and the
  post-move-zone-guarded batched `events.Imprint`), which main's side never
  carried; main's changes to other zone movers are untouched.
- `internal/testutil/agentsdoc_test.go` read `knownApproximationRows = 27`,
  matching the merged AGENTS.md (27 data rows) — the base-2341274c 30 minus
  this branch's hidden-library ChangeZone row deletion and main's `mtsp1`/
  `battle1` deletions (staticgoad1→ap1 was a net-zero swap).
- `417cb9aa` and `e1829bf9` were both ancestors; tree clean.

## Second integration — main advanced to `fac07856` mid-round

While verifying, `main` had moved `e1829bf9` → `fac07856` (the
`cli-20260923T060000Z-ctms-refhead` ticket: TriggeredCard$CastTotalManaSpent
reads the cast spend, deleting the `(castfilter1/2)` register row, plus the
layers-pt7kw `(kw:Flanking)` deletion already in its lineage). The integration
owed to main's tip was completed as `git merge main --no-edit`:

- `AGENTS.md` and `effects/zone.go` auto-merged — verified both intents
  survive: the branch's `Imprint$` fix AND main's flip-before-move
  `applyTransformed` reordering (CR 306.5b entry-face loyalty) are both
  present in the merged mover.
- ONE content conflict: `internal/testutil/agentsdoc_test.go` — the
  `knownApproximationRows` constant and its comment. Base `e1829bf9` measured
  28 data rows; the branch deleted the hidden-library ChangeZone row (28→27),
  main deleted `(castfilter1/2)` and `(kw:Flanking)` (28→26); the merged
  AGENTS.md measures **25** — verified with the test's own counting rule.
  Neither side's constant was right for the merge. Resolution:
  `knownApproximationRows = 25` with a comment naming all three disjoint
  closures.

## Commands and output

```text
git status                              # clean at 519f86ff, nothing in flight
git merge-base --is-ancestor main HEAD  # e1829bf9 yes; fac07856 NO
git merge main --no-edit
  Auto-merging AGENTS.md
  Auto-merging effects/zone.go
  Auto-merging internal/testutil/agentsdoc_test.go
  CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
row counts: base e1829bf9 = 28, HEAD = 27, main = 26, merged = 25
gofmt -l internal/testutil/agentsdoc_test.go effects/zone.go   # clean
go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' -v
  --- PASS: TestKnownApproximationsOnlyShrinks (0.00s)
  --- PASS: TestKnownApproximationRowsAreShort (0.00s)
  ok  github.com/adams-shaun/gorge/internal/testutil 0.002s
go test ./effects -run 'TestLibrarySearchExileImprintIsRetained|TestLibrarySearchNonExileImprintIsRetained' -v
  --- PASS: TestLibrarySearchExileImprintIsRetained (0.68s)
  --- PASS: TestLibrarySearchNonExileImprintIsRetained (0.00s)
  ok  github.com/adams-shaun/gorge/effects 0.695s
go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
  ok  github.com/adams-shaun/gorge/rules 0.819s
git add internal/testutil/agentsdoc_test.go && git commit --no-edit
  -> 5e6e0760 Merge branch 'main' into wt/cli-20260923T060000Z-hlcz-imprint
git status -> clean; main (fac07856) is now an ancestor of the branch
```

`.cards` is the real symlink to `/home/sadams/projects/gorge/.cards` — the
rules ratchet run (0.819s) is real, not a vacuous corpus-skipped pass.

## Issues

None new. Integration only; the merged state closes the hidden-library
ChangeZone register row (this ticket) plus main-side `(castfilter1/2)` and
`(kw:Flanking)` closures, measured at 25 data rows.

---

# Merge resolution report — mrg1

## Conflict

- `internal/testutil/agentsdoc_test.go`: main's side included the later `kw:Flanking` and `battle1` row deletions and set `knownApproximationRows` to 27; the reviewed branch also removed the `(castfilter1/2)` row and had the older constant 28. Kept all of main's updates and the branch's CTMS ref-head deletion, set the constant to 26, and updated the explanatory comment to describe the merged deletions. Measured the merged `AGENTS.md` table at 26 data rows before resolving. No other conflicted files.
- The merge auto-merged `AGENTS.md` and `rules/cast.go`, preserving both sides' changes; no manual changes were needed there.

## Commands and results

- `git status --short --branch; git status` before integration: `## wt/cli-20260923T060000Z-ctms-refhead`; clean, no operation in progress.
- `git merge main`: initially failed with a content conflict only in `internal/testutil/agentsdoc_test.go` (expected conflict); other files auto-merged.
- Counted rows from the staged merged `AGENTS.md`: `staged AGENTS data rows: 26`.
- `.cards` check: present as a symlink to `/home/sadams/projects/gorge/.cards`.
- `go test -run 'TestTriggeredCardCastTotalManaSpent|TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead' ./rules/`: `ok github.com/adams-shaun/gorge/rules 0.981s` (exit 0).
- `go test -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' ./internal/testutil/`: `ok github.com/adams-shaun/gorge/internal/testutil 0.001s` (exit 0).

## Uncertainty / concerns

None. Both requested ratchet groups and the conflict-specific CTMS tests passed.

---

# Merge resolution report — mrg1 (wt/cli-20260923T060000Z-manaexp-nonpool), 2026-09-23

## Entry state

`git status` found the tree CLEAN at branch tip `e7f775f6` (the convoke
ManaExpend fix: `b6d47f8b fix(rules): count Convoke mana for ManaExpend` +
`e7f775f6 test(rules): assert Convoke expend trigger resolves after decision
drain`). No rebase or merge in flight, no prior partial resolution — this
resolver ran the integration itself.

## Integration

`git merge main` (main at `f2599d19`). Auto-merged: AGENTS.md regions outside
the conflict, `rules/cast.go`, `rules/heads_test.go`, and ~60 other files from
main's lineage (hlcz-imprint, ctms-refhead, layers-pt7kw merges plus their
closure commits). TWO content conflicts:

1. **`AGENTS.md`** — the `(battle1)` / `(manaexpend1)` adjacent rows at one
   insertion point:
   - Base `e9ed29f0` carried BOTH rows (verified: `git show e9ed29f0:AGENTS.md`
     lines 236-237, and the `(manaexpend1)` row text is byte-identical
     base-vs-main — main did NOT touch that row's lines).
   - The branch DELETED `(manaexpend1)` (b6d47f8b: ManaExpend now counts
     convoke contributions alongside pool mana — the row's own "Removed by"
     condition met) and kept `(battle1)`.
   - Main DELETED `(battle1)` (b732b382: the CR 310.11 defeated battle is
     exiled and its owner may cast it transformed) and kept `(manaexpend1)`.
   - **Resolution: keep NEITHER row** — both closures are deliberate,
     disjoint, and reviewed; the merged table goes `(api:Clone)` →
     `(devthr1)` directly. Measured the merged table at **24 data rows**
     with the test's own `approximationRows()` rule (base 29 − 5 disjoint
     closures: manaexpend1 [branch], battle1, hlcz-imprint hidden-library
     ChangeZone row, castfilter1/2, kw:Flanking [main]).

2. **`internal/testutil/agentsdoc_test.go`** — the `knownApproximationRows`
   constant and comment: branch said 28, main said 25, neither right for the
   merge. Resolution: `knownApproximationRows = 24` with a merged comment
   naming all five disjoint closures and their commits/tickets.

## Risky auto-merge verified

`rules/cast.go` was auto-merged between main's 5390d8b3 (expend tally became
engine scratch; `payCast` folds every paid cast unconditionally) and the
branch's convoke change. The merged `payCast` computes
`manaSpentTotal(spentMana) + convokeManaSpent(pc.convoke)` — both intents
compose; `convokePayment` gained the `countsMana` field the branch's fix adds,
and `convokeManaSpent` survives the merge. Confirmed by both sides' tests
passing together (below).

## Commands and output

```text
git status            # clean at e7f775f6, nothing in flight
git merge main
  Auto-merging AGENTS.md        Auto-merging internal/testutil/agentsdoc_test.go
  Auto-merging rules/cast.go
  CONFLICT (content): Merge conflict in AGENTS.md
  CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
awk row-count of merged AGENTS.md -> 24
git diff --check      # no leftover markers after resolution
git add AGENTS.md internal/testutil/agentsdoc_test.go
go test ./internal/testutil/
  ok  github.com/adams-shaun/gorge/internal/testutil 1.266s
go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|ManaExpend|Convoke'
  ok  github.com/adams-shaun/gorge/rules 0.776s
  (verified non-vacuous with -v: 22 RUNs, all PASS — incl.
   TestEveryRepoDeckIsFullySupported, TestEveryRepoDeckParamsAreRead,
   TestTeapotSlingerManaExpendCountsConvoke, and the branch's convoke tests)
go test ./rules -run 'TestTeapotSlinger'   # main's expend tests on the merged cast.go
  ok  github.com/adams-shaun/gorge/rules 0.584s
gofmt -l <changed files>   # clean; go run ./cmd/gentypes -check clean
git commit --no-edit -> b0937567 Merge branch 'main' into wt/cli-20260923T060000Z-manaexp-nonpool
git status            # clean; main (f2599d19) is an ancestor of the branch
```

`.cards` is the real symlink to `/home/sadams/projects/gorge/.cards` — the
rules runs are real, not vacuous corpus-skipped passes.

## Uncertainty / concerns

None. The only judgement call was deleting BOTH conflicting AGENTS.md rows;
that follows from each row's deletion being a reviewed closure on its own side
(main never modified the `(manaexpend1)` lines the branch deleted, and the
branch never modified the `(battle1)` lines main deleted — verified
byte-identical against the merge base), so no side's intent was overridden.

## Issues

None new. Integration only; the merged state closes `(manaexpend1)` (branch)
plus main-side `(battle1)`, hlcz-imprint, `(castfilter1/2)` and `(kw:Flanking)`
closures, measured at 24 data rows.
