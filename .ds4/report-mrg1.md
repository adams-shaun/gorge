# Merge conflict resolution — mrg1

## Conflict: `internal/testutil/agentsdoc_test.go`

- **Branch side:** the approved cascade fix had deleted the `cascade1` approximation row and set `knownApproximationRows = 40`, documenting that branch's deletion.
- **Main side:** the token-replacement change had deleted a different row and also had a 40-row count, documenting main's accumulated row deletions.
- **Resolution:** retained both table deletions (the merged `AGENTS.md` has neither `cascade1` nor `tokrepl1`) and set the count to 39, the actual merged table size. The comment names both deletions. No engine behavior was changed as part of conflict resolution.

No other file was conflicted. Main's non-conflicting changes were retained.

## Commands and results

- `git status --short --branch` — clean before starting the merge; branch `wt/cli-20260922T225140Z-9e382c75`.
- `git merge main` — began the merge; auto-merged `AGENTS.md` and the non-conflicting main changes; reported a content conflict only in `internal/testutil/agentsdoc_test.go`.
- `python3` row-count check against merged `AGENTS.md` — `rows: 39`, `cascade1: False`, `token-replacement: False`.
- `go test -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' ./internal/testutil/` — `ok github.com/adams-shaun/gorge/internal/testutil 0.002s`.
- `.cards` check — present.
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'` — `ok github.com/adams-shaun/gorge/rules 1.004s`.

## Issues

No new engine issue was investigated or found during this integration-only resolution. The cascade ticket's remaining deviations and follow-up tickets are recorded in its existing ticket report; the cascade approximation row itself is deleted as intended.
