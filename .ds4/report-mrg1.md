# Merge conflict resolution report — cli-20260922T225141Z-1b1182e4

## Operation and conflict

- Initial `git status --short --branch`: `## wt/cli-20260922T225141Z-1b1182e4`; working tree clean, no operation in progress.
- Ran `git merge main`. It completed automatically except for one content conflict: `internal/testutil/agentsdoc_test.go`. `AGENTS.md` and the other main changes merged automatically; `rules/trigmatch_misc.go` also auto-merged.
- The conflict was `knownApproximationRows`: this branch said `53`, while main said `50`. The branch closes the First Strike Damage approximation row; main contains other approximation closures. After the merge, I measured 46 data rows in the merged `AGENTS.md` Known approximations table (one header row excluded), so resolved the constant to `46` to match the merged content. No other files were conflicted.
- Completed the merge with its default message. Merge commit: `af2f1648` (`Merge branch 'main' into wt/cli-20260922T225141Z-1b1182e4`).

## Commands and outputs

`git status --short --branch && git rev-parse --show-toplevel && git branch --show-current && git status`

```text
## wt/cli-20260922T225141Z-1b1182e4
/home/sadams/projects/gorge/.worktrees/cli-20260922T225141Z-1b1182e4
wt/cli-20260922T225141Z-1b1182e4
On branch wt/cli-20260922T225141Z-1b1182e4
nothing to commit, working tree clean
```

`git merge main`

```text
Auto-merging AGENTS.md
Auto-merging internal/testutil/agentsdoc_test.go
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
Automatic merge failed; fix conflicts and then commit the result.
```

Measured table counts from `git show <ref>:AGENTS.md`: branch `HEAD` had 53 data rows; main had 47. The automatically merged table had 46 data rows after preserving both sides' deletions.

`git add internal/testutil/agentsdoc_test.go && GIT_EDITOR=true git merge --continue && git status --short --branch`

```text
[wt/cli-20260922T225141Z-1b1182e4 af2f1648] Merge branch 'main' into wt/cli-20260922T225141Z-1b1182e4
## wt/cli-20260922T225141Z-1b1182e4
```

Corpus check: `ls .cards | head`

```text
cards.lock
cardsfolder
ir.gob.gz
ir.v4.gob.gz
tokenscripts
```

Conflicted-package check, `go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'`

```text
ok  github.com/adams-shaun/gorge/internal/testutil  0.001s
```

Post-merge ratchets, `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`

```text
ok  github.com/adams-shaun/gorge/rules  0.792s
```

Final `git status --short --branch`:

```text
## wt/cli-20260922T225141Z-1b1182e4
```

No unresolved concerns.

---

## Main lineage records from main (kept verbatim; merged at main @ e0fd7d9c)

## Main lineage record — 6a16cd8c round-2 docs (kept verbatim from main)

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

## Branch-side tail kept from the conflict (b686e452 lineage; the bullets
below it are its continuation)

Git auto-merged: main deleted the `SetName` row (1 region), the branch
deleted its 3 rows (different regions). Verified on the resolved file:

- `KReplacement` order row: absent (0 hits)
- `multikicker1` row: absent (0 hits)
- `mutate1` row: absent (0 hits)
- `SetName` approximation row: absent (0 hits)
- counted rows: 52

## Commands run (real output)

```
$ git rebase main
Rebasing (1/4)Auto-merging AGENTS.md
Auto-merging internal/testutil/agentsdoc_test.go
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
error: could not apply 6a7f336b... fix(botpolicy): add real arms ...
$ git add AGENTS.md internal/testutil/agentsdoc_test.go && GIT_EDITOR=true git rebase --continue
[detached HEAD cf81eb11] fix(botpolicy): add real arms for replacement order, multikick count and mutate placement
Rebasing (2/4)Rebasing (3/4)Rebasing (4/4)Successfully rebased and updated refs/heads/wt/cli-20260922T225140Z-8b855197.
```

Commits 2–4 replayed with no further conflict.

```
$ git status
On branch wt/cli-20260922T225140Z-8b855197
nothing to commit, working tree clean

$ git log --oneline main..HEAD
f7d099d9 test(rules): remove obsolete multikicker bot decline assertion
e8c27906 docs: record resolved approximation arms rebase
a13459fe test(botpolicy): cover approximation arms with corpus cards
cf81eb11 fix(botpolicy): add real arms for replacement order, multikick count and mutate placement
```

Row-count check on the resolved AGENTS.md (same logic as the test):

```
$ awk '/^## Known approximations/{inside=1;next} inside&&/^## /{inside=0} inside&&/^\| /{c++} END{print "rows:", c-1}' AGENTS.md
rows: 52
```

Targeted test on the conflicted file's package:

```
$ go test -v -run 'TestKnownApproximation' ./internal/testutil/
=== RUN   TestKnownApproximationsOnlyShrinks
--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)
=== RUN   TestKnownApproximationRowsAreShort
--- PASS: TestKnownApproximationRowsAreShort (0.00s)
PASS
ok  	github.com/adams-shaun/gorge/internal/testutil	0.001s
```
(No "table is down to N" log -> the row count EQUALS the constant 52 exactly.)

Ratchets main newly carries (the brief's command):

```
$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok  	github.com/adams-shaun/gorge/rules	1.033s
```

Branch's own tests (changed packages):

```
$ go test -run 'TestBotReplacementOrderRanksBySourceWorth|TestBotReplacementOrderBypassesSkip|TestBotMultikickPaysTheAffordableMaximum|TestBotMutatePlacesUnder' ./botpolicy/
ok  	github.com/adams-shaun/gorge/botpolicy	0.004s

$ go test -run 'TestBotPolicyCorpusReplacementOrder|TestBotPolicyCorpusMultikickerPaysMaximum|TestBotPolicyCorpusMutatePlacesUnder' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.634s

$ go test -run 'TestMultikickBotDeclinesThroughTheFirstOfferArm' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.002s [no tests to run]
```
(The obsolete decline assertion was removed by the branch's own commit
`f7d099d9` — that test asserted the pre-fix behaviour the branch replaced;
its removal is the reviewed fix, and the remaining multikicker tests pass:)

```
$ go test -run 'TestMultikick' ./rules/
ok  	github.com/adams-shaun/gorge/rules
```

`gofmt -l` on the six touched Go files: no output (clean).

`.cards` symlink present (points at `/home/sadams/projects/gorge/.cards`) —
the corpus-dependent runs above were real, not vacuous skips.

## Final state

- `git status`: on branch `wt/cli-20260922T225140Z-8b855197`, **working tree
  clean**; no active rebase/merge.
- `main` (`4357caf3`) is an ancestor of HEAD.
- Rebased commits: `cf81eb11` (fix), `a13459fe` (test), `e8c27906` (docs),
  `f7d099d9` (test cleanup).
- `git diff --stat main..HEAD` = the branch's intended changes plus the
  previously committed `.ds4/report-mrg1.md` docs commit (prior seat's).

## Uncertainties / notes

- Main's constant (57) is loose against main's own table (55); I resolved to
  the *counted* 52 rather than delta arithmetic (57 − 3 = 54, or 53 − 1 = 53).
  The count is what the ratchet compares against; 52 is the exact value that
  makes the constant truthful and keeps every deleted row's intent.
- No engine behaviour was changed by the resolution: only the constant, in
  the direction both sides intended. No goldens or ratchet tables were edited.
- The `.ds4/report-mrg1.md` rewrite you are reading is committed on the branch
  (the file is tracked there since `e8c27906`), keeping the tree clean.

## Issues

None found during resolution. The only conflict was the register constant.

---

# Round 3 — rebase onto main @ 9016c22 (fresh relaunch, cli-20260922T225140Z-8b855197)

## Situation found

`git status` on entry: clean tree at `70fd74ad`, no in-flight operation. The
daemon's rebase attempt (`rebase (start): checkout main` at `9016c22` in the
reflog) had already been aborted. `main` had advanced past round 2's base
(`4357caf3`) by the bot-target closure series:

- `1f10a319`/`f2070a6a`/`c73c4314`/`317a1392` botpolicy target ranking fixes
- `a70c4483` merge, `ebe94f76` heads pin, `9016c22` merge

Ran `git rebase main` again (the operation the daemon itself uses).

## Conflicted files this round

### `internal/testutil/agentsdoc_test.go` (same single line as rounds 1–2)

- **Main side**: `knownApproximationRows = 54` (main's own table after its
  `(ft1)` row deletion).
- **Branch side**: `knownApproximationRows = 52` (round 2's exact-count
  resolution: branch's 3 row deletions + main's SetName deletion).

Both sides' deletions are disjoint and coexist; the merged AGENTS.md
measured at **51 rows** (verified: `(ft1)`, `(setname1)`, KReplacement-order,
multikicker1, mutate1 all absent — `/usr/bin/grep -c` = 0 hits).

**Resolution: `knownApproximationRows = 51`** — the measured count of the
merged table, confirmed by the ratchet itself:

```
$ go test -v -run 'TestKnownApproximations' ./internal/testutil/   # probe at 52
    agentsdoc_test.go:89: table is down to 51 rows (constant says 52) -- lower
    knownApproximationRows to 51 in the same commit that deleted them.
$ go test -v -run 'TestKnownApproximations' ./internal/testutil/   # after setting 51
--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)
```

(no looseness log — the constant equals the counted rows exactly).

### `.ds4/report-mrg1.md` (docs only)

Main's version is task `cli-20260922T225140Z-677ee477`'s merge report (its
resolver overwrote this shared report path); the branch's version is THIS
task's report. Took the **branch side** (`git checkout --theirs`) — the file
is this task's report channel, and both sides are docs-only. This section
documents the round-3 resolution.

`AGENTS.md` and `botpolicy/policy.go` auto-merged cleanly both rounds.

## Commands and outputs

```
$ git rebase main
Rebasing (1/5)Auto-merging AGENTS.md
Auto-merging botpolicy/policy.go
Auto-merging internal/testutil/agentsdoc_test.go
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
error: could not apply cf81eb11... fix(botpolicy): add real arms ...
# resolved constant to 51 (measured), git add, GIT_EDITOR=true git rebase --continue
Rebasing (2/5)Rebasing (3/5)Auto-merging .ds4/report-mrg1.md
CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
error: could not apply e8c27906... docs: record resolved approximation arms rebase
# took branch side (this task's report), git add -f, GIT_EDITOR=true git rebase --continue
Rebasing (4/5)Rebasing (5/5)Successfully rebased and updated refs/heads/wt/cli-20260922T225140Z-8b855197.
```

```
$ git status --short        # clean
$ git log --oneline main..HEAD
7ec7db11 docs: record merge-conflict resolution rebase onto main
bfd3a520 test(rules): remove obsolete multikicker bot decline assertion
00a7a776 docs: record resolved approximation arms rebase
04e9f073 test(botpolicy): cover approximation arms with corpus cards
87d13658 fix(botpolicy): add real arms for replacement order, multikick count and mutate placement
```

Ratchets main newly carries (the round-2 brief's command):

```
$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok  	github.com/adams-shaun/gorge/rules	0.761s
```

Branch's own approximation-arm tests (both sides edited botpolicy; the merge
is behaviour-neutral for them — these prove the arms still pass on the merged
tree):

```
$ go test -run 'TestBotReplacementOrderRanksBySourceWorth|TestBotReplacementOrderBypassesSkip|TestBotMultikickPaysTheAffordableMaximum|TestBotMutatePlacesUnder|TestBotPolicyCorpus' ./botpolicy/ ./rules/
ok  	github.com/adams-shaun/gorge/botpolicy	0.005s
ok  	github.com/adams-shaun/gorge/rules	0.663s
```

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

Final check from main's side of the report conflict: `git status --short --branch` returned only `## wt/cli-20260922T225140Z-677ee477` (clean), HEAD `a70c4483`; its own merged approximation table measured 54 data rows.

---

## Preserved report from main (task 677ee477's own merge round)

Behaviour golden (main also changed `botpolicy/target.go`):

```
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	0.979s
```

`gofmt -l` on the touched Go file: no output. `.cards` symlink present
(→ `/home/sadams/projects/gorge/.cards`), so the corpus-backed runs above
were real, not vacuous skips.

## Uncertainties / notes

- Same method as round 2: resolved the register constant to the *measured*
  count of the merged table (51), not either side's stale value. Main's 54
  and the branch's 52 each described a table that no longer exists after the
  other side's deletions land.
- `.ds4/report-mrg1.md` keeps this task's report; main's copy belongs to task
  677ee477 and is preserved in that task's own history on main.
- No engine behaviour was changed by the resolution: one constant, docs, and
  the rebase replay of already-reviewed commits.

## Issues

None found during this round's resolution.

---

## Current merge resolution — main `c472a88f`

The tree was CLEAN at `1701eb73` (branch HEAD); the daemon's rebase onto main
and its merge fallback had both been rolled back, so no operation was in
flight. Main had advanced past `9016c22c` (the branch's previous merge base)
with the replacement-order/multikick/mutate bot-policy arm closures
(`87d13658`), the `ft1` register-constant lowering (`4dcef2ea`) and a further
merge (`c472a88f`). Rebase is forbidden in this seat (shared-git rule), so the
integration was done as `git merge main`.

### Conflicts and resolution

- `internal/testutil/agentsdoc_test.go`: both sides lowered the constant from a
  different base. Branch recorded 53 (its `non<X>` closure on top of
  `9016c22c`); main recorded 51 (its three bot-arm closures). The MEASURED
  count of the auto-merged `AGENTS.md` table is **50** — branch's 53 minus the
  three rows main deleted (KReplacement clamp fallback, `(multikicker1)`,
  `(mutate1)`) — so the constant is set to the measurement, 50, with the
  branch's `non<X>` row deletion preserved.
- `AGENTS.md`: auto-merged cleanly. Verified the merged table equals main's
  table minus the branch's `non<X>` row, and nothing else; 50 data rows.
- `.ds4/report-mrg1.md`: shared accumulated report. Kept the branch's Sections
  1–8 history verbatim and preserved main's round as a labelled section; no
  history discarded. This section appended.

### Commands and output

- `git status` before merge: clean on `wt/cli-20260922T225140Z-b686e452`.
- `git merge main`:
  ```
  Auto-merging .ds4/report-mrg1.md
  CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
  Auto-merging AGENTS.md
  Auto-merging internal/testutil/agentsdoc_test.go
  CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
  Automatic merge failed; fix conflicts and then commit the result.
  ```
- Merged approximation row count: `50` (awk over the merged `AGENTS.md` table;
  matches `knownApproximationRows = 50`).
- `go test ./internal/testutil -run 'TestKnownApproximation' -v`:
  ```
  === RUN   TestKnownApproximationsOnlyShrinks
  --- PASS: TestKnownApproximationsOnlyShrinks (0.00s)
  === RUN   TestKnownApproximationRowsAreShort
  --- PASS: TestKnownApproximationRowsAreShort (0.00s)
  PASS
  ok  	github.com/adams-shaun/gorge/internal/testutil
  ```
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`:
  ```
  ok  	github.com/adams-shaun/gorge/rules
  ```
- `go test -run 'TestNonPredicate|TestNonCopiedSpell|TestNonColorless|TestNonChosenCard' ./effects/`:
  ```
  ok  	github.com/adams-shaun/gorge/effects
  ```
- `gofmt -l internal/testutil/agentsdoc_test.go`, `git diff --check`, and a
  conflict-marker grep over the resolved report and Go file: no output.

No unresolved conflict or uncertainty remains.

## Issues

None found during this round's resolution.

Final state: `git commit --no-edit` created merge commit `0d70e635` (parents:
pre-merge branch `1701eb73`, main `c472a88f`). `git status --short --branch`
returned only `## wt/cli-20260922T225140Z-b686e452`; `git merge-base
--is-ancestor main HEAD` succeeded. Behaviour goldens `internal/archtest` and
`cmd/botbench`'s `TestConstructedDefaultIsByteIdentical` both pass; the
approximation register and every merge-ratchet test pass with the merged table
at 50 rows.

---

## Main-side tail kept from the conflict

No uncertainty remained in the conflict resolution. No non-conflict files were manually changed.

---

## Section — this merge (main cf3e3784 into wt/cli-20260922T225140Z-b686e452)

### Starting state

`git status` was CLEAN on `wt/cli-20260922T225140Z-b686e452` at `411fd2b0` (the
branch's previous merge of main); the dispatch's rebase and its merge fallback
had both been rolled back, so no operation was in flight. Main had advanced
8 commits to `cf3e3784` (the CR 704.5j legend-choice closure `74870371` + its
heads re-pin `da0a323b`, and the merge resolution `cf3e3784`). Rebase is
forbidden in this seat, so the integration was done as `git merge main`.

### Conflicted files and how each side's intent was kept

1. **`AGENTS.md`** — one conflict hunk inside the Known approximations table.
   Measured with the test's own row-extraction logic: base `c472a88f` = 51 data
   rows, HEAD = 50, main = 51. Per-row (grep counts on `git show <ref>:AGENTS.md`):
   - **legend row** (`The CR 704.5j legend rule keeps the first duplicate…`):
     base 1, HEAD 1, main 0 — main's legend ticket CLOSED it (the rule now asks
     the duplicate set's controller; remainder recorded in `74870371`'s commit
     message). Kept main's deletion.
   - **`non<X>` row**: base 1, HEAD 0, main 1 — this branch's reviewed fix
     `d56e404f` CLOSED it (all three named shapes were already handled or are
     now handled and test-pinned). Kept the branch's deletion; main's copy was
     stale.
   - **`KReplacement` bot-fallback row** (present only on main's side of the
     hunk): NOT resurrected. It says "there is no policy arm of its own", but
     `87d13658` — an ancestor of BOTH sides — deleted this exact row when it
     added the real `chooseReplacementOrder` arm, which is byte-identical in
     `botpolicy/policy.go` on both trees (verified: `git diff HEAD main --
     botpolicy/policy.go` is empty). The row was re-carried by main's own merge
     resolution `cf3e3784` in error; keeping it would leave a false row in the
     frozen register (a re-add is GROWTH).
   Resolution: the hunk collapses to nothing — merged table measures **49**
   data rows (51 − branch's `non<X>` − main's legend), a strict shrink.
2. **`internal/testutil/agentsdoc_test.go`** — auto-merged to 50; lowered to
   **49** with a comment naming the two real closures and the stale row.
3. **`.ds4/report-mrg1.md`** — HEAD's accumulated report vs main's one-line
   tail. BOTH kept (main's tail under its own heading), this section appended.

No engine code conflicted: main's `rules/sba.go` legend changes,
`rules/heads_test.go` re-pin and the rest auto-merged.

### Commands and output

- `.cards` present (real corpus symlink) — runs are real, not vacuous skips.
- `git merge main --no-edit` → conflicts in `.ds4/report-mrg1.md` and
  `AGENTS.md`; `internal/testutil/agentsdoc_test.go` auto-merged.
- Merged-table measurement (python replication of `approximationRows`): 49.
- `go test ./internal/testutil -run 'TestKnownApproximation' -v` →
  `--- PASS: TestKnownApproximationsOnlyShrinks`, `--- PASS:
  TestKnownApproximationRowsAreShort`, `ok ... 0.001s` (exact match, no slack).
- Ratchets + goldens:
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|TestHeads$'`
  → `ok github.com/adams-shaun/gorge/rules 1.845s` (heads golden passes with
  main's legend re-pin merged in — no further head moved).
- Branch-fix sanity: `go test ./effects -run 'NonPredicate|NonCopied|NonColorless|NonChosen'`
  → `ok ... 0.641s`.
- Main-side sanity: `go test ./rules -run 'Legend'` → `ok ... 0.694s`.
- Behaviour goldens: `go test ./internal/archtest/` → `ok ... 3.506s`;
  `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` →
  `ok ... 0.991s` (split did not move).
- `gofmt -l internal/testutil/agentsdoc_test.go` → clean; no conflict markers
  remain in any tracked file.

### Issues

None found beyond the resolved conflict itself. One note recorded above: main's
`cf3e3784` merge resolution resurrected the stale `KReplacement` bot-fallback
row; the merged tree drops it rather than carrying a false row forward.

---

## Main lineage record — 6a16cd8c round 2 (kept verbatim from main)

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

---

## Record — round 3 integration (this seat, b686e452 lineage, 2026-09-23)

### Starting state

`git status` on entry: tree CLEAN at `330bac6d` (the branch's round-2 merge of
main `cf3e3784`, completed by the previous seat); no rebase or merge in
flight — but `main` had advanced again 2 minutes after that merge, to
`c3256fc3` (the 6a16cd8c player-spec closure landing: `3cbf3e11`,
`c85c39b7`, `c3662d98`, `ade1f41d` + their merges), so the round-2
integration was already stale. The previous seat also never wrote its report
(`report_written: false` in the harness cutoff). I re-integrated with
`git merge main` (rebase forbidden in this seat).

### Conflicts and resolution

1. **`internal/testutil/agentsdoc_test.go`** — only the `knownApproximationRows`
   constant + comment: HEAD 49 (its two row drops), main 50 (its fx20 drop).
   The auto-merged `AGENTS.md` table measures **48** data rows (base
   `cf3e3784` = 51, minus the branch's `non<X>` drop (its fix `d56e404f`
   implements `nonCopiedSpell` — verified in `effects/filter.go:1339`) and its
   stale `KReplacement`-fallback drop (the real arm is `87d13658`'s
   `chooseReplacementOrder`, `botpolicy/policy.go:479`), minus main's (fx20)
   player-spec drop). Resolved to `knownApproximationRows = 48` with a comment
   naming all four closures.
2. **`.ds4/report-mrg1.md`** — both lineages' record accumulations (HEAD's
   b686e452 rounds vs main's 6a16cd8c/68ca4d95 records). BOTH kept verbatim,
   each under a lineage heading, plus this record appended.

`AGENTS.md` and all engine code auto-merged (main's fx20 closure touches
`events/apply.go`, `rules/*`, `state/*`, `effects/attach.go`,
`effects/filter.go` — none of them conflicted with this branch's diff).

### Commands and output

- `.cards` present (real symlink to `/home/sadams/projects/gorge/.cards`) —
  runs below are real, not vacuous skips.
- Merged-table count (the test's own awk logic): `rows: 48`; `grep -c fx20
  AGENTS.md` → 0; `KReplacement.*clamp fallback` → 0; `non<X>`-row → 0.
- `go test ./internal/testutil/ -run 'TestKnownApproximation' -v` →
  `--- PASS: TestKnownApproximationsOnlyShrinks`,
  `--- PASS: TestKnownApproximationRowsAreShort`, `ok ... 0.001s`.
- Ratchets + heads golden:
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|TestHeads$'`
  → `ok github.com/adams-shaun/gorge/rules 2.129s`.
- Main's new fx20 closure tests (merged code, both packages):
  `go test ./rules -run 'PlayerSpec|PlayerAttachment|Descend|Defending|Enchanted|Counters_'`
  → `ok ... 0.718s`;
  `go test ./effects -run 'Fx20|PlayerSpec|Enchanted|Counters_|Descend'`
  → `ok ... 0.628s`.
- Branch's own non<X> suites still pass with main's filter.go changes merged:
  `go test ./effects -run 'NonPredicate|NonCopied|NonColorless|NonChosen'`
  → `ok ... 0.636s`.
- Behaviour goldens: `go test ./internal/archtest/` → `ok ... 3.418s`;
  `go test -run 'TestConstructedDefaultIsByteIdentical' ./cmd/botbench/`
  → `ok ... 1.012s` (pinned 20-game bot split did NOT move).
- `git status` after commit: working tree clean; `main` (`c3256fc3`) is an
  ancestor of HEAD.

### Notes / uncertainties

- The branch registers no new `Mode$` matcher and closed no
  `knownUnsupported`/`knownUnsupportedParams`/`knownUnmodelledCountHeads`
  entry, and main's closure neither: the ratchet run above is green with the
  tables untouched.
- Both lineages' reports are preserved in this file under explicit headings;
  some ordering reads oddly because the sides had diverged at the same
  anchor lines, but no record was dropped.

## Issues

None found during this integration round beyond the resolved conflicts.


---

## Main lineage record — 62421b89 integration (kept verbatim from main)

# Merge-conflict resolution — cli-20260922T225140Z-62421b89

## Situation found

The worktree was clean on entry at `7a45ef9a` with no in-flight merge or rebase. `main` was at `0104252d2aac00c38b8ee63675239b78dddef2ff`, not an ancestor of the branch. `.cards` was present.

Ran `git merge main`. It auto-merged the main changes and reported one conflicted file: `AGENTS.md`.

## Conflicted file and resolution

### `AGENTS.md`

The branch side had deleted its now-closed approximation row for CR 616.1 replacement ordering; it also carried the CR 704.5j legend-rule approximation. Main's conflicting side contained the old CR 616.1 row and a botpolicy fallback row, as well as the legend-rule row. The branch has implemented both the CR 616.1 order choice and `botpolicy`'s `KReplacement` policy (`botpolicy/policy.go`, `chooseReplacementOrder`), so the two main-side CR 616.1 rows describe behavior no longer present and were not retained. Kept the branch's removal and retained main's still-applicable CR 704.5j legend-rule row. Other main edits to this file, including removal of the closed fx20/pc1 rows, were preserved by the auto-merge.

No other file had merge markers. The remaining main changes auto-merged. The required main ratchet run found two stale `apiSpecificRulesSA` entries in `rules/paramcensus_test.go`: `Engine.applyAddCounterBody` and `Engine.applyAddCounterReplacements` consume the priced result and no longer read SA params. Removed those stale classifications, retaining `Engine.counterReplaceOp`, which does read the parameters. This is the ratchet-table correction required for the merged branch; no engine behavior was changed by it.

## Commands and output

```text
$ git status --short --branch && git rev-parse --abbrev-ref HEAD && git diff --name-only --diff-filter=U
## wt/cli-20260922T225140Z-62421b89
wt/cli-20260922T225140Z-62421b89

$ git rev-parse main
0104252d2aac00c38b8ee63675239b78dddef2ff

$ git merge main
Auto-merging AGENTS.md
CONFLICT (content): Merge conflict in AGENTS.md
Auto-merging rules/engine.go
Auto-merging rules/turn.go
Automatic merge failed; fix conflicts and then commit the result.

$ git add AGENTS.md && go test -run 'TestKnownApproximation' ./internal/testutil/
ok   github.com/adams-shaun/gorge/internal/testutil  0.001s

$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
--- FAIL: TestEveryRepoDeckParamsAreRead (0.15s)
    paramcensus_test.go:2738: paramcensus rot guard: 2 findings:
        paramcensus: apiSpecificRulesSA entry "Engine.applyAddCounterBody" no longer reads SA params -- delete the stale entry
        paramcensus: apiSpecificRulesSA entry "Engine.applyAddCounterReplacements" no longer reads SA params -- delete the stale entry
FAIL
github.com/adams-shaun/gorge/rules  0.892s
FAIL

# Removed the two stale entries from rules/paramcensus_test.go.
$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok   github.com/adams-shaun/gorge/rules  0.750s
```

`git diff --check` reported no whitespace errors after conflict resolution. The first focused approximation test passed. The first ratchet run exposed the two stale entries above; the rerun passed after removing them. `.cards` existed, so the corpus-dependent ratchets were not vacuous skips.

## Uncertainties / issues

No unresolved conflict or uncertain resolution remains. The replacement-order approximation rows from main were intentionally omitted because the reviewed branch fix and the already-present botpolicy arm close those claims. The unrelated CR 704.5j row remains intact. No engine behavior change or golden edit was made during conflict resolution.

---

## Record — round 4 integration (this seat, b686e452 lineage, 2026-09-23)

### Starting state

`git status` on entry: tree CLEAN at `d396b8ba` (the branch's round-3 merge of
main `c3256fc3`, completed by the previous seat, gates green); no rebase or
merge in flight. But `main` had advanced again past `c3256fc3` to `e6a2a84d`
(three more tickets' merges: `0104252d` = ab191b03 pc1 context-bound object
predicates + `0acadaa3` imprint-filter expiry, `0c7e5ce9` = c385caf3 EachDamage
fail-closed outcomes, `e6a2a84d` = 62421b89 CR 616.1 competing-replacement
order choice), so the round-3 integration was stale again. This is why the
daemon re-dispatched mrg1. I re-integrated with `git merge main` (rebase
forbidden in this seat).

### Conflicts and resolution

1. **`AGENTS.md`** — one conflict hunk in the Known approximations table.
   HEAD side carried the all-`Updated`-competition row and the (pc1) row
   (both since CLOSED on main by `e6a2a84d`/`179a3de1`); main side carried the
   re-booked CR 704.5j legend row (deliberately re-added by the 62421b89
   lineage — main's later deliberate change, kept) and the `non<X>` row
   (closed on THIS branch by the reviewed fix `d56e404f`; main's copy is
   stale for the merged code). Resolution: keep only the legend row from
   main's side; drop both HEAD-side rows and main's stale `non<X>` row.
   The each1 row's deletion auto-merged (c385caf3). Merged table measures
   **46** rows (48 − all-`Updated` − pc1 − each1 + legend); constant lowered
   48 → 46 with a comment naming every closure.
2. **`internal/testutil/agentsdoc_test.go`** — auto-merged to HEAD's value
   (48); updated to 46 as above.
3. **`.ds4/report-mrg1.md`** — this file; HEAD's accumulation and main's
   62421b89 record BOTH kept verbatim under lineage headings; this record
   appended.

`effects/filter.go` auto-merged cleanly (main's pc1 predicate work vs this
branch's `nonCopiedSpell` fix touch different regions). No golden, ratchet
table or `heads_test.go` was edited.

### Commands and output (measured)

- `.cards` present (real symlink to `/home/sadams/projects/gorge/.cards`) —
  runs are real, not vacuous skips.
- Merged-table count (the test's own awk logic): `rows: 46`.
- `go test ./internal/testutil/ -run 'TestKnownApproximation' -v` →
  `--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)`,
  `--- PASS: TestKnownApproximationRowsAreShort (0.00s)`,
  `ok github.com/adams-shaun/gorge/internal/testutil 0.001s`.
- Ratchets: `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  → `ok github.com/adams-shaun/gorge/rules 0.755s`; spot-verbose rerun of
  `TestEveryRepoDeckParamsAreRead` shows `--- PASS (0.71s)`, no SKIP.
- Branch's non<X> suites: `go test ./effects -run 'NonPredicate|NonCopied|NonColorless|NonChosen'`
  → `ok ... 0.619s`.
- Main's new closures' suites: `go test ./rules -run 'EachDamage|ReplacementOrder|Legend|PlayerSpec|Imprinted|ContextWord|BangPredicate|ReplacementUpdated|TokenReplaceChosen|FailClosed'`
  → `ok ... 1.182s`; `go test ./effects -run 'EachDamage|FailClosed|Imprinted|ContextWord|BangPredicate|Condition'`
  → `ok ... 1.266s`.
- Behaviour goldens: `go test ./internal/archtest/` → `ok ... 3.937s`;
  `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` →
  `ok ... 0.972s` (pinned 20-game bot split did NOT move).
- `gofmt -l internal/testutil/agentsdoc_test.go` → clean.
- `git status` after the merge commit: clean; `main` (`e6a2a84d`) is now an
  ancestor of HEAD (merge commit `c53b5e1f`, parents `d396b8ba` + `e6a2a84d`).

### Notes / uncertainties

- The branch registers no new `Mode$` matcher and closed no
  `knownUnsupported`/`knownUnsupportedParams`/`knownUnmodelledCountHeads`
  entry; main's closures came with their own table edits, auto-merged.
- One residual recorded OUTSIDE the table (register is delete-only, a row
  may never be re-added narrowed): main's deleted `non<X>` row named
  `nonColorless` and `nonChosenCard` as still-unmatched shapes. They remain
  unmatched in the merged tree (only `nonCopiedSpell` was closed, by
  `d56e404f`). Someone re-book them if a ticket takes them.

## Issues

None found during this integration round beyond the resolved conflicts.

---

# Round 2 — merge main @ e0fd7d9c into wt/cli-20260922T225141Z-1b1182e4

## Starting state

`git status` found the tree CLEAN — no rebase or merge in flight (the daemon had
aborted its failed attempts before dispatching this seat). The branch's round-1
merge (af2f1648) had integrated main @ e6a2a84d; main had since advanced to
e0fd7d9c (battle-protector botpolicy 57cd1863/a04571b1, NameCard lists
9aee8b00, nonCopiedSpell d56e404f, plus their merge/doc commits).

## Operation

`git merge main` conflicted on two files:

1. `internal/testutil/agentsdoc_test.go` — HEAD had removed main's provenance
   comment above `knownApproximationRows`; main kept/extended it (both sides
   carried the constant 46). **Resolution:** kept main's comment, rewritten to
   the merged measurement, and lowered the constant to **44** — the merged
   AGENTS.md table measures 44 data rows (base e6a2a84d = 47; this branch
   deleted the First-Strike Damage row [acc7878d], main deleted the NameCard
   row [9aee8b00] and the non<X> row [d56e404f]). `knownOversizeRows = 8`
   kept: the merged table measures 5 oversize rows, still under the cap.
2. `.ds4/report-mrg1.md` — HEAD's side is this ticket's round-1 report (68
   lines); main's side is its own verbatim lineage-record pile (1141 lines,
   which does not contain this ticket's record). **Resolution:** both kept
   verbatim — this ticket's round-1 record first, then main's lineage records
   under a separator heading, then this round-2 record.

`AGENTS.md` itself auto-merged cleanly (union of the three row deletions).

## Issues

None beyond the resolved conflicts.

## Round-3 record — re-dispatch verification (this seat, 2026-09-23)

The round-2 resolver was cut off after committing the merge but before its
completion status was recorded (`status-merge-mrg1.cutoff.json` shows
`status: DONE`, `report_written: false`). This re-dispatch verified the round-2
state rather than redoing it:

- `git status` on entry: tree CLEAN at `1249aca0`, no rebase/merge in flight;
  `acc7878d` (the branch fix) and main @ `e0fd7d9c` both contained; no conflict
  markers anywhere in tracked files.
- The round-2 record above (Operation + Issues, ending "None beyond the
  resolved conflicts") documents the `1249aca0` resolution; `AGENTS.md`
  auto-merged with the union of the three row deletions and no
  First-Strike/NameCard/non<X> rows remain (grep: only the provenance comment
  mentions the closure in `internal/testutil/agentsdoc_test.go`).
- Ratchet re-run (same commands the round-2 brief mandates, all exit 0):
  `go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'` → ok 0.001s;
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'` → ok 0.886s.
  `.cards` symlink present (→ /home/sadams/projects/gorge/.cards), so the run
  is not a vacuous skip.
- Behaviour goldens: `go test ./internal/archtest/` → ok 5.233s;
  `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` → ok
  1.310s (pinned split unchanged). `gofmt -l` on the conflicted file: clean.

Note for the next round: main has since advanced past `e0fd7d9c` to `2040e5d9`
(kw:Strive, the Worthy filter predicate, layer-4 static-grammar closure, a CI
docs refresh). That integration is NOT part of this round's mrg1 conflict and
was not started here; the daemon should dispatch it as its own merge round.

## Issues

None new. The round-2 record's `## Issues` stands (none beyond the resolved
conflicts).


---

## Record — main-side report kept verbatim (main @ 1fc5b37c, task cli-20260922T225140Z-c661fa12)

# Merge-conflict resolution — task cli-20260922T225140Z-c661fa12

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

None found during this round's resolution.

# Round 4 — merge main @ 1fc5b37c into wt/cli-20260922T225141Z-1b1182e4 (2026-09-23)

## Situation found

`git status` on entry: tree CLEAN at `af871344`, no rebase/merge in flight (the
daemon had aborted its failed attempts before dispatching this seat). The
branch's round-3 tip had integrated main @ `e0fd7d9c`; main has since advanced
to `1fc5b37c`, carrying:

- `63c07260`/`463416fd` — the layer-4 filter-grammar fix (its own row closure);
- `9fe4eaf1`/`f545733a` — the Worthy filter predicate;
- `5d8ef1d1`/`2b0f723f` — kw:Strive per-extra-target additional cost;
- `6d0a209a`/`23078d5c` — trig:BecomeMonstrous + `Monstrosity$`;
- `599b3a92`/`1fc5b37c` — stat:Panharmonicon.ValidTurned + trig:TurnFaceUp;
- `77a84b3c`/`2040e5d9` — the coverage-table/CI docs refresh.

Rebase is forbidden in a seat (and the daemon's own rebase attempt conflicted on
`acc7878d` and was aborted); the same `git merge main` method as prior rounds
was used.

## Operation

`git merge main --no-edit` conflicted on exactly two files; all other code
auto-merged cleanly.

### 1. `internal/testutil/agentsdoc_test.go` (comment + constant)

Both sides agreed the constant had moved off the old `46`-row base; neither
side's number was the true merged value.

- HEAD (branch, `af871344`) said 44 with a comment describing the First-Strike
  closure this branch carries;
- main said 44 with a comment describing the layer-4 closure main carries;
- Measured base `e0fd7d9c` = 45 rows; main = 44 (deleted the layer-4 row);
  branch = 44 (deleted the First-Strike row); each side deleted a DIFFERENT
  row, so the merged table keeps NEITHER deletion as a conflict — the
  auto-merged `AGENTS.md` measures **43** data rows.

**Resolution:** set `knownApproximationRows = 43` (the measured merged count)
and replaced both comments with one that names both sides' distinct deletions
(this branch's First-Strike closure `acc7878d` + main's layer-4 closure
`63c07260`, plus main's other merged closures). Verified both deleted rows are
absent from the merged `AGENTS.md`.

### 2. `.ds4/report-mrg1.md` (archive collision)

This file is a force-added lineage archive shared across tickets by filename
(`.ds4/report-mrg1.md`), so main's copy is a *different* ticket's resolver
report (`cli-20260922T225140Z-c661fa12`) while HEAD's copy is this ticket's
accumulated archive. **Resolution: keep BOTH** — HEAD's 1284-line archive
verbatim, then main's 115-line report appended under a clearly marked
`## Record — main-side report kept verbatim` section. No content discarded.

## Commands and output

`git status --short --branch` on entry:

```text
## wt/cli-20260922T225141Z-1b1182e4
```

`git merge main --no-edit`:

```text
Auto-merging .ds4/report-mrg1.md
CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
Auto-merging AGENTS.md
Auto-merging internal/testutil/agentsdoc_test.go
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
Automatic merge failed; fix conflicts and then commit the result.
```

Table counts (`git show <ref>:AGENTS.md`, `| ` lines under `## Known
approximations` minus header): base `e0fd7d9c` = 45, main = 44, branch
`af871344` = 44, merged working tree = 43.

`go test ./internal/testutil/ -run 'TestKnownApproximation' -v`:

```text
=== RUN   TestKnownApproximationsOnlyShrinks
--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)
=== RUN   TestKnownApproximationRowsAreShort
--- PASS: TestKnownApproximationRowsAreShort (0.00s)
PASS
ok  	github.com/adams-shaun/gorge/internal/testutil	0.004s
```

`go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`:

```text
ok  	github.com/adams-shaun/gorge/rules	1.860s
```

`.cards` symlink present (→ /home/sadams/projects/gorge/.cards), so the runs are
not vacuous skips.

## Issues

None new. No engine behaviour was changed by the resolution: it touched the
registers constant + comment, the archive report, and nothing else.


---

## Record — main-side report kept verbatim (main @ 1be022eb, task cli-20260922T225141Z-48972afc)

# Merge-conflict resolution — task cli-20260922T225141Z-48972afc

## Situation found

`git status` was CLEAN on entry — no rebase or merge in flight (the daemon's
earlier attempt had left nothing behind). The branch tip was `7c4182ff`
("fix(rules): resolve mulligan redraws at the end of the declaration pass"),
one commit past the merge base `0104252d`; main had advanced to `2040e5d9`,
39 commits ahead. The owed integration was a merge of main into the branch
(rebase is forbidden in a seat).

`.cards` was **present** — a real symlink to
`/home/sadams/projects/gorge/.cards`, target exists — so every corpus-backed
ratchet below ran for real rather than skipping.

## Conflicted files and resolutions

`git merge main` produced **one** content conflict: `internal/testutil/agentsdoc_test.go`.
`AGENTS.md` auto-merged (it carried no textual conflict — each side had deleted
a different row).

### `internal/testutil/agentsdoc_test.go` (conflict: the `knownApproximationRows` constant)

Both sides changed the register constant:

- **HEAD (branch):** `knownApproximationRows = 49`. The branch base's table was
  50 rows; the mulligan ticket deleted one row and lowered 50 → 49.
- **main:** `knownApproximationRows = 44`, with a comment recording main's own
  closures (non<X> `d56e404f`, NameCard ChooseFromList$/AtRandom$ `78d3b764`,
  the battle protector row `f76f59fd`, plus pc1/each1/CR 616.1) and the
  layer-4 closure from **another** merge of this same branch (`63c07260`),
  which main had already absorbed.

Neither number is the merged count. The merged table was **measured** (same
method the test uses: lines starting `"| "` inside `## Known approximations …
next "## "`, minus the header): **43 data rows**. HEAD carried 48, main 44,
and the union of both sides' deletions (mulligan row on the branch; main's
four) gives 43. Verified the four closed rows are absent from the merged
`AGENTS.md`: the old mulligan row (`mulligan declaration's REDRAW`), the
`non<X> negation` row, the `layer-4 type grants reach only` row and the
`NameCard asks over` row — all grep to 0 occurrences.

**Resolution:** kept main's commented shape (which explains the other
closures) and wrote the merged count accurately:

    // The merged table measures 43 rows: main's closures merged here
    // (non<X> via d56e404f, NameCard ChooseFromList$/AtRandom$ via 78d3b764,
    // the battle protector row via f76f59fd, and the earlier pc1/each1/CR
    // 616.1 closures), plus this branch's own mulligan-redraw deferral
    // (fix 7c4182ff), which deletes the "mulligan declaration's REDRAW
    // resolves immediately" row.
    knownApproximationRows = 43

This is the union of the two sides' intents, not a redesign. The row-count
gate passes (`TestKnownApproximationsOnlyShrinks`, `TestKnownApproximationRowsAreShort`
— the package test is green).

## Integration failures found after the merge — and why they are part of resolving it

The merge itself is a **semantic** integration conflict, not just a textual
one. main added a whole feature after this branch forked — the hypothetical
chance planner (`rules/chance.go`, `rules/chance_test.go`) — and three of its
tests observe a mulligan's REDRAW inside the mulligan submit, because on main
`handleMulligan` still resolves the redraw immediately. This branch's reviewed
fix deliberately *defers* the redraw to the declaration-pass boundary (CR
103.4/103.5, `resolveMulliganRedraws`), which is the entire purpose of the
ticket. Both sides kept, so main's three tests now observe the wrong moment.

The three failures, measured on the merged tree before the fix below:

    --- FAIL: TestHypotheticalPlannerControlsMulliganShuffle (0.00s)
        chance_test.go:223: mulligan shuffle=[9 2 11 6 5 12 10 3 4 7 1 8] want []
    --- FAIL: TestHypotheticalReplayAndCloneOwnChanceState (0.00s)
        chance_test.go:448: mulligan did not consume chance
    --- FAIL: TestHypotheticalSubmitFailurePoisonsOnlyThatBranch (0.00s)
        chance_test.go:504: error = <nil>

I confirmed the underlying behaviour is intact by instrumenting a scratch test:
after the mulliganing seat declares and the **rest of the pass keeps**, the
planner's Ordinal-1 callback fires with exactly the context main's test
expects (`hand=0 lib=12`) and produces the planned permutation. Only the
*when* moved (from the submit to the pass boundary), exactly as CR 103.4/103.5
requires.

`rules/chance_test.go` was NOT a conflicted file. This is a documented
deviation from "do not touch files the conflict does not involve": leaving it
red would wedge the daemon's full gate, and reverting the branch fix would
destroy the ticket. The integration change is test-only and preserves each
test's intent:

- **TestHypotheticalPlannerControlsMulliganShuffle** — records the mulliganing
  seat, then drives the rest of the pass with a keep (new `submitPregameKeep`
  helper) until the planner callback has run, then asserts `lastShuffle` equals
  the planned order. The planner still controls the mulligan shuffle.
- **TestHypotheticalReplayAndCloneOwnChanceState** — after the clone's mulligan
  submit, asserts the source engine is untouched, then completes the clone's
  pass and only then asserts chance was consumed; completes `e`'s and the
  replay's pass identically before the transcript comparison. The clone/chance
  ownership contract is unchanged.
- **TestHypotheticalSubmitFailurePoisonsOnlyThatBranch** — the mulligan submit
  now succeeds (the poisoned draw is consumed by the deferred redraw); the
  pass-completing keep is the call that surfaces the `bound` chance failure;
  the poison/head-immutability and base-vs-branch assertions follow. The
  chance-failure boundary contract is unchanged.

A new helper `submitPregameKeep` documents the deferral in one place.

## Commits produced

- `61779fd9` — `Merge branch 'main' into wt/cli-20260922T225141Z-48972afc`
  (default merge message; parents `7c4182ff` + `2040e5d9`). Contains the
  `agentsdoc_test.go` resolution.
- `f0bc40a3` — `test(rules): drive the deferred mulligan redraw in the chance
  planner tests` (the `rules/chance_test.go` integration; no engine change).

## Commands run and output (real)

`.cards`: **present** (real symlink, target exists) — corpus tests ran.

    $ git merge main
    Auto-merging AGENTS.md
    Auto-merging internal/testutil/agentsdoc_test.go
    CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go

Merged-row measurement (test's own method):

    raw rows: 44 => data rows: 43
    HEAD (branch) table: 48 rows; main table: 44 rows; merged: 43 rows

Conflict-resolution gate — the conflicted file's package:

    $ go test -run 'TestKnownApproximation' ./internal/testutil/
    ok  	github.com/adams-shaun/gorge/internal/testutil	0.001s
    $ go test ./internal/testutil/
    ok  	github.com/adams-shaun/gorge/internal/testutil	1.475s

The three main-planner tests, before and after the integration fix:

    before: 3 FAIL (above)
    $ go test -run 'TestHypothetical' ./rules/ -v
    --- PASS: TestHypotheticalPlannerControlsMulliganShuffle (0.00s)
    --- PASS: TestHypotheticalReplayAndCloneOwnChanceState (0.00s)
    --- PASS: TestHypotheticalSubmitFailurePoisonsOnlyThatBranch (0.00s)
    (... every other TestHypothetical* PASS)
    ok  	github.com/adams-shaun/gorge/rules	0.007s

Post-merge ratchets (the command the brief names):

    $ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
    ok  	github.com/adams-shaun/gorge/rules	0.753s

Broader targeted mulligan/pregame suite in `rules/`:

    $ go test ./rules -run 'Mulligan|Pregame|Bottoming|Hypothetical|Redraw|StartingPlayer|Kept|Keep'
    ok  	github.com/adams-shaun/gorge/rules	0.770s

Mandatory goldens outside `rules/` (per system context):

    $ go test ./internal/archtest/
    ok  	github.com/adams-shaun/gorge/internal/archtest	3.013s
    $ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
    ok  	github.com/adams-shaun/gorge/cmd/botbench	1.101s

Format / build:

    $ gofmt -l internal/testutil/agentsdoc_test.go rules/chance_test.go
    (no output)
    $ go run ./cmd/gentypes -check
    (no output)
    $ go build ./rules/ ./internal/testutil/
    (clean)

Final state:

    $ git status --short          → (empty, clean)
    $ grep -rn '^<<<<<<<|^>>>>>>>' → (no conflict markers)

## TestHeads — expected branch movement, deliberately NOT edited

    $ go test -run 'TestHeads$' ./rules/
    --- FAIL: TestHeads (1.71s)
        heads_test.go:1323: 4 seats: chain head acce7d850cfb176a, golden 20028059e8c88ec3
        heads_test.go:1323: 6 seats: chain head 3bd695df72d9d4c9, golden 400d8d9ae2777ded
        heads_test.go:1323: 8 seats: chain head 5c90b1b3a0b38f25, golden 5e988854bf022347

These are **exactly** the values the branch's own ticket measured and
attributed (`acce7d850cfb176a` / `3bd695df72d9d4c9` / `5c90b1b3a0b38f25`; 2
seats unmoved at `19a4893657e5d549`). My resolution introduced no engine
behaviour change, so nothing from main moved them further. Per the system
context, `rules/heads_test.go` is not edited by agents — the orchestrator
re-pins it at the gate from measured values (FL-107), and the ticket's report
(`.ds4/report-t1.md`) already carries the attribution. NOT a blocker for this
resolution.

## Uncertainties / notes

- The `rules/chance_test.go` edit is the one deliberate scope extension, forced
  by main's new feature observing the behaviour this ticket changes. It is
  test-only; the alternative (leave 3 tests red, or revert the reviewed fix)
  both fail the brief.
- The `knownApproximationRows` number is the measured merged count (43), not
  either side's stale constant; I verified the four closed rows are gone.

## Issues

- No new defects found beyond the integration above.
- Coverage note (already recorded by the ticket): the repo-deck acceptance
  suite runs exactly one mulligan per game and at 2 seats that mulliganer is
  the last declarer, so the 2-seat acceptance golden does not exercise the
  deferred-redraw interleaving at all; the branch's
  `rules/mulligan_redraw_order_test.go` covers the multi-declarer shapes
  directly.
- Out of scope and untouched: the adjacent AGENTS.md row "The starting player
  is uniformly random but the toss winner never CHOOSES" remains in the table.

# Round 5 — second integration, main @ 1be022eb (2026-09-23)

## Why a second merge in the same session

The round-4 merge integrated main @ `1fc5b37c` and completed as `84b39833`.
While that work was in progress, the shared `main` ref advanced again
(`git reflog main`): another resolver merged ticket
`cli-20260922T225141Z-48972afc` (the mulligan-redraw deferral, commit
`7c4182ff` + its test/heads/docs follow-ups) into main, moving it to
`1be022eb`. `1fc5b37c` is still an ancestor of `84b39833`, but `main` no longer
was, so a second `git merge main` was required to leave the branch actually
containing current main (otherwise the daemon's gate-and-merge path would
immediately re-conflict and re-dispatch, as the round-1..3 history shows).

## Operation

`git merge main --no-edit` conflicted on the same two files as round 4; every
other file auto-merged (including `rules/heads_test.go`, whose new goldens came
from the mulligan ticket).

### 1. `internal/testutil/agentsdoc_test.go`

Main @ `1be022eb` deletes one further row (the "mulligan declaration's REDRAW
resolves immediately" row, closed by `7c4182ff`). The merged table now keeps
ALL distinct deletions from both lineages — First-Strike (this branch,
`acc7878d`), layer-4 (`63c07260`), mulligan-redraw (`7c4182ff`), non<X>
(`d56e404f`), NameCard (`78d3b764`), battle protector (`f76f59fd`) and the
pc1/each1/CR 616.1 closures — and measures **42** data rows. Resolved
`knownApproximationRows = 42` with a comment listing every closure; verified
the First-Strike, layer-4 and mulligan row texts are all absent from AGENTS.md.

### 2. `.ds4/report-mrg1.md`

Same archive collision as round 4: main @ `1be022eb` carries ticket
`48972afc`'s resolver report for this shared filename. **Resolution: keep
BOTH** — the `84b39833` archive verbatim, then main's 48972afc report appended
under a marked `## Record — main-side report kept verbatim` section.

## Commands and output

Table counts (`| ` lines under `## Known approximations`, minus header):
`1fc5b37c` = 44, `1be022eb` = 43, merged = 42.

`go test ./internal/testutil/ -run 'TestKnownApproximation'`: see round-6
verification below (same command, re-run post-second-merge).

## Issues

None new. No engine behaviour changed by the resolution itself.

## Round-5 verification (post-second-merge, all exit 0)

Merged as `107b6ee5` (parents `84b39833`, main `1be022eb`); main is now an
ancestor of HEAD and the tree is clean.

```text
go test ./internal/testutil/ -run 'TestKnownApproximation'          ok
go test -count=1 ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'  ok
go test -count=1 ./rules -run 'TestHeads'                           ok
go test ./internal/archtest/                                        ok
go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/  ok (pinned split unchanged)
gofmt -l internal/testutil/agentsdoc_test.go                        clean
```

`.cards` symlink present (→ /home/sadams/projects/gorge/.cards); no vacuous
skips. `knownApproximationRows = 42` (measured 42 data rows).

## Issues

None new across either integration. This resolution changed no engine
behaviour: the only non-archive edit was the row-register constant and its
comment in `internal/testutil/agentsdoc_test.go`.
