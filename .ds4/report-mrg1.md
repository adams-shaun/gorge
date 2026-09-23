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
