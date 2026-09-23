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
