# Scry / Temporal Anchor — hn1

Rebased the ticket's six commits onto `main` at `0c3b199d`, resolving conflicts in favour of main's independently landed, more complete scry implementation. Main already had a completed `events.Scry` record emitted **after** the KArrange answer in `rules/arrange.go`, a `ToBottom$ True` matcher in `rules/trigmatch_cards.go`, the real-card end-to-end `TestTemporalAnchorScryBottomTrigger`, the replacement proposal boundary, and deletion of the quoted approximation row. The previous branch's pre-arrangement marker and unconditional rejection of `ToBottom$` were removed; `git diff` against the rebased base retains only `rules/scry_order_choice_test.go`. No event ordinals or table rows were changed. Added the spell-origin Temporal Anchor regression, contrasting one bottomed card and an all-top choice; both check that the source is on the battlefield, distinct cards and a nonempty library remainder exist, the action is recorded only after the answer, and the triggered ability and its exile fire exactly once or not at all. Retained the existing replacement-order draw-count assertion. `.cards` was present as a symlink to the real corpus, not skipped. Commit: `3c5cb6ab` (preceded by the rebased historical ticket commits). Main advanced again during this round; unrelated later main changes are not part of this ticket.

## Fails without the fix

Temporarily removed the completed-scry emission from `rules/arrange.go`, ran the new test, and restored the file byte-for-byte (`cmp` succeeded):

```
$ go test -run '^TestScrySpellTemporalAnchorBottomChoice$' ./rules/
--- FAIL: TestScrySpellTemporalAnchorBottomChoice (0.39s)
    --- FAIL: TestScrySpellTemporalAnchorBottomChoice/bottom_one (0.39s)
        scry_order_choice_test.go:84: completed scry record = [], want one marker bottoming 1
    --- FAIL: TestScrySpellTemporalAnchorBottomChoice/keep_all_on_top (0.00s)
        scry_order_choice_test.go:84: completed scry record = [], want one marker bottoming 0
FAIL
FAIL github.com/adams-shaun/gorge/rules 0.409s
FAIL
exit=1
restored_identical
```

## Gates

```
$ go test -run 'TestScrySpellTemporalAnchorBottomChoice|TestScryReplacementOrderChangesDrawCount|TestTemporalAnchorScryBottomTrigger|TestKenessosReplacesScryCountBeforeLooking|TestEligethDrawsInsteadOfScrying|TestScryReplacementOrderChoiceAndResume' ./rules/
ok  github.com/adams-shaun/gorge/rules 0.479s
targeted_exit=0
$ go test ./internal/archtest/
ok  github.com/adams-shaun/gorge/internal/archtest 3.629s
arch_exit=0
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  github.com/adams-shaun/gorge/cmd/botbench 0.604s
bot_exit=0
$ gofmt -l rules/scry_order_choice_test.go
(no output)
$ go run ./cmd/gentypes -check
(no output)
$ git diff --check
(no output)
```

No repo-deck head or botbench movement measured; the botbench golden passed. The Scry registry ratchet and approximation-row removal had already landed on main (the older ticket-specific approximation row was absent); no additional row may be removed for the same fix.

## Issues

None found in this round. No remaining `ToBottom$ True` deviation: the pre-arrangement marker was superseded by main's completed-action marker, and the bottom and top answers are tested. The older report's alleged `ToBottom$` gap is closed by main's `rules/arrange.go` / `rules/trigmatch_cards.go` implementation. Main's subsequent unrelated row-count reductions should be retained by the integration merge.
