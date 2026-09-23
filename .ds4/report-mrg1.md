# Merge-conflict resolution — task cli-20260922T225143Z-4b0bde0d

## Entry state

The worktree entered CLEAN with no rebase or merge in flight. The daemon's
reported rebase (`rebase onto main conflicted` on `b5f51b7d fix(rules): resume
transactional life exchanges`) had already been aborted, and its merge fallback
had landed as `375ff403` ("Merge branch 'main' into
wt/cli-20260922T225143Z-4b0bde0d", parents `bbd973b4` + `08a1d59a`). That merge
integrated main as of `08a1d59a`, but `main` had since advanced to `835074e5`,
so main was NOT an ancestor of HEAD and the integration still needed completing.

This is the same shape the two prior rounds of this branch hit: each daemon
rebase attempt is aborted, the merge fallback lands, and main moves again before
the next pass. I completed the integration against the current main tip with
`git merge main` — the branch already carries merge commits, so a merge is the
right operation (a rebase would rewrite the reviewed fix's history).

## Conflicts and resolutions

`git merge main` reported two content conflicts; `AGENTS.md` and every code path
auto-merged.

### `internal/testutil/agentsdoc_test.go`

Both sides set the `knownApproximationRows` ratchet constant with different
values and stale comments.

- HEAD (`375ff403`) comment: 37, attributing the branch's transactional
  life-exchange closure plus main's cascade1/maxpower1 and earlier closures;
  constant `37`.
- main (`835074e5`) comment: 36, attributing the branch's attackprop1 closure
  and bestow1 deletions; constant `36`.

Both comments were stale once the tables composed. MEASURED the auto-merged
`AGENTS.md` (staged by the merge, not hand-edited) with the same rule the test
helper uses (`| ` lines inside the `## Known approximations` section, header row
dropped):

| tree | data rows |
|---|---|
| merge base `08a1d59a` | 38 |
| branch HEAD `375ff403` | 37 |
| main `835074e5` | 36 |
| merged worktree `AGENTS.md` | **35** |

Row-level `diff` confirms the three deletions are disjoint and all present in
the merge base: the branch deleted `api:ExchangeLifeVariant` (transactional
life-exchange, this ticket's closure); main deleted `(attackprop1)` ("The priced
attack prop is mana-only ...") and `(bestow1)` ("Three exotic bestow costs are
withheld and unoffered ..."). Disjoint deletions compose, so the merged table is
`38 - 3 = 35`.

Resolution: set `knownApproximationRows = 35` with a comment recording the
measurement and the disjoint deletions. `knownOversizeRows` was untouched by
both sides and stays `8`; the merged table's oversize-row count is 5, below the
cap. No row was added or grown.

### `.ds4/report-mrg1.md`

main's copy at this path was a sibling worktree's integration report (a
multi-round history that landed on main), not a contradiction of this
worktree's report. Kept THIS worktree's report lineage and replaced the
conflict with this round's report (this file).

## Auto-merged paths retained

`AGENTS.md`, `effects/count.go`, `effects/filter.go`, `effects/your_starting_life_test.go`,
`rules/attack_cost.go`, `rules/attackprop_altselect_test.go`,
`rules/attackprop_window_test.go`, `rules/bestow.go`, `rules/bestow_exotic_test.go`,
`rules/bestow_test.go`, `rules/cast.go`, `rules/count_head_ratchet_test.go`,
`rules/layers.go`, `rules/legal.go`, `state/object.go`, plus main's new
count-head work.

## Commands and output

```text
git status                       -> clean, branch wt/cli-20260922T225143Z-4b0bde0d
git log --oneline main -5        -> tip 835074e5
git merge-base --is-ancestor main HEAD -> NO (main not integrated)
git merge main
  Auto-merging .ds4/report-mrg1.md
  CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
  Auto-merging AGENTS.md
  Auto-merging internal/testutil/agentsdoc_test.go
  CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
  Automatic merge failed; fix conflicts and then commit the result.

# measurement (same walk as the test helper, header dropped), per tree:
#   base 08a1d59a = 38 · HEAD 375ff403 = 37 · main 835074e5 = 36 · merged = 35
# deletion diff vs base: branch removed ExchangeLifeVariant(row 39);
#                        main removed (attackprop1) and (bestow1) rows
# oversize-cell count of merged AGENTS.md = 5 (cap is knownOversizeRows = 8)
```

## Verification

`.cards` is present (symlink to the shared corpus `/home/sadams/projects/gorge/.cards`),
so the rules run below was not vacuous.

One targeted ratchet pass over the conflicted packages (the merge's own gate
suite runs afterward at the daemon):

```text
$ go test ./internal/testutil -run 'TestKnownApproximation' -count=1
ok  	github.com/adams-shaun/gorge/internal/testutil	0.001s

$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead' -count=1 -v
--- PASS: TestEveryRepoDeckIsFullySupported (0.59s)
--- PASS: TestEveryRepoDeckCountHeadResolves (0.00s)
--- PASS: TestEveryDispatchedTriggerModeHasAMatcher (0.00s)
--- PASS: TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched (0.00s)
--- PASS: TestEveryRepoDeckParamsAreRead (0.13s)
PASS
ok  	github.com/adams-shaun/gorge/rules	0.753s

# branch fix still green after the merge:
$ go test ./rules -run 'TestExchangeLife|ExchangeLifeVariant' -count=1
ok  	github.com/adams-shaun/gorge/rules	0.827s
```

The corpus-backed assertions ran for real (0.59s / 0.13s, not the ~0s a
skipped corpus test reports). `gofmt -l internal/testutil/agentsdoc_test.go`
produced no output. No golden (`heads_test.go`) was touched.

## Issues

No new unfixed defect found. No uncertainty remains about the ratchet value: 35
is the measured data-row count of the merged `AGENTS.md`, and neither conflicted
comment matched it.
