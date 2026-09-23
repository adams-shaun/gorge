# Report — playerspec-life-svar-threshold

## Changes

- `effects/filter.go`: Added `MatchesPlayerSpecWithSVars`, which resolves symbolic `life<OP><SVar>` thresholds with `EvalCountOK`, checks the context SVar table before the source face's table, and rewrites a resolved alternative into the existing literal player matcher. Literal comparison grammar is unchanged; unresolved or unmodelled values fail closed. No resolver closure was added to `Ctx`/`PlayerSpecCtx`.
- `effects/count.go`: Registered `OppGreatestLifeTotal` as the opponents' highest current life through `lifeExtreme`.
- `effects/choose_control.go`: Applied the SVar-aware matcher in ChoosePlayer's targeted and candidate walks, `controlPlayer`, and `repeatPlayers`.
- `effects/context.go`: Applied it to the `Defined$ Player.<qualifier>` bridge.
- `rules/trigmatch_combat.go`: Applied it to both `AttackedTarget$` and `AttackingPlayer$`, binding the trigger source face's SVars and the trigger controller (not the attacker) into the count context.
- `effects/defined_player_state_qualifier_test.go`: Removed `Player.lifeEQX` from the fail-closed list; its other entries are unchanged.
- New `effects/player_life_threshold_test.go`: Pins the real compiled `DBChoosePlayer` SA on The Master, Gallifrey's End (the compiled card whose rules text names Make Them Pay), the greatest-opponent count, the Defined bridge, literal/SVar/unresolvable thresholds, and the selected opponent. The brief's stated corpus lookup name `Make Them Pay` is not a registry card name, so the test uses its actual card name.
- New `rules/player_life_threshold_test.go`: Pins Breena's actual `AttackedTarget$ Opponent.lifeGTX` trigger, with a battlefield source and attacker, differing opponent life totals, then equal opponent life totals.

## Measurements and deviations

- `.cards` was present as a symlink to `/home/sadams/projects/gorge/.cards` before testing.
- Re-measured the raw grep prevalence: 7 files hit `life(GE|GT|EQ|LE|LT)[A-Z]`; `great_unclean_one` is the non-player-qualifier count-head cousin, leaving 6 real player-qualifier files as described in the brief.
- Current `AGENTS.md` has no `MatchesPlayerSpec`/fx20 approximation row (confirmed by searching it), despite the brief asking to edit that row. I did not add a row because AGENTS.md's closing register is frozen and this checkout contains no matching sentence to adjust.
- The compiled ChoosePlayer SA records the chosen player in `Ctx.Chosen`; it does not set `RememberChosen`, so the test asserts the actual chosen result rather than asserting an unpopulated `Ctx.Remembered` list. The picked seat is still proven to be seat 2, not first candidate seat 1.
- The required full-package `go test ./effects ./rules` and separate full `TestHeads|TestRepoDeckGamesReplayExactly|TestEveryRepoDeckIsFullySupported` command were not run: the task-agent test-budget instruction limits the seat to the named focused `-run` command. `TestHeads` is included in that focused command. The no-count botbench golden and archtest gates were run as directed.

## Fails without the fix

For the negative proof I saved all affected non-test files, disabled the SVar-aware matcher while leaving the call sites/build intact, removed the `OppGreatestLifeTotal` dispatch, and ran the three new feature tests. The command failed as expected; I restored the saved files and verified them with `cmp`.

```text
--- FAIL: TestMakeThemPayChoosesTheGreatestLifeOpponent (0.61s)
    player_life_threshold_test.go:22: Make Them Pay X = 0, false; want greatest opponent life 40, true
--- FAIL: TestLifeThresholdSVar (0.00s)
    player_life_threshold_test.go:48: lifeGTX failed to match exactly the seat above resolved threshold
FAIL
FAIL    github.com/adams-shaun/gorge/effects  0.638s
--- FAIL: TestBreenaAttackTriggerReadsLifeGTX (0.62s)
    player_life_threshold_test.go:49: Breena trigger did not match: params=map[AttackedTarget:Opponent.lifeGTX Execute:TrigDraw Mode:AttackersDeclaredOneTarget TriggerDescription:Whenever a player attacks one of your opponents, if that opponent has more life than another of your opponents, the attacking player draws a card and you put two +1/+1 counters on a creature you control. TriggerZones:Battlefield] sourceSVars=map[DBPutCounter:DB$ PutCounter | Choices$ Creature.YouCtrl | CounterType$ P1P1 | CounterNum$ 2 | ConditionCheckSVar$ PlayerCountDefinedTriggeredAttackedTarget$HasPropertylifeGTX TrigDraw:DB$ Draw | Defined$ AttackingPlayer | SubAbility$ DBPutCounter | ConditionCheckSVar$ PlayerCountDefinedTriggeredAttackedTarget$HasPropertylifeGTX X:PlayerCountOpponents$LowestLifeTotal] threshold=20/true filter=false alive=[0 1 2] ctrl=0 attacked=2 lives=40/20/30
FAIL
FAIL    github.com/adams-shaun/gorge/rules  0.664s
FAIL
```

## Gates run

Focused required command:

```text
go test -run 'TestMakeThemPayChoosesTheGreatestLifeOpponent|TestBreenaAttackTriggerReadsLifeGTX|TestLifeThresholdSVar|TestDefinedPlayerStateQualifier|TestChoosePlayer|TestHeads' ./effects ./rules
ok   github.com/adams-shaun/gorge/effects (cached)
ok   github.com/adams-shaun/gorge/rules (cached)
```

Other required gates:

```text
go vet ./effects ./rules
[no output; exit 0]

gofmt -l effects/filter.go effects/count.go effects/choose_control.go effects/context.go effects/defined_player_state_qualifier_test.go effects/player_life_threshold_test.go rules/trigmatch_combat.go rules/player_life_threshold_test.go
[no output; exit 0]

go run ./cmd/gentypes -check
[no output; exit 0]

go test -count=1 ./internal/archtest/
ok   github.com/adams-shaun/gorge/internal/archtest  3.369s

go test -count=1 -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok   github.com/adams-shaun/gorge/cmd/botbench  1.193s
```

`TestHeads` passed in the focused command; the botbench default golden passed unchanged. No chain-head, deck-ratchet, or botbench split movement was observed.

## Issues

Unfixed out-of-scope issues carried from the brief:

1. `galactus_devourer_of_worlds`'s `MustAttack$ Opponent.lifeEQX` remains inert: no rules/effects reader consumes the defender parameter, and the `MustAttack` static whitelist rejects that extra parameter. The player-filter fix does not address either gap.
2. `your_inescapable_doom` and `the_mighty_will_fall` scheme triggers remain unreachable because `rules/trigger_match.go`'s `forEachObject` scans `ZLibrary..ZStack`, not `ZCommand`.
3. `celestial_convergence` remains unsupported: `DB$ WinsGame` is not registered and `PlayerCountPlayers$TiedForHighestLife` is unimplemented.
4. Make Them Pay's `DB$ VillainousChoice` is still not a registered effect; this change covers its preceding ChoosePlayer only.
5. Breena's `PlayerCountDefinedTriggeredAttackedTarget$HasPropertylifeGTX` condition head is unimplemented. It fails open under the existing CheckSVar convention; the trigger's `AttackedTarget$` filter is what the new pin verifies.
6. `great_unclean_one`'s `PlayerCountOpponents$HasPropertylifeLTCount$YourLifeTotal` nested-Count count-head spelling remains unimplemented; it is the separate count-head cousin, not a player qualifier.

The scheme/celestial/Breena/count-head forms are distinct follow-up work and were not changed here.
