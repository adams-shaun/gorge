# Report — fb-20260923T050453Z-49840c2d (Fury divided damage) — seat tm2

## Outcome

`STATUS=DONE`. The brief's feature is implemented and verified on top of current
`main`. This round's dispatch was a **mechanical** one: `findings-tm2.md` is not
a review finding, it is the controller's failed rebase/merge fallback
(`error: cannot rebase: You have unstaged changes`), caused by two peer seats'
report files being dirty in the shared worktree. Nothing about the feature code
was in question.

What I did:

1. Read the brief, the prior fix-round report (`.ds4/report-t2.md`) and
   `findings-t2.md`. The prior fix round (`ad15815ed`) had already answered both
   of that review's findings.
2. Committed the two dirty report artifacts so the tree was clean (commit
   `ee4149d03` — the exact block the controller hit was
   `error: Your local changes to the following files would be overwritten by
   merge: .ds4/report-t1.md .ds4/report-t2.md`), then merged `main` (commit
   `3a93a602e`). The only merge conflicts were those same report artifacts;
   **every code file merged cleanly** (`git diff --name-only --diff-filter=U`
   listed only the two `.ds4/report-*.md` paths).
3. Re-ran the brief's gates on the merged HEAD and re-proved the fail-without-fix
   procedure (below).

Branch `wt/fb-20260923T050453Z-49840c2d` now contains `main` (`git merge-base
--is-ancestor main HEAD` → true).

## What the feature does (per file)

The feature landed in the prior round's commits, which I did not modify:

- **`effects/damage.go`** — `effDealDamage`'s `divided` branch builds the
  chosen-target list (`divTargets`, in `Defined$` order). When there are **more
  than one** target and `total > 0` it poses a `KChoose` with one option per
  target, `Min == Max == total`, `Repeatable: true`, `ResumeKind: "damage_split"`,
  `ResumeSA: sa`, before `BeginDamageBatch` (so a suspension leaves nothing
  half-emitted). The `len(divTargets) > 1` gate is the fix for the prior
  review's MAJOR (a single target has exactly one legal answer, so the ask is
  filled directly: `c.DamageSplit = []int32{total}`). The emission walk reads
  `c.DamageSplit[i]` positionally and the split is reset after the walk
  (`defer` inside the `divided` arm) so a second divided `DealDamage` in one
  resolution re-asks rather than reusing the first shares — the prior review's
  MINOR. R-9 no-host keeps `roundRobinSplit`.
- **`effects/registry.go`** — `Ctx.DamageSplit []int32` / `DamageSplitDone bool`.
- **`rules/resolution.go`** — the `"damage_split"` resume arm turns the answered
  option multiset into `ctx.DamageSplit` by `Option.Index` (the target's
  `Defined$` position).
- **`rules/fury_damage_split_test.go`** (new) — the tests: player-chosen 3/1
  split (differs from round-robin 2/2), zero-share, no-targets, single-target
  no-ask.
- **`rules/replacement_updated_test.go`** — the shared `passUntilStackEmpty`
  drain gained a `damage_split` arm that reproduces the old round-robin split,
  so pre-existing tests written around the stand-in keep their board.
- **`rules/rakdos_muscle_deck_test.go`** — the old
  `TestFuryDividesDamageAmongTargets` (which pinned the round-robin stand-in) was
  deleted; the same test name now lives in the new file with the stronger
  player-choice oracle, exactly as the brief asks.

## Gates (Done means), with real output

`.cards` was **present as a symlink** (`ls -la .cards` →
`… -> /home/sadams/projects/gorge/.cards`), so the corpus-backed tests ran rather
than skipped.

Exact targeted test from the brief:

```
$ go test -run 'TestFuryDividesDamageAmongTargets' ./rules/ 2>&1 | tail -30
ok  	github.com/adams-shaun/gorge/rules	(cached)
```

The first (uncached) run of the same command, captured before the merge:

```
$ go test -run 'TestFuryDividesDamageAmongTargets' ./rules/ > .ds4/scratch/fury.log 2>&1; echo $?
0
$ tail -1 .ds4/scratch/fury.log
ok  	github.com/adams-shaun/gorge/rules	0.419s
```

Full Fury test set (all four, verbose — proves the corpus ran, not a vacuous
0.00s skip):

```
$ go test -run 'TestFury' ./rules/ -v
--- PASS: TestFuryDividesDamageAmongTargets (0.41s)
--- PASS: TestFuryDamageSplitAllowsZeroShare (0.00s)
--- PASS: TestFuryDamageSplitSkipsAskWithNoTargets (0.00s)
--- PASS: TestFuryDamageSplitSingleTargetFillsWholeTotal (0.00s)
ok  	github.com/adams-shaun/gorge/rules	0.422s
```

Build and format:

```
$ go build ./...            # exit 0
$ gofmt -l effects/damage.go effects/registry.go rules/resolution.go rules/fury_damage_split_test.go rules/replacement_updated_test.go
                            # (empty)
$ go run ./cmd/gentypes -check   # exit 0
```

`effects/` package (I edited it, so one package run):

```
$ go test ./effects/
ok  	github.com/adams-shaun/gorge/effects	16.177s
```

The two required behaviour goldens (gorge-context.md):

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	4.689s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	0.657s
```

The botbench split did **not** move, so no re-pin was needed and no attribution is
owed. No Known-approximations row mentions `DividedAsYouChoose`/divided damage
(`grep -niE 'dividedas|divide|allocation' AGENTS.md` returns only unrelated
"round-robin across 2/4/6/8 seats" and a ports text), so nothing was deleted and
`knownApproximationRows` is correctly unchanged. The change does not touch any
trigger-mode or ratchet registry, so none of those ratchets are implicated.

## Fails without the fix

I re-ran the reviewer's discipline on the merged tree.

**(1) Player-choice test fails with the allocation ask reverted.** I changed
`if Ask(h, d) == AskAsked {` to `if false && Ask(h, d) == AskAsked {` in
`effects/damage.go`, forcing the old round-robin path:

```
$ go test -run 'TestFuryDividesDamageAmongTargets' ./rules/
--- FAIL: TestFuryDividesDamageAmongTargets (0.41s)
    fury_damage_split_test.go:101: allocation ask missing after target selection: &{Seq:156 Player:1 Kind:attackers …}
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.427s
```

Then restored byte-identically: `cp .ds4/scratch/damage.go.bak effects/damage.go;
cmp effects/damage.go .ds4/scratch/damage.go.bak` → clean, `git status --short
effects/damage.go` empty.

**(2) Single-target test fails with the `> 1` gate reverted to `> 0`.** I changed
`if len(divTargets) > 1 && total > 0 {` back to `len(divTargets) > 0`:

```
$ go test -run 'TestFuryDamageSplitSingleTargetFillsWholeTotal' ./rules/
--- FAIL: TestFuryDamageSplitSingleTargetFillsWholeTotal (0.41s)
    fury_damage_split_test.go:246: unexpected mid-resolution ask while draining: &{Seq:54 Player:0 Kind:choose Prompt:Assign 4 damage Min:4 Max:4 … Repeatable:true … ResumeKind:damage_split …}
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.429s
```

Restored byte-identically (`cmp` clean, 4 total).

Both reverted files were restored with `cp` + `cmp` (never `git stash`/`git
checkout <path>`); the final tree is clean.

## Precondition assertions in the new tests

The brief's "each new test asserts its own precondition" is satisfied by
`furySetup`/`furyAllocationAsk` and the single-target test:

- both bears are asserted on `state.ZBattlefield` (the zone `effDealDamage`
  reads) and with toughness `> 4` so a full 4-share is not lethal;
- Fury is asserted on the battlefield (its ETB fired) and the target ask is
  asserted `Min 0 / Max 4` with both bears actually offered;
- the allocation ask is asserted `KChoose`, `ResumeKind=="damage_split"`,
  `Min==Max==4`, `Repeatable`, exactly two options;
- the two share options are asserted distinguishable (`o1 != o2`) before the 3/1
  vs 2/2 comparison, so a collapsed option list cannot make the test vacuous;
- the answer's own wire validation is exercised (`Validate` rejects a 3-total and
  an out-of-range recipient), so the ask is proven to be a real constraint;
- the zero-target test also asserts `hasNote(e, "unimplemented API DealDamage")`
  is false, so it fails if the `DealDamage` registration is removed.

## Findings from findings-tm2.md

`findings-tm2.md` contains no MAJOR/MINOR review item — it is the controller's
mechanical message:

```
rebase onto main failed:
error: cannot rebase: You have unstaged changes.
--- merge fallback ---
error: Your local changes to the following files would be overwritten by merge:
	.ds4/report-t1.md
	.ds4/report-t2.md
```

Both files were peer-seat report artifacts, now committed (`ee4149d03`) so the
tree is clean; the merge then succeeded (`3a93a602e`) with zero code conflicts.
Nothing else in that file is owed.

The prior review's `findings-t2.md` (MAJOR: single-target forced ask; MINOR:
`DamageSplitDone` not consumed) was answered by the prior round and I re-verified
both behaviours: the single-target test asserts **no** `damage_split` ask is
posed, and `effects/damage.go` resets `DamageSplit`/`DamageSplitDone` after the
emission walk.

## Issues (found, not fixed)

None new this round. Two items were named by the prior review and are now
closed in code:

- single-target forced ask — closed (`len(divTargets) > 1` gate +
  `TestFuryDamageSplitSingleTargetFillsWholeTotal`).
- latent `DamageSplitDone` non-consumption — closed (post-walk reset).

One optional follow-up the prior review mentioned but did not require: a CR-lane
assertion citing CR 601.2d (divided damage is announced as part of casting)
would pin the allocation ask so it stops being invisible to `make ledger`. I did
not write it because the brief does not ask for a CR-lane test; naming it here
per the report contract. No `new-tickets` entry is filed — there is no unfixed
defect to hand over.

## Deviations from the brief

None substantive. Mechanically I could not use `git rebase` (forbidden in this
seat's rules), so I used the controller's own fallback: commit the blocking
artifacts, then `git merge main`. The branch is now a merge of `main` rather than
a rebase; the tree content is equivalent and the code merged conflict-free.

## Open concerns

- The feature's `KChoose` asks a player to click the option for each point of
  damage (Min == Max == total, Repeatable). For a large-X divided carrier this
  is an N-click pick; the bot policy's first-option/default fill keeps it
  livelock-free, and the prior review confirmed the round-robin-compatible
  `Clamp`. Not changed here; out of scope.
