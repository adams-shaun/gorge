# Merge resolution report — cli-20260922T225140Z-677ee477

## Operation and conflicts

- Initial `git status --short --branch`: `## wt/cli-20260922T225140Z-677ee477`; working tree clean, no operation in progress.
- Ran `git merge main`. It stopped with one content conflict: `internal/testutil/agentsdoc_test.go`. `AGENTS.md` and the other main-side changes merged automatically.
- In the conflict, this branch had `knownApproximationRows = 55`; main had `57`. The branch's `AGENTS.md` had already removed the `(ft1)` bot-target approximation row. Main's changes remove the `(setname1)` row. The merge result preserves both closures (and main's code/tests), so I kept the `(ft1)` deletion, retained the `(setname1)` deletion, measured the merged table at 54 data rows, and resolved the constant to 54. Main's `57` did not describe the actual merged table.
- No other file had conflict markers. There were no unresolved behavioral choices.

## Commands and outputs

`git status --short --branch && git status`

```text
## wt/cli-20260922T225140Z-677ee477
On branch wt/cli-20260922T225140Z-677ee477
nothing to commit, working tree clean
```

`git merge main`

```text
Auto-merging AGENTS.md
Auto-merging internal/testutil/agentsdoc_test.go
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
Automatic merge failed; fix conflicts and then commit the result.
```

Resolved and staged `AGENTS.md` plus `internal/testutil/agentsdoc_test.go`, then ran `GIT_EDITOR=true git merge --continue`:

```text
[wt/cli-20260922T225140Z-677ee477 a70c4483] Merge branch 'main' into wt/cli-20260922T225140Z-677ee477
```

`.cards` check: `.cards present`.

`go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`

```text
ok  github.com/adams-shaun/gorge/rules  0.770s
```

`go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'`

```text
ok  github.com/adams-shaun/gorge/internal/testutil  0.001s
```

No uncertainties remain.

---

## Section 8 — this merge (main's limited-look integration into wt/cli-20260922T225138Z-e21c29e8)

### Starting state

The tree was CLEAN at `7bdae92f`; the daemon's rebase onto main and its merge
fallback had both been rolled back (the two conflicts described in
`.ds4/merge-conflict-mrg1.md`), so no operation was in flight. Main had
advanced past `00147db0` with the limited-look/spectator-search closure from
the sibling ticket (`30adfed9`, merge `ae220e74`). Rebase is forbidden in this
seat (shared-git rule), so the integration was done as `git merge main`.

### Conflicted files and how each side's intent was kept

- `internal/testutil/agentsdoc_test.go`: both sides carried the VALUE 71
  (HEAD's previous merge had already lowered it); only the comment text
  differed. Took main's comment verbatim — it is the accurate description of
  the merged table (main's 72 closures less the limited-look/spectator-search
  row deleted by the sibling branch).
- `.ds4/report-mrg1.md`: both kept — HEAD's Sections 5–6 verbatim, main's
  section appended as Section 7 (heading renumbered to avoid a duplicate
  "Section 5"), this round as Section 8.
- `AGENTS.md` auto-merged; verified the merged table measures exactly 71 data
  rows, matching the constant, with both sides' row deletions present.

### Commands and output

All runs in this worktree with the real `.cards` corpus symlink present.

- `git merge main --no-edit`:
  ```
  Auto-merging .ds4/report-mrg1.md
  CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
  Auto-merging AGENTS.md
  Auto-merging internal/testutil/agentsdoc_test.go
  CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
  Automatic merge failed; fix conflicts and then commit the result.
  ```
  (`git status --short` → `UU .ds4/report-mrg1.md`,
  `UU internal/testutil/agentsdoc_test.go`; AGENTS.md + all engine files
  auto-merged, including this branch's `rules/legal.go` cast-offer changes and
  main's `effects/zone.go` / `view/*` limited-look changes.)
- Merged AGENTS.md table measured: `awk '/^## Known approximations/,/^##
  Trigger-relative/' AGENTS.md | grep -c '^| '` → **71** — exact match with the
  constant, no slack either way.
- `go test ./internal/testutil -run 'TestKnownApproximation'`:
  ```
  ok  github.com/adams-shaun/gorge/internal/testutil  0.001s
  ```
- Ratchets + this branch's cast-offer suites:
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|CastOffer|CastTarget|CastBound|CastLiveness'`
  → `ok  github.com/adams-shaun/gorge/rules  0.735s`.
- `go test ./rules -run 'TestHeads$'` → `ok ... 2.012s` (main's limited-look
  change did not move a head golden).
- Behaviour goldens: `go test ./internal/archtest/` → `ok ... 2.804s`;
  `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` →
  `ok ... 1.055s` (split did not move).

No engine code was conflicted; the merge is purely integration.

---

## Section N+1 — resolver round for the 2026-09-22/23 dispatch (branch wt/cli-20260922T225139Z-244016f9, main at 78a1d9a4)

## Starting state

The worktree was CLEAN at `620546f9` — no rebase or merge in flight. The
failed rebase the dispatch described had been rolled back; a prior seat had
already merged an older main tip (`af9fe768`, main at `00147db0`). Since then
main had advanced to `78a1d9a4` (the limited-look / cast-offer-census
integration), so this seat ran `git merge main` and resolved the two
conflicted files it produced.

## Conflicted files and resolution

1. `internal/testutil/agentsdoc_test.go` — only the `knownApproximationRows`
   rationale comment conflicted; both sides set the constant to 71, each with
   a stale story. Measured the MERGED `AGENTS.md` table with the test's own
   row-extraction logic: base `00147db0` held 72 data rows, the branch
   deleted the `(diguntil1)` row, main deleted the limited-look /
   spectator-search row AND the CR 601.2c cast-offer-census row — the union
   is **69**. Resolved to `knownApproximationRows = 69` with a comment naming
   all three deletions; both sides' "71" comments described only their own
   side and would have been false for the merged tree.
2. `.ds4/report-mrg1.md` — HEAD's closing paragraph vs main's appended
   "Section 2" report. Kept both: HEAD's paragraph, then main's section,
   markers dropped.

`AGENTS.md`, `decision/decision.go` and `effects/cardflow.go` auto-merged;
no engine code conflicted.

## Commands and output

- `git status` (start): `On branch wt/...; nothing to commit, working tree clean`.
- `git merge main` → conflicts in `.ds4/report-mrg1.md` and
  `internal/testutil/agentsdoc_test.go` (17 files total in the merge).
- Row measurement (`git show <ref>:AGENTS.md` piped through the test's
  region-bounded `^\| ` count): base 72, branch(af9fe768) 71, main 70,
  merged 69.
- `go test ./internal/testutil/ -run 'TestKnownApproximation' -v` →
  `--- PASS` for `TestKnownApproximationsOnlyShrinks` and
  `TestKnownApproximationRowsAreShort`, no stale-count log.
- `git add` both files; `git commit --no-edit` → merge commit `12329490`;
  `git status --short --branch` clean; `git merge-base --is-ancestor main HEAD` → merged.
- Ratchets: `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  → `ok  github.com/adams-shaun/gorge/rules  0.726s`.

## Issues

None found in the conflict resolution itself. One note: main's own
`agentsdoc_test.go` comment claimed 71 while main's table already held 70
rows (it accounted for only one of its two deletions); the merged-tree
constant 69 supersedes both comments.

---

## Section N+2 — searchmay1 (branch wt/cli-20260922T225139Z-18baec47, merge of main)

`git merge main` conflicted in `AGENTS.md`, `internal/testutil/agentsdoc_test.go`
and this report. `AGENTS.md`: both sides deleted DISJOINT rows — this branch the
`(searchmay1)` row, main the "No LIMITED-look grammar" row — so the resolution
keeps BOTH deletions (the conflict hunk collapses to nothing).
`internal/testutil/agentsdoc_test.go`: main 66, branch 73; resolved to the
measured merged count. This report resolved to main's log plus this section.

---

## Section N+3 — this merge (main f7357075 into wt/cli-20260922T225140Z-b686e452)

### Starting state

The tree was CLEAN at `d56e404f` (the branch's one commit: `non<X>` negation
closure, `effects/filter.go` + its test + the row deletion + constant 59). No
rebase or merge was in flight. Main was 4 commits ahead (`c93d85f7` →
`f7357075`, the scry/surveil pile-B-order closure and its web/decision/
botpolicy/arrange changes plus its merge chain). Ran `git merge main
--no-commit --no-ff`.

### Conflicted files and how each side's intent was kept

Only ONE file conflicted: `internal/testutil/agentsdoc_test.go`.

- **HEAD:** `knownApproximationRows = 59` (branch's non<X> row deletion).
- **main:** `knownApproximationRows = 58` (main's TWO Scry/Surveil pile-B row
  deletions).
- Both sides' deletions are disjoint; `AGENTS.md` auto-merged keeping BOTH —
  `grep` for the `non<X> negation` row, the `Scry sends its unchosen pile B`
  row and the `Surveil puts its unchosen pile B` row each finds 0 occurrences
  in the merged file.
- Merged table measured with the test's own parsing logic (python replication
  of `approximationRows`): base `f8496199` = 58 data rows, main = 56, HEAD =
  57, merged = **55** (58 − 3). Resolved the constant to **55** with a comment
  naming all three deletions. Note the base constant (60) already carried
  slack of 2; the merged value 55 removes all slack.
- `knownOversizeRows` = 8 on both sides; the merged table measures 6 oversize
  rows on every side (none of the three deleted rows was oversize), so the
  constant is untouched — shrinkage only.

No engine code conflicted: the branch's `effects/filter.go` change and main's
`rules/arrange.go`, `effects/cardflow.go`, `decision/decision.go`,
`botpolicy/policy.go`, `web/*` changes auto-merged (disjoint files/regions).

### Commands and output

- `.cards` present (real corpus symlink) — runs are real, not vacuous skips.
- `git merge main --no-commit --no-ff` →
  ```
  Auto-merging AGENTS.md
  Auto-merging internal/testutil/agentsdoc_test.go
  CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
  Automatic merge failed; fix conflicts and then commit the result.
  ```
  (`git status --short` → `UU internal/testutil/agentsdoc_test.go` only)
- `go test ./internal/testutil -run 'TestKnownApproximation' -v` (after 55):
  `--- PASS` both tests, `ok ... 0.004s` — exact match, no slack log.
- Branch-fix sanity: `go test ./effects -run 'NonPredicate|NonCopied'` →
  `ok ... 0.703s`.
- Main-side sanity: `go test ./rules -run 'Arrange'` → `ok ... 0.777s`.
- Ratchets: `go test ./rules -run 'TestNoTriggerModeIsRegistered|
  TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|
  CountHead'` → `ok ... 0.885s`.
- `go test ./rules -run 'TestHeads$'` → `ok ... 1.873s` (no head golden moved).
- Behaviour goldens: `go test ./internal/archtest/` → `ok ... 3.156s`;
  `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` →
  `ok ... 1.197s` (split did not move).
- `gofmt -l internal/testutil/agentsdoc_test.go` → clean; no conflict markers
  remain in any .go file.
- `git commit --no-edit` → merge commit `4bd357c1`; `git status --short
  --branch` → clean.

### Issues

None found beyond the resolved conflict itself.

---

## Section N+4 — verification in this resolver session

The dispatch arrived after the integration had already been completed: initial
`git status` was clean at `e262a3a4`, with merge commit `4bd357c1` underneath
it and no rebase or merge in progress. The merge commit records the sole
conflict (`internal/testutil/agentsdoc_test.go`); the preceding Section N+3
records both sides' intended row-count values, the combined AGENTS.md deletions,
and the resolution to the merged table's measured count of 55. `AGENTS.md`
auto-merged, retaining the branch's `non<X>` row deletion and main's two
Scry/Surveil row deletions.

I found a duplicated pair of `knownApproximationRows` explanation comments
left by the resolution and removed the duplicate; the constant remains 55.
This was the only additional edit. `.cards` is present in this worktree.

Commands and output:

```text
git status --short --branch
## wt/cli-20260922T225140Z-b686e452

[ -e .cards ] && echo '.cards present' || echo '.cards missing'
.cards present

go test ./internal/testutil -run 'TestKnownApproximation' -v
=== RUN   TestKnownApproximationsOnlyShrinks
--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)
=== RUN   TestKnownApproximationRowsAreShort
--- PASS: TestKnownApproximationRowsAreShort (0.00s)
PASS
ok   github.com/adams-shaun/gorge/internal/testutil 0.001s

go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok   github.com/adams-shaun/gorge/rules 0.783s
```

No conflict markers remain in the affected Go file. No engine behavior or ratchet
entries changed, and there are no additional issues or uncertainties.

### Fresh resolver verification

On this dispatch, the worktree was already clean at `f76e75ad`, following merge
commit `4bd357c1`; there was no rebase or merge operation in progress. The
conflict resolution and its report were already committed. `.cards` is present.

Commands and output from this verification:

```text
go test ./internal/testutil -run 'TestKnownApproximation' -v
=== RUN   TestKnownApproximationsOnlyShrinks
--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)
=== RUN   TestKnownApproximationRowsAreShort
--- PASS: TestKnownApproximationRowsAreShort (0.00s)
PASS
ok   github.com/adams-shaun/gorge/internal/testutil  0.001s

go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok   github.com/adams-shaun/gorge/rules  0.813s
```

The merged table and constant remain 55. No further conflict or uncertainty.

---

## Section N+5 — current dispatch (cli-20260922T225140Z-b686e452)

### Starting state and integration status

`git status --short --branch` and `git status` showed a clean tree on
`wt/cli-20260922T225140Z-b686e452`. No rebase or merge was in progress.
The failed rebase/fallback described by `.ds4/merge-conflict-mrg1.md` had
already been completed in merge commit `4bd357c1` (parents `d56e404f` and
`f7357075`); subsequent report-only commits were already present at entry.
Thus there was no in-flight operation to continue and no new conflict edit was
needed in this session.

### Conflicted files and resolution

- `internal/testutil/agentsdoc_test.go` was the sole conflict. The branch side
  carried `knownApproximationRows = 59` for its one deleted `non<X>` row;
  main carried 58 for its two Scry/Surveil pile-B deletions. `AGENTS.md`
  auto-merged and retains all three disjoint deletions. The recorded merge
  resolution sets the merged count to 55 (58 minus three), with the explanatory
  comment; it is present in `4bd357c1` and the focused test passes.
- `AGENTS.md` was auto-merged; its row deletions are retained. No engine code
  required conflict resolution.

The requested merge operation is complete. No additional commit was necessary;
the resolution is already committed as `4bd357c1`. The current tree remained
clean after tests. `.cards` is present in this worktree.

### Commands and output in this verification

```text
git status --short --branch; git status
## wt/cli-20260922T225140Z-b686e452
On branch wt/cli-20260922T225140Z-b686e452
nothing to commit, working tree clean

git show --format=fuller --no-patch 4bd357c1
commit 4bd357c1017189fc013dccf1f9cf32d98cbd4d7f
Merge: d56e404f f7357075

go test ./internal/testutil -run 'TestKnownApproximation' -v
=== RUN   TestKnownApproximationsOnlyShrinks
--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)
=== RUN   TestKnownApproximationRowsAreShort
--- PASS: TestKnownApproximationRowsAreShort (0.00s)
PASS
ok   github.com/adams-shaun/gorge/internal/testutil 0.001s

go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok   github.com/adams-shaun/gorge/rules 0.820s

[ -e .cards ] && echo '.cards present' || echo '.cards missing'
.cards present
```

No uncertainties remain about the reported conflict or its completed resolution.


---

## Current merge resolution — main `9016c22c`

The prior resolver report above is preserved. Main had advanced beyond the branch's earlier merge (`4bd357c1`), so this merge integrates current main; no rebase was in progress.

### Conflicts and resolution

- `internal/testutil/agentsdoc_test.go`: branch recorded 55 rows after its `non<X>` closure plus the two Scry/Surveil deletions. Main carries additional approximation-row deletions; after the combined disjoint deletions, the merged table measures 53. Updated the constant to 53. The merged table preserves main's and the branch's row removals.
- `.ds4/report-mrg1.md`: this is a shared accumulated report. Kept the branch's existing report history and main's trailing verification line (which refers to its own worktree) and added this round's resolution report; no history discarded.
- `AGENTS.md` auto-merged. All other main changes are retained; no engine-code conflict required manual resolution.

### Commands and output

- `git status --short --branch` before merge: clean on `wt/cli-20260922T225140Z-b686e452`.
- `git merge main --no-edit`:
  ```
  Auto-merging .ds4/report-mrg1.md
  CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
  Auto-merging AGENTS.md
  Auto-merging internal/testutil/agentsdoc_test.go
  CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
  Automatic merge failed; fix conflicts and then commit the result.
  ```
- Merged approximation count: **53** (`awk` over the merged `AGENTS.md` table); the test's measured count agrees.
- `go test ./internal/testutil -run 'TestKnownApproximation' -v`:
  ```
  === RUN   TestKnownApproximationsOnlyShrink
  --- PASS: TestKnownApproximationsOnlyShrink (0.00s)
  === RUN   TestKnownApproximationRowsAreShort
  --- PASS: TestKnownApproximationRowsAreShort (0.00s)
  PASS
  ok   github.com/adams-shaun/gorge/internal/testutil  0.002s
  ```
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`:
  ```
  ok   github.com/adams-shaun/gorge/rules  0.780s
  ```
- `gofmt -l internal/testutil/agentsdoc_test.go`, `git diff --check`, and a conflict-marker grep over the resolved report and Go file: no output.

- `git commit --no-edit` created merge commit `8eab585f` (parents: pre-merge branch `1b04c7c9`, main `9016c22c`).
- Final `git status --short --branch`:
  ```
  ## wt/cli-20260922T225140Z-b686e452
  ```
  `git merge-base --is-ancestor main HEAD` succeeded.

No unresolved conflict or uncertainty remains.

---

Final check from main's side of the report conflict: `git status --short --branch` returned only `## wt/cli-20260922T225140Z-677ee477` (clean), HEAD `a70c4483`; its own merged approximation table measured 54 data rows.
