
# Merge-conflict resolution report — mrg1

This file is shared by several pipelines' merge resolvers (one tracked path in
`.ds4/`); each integration appends its own section below. Sections in order:
the earlier merge on this branch (commit 614a3bd9, ticket
cli-20260922T225139Z-230e9834), the main-side report that arrived with the
2026-09-23 merge (ticket cli-20260922T225141Z-8d166e9c), and the newest merge
that put both together.

---

## Section 1 — branch merge of main at 614a3bd9 (this worktree's ticket)

## Conflicts and resolution

- `internal/testutil/agentsdoc_test.go`: the branch's approximation row count and main's independently lowered count conflicted. Kept both sides' `AGENTS.md` row deletions and set `knownApproximationRows` to 74, matching the merged table's 74 data rows. This preserves the reviewed branch change while accounting for main's additional deletions.
- `AGENTS.md` and `rules/clone.go`: auto-merged; retained both sides' changes.
- Merge-fallback conflict in `.ds4/report-mrg1.md`: resolved by recording this integration's final outcome here. No engine code was changed during conflict resolution.

The operation is complete in merge commit `757ffdee` (`Merge main: resolve approximation row count conflict` — recorded as `614a3bd9` after a history rewrite). The working tree is clean. `.cards` is present.

## Commands and output

- `git status --short --branch; git rev-parse --abbrev-ref HEAD`
  ```
  ## wt/cli-20260922T225139Z-230e9834
  wt/cli-20260922T225139Z-230e9834
  ```
- `git status`
  ```
  On branch wt/cli-20260922T225139Z-230e9834
  nothing to commit, working tree clean
  ```
- `git show --stat --oneline HEAD; git status --short --branch; git rev-parse HEAD^1 HEAD^2` confirmed merge commit, clean branch, parents `f4c04e2f092e61ecb37b28335a95ade095874050` and `e53c80a2132990a82767efa346dd3ee6dafc79c4`.
- Corpus check: `.cards present`.
- `go test ./internal/testutil ./rules -run 'TestKnownApproximation|TestClone'`
  ```
  ok   github.com/adams-shaun/gorge/internal/testutil  0.001s
  ok   github.com/adams-shaun/gorge/rules  0.823s
  ```

No unresolved conflict or uncertainty remains.

---

## Section 2 — main-side report (ticket cli-20260922T225141Z-8d166e9c), arrived with the 2026-09-23 merge of main

# Merge-conflict resolution report — mrg1 (ticket cli-20260922T225141Z-8d166e9c)

---

# Merge conflict resolution: mrg1 round 2 (branch wt/cli-20260922T225138Z-c4106938, main at 84cb68b9)


## Starting state

The worktree was CLEAN, HEAD = `f14b56a3` (round 1's merge of main@e53c80a2,
status DONE), with no rebase/merge in flight — the daemon's failed rebase had
been rolled back, so the conflict dispatch was against a state that no longer
existed. `main` had since advanced by one ticket: `0679b1cd` (the
withForetell/withoutForetell + Cosmos Charger + effect-delivered MayPlay
closure, merged to main via `98dcf594`/`84cb68b9` of the sibling ticket
cli-20260922T225141Z-8d166e9c).


## Conflicts and resolution

---

Rebase is forbidden in this worktree (shared-`git` seat rule), and merge is
the established integration shape here, so the integration was done as
`git merge main`.


## Conflicted files and resolution

1. **`internal/testutil/agentsdoc_test.go`** — one conflict hunk: this branch's
   explanatory comment above `knownApproximationRows` (main never had it; both
   sides set the constant to 74). Resolved to **73**, the merged table's
   measured data-row count (74 on each side; the merge deletes main's
   foretell row — closed in `0679b1cd` — on top of this branch's
   stack-option-kind closure). Comment updated to state the new measurement.
   Never raised. `knownOversizeRows` stayed 8 on both sides (merged table
   measures 6 oversize rows — shrinkage only). gofmt clean.
2. **`.ds4/report-mrg1.md`** — this tracked report file carried each side's
   prior-round report (ours from round 1, main's from the sibling ticket).
   Replaced with THIS round's report per the report-path contract.
3. **`AGENTS.md`** — auto-merged with NO textual conflict. Verified the merged
   table with the test's own parsing algorithm: **73 data rows**, both closed
   rows absent (this branch's "A spell on the stack is offered with
   `Option.Kind` \"permanent\"…" row and main's foretell row), and
   `git diff main -- AGENTS.md` shows exactly this branch's one row deletion.

All other main-side changes (`effects/filter.go`, `effects/misc.go`,
`rules/legal.go`, `rules/mayplay.go`, `rules/playerkeywords.go`,
`state/continuous.go`, `rules/foretell_grant_test.go`,
`rules/paramcensus_test.go`) auto-merged and were retained unmodified — this
branch never touched those files.

## Ratchet cross-check after the merge

The brief's ratchet command list plus the agentsdoc ratchet itself. No ratchet
table needed fixing: this branch registers no new trigger `Mode$` matcher, and
main's foretell closure already removed its own `knownUnsupportedParams`
entries together with its AGENTS.md row; the merged tree's `knownUnsupported`
/ `knownUnsupportedParams` / `knownUnmodelledCountHeads` tables are otherwise
untouched by either side.

## Commands run and output

```
git merge main
  -> AGENTS.md auto-merged; .ds4/report-mrg1.md CONFLICT;
     internal/testutil/agentsdoc_test.go CONFLICT
python3 (the test's own parsing algorithm) on merged AGENTS.md
  -> data rows: 73; oversize: 6

go test ./internal/testutil -run 'TestKnownApproximation' 2>&1 | tail -3
  -> ok  github.com/adams-shaun/gorge/internal/testutil
go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead' 2>&1 | tail -3
  -> ok  github.com/adams-shaun/gorge/rules
go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -3
  -> ok  github.com/adams-shaun/gorge/cmd/botbench
go test ./internal/archtest/ 2>&1 | tail -3
  -> ok  github.com/adams-shaun/gorge/internal/archtest
gofmt -l internal/testutil/agentsdoc_test.go
  -> (no output, clean)
```

## Result

Merge commit with the default merge message; working tree clean after.

## Issues


---

## Section 3 — this merge (main 84cb68b9 into wt/cli-20260922T225139Z-230e9834)

### Starting state

The daemon's rebase of this branch onto main had conflicted
(`internal/testutil/agentsdoc_test.go` and `effects/...` hunks) and its merge
fallback had conflicted on `.ds4/report-mrg1.md`; both attempts were rolled back
before this seat started — `git status` showed a CLEAN tree at merge commit
`614a3bd9` ("Merge main: resolve approximation row count conflict"), which had
already integrated main up to `e53c80a2`. Main had since advanced 4 commits
(`0679b1cd` → `84cb68b9`, the foretell/Cosmos Charger/MayPlay ticket and its merge
chain). So the in-flight operation to finish was a fresh `git merge main`.

### Conflicted files and how each side's intent was kept

Only ONE file conflicted: `.ds4/report-mrg1.md`.

- **HEAD:** the branch-side resolver's report for THIS ticket's earlier merge
  (`614a3bd9`), which had set `knownApproximationRows` to 74 for the dig1 deletion.
- **main:** the OTHER ticket's (cli-20260922T225141Z-8d166e9c) merge report, which
  had set the same constant to 74 for the foretell-row deletion on its own tree.
- **Resolution:** both are legitimate reports for the one shared tracked path, so
  BOTH are kept — this file now carries both sections (Section 1 = HEAD's, Section 2
  = main's) plus this Section 3. No content was dropped.

`AGENTS.md` auto-merged cleanly: it now carries BOTH sides' row deletions — the
branch's `dig1` row (this ticket) AND main's `ft1` foretell row — so the merged table
measures **73 data rows** while the constant on BOTH sides says 74. As in both prior
merges, the constant was lowered to the measured merged value in this merge commit:
`knownApproximationRows = 74 → 73` in `internal/testutil/agentsdoc_test.go`. The
test now passes with no slack in either direction.

### Golden-head check (rules/heads_test.go)

`rules/heads_test.go` auto-merged but needed verification: both sides had history
with the 4-seat golden. The mana-colour ticket's `049bd58a7b940fbc` re-pin was
already an ancestor of HEAD (merged via `614a3bd9`); the branch's dig fix re-pinned
4 seats to `20028059e8c88ec3` on top of it, and main's foretell ticket did not move
that entry. The merge correctly keeps the branch's `20028059e8c88ec3`, and
`TestHeads` PASSES on the merged tree — so the branch-side value stands and no
re-pin was needed.

### Commands and output

- `git status` (before) → clean tree on `wt/cli-20260922T225139Z-230e9834` at `614a3bd9`.
- `git rev-list --count HEAD..main` → 4.
- `git merge main --no-commit --no-ff` →
  ```
  Auto-merging .ds4/report-mrg1.md
  CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
  Auto-merging AGENTS.md
  Automatic merge failed; fix conflicts and then commit the result.
  ```
  (all engine files auto-merged; `git status --short` → `UU .ds4/report-mrg1.md`)
- Merged-table row count (test's own counting method, python): merged AGENTS.md =
  73 data rows; HEAD = 74 (dig1 deleted); main = 74 (ft1 deleted, dig1 still
  present on main); base rows differ per side as the two sections above record.
- Edit `internal/testutil/agentsdoc_test.go`: `knownApproximationRows` 74 → 73.
- `go test ./internal/testutil/ -run 'TestKnownApproximation' -v` →
  ```
  --- PASS: TestKnownApproximationsOnlyShrinks
  --- PASS: TestKnownApproximationRowsAreShort
  ok  github.com/adams-shaun/gorge/internal/testutil  0.001s
  ```
  (exact match — no "table is down to" slack log)
- `go test ./rules -run 'TestHeads$' -v | tail` → `--- PASS: TestHeads (2.08s)`.
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeckIsFullySupported|TestEveryRepoDeckParamsAreRead|CountHead' -v` → all PASS (TestEveryRepoDeckIsFullySupported 0.63s, TestEveryRepoDeckParamsAreRead 0.13s, both trigger-mode registry tests PASS, TestEveryRepoDeckCountHeadResolves PASS), `ok github.com/adams-shaun/gorge/rules 0.799s`.
- Branch-fix sanity (the dig ticket's own suites on the merged tree):
  `go test ./effects -run 'Dig'` → ok; `go test ./rules -run 'Dig'` → ok;
  `go test ./rules -run 'Foretell|MayPlay'` (main's new suites) → ok.
- Behaviour goldens: `go test ./internal/archtest/` → ok; `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` → ok (split did not move).
- `.cards` present (real corpus symlink, `ir.gob.gz` resolves) — runs are real, not vacuous skips.

### Notes / uncertainties

- The only judgement call was the heads golden: because `TestHeads` passes
  unchanged on the merged tree with the branch's `20028059e8c88ec3` value, no
  attribution or re-pin was needed.
- The merged engine code combines the branch's dig fix with main's foretell/MayPlay
  changes to `effects/misc.go`, `effects/filter.go`, `rules/legal.go`,
  `rules/mayplay.go`, `rules/playerkeywords.go`, `state/continuous.go`; the dig,
  foretell and golden suites above all pass on the combination.
- No new trigger `Mode$` matcher and no new unmodelled count head came in from
  either side, so the registry ratchets needed no table edits beyond the row-count
  constant.

---

None found beyond the resolved conflict itself.


---

## Section 4 — current merge (main c12e10ef)

The worktree was clean at `2bae6903`; the reported daemon operation was no longer
in flight. Started `git merge main --no-commit --no-ff` to integrate the current
main tip. It conflicted in this report and `internal/testutil/agentsdoc_test.go`;
`AGENTS.md` auto-merged. Kept the pre-existing report sections from both sides,
including their distinct ticket histories, and retained the existing stack-option
kind closure report. The agentsdoc conflict's main-side comment documented 73
rows; the combined `AGENTS.md` measures 72 rows, so the constant is 72.

Commands and results:
- `git status --short --branch` before merge: clean on this branch.
- `git merge main --no-commit --no-ff`: conflicts in this report and
  `internal/testutil/agentsdoc_test.go`; `AGENTS.md` auto-merged.
- `go test ./internal/testutil -run 'TestKnownApproximation'`: `ok` (0.001s).
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`: `ok` (0.742s).

No engine code was conflicted.
