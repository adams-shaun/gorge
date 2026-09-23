# Merge-conflict resolution — task cli-20260922T225143Z-4b0bde0d

## Situation found

`git status` was clean on entry, with no rebase or merge in progress. HEAD was `b5f51b7d` (`fix(rules): resume transactional life exchanges`), and `main` was `2040e5d9`. The daemon's failed rebase was therefore not in progress in this worktree. I started the merge fallback with `git merge main`; it auto-merged the other files and conflicted only in `internal/testutil/agentsdoc_test.go`.

`.cards` was present.

## Conflicted file and resolution

### `internal/testutil/agentsdoc_test.go`

The branch side set `knownApproximationRows = 49` for its own deleted exchange-life-variant row. Main's side set it to 44 after its intervening closures and carried a comment describing its merged-table count. Neither constant alone reflected the combined result. I counted rows from the auto-merged `AGENTS.md`: 43 data rows. The exchange row is removed, as are main's rows already closed by main. Resolved the hunk to `knownApproximationRows = 43`, with a concise comment noting main's 44 rows and this branch's additional deletion.

`AGENTS.md` auto-merged without a conflict. Its resulting table count is 43, consistent with the constant and the branch's reviewed row deletion.

## Commands and output

- `git status --short --branch && git rev-parse --abbrev-ref HEAD` → `## wt/cli-20260922T225143Z-4b0bde0d`; branch is the expected worktree branch.
- `git merge main` → auto-merged `AGENTS.md` and `rules/engine.go`; content conflict in `internal/testutil/agentsdoc_test.go`.
- `.cards` check → `.cards present`.
- Counted rows in merged `AGENTS.md` with a Python table-heading scan → `data rows 43`.
- `go test ./internal/testutil -run 'TestKnownApproximations' -v` →
  ```
  === RUN   TestKnownApproximationsOnlyShrinks
  --- PASS: TestKnownApproximationsOnlyShrinks (0.00s)
  PASS
  ok   github.com/adams-shaun/gorge/internal/testutil 0.001s
  ```
- Post-merge ratchets, `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'` →
  ```
  ok   github.com/adams-shaun/gorge/rules 1.322s
  ```
- `git diff --check` → no output.

## Uncertainties

None. The measured row count and targeted approximation ratchet agree. Main's other changes were auto-merged; no unrelated files were edited for conflict resolution.

## Issues

None found during this integration pass.
