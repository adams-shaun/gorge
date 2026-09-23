# Merge conflict resolution report — cli-20260922T225141Z-462eca2e

## Operation and conflict

- Initial `git status`: CLEAN, branch `wt/cli-20260922T225141Z-462eca2e`, no rebase or merge in
  flight — the daemon's rebase attempt (conflict on applying `89c77778`) and its merge fallback
  had both been rolled back before this seat started. Merge base `2040e5d9`; branch 2 commits
  ahead, main 30 ahead.
- `git rebase` is forbidden to a seat here, so integrated with `git merge main` (same shape other
  branches used, e.g. `5c5370ff`). One content conflict: `internal/testutil/agentsdoc_test.go`.
  `AGENTS.md` and everything else auto-merged.
- The conflict was the `knownApproximationRows` ratchet constant: HEAD said 43, main said 42.
  Measured, did not trust either comment: data rows in the table (`awk` between the
  `| Stand-in |` header and the next `## `, separator excluded) — merge base `2040e5d9` = 44,
  HEAD = 43 (this branch's attackprop1 row deletion, commit `89c77778`), main = 42 (main's
  Phase$ First Strike Damage and mulligan-REDRAW deletions, confirmed by the row-level diffs
  against the base). The three deletions are disjoint, so the merged table keeps all of them:
  **41**. Resolved to `knownApproximationRows = 41` with a comment recording the measurement.
  `knownOversizeRows` was untouched by both sides, so no adjustment was needed there.
- Wrote this report over the staged `.ds4/report-mrg1.md` (tracked file; both parents carry the
  same convention of each merge seat committing its own report) and amended it into the merge
  commit with `git commit --amend --no-edit` (parents preserved: `1c3df172` + `edc24484`).
  Merge commit: the current branch tip `Merge branch 'main' into
  wt/cli-20260922T225141Z-462eca2e` (this report is amended into it, so the
  commit that contains this line post-dates any sha printed here; the sha
  before the report amend was `a53d452a`, and before that `642425df`).

## Commands and outputs

`git status` on entry: clean, no operation in progress (verified no MERGE_HEAD / rebase dirs
under the worktree gitdir).

`git merge main`:

```text
Auto-merging AGENTS.md
Auto-merging internal/testutil/agentsdoc_test.go
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
Automatic merge failed; fix conflicts and then commit the result.
```

Row measurements (`git show <ref>:AGENTS.md` piped through the awk counter):
`2040e5d9: 44  HEAD: 43  main: 42`, merged working tree: 41.

`git add internal/testutil/agentsdoc_test.go && GIT_EDITOR=true git merge --continue` →
`642425df Merge branch 'main' into wt/cli-20260922T225141Z-462eca2e`, parents `1c3df172`/`edc24484`.

Ratchets (the merge brief's command, `-count=1` to prove nothing was cached, corpus symlink
present and no SKIP lines):

```text
go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead' -v -count=1
--- PASS: TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched (0.00s)
--- PASS: TestEveryRepoDeckParamsAreRead (0.13s)
--- PASS: TestEveryRepoDeckIsFullySupported (0.60s)
--- PASS: TestEveryRepoDeckCountHeadResolves (0.00s)
--- PASS: TestEveryDispatchedTriggerModeHasAMatcher (0.00s)
ok      github.com/adams-shaun/gorge/rules      0.770s
```

Conflicted file's package:

```text
go test ./internal/testutil -run 'TestKnownApproximation' -v
--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)
--- PASS: TestKnownApproximationRowsAreShort (0.00s)
ok      github.com/adams-shaun/gorge/internal/testutil  0.001s
```

Behaviour goldens (system doc):

```text
go test ./internal/archtest/                                     → ok  4.794s
go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ → ok  1.108s
```

`git status` after the amend: working tree clean.

## Issues

None new. The merge introduced no engine change of its own; the branch's attack-prop fix
(`89c77778`, `1c3df172`) and main's closures compose cleanly and every ratchet and golden above
passes at the merged tip.
