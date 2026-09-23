# Merge conflict resolution — mrg1

## Result

Integrated `main` into `wt/cli-20260922T225142Z-171edf8c` with `git merge main`.
The merge completed as a merge commit; the worktree is clean.

## Entry state

The worktree entered CLEAN at `10f64d27` (`Merge branch 'main' into
wt/cli-20260922T225142Z-171edf8c`), with no in-flight rebase or merge. The
daemon's reported rebase failure had already been aborted (reflog: `rebase
(start): checkout main` at `e142295d` -> `rebase (abort): returning to
refs/heads/wt/cli-...` -> `reset: moving to HEAD`), and its merge fallback had
landed as `10f64d27` (parents `976ae1ee` + `08a1d59a`). But `main` had since
advanced to `e142295d`, so `main` was NOT an ancestor of HEAD. The integration
was therefore completed against the current `main` tip with `git merge main`.

## Conflicts

`git merge main` reported two content conflicts; `AGENTS.md` auto-merged.

### `internal/testutil/agentsdoc_test.go`

Both sides set the `knownApproximationRows` ratchet constant, with different
values and comments.

- HEAD (our merge commit `10f64d27`) comment: 38, attributing main's
  First-Strike/layer-4/mulligan/non<X>/NameCard/battle-protector/cascade/
  token-replacement closures plus this branch's maxpower1 closure and bestow1
  deletion; constant `38`.
- main (`e142295d`) comment: this branch's attackprop1 closure plus main's
  maxpower1 closure on top of the cascade/token-replacement/replicate closures;
  constant `37`.

Both comments were stale. MEASURED the auto-merged `AGENTS.md` (staged by the
merge, not hand-edited) with the same rule the test helper uses (`| ` lines
inside the `## Known approximations` section, header row dropped):

| tree | data rows |
|---|---|
| merge base `08a1d59a` | 38 |
| branch HEAD `10f64d27` | 37 |
| main `e142295d` | 37 |
| merged worktree `AGENTS.md` | **36** |

Row-level diff confirms the two deletions are disjoint: the branch deleted
`(bestow1)` (Three exotic bestow costs withheld) and main deleted
`(attackprop1)` (The priced attack prop is mana-only ...), both present in the
merge base. Disjoint deletions compose, so the merged table is `38 - 2 = 36`.

Resolution: set `knownApproximationRows = 36` with a comment recording the
measurement and the two disjoint deletions. `knownOversizeRows` was untouched
by both sides and stays `8`.

### `.ds4/report-mrg1.md`

main's copy at this path was the sibling worktree
`wt/cli-20260922T225142Z-462eca2e`'s integration report (a 185-line multi-round
history that landed on main), not a contradiction of this worktree's report.
Kept THIS worktree's report lineage and recorded this round (this file).

## Commands run

```text
git status                       -> clean, branch wt/cli-20260922T225142Z-171edf8c
git log --oneline main -5        -> tip e142295d
git merge-base --is-ancestor main HEAD -> NO (main not integrated)
git merge main
  Auto-merging .ds4/report-mrg1.md
  CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
  Auto-merging AGENTS.md
  Auto-merging internal/testutil/agentsdoc_test.go
  CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
  Automatic merge failed; fix conflicts and then commit the result.

# measurement (awk, header dropped), per tree:
#   base 08a1d59a = 38 · HEAD 10f64d27 = 37 · main e142295d = 37 · merged = 36
# deletion diff vs base: branch removed (bestow1); main removed (attackprop1)
```

## Verification

`.cards` present (symlink to the shared corpus), so the rules run was not
vacuous. One targeted ratchet pass over the conflicted packages:

```text
go test ./internal/testutil -run 'TestKnownApproximation' -count=1
ok  github.com/adams-shaun/gorge/internal/testutil  ...

go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead' -count=1
ok  github.com/adams-shaun/gorge/rules  ...
```

No engine behaviour changed by this resolution: the only hand-edited lines are
the ratchet constant/comment and this report. The branch's reviewed bestow fix
(`rules/bestow.go` and friends) and main's attack-prop work compose without a
code conflict.

## Issues

No new unfixed defect found. No uncertainty remains about the ratchet value:
36 is the measured data-row count of the merged `AGENTS.md`, and neither
conflicted comment matched it.
