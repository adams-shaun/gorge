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

## Starting state

`git status` showed a CLEAN tree on `wt/cli-20260922T225141Z-8d166e9c` at the branch
commit 0679b1cd ("feat(rules): foretell predicates, Cosmos Charger any-turn grant,
effect-delivered free may-play"), 1 commit ahead of merge-base 270448df. The failed
rebase/merge attempts from the dispatch log had already been rolled back — no
`.git/rebase-merge`, `.git/rebase-apply` or `MERGE_HEAD` present. So there was no
in-flight operation to finish; I performed the integration myself as a **merge of
main** (merge, not rebase — this repo's standing rule forbids `git rebase`, and
main's history shows this is the established integration shape for wt branches).

## Conflicts and resolution

One file: `internal/testutil/agentsdoc_test.go`, hunk on the `knownApproximationRows`
constant:

- **HEAD (branch 0679b1cd):** `knownApproximationRows = 87` — the branch's ticket
  deleted one Known-approximations row (the foretell/Cosmos Charger/MayPlay row) from
  AGENTS.md (auto-merged cleanly) and lowered the constant accordingly (88 → 87).
- **main (e53c80a2):** `knownApproximationRows = 76` — main removed rows via its own
  landed tickets since the split.

## Resolution

The auto-merged AGENTS.md contains BOTH sides' row deletions. Measured with awk over
the merged file: **74 data rows** (base 88 − 1 branch deletion − 13 main deletions).
Set the constant to the measured merged value: `knownApproximationRows = 74`. This is
correct in both directions of the ratchet: no row was added, and the constant exactly
matches the table, so `TestKnownApproximationsOnlyShrinks` passes with no slack and no
false failure. `knownOversizeRows = 8` needed no change (main's side of the conflict
region only). No test logic or other file was touched.

Note: the `knownApproximationRows` values in the main-side history are stale relative
to the table (main's own table measured 75 data rows against a constant of 76 — a
prior resolver's report-mrg1.md in main's tree says the same); the test only fails on
growth so this never bites, but worth knowing.

The merge also auto-merged the branch fix's real code files against main's later
changes to the same files: `effects/filter.go`, `effects/misc.go`, `rules/legal.go`,
`rules/paramcensus_test.go` — git merged these cleanly and the branch's own tests
still pass (below).

## Commands run

- `git merge main` → `CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go`, all other files auto-merged.
- Row count: `awk` over merged/HEAD/main AGENTS.md → merged 74 data rows, branch 87, main 75.
- Edit constant → `git add internal/testutil/agentsdoc_test.go` → `git commit --no-edit`
  → merge commit **98dcf594** ("Merge branch 'main' into wt/cli-20260922T225141Z-8d166e9c").
- `go test ./internal/testutil/ -run 'TestKnownApproximations'` → `ok 0.001s`.
- Post-merge ratchets (per brief):
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeckIsFullySupported|TestEveryRepoDeckParamsAreRead|CountHeadRatchet' -v`
  → exit 0, 0 SKIPs, all PASS (TestEveryRepoDeckIsFullySupported 0.57s,
  TestEveryRepoDeckParamsAreRead 0.11s, both trigger-mode registry tests PASS).
  `go test ./rules -run 'TestEveryRepoDeckCountHeadResolves'` (the actual name of the
  CountHead ratchet) → `ok 0.600s`.
- Branch-fix sanity: `go test ./rules -run 'Foretell|MayPlay'` → `ok` (the branch's
  `rules/foretell_grant_test.go` suite still passes on the merged tree).
- `gofmt -l internal/testutil/agentsdoc_test.go` → empty.
- `git status` → clean.

`.cards` was present (symlink to the real corpus, `ir.gob.gz` resolves), so these runs
are real, not vacuous skips.

## Uncertainties

- This report file path itself is a TRACKED file in main (a previous pipeline's
  resolver committed their mrg1 report there for ticket cli-20260922T225141Z-d9f9a1a7).
  Per the report contract I wrote to exactly this path; the old content remains in git
  history. The report is committed as a separate `docs` commit so the branch ends
  clean.
- Commit messages in this repo carry no `Ref:` trailer and no attribution (gorge
  rules); the merge used the default generated message, the report commit is
  `docs(merge): mrg1 conflict-resolution report`.

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
