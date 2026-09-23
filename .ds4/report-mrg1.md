# Merge conflict resolution report — cli-20260922T225141Z-462eca2e

## Result

Two integration rounds are recorded here.

**Round 1 (commit `aa69892e`, merged main `edc24484`).** On entry the tree was clean — the
daemon's rebase attempt (conflict on applying `89c77778`) and its merge fallback had both been
rolled back. Rebase is forbidden to a seat, so integrated with `git merge main` (same shape other
branches used, e.g. `5c5370ff`). One content conflict: `internal/testutil/agentsdoc_test.go`'s
`knownApproximationRows` constant (HEAD 43, main 42). Measured rather than trusted either comment:
merge base `2040e5d9` = 44 rows, branch = 43 (attackprop1 deletion, `89c77778`), main = 42 (Phase$
First Strike Damage + mulligan-REDRAW deletions) — disjoint, merged **41**. Everything else
auto-merged. Result: merge commit `aa69892e`, parents `1c3df172` + `edc24484`.

**Round 2 (this commit).** Main moved on after round 1 (11 newer commits, tip `4a7bb2fe`), so the
integration was completed against the current tip with `git merge main`. Conflicts: two.

- `internal/testutil/agentsdoc_test.go` — the same ratchet constant again. HEAD (round-1 merge)
  said 41, main said 40. Measured the merged `AGENTS.md` (auto-merged, staged) with the same awk
  counter the test uses (`^\| ` lines inside the section, header dropped): **39 test rows** —
  merge base (round-1 merge, `aa69892e`) = 41, branch = 41, main = 40 (main's token-replacement
  closure `bc3f03ab` deletes `(tokrepl1)` and replicate-count-bound closure `0836163f` deletes the
  Replicate row; row-level diff against HEAD confirmed exactly those two deletions and that main
  still carries `(attackprop1)`, which this branch deletes). The deletions are disjoint, so
  resolved to `knownApproximationRows = 39` with a comment recording the measurement.
  `knownOversizeRows` untouched by both sides.
- `.ds4/report-mrg1.md` — main's copy was the sibling worktree
  `wt/cli-20260922T225141Z-22391c4d`'s integration report (landed on main via `633a30fb`), not a
  contradiction of this branch's report. Kept this worktree's report lineage and rewrote it to
  cover both rounds (this file).

`AGENTS.md` auto-merged keeping every deletion; verified the merged table contains none of the
three deleted rows (`attackprop1`, `tokrepl1`, Replicate).

## Conflict resolution commands

```text
git status            → clean at start, branch wt/cli-20260922T225141Z-462eca2e, HEAD aa69892e
git log aa69892e..main --oneline → 11 newer main commits (tip 4a7bb2fe)
git merge main --no-edit
  Auto-merging .ds4/report-mrg1.md
  CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
  Auto-merging AGENTS.md
  Auto-merging internal/testutil/agentsdoc_test.go
  CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
  Automatic merge failed; fix conflicts and then commit the result.
# row measurements (awk between '## Known approximations' and the next '## '):
#   HEAD: 42 table lines / 41 test rows · main: 41 / 40 · merged worktree: 40 / 39
git add internal/testutil/agentsdoc_test.go .ds4/report-mrg1.md
GIT_EDITOR=true git merge --continue
```

## Verification

`.cards` present (symlink), no skips. Post-merge ratchets, `-count=1`:

```text
go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead' -count=1
ok  github.com/adams-shaun/gorge/rules  0.842s   (exit=0)

go test ./internal/testutil -run 'TestKnownApproximation' -v
--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)
--- PASS: TestKnownApproximationRowsAreShort (0.00s)
ok  github.com/adams-shaun/gorge/internal/testutil  0.001s

go test ./internal/archtest/
ok  github.com/adams-shaun/gorge/internal/archtest  4.418s

go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  github.com/adams-shaun/gorge/cmd/botbench  0.994s
```

(final `git status`: working tree clean.)

## Issues

None new. The merge introduced no engine change of its own; the branch's attack-prop fix
(`89c77778`, `1c3df172`) and main's closures compose cleanly. The conflicted ratchet constant was
measured, not guessed.
