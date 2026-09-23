# Animate Dead / api:Animate.RemoveKeywords — sol2

Rebased onto `main` before any edits (clean worktree; `git rebase main` succeeded). `.cards` was present (the linked real corpus). Prior MAJOR in `findings-sol2.md` fixed: `rules/stack.go` now checks `originImpliedTargetZone` before the Attach-only `ValidTgts$ inZone` inference. For Attach with an explicit single concrete Origin and an inferred inZone zone, the explicit Origin wins; the ChangeZone origin rule remains Graveyard-only. This is scoped to Attach and preserves the TgtZone/TargetType priority. `rules/animate_attach_origin_test.go` (new) pins both conflicting directions and the no-Origin fallback. Each of its three leaves fails on unmodified main. The prior MINOR is already fixed: `.ds4/report-r2.md` was not touched; this ticket's round-2 report remains `.ds4/report-r2-animate-dead.md`. The pre-existing, unrelated `.ds4/report-sol2.md` was copied to ignored `.ds4/scratch/report-sol2-attached-preserved.md` before writing the dispatch-required report path.

Earlier committed change per file: `effects/combatfx.go` reads RemoveKeywords$ and registers the layer-6 removal (the removal field and layer walk were already on main); `rules/clone.go` deep-copies the removal slice; `rules/attach.go` reads derived enchant legality and preserves zone-positive graveyard-bearing Auras through the pre-trigger SBA window; `rules/stack.go` infers Attach target zones; `effects/attach.go` admits off-battlefield Aura bearers when their current enchant spec names that zone; `effects/filter.go` enables the chain's IsPresent gate. `rules/animate_dead_test.go`, `effects/animate_removekeywords_test.go`, and the updated Necromancy test pin the chain and cleanup ask. See `.ds4/report-r1.md` and `.ds4/report-r2-animate-dead.md` for the original unmodified-main mutation proofs. No head golden or ratchet was modified.

## Fails without the fix

With the final new test file still present, copied the changed `rules/stack.go` to `.ds4/scratch/animate-sol2-stack.go`, replaced it temporarily with `git show main:rules/stack.go`, ran the leaf, then restored it byte-identically (`cmp` exit 0). Final test removed only the fourth subtest (which had passed on main), so all three failing subtests below remain:

```
$ go test -run '^TestAnimateAttachExplicitOriginOutranksInZone$' ./rules/
--- FAIL: TestAnimateAttachExplicitOriginOutranksInZone (0.00s)
    --- FAIL: TestAnimateAttachExplicitOriginOutranksInZone/graveyard_origin_beats_exile_filter (0.00s)
        animate_attach_origin_test.go:33: targetZones(Origin="Graveyard", ValidTgts="Creature.inZoneExile", TgtZone="") = [battlefield], want [graveyard]
    --- FAIL: TestAnimateAttachExplicitOriginOutranksInZone/exile_origin_beats_graveyard_filter (0.00s)
        animate_attach_origin_test.go:33: targetZones(Origin="Exile", ValidTgts="Creature.inZoneGraveyard", TgtZone="") = [battlefield], want [exile]
    --- FAIL: TestAnimateAttachExplicitOriginOutranksInZone/no_origin_infers_graveyard (0.00s)
        animate_attach_origin_test.go:33: targetZones(Origin="", ValidTgts="Creature.inZoneGraveyard", TgtZone="") = [battlefield], want [graveyard]
FAIL
FAIL github.com/adams-shaun/gorge/rules 0.003s
mutation_exit=1 restored=0
```

## Gates (exact commands and output)

Initial targeted run revealed the returned Origin zone was hardcoded Graveyard, not the parsed Exile; fixed it to return `zones[0]` before final run:

```
$ go test -run 'TestAnimateAttachExplicitOriginOutranksInZone|TestAnimateDead|TestDanceOfTheDead|TestHeads' ./rules/   # initial
--- FAIL: TestAnimateAttachExplicitOriginOutranksInZone (0.00s)
    --- FAIL: TestAnimateAttachExplicitOriginOutranksInZone/exile_origin_beats_graveyard_filter (0.00s)
        animate_attach_origin_test.go:33: targetZones(Origin="Exile", ValidTgts="Creature.inZoneGraveyard", TgtZone="") = [graveyard], want [exile]
FAIL
FAIL github.com/adams-shaun/gorge/rules 1.806s
$ go test -run 'TestAnimateAttachExplicitOriginOutranksInZone|TestAnimateDead|TestDanceOfTheDead|TestHeads' ./rules/   # final, after removing baseline-passing subtest
ok   github.com/adams-shaun/gorge/rules 1.928s
$ go test -run 'TestNecromancy' ./rules/
ok   github.com/adams-shaun/gorge/rules 0.007s
$ go test -run 'TestAnimate' ./effects/
ok   github.com/adams-shaun/gorge/effects 0.003s
$ go test ./internal/archtest/
ok   github.com/adams-shaun/gorge/internal/archtest 3.899s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok   github.com/adams-shaun/gorge/cmd/botbench 1.466s
$ gofmt -l .
(no output; format_entries=0 exit=0)
$ go vet ./rules ./effects ./state
(no output; vet_exit=0)
$ go run ./cmd/gentypes -check
(no output; gentypes_exit=0)
```

TestHeads and botbench were unchanged. Re-measured registered `(AB|DB)$ Animate` lines containing RemoveKeywords$: **10**, matching brief. Graveyard-enchant `^K:Enchant:.*inZone` files: **3**. No new approximation row added.

## Issues

- `AnimateAll` remains unregistered (`effects/combatfx.go`, Animate-only registration). 133 corpus files have `$ AnimateAll `; 15 files/lines have `$ AnimateAll ... RemoveKeywords$` by `/usr/bin/grep -rlE '\$ AnimateAll .*RemoveKeywords\$' .cards/cardsfolder | wc -l` (the brief's claim of 13 does not hold for this pattern at this pin). A separate AnimateAll registration/semantics ticket is needed.
- Prior round documented two out-of-scope trigger issues: `rules/trigger_queue.go` delayed-trigger ordering labels read a face trigger instead of the registered delayed text; `Static$ True` T: triggers still join the stack and CR 603.3 order asks (173 corpus files by `/usr/bin/grep -rlE '^T:.*Static\$ *True' .cards/cardsfolder | wc -l`). These are not changed here.
