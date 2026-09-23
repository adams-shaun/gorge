# Merge-conflict resolution report — mrg1

This report preserves earlier merge-resolver reports in Sections 1–5. This
section records verification of the current merge commit for
`wt/cli-20260922T225139Z-244016f9`.

## Current merge verification

The conflict dispatch described a failed rebase on `b67c4d3d` and fallback
merge. At inspection, the worktree was already clean at merge commit
`af9fe768` (`Merge branch 'main' into wt/cli-20260922T225139Z-244016f9`),
with parents `eb3dfd2e` (approved branch changes) and `00147db0` (main-side
Dig closure). No rebase or merge remained in progress. The merge commit records
conflicts in `AGENTS.md` and `internal/testutil/agentsdoc_test.go`; the earlier
report section records how those were resolved. The `AGENTS.md` DigUntil row
was deleted while preserving main's separate deletion, and the approximation
row count is 71. `decision/decision.go` and `effects/cardflow.go` merged with
both sides' changes. No conflict markers remain.

## Commands and output

- `git status --short --branch; git status`:
  ```
  ## wt/cli-20260922T225139Z-244016f9
  On branch wt/cli-20260922T225139Z-244016f9
  nothing to commit, working tree clean
  ```
- `git show --stat --oneline HEAD` confirmed merge commit `af9fe768` and its
  conflict paths.
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead' 2>&1 | tail -30`:
  ```
  ok   github.com/adams-shaun/gorge/rules 0.750s
  ```

The requested ratchets passed. No uncertainty remains about conflict
resolution; the merge operation was already completed before this verification
seat started.
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

---

## Section 5 — previous round on this branch (ticket cli-20260922T225138Z-e21c29e8, merge d8fb30c6 of main c12e10ef)

# Merge-conflict resolution: mrg1

## Starting state

The worktree was clean on `wt/cli-20260922T225138Z-e21c29e8`, with no rebase or merge in progress. HEAD was `d8fb30c6`, a merge commit integrating `main`; the failed daemon operation described in `.ds4/merge-conflict-mrg1.md` had already been resolved and committed in this worktree. The committed merge includes the conflict resolution for `internal/testutil/agentsdoc_test.go`, and the prior merge report records the resolution of the fallback conflict in `.ds4/report-mrg1.md`. No conflict markers remain in `internal/testutil/agentsdoc_test.go`.

## Conflicted files and resolution

- `internal/testutil/agentsdoc_test.go`: reported as a content conflict while replaying `f68b9396`. The committed resolution sets `knownApproximationRows = 73`, agreeing with the merged `AGENTS.md` table. The existing focused ratchet test passed.
- `.ds4/report-mrg1.md`: the merge fallback reported conflicting report histories. It is now replaced with this round's report, preserving the required durable account of the completed integration.
- `AGENTS.md` and `rules/legal.go`: the daemon reported automatic merges, not conflicts. They are present in the already-committed merge.

No new engine changes were made in this resolver session. The merge operation was already complete at entry, so there was no rebase/merge command to continue.

## Commands and output

```text
git status --short --branch
## wt/cli-20260922T225138Z-e21c29e8
(clean)

git log --oneline --decorate -10
... d8fb30c6 Merge branch 'main' into wt/cli-20260922T225138Z-e21c29e8

git grep -n -E '<<<<<<<|>>>>>>>' -- internal/testutil/agentsdoc_test.go
(no matches)

[ -e .cards ] && echo '.cards present'
.cards present

go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|CastTarget|CastOffer'
ok   github.com/adams-shaun/gorge/rules 0.740s
```

The worktree has the real `.cards` corpus, so this was not a corpus-omitted run.

## Ratchets and concerns

The requested rules ratchets and target-offer tests passed. The merged approximation count is 73. No uncertainty remains about the conflict resolution itself; the merge commit predates this resolver session, and this session only updated the durable report.

## Issues

None found.

---

## Section 6 — this merge (main 00147db0 into wt/cli-20260922T225138Z-e21c29e8)

### Starting state

The tree was CLEAN at `3f71fd81`; the reflog showed the daemon had attempted the
rebase onto main four times (22:29–22:35) and rolled each one back, so no
operation was in flight. Main had advanced 6 commits since the branch's last
merge (`d8fb30c6`): the dig1 closure (`b43ab0a9` + its heads re-pin `f4c04e2f`)
and its merge chain up to `00147db0`. Rebase is forbidden in this seat
(shared-`git` rule), so the integration was done as `git merge main`.

### Conflicted files and how each side's intent was kept

Only ONE file conflicted: `.ds4/report-mrg1.md` (the shared append-log of
resolver reports). AGENTS.md, `internal/testutil/agentsdoc_test.go`,
`rules/heads_test.go`, `rules/legal.go`, `rules/cast.go` and all engine files
auto-merged.

- **HEAD:** this ticket's previous-round report (merge `d8fb30c6` of main
  `c12e10ef`).
- **main:** the accumulated resolver-report log (Sections 1–4, tickets
  230e9834 / 8d166e9c / c4106938).
- **Resolution:** BOTH kept per the file's own convention — main's Sections
  1–4 verbatim, HEAD's report preserved as Section 5, and this round's
  account as Section 6. No content dropped.

`internal/testutil/agentsdoc_test.go` auto-merged to main's constant (72), but
the merged table measures **71 rows**: both sides deleted one row each from the
72-row base — this branch's row 24 (the CR 601.2c cast-offer census row,
`castTargetsAvailable`) and main's dig1 row. The constant was lowered to 71 in
this merge commit; the ratchet now matches exactly with no slack either way.
Verified the merged AGENTS.md carries BOTH deletions: `grep -c 'cast-offer
census'` → 0 (branch's row gone), main's dig1 row gone while the separate
`(diguntil1)` row correctly remains.

`rules/heads_test.go`: main's dig ticket re-pinned the 4-seat golden; this
branch never touched it, so the auto-merge keeps main's pin, and `TestHeads`
passes on the merged tree — no re-pin needed.

### Commands and output

- `.cards` present (real corpus symlink, `ir.gob.gz` resolves) — runs are real.
- `git merge main --no-edit`:
  ```
  Auto-merging .ds4/report-mrg1.md
  CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
  Auto-merging AGENTS.md
  Automatic merge failed; fix conflicts and then commit the result.
  ```
  (`git status --short` → `UU .ds4/report-mrg1.md` only)
- `go test ./internal/testutil/ -run 'TestKnownApproximation' -v` (after 72 → 71):
  ```
  --- PASS: TestKnownApproximationsOnlyShrinks
  --- PASS: TestKnownApproximationRowsAreShort
  ok  github.com/adams-shaun/gorge/internal/testutil  0.001s
  ```
  (exact match — no slack log)
- Ratchets + this ticket's cast-offer suites:
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|CastOffer|CastTarget|CastBound|CastLiveness' -v`
  → all PASS (`TestEveryRepoDeckIsFullySupported` 0.58s,
  `TestEveryRepoDeckParamsAreRead` 0.13s, both trigger-registry tests PASS,
  all cast-offer census suites PASS), `ok github.com/adams-shaun/gorge/rules 0.780s`.
- `go test ./rules -run 'TestHeads$|Dig'` → ok (1.847s);
  `go test ./effects -run 'Dig'` → ok (0.597s) — main's dig closure intact.
- Behaviour goldens: `go test ./internal/archtest/` → ok (3.035s);
  `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` → ok
  (1.060s, split did not move).
- `gofmt -l` on the touched Go files → clean; `go run ./cmd/gentypes -check` → clean.

### Issues

None found beyond the resolved conflict itself.

---

## Section 7 — worktree cli-20260922T225139Z-30adfed9 (ticket cli-20260922T225139Z-30adfed9, limited-look/spectator closure; kept from main's side of the conflict)

Two integration rounds landed on this branch.

Round 1 (before this dispatch): the branch merged main at `efd2ff45` (merge commit
`6b9a83d9`, resolution constant 76 for that round's table) and recorded its
report here. Main has since advanced 22 commits to `00147db0`, so the branch was
behind again and the daemon's gate kept failing on the stale integration.

Round 2 (this dispatch): started from a clean tree on
`wt/cli-20260922T225139Z-30adfed9` at `c9f084d8`; no rebase/merge was in flight
(the daemon's failed rebase had been rolled back). Ran `git merge main` to
integrate main at `00147db0`.

- Conflicts: `.ds4/report-mrg1.md` (both sides edit this shared report) and
  `internal/testutil/agentsdoc_test.go` (the `knownApproximationRows` constant:
  branch said 76, main said 72). `AGENTS.md` auto-merged — it retains all of
  main's row closures AND this branch's deletion of the "No LIMITED-look
  grammar" row (grep finds 0 occurrences in the merged file).
- `internal/testutil/agentsdoc_test.go`: resolved to **71** — the merged
  table's measured data rows (awk count over the `## Known approximations`
  section: 72 lines starting `| ` including the header row = 71 data rows;
  main's 72 less the branch's one row deletion).
- `.ds4/report-mrg1.md`: resolved as the union — main's multi-section ledger
  kept verbatim, this section appended.
- No engine code was conflicted; the merge is purely integration.

Commands and output (all in this worktree, `.cards` symlink present):

```
go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'
ok  github.com/adams-shaun/gorge/internal/testutil  0.001s

go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|TestHeads'
ok  github.com/adams-shaun/gorge/rules  1.818s

go test ./rules -run 'TestEveryRepoDeckIsFullySupported$|TestHeads$|TestEveryRepoDeckParamsAreRead$' -v   # skip check
--- PASS: TestEveryRepoDeckIsFullySupported (0.57s)
--- PASS: TestEveryRepoDeckParamsAreRead (0.13s)
--- PASS: TestHeads (1.20s)      # no SKIPs; corpus-backed runs are real

go test ./internal/archtest/
ok  github.com/adams-shaun/gorge/internal/archtest  2.926s

go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  github.com/adams-shaun/gorge/cmd/botbench  1.049s
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
