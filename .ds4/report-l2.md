# Implementation report — target reserve sickness correction

## What changed

- `botpolicy/target.go`: stopped excluding `Card.Sick` from basic-land mana sources. Summoning sickness does not prohibit a basic land's `{T}` mana ability (CR 302.6); this preserves the correct behavior of the pending-payment worst-case deduction while keeping sickness facts on `Card` for possible creature-source modeling.
- `botpolicy/target_sickness_test.go`: replaced the erroneous expectation that a sick basic source is unavailable. The test asserts the source is a sick battlefield basic producing green, demonstrates the reserve survives with it, and verifies removing it changes the outcome.

The pending payment implementation already present on this branch remains the shipped arm: deterministic worst-case deduction of colored pips, then generic mana allocated against reserve-pip colors, followed by reserve probes. `chooseTargets` resolves pending cost via `Decision.Source` and matching spell `StackEntry`.

## Fails without the fix

Reverted `botpolicy/target.go`'s sick-source inclusion while running the seat integration regression; restored the file byte-identically (`cmp` succeeded):

```text
$ go test ./seat/ -run '^TestBotReserveCountsJustPlayedLand$'
--- FAIL: TestBotReserveCountsJustPlayedLand (0.00s)
    bot_reserve_test.go:133: chose option [2] (obj [202]), want option 1 (the killable 2/2, obj 201): a just-played basic land is tappable and must not be dropped from the reserve
FAIL
```

## Gates and evidence

`.cards` was present (not skipped corpus tests).

```text
$ go test -run 'TestTargetSpareMana|TestFT1|TestTargetEffectValueKillOnlyWithSpareMana' ./botpolicy/
ok   github.com/adams-shaun/gorge/botpolicy 0.572s

$ go test ./seat/ -run '^TestBotReserveCountsJustPlayedLand$'
ok   github.com/adams-shaun/gorge/seat (cached)

$ go test ./internal/archtest/
ok   github.com/adams-shaun/gorge/internal/archtest (cached)

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok   github.com/adams-shaun/gorge/cmd/botbench 0.812s

$ gofmt -l botpolicy/target.go botpolicy/target_sickness_test.go
[no output]

$ go run ./cmd/gentypes -check
[no output; exit 0]
```

The original claim-A measurement in this branch's regression tests verifies three Forests, `{G}{G}` reserve, and pending `{1}{G}` are not spare, while a one-mana pending cost is allowed when the reserve survives. The botbench gate passed unchanged; no split movement observed.

## Issues

- A target ask with `Decision.Source == 0` (notably an activated ability) has no readable pending payment, so reserve ranking uses the no-pending-cost path. This is intentionally not addressed because the ability object is not yet on the stack at the ask.
- Basic-source filtering cannot see externally prohibited activations (`cantActivate` statics). Corpus prevalence measured with `/usr/bin/grep -rlE 'CantActivate' .cards/cardsfolder | wc -l`: **2** files (`b/braided_net_braided_quipu.txt`, `x/xathrid_gorgon.txt`). No new engine facts were threaded, as scoped.
- The claimed “sick basic is dead mana” premise is incorrect under CR 302.6; the review regression exposed this and this round corrects it. No other unaddressed defects were found.
