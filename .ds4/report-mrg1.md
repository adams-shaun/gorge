# Merge-conflict resolution — task cli-20260922T225143Z-4b0bde0d

## Operation

Entry `git status --short --branch` showed a clean worktree on
`wt/cli-20260922T225143Z-4b0bde0d`; no rebase or merge was in progress. The
branch already included main as of `1be022eb`, but not current `main` at
`edc24484`. Ran `git merge main --no-edit`; it stopped on the two conflicts
listed below. Resolved them, staged the resolved files, and completed the
merge with its default message.

`.cards` was present and resolves to `/home/sadams/projects/gorge/.cards`.
Corpus-dependent ratchets therefore did not silently skip for lack of corpus.

## Conflicts and resolutions

### `internal/testutil/agentsdoc_test.go`

The branch comment documented a 42-row merged approximation table, including
its transactional ExchangeLifeVariant closure. Main's conflicting comment
also said 42, but documented the First-Strike Damage closure and its other
row deletions. The auto-merged `AGENTS.md` contains both sides' deletions; a
measurement of the resolved table showed **41 data rows**. Kept the branch's
ExchangeLifeVariant closure and the First-Strike closure in the combined
comment, with the main-side closures, and set `knownApproximationRows` to 41.

Measured counts (`AGENTS.md` data rows): `HEAD` 42, `main` 42, merged working
tree 41. The ExchangeLifeVariant row is absent from the merge; main's
First-Strike row remains absent as well. No approximation row was added or
grown.

### `.ds4/report-mrg1.md`

This is a shared tracked report path, so the two sides contained different
merge-resolver reports. Kept this task's report rather than main's unrelated
report for another ticket, and replaced it with this task's current resolution
record, as required by the report contract.

### `AGENTS.md` and other merged paths

`AGENTS.md` auto-merged. Its distinct table edits from both histories are
retained. The other main changes (including `decision/`, `effects/`, `rules/`,
and `state/` paths shown by `git status`) also auto-merged without textual
conflict and were left intact.

## Commands and output

- `git status --short --branch && git status` (before merge):
  `## wt/cli-20260922T225143Z-4b0bde0d`; `nothing to commit, working tree clean`.
- `git merge main --no-edit`:
  stopped with content conflicts in `.ds4/report-mrg1.md` and
  `internal/testutil/agentsdoc_test.go`; other paths auto-merged.
- `git diff --check` before report replacement found conflict markers in the
  report, as expected while resolving. The conflict markers were removed.
- `[ -e .cards ] && readlink -f .cards`:
  `/home/sadams/projects/gorge/.cards`.
- `go test -run 'TestKnownApproximations' ./internal/testutil/`:
  `ok   github.com/adams-shaun/gorge/internal/testutil  0.001s`
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`:
  `ok   github.com/adams-shaun/gorge/rules  1.044s`

## Issues

No new engine defects found during conflict resolution. The worktree's
transactional life-exchange fix and main's changes are preserved. The
approximation ratchet was lowered to the measured merged table count of 41.

## Round 3 — third main integration (2026-09-23 08:44)

After the round-2 merge (`eaeef0b2`, main at `edc24484`) landed, main advanced
to `fead1e59` (sibling ticket `cli-20260922T225141Z-5641f97b` merged its
round-1 work: `0836163f fix(rules): price the repeatable-cost count bound at
the real charge`). Ran `git merge main --no-edit`; one conflict:

### `.ds4/report-mrg1.md`

Same shared-path collision as round 2: main's copy is the sibling ticket
`5641f97b`'s own `mrg1` report (its rounds 1 and 2, reached through its merge
chain); this file at the canonical path is THIS ticket's report. Kept ours and
appended this section, as in both earlier rounds.

### `internal/testutil/agentsdoc_test.go`

Auto-merged to the constant 41 this branch set in round 1 (main's 50->49 and
42 lineages both reduced to the same line; git kept our 41). Counting the
merged `AGENTS.md` table gave **40 data rows** — main's replicate-count row
deletion (`0836163f`) removed one more row on top of the 41 measured in round
1. Lowered `knownApproximationRows` to 40 and updated the comment to name
main's replicate-count-bound closure. No row added or grown.

### Other paths

`AGENTS.md` and `rules/cast.go` auto-merged; `rules/replicate_count_bound_test.go`
came in as a new file. Everything else auto-merged without conflict.

### Commands and outputs

- `git status --short --branch; git status` (before merge):
  `## wt/cli-20260922T225143Z-4b0bde0d`; clean, no rebase/merge in flight.
- `git merge main --no-edit`:
  `Auto-merging .ds4/report-mrg1.md / CONFLICT (content): Merge conflict in .ds4/report-mrg1.md`
  `Auto-merging AGENTS.md / Automatic merge failed; fix conflicts and then commit the result.`
  (`AGENTS.md`, `rules/cast.go` auto-merged; `rules/replicate_count_bound_test.go` added.)
- Row count over merged `AGENTS.md`: 40 data rows (python line count).
- `.cards` check: present, resolves to `/home/sadams/projects/gorge/.cards`.
- `go test -run 'TestKnownApproximations' ./internal/testutil/` — see below.
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|TestReplicateCountBound'` — see below.

### Issues

No new unfixed issue found in this round. The standing note from main's side is
absorbed: the `offerCastable` pool-only replicate-offer gate was closed by
`0836163f` itself (the remaining limitation is documented in its commit
message), so nothing is carried forward from it here.
