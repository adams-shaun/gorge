# Merge-conflict resolution — wt/agent-20260919T055356Z-504b1359 (mrg1)

STATUS: DONE.

The daemon's rebase onto `main` conflicted and its merge fallback also
conflicted. I found the tree clean (rebase already aborted — reflog
`HEAD@{1}: rebase (abort)`), so I re-ran the merge fallback (`git merge main`),
resolved the three conflicted files, and concluded the merge with the default
merge message.

- Merge commit: `bfa400d8` (`Merge branch 'main' into wt/agent-20260919T055356Z-504b1359`),
  parents `ae1dc08c` (branch tip) + `767f3dd4` (main).
- Working tree clean; no unmerged paths; no conflict markers remain.

## Conflicted files and resolution

### `rules/replacement.go`

Two sides touch the same `effects.RegisterNonAPI(...)` call in `init()`:

- **HEAD (branch fix `6fcf1fcd`)** removed the closing paren and appended the
  `api:ReplaceDamage` census token plus a comment explaining it is handled
  inline by `applyReplaceDamageBody` (never through `effects.Resolve`).
- **main** added the new token `"repl:Scry"` to the earlier `"repl:Attached"`
  line (from the scry-replacement work: `ae4fed1e`/`8d1151fb`/`1dab341a`).

These are independent additions, not a contradiction. Resolution keeps BOTH:
main's `"repl:Scry",` on the `"repl:Attached"` line, and the branch's
`api:ReplaceDamage` + comment block after `"api:ReplaceCounter"`. Verified:
`grep -c '"repl:Scry"' rules/replacement.go` = 1, and the full
`api:ReplaceDamage` comment/registration is present.

Diff of `rules/replacement.go` vs main is exactly the branch's 7-line
addition (+6/-1), i.e. main's `repl:Scry` survives unchanged.

### `.ds4/report-sol1.md`

Both sides independently replaced the tail of this accumulated report file:

- base (`c4560130`) = 679 lines; main appended 71 lines (the Mill-trigger
  ticket's `# Mill-trigger replacement redirection — agent-20260919T183731Z-085022e9`
  report); HEAD appended 63 lines (the `# replcensus1 — ReplaceDamage
  census-token fix round` report).

Both are pure additions after the identical shared history and both are
legitimate historical record (the file is a shared accumulation across
tickets). Resolution is the UNION: base history + main's appended block +
HEAD's appended block. Both blocks verified fully present; base preserved
exactly once; no content from either side dropped.

### `.ds4/report-t1.md`

Same shape: base = 2162 lines; main appended 294 lines; HEAD appended 172
lines (the branch's report explicitly restored main's 2162-line history and
appended its own round-1 report). Union resolution: base + main's block +
HEAD's block. Both additions verified present, base preserved exactly once.

Note: `.ds4` is in `.gitignore` but these specific report files are tracked
(they were force-added historically), so I used `git add -f` to stage the two
resolved report files, consistent with their tracked status.

## Sanity checks (commands and real output)

`.cards` present as a symlink to `/home/sadams/projects/gorge/.cards`, so the
corpus tests exercised the corpus rather than skipping.

```text
$ gofmt -l rules/replacement.go
(no output)
$ go build ./rules/
build-exit=0
```

Branch ticket's focused tests plus the merged scry-replacement tests:

```text
$ go test -run 'TestReplaceDamage|TestScryReplacement' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.947s
exit=0
```

Ratchets main newly carries (merge instruction):

```text
$ go test -v ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead'
--- PASS: TestEveryRepoDeckIsFullySupported (0.70s)
--- PASS: TestEveryRepoDeckCountHeadResolves (0.00s)
--- PASS: TestEveryDispatchedTriggerModeHasAMatcher (0.00s)
--- PASS: TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched (0.00s)
--- PASS: TestEveryRepoDeckParamsAreRead (0.23s)
ok  	github.com/adams-shaun/gorge/rules	0.992s
exit=0
```

Behaviour goldens outside `rules/`:

```text
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	4.123s
arch-exit=0
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.571s
bot-exit=0
```

No head/ratchet golden moved. `rules/heads_test.go` and
`rules/acceptance_test.go` were not edited.

## Issues

No new defect surfaced by the conflict resolution; the conflicts were pure
integration collisions (one shared `init()` registration list and two shared
accumulated report files). No CR-lane test requested.
