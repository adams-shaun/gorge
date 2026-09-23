# Merge conflict resolution report — cli-20260922T225141Z-462eca2e

## Conflict: `internal/testutil/agentsdoc_test.go`

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

## Round 3 (this commit)

Main moved again after round 2 (tip `4cdffbc1`: the sibling cascade branch
`wt/cli-20260922T225140Z-9e382c75` and the land-type-statics import landed). Integrated with
`git merge main`. Conflicts: two, the same pair as before.

- `internal/testutil/agentsdoc_test.go` — the ratchet constant once more. Both sides said 39,
  but for disjoint reasons: HEAD = 39 (round-2 merge, carrying this branch's `attackprop1`
  deletion plus main's `tokrepl1`/Replicate deletions), main = 39 (its own copy inherited the
  cascade branch's `cascade1` deletion and the token-replacement deletion, and still carries
  `attackprop1`). Merge base `4a7bb2fe` = 40 rows carrying BOTH `attackprop1` and `cascade1`
  and neither `tokrepl1` nor Replicate. The merged `AGENTS.md` (auto-merged) measured with the
  same awk counter the test uses: **38 test rows**, containing none of the four deleted rows.
  Disjoint deletions compose, so resolved to `knownApproximationRows = 38`.
- `.ds4/report-mrg1.md` — main's copy was the cascade worktree's integration report (landed on
  main via `4cdffbc1`), not a contradiction. Kept this worktree's lineage and appended this
  round-3 section.

`AGENTS.md` auto-merged; verified the merged table carries none of `attackprop1`, `cascade1`,
`tokrepl1`, Replicate, and still carries the untouched rows (`(rv2b)`, `(bestow1)` spot-checked).

## Round-3 commands

```text
git log 4a7bb2fe..main --oneline → 11 newer main commits (tip 4cdffbc1)
git merge main --no-edit
  Auto-merging .ds4/report-mrg1.md
  CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
  Auto-merging AGENTS.md
  Auto-merging internal/testutil/agentsdoc_test.go
  CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
# row measurements (awk between '## Known approximations' and next '## '):
#   merge base 4a7bb2fe: 40 rows (attackprop1 AND cascade1 both present)
#   merged worktree AGENTS.md: 39 table lines / 38 test rows, none of the four deleted markers
git add internal/testutil/agentsdoc_test.go .ds4/report-mrg1.md
GIT_EDITOR=true git merge --continue
```

## Round-3 verification

`.cards` present (symlink to the shared corpus), so no run was vacuous.

```text
go test ./internal/testutil -run 'TestKnownApproximation' -count=1
ok  github.com/adams-shaun/gorge/internal/testutil  0.001s

go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead' -count=1
ok  github.com/adams-shaun/gorge/rules  0.776s

go test ./internal/archtest/
ok  github.com/adams-shaun/gorge/internal/archtest  4.708s

go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  github.com/adams-shaun/gorge/cmd/botbench  1.591s
```

The botbench 20-game pin did NOT move despite main's engine changes (cascade closure, land-type
statics) — the golden holds byte-identically. Merge commit `88bcca38`, parents `5c3b38c3` +
`4cdffbc1`. Result: three integration rounds total, `knownApproximationRows` 44 → 41 → 39 → 38.

## Issues

None new in round 3 — integration only; the engine changes merged in from main are the cascade
branch's own reviewed work. See round-1/2 notes above for the earlier state.

## Round 4 — current main integration

The worktree was clean at entry; the earlier recorded integration was clean, but `main` had
advanced beyond that merge. Integrated the then-current `main` with `git merge main --no-edit`.
The merge reported content conflicts in this report and `internal/testutil/agentsdoc_test.go`;
`AGENTS.md` and the rules changes auto-merged.

- `internal/testutil/agentsdoc_test.go`: both versions set the ratchet to 38 but had different
  history comments. Kept the measured value 38 and updated the concise comment to include this
  branch's `(attackprop1)` closure and main's `(maxpower1)` closure; all earlier closures remain
  represented by the merged `AGENTS.md`.
- `.ds4/report-mrg1.md`: both reports described different merge histories. Kept this worktree's
  existing report and added this round's record rather than replacing its history.

`AGENTS.md` automatically merged the branch's attack-prop row deletion with main's newer
(maxpower1) deletion. No engine-code conflict required manual resolution.

## Round-4 commands and verification


The merged `AGENTS.md` table has 38 lines including the header, i.e. **37 data rows** as counted
by `approximationRows`; the conflicted constant had been 38 and was therefore lowered to 37.
The five closed markers `(attackprop1)`, `(maxpower1)`, `(cascade1)`, `(tokrepl1)` and the
Replicate row are absent. `.cards` was present (`cards.lock`, `cardsfolder`, IR files and token
scripts), so the rules checks did not corpus-skip.

```text
python3 row count → table rows including header: 38; data rows: 37
python3 marker checks → attackprop1 False; maxpower1 False; cascade1 False; tokrepl1 False; Replicate False
ls .cards | head -5 → cards.lock, cardsfolder, ir.gob.gz, ir.v4.gob.gz, tokenscripts

go test ./internal/testutil -run 'TestKnownApproximation' -count=1
ok   github.com/adams-shaun/gorge/internal/testutil  0.001s

go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok   github.com/adams-shaun/gorge/rules  0.774s
```

No uncertainty remains in the resolved count: 37 is the data-row count used by the test helper.
