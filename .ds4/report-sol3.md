# Animate Dead / api:Animate.RemoveKeywords — sol3

Rebased cleanly onto `main` (`b3523eab`) per controller directive before edits. `.cards` was present (real corpus symlink). The sole new MAJOR in `findings-sol3.md` was a report-path collision: `.ds4/report-sol2.md` belonged to the Attached-predicates task. Restored that file **byte-for-byte** from `main`; deleted the redundant `.ds4/report-sol2-attached.md` copy rather than relocating the sibling report. `git diff --quiet main -- .ds4/report-sol2.md` returned 0; `git diff --name-status main HEAD -- .ds4/` contains only this ticket's `.ds4/report-r2-animate-dead.md`. This report uses the unused ticket-unique `.ds4/report-sol3.md` path. No Go code or tests changed this round.

Previous MINOR findings: Attach Origin precedence was fixed and tested in `rules/animate_attach_origin_test.go`; the shared `.ds4/report-r2.md` remains untouched. Previous MAJOR (report overwrite) is now resolved, not just renamed. Earlier commits changed `effects/combatfx.go` (read RemoveKeywords), `effects/attach.go` (graveyard bearer), `effects/filter.go` (StrictlySelf presence gate), `rules/attach.go` (derived enchant and zone-positive off-battlefield SBA), `rules/stack.go` (Attach target zones with explicit Origin precedence), `rules/clone.go` (deep copy removal slice), and `rules/mayflashsac_test.go` (answer a real trigger-order ask). Existing main already provides the state/Derived layer-6 removal implementation. New tests are `rules/animate_dead_test.go`, `rules/animate_attach_origin_test.go`, `effects/animate_removekeywords_test.go`. No heads/ratchet edits.

## Fails without the fix

No new test this round. Existing leaves were proven against unfixed code with a byte-identical restore (`cmp`); excerpts of the earlier recorded failures:

```
--- FAIL: TestAnimateDeadGraveyardWindowSurvivesSBA
    the pre-reanimate window attachment was swept; CR 704.5m must admit the graveyard-enchant bearer
--- FAIL: TestAnimateDeadDerivedEnchantDrivesTheSBA
    the post-animate raw board was swept; the SBA read the printed enchant instead of the derived one
--- FAIL: TestAnimateDeadCastOfferedTargetsGraveyardCreature
    the graveyard-enchant cast was not offered with a legal graveyard target: [play_land ×6, pass, concede]
--- FAIL: TestAnimateRemoveKeywordsRegistersLayer6Removal
    RemoveKeywords = [], want the RemoveKeywords$ read
--- FAIL: TestAnimateRemoveKeywordsOnlyBodyStillRegisters
    continuous = [], want one layer-6 effect for the removal-only body
--- FAIL: TestAnimateDeadReanimateChainCompletes
    the reanimate chain never returned the enchanted card to the battlefield
```

Origin-precedence test, with `rules/stack.go` replaced by `main` and restored by `cmp`:

```
--- FAIL: TestAnimateAttachExplicitOriginOutranksInZone/graveyard_origin_beats_exile_filter
    targetZones(Origin="Graveyard", ValidTgts="Creature.inZoneExile", TgtZone="") = [battlefield], want [graveyard]
--- FAIL: TestAnimateAttachExplicitOriginOutranksInZone/exile_origin_beats_graveyard_filter
    targetZones(Origin="Exile", ValidTgts="Creature.inZoneGraveyard", TgtZone="") = [battlefield], want [exile]
--- FAIL: TestAnimateAttachExplicitOriginOutranksInZone/no_origin_infers_graveyard
    targetZones(Origin="", ValidTgts="Creature.inZoneGraveyard", TgtZone="") = [battlefield], want [graveyard]
```

## Gates run on rebased tree (exact output)

```
$ go test -run 'TestAnimateDead|TestDanceOfTheDead|TestHeads|TestAnimateAttach' ./rules/
ok  	github.com/adams-shaun/gorge/rules	1.838s
rules_exit=0
$ go test -run 'TestAnimate' ./effects/
ok  	github.com/adams-shaun/gorge/effects	0.004s
effects_exit=0
$ go test -run 'TestNecromancy|TestMayFlashSac' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.058s
necro_exit=0
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.199s
arch_exit=0
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.279s
bot_exit=0
$ gofmt -l .
fmt_exit=0
$ go vet ./rules ./effects ./state
vet_exit=0
$ go run ./cmd/gentypes -check
gen_exit=0
$ git diff --quiet main -- .ds4/report-sol2.md
report_sol2_matches_main=0
```

## Deviations / concerns

Earlier implementation reuses existing head-only RemoveKeywords matching rather than changing the shared layer walker to head+first-parameter; it removes any prior Enchant and then grants Animate Dead's new one in the same pass. StrictlySelf had to be recognized for the ETB chain to run (39 measured newly-live presence/valid-card carriers). Neither change moved TestHeads or the constructed botbench golden. No new behavioural deviation in this docs-only round.

## Issues

- AnimateAll's `RemoveKeywords$` is intentionally unread; the earlier measurement counted 13 affected files out of 133 `$ AnimateAll` files; see `effects/combatfx.go` (`effAnimateAll`).
- Spellweaver Volute's graveyard cast works, but `DB$ Attach | Choices$ Instant | ChoiceZone$ Graveyard` reattachment remains battlefield-only in `effects/attach.go` (`effAttach`); `ChoiceZone$` is unread.
- Dance of the Dead's `DBAnimate` `OverwriteSpells$ True` is unread by `effects/combatfx.go` (`parseAnimateGrant`). Takklemaggot's `ConditionDefined$ ChosenCard`, `Triggers$`, and `RememberObjects$` also remain unread there.
- Necromancy's delayed-trigger order ask shows a sibling trigger's label (`rules` `triggerLabel` resolves `pt.Idx` against the printed face rather than the delayed trigger), and `Static$ True` triggers still use the stack (173 measured corpus files, `rules` trigger queue); see `.ds4/report-r2-animate-dead.md` for exact repro and CR 603.3 scope. Its remembered LKI set also emits a benign already-departed-source Note.
