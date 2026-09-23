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

# Merge-conflict resolution report — mrg1 (ticket cli-20260922T225140Z-c9271412)

## Starting state

`git status` showed a CLEAN tree on `wt/cli-20260922T225140Z-c9271412` at the branch
commit 3c9da13e ("fix(rules): fizzle infeasible distinct Charm target declarations";
branch series 1aedc150 → 65f6d053 → 11db0cb0 → 3c9da13e), with `main` (00147db0) NOT
an ancestor — the dispatch log's failed rebase and merge-fallback attempts had already
been rolled back (no rebase-merge/REBASE_HEAD/MERGE_HEAD in flight). So there was no
in-flight operation to finish; I performed the integration myself as a **merge of
main** (merge, not rebase — this repo's standing rule forbids `git rebase`, and main's
history shows the merge shape is the established integration path for wt branches).

`.cards` was present (real corpus, so corpus-backed tests ran, not skipped).

## Conflicted file — `internal/testutil/agentsdoc_test.go` (only conflict)

Both sides touched the `knownApproximationRows` constant:

- **HEAD (branch):** `knownApproximationRows = 73` — the branch's ticket deleted the
  Charm row from AGENTS.md's Known-approximations table (auto-merged cleanly) and
  lowered the constant.
- **main:** `knownApproximationRows = 72`, comment naming main's stack-option-kind
  closure ("a spell on the stack is offered with Option.Kind permanent", c12e10ef)
  plus the Dig closure.

**Resolution:** measured the MERGED AGENTS.md table with the test's own counting rule
(all `| ` lines between `## Known approximations` and the next `## `, minus the header
row): **71 data rows** = 73 (branch measured) − 2 (main's closures since the split).
Set `knownApproximationRows = 71` with a comment naming both sides' contributions.
Note: a naive awk that stops at the first blank line undercounts by one, because a
stray blank line sits between the last two table rows on BOTH branch and main
(pre-existing in both; the test's region extends past it and counts the
`api:ExchangeLifeVariant` row after it). The first draft set 70 from that awk; the
test's own measurement corrected it to 71 — the test itself is the arbiter.

## Semantic (non-textual) conflict — `rules/copy_target_provenance_test.go`

Git auto-merged main's `copy_target_provenance_test.go` (6246e568, "preserve copy
target declaration provenance and modal modes") with the branch's charm machinery in
`rules/stack.go`, but the test then FAILED:

```
--- FAIL: TestCopyCharmAsksEveryChosenTargetMode
    copy_target_provenance_test.go:124: submit [2]: expected 2..2 choices, got 1
```

What each side wanted:

- **main's test:** for Winterflame (`CharmNum$ 2`, two modes, one creature target
  each) the cast's target ask accepted ONE submitted target (main's pre-branch
  per-mode/legacy shared-list shape), then the test exercises the COPY
  (Mirrorpool) path's per-declaration `copy_targets` provenance — the test's real
  subject.
- **branch (the reviewed fix, 11db0cb0 + 1aedc150):** a distinct-mode modal cast now
  poses ONE grouped KTarget decision, Min=2/Max=2, one exclusive group per mode
  (`charm-mode-0`/`charm-mode-1`) — asserted verbatim by the branch's own
  `TestDistinctCharmModesKeepTheirOwnTargets`.

Per the ground rules the branch's behaviour is the approved fix and main carries no
later deliberate change to that ask shape (main's 6246e568 is about the COPY path's
provenance, not the cast ask's shape), so the branch's ask wins and main's TEST was
brought up to it: the bear is offered once per mode group, so the cast target ask is
answered with both options (`found, found2`), keeping every copy-provenance assertion
downstream unchanged. The full test now passes.

## Rot-guard fix — `rules/paramcensus_test.go`

The ratchet run also flagged the paramcensus rot guard (this appears only after the
merge, because the branch's charm code is what introduced the reads):

```
paramcensus: 2 unclassified Params reads (the census cannot rot):
cast.go:6179:29:  unclassified Params base "root"
stack.go:1822:27: unclassified Params base "root"
```

Both read `root.Params["Choices"]` where `root` is a resolved `*cards.SA`
(cast.go's `f.SpellAbility()`, the same shape `o.Ability` covers; stack.go's
`askCharmModeTargets` root parameter). Classified as `"root": bSA` in `baseBuckets`
with a justification comment. No behaviour change.

## Commands run (with real outcomes)

- `git merge main` → `CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go`;
  AGENTS.md, rules/stack.go, rules/clone.go, rules/heads_test.go, cmd/botbench,
  effects/* etc. auto-merged.
- `go test -run 'TestKnownApproximations' ./internal/testutil/` → first FAIL
  ("lower knownApproximationRows to 71" — corrected the awk-derived 70 to the
  test-measured 71), then **ok**.
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  → first FAIL (paramcensus rot guard above), then **ok** after the baseBuckets entry.
- `go test ./rules -run 'Charm|TestHeads'` → first FAIL
  (TestCopyCharmAsksEveryChosenTargetMode), then **ok** after the test update; TestHeads
  passed both times (no head movement — resolution restored the branch's own reviewed
  behaviour, and the branch series already carried any re-pin it needed).
- `go test -run 'Charm' ./effects/` → **ok**.
- `go test ./internal/archtest/` → **ok**.
- `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` → **ok**
  (botbench split did NOT move: the repo decks do not exercise the grouped charm ask
  in a way that changes bot decisions).
- `gofmt -l` on the three edited files → clean.
- `git add` + `git commit` (merge) + `git commit --amend --no-edit` to fold the three
  resolution edits into the merge commit → **167bb13a** "Merge branch 'main' into
  wt/cli-20260922T225140Z-c9271412" (parents: 3c9da13e branch tip, 00147db0 main tip).
- `git status --short` → clean.

## Deviations / notes

- I edited `rules/copy_target_provenance_test.go` (a main-side test) — beyond the one
  textually conflicted file, but this is exactly the "both sides' intent" case: the
  branch's reviewed ask shape is preserved, main's copy-provenance coverage is
  preserved, and the daemon's full gate would otherwise fail. The edit only changes how
  the cast target ask is ANSWERED (both mode-group options instead of one); no
  assertion about the copy path was weakened.
- `knownApproximationRows = 71` matches the merged table exactly (no slack); the
  `knownOversizeRows` constant was outside the conflict region and needed no change.

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
- The stray blank line inside the Known-approximations table (between the `kw:Infect`
  row and the `api:ExchangeLifeVariant` row) exists on both sides' AGENTS.md. Harmless
  to the test (its region counts past it) but it breaks Markdown table rendering for
  the last row and it silently breaks naive line-scanners that stop at the first blank
  (as mine did). A docs ticket could remove it and lower the constant by 1.
- No other defects found; nothing in the ledger was closed by this merge beyond the
  branch ticket's own Charm row (AGENTS.md row already deleted by the branch series).
