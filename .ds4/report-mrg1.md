# Merge resolution report — mrg1

## Conflict

- `internal/testutil/agentsdoc_test.go`: main's side included the later `kw:Flanking` and `battle1` row deletions and set `knownApproximationRows` to 27; the reviewed branch also removed the `(castfilter1/2)` row and had the older constant 28. Kept all of main's updates and the branch's CTMS ref-head deletion, set the constant to 26, and updated the explanatory comment to describe the merged deletions. Measured the merged `AGENTS.md` table at 26 data rows before resolving. No other conflicted files.
- The merge auto-merged `AGENTS.md` and `rules/cast.go`, preserving both sides' changes; no manual changes were needed there.

## Commands and results

- `git status --short --branch; git status` before integration: `## wt/cli-20260923T060000Z-ctms-refhead`; clean, no operation in progress.
- `git merge main`: initially failed with a content conflict only in `internal/testutil/agentsdoc_test.go` (expected conflict); other files auto-merged.
- Counted rows from the staged merged `AGENTS.md`: `staged AGENTS data rows: 26`.
- `.cards` check: present as a symlink to `/home/sadams/projects/gorge/.cards`.
- `go test -run 'TestTriggeredCardCastTotalManaSpent|TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead' ./rules/`: `ok github.com/adams-shaun/gorge/rules 0.981s` (exit 0).
- `go test -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' ./internal/testutil/`: `ok github.com/adams-shaun/gorge/internal/testutil 0.001s` (exit 0).

## Uncertainty / concerns

None. Both requested ratchet groups and the conflict-specific CTMS tests passed.
