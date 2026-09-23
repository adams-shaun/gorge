# Merge-conflict resolution report — mrg1 (branch wt/cli-20260922T225142Z-1d4558a1, main at 0104252d)

## Situation found

The dispatch described a failed rebase (1/1, conflict in
`internal/testutil/agentsdoc_test.go`) and a failed merge fallback on commit
`6e77a1e8` ("feat(rules): price the composite CantBlockUnless block charge and
admit delivered statics"). On arrival the worktree was CLEAN at `6e77a1e8` with
no rebase or merge in progress — the daemon had aborted the in-flight
operation. I therefore performed the integration myself:

```
git merge main --no-edit
```

which auto-merged everything except one file:

```
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
```

## The conflict and its resolution

`internal/testutil/agentsdoc_test.go`, the `knownApproximationRows` ratchet
constant:

- **Branch side (6e77a1e8):** `knownApproximationRows = 56`. The branch's fix
  deleted the `(blockprop1)` row (stat:CantBlockUnless) from AGENTS.md —
  measured: branch AGENTS.md table = 54 rows (base 55), branch constant 56
  (already 2 above its actual, which the test tolerates since it only fails on
  growth).
- **Main side (0104252d):** `knownApproximationRows = 50`. Main deleted six
  rows; measured main table = 49 rows, constant 50.

The merge auto-resolved AGENTS.md itself (both deletions are disjoint: 7 rows
deleted from 55 → **48 rows in the merged table**, verified with the exact
counting logic of `approximationRows()` — lines with prefix `| ` between
`## Known approximations` and the next `## `, header row dropped).

**Resolution:** set the constant to **48**, the merged table's actual count,
with a comment naming both sides' deletions. This preserves both sides' intent
(row deletions from the branch's reviewed fix AND main's closures); the
constant is lowered, which the register always permits, and it is more
accurate than either side (both sides' constants were above their actuals, so
this also removes the pre-existing slack).

`AGENTS.md` itself needed no manual edit — git's auto-merge correctly deleted
all 7 rows (branch's `(blockprop1)` plus main's six).

## Commands and output

- `git status` (arrival): clean at 6e77a1e8, nothing in flight.
- `git log --oneline HEAD ^main`: exactly `6e77a1e8` (the reviewed fix) — one
  branch-side commit to integrate.
- `git merge main --no-edit`: conflict in `internal/testutil/agentsdoc_test.go`
  only; everything else auto-merged (AGENTS.md, botpolicy/, decision/,
  rules/…).
- Row counts (test's own algorithm): merged AGENTS.md 48, branch 54, main 49,
  merge base 55.
- `git add internal/testutil/agentsdoc_test.go && git commit --no-edit` →
  merge commit `e3aa3074` ("Merge branch 'main' into
  wt/cli-20260922T225142Z-1d4558a1").
- `git status` after: clean.
- `.cards` was already the correct symlink to the main checkout's corpus
  (verified before any test run — no vacuous green).
- `go test ./internal/testutil/` → `ok github.com/adams-shaun/gorge/internal/testutil 1.826s`
  (the ratchet tests: `TestKnownApproximationsOnlyShrinks` and
  `TestKnownApproximationRowsAreShort` pass at 48/48 and oversize 8).
- Post-merge ratchets:
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  → `ok github.com/adams-shaun/gorge/rules 0.781s`
- Behaviour goldens:
  `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` →
  `ok github.com/adams-shaun/gorge/cmd/botbench 1.074s` (the 20-game pinned
  split did not move under the merged engine).
  `go test ./internal/archtest/` → `ok ... 4.011s`.
- Conflict-marker sweep over the resolved package and AGENTS.md: none.

## Uncertainties

None material. The only judgement call was the constant's exact value; 48
equals the merged table's measured count and is at-or-below both sides'
values, so no gate can fail on it.


---

# Earlier merge-resolver reports (prior rounds, preserved)

# Merge-conflict resolution — task cli-20260922T225140Z-6a16cd8c

(Record 1 below is this ticket's first integration round, kept from the
branch side; record 2 is the 68ca4d95 legend-rule ticket's record that main
carried in via `08955738`. Both are kept verbatim; record 3 is appended by
the round-2 docs commit.)

## Starting state

`git status` found the tree CLEAN — no rebase or merge in flight. The
daemon's earlier attempt (rebase, then a merge fallback, both conflicting per
`.ds4/merge-conflict-mrg1.md`) had been fully aborted before this seat
started; branch tip was `ade1f41d` ("test(rules): assert player attachment
and descend replay folds"). I redid the integration as a **merge of `main`
into the branch** (`git merge main`), which reproduced exactly the two
conflicts the daemon saw: `AGENTS.md` and `internal/testutil/agentsdoc_test.go`.
No other files conflicted.

## Conflicted file 1: AGENTS.md (one hunk)

Both sides touched the same slot in the "Known approximations" table — each
had DELETED a different row that the other kept:

- **main side** deleted the **(ft1)** row ("Bot target selection is
  effect-blind except for a known literal damage amount"). Main closed ft1
  with real bot-policy arms: `botpolicy/replacement.go`,
  `botpolicy/target.go` / `botpolicy/combat.go` rewrites, commit
  `87d13658` "fix(botpolicy): add real arms for replacement order, multikick
  count and mutate placement" plus `4dcef2ea` which lowered the register
  constant to 51 "after ft1 closure on main".
- **branch side** deleted the **(fx20)** row ("`MatchesPlayerSpec` still
  fails closed on `EnchantedBy` … `counters_` and compound/space forms").
  The branch closed fx20 with its reviewed commits: `c85c39b7`
  "feat(effects): resolve Player.EnchantedController,
  TriggeredDefendingPlayer and counters_", `c3662d98`, `3cbf3e11`
  "fix(rules): close player-spec grammar with descend and player
  attachments", `ade1f41d`.

**Resolution:** delete BOTH rows — each side's deletion is a legitimate
closure, and the register is delete-only. Verified by counting with the
test's own row logic (`## Known approximations` … next `## `, lines starting
`| `, minus header): base `f7357075` = 56 rows, main = 51 (its constant),
branch = 55 (its constant 57 lags — allowed, the test only fails on growth),
merged = **50**.

## Conflicted file 2: internal/testutil/agentsdoc_test.go (one hunk)

Only the `knownApproximationRows` constant conflicted: base 58, branch 57
(fx20 deletion), main 51 (ft1 + earlier main closures).

**Resolution:** `knownApproximationRows = 50` — merged table = 50 rows.
`knownOversizeRows` (8) and `standInCellLimit` (600) are identical on all
three sides; no other constant changed.

## Commands run and output

- `git merge main` →
  `CONFLICT (content): Merge conflict in AGENTS.md`,
  `CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go`
  (reproduced the daemon's conflict set).
- After resolving and `git add`:
  `git commit --no-edit` →
  `[wt/cli-20260922T225140Z-6a16cd8c 9fc45eaf] Merge branch 'main' into wt/cli-20260922T225140Z-6a16cd8c`
- `git status --short` → clean (no conflicts left; only my later report
  edit, committed separately).
- `.cards` check: `[ -e .cards ]` → **present** (real symlink; corpus-backed
  runs are not vacuous).
- `go test ./internal/testutil/ -run 'TestKnownApproximations' -v` →
  `--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)` / `ok`.
- Ratchet pass after the merge (per the 2026-09-22 instruction):
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  → `ok github.com/adams-shaun/gorge/rules 0.894s`. Verified NOT a vacuous
  skip run: verbose listing shows all five tests RUN and PASS
  (TestEveryRepoDeckIsFullySupported 0.56s, TestEveryRepoDeckParamsAreRead
  0.12s, TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched,
  TestEveryDispatchedTriggerModeHasAMatcher,
  TestEveryRepoDeckCountHeadResolves) with **0 SKIPs**.

## Post-merge ratchet check (branch-vs-main interaction)

- The branch registers NO new `Mode$` trigger matcher (its diff touches
  `trigmatch_cast.go`/`trigmatch_combat.go`/`trigmatch_misc.go` only in
  existing matchers for the player-spec closure), so nothing needed adding
  to `addedAfterTheSplit`.
- The branch's new player-spec primitives did not require edits to
  `knownUnsupported` / `knownUnsupportedParams` / `knownUnmodelledCountHeads`
  — the ratchet pass above is green with the merged tables untouched.

## Commits

- `9fc45eaf` — Merge branch 'main' into wt/cli-20260922T225140Z-6a16cd8c
  (conflict resolution; both ft1 and fx20 rows deleted, constant 58→50
  across base→merged).
- This report commit (docs only).

## Unsure / notes

- The `.ds4/report-mrg1.md` file is tracked in git despite living under the
  excluded `.ds4/` path (prior docs commits record it), so main's version
  auto-merged into the merge commit and this report replaces it as a docs
  commit to leave the tree clean.
- I chose merge (matching the daemon's own fallback path) over rebase since
  no operation was in flight and rebase of 4 commits would re-conflict the
  same two hunks; the merge preserves both histories verbatim.

---

## Record 2 — main's side (68ca4d95 legend-rule ticket, via main commit `08955738`)

# Merge-conflict resolution report — mrg1

## Conflicted files

- `AGENTS.md` — the only unmerged path.

## Resolution

The initial `git status` showed a clean worktree and no active rebase/merge, so I completed integration with the main branch via `git merge main`. That merge conflicted in `AGENTS.md` at the adjacent Known approximations rows.

- The branch side contained the new main-side approximation for the `KReplacement` bot fallback. Main contained the obsolete CR 704.5j legend-rule row, which this reviewed branch closes by adding the controller choice.
- Kept main's `KReplacement` row and removed the superseded legend-rule row, preserving both current intents. No other conflict markers or unmerged paths remained.
- Completed the merge as `760578de` (`Merge branch 'main' into wt/cli-20260922T225140Z-68ca4d95`).

## Commands and output

- `git status --short --branch && git status`
  ```
  ## wt/cli-20260922T225140Z-68ca4d95
  On branch wt/cli-20260922T225140Z-68ca4d95
  nothing to commit, working tree clean
  ```
- `git merge main`
  ```
  Auto-merging AGENTS.md
  CONFLICT (content): Merge conflict in AGENTS.md
  Automatic merge failed; fix conflicts and then commit the result.
  ```
- `git add AGENTS.md && GIT_EDITOR=true git merge --continue`
  ```
  [wt/cli-20260922T225140Z-68ca4d95 760578de] Merge branch 'main' into wt/cli-20260922T225140Z-68ca4d95
  ```
- Corpus check: `.cards` was present.
- `go test -run 'TestLegendRule' ./rules/`
  ```
  ok  github.com/adams-shaun/gorge/rules  0.008s
  ```
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  ```
  ok  github.com/adams-shaun/gorge/rules  0.759s
  ```
- Final `git status --short --branch`
  ```
  ## wt/cli-20260922T225140Z-68ca4d95
  ```

No uncertainty remained in the conflict resolution. No non-conflict files were manually changed.

---

## Record 3 — round 2 integration (this seat, 2026-09-23)

### Starting state

`git status` found the tree CLEAN at `d3e21bfa`; no rebase or merge in
flight (the daemon's second integration attempt was fully aborted before
dispatch). The branch's merge-base with `main` was `c472a88f` — main had
moved past the branch's round-1 merge (`9fc45eaf`) with the 68ca4d95
legend-rule landing (`da0a323b` heads pin + `cf3e3784` merge). I redid the
integration as `git merge main`.

### Conflicted file: .ds4/report-mrg1.md (the only one)

- **branch side** (`d3e21bfa`): this ticket's round-1 resolution record.
- **main side** (`08955738`): the 68ca4d95 legend-rule ticket's own
  resolution record, committed to main.
- **Resolution:** keep BOTH records verbatim, headed, plus this record 3.
  `AGENTS.md` and `internal/testutil/agentsdoc_test.go` auto-merged this
  time: branch deleted the (fx20) row; main swapped the legend row for the
  `KReplacement` bot-fallback row (row count unchanged). Merged register =
  50 rows = the constant (verified below). `rules/heads_test.go` was
  auto-merged from main (2-seat head → `19a4893657e5d549`); the branch
  never touched it, so no head conflict arose and no golden was edited.

### Commands and output

- `.cards`: present (real symlink) — runs are not vacuous.
- `git merge main` → `CONFLICT (content): Merge conflict in .ds4/report-mrg1.md`
  (only unmerged path); resolved with both records kept, `git add -f`
  (`.ds4` is gitignored for untracked files), `git commit --no-edit` →
  `bdbdd7cb Merge branch 'main' into wt/cli-20260922T225140Z-6a16cd8c`.
- `git status --short` → clean.
- `grep -c 'fx20' AGENTS.md` → `0`; `grep -c 'KReplacement.*clamp fallback' AGENTS.md` → `1`.
- `go test ./internal/testutil/ -run 'TestKnownApproximations' -v` →
  `--- PASS: TestKnownApproximationsOnlyShrinks` / `ok`.
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  → `ok github.com/adams-shaun/gorge/rules 0.756s`; verbose log
  (`.ds4/scratch/ratchet.log`) shows all five RUN and PASS, **0 SKIPs**.
- Behaviour goldens: `go test ./internal/archtest/` → `ok 3.225s`;
  `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` →
  `ok 1.075s` — the pinned 20-game bot split did NOT move.

### Notes

- The branch registers no new `Mode$` matcher and closed no
  `knownUnsupported`/`knownUnsupportedParams`/`knownUnmodelledCountHeads`
  entry, so no ratchet table edit was needed; all pass untouched.
- Commits: `bdbdd7cb` (merge, conflict resolution) + this docs commit.
