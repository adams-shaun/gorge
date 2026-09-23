# Merge resolution report — task cli-20260923T060000Z-equip-reduce

## Entry state

The worktree entered CLEAN with no rebase or merge in flight. The daemon's
reported rebase (`rebase onto main conflicted` on `62c39530 fix(rules): bound
targeted equip menu by payable mana window`) had already been aborted: the
branch still sat on its original base `62ae4746` with all four of its commits
(`a1e46ad1`, `14ce2fbf`, `62c39530`, `559701e2`) unrewritten. Main had advanced
to `13f75557` (the cantsac1 r2 merge, plus ChooseType enumeration, mana-spent
activation riders, granted Goad statics, kw_prevent, ChooseType roots, battle
attack tests, and more — ~90 paths).

Integration was completed with `git merge main` (the branch already carries
merge commits; a rebase would rewrite the reviewed fix's history and is
forbidden to this seat).

## Conflicts and resolutions

`git merge main` reported ONE content conflict: `rules/cast.go` (two hunks).
Everything else auto-merged.

### `rules/cast.go` hunk 1 — `finishTargetedCast`

- **Branch** had collapsed the function to `e.payCast()` — its fix moved an
  ability's root-target recording OUT of this tail and INTO `payCast`'s
  ability arm, because a target answer can now suspend payment in the
  601.2g mana window; a recording done in `finishTargetedCast` misses a
  proposal whose stack object is minted only on the resumed `payCast` pass.
- **Main** had kept the base's `isAbility()` branch (with the old
  `recordChosenTargets` here) and ADDED the deferred mana-spent rider
  dispatch: after `payCast`, re-arm `e.cast = pc` and fire
  `fireManaSpentTriggers(AbilityPush)` so a `TriggersWhenSpent$` rider on an
  activation matches with the completed target bindings available, then
  close again (`82d3ba68`/`5cfb27c5`/`d506db1c`, mana-spent riders on
  ability activations).

**Resolution — both intents, composed.** Kept the branch's `e.payCast()`
(the target recording now lives inside `payCast`, per its own comment), and
kept main's deferred dispatch under `pc.isAbility() && pc.stackObj != 0`
(main's guard; it also keeps the dispatch off a proposal still parked in the
mana window, where `stackObj` is 0). The shared comment records both halves.

### `rules/cast.go` hunk 2 — `payCast`'s ability arm tail

- **Branch** added, after the LKI capture: record `pc.rootOpts` targets once
  `pc.stackObj != 0` — the immediate pay path OR a resumed `payCast`.
- **Main** added: when `pc.rootOpts == nil` (no target-recording
  continuation), dispatch `fireManaSpentTriggers` right at the completed
  `AbilityPush` boundary while the spent-source capture is still live.

**Resolution — both, complementary:** the branch's `recordChosenTargets`
guard (`stackObj != 0 && rootOpts != nil`) followed by main's
`rootOpts == nil` immediate dispatch. They are mutually exclusive on
`rootOpts`, so exactly one (or neither) runs; no double dispatch. A
target-answer + mana-window proposal still fires nothing on the resumed
path — exactly main's pre-existing behaviour (its deferred site needs
`stackObj != 0`, which is false at `finishTargetedCast` time in that case);
the merge introduces no new fire or miss relative to either side.

## Post-merge ratchet fix (part of the integration, per the brief)

Main's paramcensus rot guard newly scanned the branch's code and failed with
9 findings — the branch introduced `bodyReadsRef`/`bodyReadsRootTarget` (the
shared transitive SVar walk behind `bodyReadsAllTargeted`, commit `a1e46ad1`)
and reads `source.original.Params["Produced"]` in the equip window probe
(`559701e2`). Classified them in `rules/paramcensus_test.go`:

- `stringMapParams`: added `"rules:bodyReadsRef:svars"` (SVar-table walk by
  SVar name, not a card Params map). Whitelisting the callee clears the
  direct dynamic-key finding (`svars[w]` at cast.go:6929) AND the seven
  caller-attribution findings, since callers forwarding `svars` are no
  longer attributed through a whitelisted map. Main's existing
  `"rules:bodyReadsAllTargeted:svars"` entry stays (the branch's refactor
  made it inert, but there is no staleness check and deleting it is not
  required).
- `baseBuckets`: added `"source.original": bSA` — `attackManaSource.original`
  is a `*cards.SA` (the compiled pile ability), the same classification as
  the existing bare `"original"` entry.

## Commands and output

```text
git status                                   -> clean, no rebase/merge in flight
git merge-base HEAD main                     -> 62ae4746 (main NOT integrated)
git merge main
  Auto-merging rules/cast.go
  CONFLICT (content): Merge conflict in rules/cast.go
<resolve two hunks in rules/cast.go>
gofmt -l rules/cast.go                       -> clean
go build ./rules/                            -> ok
go test -run 'TestBeltOfGiantStrength|TestSunkenPalaceManaSpent|TestRestrictedManaKeepsSpentActivationRider' ./rules/
  ok  github.com/adams-shaun/gorge/rules  0.670s     (branch equip-window tests + main mana-spent activation tests)
git add rules/cast.go
go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
  FAIL  (paramcensus rot guard: 9 findings — see above)
<classify in rules/paramcensus_test.go>
go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
  ok  github.com/adams-shaun/gorge/rules  0.815s
go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
  ok  github.com/adams-shaun/gorge/cmd/botbench  1.242s    (bot win-split golden unmoved)
go test ./internal/archtest/
  ok  github.com/adams-shaun/gorge/internal/archtest  5.679s
git merge --continue
```

`.cards` was present (symlink to the shared corpus) for every run above — no
vacuous skips.

## Unsure about

- The `.ds4/report-mrg1.md` file is TRACKED on both branches (main carries
  prior rounds' merge reports at this path), so this report replaces the
  auto-merged prior content, matching the established convention.
- The mana-window + targets + `TriggersWhenSpent$` combination (a rider on a
  targeted activation that needed the mana window) dispatches on neither
  path — inherited from main unchanged, not introduced here. Worth a ticket
  if a corpus carrier exists.

## Issues

- None new. The branch's commit messages already record the conservative
  window-probe remainders (exotic paid/variable mana productions unpriced).
