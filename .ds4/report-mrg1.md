# Merge-conflict resolution report — mrg1 (branch wt/cli-20260922T225142Z-1d4558a1, main at 0104252d)

## Preserved main-side report record

The following report record was carried from main's integration of
`cli-20260922T225140Z-c661fa12`; it documents that separate integration.

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

## Main-side report record — task cli-20260922T225140Z-c661fa12

## Situation found

`git status` was CLEAN on entry (no rebase/merge in flight in this worktree;
the daemon's rebase attempt had been aborted), and the branch tip
`47b865e8` already merged main `e6a2a84d` (the PRIOR round's work, recorded in
the report this file carried). But main had advanced again to `e0fd7d9c`:

- `57cd1863`/`a04571b1`/`f76f59fd` — the battle-protector ticket (bot arm,
  re-derive, `botpolicy/protector_test.go`);
- `9aee8b00`/`78d3b764` — the NameCard ChooseFromList$/AtRandom$ closure;
- `d56e404f`/`e0fd7d9c` — the non<X> negation closure (nonColorless,
  nonChosenCard, nonCopiedSpell now expressed by the generic negation);
- plus its own main-merge commits (`23400548` etc).

So the owed integration was a merge of main `e0fd7d9c` into the branch.
Rebase is forbidden in a seat; the same `git merge main` method as prior
rounds was used.

## Conflicted files and resolutions

`git merge main` conflicted on exactly three files; all code auto-merged.

### 1. `AGENTS.md` (one hunk, lines 222-226)

- HEAD (branch) side: the `non<X>` negation row (still present on the branch,
  whose base merged main only up to `e6a2a84d` — before the non<X> closure).
- main side: the layer-4 row, which main still carries because the layer-4
  fix `63c07260` is THIS branch's reviewed commit, never merged to main.
- Both rows are CLOSED by commits on the two sides: layer-4 by this branch's
  fix, non<X> by main's `d56e404f`. **Resolution: delete BOTH rows** — which
  is exactly the union of the two sides' intents (each side deleted its own
  closed row; the merge should keep neither).

Verified against the measured tables: main's table is 45 rows (its
closures + its own added rows), HEAD's is 46; the merged table is main's 45
minus the layer-4 row = **44 rows** (`awk` over `## Known approximations` …
next `##`, `| ` lines minus header). Both deleted rows are absent from the
merged file; nothing else in the table changed relative to main
(`diff <(git show main:AGENTS.md | table-extract) <(table-extract AGENTS.md)`
shows exactly the one deleted row).

### 2. `internal/testutil/agentsdoc_test.go` (one hunk, comment only)

Both sides already agreed on the VALUE `knownApproximationRows = 46`; the
conflict was only in the history comment above it (main's side carried its
merge-round comment; the branch's side had a bare constant). **Resolution:**
kept main's commented shape, rewritten to describe THIS merge accurately,
and lowered the constant to the measured **44**:

    // The merged table measures 44 rows: main's closures merged here
    // (non<X> via d56e404f, NameCard ChooseFromList$/AtRandom$ via 78d3b764,
    // the battle protector row via f76f59fd, and the earlier pc1/each1/CR
    // 616.1 closures), plus this branch's own layer-4 filter-grammar closure
    // (fix 63c07260), which deletes main's last remaining layer-4 row.
    knownApproximationRows = 44

The test fails only on growth, but the constant should carry the measured
count (same method as main's precedent `4dcef2ea` and the prior round).

### 3. `.ds4/report-mrg1.md` (wholesale)

Both sides carry prior resolver rounds' reports (branch: the previous
integration round; main: another ticket's merge report). Neither is source
of truth. **Resolution: replaced wholesale with THIS round's report.**

## Auto-merged files — both sides' intent verified

- `effects/filter.go` — the branch's `SpecContext.DerivedTypes` layer-4
  plumbing (lines ~3193-3279) and main's non<X> generic-negation work
  (wordColorless/wordCopiedSpell, `nonCopiedSpell` comments at 1058/1377)
  compose; `go build ./effects/ ./rules/ ./internal/testutil/` clean.
- `rules/engine.go`, `rules/cast.go`, `rules/replacement.go` — main's
  battle-protector and NameCard work; no branch-side contradiction (the
  branch's fix touched layers/statics/clone/filter only).
- `botpolicy/policy.go` + `botpolicy/protector_test.go` — main's new files,
  added cleanly.
- `AGENTS.md`'s other hunks — main's row deletions (NameCard row,
  battle-protector row) applied by auto-merge; nothing reintroduced.

## Commands run and output

- `.cards` check: **present** (real symlink → `/home/sadams/projects/gorge/.cards`),
  so the corpus-backed ratchets were real, not vacuous skips.
- `git merge main` → 3 content conflicts (`AGENTS.md`,
  `internal/testutil/agentsdoc_test.go`, `.ds4/report-mrg1.md`), 17 files
  total in the merge.
- `go test ./internal/testutil/ -run 'TestKnownApproximations' -v`
  → `--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)` / `ok 0.001s`.
- Post-merge ratchets:
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|TestLayer4'`
  → `ok github.com/adams-shaun/gorge/rules 1.025s`.
- main's closures on the merged tree:
  `go test ./effects -run 'TestPlayerSpecFx20Grammar|TestUnknownPredicates|NameCard|NonPredicateStack'`
  → `ok github.com/adams-shaun/gorge/effects 0.008s`.
- main's bot-policy addition: `go test ./botpolicy ./internal/archtest/`
  → `ok ... botpolicy 0.719s` / `ok ... internal/archtest 3.846s`.
- `gofmt -l internal/testutil/agentsdoc_test.go` → clean.
- `go build ./effects/ ./rules/ ./internal/testutil/` → clean.

## Uncertainties / notes

- The dual-row deletion in `AGENTS.md` is the one judgment call: each side
  deleted a DIFFERENT row, so the merged table keeps neither. This is the
  union of the two reviewed intents, not a redesign.
- No engine behaviour beyond the branch's reviewed fix `63c07260` was
  introduced by the resolution itself; the resolution touched `AGENTS.md`,
  the register constant + its comment, and this report.
- `git diff --stat main` post-merge shows only the branch's own fix files
  plus the constant/report, confirming main's content came through intact.

## Issues

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


## Current dispatch confirmation (2026-09-23)

The dispatch additionally reported an earlier rebase conflict in `AGENTS.md` and `internal/testutil/agentsdoc_test.go`, followed by a merge-fallback conflict in `.ds4/report-mrg1.md` and `AGENTS.md`. Those attempts were not active when this resolver seat began: the branch already contained the completed merge `e3aa3074` and report commit `f3148515`. The actual merge resolution and conflicts are documented above; `AGENTS.md` auto-merged both sides' row deletions, and the only unmerged path in the completed merge was `internal/testutil/agentsdoc_test.go`. No unmerged paths remain.

Reconfirmed in this seat:

- `.cards` exists; corpus-dependent tests are not vacuous.
- Before this report update, `git status --short --branch` showed only `## wt/cli-20260922T225142Z-1d4558a1`.
- `git diff --check HEAD^ HEAD` produced no output.
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead' 2>&1 | tail -30` returned `ok github.com/adams-shaun/gorge/rules 0.756s`.

No code or conflict-resolution changes were needed in this confirmation seat.

The main-side report record's resolution noted: None found during that round.

---

## Current integration round — main at 2040e5d9

### Conflicts and resolution

The earlier merge commit `e3aa3074` integrated main at `0104252d`, but main
advanced to `2040e5d9` before this dispatch completed. I merged the current
`main` (rather than leaving the branch behind it). The merge auto-merged
engine and bot-policy changes, and conflicted in three tracked files:

- `AGENTS.md`: branch side had deleted the `(blockprop1)` row as part of the
  approved fix; main still carried that stale row. Kept the branch deletion.
  The adjacent `(each1)` row was preserved. This keeps the reviewed closure
  and main's other distinct row closures.
- `internal/testutil/agentsdoc_test.go`: reconciled the table ratchet to 44,
  confirmed by `TestKnownApproximationsOnlyShrinks` against the merged
  `AGENTS.md` (the initially attempted 43 failed and was corrected to the
  measured 44).
- `.ds4/report-mrg1.md`: preserved the branch's mrg1 history and the
  main-side `cli-20260922T225140Z-c661fa12` report as separate records, then
  appended this integration record. No report content was discarded.

### Commands and output

- `git status` before integrating current main: clean at `a8f650ed`.
- `git merge main --no-edit`:
  ```
  Auto-merging .ds4/report-mrg1.md
  CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
  Auto-merging AGENTS.md
  CONFLICT (content): Merge conflict in AGENTS.md
  Auto-merging botpolicy/policy.go
  Auto-merging internal/testutil/agentsdoc_test.go
  CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
  Auto-merging rules/statics.go
  Automatic merge failed; fix conflicts and then commit the result.
  ```
- Conflict-marker scan and `git diff --check`: no output after resolution.
- `.cards` check: present.
- `go test ./internal/testutil/ -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'`:
  `ok github.com/adams-shaun/gorge/internal/testutil 0.006s`.
- `go test ./rules/ -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`:
  `ok github.com/adams-shaun/gorge/rules 0.771s`.

No code behaviour was changed during resolution. The only uncertainty was the
ratchet count, resolved by the targeted test's direct measurement (44).
