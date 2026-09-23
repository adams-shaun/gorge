# Merge conflict resolution — static Goad integration

## Starting state and operation

Arrival was a clean tree on `wt/cli-20260922T225142Z-885d3d75` at `8f4f265cd`, with no merge/rebase in progress. That commit already integrated main through `62ae4746`; current `main` (`2341274c6` at merge start) was not an ancestor. The reported rebase had not left an operation in progress, so I completed integration using the merge fallback:

```text
git merge --no-edit main
Auto-merging .ds4/report-mrg1.md
CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
Auto-merging AGENTS.md
CONFLICT (content): Merge conflict in AGENTS.md
Auto-merging effects/filter.go
Auto-merging effects/registry.go
CONFLICT (content): Merge conflict in effects/registry.go
Auto-merging effects/trigger_referents.go
CONFLICT (content): Merge conflict in effects/trigger_referents.go
Auto-merging internal/testutil/agentsdoc_test.go
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
Auto-merging rules/paramcensus_test.go
Automatic merge failed; fix conflicts and then commit the result.
```

## Conflicts and resolutions

- `AGENTS.md`: branch retains its `ap1` row while main's side had the `staticgoad1` row. The reviewed branch fix closes staticgoad1, so that row remains deleted; main's other register changes are retained. The merged table was measured directly at **31 data rows**.
- `effects/registry.go`: combined branch's `StaticGoads` context field, `goadTableHost`/Goaded-parameter detection and publication with main's `TargetableObjects` field, host interface, and publication. Both values are refreshed at Resolve entry and at each SA boundary; absence clears the value to prevent stale-context leakage.
- `effects/trigger_referents.go`: `SpecContext` now carries both `StaticGoads` and `TargetableObjects`.
- `internal/testutil/agentsdoc_test.go`: set `knownApproximationRows = 31`, matching the merged AGENTS.md measured with the test's row-count rule, rather than using either stale side comment.
- `.ds4/report-mrg1.md`: this file is an accumulated report archive. Preserved both conflict sides verbatim after this round's report below; no prior archive content was discarded.

The non-conflicting changes, including `effects/filter.go` and `rules/paramcensus_test.go`, remain as auto-merged. No other files were intentionally changed.

## Checks

- `[ -e .cards ] && echo '.cards present'` → `.cards present` (corpus available; no corpus-backed false-green).
- Merged approximation table measurement → `approximation rows: 31`.
- `git diff --check` → no output (clean).
- `go test ./internal/testutil/ -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'` → `ok github.com/adams-shaun/gorge/internal/testutil 0.001s`.
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|TestEffectGrantedGoad|TestCloneGrantedGoad|TestStaticGoad'` → `ok github.com/adams-shaun/gorge/rules 1.076s`.
- `gofmt -w effects/registry.go effects/trigger_referents.go internal/testutil/agentsdoc_test.go`; subsequent `gofmt -l` returned no paths.

## Issues

No new unfixed issue identified during conflict resolution. No new trigger mode is registered by this change; no `addedAfterTheSplit` adjustment is indicated. No golden was modified.

## Completion

- `GIT_EDITOR=true git merge --continue` → `[wt/cli-20260922T225142Z-885d3d75 c85b1900] Merge branch 'main' into wt/cli-20260922T225142Z-885d3d75`.
- `git merge-base --is-ancestor main HEAD` → exit 0.
- `git status --short --branch` → `## wt/cli-20260922T225142Z-885d3d75` (clean).
- `git diff --check HEAD^ HEAD` → clean.

## Archived conflict-side report — branch version

# mrg1 conflict resolution report

## Conflict

Only `internal/testutil/agentsdoc_test.go` was conflicted. The branch side had `knownApproximationRows = 34` with a note that it was measured after rebase. Main's side said its auto-merged `AGENTS.md` had 32 rows, including several main-side closures, plus the branch's `(choosesource1)` deletion, and set the constant to 32. I retained the merged table count but measured the actual merged `AGENTS.md`: 31 approximation data rows. The conflict resolution therefore sets `knownApproximationRows = 31`; this retains main's table changes and the branch's row deletion while accurately matching the merged table.

The post-merge ratchet also exposed unclassified Goad static parameter-recognition reads in `rules/paramcensus_test.go`. I classified the helper's parsed SVar map as non-card Params and its `saMentionsGoaded` range as recognition rather than consumption, with comments. This was needed for the required post-merge ratchets to pass.

## Commands and output

- `git status --short --branch && git rev-parse --show-toplevel && git log -1 --oneline --decorate && git status`
  ```
  ## wt/cli-20260922T225142Z-885d3d75
  /home/sadams/projects/gorge/.worktrees/cli-20260922T225142Z-885d3d75
  e0f7658d3 (HEAD -> wt/cli-20260922T225142Z-885d3d75) fix(rules): preserve static goad in trigger LKI
  On branch wt/cli-20260922T225142Z-885d3d75
  nothing to commit, working tree clean
  ```
- `git merge --no-edit main`
  ```
  Auto-merging AGENTS.md
  Auto-merging effects/filter.go
  Auto-merging effects/misc.go
  Auto-merging effects/registry.go
  Auto-merging internal/testutil/agentsdoc_test.go
  CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
  Auto-merging rules/cast.go
  Auto-merging rules/engine.go
  Automatic merge failed; fix conflicts and then commit the result.
  ```
- `python3` count of `AGENTS.md` approximation rows: `approximation data rows: 31`.
- `ls .cards | head`: corpus present (`cards.lock`, `cardsfolder`, `ir.gob.gz`, `ir.v4.gob.gz`, `tokenscripts`).
- `go test ./internal/testutil/ -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'`
  ```
  ok   github.com/adams-shaun/gorge/internal/testutil  0.001s
  ```
- First required ratchet run, `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`, failed in `TestEveryRepoDeckParamsAreRead`: seven findings for the new Goad helpers' `params` map and `saMentionsGoaded` range. I added the corresponding explicit census classifications.
- Rerun of that same required ratchet command after classification:
  ```
  ok   github.com/adams-shaun/gorge/rules  0.808s
  ```

## Issues

No engine behavior defect found during this integration. The only issue uncovered was the param-census classification noted above; it is fixed here. No additional CR-lane test issue identified.

## Completion

The merged approximation count is 31, not either stale conflict-side count. No uncertainty remains. The merge commit includes the conflict resolution and required ratchet classification.


## Archived conflict-side report — main version

# Merge resolution report — cli-20260922T225143Z-bc326d39

---

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

Because `main` is a LIVE, moving target (other agent branches merge into it
concurrently), it advanced again during this pass: the first merge commit
(`71123e48`, against main `835074e5`) was immediately followed by a second
`git merge main` (`b5a6f07a`, against main `06f2a294`, the Corrupted
poison-readers merge), which auto-merged with NO conflicts (5 files:
`effects/context.go`, `effects/count.go`, `effects/filter.go`,
`effects/playercount_statebacked_test.go`, new `rules/corrupted_poison_readers_test.go`).
`main` is now an ancestor of HEAD.

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

The corpus-backed assertions ran for real (0.58s / 0.12s, not the ~0s a
skipped corpus test reports). `gofmt -l internal/testutil/agentsdoc_test.go`
produced no output. No golden (`heads_test.go`) was touched.

After the second merge (main `06f2a294`) the same ratchets were re-run against
the final tree:

```text
$ go test ./internal/testutil -run 'TestKnownApproximation' -count=1
ok  	github.com/adams-shaun/gorge/internal/testutil	0.002s

$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead' -count=1 -v
--- PASS: TestEveryRepoDeckIsFullySupported (0.58s)
--- PASS: TestEveryRepoDeckCountHeadResolves (0.00s)
--- PASS: TestEveryDispatchedTriggerModeHasAMatcher (0.00s)
--- PASS: TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched (0.00s)
--- PASS: TestEveryRepoDeckParamsAreRead (0.12s)
PASS
ok  	github.com/adams-shaun/gorge/rules	0.737s

# branch fix still green after both merges:
$ go test ./rules -run 'TestExchangeLife|ExchangeLifeVariant' -count=1
ok  	github.com/adams-shaun/gorge/rules	0.650s
```

The merged `AGENTS.md` still measures 35 data rows with constant 35 after the
second merge (main's Corrupted work added no Known-approximations row and
deleted none).

## Issues

No new unfixed defect found. No uncertainty remains about the ratchet value: 35
is the measured data-row count of the merged `AGENTS.md`, and neither conflicted
comment matched it.

---

# Preserved main-side report archive (carried from main at 122a388c)

# Merge-conflict resolution report — mrg1 (branch wt/cli-20260922T225142Z-1d4558a1, main at 0104252d)

## Preserved main-side report record

The following report record was carried from main's integration of
`cli-20260922T225140Z-c661fa12`; it documents that separate integration.

## Situation found

The dispatch described a failed rebase (1/1, conflict in
`internal/testutil/agentsdoc_test.go`) and a failed merge fallback on commit
`6e77a1e8` ("feat(rules): price the composite CantBlockUnless block charge and
admit delivered statics"). On arrival the worktree was CLEAN at `6e77a1e8` with
no rebase or merge in progress — the daemon had aborted the in-flight
operation. I therefore performed the integration myself:

```
git merge main --no-edit
```

which auto-merged everything except one file:

```
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
```

## The conflict and its resolution

`internal/testutil/agentsdoc_test.go`, the `knownApproximationRows` ratchet
constant:

- **Branch side (6e77a1e8):** `knownApproximationRows = 56`. The branch's fix
  deleted the `(blockprop1)` row (stat:CantBlockUnless) from AGENTS.md —
  measured: branch AGENTS.md table = 54 rows (base 55), branch constant 56
  (already 2 above its actual, which the test tolerates since it only fails on
  growth).
- **Main side (0104252d):** `knownApproximationRows = 50`. Main deleted six
  rows; measured main table = 49 rows, constant 50.

The merge auto-resolved AGENTS.md itself (both deletions are disjoint: 7 rows
deleted from 55 → **48 rows in the merged table**, verified with the exact
counting logic of `approximationRows()` — lines with prefix `| ` between
`## Known approximations` and the next `## `, header row dropped).

**Resolution:** set the constant to **48**, the merged table's actual count,
with a comment naming both sides' deletions. This preserves both sides' intent
(row deletions from the branch's reviewed fix AND main's closures); the
constant is lowered, which the register always permits, and it is more
accurate than either side (both sides' constants were above their actuals, so
this also removes the pre-existing slack).

`AGENTS.md` itself needed no manual edit — git's auto-merge correctly deleted
all 7 rows (branch's `(blockprop1)` plus main's six).

## Commands and output

- `git status` (arrival): clean at 6e77a1e8, nothing in flight.
- `git log --oneline HEAD ^main`: exactly `6e77a1e8` (the reviewed fix) — one
  branch-side commit to integrate.
- `git merge main --no-edit`: conflict in `internal/testutil/agentsdoc_test.go`
  only; everything else auto-merged (AGENTS.md, botpolicy/, decision/,
  rules/…).
- Row counts (test's own algorithm): merged AGENTS.md 48, branch 54, main 49,
  merge base 55.
- `git add internal/testutil/agentsdoc_test.go && git commit --no-edit` →
  merge commit `e3aa3074` ("Merge branch 'main' into
  wt/cli-20260922T225142Z-1d4558a1").
- `git status` after: clean.
- `.cards` was already the correct symlink to the main checkout's corpus
  (verified before any test run — no vacuous green).
- `go test ./internal/testutil/` → `ok github.com/adams-shaun/gorge/internal/testutil 1.826s`
  (the ratchet tests: `TestKnownApproximationsOnlyShrinks` and
  `TestKnownApproximationRowsAreShort` pass at 48/48 and oversize 8).
- Post-merge ratchets:
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  → `ok github.com/adams-shaun/gorge/rules 0.781s`
- Behaviour goldens:
  `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` →
  `ok github.com/adams-shaun/gorge/cmd/botbench 1.074s` (the 20-game pinned
  split did not move under the merged engine).
  `go test ./internal/archtest/` → `ok ... 4.011s`.
- Conflict-marker sweep over the resolved package and AGENTS.md: none.

## Uncertainties

None material. The only judgement call was the constant's exact value; 48
equals the merged table's measured count and is at-or-below both sides'
values, so no gate can fail on it.


---

# Earlier merge-resolver reports (prior rounds, preserved)

# Merge-conflict resolution — task cli-20260922T225140Z-6a16cd8c

## Main-side report record — task cli-20260922T225140Z-c661fa12

## Situation found

`git status` was CLEAN on entry (no rebase/merge in flight in this worktree;
the daemon's rebase attempt had been aborted), and the branch tip
`47b865e8` already merged main `e6a2a84d` (the PRIOR round's work, recorded in
the report this file carried). But main had advanced again to `e0fd7d9c`:

- `57cd1863`/`a04571b1`/`f76f59fd` — the battle-protector ticket (bot arm,
  re-derive, `botpolicy/protector_test.go`);
- `9aee8b00`/`78d3b764` — the NameCard ChooseFromList$/AtRandom$ closure;
- `d56e404f`/`e0fd7d9c` — the non<X> negation closure (nonColorless,
  nonChosenCard, nonCopiedSpell now expressed by the generic negation);
- plus its own main-merge commits (`23400548` etc).

So the owed integration was a merge of main `e0fd7d9c` into the branch.
Rebase is forbidden in a seat; the same `git merge main` method as prior
rounds was used.

## Conflicted files and resolutions

`git merge main` conflicted on exactly three files; all code auto-merged.

### 1. `AGENTS.md` (one hunk, lines 222-226)

- HEAD (branch) side: the `non<X>` negation row (still present on the branch,
  whose base merged main only up to `e6a2a84d` — before the non<X> closure).
- main side: the layer-4 row, which main still carries because the layer-4
  fix `63c07260` is THIS branch's reviewed commit, never merged to main.
- Both rows are CLOSED by commits on the two sides: layer-4 by this branch's
  fix, non<X> by main's `d56e404f`. **Resolution: delete BOTH rows** — which
  is exactly the union of the two sides' intents (each side deleted its own
  closed row; the merge should keep neither).

Verified against the measured tables: main's table is 45 rows (its
closures + its own added rows), HEAD's is 46; the merged table is main's 45
minus the layer-4 row = **44 rows** (`awk` over `## Known approximations` …
next `##`, `| ` lines minus header). Both deleted rows are absent from the
merged file; nothing else in the table changed relative to main
(`diff <(git show main:AGENTS.md | table-extract) <(table-extract AGENTS.md)`
shows exactly the one deleted row).

### 2. `internal/testutil/agentsdoc_test.go` (one hunk, comment only)

Both sides already agreed on the VALUE `knownApproximationRows = 46`; the
conflict was only in the history comment above it (main's side carried its
merge-round comment; the branch's side had a bare constant). **Resolution:**
kept main's commented shape, rewritten to describe THIS merge accurately,
and lowered the constant to the measured **44**:

    // The merged table measures 44 rows: main's closures merged here
    // (non<X> via d56e404f, NameCard ChooseFromList$/AtRandom$ via 78d3b764,
    // the battle protector row via f76f59fd, and the earlier pc1/each1/CR
    // 616.1 closures), plus this branch's own layer-4 filter-grammar closure
    // (fix 63c07260), which deletes main's last remaining layer-4 row.
    knownApproximationRows = 44

The test fails only on growth, but the constant should carry the measured
count (same method as main's precedent `4dcef2ea` and the prior round).

### 3. `.ds4/report-mrg1.md` (wholesale)

Both sides carry prior resolver rounds' reports (branch: the previous
integration round; main: another ticket's merge report). Neither is source
of truth. **Resolution: replaced wholesale with THIS round's report.**

## Auto-merged files — both sides' intent verified

- `effects/filter.go` — the branch's `SpecContext.DerivedTypes` layer-4
  plumbing (lines ~3193-3279) and main's non<X> generic-negation work
  (wordColorless/wordCopiedSpell, `nonCopiedSpell` comments at 1058/1377)
  compose; `go build ./effects/ ./rules/ ./internal/testutil/` clean.
- `rules/engine.go`, `rules/cast.go`, `rules/replacement.go` — main's
  battle-protector and NameCard work; no branch-side contradiction (the
  branch's fix touched layers/statics/clone/filter only).
- `botpolicy/policy.go` + `botpolicy/protector_test.go` — main's new files,
  added cleanly.
- `AGENTS.md`'s other hunks — main's row deletions (NameCard row,
  battle-protector row) applied by auto-merge; nothing reintroduced.

## Commands run and output

- `.cards` check: **present** (real symlink → `/home/sadams/projects/gorge/.cards`),
  so the corpus-backed ratchets were real, not vacuous skips.
- `git merge main` → 3 content conflicts (`AGENTS.md`,
  `internal/testutil/agentsdoc_test.go`, `.ds4/report-mrg1.md`), 17 files
  total in the merge.
- `go test ./internal/testutil/ -run 'TestKnownApproximations' -v`
  → `--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)` / `ok 0.001s`.
- Post-merge ratchets:
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|TestLayer4'`
  → `ok github.com/adams-shaun/gorge/rules 1.025s`.
- main's closures on the merged tree:
  `go test ./effects -run 'TestPlayerSpecFx20Grammar|TestUnknownPredicates|NameCard|NonPredicateStack'`
  → `ok github.com/adams-shaun/gorge/effects 0.008s`.
- main's bot-policy addition: `go test ./botpolicy ./internal/archtest/`
  → `ok ... botpolicy 0.719s` / `ok ... internal/archtest 3.846s`.
- `gofmt -l internal/testutil/agentsdoc_test.go` → clean.
- `go build ./effects/ ./rules/ ./internal/testutil/` → clean.

## Uncertainties / notes

- The dual-row deletion in `AGENTS.md` is the one judgment call: each side
  deleted a DIFFERENT row, so the merged table keeps neither. This is the
  union of the two reviewed intents, not a redesign.
- No engine behaviour beyond the branch's reviewed fix `63c07260` was
  introduced by the resolution itself; the resolution touched `AGENTS.md`,
  the register constant + its comment, and this report.
- `git diff --stat main` post-merge shows only the branch's own fix files
  plus the constant/report, confirming main's content came through intact.

## Issues

- `9fc45eaf` — Merge branch 'main' into wt/cli-20260922T225140Z-6a16cd8c
  (conflict resolution; both ft1 and fx20 rows deleted, constant 58→50
  across base→merged).
- This report commit (docs only).

## Unsure / notes

- The `.ds4/report-mrg1.md` file is tracked in git despite living under the
  excluded `.ds4/` path (prior docs commits record it), so main's version
  auto-merged into the merge commit and this report replaces it as a docs
  commit to leave the tree clean.
- I chose merge (matching the daemon's own fallback path) over rebase since
  no operation was in flight and rebase of 4 commits would re-conflict the
  same two hunks; the merge preserves both histories verbatim.

---

## Record 2 — main's side (68ca4d95 legend-rule ticket, via main commit `08955738`)

# Merge-conflict resolution report — mrg1

## Conflicted files

- `AGENTS.md` — the only unmerged path.

## Resolution

The initial `git status` showed a clean worktree and no active rebase/merge, so I completed integration with the main branch via `git merge main`. That merge conflicted in `AGENTS.md` at the adjacent Known approximations rows.

- The branch side contained the new main-side approximation for the `KReplacement` bot fallback. Main contained the obsolete CR 704.5j legend-rule row, which this reviewed branch closes by adding the controller choice.
- Kept main's `KReplacement` row and removed the superseded legend-rule row, preserving both current intents. No other conflict markers or unmerged paths remained.
- Completed the merge as `760578de` (`Merge branch 'main' into wt/cli-20260922T225140Z-68ca4d95`).

## Commands and output

- `git status --short --branch && git status`
  ```
  ## wt/cli-20260922T225140Z-68ca4d95
  On branch wt/cli-20260922T225140Z-68ca4d95
  nothing to commit, working tree clean
  ```
- `git merge main`
  ```
  Auto-merging AGENTS.md
  CONFLICT (content): Merge conflict in AGENTS.md
  Automatic merge failed; fix conflicts and then commit the result.
  ```
- `git add AGENTS.md && GIT_EDITOR=true git merge --continue`
  ```
  [wt/cli-20260922T225140Z-68ca4d95 760578de] Merge branch 'main' into wt/cli-20260922T225140Z-68ca4d95
  ```
- Corpus check: `.cards` was present.
- `go test -run 'TestLegendRule' ./rules/`
  ```
  ok  github.com/adams-shaun/gorge/rules  0.008s
  ```
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  ```
  ok  github.com/adams-shaun/gorge/rules  0.759s
  ```
- Final `git status --short --branch`
  ```
  ## wt/cli-20260922T225140Z-68ca4d95
  ```

No uncertainty remained in the conflict resolution. No non-conflict files were manually changed.

---

## Record 3 — round 2 integration (this seat, 2026-09-23)

### Starting state

`git status` found the tree CLEAN at `d3e21bfa`; no rebase or merge in
flight (the daemon's second integration attempt was fully aborted before
dispatch). The branch's merge-base with `main` was `c472a88f` — main had
moved past the branch's round-1 merge (`9fc45eaf`) with the 68ca4d95
legend-rule landing (`da0a323b` heads pin + `cf3e3784` merge). I redid the
integration as `git merge main`.

### Conflicted file: .ds4/report-mrg1.md (the only one)

- **branch side** (`d3e21bfa`): this ticket's round-1 resolution record.
- **main side** (`08955738`): the 68ca4d95 legend-rule ticket's own
  resolution record, committed to main.
- **Resolution:** keep BOTH records verbatim, headed, plus this record 3.
  `AGENTS.md` and `internal/testutil/agentsdoc_test.go` auto-merged this
  time: branch deleted the (fx20) row; main swapped the legend row for the
  `KReplacement` bot-fallback row (row count unchanged). Merged register =
  50 rows = the constant (verified below). `rules/heads_test.go` was
  auto-merged from main (2-seat head → `19a4893657e5d549`); the branch
  never touched it, so no head conflict arose and no golden was edited.

### Commands and output

- `.cards`: present (real symlink) — runs are not vacuous.
- `git merge main` → `CONFLICT (content): Merge conflict in .ds4/report-mrg1.md`
  (only unmerged path); resolved with both records kept, `git add -f`
  (`.ds4` is gitignored for untracked files), `git commit --no-edit` →
  `bdbdd7cb Merge branch 'main' into wt/cli-20260922T225140Z-6a16cd8c`.
- `git status --short` → clean.
- `grep -c 'fx20' AGENTS.md` → `0`; `grep -c 'KReplacement.*clamp fallback' AGENTS.md` → `1`.
- `go test ./internal/testutil/ -run 'TestKnownApproximations' -v` →
  `--- PASS: TestKnownApproximationsOnlyShrinks` / `ok`.
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  → `ok github.com/adams-shaun/gorge/rules 0.756s`; verbose log
  (`.ds4/scratch/ratchet.log`) shows all five RUN and PASS, **0 SKIPs**.
- Behaviour goldens: `go test ./internal/archtest/` → `ok 3.225s`;
  `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` →
  `ok 1.075s` — the pinned 20-game bot split did NOT move.

### Notes

- The branch registers no new `Mode$` matcher and closed no
  `knownUnsupported`/`knownUnsupportedParams`/`knownUnmodelledCountHeads`
  entry, so no ratchet table edit was needed; all pass untouched.
- Commits: `bdbdd7cb` (merge, conflict resolution) + this docs commit.


## Current dispatch confirmation (2026-09-23)

The dispatch additionally reported an earlier rebase conflict in `AGENTS.md` and `internal/testutil/agentsdoc_test.go`, followed by a merge-fallback conflict in `.ds4/report-mrg1.md` and `AGENTS.md`. Those attempts were not active when this resolver seat began: the branch already contained the completed merge `e3aa3074` and report commit `f3148515`. The actual merge resolution and conflicts are documented above; `AGENTS.md` auto-merged both sides' row deletions, and the only unmerged path in the completed merge was `internal/testutil/agentsdoc_test.go`. No unmerged paths remain.

Reconfirmed in this seat:

- `.cards` exists; corpus-dependent tests are not vacuous.
- Before this report update, `git status --short --branch` showed only `## wt/cli-20260922T225142Z-1d4558a1`.
- `git diff --check HEAD^ HEAD` produced no output.
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead' 2>&1 | tail -30` returned `ok github.com/adams-shaun/gorge/rules 0.756s`.

No code or conflict-resolution changes were needed in this confirmation seat.

The main-side report record's resolution noted: None found during that round.

---

## Current integration round — main at 2040e5d9

### Conflicts and resolution

The earlier merge commit `e3aa3074` integrated main at `0104252d`, but main
advanced to `2040e5d9` before this dispatch completed. I merged the current
`main` (rather than leaving the branch behind it). The merge auto-merged
engine and bot-policy changes, and conflicted in three tracked files:

- `AGENTS.md`: branch side had deleted the `(blockprop1)` row as part of the
  approved fix; main still carried that stale row. Kept the branch deletion.
  The adjacent `(each1)` row was preserved. This keeps the reviewed closure
  and main's other distinct row closures.
- `internal/testutil/agentsdoc_test.go`: reconciled the table ratchet to 44,
  confirmed by `TestKnownApproximationsOnlyShrinks` against the merged
  `AGENTS.md` (the initially attempted 43 failed and was corrected to the
  measured 44).
- `.ds4/report-mrg1.md`: preserved the branch's mrg1 history and the
  main-side `cli-20260922T225140Z-c661fa12` report as separate records, then
  appended this integration record. No report content was discarded.

### Commands and output

- `git status` before integrating current main: clean at `a8f650ed`.
- `git merge main --no-edit`:
  ```
  Auto-merging .ds4/report-mrg1.md
  CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
  Auto-merging AGENTS.md
  CONFLICT (content): Merge conflict in AGENTS.md
  Auto-merging botpolicy/policy.go
  Auto-merging internal/testutil/agentsdoc_test.go
  CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
  Auto-merging rules/statics.go
  Automatic merge failed; fix conflicts and then commit the result.
  ```
- Conflict-marker scan and `git diff --check`: no output after resolution.
- `.cards` check: present.
- `go test ./internal/testutil/ -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'`:
  `ok github.com/adams-shaun/gorge/internal/testutil 0.006s`.
- `go test ./rules/ -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`:
  `ok github.com/adams-shaun/gorge/rules 0.771s`.

No code behaviour was changed during resolution. The only uncertainty was the
ratchet count, resolved by the targeted test's direct measurement (44).

---

## Record 3 — main's side (48972afc mulligan-redraw ticket, via main commit `a5dbe7b1`)

# Merge-conflict resolution — task cli-20260922T225141Z-48972afc

## Situation found

`git status` was CLEAN on entry — no rebase or merge in flight (the daemon's
earlier attempt had left nothing behind). The branch tip was `7c4182ff`
("fix(rules): resolve mulligan redraws at the end of the declaration pass"),
one commit past the merge base `0104252d`; main had advanced to `2040e5d9`,
39 commits ahead. The owed integration was a merge of main into the branch
(rebase is forbidden in a seat).

`.cards` was **present** — a real symlink to
`/home/sadams/projects/gorge/.cards`, target exists — so every corpus-backed
ratchet below ran for real rather than skipping.

## Conflicted files and resolutions

`git merge main` produced **one** content conflict: `internal/testutil/agentsdoc_test.go`.
`AGENTS.md` auto-merged (it carried no textual conflict — each side had deleted
a different row).

### `internal/testutil/agentsdoc_test.go` (conflict: the `knownApproximationRows` constant)

Both sides changed the register constant:

- **HEAD (branch):** `knownApproximationRows = 49`. The branch base's table was
  50 rows; the mulligan ticket deleted one row and lowered 50 → 49.
- **main:** `knownApproximationRows = 44`, with a comment recording main's own
  closures (non<X> `d56e404f`, NameCard ChooseFromList$/AtRandom$ `78d3b764`,
  the battle protector row `f76f59fd`, plus pc1/each1/CR 616.1) and the
  layer-4 closure from **another** merge of this same branch (`63c07260`),
  which main had already absorbed.

Neither number is the merged count. The merged table was **measured** (same
method the test uses: lines starting `"| "` inside `## Known approximations …
next "## "`, minus the header): **43 data rows**. HEAD carried 48, main 44,
and the union of both sides' deletions (mulligan row on the branch; main's
four) gives 43. Verified the four closed rows are absent from the merged
`AGENTS.md`: the old mulligan row (`mulligan declaration's REDRAW`), the
`non<X> negation` row, the `layer-4 type grants reach only` row and the
`NameCard asks over` row — all grep to 0 occurrences.

**Resolution:** kept main's commented shape (which explains the other
closures) and wrote the merged count accurately:

    // The merged table measures 43 rows: main's closures merged here
    // (non<X> via d56e404f, NameCard ChooseFromList$/AtRandom$ via 78d3b764,
    // the battle protector row via f76f59fd, and the earlier pc1/each1/CR
    // 616.1 closures), plus this branch's own mulligan-redraw deferral
    // (fix 7c4182ff), which deletes the "mulligan declaration's REDRAW
    // resolves immediately" row.
    knownApproximationRows = 43

This is the union of the two sides' intents, not a redesign. The row-count
gate passes (`TestKnownApproximationsOnlyShrinks`, `TestKnownApproximationRowsAreShort`
— the package test is green).

## Integration failures found after the merge — and why they are part of resolving it

The merge itself is a **semantic** integration conflict, not just a textual
one. main added a whole feature after this branch forked — the hypothetical
chance planner (`rules/chance.go`, `rules/chance_test.go`) — and three of its
tests observe a mulligan's REDRAW inside the mulligan submit, because on main
`handleMulligan` still resolves the redraw immediately. This branch's reviewed
fix deliberately *defers* the redraw to the declaration-pass boundary (CR
103.4/103.5, `resolveMulliganRedraws`), which is the entire purpose of the
ticket. Both sides kept, so main's three tests now observe the wrong moment.

The three failures, measured on the merged tree before the fix below:

    --- FAIL: TestHypotheticalPlannerControlsMulliganShuffle (0.00s)
        chance_test.go:223: mulligan shuffle=[9 2 11 6 5 12 10 3 4 7 1 8] want []
    --- FAIL: TestHypotheticalReplayAndCloneOwnChanceState (0.00s)
        chance_test.go:448: mulligan did not consume chance
    --- FAIL: TestHypotheticalSubmitFailurePoisonsOnlyThatBranch (0.00s)
        chance_test.go:504: error = <nil>

I confirmed the underlying behaviour is intact by instrumenting a scratch test:
after the mulliganing seat declares and the **rest of the pass keeps**, the
planner's Ordinal-1 callback fires with exactly the context main's test
expects (`hand=0 lib=12`) and produces the planned permutation. Only the
*when* moved (from the submit to the pass boundary), exactly as CR 103.4/103.5
requires.

`rules/chance_test.go` was NOT a conflicted file. This is a documented
deviation from "do not touch files the conflict does not involve": leaving it
red would wedge the daemon's full gate, and reverting the branch fix would
destroy the ticket. The integration change is test-only and preserves each
test's intent:

- **TestHypotheticalPlannerControlsMulliganShuffle** — records the mulliganing
  seat, then drives the rest of the pass with a keep (new `submitPregameKeep`
  helper) until the planner callback has run, then asserts `lastShuffle` equals
  the planned order. The planner still controls the mulligan shuffle.
- **TestHypotheticalReplayAndCloneOwnChanceState** — after the clone's mulligan
  submit, asserts the source engine is untouched, then completes the clone's
  pass and only then asserts chance was consumed; completes `e`'s and the
  replay's pass identically before the transcript comparison. The clone/chance
  ownership contract is unchanged.
- **TestHypotheticalSubmitFailurePoisonsOnlyThatBranch** — the mulligan submit
  now succeeds (the poisoned draw is consumed by the deferred redraw); the
  pass-completing keep is the call that surfaces the `bound` chance failure;
  the poison/head-immutability and base-vs-branch assertions follow. The
  chance-failure boundary contract is unchanged.

A new helper `submitPregameKeep` documents the deferral in one place.

## Commits produced

- `61779fd9` — `Merge branch 'main' into wt/cli-20260922T225141Z-48972afc`
  (default merge message; parents `7c4182ff` + `2040e5d9`). Contains the
  `agentsdoc_test.go` resolution.
- `f0bc40a3` — `test(rules): drive the deferred mulligan redraw in the chance
  planner tests` (the `rules/chance_test.go` integration; no engine change).

## Commands run and output (real)

`.cards`: **present** (real symlink, target exists) — corpus tests ran.

    $ git merge main
    Auto-merging AGENTS.md
    Auto-merging internal/testutil/agentsdoc_test.go
    CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go

Merged-row measurement (test's own method):

    raw rows: 44 => data rows: 43
    HEAD (branch) table: 48 rows; main table: 44 rows; merged: 43 rows

Conflict-resolution gate — the conflicted file's package:

    $ go test -run 'TestKnownApproximation' ./internal/testutil/
    ok  	github.com/adams-shaun/gorge/internal/testutil	0.001s
    $ go test ./internal/testutil/
    ok  	github.com/adams-shaun/gorge/internal/testutil	1.475s

The three main-planner tests, before and after the integration fix:

    before: 3 FAIL (above)
    $ go test -run 'TestHypothetical' ./rules/ -v
    --- PASS: TestHypotheticalPlannerControlsMulliganShuffle (0.00s)
    --- PASS: TestHypotheticalReplayAndCloneOwnChanceState (0.00s)
    --- PASS: TestHypotheticalSubmitFailurePoisonsOnlyThatBranch (0.00s)
    (... every other TestHypothetical* PASS)
    ok  	github.com/adams-shaun/gorge/rules	0.007s

Post-merge ratchets (the command the brief names):

    $ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
    ok  	github.com/adams-shaun/gorge/rules	0.753s

Broader targeted mulligan/pregame suite in `rules/`:

    $ go test ./rules -run 'Mulligan|Pregame|Bottoming|Hypothetical|Redraw|StartingPlayer|Kept|Keep'
    ok  	github.com/adams-shaun/gorge/rules	0.770s

Mandatory goldens outside `rules/` (per system context):

    $ go test ./internal/archtest/
    ok  	github.com/adams-shaun/gorge/internal/archtest	3.013s
    $ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
    ok  	github.com/adams-shaun/gorge/cmd/botbench	1.101s

Format / build:

    $ gofmt -l internal/testutil/agentsdoc_test.go rules/chance_test.go
    (no output)
    $ go run ./cmd/gentypes -check
    (no output)
    $ go build ./rules/ ./internal/testutil/
    (clean)

Final state:

    $ git status --short          → (empty, clean)
    $ grep -rn '^<<<<<<<|^>>>>>>>' → (no conflict markers)

## TestHeads — expected branch movement, deliberately NOT edited

    $ go test -run 'TestHeads$' ./rules/
    --- FAIL: TestHeads (1.71s)
        heads_test.go:1323: 4 seats: chain head acce7d850cfb176a, golden 20028059e8c88ec3
        heads_test.go:1323: 6 seats: chain head 3bd695df72d9d4c9, golden 400d8d9ae2777ded
        heads_test.go:1323: 8 seats: chain head 5c90b1b3a0b38f25, golden 5e988854bf022347

These are **exactly** the values the branch's own ticket measured and
attributed (`acce7d850cfb176a` / `3bd695df72d9d4c9` / `5c90b1b3a0b38f25`; 2
seats unmoved at `19a4893657e5d549`). My resolution introduced no engine
behaviour change, so nothing from main moved them further. Per the system
context, `rules/heads_test.go` is not edited by agents — the orchestrator
re-pins it at the gate from measured values (FL-107), and the ticket's report
(`.ds4/report-t1.md`) already carries the attribution. NOT a blocker for this
resolution.

## Uncertainties / notes

- The `rules/chance_test.go` edit is the one deliberate scope extension, forced
  by main's new feature observing the behaviour this ticket changes. It is
  test-only; the alternative (leave 3 tests red, or revert the reviewed fix)
  both fail the brief.
- The `knownApproximationRows` number is the measured merged count (43), not
  either side's stale constant; I verified the four closed rows are gone.

## Issues

- No new defects found beyond the integration above.
- Coverage note (already recorded by the ticket): the repo-deck acceptance
  suite runs exactly one mulligan per game and at 2 seats that mulliganer is
  the last declarer, so the 2-seat acceptance golden does not exercise the
  deferred-redraw interleaving at all; the branch's
  `rules/mulligan_redraw_order_test.go` covers the multi-declarer shapes
  directly.
- Out of scope and untouched: the adjacent AGENTS.md row "The starting player
  is uniformly random but the toss winner never CHOOSES" remains in the table.

---

## Current integration round — main at `1be022eb` (2026-09-23, this seat)

### Starting state

On arrival the tree was CLEAN at the completed merge `231065a4` (main at
`2040e5d9`, resolved by the prior round below) — no rebase or merge in
flight. The dispatch's rebase/merge-fallback failures correspond to that
already-resolved round. But main had advanced again to `1be022eb` (the
48972afc mulligan-redraw landing: `7c4182ff`, `f0bc40a3`, `599b3a92`,
`6d0a209a`, heads pin `408fdf32`) minutes before this seat began, so the owed
integration was a fresh `git merge main`.

### Conflicted files and resolutions

`git merge main --no-edit` auto-merged everything except two files:

- `.ds4/report-mrg1.md`: both sides appended integration records (branch: the
  round at `2040e5d9`; main: the 48972afc ticket's own record), and git
  textually mis-aligned the two appends. **Resolution:** kept the branch's
  accumulated log verbatim, appended main's 48972afc record verbatim as
  Record 3, then appended this round's record. No record discarded.
- `internal/testutil/agentsdoc_test.go`: the `knownApproximationRows`
  constant — HEAD 44 (the 2040e5d9 merged count), main 43 (its own
  mulligan-row closure `7c4182ff`). **Resolution:** measured the merged
  table with the test's own algorithm (lines starting `| ` inside
  `## Known approximations` … next `## `, header dropped) → **43 data rows**;
  took main's value 43 with a comment naming both sides' deletions (the
  branch's `(blockprop1)` via 6e77a1e8 and main's mulligan-redraw row via
  7c4182ff). `AGENTS.md` auto-merged both deletions (grep for `blockprop1`
  and `mulligan declaration` → 0 occurrences).

Everything else — `rules/mulligan.go`, `rules/heads_test.go` (main's re-pin
for the 48972afc movement), `rules/statics.go`, `rules/trigmatch_faceup.go`,
`rules/trigmatch_registry_test.go`, `effects/misc.go`, `events/` —
auto-merged; the branch did not touch those lines.

### Commands and output

- `.cards`: present (real symlink to `/home/sadams/projects/gorge/.cards`) —
  runs are not vacuous.
- `git status` on arrival: clean at `231065a4`.
- `git merge main --no-edit`:
  ```
  Auto-merging .ds4/report-mrg1.md
  CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
  Auto-merging AGENTS.md
  Auto-merging effects/misc.go
  Auto-merging internal/testutil/agentsdoc_test.go
  CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
  Auto-merging rules/statics.go
  Automatic merge failed; fix conflicts and then commit the result.
  ```
- Merged-table measurement (test's own awk shape): **43 rows**.
- `git add -f .ds4/report-mrg1.md internal/testutil/agentsdoc_test.go &&
  git commit --no-edit` → merge commit `a6676454` for this round.
- `go test ./internal/testutil/ -run 'TestKnownApproximations' -v`:
  ```
  === RUN   TestKnownApproximationsOnlyShrinks
  --- PASS: TestKnownApproximationsOnlyShrinks (0.00s)
  ok  github.com/adams-shaun/gorge/internal/testutil  0.001s
  ```
- Post-merge ratchets:
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  → `ok github.com/adams-shaun/gorge/rules 0.800s`.
- `go test -run 'TestHeads$' ./rules/` →
  `ok github.com/adams-shaun/gorge/rules 1.825s` — main's re-pinned goldens
  (`408fdf32`, the 48972afc mulligan movement) hold on the merged tree; the
  branch's CantBlockUnless fix moves no additional head.
- Behaviour goldens: `go test ./internal/archtest/` →
  `ok github.com/adams-shaun/gorge/internal/archtest 3.812s`;
  `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` →
  `ok github.com/adams-shaun/gorge/cmd/botbench 1.070s` (the pinned 20-game
  split did not move).
- `gofmt -l internal/testutil/agentsdoc_test.go` → clean; conflict-marker
  sweep over the resolved files: none; `git diff --cached --check`: clean.
- `git status` after: clean at `a6676454`.

### Uncertainties

None material. The constant judgement call (43 vs 44) was settled by direct
measurement of the merged table, matching main's value exactly.

### Issues

- No new defects found. Carried from main's 48972afc record: the 2-seat
  acceptance golden does not exercise the deferred-redraw interleaving (the
  branch's `rules/mulligan_redraw_order_test.go` covers it directly); the
  toss-winner-never-chooses AGENTS.md row remains in the table.
- Nothing deserving a new CR-lane test was observed during this resolution;
  the resolution changed no engine behaviour.


---

## Main-side merge-resolution archive (preserved verbatim)

# Merge conflict resolution report — cli-20260922T225141Z-1b1182e4

## Operation and conflict

- Initial `git status --short --branch`: `## wt/cli-20260922T225141Z-1b1182e4`; working tree clean, no operation in progress.
- Ran `git merge main`. It completed automatically except for one content conflict: `internal/testutil/agentsdoc_test.go`. `AGENTS.md` and the other main changes merged automatically; `rules/trigmatch_misc.go` also auto-merged.
- The conflict was `knownApproximationRows`: this branch said `53`, while main said `50`. The branch closes the First Strike Damage approximation row; main contains other approximation closures. After the merge, I measured 46 data rows in the merged `AGENTS.md` Known approximations table (one header row excluded), so resolved the constant to `46` to match the merged content. No other files were conflicted.
- Completed the merge with its default message. Merge commit: `af2f1648` (`Merge branch 'main' into wt/cli-20260922T225141Z-1b1182e4`).

## Commands and outputs

`git status --short --branch && git rev-parse --show-toplevel && git branch --show-current && git status`

```text
## wt/cli-20260922T225141Z-1b1182e4
/home/sadams/projects/gorge/.worktrees/cli-20260922T225141Z-1b1182e4
wt/cli-20260922T225141Z-1b1182e4
On branch wt/cli-20260922T225141Z-1b1182e4
nothing to commit, working tree clean
```

`git merge main`

```text
Auto-merging AGENTS.md
Auto-merging internal/testutil/agentsdoc_test.go
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
Automatic merge failed; fix conflicts and then commit the result.
```

Measured table counts from `git show <ref>:AGENTS.md`: branch `HEAD` had 53 data rows; main had 47. The automatically merged table had 46 data rows after preserving both sides' deletions.

`git add internal/testutil/agentsdoc_test.go && GIT_EDITOR=true git merge --continue && git status --short --branch`

```text
[wt/cli-20260922T225141Z-1b1182e4 af2f1648] Merge branch 'main' into wt/cli-20260922T225141Z-1b1182e4
## wt/cli-20260922T225141Z-1b1182e4
```

Corpus check: `ls .cards | head`

```text
cards.lock
cardsfolder
ir.gob.gz
ir.v4.gob.gz
tokenscripts
```

Conflicted-package check, `go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'`

```text
ok  github.com/adams-shaun/gorge/internal/testutil  0.001s
```

Post-merge ratchets, `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`

```text
ok  github.com/adams-shaun/gorge/rules  0.792s
```

Final `git status --short --branch`:

```text
## wt/cli-20260922T225141Z-1b1182e4
```

No unresolved concerns.

---

## Main lineage records from main (kept verbatim; merged at main @ e0fd7d9c)

## Main lineage record — 6a16cd8c round-2 docs (kept verbatim from main)

# Merge-conflict resolution — task cli-20260922T225140Z-6a16cd8c

(Record 1 below is this ticket's first integration round, kept from the
branch side; record 2 is the 68ca4d95 legend-rule ticket's record that main
carried in via `08955738`. Both are kept verbatim; record 3 is appended by
the round-2 docs commit.)

## Starting state

`git status` found the tree CLEAN — no rebase or merge in flight. The
daemon's earlier attempt (rebase, then a merge fallback, both conflicting per
`.ds4/merge-conflict-mrg1.md`) had been fully aborted before this seat
started; branch tip was `ade1f41d` ("test(rules): assert player attachment
and descend replay folds"). I redid the integration as a **merge of `main`
into the branch** (`git merge main`), which reproduced exactly the two
conflicts the daemon saw: `AGENTS.md` and `internal/testutil/agentsdoc_test.go`.
No other files conflicted.

## Conflicted file 1: AGENTS.md (one hunk)

Both sides touched the same slot in the "Known approximations" table — each
had DELETED a different row that the other kept:

- **main side** deleted the **(ft1)** row ("Bot target selection is
  effect-blind except for a known literal damage amount"). Main closed ft1
  with real bot-policy arms: `botpolicy/replacement.go`,
  `botpolicy/target.go` / `botpolicy/combat.go` rewrites, commit
  `87d13658` "fix(botpolicy): add real arms for replacement order, multikick
  count and mutate placement" plus `4dcef2ea` which lowered the register
  constant to 51 "after ft1 closure on main".
- **branch side** deleted the **(fx20)** row ("`MatchesPlayerSpec` still
  fails closed on `EnchantedBy` … `counters_` and compound/space forms").
  The branch closed fx20 with its reviewed commits: `c85c39b7`
  "feat(effects): resolve Player.EnchantedController,
  TriggeredDefendingPlayer and counters_", `c3662d98`, `3cbf3e11`
  "fix(rules): close player-spec grammar with descend and player
  attachments", `ade1f41d`.

**Resolution:** delete BOTH rows — each side's deletion is a legitimate
closure, and the register is delete-only. Verified by counting with the
test's own row logic (`## Known approximations` … next `## `, lines starting
`| `, minus header): base `f7357075` = 56 rows, main = 51 (its constant),
branch = 55 (its constant 57 lags — allowed, the test only fails on growth),
merged = **50**.

## Conflicted file 2: internal/testutil/agentsdoc_test.go (one hunk)

Only the `knownApproximationRows` constant conflicted: base 58, branch 57
(fx20 deletion), main 51 (ft1 + earlier main closures).

**Resolution:** `knownApproximationRows = 50` — merged table = 50 rows.
`knownOversizeRows` (8) and `standInCellLimit` (600) are identical on all
three sides; no other constant changed.

## Commands run and output

- `git merge main` →
  `CONFLICT (content): Merge conflict in AGENTS.md`,
  `CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go`
  (reproduced the daemon's conflict set).
- After resolving and `git add`:
  `git commit --no-edit` →
  `[wt/cli-20260922T225140Z-6a16cd8c 9fc45eaf] Merge branch 'main' into wt/cli-20260922T225140Z-6a16cd8c`
- `git status --short` → clean (no conflicts left; only my later report
  edit, committed separately).
- `.cards` check: `[ -e .cards ]` → **present** (real symlink; corpus-backed
  runs are not vacuous).
- `go test ./internal/testutil/ -run 'TestKnownApproximations' -v` →
  `--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)` / `ok`.
- Ratchet pass after the merge (per the 2026-09-22 instruction):
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  → `ok github.com/adams-shaun/gorge/rules 0.894s`. Verified NOT a vacuous
  skip run: verbose listing shows all five tests RUN and PASS
  (TestEveryRepoDeckIsFullySupported 0.56s, TestEveryRepoDeckParamsAreRead
  0.12s, TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched,
  TestEveryDispatchedTriggerModeHasAMatcher,
  TestEveryRepoDeckCountHeadResolves) with **0 SKIPs**.

## Post-merge ratchet check (branch-vs-main interaction)

- The branch registers NO new `Mode$` trigger matcher (its diff touches
  `trigmatch_cast.go`/`trigmatch_combat.go`/`trigmatch_misc.go` only in
  existing matchers for the player-spec closure), so nothing needed adding
  to `addedAfterTheSplit`.
- The branch's new player-spec primitives did not require edits to
  `knownUnsupported` / `knownUnsupportedParams` / `knownUnmodelledCountHeads`
  — the ratchet pass above is green with the merged tables untouched.

## Commits

- `9fc45eaf` — Merge branch 'main' into wt/cli-20260922T225140Z-6a16cd8c
  (conflict resolution; both ft1 and fx20 rows deleted, constant 58→50
  across base→merged).
- This report commit (docs only).

## Unsure / notes

- The `.ds4/report-mrg1.md` file is tracked in git despite living under the
  excluded `.ds4/` path (prior docs commits record it), so main's version
  auto-merged into the merge commit and this report replaces it as a docs
  commit to leave the tree clean.
- I chose merge (matching the daemon's own fallback path) over rebase since
  no operation was in flight and rebase of 4 commits would re-conflict the
  same two hunks; the merge preserves both histories verbatim.

---

## Record 2 — main's side (68ca4d95 legend-rule ticket, via main commit `08955738`)

# Merge-conflict resolution report — mrg1

## Branch-side tail kept from the conflict (b686e452 lineage; the bullets
below it are its continuation)

Git auto-merged: main deleted the `SetName` row (1 region), the branch
deleted its 3 rows (different regions). Verified on the resolved file:

- `KReplacement` order row: absent (0 hits)
- `multikicker1` row: absent (0 hits)
- `mutate1` row: absent (0 hits)
- `SetName` approximation row: absent (0 hits)
- counted rows: 52

## Commands run (real output)

```
$ git rebase main
Rebasing (1/4)Auto-merging AGENTS.md
Auto-merging internal/testutil/agentsdoc_test.go
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
error: could not apply 6a7f336b... fix(botpolicy): add real arms ...
$ git add AGENTS.md internal/testutil/agentsdoc_test.go && GIT_EDITOR=true git rebase --continue
[detached HEAD cf81eb11] fix(botpolicy): add real arms for replacement order, multikick count and mutate placement
Rebasing (2/4)Rebasing (3/4)Rebasing (4/4)Successfully rebased and updated refs/heads/wt/cli-20260922T225140Z-8b855197.
```

Commits 2–4 replayed with no further conflict.

```
$ git status
On branch wt/cli-20260922T225140Z-8b855197
nothing to commit, working tree clean

$ git log --oneline main..HEAD
f7d099d9 test(rules): remove obsolete multikicker bot decline assertion
e8c27906 docs: record resolved approximation arms rebase
a13459fe test(botpolicy): cover approximation arms with corpus cards
cf81eb11 fix(botpolicy): add real arms for replacement order, multikick count and mutate placement
```

Row-count check on the resolved AGENTS.md (same logic as the test):

```
$ awk '/^## Known approximations/{inside=1;next} inside&&/^## /{inside=0} inside&&/^\| /{c++} END{print "rows:", c-1}' AGENTS.md
rows: 52
```

Targeted test on the conflicted file's package:

```
$ go test -v -run 'TestKnownApproximation' ./internal/testutil/
=== RUN   TestKnownApproximationsOnlyShrinks
--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)
=== RUN   TestKnownApproximationRowsAreShort
--- PASS: TestKnownApproximationRowsAreShort (0.00s)
PASS
ok  	github.com/adams-shaun/gorge/internal/testutil	0.001s
```
(No "table is down to N" log -> the row count EQUALS the constant 52 exactly.)

Ratchets main newly carries (the brief's command):

```
$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok  	github.com/adams-shaun/gorge/rules	1.033s
```

Branch's own tests (changed packages):

```
$ go test -run 'TestBotReplacementOrderRanksBySourceWorth|TestBotReplacementOrderBypassesSkip|TestBotMultikickPaysTheAffordableMaximum|TestBotMutatePlacesUnder' ./botpolicy/
ok  	github.com/adams-shaun/gorge/botpolicy	0.004s

$ go test -run 'TestBotPolicyCorpusReplacementOrder|TestBotPolicyCorpusMultikickerPaysMaximum|TestBotPolicyCorpusMutatePlacesUnder' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.634s

$ go test -run 'TestMultikickBotDeclinesThroughTheFirstOfferArm' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.002s [no tests to run]
```
(The obsolete decline assertion was removed by the branch's own commit
`f7d099d9` — that test asserted the pre-fix behaviour the branch replaced;
its removal is the reviewed fix, and the remaining multikicker tests pass:)

```
$ go test -run 'TestMultikick' ./rules/
ok  	github.com/adams-shaun/gorge/rules
```

`gofmt -l` on the six touched Go files: no output (clean).

`.cards` symlink present (points at `/home/sadams/projects/gorge/.cards`) —
the corpus-dependent runs above were real, not vacuous skips.

## Final state

- `git status`: on branch `wt/cli-20260922T225140Z-8b855197`, **working tree
  clean**; no active rebase/merge.
- `main` (`4357caf3`) is an ancestor of HEAD.
- Rebased commits: `cf81eb11` (fix), `a13459fe` (test), `e8c27906` (docs),
  `f7d099d9` (test cleanup).
- `git diff --stat main..HEAD` = the branch's intended changes plus the
  previously committed `.ds4/report-mrg1.md` docs commit (prior seat's).

## Uncertainties / notes

- Main's constant (57) is loose against main's own table (55); I resolved to
  the *counted* 52 rather than delta arithmetic (57 − 3 = 54, or 53 − 1 = 53).
  The count is what the ratchet compares against; 52 is the exact value that
  makes the constant truthful and keeps every deleted row's intent.
- No engine behaviour was changed by the resolution: only the constant, in
  the direction both sides intended. No goldens or ratchet tables were edited.
- The `.ds4/report-mrg1.md` rewrite you are reading is committed on the branch
  (the file is tracked there since `e8c27906`), keeping the tree clean.

## Issues

None found during resolution. The only conflict was the register constant.

---

# Round 3 — rebase onto main @ 9016c22 (fresh relaunch, cli-20260922T225140Z-8b855197)

## Situation found

`git status` on entry: clean tree at `70fd74ad`, no in-flight operation. The
daemon's rebase attempt (`rebase (start): checkout main` at `9016c22` in the
reflog) had already been aborted. `main` had advanced past round 2's base
(`4357caf3`) by the bot-target closure series:

- `1f10a319`/`f2070a6a`/`c73c4314`/`317a1392` botpolicy target ranking fixes
- `a70c4483` merge, `ebe94f76` heads pin, `9016c22` merge

Ran `git rebase main` again (the operation the daemon itself uses).

## Conflicted files this round

### `internal/testutil/agentsdoc_test.go` (same single line as rounds 1–2)

- **Main side**: `knownApproximationRows = 54` (main's own table after its
  `(ft1)` row deletion).
- **Branch side**: `knownApproximationRows = 52` (round 2's exact-count
  resolution: branch's 3 row deletions + main's SetName deletion).

Both sides' deletions are disjoint and coexist; the merged AGENTS.md
measured at **51 rows** (verified: `(ft1)`, `(setname1)`, KReplacement-order,
multikicker1, mutate1 all absent — `/usr/bin/grep -c` = 0 hits).

**Resolution: `knownApproximationRows = 51`** — the measured count of the
merged table, confirmed by the ratchet itself:

```
$ go test -v -run 'TestKnownApproximations' ./internal/testutil/   # probe at 52
    agentsdoc_test.go:89: table is down to 51 rows (constant says 52) -- lower
    knownApproximationRows to 51 in the same commit that deleted them.
$ go test -v -run 'TestKnownApproximations' ./internal/testutil/   # after setting 51
--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)
```

(no looseness log — the constant equals the counted rows exactly).

### `.ds4/report-mrg1.md` (docs only)

Main's version is task `cli-20260922T225140Z-677ee477`'s merge report (its
resolver overwrote this shared report path); the branch's version is THIS
task's report. Took the **branch side** (`git checkout --theirs`) — the file
is this task's report channel, and both sides are docs-only. This section
documents the round-3 resolution.

`AGENTS.md` and `botpolicy/policy.go` auto-merged cleanly both rounds.

## Commands and outputs

```
$ git rebase main
Rebasing (1/5)Auto-merging AGENTS.md
Auto-merging botpolicy/policy.go
Auto-merging internal/testutil/agentsdoc_test.go
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
error: could not apply cf81eb11... fix(botpolicy): add real arms ...
# resolved constant to 51 (measured), git add, GIT_EDITOR=true git rebase --continue
Rebasing (2/5)Rebasing (3/5)Auto-merging .ds4/report-mrg1.md
CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
error: could not apply e8c27906... docs: record resolved approximation arms rebase
# took branch side (this task's report), git add -f, GIT_EDITOR=true git rebase --continue
Rebasing (4/5)Rebasing (5/5)Successfully rebased and updated refs/heads/wt/cli-20260922T225140Z-8b855197.
```

```
$ git status --short        # clean
$ git log --oneline main..HEAD
7ec7db11 docs: record merge-conflict resolution rebase onto main
bfd3a520 test(rules): remove obsolete multikicker bot decline assertion
00a7a776 docs: record resolved approximation arms rebase
04e9f073 test(botpolicy): cover approximation arms with corpus cards
87d13658 fix(botpolicy): add real arms for replacement order, multikick count and mutate placement
```

Ratchets main newly carries (the round-2 brief's command):

```
$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok  	github.com/adams-shaun/gorge/rules	0.761s
```

Branch's own approximation-arm tests (both sides edited botpolicy; the merge
is behaviour-neutral for them — these prove the arms still pass on the merged
tree):

```
$ go test -run 'TestBotReplacementOrderRanksBySourceWorth|TestBotReplacementOrderBypassesSkip|TestBotMultikickPaysTheAffordableMaximum|TestBotMutatePlacesUnder|TestBotPolicyCorpus' ./botpolicy/ ./rules/
ok  	github.com/adams-shaun/gorge/botpolicy	0.005s
ok  	github.com/adams-shaun/gorge/rules	0.663s
```

## Section 8 — this merge (main's limited-look integration into wt/cli-20260922T225138Z-e21c29e8)

### Starting state

The tree was CLEAN at `7bdae92f`; the daemon's rebase onto main and its merge
fallback had both been rolled back (the two conflicts described in
`.ds4/merge-conflict-mrg1.md`), so no operation was in flight. Main had
advanced past `00147db0` with the limited-look/spectator-search closure from
the sibling ticket (`30adfed9`, merge `ae220e74`). Rebase is forbidden in this
seat (shared-git rule), so the integration was done as `git merge main`.

### Conflicted files and how each side's intent was kept

- `internal/testutil/agentsdoc_test.go`: both sides carried the VALUE 71
  (HEAD's previous merge had already lowered it); only the comment text
  differed. Took main's comment verbatim — it is the accurate description of
  the merged table (main's 72 closures less the limited-look/spectator-search
  row deleted by the sibling branch).
- `.ds4/report-mrg1.md`: both kept — HEAD's Sections 5–6 verbatim, main's
  section appended as Section 7 (heading renumbered to avoid a duplicate
  "Section 5"), this round as Section 8.
- `AGENTS.md` auto-merged; verified the merged table measures exactly 71 data
  rows, matching the constant, with both sides' row deletions present.

### Commands and output

All runs in this worktree with the real `.cards` corpus symlink present.

- `git merge main --no-edit`:
  ```
  Auto-merging .ds4/report-mrg1.md
  CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
  Auto-merging AGENTS.md
  Auto-merging internal/testutil/agentsdoc_test.go
  CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
  Automatic merge failed; fix conflicts and then commit the result.
  ```
  (`git status --short` → `UU .ds4/report-mrg1.md`,
  `UU internal/testutil/agentsdoc_test.go`; AGENTS.md + all engine files
  auto-merged, including this branch's `rules/legal.go` cast-offer changes and
  main's `effects/zone.go` / `view/*` limited-look changes.)
- Merged AGENTS.md table measured: `awk '/^## Known approximations/,/^##
  Trigger-relative/' AGENTS.md | grep -c '^| '` → **71** — exact match with the
  constant, no slack either way.
- `go test ./internal/testutil -run 'TestKnownApproximation'`:
  ```
  ok  github.com/adams-shaun/gorge/internal/testutil  0.001s
  ```
- Ratchets + this branch's cast-offer suites:
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|CastOffer|CastTarget|CastBound|CastLiveness'`
  → `ok  github.com/adams-shaun/gorge/rules  0.735s`.
- `go test ./rules -run 'TestHeads$'` → `ok ... 2.012s` (main's limited-look
  change did not move a head golden).
- Behaviour goldens: `go test ./internal/archtest/` → `ok ... 2.804s`;
  `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` →
  `ok ... 1.055s` (split did not move).

No engine code was conflicted; the merge is purely integration.

---

## Section N+1 — resolver round for the 2026-09-22/23 dispatch (branch wt/cli-20260922T225139Z-244016f9, main at 78a1d9a4)

## Starting state

The worktree was CLEAN at `620546f9` — no rebase or merge in flight. The
failed rebase the dispatch described had been rolled back; a prior seat had
already merged an older main tip (`af9fe768`, main at `00147db0`). Since then
main had advanced to `78a1d9a4` (the limited-look / cast-offer-census
integration), so this seat ran `git merge main` and resolved the two
conflicted files it produced.

## Conflicted files and resolution

1. `internal/testutil/agentsdoc_test.go` — only the `knownApproximationRows`
   rationale comment conflicted; both sides set the constant to 71, each with
   a stale story. Measured the MERGED `AGENTS.md` table with the test's own
   row-extraction logic: base `00147db0` held 72 data rows, the branch
   deleted the `(diguntil1)` row, main deleted the limited-look /
   spectator-search row AND the CR 601.2c cast-offer-census row — the union
   is **69**. Resolved to `knownApproximationRows = 69` with a comment naming
   all three deletions; both sides' "71" comments described only their own
   side and would have been false for the merged tree.
2. `.ds4/report-mrg1.md` — HEAD's closing paragraph vs main's appended
   "Section 2" report. Kept both: HEAD's paragraph, then main's section,
   markers dropped.

`AGENTS.md`, `decision/decision.go` and `effects/cardflow.go` auto-merged;
no engine code conflicted.

## Commands and output

- `git status` (start): `On branch wt/...; nothing to commit, working tree clean`.
- `git merge main` → conflicts in `.ds4/report-mrg1.md` and
  `internal/testutil/agentsdoc_test.go` (17 files total in the merge).
- Row measurement (`git show <ref>:AGENTS.md` piped through the test's
  region-bounded `^\| ` count): base 72, branch(af9fe768) 71, main 70,
  merged 69.
- `go test ./internal/testutil/ -run 'TestKnownApproximation' -v` →
  `--- PASS` for `TestKnownApproximationsOnlyShrinks` and
  `TestKnownApproximationRowsAreShort`, no stale-count log.
- `git add` both files; `git commit --no-edit` → merge commit `12329490`;
  `git status --short --branch` clean; `git merge-base --is-ancestor main HEAD` → merged.
- Ratchets: `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  → `ok  github.com/adams-shaun/gorge/rules  0.726s`.

## Issues

None found in the conflict resolution itself. One note: main's own
`agentsdoc_test.go` comment claimed 71 while main's table already held 70
rows (it accounted for only one of its two deletions); the merged-tree
constant 69 supersedes both comments.

---

## Section N+2 — searchmay1 (branch wt/cli-20260922T225139Z-18baec47, merge of main)

`git merge main` conflicted in `AGENTS.md`, `internal/testutil/agentsdoc_test.go`
and this report. `AGENTS.md`: both sides deleted DISJOINT rows — this branch the
`(searchmay1)` row, main the "No LIMITED-look grammar" row — so the resolution
keeps BOTH deletions (the conflict hunk collapses to nothing).
`internal/testutil/agentsdoc_test.go`: main 66, branch 73; resolved to the
measured merged count. This report resolved to main's log plus this section.

---

## Section N+3 — this merge (main f7357075 into wt/cli-20260922T225140Z-b686e452)

### Starting state

The tree was CLEAN at `d56e404f` (the branch's one commit: `non<X>` negation
closure, `effects/filter.go` + its test + the row deletion + constant 59). No
rebase or merge was in flight. Main was 4 commits ahead (`c93d85f7` →
`f7357075`, the scry/surveil pile-B-order closure and its web/decision/
botpolicy/arrange changes plus its merge chain). Ran `git merge main
--no-commit --no-ff`.

### Conflicted files and how each side's intent was kept

Only ONE file conflicted: `internal/testutil/agentsdoc_test.go`.

- **HEAD:** `knownApproximationRows = 59` (branch's non<X> row deletion).
- **main:** `knownApproximationRows = 58` (main's TWO Scry/Surveil pile-B row
  deletions).
- Both sides' deletions are disjoint; `AGENTS.md` auto-merged keeping BOTH —
  `grep` for the `non<X> negation` row, the `Scry sends its unchosen pile B`
  row and the `Surveil puts its unchosen pile B` row each finds 0 occurrences
  in the merged file.
- Merged table measured with the test's own parsing logic (python replication
  of `approximationRows`): base `f8496199` = 58 data rows, main = 56, HEAD =
  57, merged = **55** (58 − 3). Resolved the constant to **55** with a comment
  naming all three deletions. Note the base constant (60) already carried
  slack of 2; the merged value 55 removes all slack.
- `knownOversizeRows` = 8 on both sides; the merged table measures 6 oversize
  rows on every side (none of the three deleted rows was oversize), so the
  constant is untouched — shrinkage only.

No engine code conflicted: the branch's `effects/filter.go` change and main's
`rules/arrange.go`, `effects/cardflow.go`, `decision/decision.go`,
`botpolicy/policy.go`, `web/*` changes auto-merged (disjoint files/regions).

### Commands and output

- `.cards` present (real corpus symlink) — runs are real, not vacuous skips.
- `git merge main --no-commit --no-ff` →
  ```
  Auto-merging AGENTS.md
  Auto-merging internal/testutil/agentsdoc_test.go
  CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
  Automatic merge failed; fix conflicts and then commit the result.
  ```
  (`git status --short` → `UU internal/testutil/agentsdoc_test.go` only)
- `go test ./internal/testutil -run 'TestKnownApproximation' -v` (after 55):
  `--- PASS` both tests, `ok ... 0.004s` — exact match, no slack log.
- Branch-fix sanity: `go test ./effects -run 'NonPredicate|NonCopied'` →
  `ok ... 0.703s`.
- Main-side sanity: `go test ./rules -run 'Arrange'` → `ok ... 0.777s`.
- Ratchets: `go test ./rules -run 'TestNoTriggerModeIsRegistered|
  TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|
  CountHead'` → `ok ... 0.885s`.
- `go test ./rules -run 'TestHeads$'` → `ok ... 1.873s` (no head golden moved).
- Behaviour goldens: `go test ./internal/archtest/` → `ok ... 3.156s`;
  `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` →
  `ok ... 1.197s` (split did not move).
- `gofmt -l internal/testutil/agentsdoc_test.go` → clean; no conflict markers
  remain in any .go file.
- `git commit --no-edit` → merge commit `4bd357c1`; `git status --short
  --branch` → clean.

### Issues

None found beyond the resolved conflict itself.

---

## Section N+4 — verification in this resolver session

The dispatch arrived after the integration had already been completed: initial
`git status` was clean at `e262a3a4`, with merge commit `4bd357c1` underneath
it and no rebase or merge in progress. The merge commit records the sole
conflict (`internal/testutil/agentsdoc_test.go`); the preceding Section N+3
records both sides' intended row-count values, the combined AGENTS.md deletions,
and the resolution to the merged table's measured count of 55. `AGENTS.md`
auto-merged, retaining the branch's `non<X>` row deletion and main's two
Scry/Surveil row deletions.

I found a duplicated pair of `knownApproximationRows` explanation comments
left by the resolution and removed the duplicate; the constant remains 55.
This was the only additional edit. `.cards` is present in this worktree.

Commands and output:

```text
git status --short --branch
## wt/cli-20260922T225140Z-b686e452

[ -e .cards ] && echo '.cards present' || echo '.cards missing'
.cards present

go test ./internal/testutil -run 'TestKnownApproximation' -v
=== RUN   TestKnownApproximationsOnlyShrinks
--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)
=== RUN   TestKnownApproximationRowsAreShort
--- PASS: TestKnownApproximationRowsAreShort (0.00s)
PASS
ok   github.com/adams-shaun/gorge/internal/testutil 0.001s

go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok   github.com/adams-shaun/gorge/rules 0.783s
```

No conflict markers remain in the affected Go file. No engine behavior or ratchet
entries changed, and there are no additional issues or uncertainties.

### Fresh resolver verification

On this dispatch, the worktree was already clean at `f76e75ad`, following merge
commit `4bd357c1`; there was no rebase or merge operation in progress. The
conflict resolution and its report were already committed. `.cards` is present.

Commands and output from this verification:

```text
go test ./internal/testutil -run 'TestKnownApproximation' -v
=== RUN   TestKnownApproximationsOnlyShrinks
--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)
=== RUN   TestKnownApproximationRowsAreShort
--- PASS: TestKnownApproximationRowsAreShort (0.00s)
PASS
ok   github.com/adams-shaun/gorge/internal/testutil  0.001s

go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok   github.com/adams-shaun/gorge/rules  0.813s
```

The merged table and constant remain 55. No further conflict or uncertainty.

---

## Section N+5 — current dispatch (cli-20260922T225140Z-b686e452)

### Starting state and integration status

`git status --short --branch` and `git status` showed a clean tree on
`wt/cli-20260922T225140Z-b686e452`. No rebase or merge was in progress.
The failed rebase/fallback described by `.ds4/merge-conflict-mrg1.md` had
already been completed in merge commit `4bd357c1` (parents `d56e404f` and
`f7357075`); subsequent report-only commits were already present at entry.
Thus there was no in-flight operation to continue and no new conflict edit was
needed in this session.

### Conflicted files and resolution

- `internal/testutil/agentsdoc_test.go` was the sole conflict. The branch side
  carried `knownApproximationRows = 59` for its one deleted `non<X>` row;
  main carried 58 for its two Scry/Surveil pile-B deletions. `AGENTS.md`
  auto-merged and retains all three disjoint deletions. The recorded merge
  resolution sets the merged count to 55 (58 minus three), with the explanatory
  comment; it is present in `4bd357c1` and the focused test passes.
- `AGENTS.md` was auto-merged; its row deletions are retained. No engine code
  required conflict resolution.

The requested merge operation is complete. No additional commit was necessary;
the resolution is already committed as `4bd357c1`. The current tree remained
clean after tests. `.cards` is present in this worktree.

### Commands and output in this verification

```text
git status --short --branch; git status
## wt/cli-20260922T225140Z-b686e452
On branch wt/cli-20260922T225140Z-b686e452
nothing to commit, working tree clean

git show --format=fuller --no-patch 4bd357c1
commit 4bd357c1017189fc013dccf1f9cf32d98cbd4d7f
Merge: d56e404f f7357075

go test ./internal/testutil -run 'TestKnownApproximation' -v
=== RUN   TestKnownApproximationsOnlyShrinks
--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)
=== RUN   TestKnownApproximationRowsAreShort
--- PASS: TestKnownApproximationRowsAreShort (0.00s)
PASS
ok   github.com/adams-shaun/gorge/internal/testutil 0.001s

go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok   github.com/adams-shaun/gorge/rules 0.820s

[ -e .cards ] && echo '.cards present' || echo '.cards missing'
.cards present
```

No uncertainties remain about the reported conflict or its completed resolution.


---

## Current merge resolution — main `9016c22c`

The prior resolver report above is preserved. Main had advanced beyond the branch's earlier merge (`4bd357c1`), so this merge integrates current main; no rebase was in progress.

### Conflicts and resolution

- `internal/testutil/agentsdoc_test.go`: branch recorded 55 rows after its `non<X>` closure plus the two Scry/Surveil deletions. Main carries additional approximation-row deletions; after the combined disjoint deletions, the merged table measures 53. Updated the constant to 53. The merged table preserves main's and the branch's row removals.
- `.ds4/report-mrg1.md`: this is a shared accumulated report. Kept the branch's existing report history and main's trailing verification line (which refers to its own worktree) and added this round's resolution report; no history discarded.
- `AGENTS.md` auto-merged. All other main changes are retained; no engine-code conflict required manual resolution.

### Commands and output

- `git status --short --branch` before merge: clean on `wt/cli-20260922T225140Z-b686e452`.
- `git merge main --no-edit`:
  ```
  Auto-merging .ds4/report-mrg1.md
  CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
  Auto-merging AGENTS.md
  Auto-merging internal/testutil/agentsdoc_test.go
  CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
  Automatic merge failed; fix conflicts and then commit the result.
  ```
- Merged approximation count: **53** (`awk` over the merged `AGENTS.md` table); the test's measured count agrees.
- `go test ./internal/testutil -run 'TestKnownApproximation' -v`:
  ```
  === RUN   TestKnownApproximationsOnlyShrink
  --- PASS: TestKnownApproximationsOnlyShrink (0.00s)
  === RUN   TestKnownApproximationRowsAreShort
  --- PASS: TestKnownApproximationRowsAreShort (0.00s)
  PASS
  ok   github.com/adams-shaun/gorge/internal/testutil  0.002s
  ```
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`:
  ```
  ok   github.com/adams-shaun/gorge/rules  0.780s
  ```
- `gofmt -l internal/testutil/agentsdoc_test.go`, `git diff --check`, and a conflict-marker grep over the resolved report and Go file: no output.

- `git commit --no-edit` created merge commit `8eab585f` (parents: pre-merge branch `1b04c7c9`, main `9016c22c`).
- Final `git status --short --branch`:
  ```
  ## wt/cli-20260922T225140Z-b686e452
  ```
  `git merge-base --is-ancestor main HEAD` succeeded.

No unresolved conflict or uncertainty remains.

Final check from main's side of the report conflict: `git status --short --branch` returned only `## wt/cli-20260922T225140Z-677ee477` (clean), HEAD `a70c4483`; its own merged approximation table measured 54 data rows.

---

## Preserved report from main (task 677ee477's own merge round)

Behaviour golden (main also changed `botpolicy/target.go`):

```
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	0.979s
```

`gofmt -l` on the touched Go file: no output. `.cards` symlink present
(→ `/home/sadams/projects/gorge/.cards`), so the corpus-backed runs above
were real, not vacuous skips.

## Uncertainties / notes

- Same method as round 2: resolved the register constant to the *measured*
  count of the merged table (51), not either side's stale value. Main's 54
  and the branch's 52 each described a table that no longer exists after the
  other side's deletions land.
- `.ds4/report-mrg1.md` keeps this task's report; main's copy belongs to task
  677ee477 and is preserved in that task's own history on main.
- No engine behaviour was changed by the resolution: one constant, docs, and
  the rebase replay of already-reviewed commits.

## Issues

None found during this round's resolution.

---

## Current merge resolution — main `c472a88f`

The tree was CLEAN at `1701eb73` (branch HEAD); the daemon's rebase onto main
and its merge fallback had both been rolled back, so no operation was in
flight. Main had advanced past `9016c22c` (the branch's previous merge base)
with the replacement-order/multikick/mutate bot-policy arm closures
(`87d13658`), the `ft1` register-constant lowering (`4dcef2ea`) and a further
merge (`c472a88f`). Rebase is forbidden in this seat (shared-git rule), so the
integration was done as `git merge main`.

### Conflicts and resolution

- `internal/testutil/agentsdoc_test.go`: both sides lowered the constant from a
  different base. Branch recorded 53 (its `non<X>` closure on top of
  `9016c22c`); main recorded 51 (its three bot-arm closures). The MEASURED
  count of the auto-merged `AGENTS.md` table is **50** — branch's 53 minus the
  three rows main deleted (KReplacement clamp fallback, `(multikicker1)`,
  `(mutate1)`) — so the constant is set to the measurement, 50, with the
  branch's `non<X>` row deletion preserved.
- `AGENTS.md`: auto-merged cleanly. Verified the merged table equals main's
  table minus the branch's `non<X>` row, and nothing else; 50 data rows.
- `.ds4/report-mrg1.md`: shared accumulated report. Kept the branch's Sections
  1–8 history verbatim and preserved main's round as a labelled section; no
  history discarded. This section appended.

### Commands and output

- `git status` before merge: clean on `wt/cli-20260922T225140Z-b686e452`.
- `git merge main`:
  ```
  Auto-merging .ds4/report-mrg1.md
  CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
  Auto-merging AGENTS.md
  Auto-merging internal/testutil/agentsdoc_test.go
  CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
  Automatic merge failed; fix conflicts and then commit the result.
  ```
- Merged approximation row count: `50` (awk over the merged `AGENTS.md` table;
  matches `knownApproximationRows = 50`).
- `go test ./internal/testutil -run 'TestKnownApproximation' -v`:
  ```
  === RUN   TestKnownApproximationsOnlyShrinks
  --- PASS: TestKnownApproximationsOnlyShrinks (0.00s)
  === RUN   TestKnownApproximationRowsAreShort
  --- PASS: TestKnownApproximationRowsAreShort (0.00s)
  PASS
  ok  	github.com/adams-shaun/gorge/internal/testutil
  ```
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`:
  ```
  ok  	github.com/adams-shaun/gorge/rules
  ```
- `go test -run 'TestNonPredicate|TestNonCopiedSpell|TestNonColorless|TestNonChosenCard' ./effects/`:
  ```
  ok  	github.com/adams-shaun/gorge/effects
  ```
- `gofmt -l internal/testutil/agentsdoc_test.go`, `git diff --check`, and a
  conflict-marker grep over the resolved report and Go file: no output.

No unresolved conflict or uncertainty remains.

## Issues

None found during this round's resolution.

Final state: `git commit --no-edit` created merge commit `0d70e635` (parents:
pre-merge branch `1701eb73`, main `c472a88f`). `git status --short --branch`
returned only `## wt/cli-20260922T225140Z-b686e452`; `git merge-base
--is-ancestor main HEAD` succeeded. Behaviour goldens `internal/archtest` and
`cmd/botbench`'s `TestConstructedDefaultIsByteIdentical` both pass; the
approximation register and every merge-ratchet test pass with the merged table
at 50 rows.

---

## Main-side tail kept from the conflict

No uncertainty remained in the conflict resolution. No non-conflict files were manually changed.

---

## Section — this merge (main cf3e3784 into wt/cli-20260922T225140Z-b686e452)

### Starting state

`git status` was CLEAN on `wt/cli-20260922T225140Z-b686e452` at `411fd2b0` (the
branch's previous merge of main); the dispatch's rebase and its merge fallback
had both been rolled back, so no operation was in flight. Main had advanced
8 commits to `cf3e3784` (the CR 704.5j legend-choice closure `74870371` + its
heads re-pin `da0a323b`, and the merge resolution `cf3e3784`). Rebase is
forbidden in this seat, so the integration was done as `git merge main`.

### Conflicted files and how each side's intent was kept

1. **`AGENTS.md`** — one conflict hunk inside the Known approximations table.
   Measured with the test's own row-extraction logic: base `c472a88f` = 51 data
   rows, HEAD = 50, main = 51. Per-row (grep counts on `git show <ref>:AGENTS.md`):
   - **legend row** (`The CR 704.5j legend rule keeps the first duplicate…`):
     base 1, HEAD 1, main 0 — main's legend ticket CLOSED it (the rule now asks
     the duplicate set's controller; remainder recorded in `74870371`'s commit
     message). Kept main's deletion.
   - **`non<X>` row**: base 1, HEAD 0, main 1 — this branch's reviewed fix
     `d56e404f` CLOSED it (all three named shapes were already handled or are
     now handled and test-pinned). Kept the branch's deletion; main's copy was
     stale.
   - **`KReplacement` bot-fallback row** (present only on main's side of the
     hunk): NOT resurrected. It says "there is no policy arm of its own", but
     `87d13658` — an ancestor of BOTH sides — deleted this exact row when it
     added the real `chooseReplacementOrder` arm, which is byte-identical in
     `botpolicy/policy.go` on both trees (verified: `git diff HEAD main --
     botpolicy/policy.go` is empty). The row was re-carried by main's own merge
     resolution `cf3e3784` in error; keeping it would leave a false row in the
     frozen register (a re-add is GROWTH).
   Resolution: the hunk collapses to nothing — merged table measures **49**
   data rows (51 − branch's `non<X>` − main's legend), a strict shrink.
2. **`internal/testutil/agentsdoc_test.go`** — auto-merged to 50; lowered to
   **49** with a comment naming the two real closures and the stale row.
3. **`.ds4/report-mrg1.md`** — HEAD's accumulated report vs main's one-line
   tail. BOTH kept (main's tail under its own heading), this section appended.

No engine code conflicted: main's `rules/sba.go` legend changes,
`rules/heads_test.go` re-pin and the rest auto-merged.

### Commands and output

- `.cards` present (real corpus symlink) — runs are real, not vacuous skips.
- `git merge main --no-edit` → conflicts in `.ds4/report-mrg1.md` and
  `AGENTS.md`; `internal/testutil/agentsdoc_test.go` auto-merged.
- Merged-table measurement (python replication of `approximationRows`): 49.
- `go test ./internal/testutil -run 'TestKnownApproximation' -v` →
  `--- PASS: TestKnownApproximationsOnlyShrinks`, `--- PASS:
  TestKnownApproximationRowsAreShort`, `ok ... 0.001s` (exact match, no slack).
- Ratchets + goldens:
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|TestHeads$'`
  → `ok github.com/adams-shaun/gorge/rules 1.845s` (heads golden passes with
  main's legend re-pin merged in — no further head moved).
- Branch-fix sanity: `go test ./effects -run 'NonPredicate|NonCopied|NonColorless|NonChosen'`
  → `ok ... 0.641s`.
- Main-side sanity: `go test ./rules -run 'Legend'` → `ok ... 0.694s`.
- Behaviour goldens: `go test ./internal/archtest/` → `ok ... 3.506s`;
  `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` →
  `ok ... 0.991s` (split did not move).
- `gofmt -l internal/testutil/agentsdoc_test.go` → clean; no conflict markers
  remain in any tracked file.

### Issues

None found beyond the resolved conflict itself. One note recorded above: main's
`cf3e3784` merge resolution resurrected the stale `KReplacement` bot-fallback
row; the merged tree drops it rather than carrying a false row forward.

---

## Main lineage record — 6a16cd8c round 2 (kept verbatim from main)

## Record 3 — round 2 integration (this seat, 2026-09-23)

### Starting state

`git status` found the tree CLEAN at `d3e21bfa`; no rebase or merge in
flight (the daemon's second integration attempt was fully aborted before
dispatch). The branch's merge-base with `main` was `c472a88f` — main had
moved past the branch's round-1 merge (`9fc45eaf`) with the 68ca4d95
legend-rule landing (`da0a323b` heads pin + `cf3e3784` merge). I redid the
integration as `git merge main`.

### Conflicted file: .ds4/report-mrg1.md (the only one)

- **branch side** (`d3e21bfa`): this ticket's round-1 resolution record.
- **main side** (`08955738`): the 68ca4d95 legend-rule ticket's own
  resolution record, committed to main.
- **Resolution:** keep BOTH records verbatim, headed, plus this record 3.
  `AGENTS.md` and `internal/testutil/agentsdoc_test.go` auto-merged this
  time: branch deleted the (fx20) row; main swapped the legend row for the
  `KReplacement` bot-fallback row (row count unchanged). Merged register =
  50 rows = the constant (verified below). `rules/heads_test.go` was
  auto-merged from main (2-seat head → `19a4893657e5d549`); the branch
  never touched it, so no head conflict arose and no golden was edited.

### Commands and output

- `.cards`: present (real symlink) — runs are not vacuous.
- `git merge main` → `CONFLICT (content): Merge conflict in .ds4/report-mrg1.md`
  (only unmerged path); resolved with both records kept, `git add -f`
  (`.ds4` is gitignored for untracked files), `git commit --no-edit` →
  `bdbdd7cb Merge branch 'main' into wt/cli-20260922T225140Z-6a16cd8c`.
- `git status --short` → clean.
- `grep -c 'fx20' AGENTS.md` → `0`; `grep -c 'KReplacement.*clamp fallback' AGENTS.md` → `1`.
- `go test ./internal/testutil/ -run 'TestKnownApproximations' -v` →
  `--- PASS: TestKnownApproximationsOnlyShrinks` / `ok`.
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  → `ok github.com/adams-shaun/gorge/rules 0.756s`; verbose log
  (`.ds4/scratch/ratchet.log`) shows all five RUN and PASS, **0 SKIPs**.
- Behaviour goldens: `go test ./internal/archtest/` → `ok 3.225s`;
  `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` →
  `ok 1.075s` — the pinned 20-game bot split did NOT move.

### Notes

- The branch registers no new `Mode$` matcher and closed no
  `knownUnsupported`/`knownUnsupportedParams`/`knownUnmodelledCountHeads`
  entry, so no ratchet table edit was needed; all pass untouched.
- Commits: `bdbdd7cb` (merge, conflict resolution) + this docs commit.

---

## Record — round 3 integration (this seat, b686e452 lineage, 2026-09-23)

### Starting state

`git status` on entry: tree CLEAN at `330bac6d` (the branch's round-2 merge of
main `cf3e3784`, completed by the previous seat); no rebase or merge in
flight — but `main` had advanced again 2 minutes after that merge, to
`c3256fc3` (the 6a16cd8c player-spec closure landing: `3cbf3e11`,
`c85c39b7`, `c3662d98`, `ade1f41d` + their merges), so the round-2
integration was already stale. The previous seat also never wrote its report
(`report_written: false` in the harness cutoff). I re-integrated with
`git merge main` (rebase forbidden in this seat).

### Conflicts and resolution

1. **`internal/testutil/agentsdoc_test.go`** — only the `knownApproximationRows`
   constant + comment: HEAD 49 (its two row drops), main 50 (its fx20 drop).
   The auto-merged `AGENTS.md` table measures **48** data rows (base
   `cf3e3784` = 51, minus the branch's `non<X>` drop (its fix `d56e404f`
   implements `nonCopiedSpell` — verified in `effects/filter.go:1339`) and its
   stale `KReplacement`-fallback drop (the real arm is `87d13658`'s
   `chooseReplacementOrder`, `botpolicy/policy.go:479`), minus main's (fx20)
   player-spec drop). Resolved to `knownApproximationRows = 48` with a comment
   naming all four closures.
2. **`.ds4/report-mrg1.md`** — both lineages' record accumulations (HEAD's
   b686e452 rounds vs main's 6a16cd8c/68ca4d95 records). BOTH kept verbatim,
   each under a lineage heading, plus this record appended.

`AGENTS.md` and all engine code auto-merged (main's fx20 closure touches
`events/apply.go`, `rules/*`, `state/*`, `effects/attach.go`,
`effects/filter.go` — none of them conflicted with this branch's diff).

### Commands and output

- `.cards` present (real symlink to `/home/sadams/projects/gorge/.cards`) —
  runs below are real, not vacuous skips.
- Merged-table count (the test's own awk logic): `rows: 48`; `grep -c fx20
  AGENTS.md` → 0; `KReplacement.*clamp fallback` → 0; `non<X>`-row → 0.
- `go test ./internal/testutil/ -run 'TestKnownApproximation' -v` →
  `--- PASS: TestKnownApproximationsOnlyShrinks`,
  `--- PASS: TestKnownApproximationRowsAreShort`, `ok ... 0.001s`.
- Ratchets + heads golden:
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|TestHeads$'`
  → `ok github.com/adams-shaun/gorge/rules 2.129s`.
- Main's new fx20 closure tests (merged code, both packages):
  `go test ./rules -run 'PlayerSpec|PlayerAttachment|Descend|Defending|Enchanted|Counters_'`
  → `ok ... 0.718s`;
  `go test ./effects -run 'Fx20|PlayerSpec|Enchanted|Counters_|Descend'`
  → `ok ... 0.628s`.
- Branch's own non<X> suites still pass with main's filter.go changes merged:
  `go test ./effects -run 'NonPredicate|NonCopied|NonColorless|NonChosen'`
  → `ok ... 0.636s`.
- Behaviour goldens: `go test ./internal/archtest/` → `ok ... 3.418s`;
  `go test -run 'TestConstructedDefaultIsByteIdentical' ./cmd/botbench/`
  → `ok ... 1.012s` (pinned 20-game bot split did NOT move).
- `git status` after commit: working tree clean; `main` (`c3256fc3`) is an
  ancestor of HEAD.

### Notes / uncertainties

- The branch registers no new `Mode$` matcher and closed no
  `knownUnsupported`/`knownUnsupportedParams`/`knownUnmodelledCountHeads`
  entry, and main's closure neither: the ratchet run above is green with the
  tables untouched.
- Both lineages' reports are preserved in this file under explicit headings;
  some ordering reads oddly because the sides had diverged at the same
  anchor lines, but no record was dropped.

## Issues

None found during this integration round beyond the resolved conflicts.


---

## Main lineage record — 62421b89 integration (kept verbatim from main)

# Merge-conflict resolution — cli-20260922T225140Z-62421b89

## Operation and starting state

- `git status` at start: `On branch wt/cli-20260922T225143Z-bc326d39; nothing to commit, working tree clean` — no rebase or merge was in flight (the daemon's failed integration was rolled back; `main` was NOT an ancestor of HEAD).
- Ran `git merge main`. Auto-merged `AGENTS.md` and `rules/cast.go`; one content conflict: `internal/testutil/agentsdoc_test.go`.

## The conflict and its resolution

`internal/testutil/agentsdoc_test.go`, the `knownApproximationRows` constant only:

- HEAD (this branch, tip `e748019d`): `knownApproximationRows = 53`
- main (tip `78d3b764`): `knownApproximationRows = 50`

Both constants were measured at their own tips; the merge auto-combined BOTH sides' AGENTS.md row deletions, so neither described the merged table. I re-measured the merged `AGENTS.md` using the test's own row-counting logic (`| `-prefixed lines between the `## Known approximations` heading and the next `## ` heading, dropping the header row): **45 data rows**. Set `knownApproximationRows = 45`.

No other file had conflict markers. No behavioural choice was involved — the branch's damage-LKI fix (`56f98b13` + `e748019d`) and main's changes to `rules/cast.go` auto-merged.

## Commands and outputs

`git merge main`:

```text
Auto-merging AGENTS.md
Auto-merging internal/testutil/agentsdoc_test.go
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
Auto-merging rules/cast.go
Automatic merge failed; fix conflicts and then commit the result.
```

Row measurement:

```text
awk '/^## Known approximations/{inside=1; next} inside&&/^## /{inside=0} inside&&/^\| /{c++} END{print c}' AGENTS.md
46   (includes the header row => 45 data rows)
```

After staging the resolved file: `GIT_EDITOR=true git merge --continue` →

```text
[wt/cli-20260922T225143Z-bc326d39 f89a62fd] Merge branch 'main' into wt/cli-20260922T225143Z-bc326d39
```

Post-merge checks (`.cards` present):

- `go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'` → `ok github.com/adams-shaun/gorge/internal/testutil 0.001s`
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'` → `ok github.com/adams-shaun/gorge/rules 0.741s`
- `go test ./rules -run 'TestHeads'` → `ok github.com/adams-shaun/gorge/rules 1.744s` (main moved the golden heads; the merged tree reproduces them — no head movement from this branch's cost-LKI fix)

Final `git status --short`: empty (clean). HEAD = `f89a62fd`.

## Notes / unsure about

- The stale `.ds4/report-mrg1.md` from the sibling branch (677ee477) was tracked in git and the merge staged main's/theirs version of it; this report overwrites it. Not a conflict file.
- The merged approximation table at 45 rows is lower than EITHER side's constant because the two branches closed different rows and the auto-merge keeps both closures — measured, not assumed.

---

## Situation found

The worktree was clean on entry at `7a45ef9a` with no in-flight merge or rebase. `main` was at `0104252d2aac00c38b8ee63675239b78dddef2ff`, not an ancestor of the branch. `.cards` was present.

Ran `git merge main`. It auto-merged the main changes and reported one conflicted file: `AGENTS.md`.

## Conflicted file and resolution

### `AGENTS.md`

The branch side had deleted its now-closed approximation row for CR 616.1 replacement ordering; it also carried the CR 704.5j legend-rule approximation. Main's conflicting side contained the old CR 616.1 row and a botpolicy fallback row, as well as the legend-rule row. The branch has implemented both the CR 616.1 order choice and `botpolicy`'s `KReplacement` policy (`botpolicy/policy.go`, `chooseReplacementOrder`), so the two main-side CR 616.1 rows describe behavior no longer present and were not retained. Kept the branch's removal and retained main's still-applicable CR 704.5j legend-rule row. Other main edits to this file, including removal of the closed fx20/pc1 rows, were preserved by the auto-merge.

No other file had merge markers. The remaining main changes auto-merged. The required main ratchet run found two stale `apiSpecificRulesSA` entries in `rules/paramcensus_test.go`: `Engine.applyAddCounterBody` and `Engine.applyAddCounterReplacements` consume the priced result and no longer read SA params. Removed those stale classifications, retaining `Engine.counterReplaceOp`, which does read the parameters. This is the ratchet-table correction required for the merged branch; no engine behavior was changed by it.

## Commands and output

```text
$ git status --short --branch && git rev-parse --abbrev-ref HEAD && git diff --name-only --diff-filter=U
## wt/cli-20260922T225140Z-62421b89
wt/cli-20260922T225140Z-62421b89

$ git rev-parse main
0104252d2aac00c38b8ee63675239b78dddef2ff

$ git merge main
Auto-merging AGENTS.md
CONFLICT (content): Merge conflict in AGENTS.md
Auto-merging rules/engine.go
Auto-merging rules/turn.go
Automatic merge failed; fix conflicts and then commit the result.

$ git add AGENTS.md && go test -run 'TestKnownApproximation' ./internal/testutil/
ok   github.com/adams-shaun/gorge/internal/testutil  0.001s

$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
--- FAIL: TestEveryRepoDeckParamsAreRead (0.15s)
    paramcensus_test.go:2738: paramcensus rot guard: 2 findings:
        paramcensus: apiSpecificRulesSA entry "Engine.applyAddCounterBody" no longer reads SA params -- delete the stale entry
        paramcensus: apiSpecificRulesSA entry "Engine.applyAddCounterReplacements" no longer reads SA params -- delete the stale entry
FAIL
github.com/adams-shaun/gorge/rules  0.892s
FAIL

# Removed the two stale entries from rules/paramcensus_test.go.
$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok   github.com/adams-shaun/gorge/rules  0.750s
```

`git diff --check` reported no whitespace errors after conflict resolution. The first focused approximation test passed. The first ratchet run exposed the two stale entries above; the rerun passed after removing them. `.cards` existed, so the corpus-dependent ratchets were not vacuous skips.

## Uncertainties / issues

No unresolved conflict or uncertain resolution remains. The replacement-order approximation rows from main were intentionally omitted because the reviewed branch fix and the already-present botpolicy arm close those claims. The unrelated CR 704.5j row remains intact. No engine behavior change or golden edit was made during conflict resolution.

---

## Record — round 4 integration (this seat, b686e452 lineage, 2026-09-23)

### Starting state

`git status` on entry: tree CLEAN at `d396b8ba` (the branch's round-3 merge of
main `c3256fc3`, completed by the previous seat, gates green); no rebase or
merge in flight. But `main` had advanced again past `c3256fc3` to `e6a2a84d`
(three more tickets' merges: `0104252d` = ab191b03 pc1 context-bound object
predicates + `0acadaa3` imprint-filter expiry, `0c7e5ce9` = c385caf3 EachDamage
fail-closed outcomes, `e6a2a84d` = 62421b89 CR 616.1 competing-replacement
order choice), so the round-3 integration was stale again. This is why the
daemon re-dispatched mrg1. I re-integrated with `git merge main` (rebase
forbidden in this seat).

### Conflicts and resolution

1. **`AGENTS.md`** — one conflict hunk in the Known approximations table.
   HEAD side carried the all-`Updated`-competition row and the (pc1) row
   (both since CLOSED on main by `e6a2a84d`/`179a3de1`); main side carried the
   re-booked CR 704.5j legend row (deliberately re-added by the 62421b89
   lineage — main's later deliberate change, kept) and the `non<X>` row
   (closed on THIS branch by the reviewed fix `d56e404f`; main's copy is
   stale for the merged code). Resolution: keep only the legend row from
   main's side; drop both HEAD-side rows and main's stale `non<X>` row.
   The each1 row's deletion auto-merged (c385caf3). Merged table measures
   **46** rows (48 − all-`Updated` − pc1 − each1 + legend); constant lowered
   48 → 46 with a comment naming every closure.
2. **`internal/testutil/agentsdoc_test.go`** — auto-merged to HEAD's value
   (48); updated to 46 as above.
3. **`.ds4/report-mrg1.md`** — this file; HEAD's accumulation and main's
   62421b89 record BOTH kept verbatim under lineage headings; this record
   appended.

`effects/filter.go` auto-merged cleanly (main's pc1 predicate work vs this
branch's `nonCopiedSpell` fix touch different regions). No golden, ratchet
table or `heads_test.go` was edited.

### Commands and output (measured)

- `.cards` present (real symlink to `/home/sadams/projects/gorge/.cards`) —
  runs are real, not vacuous skips.
- Merged-table count (the test's own awk logic): `rows: 46`.
- `go test ./internal/testutil/ -run 'TestKnownApproximation' -v` →
  `--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)`,
  `--- PASS: TestKnownApproximationRowsAreShort (0.00s)`,
  `ok github.com/adams-shaun/gorge/internal/testutil 0.001s`.
- Ratchets: `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  → `ok github.com/adams-shaun/gorge/rules 0.755s`; spot-verbose rerun of
  `TestEveryRepoDeckParamsAreRead` shows `--- PASS (0.71s)`, no SKIP.
- Branch's non<X> suites: `go test ./effects -run 'NonPredicate|NonCopied|NonColorless|NonChosen'`
  → `ok ... 0.619s`.
- Main's new closures' suites: `go test ./rules -run 'EachDamage|ReplacementOrder|Legend|PlayerSpec|Imprinted|ContextWord|BangPredicate|ReplacementUpdated|TokenReplaceChosen|FailClosed'`
  → `ok ... 1.182s`; `go test ./effects -run 'EachDamage|FailClosed|Imprinted|ContextWord|BangPredicate|Condition'`
  → `ok ... 1.266s`.
- Behaviour goldens: `go test ./internal/archtest/` → `ok ... 3.937s`;
  `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` →
  `ok ... 0.972s` (pinned 20-game bot split did NOT move).
- `gofmt -l internal/testutil/agentsdoc_test.go` → clean.
- `git status` after the merge commit: clean; `main` (`e6a2a84d`) is now an
  ancestor of HEAD (merge commit `c53b5e1f`, parents `d396b8ba` + `e6a2a84d`).

### Notes / uncertainties

- The branch registers no new `Mode$` matcher and closed no
  `knownUnsupported`/`knownUnsupportedParams`/`knownUnmodelledCountHeads`
  entry; main's closures came with their own table edits, auto-merged.
- One residual recorded OUTSIDE the table (register is delete-only, a row
  may never be re-added narrowed): main's deleted `non<X>` row named
  `nonColorless` and `nonChosenCard` as still-unmatched shapes. They remain
  unmatched in the merged tree (only `nonCopiedSpell` was closed, by
  `d56e404f`). Someone re-book them if a ticket takes them.

## Issues

None found during this integration round beyond the resolved conflicts.

---

## Record — current integration round (cli-20260922T225143Z-bc326d39)

### Starting state and operation

The worktree was clean at `d82018e4`; no rebase or merge was in flight. The prior merge commit `f89a62fd` did not contain the latest `main` tip, so I ran `git merge main`. It stopped with conflicts in `.ds4/report-mrg1.md` and `internal/testutil/agentsdoc_test.go`; `AGENTS.md` and the other changed paths merged automatically.

### Conflicts and resolution

1. **`internal/testutil/agentsdoc_test.go`:** branch side had `knownApproximationRows = 45`; main side had `46` and explanatory history. Both reflect earlier table snapshots. The merged `AGENTS.md` table currently counts **44 data rows** using the test's counting rules (45 `| ` lines including the header). Set the constant to 44, keeping the delete-only ratchet aligned to the actual merged table. No table rows were changed manually during this resolution.
2. **`.ds4/report-mrg1.md`:** the branch side held this ticket's concise integration report; main held accumulated lineage and prior-round reports. Kept both sides' text and removed only conflict markers, so neither historical report was discarded. Appended this round's resolution record here.

### Commands and outputs

- `git status --short --branch && git rev-parse --show-toplevel && git log -1 --oneline --decorate` → clean branch at `d82018e4`, no in-flight operation.
- `git merge main` → conflicts in `.ds4/report-mrg1.md` and `internal/testutil/agentsdoc_test.go`.
- `ls .cards | head` → `cards.lock`, `cardsfolder`, `ir.gob.gz`, `ir.v4.gob.gz`, `tokenscripts` (corpus present).
- `awk '/^## Known approximations/{inside=1; next} inside&&/^## /{inside=0} inside&&/^\\| /{c++} END{print "table rows including header:",c,"data rows:",c-1}' AGENTS.md` → `table rows including header: 45 data rows: 44`.
- `go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'` → `ok github.com/adams-shaun/gorge/internal/testutil 0.002s`.
- Required post-merge ratchets, `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'` → `ok github.com/adams-shaun/gorge/rules 0.823s`.
- No engine source or behavior was changed in this integration round.

### Uncertainty

None about the resolution. The count in the inherited main-side comment was stale relative to the fully auto-merged current table; the measured 44 is authoritative.

The first `git add .ds4/report-mrg1.md ...` attempt refused because `.ds4` is ignored; `git ls-files -v .ds4/report-mrg1.md` confirmed it is a tracked path (`H`), so I staged it with `git add -f` (and staged `internal/testutil/agentsdoc_test.go` normally).

`GIT_EDITOR=true git merge --continue` completed successfully:

```text
[wt/cli-20260922T225143Z-bc326d39 7a5b5423] Merge branch 'main' into wt/cli-20260922T225143Z-bc326d39
```

I then committed the final report update as `09e5711e` (`docs(merge): record latest mrg1 integration`). Final `git status --short --branch` showed only the branch header, with no changes; `git merge-base --is-ancestor main HEAD` succeeded.

---

## Preserved main-side resolver report — cli-20260922T225140Z-c661fa12

---

# Round 2 — merge main @ e0fd7d9c into wt/cli-20260922T225141Z-1b1182e4

## Starting state

`git status` found the tree CLEAN — no rebase or merge in flight (the daemon had
aborted its failed attempts before dispatching this seat). The branch's round-1
merge (af2f1648) had integrated main @ e6a2a84d; main had since advanced to
e0fd7d9c (battle-protector botpolicy 57cd1863/a04571b1, NameCard lists
9aee8b00, nonCopiedSpell d56e404f, plus their merge/doc commits).

## Operation

`git merge main` conflicted on two files:

1. `internal/testutil/agentsdoc_test.go` — HEAD had removed main's provenance
   comment above `knownApproximationRows`; main kept/extended it (both sides
   carried the constant 46). **Resolution:** kept main's comment, rewritten to
   the merged measurement, and lowered the constant to **44** — the merged
   AGENTS.md table measures 44 data rows (base e6a2a84d = 47; this branch
   deleted the First-Strike Damage row [acc7878d], main deleted the NameCard
   row [9aee8b00] and the non<X> row [d56e404f]). `knownOversizeRows = 8`
   kept: the merged table measures 5 oversize rows, still under the cap.
2. `.ds4/report-mrg1.md` — HEAD's side is this ticket's round-1 report (68
   lines); main's side is its own verbatim lineage-record pile (1141 lines,
   which does not contain this ticket's record). **Resolution:** both kept
   verbatim — this ticket's round-1 record first, then main's lineage records
   under a separator heading, then this round-2 record.

`AGENTS.md` itself auto-merged cleanly (union of the three row deletions).

## Issues

None beyond the resolved conflicts.

## Round-3 record — re-dispatch verification (this seat, 2026-09-23)

The round-2 resolver was cut off after committing the merge but before its
completion status was recorded (`status-merge-mrg1.cutoff.json` shows
`status: DONE`, `report_written: false`). This re-dispatch verified the round-2
state rather than redoing it:

- `git status` on entry: tree CLEAN at `1249aca0`, no rebase/merge in flight;
  `acc7878d` (the branch fix) and main @ `e0fd7d9c` both contained; no conflict
  markers anywhere in tracked files.
- The round-2 record above (Operation + Issues, ending "None beyond the
  resolved conflicts") documents the `1249aca0` resolution; `AGENTS.md`
  auto-merged with the union of the three row deletions and no
  First-Strike/NameCard/non<X> rows remain (grep: only the provenance comment
  mentions the closure in `internal/testutil/agentsdoc_test.go`).
- Ratchet re-run (same commands the round-2 brief mandates, all exit 0):
  `go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'` → ok 0.001s;
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'` → ok 0.886s.
  `.cards` symlink present (→ /home/sadams/projects/gorge/.cards), so the run
  is not a vacuous skip.
- Behaviour goldens: `go test ./internal/archtest/` → ok 5.233s;
  `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` → ok
  1.310s (pinned split unchanged). `gofmt -l` on the conflicted file: clean.

Note for the next round: main has since advanced past `e0fd7d9c` to `2040e5d9`
(kw:Strive, the Worthy filter predicate, layer-4 static-grammar closure, a CI
docs refresh). That integration is NOT part of this round's mrg1 conflict and
was not started here; the daemon should dispatch it as its own merge round.

## Issues

None new. The round-2 record's `## Issues` stands (none beyond the resolved
conflicts).


---

## Record — main-side report kept verbatim (main @ 1fc5b37c, task cli-20260922T225140Z-c661fa12)

# Merge-conflict resolution — task cli-20260922T225140Z-c661fa12

## Situation found

`git status` was CLEAN on entry (no rebase/merge in flight in this worktree;
the daemon's rebase attempt had been aborted), and the branch tip
`47b865e8` already merged main `e6a2a84d` (the PRIOR round's work, recorded in
the report this file carried). But main had advanced again to `e0fd7d9c`:

- `57cd1863`/`a04571b1`/`f76f59fd` — the battle-protector ticket (bot arm,
  re-derive, `botpolicy/protector_test.go`);
- `9aee8b00`/`78d3b764` — the NameCard ChooseFromList$/AtRandom$ closure;
- `d56e404f`/`e0fd7d9c` — the non<X> negation closure (nonColorless,
  nonChosenCard, nonCopiedSpell now expressed by the generic negation);
- plus its own main-merge commits (`23400548` etc).

So the owed integration was a merge of main `e0fd7d9c` into the branch.
Rebase is forbidden in a seat; the same `git merge main` method as prior
rounds was used.

## Conflicted files and resolutions

`git merge main` conflicted on exactly three files; all code auto-merged.

### 1. `AGENTS.md` (one hunk, lines 222-226)

- HEAD (branch) side: the `non<X>` negation row (still present on the branch,
  whose base merged main only up to `e6a2a84d` — before the non<X> closure).
- main side: the layer-4 row, which main still carries because the layer-4
  fix `63c07260` is THIS branch's reviewed commit, never merged to main.
- Both rows are CLOSED by commits on the two sides: layer-4 by this branch's
  fix, non<X> by main's `d56e404f`. **Resolution: delete BOTH rows** — which
  is exactly the union of the two sides' intents (each side deleted its own
  closed row; the merge should keep neither).

Verified against the measured tables: main's table is 45 rows (its
closures + its own added rows), HEAD's is 46; the merged table is main's 45
minus the layer-4 row = **44 rows** (`awk` over `## Known approximations` …
next `##`, `| ` lines minus header). Both deleted rows are absent from the
merged file; nothing else in the table changed relative to main
(`diff <(git show main:AGENTS.md | table-extract) <(table-extract AGENTS.md)`
shows exactly the one deleted row).

### 2. `internal/testutil/agentsdoc_test.go` (one hunk, comment only)

Both sides already agreed on the VALUE `knownApproximationRows = 46`; the
conflict was only in the history comment above it (main's side carried its
merge-round comment; the branch's side had a bare constant). **Resolution:**
kept main's commented shape, rewritten to describe THIS merge accurately,
and lowered the constant to the measured **44**:

    // The merged table measures 44 rows: main's closures merged here
    // (non<X> via d56e404f, NameCard ChooseFromList$/AtRandom$ via 78d3b764,
    // the battle protector row via f76f59fd, and the earlier pc1/each1/CR
    // 616.1 closures), plus this branch's own layer-4 filter-grammar closure
    // (fix 63c07260), which deletes main's last remaining layer-4 row.
    knownApproximationRows = 44

The test fails only on growth, but the constant should carry the measured
count (same method as main's precedent `4dcef2ea` and the prior round).

### 3. `.ds4/report-mrg1.md` (wholesale)

Both sides carry prior resolver rounds' reports (branch: the previous
integration round; main: another ticket's merge report). Neither is source
of truth. **Resolution: replaced wholesale with THIS round's report.**

## Auto-merged files — both sides' intent verified

- `effects/filter.go` — the branch's `SpecContext.DerivedTypes` layer-4
  plumbing (lines ~3193-3279) and main's non<X> generic-negation work
  (wordColorless/wordCopiedSpell, `nonCopiedSpell` comments at 1058/1377)
  compose; `go build ./effects/ ./rules/ ./internal/testutil/` clean.
- `rules/engine.go`, `rules/cast.go`, `rules/replacement.go` — main's
  battle-protector and NameCard work; no branch-side contradiction (the
  branch's fix touched layers/statics/clone/filter only).
- `botpolicy/policy.go` + `botpolicy/protector_test.go` — main's new files,
  added cleanly.
- `AGENTS.md`'s other hunks — main's row deletions (NameCard row,
  battle-protector row) applied by auto-merge; nothing reintroduced.

## Commands run and output

- `.cards` check: **present** (real symlink → `/home/sadams/projects/gorge/.cards`),
  so the corpus-backed ratchets were real, not vacuous skips.
- `git merge main` → 3 content conflicts (`AGENTS.md`,
  `internal/testutil/agentsdoc_test.go`, `.ds4/report-mrg1.md`), 17 files
  total in the merge.
- `go test ./internal/testutil/ -run 'TestKnownApproximations' -v`
  → `--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)` / `ok 0.001s`.
- Post-merge ratchets:
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|TestLayer4'`
  → `ok github.com/adams-shaun/gorge/rules 1.025s`.
- main's closures on the merged tree:
  `go test ./effects -run 'TestPlayerSpecFx20Grammar|TestUnknownPredicates|NameCard|NonPredicateStack'`
  → `ok github.com/adams-shaun/gorge/effects 0.008s`.
- main's bot-policy addition: `go test ./botpolicy ./internal/archtest/`
  → `ok ... botpolicy 0.719s` / `ok ... internal/archtest 3.846s`.
- `gofmt -l internal/testutil/agentsdoc_test.go` → clean.
- `go build ./effects/ ./rules/ ./internal/testutil/` → clean.

## Uncertainties / notes

- The dual-row deletion in `AGENTS.md` is the one judgment call: each side
  deleted a DIFFERENT row, so the merged table keeps neither. This is the
  union of the two reviewed intents, not a redesign.
- No engine behaviour beyond the branch's reviewed fix `63c07260` was
  introduced by the resolution itself; the resolution touched `AGENTS.md`,
  the register constant + its comment, and this report.
- `git diff --stat main` post-merge shows only the branch's own fix files
  plus the constant/report, confirming main's content came through intact.

## Issues

None found during this round's resolution.

# Round 4 — merge main @ 1fc5b37c into wt/cli-20260922T225141Z-1b1182e4 (2026-09-23)

## Situation found

`git status` on entry: tree CLEAN at `af871344`, no rebase/merge in flight (the
daemon had aborted its failed attempts before dispatching this seat). The
branch's round-3 tip had integrated main @ `e0fd7d9c`; main has since advanced
to `1fc5b37c`, carrying:

- `63c07260`/`463416fd` — the layer-4 filter-grammar fix (its own row closure);
- `9fe4eaf1`/`f545733a` — the Worthy filter predicate;
- `5d8ef1d1`/`2b0f723f` — kw:Strive per-extra-target additional cost;
- `6d0a209a`/`23078d5c` — trig:BecomeMonstrous + `Monstrosity$`;
- `599b3a92`/`1fc5b37c` — stat:Panharmonicon.ValidTurned + trig:TurnFaceUp;
- `77a84b3c`/`2040e5d9` — the coverage-table/CI docs refresh.

Rebase is forbidden in a seat (and the daemon's own rebase attempt conflicted on
`acc7878d` and was aborted); the same `git merge main` method as prior rounds
was used.

## Operation

`git merge main --no-edit` conflicted on exactly two files; all other code
auto-merged cleanly.

### 1. `internal/testutil/agentsdoc_test.go` (comment + constant)

Both sides agreed the constant had moved off the old `46`-row base; neither
side's number was the true merged value.

- HEAD (branch, `af871344`) said 44 with a comment describing the First-Strike
  closure this branch carries;
- main said 44 with a comment describing the layer-4 closure main carries;
- Measured base `e0fd7d9c` = 45 rows; main = 44 (deleted the layer-4 row);
  branch = 44 (deleted the First-Strike row); each side deleted a DIFFERENT
  row, so the merged table keeps NEITHER deletion as a conflict — the
  auto-merged `AGENTS.md` measures **43** data rows.

**Resolution:** set `knownApproximationRows = 43` (the measured merged count)
and replaced both comments with one that names both sides' distinct deletions
(this branch's First-Strike closure `acc7878d` + main's layer-4 closure
`63c07260`, plus main's other merged closures). Verified both deleted rows are
absent from the merged `AGENTS.md`.

### 2. `.ds4/report-mrg1.md` (archive collision)

This file is a force-added lineage archive shared across tickets by filename
(`.ds4/report-mrg1.md`), so main's copy is a *different* ticket's resolver
report (`cli-20260922T225140Z-c661fa12`) while HEAD's copy is this ticket's
accumulated archive. **Resolution: keep BOTH** — HEAD's 1284-line archive
verbatim, then main's 115-line report appended under a clearly marked
`## Record — main-side report kept verbatim` section. No content discarded.

## Commands and output

`git status --short --branch` on entry:

```text
## wt/cli-20260922T225141Z-1b1182e4
```

`git merge main --no-edit`:

```text
Auto-merging .ds4/report-mrg1.md
CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
Auto-merging AGENTS.md
Auto-merging internal/testutil/agentsdoc_test.go
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
Automatic merge failed; fix conflicts and then commit the result.
```

Table counts (`git show <ref>:AGENTS.md`, `| ` lines under `## Known
approximations` minus header): base `e0fd7d9c` = 45, main = 44, branch
`af871344` = 44, merged working tree = 43.

`go test ./internal/testutil/ -run 'TestKnownApproximation' -v`:

```text
=== RUN   TestKnownApproximationsOnlyShrinks
--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)
=== RUN   TestKnownApproximationRowsAreShort
--- PASS: TestKnownApproximationRowsAreShort (0.00s)
PASS
ok  	github.com/adams-shaun/gorge/internal/testutil	0.004s
```

`go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`:

```text
ok  	github.com/adams-shaun/gorge/rules	1.860s
```

`.cards` symlink present (→ /home/sadams/projects/gorge/.cards), so the runs are
not vacuous skips.

## Issues

None new. No engine behaviour was changed by the resolution: it touched the
registers constant + comment, the archive report, and nothing else.


---

## Record — main-side report kept verbatim (main @ 1be022eb, task cli-20260922T225141Z-48972afc)

# Merge-conflict resolution — task cli-20260922T225141Z-48972afc

## Situation found

`git status` was CLEAN on entry — no rebase or merge in flight (the daemon's
earlier attempt had left nothing behind). The branch tip was `7c4182ff`
("fix(rules): resolve mulligan redraws at the end of the declaration pass"),
one commit past the merge base `0104252d`; main had advanced to `2040e5d9`,
39 commits ahead. The owed integration was a merge of main into the branch
(rebase is forbidden in a seat).

`.cards` was **present** — a real symlink to
`/home/sadams/projects/gorge/.cards`, target exists — so every corpus-backed
ratchet below ran for real rather than skipping.

## Conflicted files and resolutions

`git merge main` produced **one** content conflict: `internal/testutil/agentsdoc_test.go`.
`AGENTS.md` auto-merged (it carried no textual conflict — each side had deleted
a different row).

### `internal/testutil/agentsdoc_test.go` (conflict: the `knownApproximationRows` constant)

Both sides changed the register constant:

- **HEAD (branch):** `knownApproximationRows = 49`. The branch base's table was
  50 rows; the mulligan ticket deleted one row and lowered 50 → 49.
- **main:** `knownApproximationRows = 44`, with a comment recording main's own
  closures (non<X> `d56e404f`, NameCard ChooseFromList$/AtRandom$ `78d3b764`,
  the battle protector row `f76f59fd`, plus pc1/each1/CR 616.1) and the
  layer-4 closure from **another** merge of this same branch (`63c07260`),
  which main had already absorbed.

Neither number is the merged count. The merged table was **measured** (same
method the test uses: lines starting `"| "` inside `## Known approximations …
next "## "`, minus the header): **43 data rows**. HEAD carried 48, main 44,
and the union of both sides' deletions (mulligan row on the branch; main's
four) gives 43. Verified the four closed rows are absent from the merged
`AGENTS.md`: the old mulligan row (`mulligan declaration's REDRAW`), the
`non<X> negation` row, the `layer-4 type grants reach only` row and the
`NameCard asks over` row — all grep to 0 occurrences.

**Resolution:** kept main's commented shape (which explains the other
closures) and wrote the merged count accurately:

    // The merged table measures 43 rows: main's closures merged here
    // (non<X> via d56e404f, NameCard ChooseFromList$/AtRandom$ via 78d3b764,
    // the battle protector row via f76f59fd, and the earlier pc1/each1/CR
    // 616.1 closures), plus this branch's own mulligan-redraw deferral
    // (fix 7c4182ff), which deletes the "mulligan declaration's REDRAW
    // resolves immediately" row.
    knownApproximationRows = 43

This is the union of the two sides' intents, not a redesign. The row-count
gate passes (`TestKnownApproximationsOnlyShrinks`, `TestKnownApproximationRowsAreShort`
— the package test is green).

## Integration failures found after the merge — and why they are part of resolving it

The merge itself is a **semantic** integration conflict, not just a textual
one. main added a whole feature after this branch forked — the hypothetical
chance planner (`rules/chance.go`, `rules/chance_test.go`) — and three of its
tests observe a mulligan's REDRAW inside the mulligan submit, because on main
`handleMulligan` still resolves the redraw immediately. This branch's reviewed
fix deliberately *defers* the redraw to the declaration-pass boundary (CR
103.4/103.5, `resolveMulliganRedraws`), which is the entire purpose of the
ticket. Both sides kept, so main's three tests now observe the wrong moment.

The three failures, measured on the merged tree before the fix below:

    --- FAIL: TestHypotheticalPlannerControlsMulliganShuffle (0.00s)
        chance_test.go:223: mulligan shuffle=[9 2 11 6 5 12 10 3 4 7 1 8] want []
    --- FAIL: TestHypotheticalReplayAndCloneOwnChanceState (0.00s)
        chance_test.go:448: mulligan did not consume chance
    --- FAIL: TestHypotheticalSubmitFailurePoisonsOnlyThatBranch (0.00s)
        chance_test.go:504: error = <nil>

I confirmed the underlying behaviour is intact by instrumenting a scratch test:
after the mulliganing seat declares and the **rest of the pass keeps**, the
planner's Ordinal-1 callback fires with exactly the context main's test
expects (`hand=0 lib=12`) and produces the planned permutation. Only the
*when* moved (from the submit to the pass boundary), exactly as CR 103.4/103.5
requires.

`rules/chance_test.go` was NOT a conflicted file. This is a documented
deviation from "do not touch files the conflict does not involve": leaving it
red would wedge the daemon's full gate, and reverting the branch fix would
destroy the ticket. The integration change is test-only and preserves each
test's intent:

- **TestHypotheticalPlannerControlsMulliganShuffle** — records the mulliganing
  seat, then drives the rest of the pass with a keep (new `submitPregameKeep`
  helper) until the planner callback has run, then asserts `lastShuffle` equals
  the planned order. The planner still controls the mulligan shuffle.
- **TestHypotheticalReplayAndCloneOwnChanceState** — after the clone's mulligan
  submit, asserts the source engine is untouched, then completes the clone's
  pass and only then asserts chance was consumed; completes `e`'s and the
  replay's pass identically before the transcript comparison. The clone/chance
  ownership contract is unchanged.
- **TestHypotheticalSubmitFailurePoisonsOnlyThatBranch** — the mulligan submit
  now succeeds (the poisoned draw is consumed by the deferred redraw); the
  pass-completing keep is the call that surfaces the `bound` chance failure;
  the poison/head-immutability and base-vs-branch assertions follow. The
  chance-failure boundary contract is unchanged.

A new helper `submitPregameKeep` documents the deferral in one place.

## Commits produced

- `61779fd9` — `Merge branch 'main' into wt/cli-20260922T225141Z-48972afc`
  (default merge message; parents `7c4182ff` + `2040e5d9`). Contains the
  `agentsdoc_test.go` resolution.
- `f0bc40a3` — `test(rules): drive the deferred mulligan redraw in the chance
  planner tests` (the `rules/chance_test.go` integration; no engine change).

## Commands run and output (real)

`.cards`: **present** (real symlink, target exists) — corpus tests ran.

    $ git merge main
    Auto-merging AGENTS.md
    Auto-merging internal/testutil/agentsdoc_test.go
    CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go

Merged-row measurement (test's own method):

    raw rows: 44 => data rows: 43
    HEAD (branch) table: 48 rows; main table: 44 rows; merged: 43 rows

Conflict-resolution gate — the conflicted file's package:

    $ go test -run 'TestKnownApproximation' ./internal/testutil/
    ok  	github.com/adams-shaun/gorge/internal/testutil	0.001s
    $ go test ./internal/testutil/
    ok  	github.com/adams-shaun/gorge/internal/testutil	1.475s

The three main-planner tests, before and after the integration fix:

    before: 3 FAIL (above)
    $ go test -run 'TestHypothetical' ./rules/ -v
    --- PASS: TestHypotheticalPlannerControlsMulliganShuffle (0.00s)
    --- PASS: TestHypotheticalReplayAndCloneOwnChanceState (0.00s)
    --- PASS: TestHypotheticalSubmitFailurePoisonsOnlyThatBranch (0.00s)
    (... every other TestHypothetical* PASS)
    ok  	github.com/adams-shaun/gorge/rules	0.007s

Post-merge ratchets (the command the brief names):

    $ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
    ok  	github.com/adams-shaun/gorge/rules	0.753s

Broader targeted mulligan/pregame suite in `rules/`:

    $ go test ./rules -run 'Mulligan|Pregame|Bottoming|Hypothetical|Redraw|StartingPlayer|Kept|Keep'
    ok  	github.com/adams-shaun/gorge/rules	0.770s

Mandatory goldens outside `rules/` (per system context):

    $ go test ./internal/archtest/
    ok  	github.com/adams-shaun/gorge/internal/archtest	3.013s
    $ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
    ok  	github.com/adams-shaun/gorge/cmd/botbench	1.101s

Format / build:

    $ gofmt -l internal/testutil/agentsdoc_test.go rules/chance_test.go
    (no output)
    $ go run ./cmd/gentypes -check
    (no output)
    $ go build ./rules/ ./internal/testutil/
    (clean)

Final state:

    $ git status --short          → (empty, clean)
    $ grep -rn '^<<<<<<<|^>>>>>>>' → (no conflict markers)

## TestHeads — expected branch movement, deliberately NOT edited

    $ go test -run 'TestHeads$' ./rules/
    --- FAIL: TestHeads (1.71s)
        heads_test.go:1323: 4 seats: chain head acce7d850cfb176a, golden 20028059e8c88ec3
        heads_test.go:1323: 6 seats: chain head 3bd695df72d9d4c9, golden 400d8d9ae2777ded
        heads_test.go:1323: 8 seats: chain head 5c90b1b3a0b38f25, golden 5e988854bf022347

These are **exactly** the values the branch's own ticket measured and
attributed (`acce7d850cfb176a` / `3bd695df72d9d4c9` / `5c90b1b3a0b38f25`; 2
seats unmoved at `19a4893657e5d549`). My resolution introduced no engine
behaviour change, so nothing from main moved them further. Per the system
context, `rules/heads_test.go` is not edited by agents — the orchestrator
re-pins it at the gate from measured values (FL-107), and the ticket's report
(`.ds4/report-t1.md`) already carries the attribution. NOT a blocker for this
resolution.

## Uncertainties / notes

- The `rules/chance_test.go` edit is the one deliberate scope extension, forced
  by main's new feature observing the behaviour this ticket changes. It is
  test-only; the alternative (leave 3 tests red, or revert the reviewed fix)
  both fail the brief.
- The `knownApproximationRows` number is the measured merged count (43), not
  either side's stale constant; I verified the four closed rows are gone.

## Issues

- No new defects found beyond the integration above.
- Coverage note (already recorded by the ticket): the repo-deck acceptance
  suite runs exactly one mulligan per game and at 2 seats that mulliganer is
  the last declarer, so the 2-seat acceptance golden does not exercise the
  deferred-redraw interleaving at all; the branch's
  `rules/mulligan_redraw_order_test.go` covers the multi-declarer shapes
  directly.
- Out of scope and untouched: the adjacent AGENTS.md row "The starting player
  is uniformly random but the toss winner never CHOOSES" remains in the table.

---

## Current integration — main 2040e5d9

### Starting state and operation

`git status --short --branch` initially showed a clean branch at `bb5461ee`, with no operation in flight. The prior integration was recorded at `7a5b5423`, but main had advanced to `2040e5d9`. `git merge main` reproduced conflicts in `.ds4/report-mrg1.md` and `internal/testutil/agentsdoc_test.go`; `AGENTS.md` and `rules/cast.go` auto-merged.

### Conflicts and resolution

- `internal/testutil/agentsdoc_test.go`: both sides' constant values were stale for the combined auto-merged table. Measured the merged table at 43 data rows and set `knownApproximationRows = 43`; the table includes both main's and this branch's row closures.
- `.ds4/report-mrg1.md`: retained the complete branch-side report history and appended main's complete 115-line report under a lineage heading, followed by this record. No report history was discarded.
- `AGENTS.md`: auto-merged. Main's changes and the branch's reviewed damage-cost LKI changes are retained.

### Commands and output

- `git status --short --branch` → `## wt/cli-20260922T225143Z-bc326d39` (clean before merge).
- `git merge main` → conflicts in `.ds4/report-mrg1.md` and `internal/testutil/agentsdoc_test.go`; auto-merged `AGENTS.md` and `rules/cast.go`.
- Table measurement: 44 `| ` lines including header, hence 43 data rows; constant is 43.
- `.cards` check → `.cards present`.
- `go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' -v`:
  ```
  === RUN   TestKnownApproximationsOnlyShrinks
  --- PASS: TestKnownApproximationsOnlyShrinks (0.00s)
  === RUN   TestKnownApproximationRowsAreShort
  --- PASS: TestKnownApproximationRowsAreShort (0.00s)
  PASS
  ok github.com/adams-shaun/gorge/internal/testutil 0.002s
  ```
- Required merged-tree ratchets, `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'` → `ok github.com/adams-shaun/gorge/rules 0.873s`.
- `git diff --check` → no output; conflict-marker grep → no matches.

### Issues / uncertainty

No conflict-resolution uncertainty identified. No additional approximation row or behavior change was introduced by resolution.

---

## Record — current integration round (cli-20260922T225143Z-bc326d39)

### Starting state and operation

The worktree was CLEAN at `7e0ca3a2`, the branch's previous merge of main
`2040e5d9`; no rebase or merge was in flight (the daemon's rebase and its
merge fallback had both been rolled back). `main` had advanced past
`2040e5d9` to `1be022eb` (the cli-20260922T225141Z-48972afc mulligan-redraw
deferral landing: `7c4182ff` + `f0bc40a3` + `502298cb`, plus the
Panharmonicon/`trig:TurnFaceUp` and `trig:BecomeMonstrous` merges and the
heads re-pin `408fdf32`). Rebase is forbidden in this seat, so I re-integrated
with `git merge main`.

### Conflicts and resolution

1. **`internal/testutil/agentsdoc_test.go`** — only the `knownApproximationRows`
   comment/constant. HEAD side carried 43 (its own closures); main side
   carried 43 (its mulligan-redraw closure) with a longer comment. Both
   values were stale for the auto-merged table: the two sides deleted
   DISJOINT rows, so the merge keeps both closures —
   - this branch deleted the `kw:Infect` row (fix `56f98b13`, which reads
     CR 113.7a last-known characteristics for damage cost keywords at the two
     cost sites), and
   - main deleted the "mulligan declaration's REDRAW resolves immediately"
     row (fix `7c4182ff`).
   Base `2040e5d9` measured 44 data rows; `44 - 2 = 42`. The merged `AGENTS.md`
   table measures exactly **42** data rows (the test's own
   `approximationRows` logic), and both deleted rows are absent
   (`grep -c 'kw:Infect'` → 0; `grep -c "mulligan declaration's REDRAW
   resolves immediately"` → 0). Resolved to `knownApproximationRows = 42`
   with a comment naming both closures.
2. **`.ds4/report-mrg1.md`** — the shared accumulated report. HEAD's
   full report history (1336 lines) and main's 48972afc report (214 lines)
   were the two sides of a single large conflict region; kept BOTH verbatim
   under their headings and dropped only the conflict markers.

`AGENTS.md` auto-merged (both disjoint row deletions retained). No engine code
conflicted: main's `rules/mulligan.go`, `rules/statics.go`,
`rules/trigmatch_faceup.go`, `events/*`, `effects/misc.go`, `view/describe.go`
and this branch's `rules/cast.go` / `rules/resolution.go` damage-cost LKI
changes merged cleanly (disjoint files/regions).

### Commands and output

- `.cards` present (real symlink to `/home/sadams/projects/gorge/.cards`) —
  runs are real, not vacuous skips.
- `git merge main --no-edit` →
  ```

---

# Round 5 — second integration, main @ 1be022eb (2026-09-23)

## Why a second merge in the same session

The round-4 merge integrated main @ `1fc5b37c` and completed as `84b39833`.
While that work was in progress, the shared `main` ref advanced again
(`git reflog main`): another resolver merged ticket
`cli-20260922T225141Z-48972afc` (the mulligan-redraw deferral, commit
`7c4182ff` + its test/heads/docs follow-ups) into main, moving it to
`1be022eb`. `1fc5b37c` is still an ancestor of `84b39833`, but `main` no longer
was, so a second `git merge main` was required to leave the branch actually
containing current main (otherwise the daemon's gate-and-merge path would
immediately re-conflict and re-dispatch, as the round-1..3 history shows).

## Operation

`git merge main --no-edit` conflicted on the same two files as round 4; every
other file auto-merged (including `rules/heads_test.go`, whose new goldens came
from the mulligan ticket).

### 1. `internal/testutil/agentsdoc_test.go`

Main @ `1be022eb` deletes one further row (the "mulligan declaration's REDRAW
resolves immediately" row, closed by `7c4182ff`). The merged table now keeps
ALL distinct deletions from both lineages — First-Strike (this branch,
`acc7878d`), layer-4 (`63c07260`), mulligan-redraw (`7c4182ff`), non<X>
(`d56e404f`), NameCard (`78d3b764`), battle protector (`f76f59fd`) and the
pc1/each1/CR 616.1 closures — and measures **42** data rows. Resolved
`knownApproximationRows = 42` with a comment listing every closure; verified
the First-Strike, layer-4 and mulligan row texts are all absent from AGENTS.md.

### 2. `.ds4/report-mrg1.md`

Same archive collision as round 4: main @ `1be022eb` carries ticket
`48972afc`'s resolver report for this shared filename. **Resolution: keep
BOTH** — the `84b39833` archive verbatim, then main's 48972afc report appended
under a marked `## Record — main-side report kept verbatim` section.

## Commands and output

Table counts (`| ` lines under `## Known approximations`, minus header):
`1fc5b37c` = 44, `1be022eb` = 43, merged = 42.

`go test ./internal/testutil/ -run 'TestKnownApproximation'`: see round-6
verification below (same command, re-run post-second-merge).

## Issues

None new. No engine behaviour changed by the resolution itself.

## Round-5 verification (post-second-merge, all exit 0)

Merged as `107b6ee5` (parents `84b39833`, main `1be022eb`); main is now an
ancestor of HEAD and the tree is clean.

```text
go test ./internal/testutil/ -run 'TestKnownApproximation'          ok
go test -count=1 ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'  ok
go test -count=1 ./rules -run 'TestHeads'                           ok
go test ./internal/archtest/                                        ok
go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/  ok (pinned split unchanged)
gofmt -l internal/testutil/agentsdoc_test.go                        clean
```

`.cards` symlink present (→ /home/sadams/projects/gorge/.cards); no vacuous
skips. `knownApproximationRows = 42` (measured 42 data rows).

## Issues

None new across either integration. This resolution changed no engine
behaviour: the only non-archive edit was the row-register constant and its
comment in `internal/testutil/agentsdoc_test.go`.


---

# Current integration round — main at edc24484

Starting state: the worktree was clean at `c87492a8`; no rebase or merge was in progress. `main` had advanced from the integrated `1be022eb` to `edc24484`. `git merge main --no-edit` conflicted in `.ds4/report-mrg1.md`, `effects/misc.go`, `internal/testutil/agentsdoc_test.go`, and `rules/combat.go`. The report conflict was resolved by preserving both index-stage versions verbatim in this archive, followed by this round's record.

### Resolution

- `effects/misc.go`: retained both effect-delivered restriction modes, `CantBlockUnless` from the reviewed branch and `MustBlock` from main.
- `internal/testutil/agentsdoc_test.go`: set `knownApproximationRows` to 42, matching the merged `AGENTS.md` table (measured 42 data rows). This preserves the branch's `(blockprop1)` row deletion and main's First-Strike Damage deletion, along with the previously merged closures.
- `rules/combat.go`: composed both changes. Kept main's shared blocker-pair scope, MustBlock candidate/team quota and Menace minimum; retained the branch's full composite-charge affordability gate and exposed mana, life and tap costs on options. Mana remains the option `Value` for `MaxSum`; Menace minimum is applied to required options. No side's feature was discarded.
- `.ds4/report-mrg1.md`: both accumulated report histories were retained; this round's report follows them.
- `AGENTS.md` and other main files auto-merged. `AGENTS.md` has 42 approximation rows; `(blockprop1)` and First-Strike row are absent.

### Commands and output

- `git status --short --branch` on arrival: `## wt/cli-20260922T225142Z-1d4558a1`, clean.
- `git merge main --no-edit`: conflicts as listed above; all other changes auto-merged.
- `.cards`: present, resolves to `/home/sadams/projects/gorge/.cards`.
- `git diff --check`: initially reported an extra blank line at end of the report archive; trimmed while adding this record.
- Conflict-marker scan of resolved source files: no markers.

Tests and final status are recorded after resolution below.

### Issues

No new engine issue was identified during integration. The MustBlock feature and composite CantBlockUnless costs were both retained; no behavior outside the conflict was intentionally changed.

Post-resolution checks (all exit 0):

```text
go test -run 'Block|MustBlock|CantBlockUnless' ./rules/
ok   github.com/adams-shaun/gorge/rules  0.818s

go test ./effects -run 'Effect|AnimateStatic'
ok   github.com/adams-shaun/gorge/effects  0.622s

go test ./decision -run 'Block|Required'
ok   github.com/adams-shaun/gorge/decision  0.011s

go test ./internal/testutil/ -run 'TestKnownApproximation'
ok   github.com/adams-shaun/gorge/internal/testutil  0.001s

go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok   github.com/adams-shaun/gorge/rules  0.804s

go test -run 'TestHeads$' ./rules/
ok   github.com/adams-shaun/gorge/rules  2.226s

go test ./internal/archtest/
ok   github.com/adams-shaun/gorge/internal/archtest  3.715s

go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok   github.com/adams-shaun/gorge/cmd/botbench  1.180s
```

The repository-deck ratchets ran with `.cards` present (not skipped). `TestHeads` and the pinned botbench split pass. One compile error was caught on the initial rules run: main's new MustBlock tests compared `blockPairCharge` (now the branch's composite struct) directly to an integer. Updated those assertions to check `.mana`, and updated the shared declarability check to use composite affordability, matching the branch's option/payment/validation contract; all reruns above pass.

`gofmt` was applied to the merged Go files. `git diff --check` is clean. No unresolved conflict markers remain.

Integration completed as merge commit `d753b80e` (`Merge branch 'main' into wt/cli-20260922T225142Z-1d4558a1`), with `edc24484` (current main) as its second parent. `git merge-base --is-ancestor main HEAD` exited 0. Final `git status` reported: `On branch wt/cli-20260922T225142Z-1d4558a1` / `nothing to commit, working tree clean`.

---

## Preserved sibling-side report (carried verbatim from main at fead1e59)

The following report is the sibling ticket `cli-20260922T225141Z-462eca2e`'s `mrg1` report, which on main replaced the file at this canonical path. Preserved per the archive convention; this ticket's own report history is above.

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
  ```
- Merged-table measurement (the test's own counting logic): `raw | lines: 43`,
  `data rows: 42`.
- `go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' -v`:
  ```
  --- PASS: TestKnownApproximationsOnlyShrinks (0.00s)
  --- PASS: TestKnownApproximationRowsAreShort (0.00s)
  ok  	github.com/adams-shaun/gorge/internal/testutil	0.002s
  ```
- Required post-merge ratchets,
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead' -v`:
  ```
  --- PASS: TestEveryRepoDeckIsFullySupported (1.32s)
  --- PASS: TestEveryRepoDeckCountHeadResolves (0.01s)
  --- PASS: TestEveryRepoDeckParamsAreRead (0.26s)
  --- PASS: TestEveryDispatchedTriggerModeHasAMatcher (0.00s)
  --- PASS: TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched (0.00s)
  ok  	github.com/adams-shaun/gorge/rules	1.686s
  ```
- Branch fix + main's new closures on the merged tree:
  `go test ./rules -run 'DamageCost|DamageKeyword|LKI|Infect'` →
  `ok ... 0.647s`; `go test ./rules -run 'Monstrosity|FaceUp|TurnFaceUp|Panoptic|Mulligan'`
  → `ok ... 0.653s`.
- `go test ./rules -run 'TestHeads$'` → `ok github.com/adams-shaun/gorge/rules 1.781s`
  (main's re-pinned heads `408fdf32` reproduce on the merged tree; no head
  moved from this resolution).
- `gofmt -l internal/testutil/agentsdoc_test.go` → clean; conflict-marker grep
  over all tracked files → none.

### Issues

None found during this integration round beyond the resolved conflicts. One
note: both sides' `knownApproximationRows` comments were each true only for
their own tip; the merged-tree constant 42 supersedes both.

---

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

## Issues (this ticket)

No new unfixed issue was found while resolving this integration conflict. The branch commit's reported replicated-mode offer-gate limitation remains as documented in its commit message: `legal.go`'s `offerCastable` remains pool-only, so a replicate option can be withheld when only convoke would fund it. It was outside the conflict and was not changed here.

## Round 2 — second main integration (2026-09-23 02:40)

After the round-1 merge (`f8bbf659`, main at `1be022eb`) landed, main advanced to
`edc24484` (the sibling ticket `cli-20260922T225141Z-1b1182e4` merged: MustBlock
enforcement, `ImprintOnHost$`, `RememberCounteredCMC`, first-strike phase gating).
Ran `git merge main` again; two conflicts:

### `internal/testutil/agentsdoc_test.go`

Both sides had already resolved `knownApproximationRows` to `42` (main's comment
lists its closure set: acc7878d, 63c07260, 7c4182ff, d56e404f, 78d3b764,
f76f59fd and earlier ones; our round-1 comment said the same count). Took main's
more detailed comment verbatim — the constants agree, only the explanatory text
differed. Verified by counting the merged `AGENTS.md` table: **42 data rows**,
matching the constant.

### `.ds4/report-mrg1.md`

Path collision with the sibling ticket `1b1182e4`, whose own `mrg1` report
(1799 lines) reached main through its merge chain. This file at the canonical
path is THIS ticket's report; kept ours (as round 1 did) and appended this
section. Main's copy is the sibling's report and stays in its own merge
commits (`af2f1648` etc.); nothing else referenced it.

Everything else auto-merged (`AGENTS.md`, `rules/combat.go`, `effects/misc.go`,
`decision/*`, `state/phase.go`, new tests and sources from main).

### Commands and outputs

- `git merge main`:
  `Auto-merging .ds4/report-mrg1.md / CONFLICT (content): Merge conflict in .ds4/report-mrg1.md`
  `Auto-merging AGENTS.md / Auto-merging internal/testutil/agentsdoc_test.go / CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go`
  `Automatic merge failed; fix conflicts and then commit the result.`
- Row count: `awk` over merged `AGENTS.md` → 42 data rows.
- `.cards` check: present (`CARDS_OK`).

### Issues

No new unfixed issue found in this round. The round-1 note stands: `legal.go`'s
`offerCastable` remains pool-only, so a replicate option can be withheld when
only convoke would fund it (documented in `0836163f`'s commit message).

---

## Round 3 — this ticket's integration of main at edc24484, and round 4 — main at fead1e59 (2026-09-23)

### Rounds 1–2 (this ticket, main at edc24484)

A previous resolver session of THIS ticket completed the dispatched rebase-conflict
resolution as a merge fallback (commits `d753b80e` + `d54ba722`; the full record is
in the "Current integration round — main at edc24484" section above): the composite
CantBlockUnless block charge and main's MustBlock statics were both kept, and all
gates passed.

### Round 4 — main advanced to fead1e59

Main moved from `edc24484` to `fead1e59` (sibling `cli-20260922T225141Z-5641f97b`
merged: `0836163f`, the replicate count bound priced at the real charge). Ran
`git merge main` again on this branch; one conflict:

### `.ds4/report-mrg1.md`

Same path collision as before: main's version REPLACED the accumulated archive with
the sibling ticket's 77-line report. Resolved per the established convention: kept
this ticket's 2571-line archive verbatim (stage 2), appended the sibling's 77-line
report verbatim under a preservation header (stage 3), then this record.

### `internal/testutil/agentsdoc_test.go`

Auto-merged; the constant was 42 while the merged `AGENTS.md` table measures
**41 data rows** (main's replicate-count-bound row deletion from `0836163f` was
merged into the table on main without lowering the constant there). Lowered
`knownApproximationRows` 42 → 41 to match the merged content, per the constant's
own instruction; the comment now names all three joined closures.

`AGENTS.md`, `rules/cast.go` and `rules/replicate_count_bound_test.go` auto-merged.

### Commands and outputs

- `git status` on arrival: clean; merge commit `d753b80e` already present; main at `fead1e59`.
- `git merge main --no-edit`:
  `Auto-merging .ds4/report-mrg1.md / CONFLICT (content): Merge conflict in .ds4/report-mrg1.md / Auto-merging AGENTS.md`
- Row count: 41 data rows in merged `AGENTS.md` (awk over the table), 41 on main.
- `.cards` check: present, symlink to `/home/sadams/projects/gorge/.cards`.
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|TestHeads$|Block|CantBlockUnless'`:
  `ok github.com/adams-shaun/gorge/rules 1.954s`
- `go test ./internal/testutil -run 'TestKnownApproximation'`:
  `ok github.com/adams-shaun/gorge/internal/testutil 0.001s`
- `go test ./internal/archtest/`:
  `ok github.com/adams-shaun/gorge/internal/archtest 2.805s`
- `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/`:
  `ok github.com/adams-shaun/gorge/cmd/botbench 0.985s`
- `gofmt -l` on the touched Go files: clean.
- `git merge-base --is-ancestor main HEAD`: exit 0.
- Merge commit: `e8e9ad6a`; final `git status`: clean.

### Issues

No new engine issue found. Carried note from the sibling's report (unchanged, outside
this conflict): `legal.go`'s `offerCastable` remains pool-only, so a replicate option
can be withheld when only convoke would fund it (documented in `0836163f`'s commit
message).

---

## Round 5 — main at 4a7bb2fe (2026-09-23)

Main advanced from `fead1e59` to `4a7bb2fe`: the sibling ticket
`cli-20260922T225141Z-22391c4d` merged (token-replacement `Optional$` ask,
`Type$ ReplaceController`, `Amount$` and per-mint riders — commits `bc3f03ab`,
`f89770be`, `799ac002`), plus sibling `agent-20260919T203859Z-269892c3`'s
filter combat-history predicates (`IsGoaded`, controller-dealt-combat-damage).
Ran `git merge main --no-edit` on this branch; two conflicts:

### `.ds4/report-mrg1.md`

The same canonical-path collision as rounds 2 and 4: main's copy carries the
sibling ticket's report tail. Kept this ticket's archive (stages 2–4) and
preserved main's side verbatim under the header below, per the established
convention.

### `internal/testutil/agentsdoc_test.go`

Both sides had edited `knownApproximationRows`: HEAD said 41, main said 40. The
merged `AGENTS.md` (auto-merged, both sides' row deletions applied) measures
**40 data rows** using the test's own `approximationRows()` algorithm (lines
with prefix `| ` between `## Known approximations` and the next `## `, header
row dropped). Resolved to **40** with a comment naming both sides' closures —
this is exact (neither side's slack retained) and preserves both sides' row
deletions.

### Preserved main-side report record (sibling `22391c4d`)

> No new unfixed issue was found while resolving the merge. The token-replacement
> implementation's out-of-scope remainder is recorded in the branch's ticket
> report/commit history; this conflict resolution did not alter engine behavior
> outside retaining main's changes.

### Commands and outputs

- `git status` on arrival: clean at `895da057`, nothing in flight (prior round's
  rebase was aborted by the daemon).
- `git merge main --no-edit`:
  `Auto-merging .ds4/report-mrg1.md / CONFLICT (content): Merge conflict in .ds4/report-mrg1.md`
  `Auto-merging AGENTS.md / Auto-merging effects/misc.go / Auto-merging internal/testutil/agentsdoc_test.go / CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go`
  `Automatic merge failed; fix conflicts and then commit the result.`
- Row count (test's own algorithm) over merged `AGENTS.md` → 40 data rows.
- `git add internal/testutil/agentsdoc_test.go; git add -f .ds4/report-mrg1.md`,
  then `git -c core.editor=true commit --no-edit`:
  `[wt/cli-20260922T225142Z-1d4558a1 18da36e9] Merge branch 'main' into wt/cli-20260922T225142Z-1d4558a1`.
- `git merge-base --is-ancestor main HEAD`: exit 0 (main `4a7bb2fe` is now an
  ancestor); final `git status`: clean.
- `.cards` check: present, symlink to `/home/sadams/projects/gorge/.cards` (the
  rules run took 1.193s+, not the ~0s of a vacuous corpus-less run).
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`:
  `ok github.com/adams-shaun/gorge/rules 1.193s`.
- `go test ./internal/testutil -run 'TestKnownApproximation'`:
  `ok github.com/adams-shaun/gorge/internal/testutil 0.001s`.
- `go test ./rules -run 'Block|CantBlockUnless|TestHeads'`:
  `ok github.com/adams-shaun/gorge/rules 2.054s` (heads unmoved).
- `go test ./internal/archtest/`:
  `ok github.com/adams-shaun/gorge/internal/archtest 3.028s`.
- `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/`:
  `ok github.com/adams-shaun/gorge/cmd/botbench 0.994s` (20-game split unmoved).
- `gofmt -l internal/testutil/agentsdoc_test.go`: clean; `git diff --check`: no output.

### Issues

No new engine issue found in this round. The carried note stands: `legal.go`'s
`offerCastable` remains pool-only, so a replicate option can be withheld when
only convoke would fund it (documented in `0836163f`'s commit message).

---

## Round 6 — main at 0fbc2d10 (2026-09-23)

After round 5's merge commit (`18da36e9`) and report commit (`9b9a3e55`) landed,
main advanced again to `0fbc2d10` (sibling `agent-20260919T181629Z-732aedb9`:
`AddType$ AllBasicLandType`/`AllNonBasicLandType` land-type statics, plus the
intrinsic-append Produced read and the expanded land-type statics). Ran
`git merge main --no-edit`; **no conflicts** — clean ort auto-merge across
`rules/all_land_types_test.go`, `rules/clone.go`, `rules/engine.go`,
`rules/layers.go`, `rules/mana_activation.go`, `rules/paramcensus_test.go`.

Since main touched `rules/paramcensus_test.go` (a ratchet), re-ran the post-merge
ratchet set after the merge:

- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`:
  `ok github.com/adams-shaun/gorge/rules 1.373s`.
- `git merge-base --is-ancestor main HEAD`: exit 0 (main `0fbc2d10` ancestor);
  final `git status`: clean.

No further conflict was forced by this round.

No new engine issue was investigated or found during this integration-only resolution. The cascade ticket's remaining deviations and follow-up tickets are recorded in its existing ticket report; the cascade approximation row itself is deleted as intended.

---

## Round 7 — main at 4cdffbc1 (2026-09-23, ticket cli-20260922T225142Z-1d4558a1)

### Situation found

`git status` on arrival: **clean**, no rebase or merge in flight. `git reflog`
showed the daemon's last action was `rebase (start): checkout main`
(`4cdffbc1`) immediately followed by `rebase (abort): returning to
refs/heads/wt/cli-20260922T225142Z-1d4558a1` (`243f01b3`), so the reported
"rebase conflicted" and "merge fallback conflicted" captures describe attempts
the daemon then rolled back. The branch was at `243f01b3` (round-6 record),
whose merge parent integrated main up to `0fbc2d10`. Main had since advanced
to `4cdffbc1` (sibling `cli-20260922T225140Z-9e382c75`: the cascade closure —
`Triggers$ ExileEffect` lifetime, CR 601.3 free-cast gate, `{X}` mana value,
plus the delete-only cascade register edit).

Since no operation was in flight, I started the integration:

```
git merge main --no-edit
```

which auto-merged everything except two files:

```
Auto-merging .ds4/report-mrg1.md
CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
Auto-merging AGENTS.md
Auto-merging effects/misc.go
Auto-merging internal/testutil/agentsdoc_test.go
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
```

`AGENTS.md` itself auto-merged cleanly; `effects/cascade.go`,
`effects/misc.go`, `rules/cast.go` and the new `rules/cascade_approx_test.go`
all came across from main with no conflict.

### Conflict 1 — `internal/testutil/agentsdoc_test.go` (`knownApproximationRows`)

- **Branch side (`243f01b3`):** `knownApproximationRows = 40`, comment naming
  this branch's CantBlockUnless (`blockprop1`) deletion plus prior closures.
- **Main side (`4cdffbc1`):** `knownApproximationRows = 39`, comment naming
  the cascade1 deletion.

Measured with the test's own `approximationRows()` algorithm against the
**auto-merged** `AGENTS.md`:

```
merged AGENTS.md | rows: 39
branch AGENTS.md | rows: 40
main   AGENTS.md | rows: 39
```

The two sides deleted disjoint rows: the branch deleted `(blockprop1)`
(stat:CantBlockUnless), main deleted `(cascade1)` (cascade). The auto-merged
table contains **neither** row — `grep -c blockprop1 AGENTS.md` → 0,
`grep -c cascade1 AGENTS.md` → 0, `grep -c each1 AGENTS.md` → 1 (kept). The
merged table is therefore 39 rows and BOTH closures are preserved.

**Resolution:** set the constant to **39**, with a comment naming both sides'
deletions. Both sides' constants were already above their own tables (40 vs
40 branch / 39 vs 39 main), so 39 is exactly the measured merged count and the
register stays delete-only.

### Conflict 2 — `.ds4/report-mrg1.md` (report artifact)

This path is shared: the auto-merge pulled in main's copy from the sibling
cascade ticket while the branch carried this ticket's accumulated round 1–6
report. Both are documentation, not code, and neither contradicts the other's
intent. **Resolution:** union — keep both sides' text (removed the markers,
preserved the branch's rounds 1–6 and maintained main's cascade-ticket issue
note), then append this Round 7 section. No engine content is involved.

### Commands and measured output

- `git status` on arrival — clean at `243f01b3`; `git reflog` showed the
  aborted rebase described above; `git log --oneline main ^HEAD` showed the
  cascade commits (`e46f051d`, `1609ef43`, `ec9738c1`, …) not yet integrated.
- `.cards` check — symlink present (`cards.lock`, `cardsfolder`,
  `ir.gob.gz`), so no vacuous corpus-less run.
- `git merge main --no-edit` — conflicts only in the two files above.
- Row counts (test's own algorithm): merged 39, branch 40, main 39.
- `git add -f internal/testutil/agentsdoc_test.go .ds4/report-mrg1.md`
  (the `-f` is needed because `.ds4` is git-excluded but the file is tracked)
  then `git commit --no-edit` → merge commit `df9f695f`
  ("Merge branch 'main' into wt/cli-20260922T225142Z-1d4558a1").
- `git merge-base --is-ancestor main HEAD` → exit 0 (main `4cdffbc1` is now an
  ancestor).
- `go test -count=1 -v -run 'TestKnownApproximation' ./internal/testutil/` →
  `PASS` both tests, `ok ... 0.001s` (constant 39 = measured 39).
- Post-merge ratchet set:
  `go test -count=1 -v ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  → 5 tests RUN, 0 SKIP, all PASS, `ok ... 0.801s`.
- `go test -count=1 ./rules -run 'Block|CantBlockUnless|TestHeads'` →
  `ok ... 1.855s` (heads unmoved).
- `go test ./internal/archtest/` → `ok ... 3.453s`.
- `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` →
  `ok ... 1.117s` (20-game pinned split unmoved).

### Issues

No new engine issue found in this round. This was integration-only: the only
code change is the `knownApproximationRows` constant, and the two row
deletions (blockprop1, cascade1) are already reflected in the merged
`AGENTS.md`.

--- (main's side of the round-8 conflict follows) ---

No new unfixed issue was found while resolving the merge; the integration
introduced no engine-code change. The `knownApproximationRows` comment drift
between branches is cosmetic noise from the tracked per-merge report artifact;
no action needed.

(Preserved from main's side of the round-8 conflict: the text above is the
Issues section carried by main's copy of this report from its integration of
`cli-20260922T225142Z-0ab0cb60`.)

## Round 8 — main advanced to 08a1d59a (2026-09-23)

After round 7 (`df9f695f`, main at `4cdffbc1`) landed, main advanced to
`08a1d59a` — the sibling ticket `cli-20260922T225142Z-0ab0cb60` merged
(`8d83f028`: MaxTotalTargetPower$ offset bounded by TargetMax$, cap read in
the cross-mode Charm ask). Ran `git merge main`; conflicts:

- `.ds4/report-mrg1.md` — the accumulated Issues section (both sides had
  appended their own). Resolved by keeping this branch's full history and
  preserving main's Issues text above.
- `internal/testutil/agentsdoc_test.go` — `knownApproximationRows`: HEAD said
  39, main said 38.
- `AGENTS.md`, `rules/stack.go`, `rules/target_max_power_cap_test.go`
  auto-merged cleanly.

### The stale each1 row — the substantive finding

Measuring the auto-merged AGENTS.md table with the exact `approximationRows()`
logic (`| `-prefixed lines between `## Known approximations` and the next
`## `, header row dropped; note the trailing `api:ExchangeLifeVariant` row
sits past a blank line and is still counted) gave **38 data rows**. A row-id
set comparison against main exposed why:

- HEAD's table still carried the `(each1)` row (EachDamage fail-closed),
  which MAIN had closed in `49a2fde8` ("fix(effects): surface EachDamage
  fail-closed outcomes", ancestor of the round-7 base `4cdffbc1`). Round 7's
  merge (`df9f695f`) had kept the branch's stale copy instead of main's
  deletion — its side of that merge: branch 1 each1, main 0, result 1. The
  row text on the branch is byte-identical to `49a2fde8^`'s version.
- main's table still carried `(blockprop1)`; this branch's fix (`6e77a1e8`)
  deletes it, and the auto-merge had done so correctly.

**Resolution:** deleted the stale `(each1)` row from the merged AGENTS.md
(delete-only register rule: a row main closed must never be resurrected).
Merged table is now **37 data rows**; the constant is set to 37 with a
comment naming the three closures (blockprop1 by this branch, each1 by
main's `49a2fde8`, maxpower1 by main's `8d83f028`).

## Commands and output (round 8)

- `git merge main` — `CONFLICT (content): Merge conflict in .ds4/report-mrg1.md`,
  `CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go`;
  AGENTS.md, rules/stack.go, rules/target_max_power_cap_test.go auto-merged.
- Row census (test logic, incl. the trailing ExchangeLifeVariant row):
  HEAD 39 data rows (incl. stale each1, excl. blockprop1), main 38
  (excl. each1 and maxpower1), auto-merged 38 (incl. stale each1),
  after the each1 removal **37**.
- `git show main:AGENTS.md` vs merged row-id sets — only each1 (stale) and
  blockprop1 (closed here) differed; after the fix the sets match on main's
  side exactly.

## Issues (round 8)

- The stale `(each1)` resurrection was introduced by an earlier round of THIS
  branch's integration history (round 7's `df9f695f`-class resolution kept the
  branch copy over main's `49a2fde8` deletion); no engine defect.
- No new engine issue found in this round; the only AGENTS.md change is the
  register deletion. Engine code merged from main (`8d83f028`) is reviewed
  upstream and untouched here.


## Verification (round 8)

`.cards` was present (symlink to the main checkout's corpus), so no corpus
test skipped vacuously. Real output, in order:

- `go test -run 'TestKnownApproximations' ./internal/testutil/` →
  `ok github.com/adams-shaun/gorge/internal/testutil 0.001s` (after the
  constant was corrected to 37; the first attempt at 36 FAILED with
  "has 37 rows", which is what surfaced the trailing ExchangeLifeVariant row
  the section-level counter reads past a blank line).
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  → `ok github.com/adams-shaun/gorge/rules 0.809s`; the `-v` rerun shows
  5 top-level RUNs (TestEveryRepoDeckIsFullySupported,
  TestEveryRepoDeckCountHeadResolves, TestEveryRepoDeckParamsAreRead, and the
  two registry/dispatch ratchets), zero FAIL, zero SKIP.
- `go test ./rules -run 'Block|CantBlockUnless|TargetMax|MaxTotalTargetPower'`
  → `ok ... 0.732s` (this branch's blockprop fix and main's TargetMax work
  both green together).
- `go test ./internal/archtest/` → `ok ... 2.344s`.
- `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` →
  `ok ... 1.024s` (20-game pinned split unmoved by the integration).

Final state: `d1815beb` is the merge commit; `git status` clean.

---

## Preserved sibling-side report (carried from main at e142295d)

The following is the sibling ticket `cli-20260922T225141Z-462eca2e`'s `mrg1` report continuation (its issues note and round-3/round-4 records), which on main replaced the file at this canonical path. Preserved per the archive convention; this ticket's own report history is above.

## Issues (sibling ticket)

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

---

## Round 9 — merge-conflict resolution (2026-09-23, dispatcher mrg1)

### Situation found

The worktree was CLEAN at entry (`git status` clean, no rebase/merge in
flight) at `f8d7f44f`, the round-8 docs record. `main` had advanced past the
round-8 integration base `08a1d59a` to `e142295d`. `git merge-base --is-ancestor
main HEAD` exited 1. The dispatch's quoted rebase conflict (1/10 on `6e77a1e8`)
was stale — that rebase had been rolled back; the daemon's merge fallback is
the operation to finish, so I ran `git merge main --no-edit` against
`e142295d`.

Main's new work since round 8 was the sibling attack-prop ticket
(`wt/cli-20260922T225141Z-462eca2e`, tip merge `e142295d`): commits
`89c77778` ("widen the attack-prop mana window and price per attacker") and
`1c3df172` ("the attack/block prop window resolves the tap alternative the
payer selected"), plus a `precondition: blockPairCharge = %d, want 2` line in
`attackprop_altselect_test.go:121`.

### Conflicts and resolution

`git merge main --no-edit` (against `e142295d`) produced four content
conflicts plus two auto-added files:

```
CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
CONFLICT (content): Merge conflict in AGENTS.md
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
CONFLICT (content): Merge conflict in rules/attack_cost.go
A  rules/attackprop_altselect_test.go
A  rules/attackprop_window_test.go
```

**`rules/attack_cost.go`** — two hunks. Both sides reworked the SAME
block-prop plumbing from a different angle:

- HEAD (`6e77a1e8`, the reviewed fix) replaced the mana-only `int32` block
  charge with the composite `blockCharge` (mana + life + tap obligations) and
  its `blockUnlessCharge` / `blockTapPlan` / `blockChargeAffordable` /
  `payBlockExtras` support, and delivered `AddStaticAbilities$` statics.
- main (`1c3df172`) rewrote the block pay window to resolve the EXACT tap
  alternative the payer selected (`blockPayWindow.sources []attackManaSource`
  + `paySourceForAnswer`) and changed `attackUnlessPrice` to take an
  `attacker` argument.

Resolution: kept HEAD's composite machinery (it is the reviewed fix) and
merged main's source-identity into it, so both intents hold:

1. `blockPairCharge`'s delivered-static loop kept HEAD's composite
   `blockUnlessCharge(sv)` (richer than main's `attackUnlessPrice(sv, 0)`; the
   branch's `blockUnlessCharge` prices the non-mana components main's row was
   still skipping).
2. `blockPayWindow` now carries BOTH HEAD's `charge blockCharge` + frozen
   `taps []state.ObjID` AND main's `sources []attackManaSource`. The
   auto-merged `startBlockPay` / `askNextBlockPay` / `blockPayAnswer` already
   used `charge.mana`, `st.taps`, `paySourceForAnswer(st.sources, …)` and the
   `s.prod` label, so only the struct field set needed merging.
3. `attackprop_altselect_test.go:121` calls `e.blockPairCharge(qal, bear)` and
   compares to `2`; the branch's `blockPairCharge` returns `blockCharge`. This
   is a forced consequence of the reviewed signature change, so it was
   updated to `.mana` — the same read the branch's own block tests use
   (`block_prop_test.go`, `block_prop_nonmana_test.go`, `mustblock_*_test.go`
   all read `.mana`/`.life`/`.taps` off the composite).

**`AGENTS.md`** — one hunk. Both sides deleted a DIFFERENT `Known
approximations` row from the merge base (which carried both):

- HEAD deleted `(blockprop1)` (the branch closed the CantBlockUnless
  approximation).
- main deleted `(attackprop1)` (its attack-prop work closed that row).

Delete-only register: both deletions are valid and disjoint, so the resolution
keeps NEITHER row. Measured with the test helper's own counter (`^\| ` lines
between `## Known approximations` and the next `## `, header dropped): merge
base `08a1d59a` = 38 rows, HEAD = 37, main = 37, merged = **36**.

**`internal/testutil/agentsdoc_test.go`** — the `knownApproximationRows`
ratchet. HEAD's comment claimed 37 and main's constant also said 37; neither
had integrated the other's deletion. Both comments were rewrites (not
contradictions), so I wrote one merged comment recording both closures and set
the constant to the measured **36**. `TestKnownApproximationsOnlyShrinks`
passes at 36.

**`.ds4/report-mrg1.md`** — a report artifact, not code. Per the established
archive convention, kept this worktree's accumulated report lineage and
preserved main's sibling-ticket report (`462eca2e`) as a clearly-delimited
section rather than discarding either history.

### Second integration (current main)

While the first merge was in flight, `main` moved again, from `e142295d` to
`827ca863` (the sibling devthr ticket: `1e32c0e0` "resolve YourStartingLife
Count head", touching `effects/count.go`, a new
`effects/your_starting_life_test.go`, and removing the `YourStartingLife`
entry from `rules/count_head_ratchet_test.go`). That delta is disjoint from
this branch's conflict areas, so `git merge main --no-edit` merged cleanly
(`a789e0e0`). `main` is now an ancestor of HEAD.

Note: the target branch moved twice inside one dispatch window; rounds 1–8 of
this file record the same churn. The full gate run after DONE is the arbiter.

### Commands run (real output)

```text
git merge-base --is-ancestor main HEAD   → exit 1 at entry (main e142295d), exit 0 after a789e0e0
git merge main --no-edit                 → 4 conflicts (above)
# row measurement with the test's own counter:
#   merge base 08a1d59a = 38 data rows; HEAD = 37; main = 37; merged = 36
gofmt -l rules/attack_cost.go internal/testutil/agentsdoc_test.go   → (empty)
go build ./...                            → exit 0
go vet ./rules/                           → exit 0
go test ./internal/testutil -run 'TestKnownApproximation' -count=1
  ok  github.com/adams-shaun/gorge/internal/testutil  0.001s
go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
  ok  github.com/adams-shaun/gorge/rules  0.772s
go test ./rules -run 'TestBlockProp|TestAttackProp|TestCantBlockUnless|TargetMax|MaxTotalTargetPower'
  ok  github.com/adams-shaun/gorge/rules  0.772s
go test ./effects -run 'TestYourStartingLife|TestCount'
  ok  github.com/adams-shaun/gorge/effects  0.646s
go test ./internal/archtest/              → ok  github.com/adams-shaun/gorge/internal/archtest  3.111s
go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
  ok  github.com/adams-shaun/gorge/cmd/botbench  1.066s   (20-game pinned split unmoved)
```

`.cards` was present (symlink to the shared corpus: `cards.lock`,
`cardsfolder`, `ir.gob.gz`), so no corpus-backed test skipped vacuously.

### Merge commits

- `7c4ee058` — `Merge branch 'main' into wt/cli-20260922T225142Z-1d4558a1`
  (parents `f8d7f44f` + `e142295d`), the conflict resolution.
- `a789e0e0` — second, conflict-free integration of current main
  (`827ca863`).

### Issues

None new. This was integration only. The two approximations whose rows the
merge base carried are both now closed by the respective sides
(`(blockprop1)` by this branch, `(attackprop1)` by main) and both rows are
deleted from the register. No engine behaviour was changed by the resolution
itself; the only non-conflict edit was the forced `.mana` read in
`attackprop_altselect_test.go`, which is a signature-adaptation, not a
behaviour change.

No defect found while resolving this merge that needs a new CR-lane test.


---

## Preserved main-side report (kept verbatim from the merge conflict, main side)

---

# Preserved main-side report record (bestow-merge round, main at 835074e5)

The following report is the sibling ticket `cli-20260922T225142Z-171edf8c` (bestow)
merge report, which on main replaced the file at this canonical path. Preserved
per the archive convention; this ticket's own report history is above.

# Merge conflict resolution — mrg1

## Result

Merged `main` into `wt/cli-20260922T225142Z-e9128096`. The merge initially conflicted in `internal/testutil/agentsdoc_test.go`; resolved it, applied the table count matching the auto-merged `AGENTS.md`, and completed the merge. Added the missing `spell.Ability` census classification required by the requested post-merge ratchet.

## Entry state and conflict

Initial `git status` reported a clean tree on `wt/cli-20260922T225142Z-e9128096` at reviewed commit `6c86af9b`; no rebase or merge was in progress. Ran `git merge main`, which conflicted only in `internal/testutil/agentsdoc_test.go`. `AGENTS.md` and `effects/filter.go` auto-merged.

The conflict was over `knownApproximationRows` and its explanatory comment. The branch had removed the `CanBeTargetedByTriggeredSpellAbility` approximation row; main had independently updated/deleted approximation rows. I kept both sides' table changes and measured the auto-merged table using the same `| ` row parsing and header removal as the test: **35 data rows**. Set the constant to 35 and replaced both stale comments with a concise note reflecting that merged measurement. `knownOversizeRows` remains unchanged.

No code conflict occurred in `effects/filter.go`; its main and branch changes were auto-merged. The merge also brought main's changes to `AGENTS.md`, `effects/count.go`, `rules/attack_cost.go`, `rules/bestow.go`, `rules/cast.go`, `rules/count_head_ratchet_test.go`, `rules/layers.go`, `rules/legal.go`, `state/object.go`, and associated tests. These were merge results, not hand-edited conflict resolutions.

## Ratchet follow-up

The first requested post-merge rules ratchet run failed because the branch's new `rules/triggered_target_legality.go` reads `spell.Ability.Params`, and main's parameter-census rot guard had not classified that selector base. Added `spell.Ability` to `rules/paramcensus_test.go`'s `baseBuckets` as `bSA`, with a comment identifying it as the stack object's resolved `*cards.SA`. This is the type-justified census entry required by the new rules-side code.

## Commands and output

```text
git status --short --branch && git status
## wt/cli-20260922T225142Z-e9128096
On branch wt/cli-20260922T225142Z-e9128096
nothing to commit, working tree clean

git merge main
Auto-merging AGENTS.md
Auto-merging effects/filter.go
Auto-merging internal/testutil/agentsdoc_test.go
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
Automatic merge failed; fix conflicts and then commit the result.

# Measured merged Known approximations data rows
Known approximations data rows: 35

# .cards check
lrwxrwxrwx ... .cards -> /home/sadams/projects/gorge/.cards

go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'
ok   github.com/adams-shaun/gorge/internal/testutil  0.002s

go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
--- FAIL: TestEveryRepoDeckParamsAreRead (0.12s)
    paramcensus_test.go:2743: paramcensus rot guard: 1 findings:
        paramcensus: 1 unclassified Params reads (the census cannot rot):
        triggered_target_legality.go:20:24: unclassified Params base "spell.Ability" -- add it to baseBuckets with a type justification
FAIL
FAIL    github.com/adams-shaun/gorge/rules  0.759s
FAIL

# After adding the verified spell.Ability -> bSA census classification:
go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok   github.com/adams-shaun/gorge/rules  0.755s
```

The failing first ratchet run is recorded above; the same ratchet command passed after the census classification. `.cards` was present as the shared-corpus symlink, so the rules check was not a vacuous corpus-skipped run.

## Issues

No unresolved merge issue. The initial ratchet failure was resolved by adding the missing, type-justified census bucket; the separate `UnlessCost$` grammar remainder remains tracked by the related ticket cited in the issue brief.

<!-- main-side report, kept verbatim -->

No new unfixed issue was found while resolving this integration conflict. The branch commit's reported replicated-mode offer-gate limitation remains as documented in its commit message: `legal.go`'s `offerCastable` remains pool-only, so a replicate option can be withheld when only convoke would fund it. It was outside the conflict and was not changed here.

## Round 2 — second main integration (2026-09-23 02:40)

After the round-1 merge (`f8bbf659`, main at `1be022eb`) landed, main advanced to
`edc24484` (the sibling ticket `cli-20260922T225141Z-1b1182e4` merged: MustBlock
enforcement, `ImprintOnHost$`, `RememberCounteredCMC`, first-strike phase gating).
Ran `git merge main` again; two conflicts:

### `internal/testutil/agentsdoc_test.go`

Both sides had already resolved `knownApproximationRows` to `42` (main's comment
lists its closure set: acc7878d, 63c07260, 7c4182ff, d56e404f, 78d3b764,
f76f59fd and earlier ones; our round-1 comment said the same count). Took main's
more detailed comment verbatim — the constants agree, only the explanatory text
differed. Verified by counting the merged `AGENTS.md` table: **42 data rows**,
matching the constant.

### `.ds4/report-mrg1.md`

Path collision with the sibling ticket `1b1182e4`, whose own `mrg1` report
(1799 lines) reached main through its merge chain. This file at the canonical
path is THIS ticket's report; kept ours (as round 1 did) and appended this
section. Main's copy is the sibling's report and stays in its own merge
commits (`af2f1648` etc.); nothing else referenced it.

Everything else auto-merged (`AGENTS.md`, `rules/combat.go`, `effects/misc.go`,
`decision/*`, `state/phase.go`, new tests and sources from main).

### Commands and outputs

- `git merge main`:
  `Auto-merging .ds4/report-mrg1.md / CONFLICT (content): Merge conflict in .ds4/report-mrg1.md`
  `Auto-merging AGENTS.md / Auto-merging internal/testutil/agentsdoc_test.go / CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go`
  `Automatic merge failed; fix conflicts and then commit the result.`
- Row count: `awk` over merged `AGENTS.md` → 42 data rows.
- `.cards` check: present (`CARDS_OK`).

### Issues

No new unfixed issue found in this round. The round-1 note stands: `legal.go`'s
`offerCastable` remains pool-only, so a replicate option can be withheld when
only convoke would fund it (documented in `0836163f`'s commit message).

---

## Record — current integration round (cli-20260922T225143Z-bc326d39, round 2)

### Starting state and operation

The worktree was CLEAN at `da4e8c78`, the branch's prior merge of main
`1be022eb`; no rebase or merge was in flight (the daemon's rebase and its
merge fallback had both been rolled back, and the reflog showed a final
`reset: moving to HEAD`). `main` had advanced past `1be022eb` to `fead1e59`
(the cli-20260922T225141Z-5641f97b Replicate count-bound landing, plus the
MustBlock / `ImprintOnHost$` / `RememberCounteredCMC` / first-strike-phase
merges from cli-20260922T225141Z-1b1182e4). Rebase is forbidden in this seat,
so I re-integrated with `git merge main`.

### Conflicts and resolution

1. **`internal/testutil/agentsdoc_test.go`** — only the
   `knownApproximationRows` comment/constant. HEAD side carried 42; main side
   carried 42 with a longer comment. Both values were stale for the merged
   table because the two sides deleted DISJOINT rows:
   - base `1be022eb` measured **43** data rows;
   - this branch deleted the `kw:Infect` row (fix `56f98b13`, CR 113.7a
     last-known characteristics for damage cost keywords at the two cost
     sites);
   - main deleted the `Phase$ First Strike Damage` row (fix `acc7878d`,
     combat-presence gating) and the Replicate count-bound row (fix
     `0836163f`, the repeatable-cost charge).
   `43 - 3 = 40`. The auto-merged `AGENTS.md` table measures exactly **40**
   data rows (the test's own `approximationRows` logic) and all three deleted
   rows are absent (`grep -c` → 0). Resolved to
   `knownApproximationRows = 40` with a comment naming all three closures.
2. **`.ds4/report-mrg1.md`** — the shared accumulated report. HEAD's full
   report history (1526 lines) and main's report (77 lines, the sibling
   cli-20260922T225141Z-5641f97b round-2 record) were the two sides. Kept
   HEAD's complete history verbatim, appended main's copy verbatim under a
   `## Preserved main-side report` heading, and dropped only the conflict
   markers; neither historical report was discarded.

`AGENTS.md` auto-merged (all three disjoint row deletions retained). No engine
code conflicted: main's `decision/*`, `effects/count.go`, `effects/misc.go`,
`effects/registry.go`, `rules/combat.go`, `rules/trigmatch_misc.go`,
`state/phase.go` and new tests, together with this branch's `rules/cast.go`
damage-cost LKI change, merged cleanly (disjoint files/regions).

### Commands and output

- `git merge main --no-edit` →
  ```
  Auto-merging .ds4/report-mrg1.md
  CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
  Auto-merging AGENTS.md
  Auto-merging internal/testutil/agentsdoc_test.go
  CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
  Auto-merging rules/cast.go
  Automatic merge failed; fix conflicts and then commit the result.
  ```
- `.cards` present (real symlink to `/home/sadams/projects/gorge/.cards`) —
  runs are real, not vacuous skips.
- Merged-table measurement (the test's own counting logic): base `1be022eb` 43
  data rows; merged tree 40 data rows; `grep -c` on the three deleted rows → 0.
- `go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' -v`:
  ```
  === RUN   TestKnownApproximationsOnlyShrinks
  --- PASS: TestKnownApproximationsOnlyShrinks (0.00s)
  === RUN   TestKnownApproximationRowsAreShort
  --- PASS: TestKnownApproximationRowsAreShort (0.00s)
  PASS
  ok  	github.com/adams-shaun/gorge/internal/testutil	0.001s
  ```
- Required post-merge ratchets,
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead' -v`:
  ```
  --- PASS: TestEveryRepoDeckIsFullySupported (0.62s)
  --- PASS: TestEveryRepoDeckCountHeadResolves (0.00s)
  --- PASS: TestEveryRepoDeckParamsAreRead (0.13s)
  --- PASS: TestEveryDispatchedTriggerModeHasAMatcher (0.00s)
  --- PASS: TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched (0.00s)
  ok  	github.com/adams-shaun/gorge/rules	0.792s
  ```
- `gofmt -l internal/testutil/agentsdoc_test.go` → clean; conflict-marker grep
  over all tracked files → none.

### Issues / uncertainty

No new unfixed issue found during this integration round. The count in the
inherited main-side comment (and this branch's prior comment) was stale
because neither side knew about the other's row deletions; the measured 40 is
authoritative. Note (already recorded by the ticket): `legal.go`'s
`offerCastable` remains pool-only, so a replicate option can be withheld when
only convoke would fund it — outside this conflict, unchanged here.

### Addendum — second main integration (`90044c3f`)

After the `fead1e59` merge completed as `f46e1167`, `main` had advanced once
more to `90044c3f` (the `799ac002` filter combat-history predicates, merged
by agent-20260919T203859Z-269892c3). Merged it cleanly with
`git merge main --no-edit` (no conflicts):

```
 effects/filter.go                   |  31 ++++++++-
 effects/zone.go                     |   4 +-
 rules/combat_history_filter_test.go | 121 ++++++++++++++++++++++++++++++++++++
 3 files changed, 152 insertions(+), 4 deletions(-)
```

This merge commit is `ca4d847f`. `main` (`90044c3f`) is now an ancestor of the
branch, and the tree is clean.

Final checks on `ca4d847f`:

- `go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'` →
  `ok github.com/adams-shaun/gorge/internal/testutil 0.001s`
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'` →
  `ok github.com/adams-shaun/gorge/rules 0.741s`
- `go test ./rules -run 'DamageCost|DamageKeyword|LKI|Infect'` →
  `ok github.com/adams-shaun/gorge/rules 0.629s`
- `git grep -l '^<<<<<<<\|^>>>>>>>' HEAD` → no matches; `git status` clean.

The approximation table was unchanged by the second merge, so the constant
held at 40 for that round.

## Round 3 — third main integration (2026-09-23, this seat)

The previous mrg1 seat finished DONE at `b7de800a`, with `main` then at
`90044c3f` (already an ancestor of the branch). `main` has since advanced to
`08a1d59a`; the daemon re-dispatched this merge round because the branch was
again behind. `git status` found the tree CLEAN — no rebase or merge in flight
(the daemon's failed attempt had been fully aborted). Redid the integration as
`git merge main --no-edit`.

`main` moved from `90044c3f` to `08a1d59a`; the auto-merge result is the
conflict set the daemon reported: `internal/testutil/agentsdoc_test.go`
(content) and this report file. `AGENTS.md` auto-merged, as did `rules/cast.go`
and the rest of the tree.

### `internal/testutil/agentsdoc_test.go`

Only the `knownApproximationRows` constant and its comment conflicted. Base
`90044c3f` measured 41 data rows. Main deleted THREE rows
(`cascade1` from `4cdffbc1`, `tokrepl1`, `maxpower1`), lowering its constant to
38; the branch deleted ONE row (`kw:Infect` from `56f98b13`), which is what
this ticket's fix closed. The deletions are DISJOINT, so the merged table is
41 - 4 = **37 data rows** — verified by running the test's own counting logic
over the merged `AGENTS.md` (`## Known approximations` ... next `## `, lines
starting `| `, minus header) and by confirming all four rows are absent from
the merged file:

```
merge-base 90044c3f: 41
main 08a1d59a:      38
branch HEAD:        40
merged worktree:    37
```

Resolution: `knownApproximationRows = 37`, with the comment naming all four
closures (branch: `kw:Infect`; main: `cascade1`, `tokrepl1`, `maxpower1`).
Neither side's constant (40 / 38) was correct for the merge — each side was
unaware of the other's deletions. 37 is the measured, authoritative value.

### `.ds4/report-mrg1.md`

Path collision: the canonical report path is THIS ticket's report (ours), and
main's side carries a different ticket's 72-line mrg1 report. As in the
previous two rounds, kept ours and appended this section; main's different
ticket's report stays in its own merge chain.

### Commands run

- `git merge main --no-edit` → conflicts in `.ds4/report-mrg1.md` and
  `internal/testutil/agentsdoc_test.go`; `AGENTS.md` and `rules/cast.go`
  auto-merged. Matches the daemon's reported set.
- `go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'`
  → `ok github.com/adams-shaun/gorge/internal/testutil`
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  → `ok github.com/adams-shaun/gorge/rules`
- `gofmt -l internal/testutil/agentsdoc_test.go` → clean
- conflict-marker grep over the resolved files → none

`.cards` is present (real symlink), so corpus-backed ratchets are real runs,
not vacuous skips.

### Issues

No new issue was found in resolving the merge. The known pool-only
`offerCastable` replicate/convoke limitation noted in the inherited ticket
report was outside the conflicts and remains unchanged.

## Round 4 — integration of main `835074e5` (2026-09-23)

The tree was clean at merge commit `7a9985bd`, which had integrated main
`08a1d59a`; main had since advanced to `835074e5`. Ran `git merge main --no-edit`.
It conflicted in this accumulated report and `internal/testutil/agentsdoc_test.go`;
`AGENTS.md` and `rules/cast.go` merged automatically. This report is the
branch-side accumulated report, so its prior history is retained.

- `internal/testutil/agentsdoc_test.go`: the constant/comment conflicted.
  Main deletes the `(bestow1)` and `(attackprop1)` rows relative to the prior
  merged tree, while the branch's `kw:Infect` closure remains. The merged
  `AGENTS.md` table counts **35 data rows** (36 table lines including its
  header), so resolved `knownApproximationRows` to 35; neither stale comment
  value (37 or 36) described this merged tree.
- `.ds4/report-mrg1.md`: retained the full branch-side report history and
  replaced only this conflict tail with the present round's record.
- Main's other changes, including bestow and attack-prop fixes/tests, were
  retained. No engine-code conflict required a manual choice.

Commands and results:

- `git status --short --branch` before merge → `## wt/cli-20260922T225143Z-bc326d39` (clean).
- `git merge main --no-edit` → conflicted as recorded above.
- `go test ./internal/testutil -run 'TestKnownApproximation' -v` → both `TestKnownApproximationsOnlyShrinks` and `TestKnownApproximationRowsAreShort` passed; `ok github.com/adams-shaun/gorge/internal/testutil 0.001s`.
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead' -v` → all five selected ratchets passed; `TestEveryRepoDeckIsFullySupported` reported 4 of 916 distinct cards unsupported and `TestEveryRepoDeckParamsAreRead` reported 29 of 916 cards with unread parameters; package `ok github.com/adams-shaun/gorge/rules 0.767s`.
- The table parser counted 35 data rows; `.cards` is present.
- `git diff --check` found one extra blank line at EOF in this report; it was removed. The post-resolution conflict-marker scan over `AGENTS.md`, `internal/testutil/agentsdoc_test.go`, and this report produced no matches.
- `GIT_EDITOR=true git merge --continue` created merge commit `b5ca56b5` (parents `7a9985bd` and `835074e5`); `git status --short --branch` showed a clean tree. Main advanced again to `e9dfbe9c` during validation; this merge integrates the `835074e5` tip that was current when the operation began.

---

No new unfixed defect found. No uncertainty remains about the ratchet value:
36 is the measured data-row count of the merged `AGENTS.md`, and neither
conflicted comment matched it.

---

# Round 10 — integration of main at 835074e5 (merge commit cc1ff692)

Entry state: tree clean at `08a95dff` (round 9); `main` had advanced from
`827ca863` to `835074e5` (the sibling bestow ticket `cli-20260922T225142Z-171edf8c`:
`0b9ae217` prices and offers the three exotic bestow costs and makes a bestowed
spell an Aura spell; `976ae1ee` removes its non-regression evidence test; three
merge commits). `git merge main --no-edit` conflicted in three files:

- `AGENTS.md` — the two sides deleted two ADJACENT rows of the frozen register:
  this branch closed `(blockprop1)` (6e77a1e8), main's bestow ticket closed
  `(bestow1)`. Resolution keeps NEITHER row. Measured data rows: merge base
  827ca863 = 37, HEAD = 36, main = 36, merged = 35.
- `internal/testutil/agentsdoc_test.go` — both conflicted comments said 36 for
  their own pre-merge states; resolution sets `knownApproximationRows = 35`
  with a comment recording the measured base 37 − 2 = 35.
- `.ds4/report-mrg1.md` — kept this ticket's full lineage (`--ours`), appended
  main's current report (the bestow-merge round, 104 lines) as a delimited
  preserved section per the archive convention.

Verification (all real output in `.ds4/scratch/r*.log`):

```text
go test ./internal/testutil -run 'TestKnownApproximation'   → ok 0.001s (constant 35 holds)
go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
                                                            → ok 0.784s
go test ./rules -run 'TestBlockProp|TestAttackProp|TestCantBlockUnless|Bestow|Block'
                                                            → ok 0.887s
go test ./effects -run 'TestYourStartingLife|TestCount'     → ok 0.931s
go test ./internal/archtest/                                → ok 4.471s
go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
                                                            → ok 1.304s (20-game pinned split unmoved)
go build ./...                                              → exit 0
```

`.cards` present (symlink: `cards.lock`, `cardsfolder`, `ir.gob.gz`) — no
vacuous skips. `git merge-base --is-ancestor main HEAD` → exit 0. No new
trigger mode registered by the merged delta (no `addedAfterTheSplit` entry
needed); no engine behaviour change from the resolution itself.

## Issues

None new — integration only. The merged state closes BOTH register rows:
`(blockprop1)` by this branch and `(bestow1)` by main's bestow ticket.

---

## Round 11 — integration of main at 122a388c (merge commit 56811874)

Entry state: clean tree on `wt/cli-20260922T225142Z-e9128096` at `f14413fc`
(the prior round's merge of main at `835074e5`); no rebase or merge in flight,
no conflict markers. `main` had advanced to `122a388c`, so main was NOT an
ancestor of HEAD and this round integrated the newer main.

`git merge main --no-edit` conflicted in exactly two files; every other file
(it includes `AGENTS.md`, `effects/filter.go`, `rules/paramcensus_test.go`)
auto-merged:

- `internal/testutil/agentsdoc_test.go` — the `knownApproximationRows`
  ratchet comment and constant.
- `.ds4/report-mrg1.md` — this tracked report archive.

### `internal/testutil/agentsdoc_test.go`

Measured with the test's own algorithm (`| `-prefixed lines between
`## Known approximations` and the next `## `, header row dropped):

- merge base `835074e5`: 36 data rows
- HEAD `f14413fc`: 35 rows (this branch deleted the `(chosencopy1)`
  `CanBeTargetedByTriggeredSpellAbility` row)
- main `122a388c`: 35 rows (main deleted the `(blockprop1)`
  `stat:CantBlockUnless` row)
- merged worktree: **34 rows** (36 − 2; the merged register keeps neither)

Both sides' pre-merge comments said 35 — each matched only its own pre-merge
state, and main's comment additionally used a stale base of 37. Resolution
sets `knownApproximationRows = 34`, the merged table's actual count, with a
comment recording the measurement. The ratchet fails only on growth, so 34 is
the strictly correct merged value and removes the slack.

### `.ds4/report-mrg1.md`

This file is a tracked append-only archive of merge-resolver reports; main's
copy had accumulated ~3480 lines. The conflict was the `## Issues` region of
HEAD's report versus main's `## Issues` paragraph plus its "Round 10" section.
Resolved by keeping BOTH sides verbatim (HEAD's issues paragraph, then main's
issues paragraph and Round 10 section), preserving all archive content; no
information from either side was dropped.

<!-- main-side report, kept verbatim -->

# Round 11 — integration of main at 122a388c (this worktree, ticket cli-20260922T225142Z-9630515c)

Entry state: tree CLEAN at `ef38de87` (this branch's tip: the addphase ticket closing the
`(ap1)` row), no rebase or merge in flight. `main` had advanced to `122a388c`. Ran
`git merge main --no-edit`; two content conflicts, everything else auto-merged:

```
Auto-merging AGENTS.md
CONFLICT (content): Merge conflict in AGENTS.md
Auto-merging internal/testutil/agentsdoc_test.go
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
```

`.ds4/report-mrg1.md` auto-merged cleanly this round (no conflict).

### Measured ground truth (the register's own `| `-line counter, header dropped)

| tree | data rows | delta vs merge base `08a1d59a` (38) |
|---|---|---|
| HEAD (this branch) | 37 | deleted `(ap1)` |
| main | 35 | deleted `(attackprop1)`, `(blockprop1)`, `(bestow1)` |
| merged | **34** | 38 − 4 disjoint deletions |

Row-id diff of the merged table against main's set: identical except the `ap1` row
(this branch's deletion) — no stale resurrection this round.

### `AGENTS.md`

Both sides deleted disjoint rows of the frozen register from the same region, which is why
git conflicted. Resolution keeps NEITHER side's block: all four rows are removed
(38 − 4 = 34). Verified none of `ap1`, `attackprop1`, `blockprop1`, `bestow1` remains.

### `internal/testutil/agentsdoc_test.go`

HEAD said 37, main said 35 — each accurate for its own pre-merge tree, neither matching the
merged content. Set `knownApproximationRows = 34` with a comment recording the measurement
(base 38, four disjoint closures). `knownOversizeRows` untouched by both sides, stays 8.

### Commands and real output

- `git status` on arrival: clean at `ef38de87`; `main` tip `122a388c` not an ancestor.
- `git merge main --no-edit` → the two conflicts above; `.ds4/report-mrg1.md`,
  `rules/*`, `effects/*`, `cards/*`, `botpolicy/*`, `decision/*` all auto-merged.
- `gofmt -l internal/testutil/agentsdoc_test.go` → clean.
- `go test ./internal/testutil -run 'TestKnownApproximation' -v` →
  `TestKnownApproximationsOnlyShrinks PASS`, `TestKnownApproximationRowsAreShort PASS`,
  `ok ... 0.001s` (constant 34 = measured 34).
- `git add AGENTS.md internal/testutil/agentsdoc_test.go && git commit --no-edit` →
  merge commit `362e54d0` ("Merge branch 'main' into wt/cli-20260922T225142Z-9630515c").
- `git merge-base --is-ancestor main HEAD` → exit 0; `git status --short --branch` →
  `## wt/cli-20260922T225142Z-9630515c` clean.
- Post-merge ratchets:
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  → `ok github.com/adams-shaun/gorge/rules 0.775s` (5 top-level ratchets; no new
  `Mode$` matcher registered by the merged delta, no `addedAfterTheSplit` entry needed).
- `go test ./rules -run 'TestHeads|AddPhase|ExtraPhase'` →
  `ok github.com/adams-shaun/gorge/rules 1.804s` (chain heads unmoved; this branch's
  addphase behaviour still green on the merged tree).
- `.cards` present (symlink to the shared corpus), so no vacuous corpus-less run.

### Issues

None new — integration only. The only non-merge-commit edit is the ratchet constant and
its comment; the register shrinks 38 → 34 in the merged state, with all four closures
preserved. No engine behaviour was changed by the resolution itself.

### Resolver re-confirmation (2026-09-23)

This fresh resolver invocation found no active rebase/merge and a clean tree;
the reported conflict had already been resolved and committed as `362e54d0`,
with its report commit `6dc57239`. `122a388c` (the main tip named in that
integration record) is an ancestor of HEAD. The shared `main` ref has since
advanced to `a347d4130`; this re-confirmation did not start a second integration
because the dispatched conflict was the already-completed integration at
`122a388c`.

- `git status`: `On branch wt/cli-20260922T225142Z-9630515c`; nothing to commit,
  working tree clean.
- `ls .cards | head`: `cards.lock`, `cardsfolder`, `ir.gob.gz`, `ir.v4.gob.gz`,
  `tokenscripts` (corpus present).
- `go test ./internal/testutil -run 'TestKnownApproximation'`:
  `ok github.com/adams-shaun/gorge/internal/testutil 0.001s`.
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`:
  `ok github.com/adams-shaun/gorge/rules 0.866s`.
## Current integration round — main at 122a388c (2026-09-23)

The prior merge commit was clean, but main advanced from 06f2a294 to 122a388c. `git merge-base --is-ancestor main HEAD` showed main was not yet integrated, so I merged the current main tip. The merge conflicted in `.ds4/report-mrg1.md` and `internal/testutil/agentsdoc_test.go`; `AGENTS.md` and engine source auto-merged.

### Resolutions

- `internal/testutil/agentsdoc_test.go`: both versions had a stale value of 35. Counted the merged `AGENTS.md` table using the test's rule (`| ` lines inside Known approximations, minus the header): 34 rows. The branch's ExchangeLifeVariant closure and main's existing blockprop1/bestow1 closures are preserved. Set `knownApproximationRows = 34`.
- `.ds4/report-mrg1.md`: this tracked report path contains accumulated reports from sibling integrations on main. Preserved this branch's report verbatim and appended main's report verbatim under a separate heading, followed by this round's record.
- All other main changes auto-merged; no additional source conflict required manual changes.

### Commands and output

```text
git status
On branch wt/cli-20260922T225142Z-e9128096
nothing to commit, working tree clean

git merge main --no-edit
Auto-merging .ds4/report-mrg1.md
CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
Auto-merging AGENTS.md
Auto-merging effects/filter.go
Auto-merging internal/testutil/agentsdoc_test.go
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
Auto-merging rules/paramcensus_test.go
Automatic merge failed; fix conflicts and then commit the result.

# merged AGENTS.md Known approximations data rows (test's own algorithm)
data rows: 34

GIT_EDITOR=true git merge --continue
[wt/cli-20260922T225142Z-e9128096 56811874] Merge branch 'main' into wt/cli-20260922T225142Z-e9128096

git merge-base --is-ancestor main HEAD   # exit 0
git grep -n '^<<<<<<<\|^>>>>>>>' HEAD    # no matches

# .cards present as the shared-corpus symlink (cards.lock, cardsfolder, ir.gob.gz)
go test ./internal/testutil -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort'
ok  	github.com/adams-shaun/gorge/internal/testutil	0.001s

go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok  	github.com/adams-shaun/gorge/rules	0.770s

go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.205s

go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.161s
```

### Issues

No new unfixed defect found — this was integration only. The merged register
closes BOTH rows: `(chosencopy1)` by this branch's fix and `(blockprop1)` by
main. No engine behaviour was changed by the resolution itself.

### Note — main advanced during this seat (do not chase)

The dispatched conflict named `main` at `122a388c`; that was the `MERGE_HEAD`
this round integrated, and `122a388c` IS now an ancestor of this branch. While
the round was in progress `main` advanced again to `9f384719` (sibling ticket
`cli-20260922T225143Z-4b0bde0d`, the `api:ExchangeLifeVariant` row closure).

A read-only `git merge-tree` (no refs touched) shows a further integration of
`9f384719` would conflict in the SAME two files — `internal/testutil/agentsdoc_test.go`
(comment text only; both sides already carry `knownApproximationRows = 34`)
and `.ds4/report-mrg1.md` (the archive). That is the daemon's next integration
round, not this seat's: chasing a moving tip would mean re-resolving the same
comment each time main moves. When it is integrated, re-measure the merged
`AGENTS.md` table with the test's algorithm before setting the constant —
`9f384719` closes a third register row (`ExchangeLifeVariant`) in addition to
this branch's `(chosencopy1)` and main's `(blockprop1)`.

<!-- main-side report, kept verbatim -->

git status --short --branch
## wt/cli-20260922T225143Z-4b0bde0d

git merge main
CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go

Merged AGENTS.md Known approximations data rows: 34

 go test ./internal/testutil -run 'TestKnownApproximation'
 ok   github.com/adams-shaun/gorge/internal/testutil  0.001s

 go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
 ok   github.com/adams-shaun/gorge/rules  0.772s
```

The corpus symlink `.cards` is present. The ratchet checks passed. No engine behaviour was changed during this conflict resolution. Main is integrated in merge commit `2428b899`; the worktree was clean immediately after completion.

## Issues

No new unfixed issue found while resolving this integration conflict.

---

## Record — this integration round (merge-resolver dispatch, 2026-09-23)

### Situation found

`git status` showed the branch `wt/cli-20260922T225143Z-bc326d39` CLEAN at
`d792d1cf` — no rebase or merge in flight; both of the daemon's attempts
(rebase at `56f98b13`, then its merge fallback) had been aborted before this
seat started. Main was at `9f384719`, 48 commits ahead. Redid the integration
as `git merge main`, which reproduced exactly the daemon's merge-fallback
conflict set: `.ds4/report-mrg1.md` and `internal/testutil/agentsdoc_test.go`
(`AGENTS.md` and all code paths auto-merged, including `rules/cast.go`).

### Conflicts and resolutions

1. **`internal/testutil/agentsdoc_test.go`** — the `knownApproximationRows`
   constant only: HEAD said 35 (comment about a 35-row merged snapshot), main
   said 34. Both comments described stale pre-merge snapshots, so I measured
   the auto-merged `AGENTS.md` with the test's own counting rules: 34 `| `
   lines inside the `## Known approximations` section = **33 data rows**.
   Verified by three-way arithmetic against the merge base `835074e5`
   (36 rows): the branch deleted the `kw:Infect` row (this ticket's
   damage-cost-LKI closure, base row 36), main deleted the
   `api:ExchangeLifeVariant` row (ticket 4b0bde0d's closure, base row 37) and
   the `(blockprop1)` row (base row 21); 36 − 3 = 33, all disjoint. Set the
   constant to 33 with a corrected comment. `knownOversizeRows` (8) and
   `standInCellLimit` (600) are identical on all sides; the merged table has
   4 oversize rows, under the cap.
2. **`.ds4/report-mrg1.md`** — the append-only round-record archive: each side
   had inserted a different record at three archive positions (this ticket's
   own rounds vs tickets 4b0bde0d / 62421b89 / 1b1182e4 carried by main).
   Kept BOTH sides' records verbatim at each hunk (removed only the markers,
   inserting `---` separators, per the established convention), then appended
   this record. No historical record was discarded.

No engine-behaviour choice was involved; the branch's reviewed fix
(`56f98b13` + `e748019d`, damage keyword LKI at the two cost sites) and main's
changes to `rules/cast.go` auto-merged.

### Commands and outputs

- `git merge main` → `CONFLICT (content): Merge conflict in .ds4/report-mrg1.md`,
  `Auto-merging AGENTS.md`,
  `CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go`,
  `Automatic merge failed; fix conflicts and then commit the result.`
- Row measurement: `awk` walk over `AGENTS.md` (same rule as
  `approximationRows`): `total | lines: 34 data rows: 33`.
- `.cards` check: `[ -e .cards ]` → present (real corpus; runs not vacuous).
- `go test ./internal/testutil -run 'TestKnownApproximations'` → PASS (below).
- Post-merge ratchets (2026-09-22 instruction) → PASS (below).

- `go test ./internal/testutil -run 'TestKnownApproximation' -v` →
  `--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)` /
  `--- PASS: TestKnownApproximationRowsAreShort (0.00s)` / `ok`.
- `go test ./rules -run 'TestDamageYouCostUsesSourceDamageKeywordLKI|TestUnlessDamageCostUsesSourceDamageKeywordLKI' -v`
  (the branch fix's own tests, now composed with main's `rules/cast.go`) →
  both PASS, `ok github.com/adams-shaun/gorge/rules 0.646s`.
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'`
  → `ok github.com/adams-shaun/gorge/rules 0.771s`.
- `go test ./rules -run 'TestHeads'` → PASS (2.19s) — the golden chain heads
  reproduce on the merged tree; no head movement.
- `go test ./rules -run 'TestEveryRepoDeckIsFullySupported|TestRepoDecks'` →
  `ok github.com/adams-shaun/gorge/rules 1.815s`.
- `GIT_EDITOR=true git merge --continue` →
  `[wt/cli-20260922T225143Z-bc326d39 63c8aedf] Merge branch 'main' into wt/cli-20260922T225143Z-bc326d39`;
  final `git status --short` → empty.

### Verification notes

`.cards` is present (real symlink), so none of the corpus-backed runs was a
vacuous skip; the multi-second runtimes corroborate. The branch registers no
new `Mode$` trigger matcher and closes no register row beyond `kw:Infect`
(deleted), so nothing needed adding to `addedAfterTheSplit` or re-hulling.

## Issues

None found in this round beyond the resolved conflicts: the only defects
touched were the stale ratchet constants on both sides, and the row-count
dispute was settled by measuring the merged table (33) rather than adopting
either side's comment. No new approximation, no golden edit, no engine
behaviour change.

---

# Merge-conflict resolution — task cli-20260922T225142Z-71f376c3

## Entry state

The worktree entered CLEAN with no rebase or merge in flight. The daemon's
rebase attempt (`error: could not apply 0597251f ... fix(effects): end the
one-shot Effect from its Triggers$ body and widen shadow`) had been aborted,
and its merge fallback had also been aborted before landing — `git status` was
empty and `git log -1` was the branch tip `0597251f`, with `main` at
`a347d413` not an ancestor of HEAD. Integration still needed completing. The
branch already carries merge commits, so a plain merge (not a rebase) is the
consistent operation.

## Conflicted file

Exactly one real content conflict: **`internal/testutil/agentsdoc_test.go`**,
in the `knownApproximationRows` constant's comment-and-value block. All other
files the daemon named (`AGENTS.md`, `effects/filter.go`, `effects/misc.go`,
`rules/engine.go`, `rules/layers.go`, `rules/stack.go`, `state/continuous.go`)
auto-merged cleanly.

### What each side wanted

- **Main side (`a347d413`)** asserted `knownApproximationRows = 33`, with a
  comment saying its own auto-merged AGENTS.md measured 33 data rows and that
  from the 36-row base it deleted `kw:Infect`, `api:ExchangeLifeVariant`
  (ticket 4b0bde0d) and `(blockprop1)`.
- **Branch side (`0597251f`)** asserted `knownApproximationRows = 37`, with a
  comment written when the branch still measured 38 rows pre-main; the branch
  commit deleted the `(choosesource1)` row and lowered 38 -> 37.

Both comments were stale snapshots of their own pre-merge table. The merged
`AGENTS.md` (already auto-merged, the conflict was only in the Go constant)
measures **32** data rows: main's 33 minus the branch's `(choosesource1)`
deletion. I measured it with the exact scan the test uses
(`/^\| /` inside the `## Known approximations` region, minus the header row)
and also cross-checked main's own table at 33/constant 33, which confirms the
one-row delta is entirely the branch's deletion.

### Resolution

Kept main's explanatory style, updated to the measured truth:
`knownApproximationRows = 32`, with a comment naming main's 33-row table, the
branch's one further deletion, and that both side comments (37 and 33) were
stale. No other line touched.

## Commands and output

```
$ awk '/^## Known approximations/{i=1;next} i&&/^## /{i=0} i&&/^\| /{r++} END{print r}' AGENTS.md
33            # matched lines incl. header -> 32 data rows
$ awk ... /tmp/main-agents.md
34            # main's table: 33 data rows (constant 33) -> confirms the delta
```

```
$ git merge main --no-edit
Auto-merging AGENTS.md
Auto-merging effects/filter.go
Auto-merging effects/misc.go
Auto-merging internal/testutil/agentsdoc_test.go
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
...
Automatic merge failed; fix conflicts and then commit the result.
$ go test -run 'TestKnownApproximation' ./internal/testutil/
ok  github.com/adams-shaun/gorge/internal/testutil  0.001s
$ GIT_EDITOR=true git commit --no-edit
[wt/cli-20260922T225142Z-71f376c3 5d0ac9fd] Merge branch 'main' into wt/cli-20260922T225142Z-71f376c3
$ git status
nothing to commit, working tree clean
```

Post-merge verification (all from the merged tree):

```
$ go build ./...
exit=0
$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok  github.com/adams-shaun/gorge/rules  0.977s
$ go test ./internal/testutil/
ok  github.com/adams-shaun/gorge/internal/testutil  1.360s
$ go test ./rules/ -run 'TestEffectTriggerBodySelfExile|TestEffectChainSelfExile' -v
--- PASS: TestEffectTriggerBodySelfExileEndsTheEffect
--- PASS: TestEffectTriggerBodySelfExileLeavesPrintedStatics
--- PASS: TestEffectChainSelfExileEndsTheEffect
ok  github.com/adams-shaun/gorge/rules  0.018s
$ go test ./effects/ -run 'TestSourceScopedSelfExile|TestColourSourcePredicates|TestShadowPredicates' -v
--- PASS: TestSourceScopedSelfExileEndsOnlyEffectRegistrations
--- PASS: TestColourSourcePredicatesMatch
--- PASS: TestShadowPredicatesMatch
ok  github.com/adams-shaun/gorge/effects  0.006s
$ go test ./internal/archtest/
ok  github.com/adams-shaun/gorge/internal/archtest  3.308s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  github.com/adams-shaun/gorge/cmd/botbench  1.684s
```

`.cards` was present as a real symlink (`-> /home/sadams/projects/gorge/.cards`),
so no corpus-backed run was a vacuous skip; the multi-second runtimes on
`testutil` and `archtest` corroborate.

## Ratchet movement

No trigger mode is registered by either side of this resolution; no
`addedAfterTheSplit` entry was needed. The only ratchet touched is the
`knownApproximationRows` constant, lowered/measured to 32 (the merged table).
No `knownUnsupported`, `knownUnsupportedParams` or `knownUnmodelledCountHeads`
entry needed changing: this branch's fix (`0597251f`) closes the
`(choosesource1)` row (deleted) and touches no repo-deck ratchet entry, and
main's closures were already reflected on the main side. `TestHeads` was not
re-run here (daemon-only), but nothing in the resolution changes engine
behaviour — the conflict was a documentation constant only.

## Deviations from the brief

None. The conflict did not force any change outside
`internal/testutil/agentsdoc_test.go`.

## Issues

None found in this round. The only defect encountered was the stale ratchet
constant on each side, resolved by measuring the merged table (32) rather than
adopting either side's comment. No new approximation, no golden edit, no engine
behaviour change.

---

## Round 12 — integration of main at 62ae4746 (merge commit 70db5942)

Entry state: clean tree on `wt/cli-20260922T225142Z-e9128096` at `ff6d65f7`
(the round-11 merge of main at `122a388c`); no rebase or merge in flight — the
daemon had aborted its failed rebase. `main` had advanced to `62ae4746`, so
main was NOT an ancestor of HEAD. Integrated with `git merge main --no-edit`.

Conflicted in exactly three files; all engine source (including
`effects/filter.go`, `effects/registry.go`) auto-merged:

- `AGENTS.md` — one conflict region: HEAD carried the `(choosesource1)` row,
  main carried the `(chosencopy1)` row. Measured against the merge base
  `122a388c` (35 data rows): the branch's fix `6c86af9b` deleted the
  `(chosencopy1)` `CanBeTargetedByTriggeredSpellAbility` row (HEAD = 34) and
  main deleted the `(choosesource1)` one-shot-Effect row, the
  `api:ExchangeLifeVariant` row and the `(kw:Infect)` row (main = 32). Both
  sides' closures are delete-only register moves, so the merged table keeps
  NEITHER conflicted row: **31 data rows**, measured with the test's own
  algorithm (`| `-prefixed lines inside the section, header row dropped;
  verified against 35 − 4 = 31).
- `internal/testutil/agentsdoc_test.go` — the `knownApproximationRows`
  constant. Both sides' comments described stale snapshots of their own
  earlier merges (branch comment said 34/base 835074e5; main's comment said
  32 for the 71f376c3 merge). Resolution sets `knownApproximationRows = 31`,
  the merged table's measured count, with a comment naming all four
  deletions. `knownOversizeRows` stays 8 (fail-on-growth ceiling; the merged
  table measures 4 oversize rows — RevealAllValid, combatrestriction1,
  kw:MayFlashSac, kw:Flanking).
- `.ds4/report-mrg1.md` — append-only report archive; three conflict regions
  (an `## Issues` pair, a Round-11/Current-round pair, and the Commands
  blocks). Resolved by keeping BOTH sides verbatim (HEAD body, then main's
  body under a verbatim marker); no archive content dropped.

### Commands and output

```text
git status            # arrival: clean at ff6d65f7, nothing in flight
git merge main --no-edit
  CONFLICT: .ds4/report-mrg1.md, AGENTS.md, internal/testutil/agentsdoc_test.go
  (effects/filter.go, effects/registry.go and all other source auto-merged)
row counts (test's algorithm): base 122a388c = 35, HEAD = 34, main = 32, merged = 31
git add -f .ds4/report-mrg1.md && git add AGENTS.md internal/testutil/agentsdoc_test.go
git commit --no-edit  ->  70db5942 Merge branch 'main' into wt/cli-20260922T225142Z-e9128096
git status            # clean

go test ./internal/testutil/
  ok github.com/adams-shaun/gorge/internal/testutil 1.167s
go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
  ok github.com/adams-shaun/gorge/rules 0.744s
go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
  ok github.com/adams-shaun/gorge/cmd/botbench 1.419s
go test ./internal/archtest/
  ok github.com/adams-shaun/gorge/internal/archtest 3.874s
```

`.cards` was present as the shared-corpus symlink from the worktree's
creation, so the rules run was not a vacuous corpus-skipped one (0.744s is in
line with round 11's own 0.781s for the identical command). The 20-game
botbench pinned split did not move under the merged engine.

## Issues

None new — integration only. The merged state closes BOTH conflicted register
rows: `(chosencopy1)` by this branch's Feather/target-legality fix and
`(choosesource1)` by main's one-shot-Effect ticket, on top of main's
`api:ExchangeLifeVariant` and `(kw:Infect)` closures already carried in.
# Round 12 — integration of main at 62ae4746 (this worktree, ticket cli-20260922T225142Z-9630515c)

## Situation found

Entry state: tree CLEAN at `bbcd6c84`, no rebase or merge in flight (the daemon had
aborted its `rebase onto main` attempt — the dispatch's "Rebasing (1/3)" text). The
round-11 dispatch had already been resolved and confirmed, but `main` had advanced
again since: `122a388c` → `62ae4746` (the `71f376c3` one-shot-Effect/choosesource1
ticket, plus `bc326d39` infect-LKI and `4b0bde0d` life-exchange merges), committed
09:48:27Z, and the daemon re-dispatched this resolver at 09:48:37Z. A fresh
integration was genuinely owed — unlike round 11's re-confirmation, main's tip was
new content, not the base of an already-landed merge.

## Resolution

`git merge main --no-edit` → exactly two content conflicts, everything else
auto-merged (`AGENTS.md` composed the row deletions cleanly this time):

```
Auto-merging .ds4/report-mrg1.md
CONFLICT (content): Merge conflict in .ds4/report-mrg1.md
Auto-merging AGENTS.md
Auto-merging internal/testutil/agentsdoc_test.go
CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go
```

### `internal/testutil/agentsdoc_test.go`

HEAD said 34, main said 32 — each accurate for its own pre-merge tree. MEASURED the
auto-merged AGENTS.md with the test helper's own rule: **31 data rows**. Lineage:
merge base `122a388c` carried 35; disjoint deletions compose — this branch deleted
`(ap1)` (ef38de87); main deleted `(choosesource1)` (71f376c3),
`api:ExchangeLifeVariant` (4b0bde0d, b5f51b7d) and the `kw:Infect`
damage-cost-LKI row (bc326d39, 56f98b13): 35 − 4 = 31. Set the constant to
31 with that lineage in the comment. `knownOversizeRows` untouched (8).

### `.ds4/report-mrg1.md`

Both sides are append-only report ledgers diverging at the same tail point; kept
BOTH (HEAD's round-11 report, then main's appended sibling-worktree rounds),
deleting only the three marker lines. No content dropped from either side.

## Commands and output

- `git status` on arrival: clean, nothing in flight; main tip `62ae4746` not an
  ancestor of HEAD.
- `git merge main --no-edit` → the two conflicts above.
- Merged AGENTS.md row count: 31 (awk walk identical to `approximationRows()`).
- `git add internal/testutil/agentsdoc_test.go && git add -f .ds4/report-mrg1.md`
  (`.ds4` is gitignored but the file is tracked) `&& git commit --no-edit` →
  merge commit `03671f96`.
- `git status`: clean; `git merge-base --is-ancestor main HEAD` → exit 0.

## Verification

`.cards` present (symlink to the shared corpus) — no vacuous run.

```
$ go test ./internal/testutil -run 'TestKnownApproximation'
ok  github.com/adams-shaun/gorge/internal/testutil  0.001s

$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|AddPhase'
ok  github.com/adams-shaun/gorge/rules  0.747s

$ go test ./internal/archtest/
ok  github.com/adams-shaun/gorge/internal/archtest  3.374s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  github.com/adams-shaun/gorge/cmd/botbench  1.645s
```

The botbench 20-game pinned split did not move. Conflict-marker sweep over
AGENTS.md, the test file and the report ledger: clean. `gofmt -l` clean. The
branch's addphase fix (ef38de87) is unchanged by the merge
(`git diff 362e54d0 HEAD -- effects/addphase.go rules/turn.go` → empty).

## Deviations / uncertainties

None material. The only judgement calls: (1) the constant's value — measured, not
inherited, so no gate can fail on it; (2) integrating the NEW main rather than
re-confirming the old integration — the main tip post-dated the round-11
confirmation, so the daemon's next gate would otherwise conflict again.

## Issues

None new — integration only; no engine behaviour change by the resolution itself
(main's chosensource1/life-exchange/infect-LKI fixes arrive reviewed from their own
tickets).

---

## Round 12b — integration of main at `739aeca7` (merge commit `4a9e1acd`, this worktree, 2026-09-23)

### Situation found

The tree was CLEAN at `4117e21c` (the round-12 merge of main at `62ae4746`,
already resolved and committed by the prior seat, with its record appended
here); nothing in flight. `main` had advanced again to `739aeca7` — the
`9630515c` addphase ticket (`ef38de87`, closing the `(ap1)` row) landed on
main via `03671196`/`739aeca7` after that round — so a fresh integration was
owed. `.cards` present (shared-corpus symlink).

### The merge

`git merge main --no-edit` → exactly two content conflicts; all engine source
(including `effects/addphase.go`, `events/apply.go`, `rules/addphase_*.go`)
auto-merged:

- `internal/testutil/agentsdoc_test.go` — only the `knownApproximationRows`
  comment/constant. Both sides' constants said 31, each for its own pre-merge
  tree: HEAD's comment described the round-12 merge (base `122a388c` 35, four
  closures); main's comment described the `9630515c` sibling-worktree merge.
  The auto-merged AGENTS.md measures **30 data rows** with the test's own
  algorithm (base `122a388c` 35 − 5 disjoint deletions: this branch's
  `(chosencopy1)` closure `6c86af9b`, plus main's `(choosesource1)`
  `71f376c3`, `api:ExchangeLifeVariant` `4b0bde0d`/`b5f51b7d`,
  `(kw:Infect)` `bc326d39`/`56f98b13` and `(ap1)` `ef38de87`). Resolution
  sets `knownApproximationRows = 30` with that lineage in the comment.
  Merged table measures 4 oversize rows (unchanged set:
  RevealAllValid/combatrestriction1/kw:MayFlashSac/kw:Flanking), so
  `knownOversizeRows` stays 8.
- `.ds4/report-mrg1.md` — append-only archive; two conflict regions, both the
  same shape as every prior round (both sides appended their round-11 and
  round-12 records at the same tail point). Resolved by keeping BOTH sides
  verbatim and deleting only the six marker lines; no archive content dropped.

### Commands and output

```text
git status            # arrival: clean at 4117e21c, nothing in flight
git merge main --no-edit
  CONFLICT: .ds4/report-mrg1.md, internal/testutil/agentsdoc_test.go
  (AGENTS.md and all engine source auto-merged)
row counts (test's algorithm): base 122a388c = 35, HEAD = 31, main = 31, merged = 30
gofmt -l internal/testutil/agentsdoc_test.go   → clean
go test ./internal/testutil -run 'TestKnownApproximation' -v
  → TestKnownApproximationsOnlyShrinks PASS, TestKnownApproximationRowsAreShort PASS
git add AGENTS.md internal/testutil/agentsdoc_test.go && git add -f .ds4/report-mrg1.md
git commit --no-edit  →  4a9e1acd Merge branch 'main' into wt/cli-20260922T225142Z-e9128096
git status            # clean; git merge-base --is-ancestor main HEAD → exit 0

go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
  → ok github.com/adams-shaun/gorge/rules 0.768s
```

No new trigger `Mode$` matcher was registered by the merged delta, so no
`addedAfterTheSplit` entry was needed.

## Issues

None new — integration only. The merged register shrinks 35 → 30 with all five
closures preserved (this branch's `(chosencopy1)` plus main's four). No engine
behaviour was changed by the resolution itself; the addphase feature arriving
from main is main's own gated change.
