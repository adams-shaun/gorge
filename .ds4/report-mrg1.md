# Merge conflict resolution mrg1

- `internal/testutil/agentsdoc_test.go`: the branch lowered `knownApproximationRows` from 88 to 87 after deleting its stale cost-verb approximation row. Main independently removed three other rows and set its count to 84. The merged `AGENTS.md` includes both sides' deletions; its measured data-row count is 83, so the resolved constant is 83.
- `AGENTS.md`: main's independent table updates were retained alongside the branch's deletion of the now-false Exile/Discard/Return cost row. No unrelated files were manually edited; merge changes from main are retained as part of integration.

Commands and results:
- `git status` — initially clean; no operation was in flight. The supplied conflict note described the failed rebase, but current branch was clean.
- `git merge main` — content conflict in `internal/testutil/agentsdoc_test.go` only; `AGENTS.md` auto-merged.
- Counted Known approximations rows from both refs: branch `HEAD` had 86, main had 84 (main still contained the cost-verb row). Combined intended state is 83.
- `[ -e .cards ]` — `CORPUS_PRESENT`.
- `go test ./internal/testutil/ -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'` — `ok github.com/adams-shaun/gorge/internal/testutil 0.001s`.
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'` — `ok github.com/adams-shaun/gorge/rules 0.855s`.

Uncertainty: none. Main's three independent row deletions plus this branch's one deletion account for the combined count of 83.
