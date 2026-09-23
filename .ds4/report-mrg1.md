# Merge-conflict resolution — task cli-20260922T225140Z-8b855197 (fresh relaunch)

## Situation found

`git status` on entry: clean tree on `wt/cli-20260922T225140Z-8b855197` at
`739472a5`. The reflog showed the daemon's integration attempts — four
`rebase (start): checkout main` each ending in `rebase (abort)` — leaving no
in-flight operation. `main` had advanced past the previous resolver's base
(`f7357075`) to `4357caf3` with three new commits:

- `f43df9fc` fix(rules): let a resolving effect's name filter read SetName$ renames
- `cb6b0007` fix(effects): refresh SetName snapshots between abilities
- `4357caf3` merge(cli-20260922T225140Z-72e1251c): approx: layer-3 SetName$ row closure

The branch carried 4 commits (the reviewed fix + 3 follow-ups, including the
`739472a5` removal of the obsolete multikicker decline assertion that had
failed the earlier gate run).

I re-ran `git rebase main`.

## Conflicted files

### `internal/testutil/agentsdoc_test.go` (the only conflict)

Both sides edited the same single line — the `knownApproximationRows`
constant.

- **Main side**: `knownApproximationRows = 57` (main's own table has 55 rows —
  the constant is loose by 2 on main, a pre-existing looseness inherited from
  the base constant).
- **Branch side**: `knownApproximationRows = 53` (the previous resolver's
  exact-count resolution of this same conflict at the older main base).

**Resolution: `knownApproximationRows = 52`.** The counted row count of the
merged AGENTS.md is authoritative, and both sides' deletions are disjoint and
coexist (verified below):

- merged table = **52 rows** (branch's 53 minus main's 1 newly deleted
  SetName row — exactly the "lower by the number of rows your change deletes"
  arithmetic from the branch's truthful 53).
- 52 makes the constant exact; the test then passes with no "table is down to
  N" looseness log.

### `AGENTS.md` — auto-merged, no conflict

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
