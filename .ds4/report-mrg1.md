# Merge-conflict resolution — task cli-20260922T225140Z-6a16cd8c

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
