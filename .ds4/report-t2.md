# Report — Verify and pin multiple MustBlock blockers

## Changes

Restored `.ds4/report-t1.md` byte-for-byte from the parent of `80d29498`, undoing that commit's unrelated destructive rewrite (review finding). This round's report is only in `.ds4/report-t2.md`. No production files or tests changed. The existing `TestMustBlockTwoWatchdogsShareAttacker` regression test is already present in `rules/mustblock_min_team_test.go` and the fix is already landed in `4fe4eadc` (`fix(rules): satisfy MustBlock with legal whole blocking teams`). No ratchet, allowlist, or Known-approximations entry changed.

`.cards` is a present symlink to the real corpus in this worktree, so the corpus-dependent test was not silently skipped for lack of corpus. `/usr/bin/grep -rlE 'Mode\\$ MustBlock' .cards/cardsfolder | wc -l` returned `27`, matching the brief.

## Gates run

Exact targeted command from the brief:

```text
$ go test -run 'TestMustBlockTwoWatchdogsShareAttacker$' ./rules/ 2>&1 | tail-30
ok   github.com/adams-shaun/gorge/rules (cached)
```

```text
$ go test ./internal/archtest/ 2>&1 | tail -15
ok   github.com/adams-shaun/gorge/internal/archtest (cached)

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -5
ok   github.com/adams-shaun/gorge/cmd/botbench (cached)
```

All three gates passed in this round. The Go test cache returned the results as shown; no code or tests were changed during this verification.

## Fails without the fix

Not applicable: no test was added. The already-existing regression test is part of the fixing commit `4fe4eadc`; this task did not revert or alter production code.

## Issues

This defect is already fixed by `4fe4eadc`. No other defect was investigated or fixed. The reported prevalence of 27 corpus files describes the mechanic, not a remaining defect; no acceptance census or approximation entry requires a change.
