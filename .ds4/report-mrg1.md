# Merge-conflict resolution — task cli-20260922T225140Z-8b855197

## Situation found

`git status` on entry showed a clean tree on
`wt/cli-20260922T225140Z-8b855197` at `2af8ffff`. The reflog showed the
daemon's integration attempt had been `git rebase main` -> conflict ->
`rebase (abort)`, leaving no in-flight operation. `main` was **not** an
ancestor of HEAD: 4 commits missing from main, 2 commits on the branch.

I re-ran `git rebase main` and resolved the conflict it raised.

## Conflicted files

### `internal/testutil/agentsdoc_test.go` (the only conflict)

Both sides edited the same single line — the `knownApproximationRows`
constant, the ratchet that counts rows in AGENTS.md's frozen "Known
approximations" table.

- **Merge base** (`f8496199`): `knownApproximationRows = 60`.
- **Branch side** (commit `fe5bde46`): `60 -> 57`. That commit deletes the
  three rows this ticket closes: the `KReplacement` order row, the
  `(multikicker1)` row, and the `(mutate1)` row.
- **Main side** (merge `f7357075`, via `c93d85f7` / `5b267585`): `60 -> 58`.
  That work deletes two different rows: the `Scry` pile-B row and the
  `Surveil` pile-B row.

The deletions are **disjoint**, so both sides' intents must coexist. The
counted number is authoritative, not arithmetic off the stale base
constant: the base constant was already 2 above the table it actually had
(base table = 58 rows; the test only fails on GROWTH, so the loose constant
was live). Rebased onto main, AGENTS.md has:

- base table 58 rows
- minus main's 2 deleted rows = 56
- minus the branch's 3 deleted rows = **53**

I set `knownApproximationRows = 53`, matching the real counted row count
(not a computed delta off the stale base constant).

**Resolution text:**

```go
	// knownApproximationRows is the number of data rows in the table. Lower it
	// by exactly the number of rows your change deletes. NEVER raise it.
	knownApproximationRows = 53
```

### `AGENTS.md` — auto-merged, no conflict

Git auto-merged it: main deleted its two Scry/Surveil rows (one region) and
the branch deleted its three rows (different regions), so all five
deletions coexist. Verified all five quoted row texts are absent from the
working tree, and that the file carries exactly the branch's 3 deletions
relative to `main` (main's 2 are already in `main`).

### `botpolicy/policy.go` — auto-merged, no conflict

Main's +10-line hunk (the `KArrange` `Intent.Rest` repair-fragile guard in
`Clamp`) is preserved (verified present at line ~1136), and the branch's
three new arms (`KReplacement`, `multikick`, `mutate_place`) are present.

## Other files

`botpolicy/approx_arms_test.go`, `botpolicy/replacement.go` (branch-added)
and `rules/botpolicy_approx_test.go` (branch's second commit) applied
cleanly; main's other changes (`rules/arrange.go`, `decision/`, `web/`,
`effects/cardflow.go`, etc.) were untouched by the branch and carried
through the rebase.

## Commands run (real output)

```
$ git status
On branch wt/cli-20260922T225140Z-8b855197
nothing to commit, working tree clean

$ git log --oneline -3
3c8f04b3 test(botpolicy): cover approximation arms with corpus cards
6a7f336b fix(botpolicy): add real arms for replacement order, multikick count and mutate placement
f7357075 merge(cli-20260922T225139Z-ef2bd366): approx: Scry's bottom pile and Surveil's graveyard pile go in the OFFERE
```

Row-count check on the resolved AGENTS.md (same logic as the test):

```
$ awk '/^## Known approximations/{inside=1;next} inside&&/^## /{inside=0} inside&&/^\| /{c++} END{print c-1}' AGENTS.md
53

$ grep -n 'knownApproximationRows =' internal/testutil/agentsdoc_test.go
22:	knownApproximationRows = 53
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
(No "table is down to N" log -> the row count EQUALS the constant exactly.)

Ratchets main newly carries (the brief's command):

```
$ go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
ok  	github.com/adams-shaun/gorge/rules	0.776s
```
verbose: 5 RUN / 5 PASS / 0 SKIP
(`TestEveryRepoDeckIsFullySupported`, `TestEveryRepoDeckCountHeadResolves`,
`TestEveryRepoDeckParamsAreRead`, `TestEveryDispatchedTriggerModeHasAMatcher`,
`TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched`).

Branch's own tests (changed packages):

```
$ go test -run 'TestBotReplacementOrderRanksBySourceWorth|TestBotReplacementOrderBypassesSkip|TestBotMultikickPaysTheAffordableMaximum|TestBotMutatePlacesUnder' ./botpolicy/
ok  	github.com/adams-shaun/gorge/botpolicy	0.005s

$ go test -run 'TestBotPolicyCorpusReplacementOrder|TestBotPolicyCorpusMultikickerPaysMaximum|TestBotPolicyCorpusMutatePlacesUnder' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.666s
```
verbose: 4 PASS + 3 PASS, 0 SKIP.

`gofmt -l` on the four touched Go files: no output (clean).

## Final state

- `git status`: on branch `wt/cli-20260922T225140Z-8b855197`, **working
  tree clean**.
- `main` is an ancestor of HEAD.
- No active rebase / merge (`MERGE_HEAD` absent, `rebase-merge` dir gone).
- Rebased commits: `6a7f336b` (fix) and `3c8f04b3` (test).
- `git diff --stat main..HEAD` = exactly the branch's intended changes
  (AGENTS.md -3 rows, 3 botpolicy files, rules test, agentsdoc constant).

## Uncertainties / notes

- The base constant `60` did not equal the base table's 58 rows; I resolved
  to the *counted* 53 rather than to `main's 58 - 3 = 55`. The count is what
  the ratchet compares against and what `TestKnownApproximationsOnlyShrinks`
  reports; 53 is the exact value that makes the constant truthful. If a
  reviewer expected pure delta arithmetic, that would have left the constant
  two above the table (still green, but wrong).
- No engine behaviour was changed by the resolution: only the constant, in
  the direction the branch intended. No goldens were edited.

STATUS=DONE
COMMITS=6a7f336b 3c8f04b3
TESTS=go test -run TestKnownApproximation ./internal/testutil/ (PASS); ratchet suite ./rules (5 PASS, 0 SKIP); botpolicy + rules approx tests (7 PASS, 0 SKIP)
