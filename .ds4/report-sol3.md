# Restart (new seed) — hn1 Svelte gate remediation

The feature remains in `ba447b82`, `9201452b`, `b5a9a58a`, and `41884f16`: additive protocol/host fields and goldens, generated TS, rematch POST preserving deck roles and exact IDs, and the rail control with eligibility, arm/confirm, route/component/helper tests and wire fixture. This round merged current `main` (merge commit `710e243e`; no conflicts) and committed `721bb8dd`: `web/src/lib/tables.svelte.test.ts` now supplies `mulligans: 6` in its shared `TableInfo` fixture. `protocol/protocol.go` emits `mulligans` unconditionally (`json:"mulligans"`); the generated type correctly requires it, including when zero. Did not weaken the wire type. No engine or test-golden/ratchet behavior was changed. `.cards` and `web/node_modules` were present. Rebase was forbidden by the task workspace rules, so a merge brought in main instead. Prior restart implementation tests and fail-without-fix evidence are in the ticket's earlier reports; the latest verdict-sol3 was APPROVE with only a MINOR request to retain that evidence.

## Fails without the fix

Before editing the fixture, `cd web && npx svelte-check` failed (exit 1):

```
ERROR "src/lib/tables.svelte.test.ts" 19:4 "Type '{ id: string; name: string; seats: number; spectator: string; state: string; match: number; perpetual: boolean; format: string; bot_policy: string; seat_names?: string[] | undefined; mulligans?: number | undefined; }' is not assignable to type 'TableInfo'.\n  Types of property 'mulligans' are incompatible.\n    Type 'number | undefined' is not assignable to type 'number'.\n      Type 'undefined' is not assignable to type 'number'."
COMPLETED 497 FILES 1 ERRORS 1 WARNINGS 2 FILES_WITH_PROBLEMS
```

The test fixture types its return value as `TableInfo`, so the compile gate directly covers the missing field. No new test was added this round; existing feature tests and their mutation proofs were recorded in previous rounds. The `tables` test still asserts its seat/match prerequisites, and the restart test preconditions and no-fix proofs remain in the prior restart reports.

## Gates (commands and actual output)

```
$ cd web && npx svelte-check
WARNING "src/components/ResolvedCard.svelte" 97:42 "This reference only captures the initial value of `anchorProp`. Did you mean to reference it inside a derived instead?"
COMPLETED 497 FILES 0 ERRORS 1 WARNINGS 1 FILES_WITH_PROBLEMS
exit=0
$ cd web && npx vitest run src/lib/tables.svelte.test.ts src/lib/playvsbot.rematch.test.ts src/lib/playvsbot.test.ts src/components/RestartControl.svelte.test.ts src/routes/Table.restart.svelte.test.ts src/routes/Table.svelte.test.ts src/components/SeatPanel.svelte.test.ts
Test Files  7 passed (7)
     Tests  58 passed (58)
exit=0
$ go test -run 'TestGoldens|TestFramesRoundTrip' ./protocol/
ok   github.com/adams-shaun/gorge/protocol 0.002s
$ go test -run 'TestTableInfoCarriesRestartConfig|TestTheMulliganStarterFixtureIsTheEngineSOwnWireBytes' ./host/
ok   github.com/adams-shaun/gorge/host 0.110s
$ go test ./internal/archtest/
ok   github.com/adams-shaun/gorge/internal/archtest 3.751s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok   github.com/adams-shaun/gorge/cmd/botbench 0.597s
$ go run ./cmd/gentypes -check
(no output; exit 0)
$ gofmt -l protocol/protocol.go protocol/protocol_test.go host/table.go host/match.go host/restart_config_test.go
(no output; exit 0)
$ git diff --check
(no output; exit 0)
```

No head/ratchet movement measured (no engine changes); no new approximation row. The ResolvedCard warning predated this fix. No bundle rebuild, npm install, or npm ci was run.

## Issues

None found in scope. The existing Svelte `ResolvedCard.svelte:97` warning is unrelated to restart and remains open; it concerns the locally captured `anchorProp` value.

---

# PayLife<X> ETB replacement — hn1 merge resolution

Rebased clean worktree onto main (`0c3b199d`), resolved AGENTS.md by deleting only this ticket's row, appended `events.StoreSVar` after main's `Scry` to preserve ordinals, and measured the merged table rather than trusting either stale count: main has 18 rows despite constant 16; with this deletion it has 17. Corrected `internal/testutil/agentsdoc_test.go` to 17. Preserved both versions of the accumulated `.ds4/report-mrg1.md` and removed conflict markers in a follow-up commit. Approved implementation (`5f5a63eb`) otherwise unchanged. `.cards` was present as a symlink to the corpus. No measured golden or deck-ratchet movement; botbench stays green.

## Fails without the fix

Copied rebased `rules/cast.go` to `.ds4/scratch/paylife-cast-rebased.go`, replaced it temporarily with `main:rules/cast.go`, ran the three new tests, then restored and `cmp` confirmed identical bytes (exit 0). Actual output:

```
--- FAIL: TestMinionOfTheWastesEntersWithPaidLifePT (0.39s)
    etb_paylife_test.go:89: entry ask = <nil>, want a paylife ETB choice
--- FAIL: TestPhyrexianProcessorTokenUsesPaidLife (0.00s)
    etb_paylife_test.go:127: entry ask = <nil>, want a paylife ETB choice
--- FAIL: TestNamelessRacePayLifeIsCappedByXMax (0.00s)
    etb_paylife_test.go:181: precondition: options = <nil>, want exactly 0..2 (XMax = 1 permanent + 1 card)
FAIL
FAIL github.com/adams-shaun/gorge/rules 0.433s
FAIL
no_fix_exit=1
restored_cmp=0
```

## Gates (exact commands and outputs)

```
$ go test -run 'TestMinionOfTheWastesEntersWithPaidLifePT|TestPhyrexianProcessorTokenUsesPaidLife|TestNamelessRacePayLifeIsCappedByXMax' ./rules/
ok   github.com/adams-shaun/gorge/rules 0.421s
target_exit=0
$ go test ./internal/archtest/
ok   github.com/adams-shaun/gorge/internal/archtest 3.518s
arch_exit=0
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok   github.com/adams-shaun/gorge/cmd/botbench 0.662s
bot_exit=0
$ go test -run 'TestKnownApproximationRowsAreShort|TestKnownApproximationsOnlyShrinks' ./internal/testutil/
ok   github.com/adams-shaun/gorge/internal/testutil 0.001s
rows_exit=0
$ gofmt -l events/event.go internal/testutil/agentsdoc_test.go
(no output)
$ go run ./cmd/gentypes -check
(no output)
$ git diff --check
(no output)
```

Initial row gate with the incorrectly rebased 15 failed with `AGENTS.md's Known approximations table has 17 rows, above the frozen count of 15`; corrected to the measured 17 and reran successfully. No runtime behaviour was changed for this correction.

## Issues

- `effects/storesvar.go` (`effStoreSVar`) explicitly does not model `Type$ Triggered`/`Targeted` or other unrecognised types: it emits a Note and stores nothing. Of 65 corpus StoreSVar files, 3 have a same-line `Type$ Triggered`/`Targeted` shape (grep of `.cards/cardsfolder`); modelling these needs property-ref resolution and separate tests. This does not affect the three pay-life replacement cards; no new approximation row added.
- `rules/cast.go` (`etbPayLifeBound`) covers only the exact ETB `Count$xPaid` life-cost family; any other replacement life cost still needs an announcement/payment channel. Corpus prevalence not measured for that broader grammar.

---

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


---

# api:Clone closing register — hn1

STATUS: DONE_WITH_CONCERNS. Rebased twice onto moving `main` (last base `540554fc`); main's Clone row still existed. `git diff main -- AGENTS.md` removes **only** that row; `knownApproximationRows` goes from main's **12** to **11**. `.cards` was present and nonempty (`cards.lock`, `cardsfolder`, IR caches).

## Changes

- `effects/clone.go`: real `Choices$` answer continuation and narrow `GainThisAbility$` were retained from prior rounds; this round adds `AttachedTo$` battlefield resolution, chosen-name lookup in the immutable name universe, `CloneZone$` source-zone gate, face-down/up events, ETB-only `IntoPlayTapped$`, and all named `AddStaticAbilities$` SVar bodies. Unsupported source/name fails closed. Standalone entry-tap still emits its existing loud Note.
- `events/apply.go`, `events/event.go`, `view/describe.go`, `rules/trigger_eligibility.go`, `state/continuous.go`, `rules/layers.go`: appended replay-visible `CloneStatic` kind (after `TurnFaceDown`), folded named statics on the copied face, and retained their bodies on the clone marker for deterministic expiry/rebase. Event interest and view coverage updated. `ClonePermanent` also resolves a chosen card from `NameUniverse` during Apply, not by direct state mutation.
- `rules/cast.go`: admits Vesuva's supported entry tapping and any parsed named static; rejects malformed names/values. `rules/etb_clone_whitelist_census_test.go`, `rules/paramcensus_test.go`: re-pinned Vesuva and Sakashima of a Thousand Faces, retired Vesuva's unread-parameter entry, classified the SVar-name lookup (not a card Params lookup).
- `effects/clone_riders_test.go` and `rules/clone_vesuva_entry_test.go`: real corpus Cephalid Facetaker, Killer Cosplay, Permeating Mass, Vesuva, and Sakashima; fixture-only AttachedTo/FaceDown/KeepFacedown where the corpus has no DB$ Clone carriers. Each test checks distinct operands and relevant zone/status. Real Vesuva and Sakashima tests drive the as-enters election.

Measured raw `DB$ Clone` rider lines at the pinned corpus: `AttachedTo$` **0**, `CopyFromChosenName$` **1**, `CloneZone$` **2**, `FaceDown$` **0**, `KeepFacedown$` **0**, `IntoPlayTapped$` **3**, `AddStaticAbilities$` **3**. The other two IntoPlayTapped carriers also have separately unsupported ETB keys (`ChoiceZone$` on Echoing Deeps; additional riders on Callidus Assassin); only Vesuva's supported template is newly offered. No chain-head or deck-ratchet golden edited; botbench stayed green (no re-pin).

## Fails without the fix

Saved `effects/clone.go` under `.ds4/scratch/`, temporarily replaced it with its pre-rider revision, ran the new effects tests, restored byte-identically (`cmp` exit 0). Actual output:

```
--- FAIL: TestCloneNamedStaticFromCephalidFacetaker (0.76s)
    clone_riders_test.go:32: named static not granted
--- FAIL: TestCloneKillerCosplayCopiesNamedCard (0.00s)
    clone_riders_test.go:66: chosen-name copy = &{Forest ...}
--- FAIL: TestClonePermeatingMassHonorsCloneZone (0.00s)
    clone_riders_test.go:95: CloneZone Battlefield copied an object in graveyard
--- FAIL: TestCloneVesuvaIntoPlayTapped (0.00s)
    clone_riders_test.go:116: Vesuva entry: tapped=false face=Forest
FAIL
no_fix_exit=1 restored_cmp=0
```

The two additional fixture tests with the pre-rider revision also failed (restored_cmp=0):

```
--- FAIL: TestCloneAttachedToNamesBattlefieldBearer
    clone_riders_test.go:128: clone attachment = 0, want bearer 2
--- FAIL: TestCloneFaceDownAndKeepFacedown
    clone_riders_test.go:139: FaceDown$ did not turn the copy face down
```

For the rules-level tests, removed *only* each production event emission temporarily and restored byte-identically (`cmp` exit 0): `TestCloneVesuvaETBEntersAsTappedLand` failed because `Tapped:false` on its Forest copy; `TestCloneSakashimaETBGrantsNamedIgnoreLegendRule` failed because the copy had no static (`clone_vesuva_entry_test.go:73`). Full outputs: `.ds4/scratch/clone-vesuva-without2.log`, `.ds4/scratch/clone-sakashima-without.log`.

## Gates (exact commands, output)

```
$ go test -run 'TestClone|TestETBCloneWhitelist|TestTriggerEventInterestMapping|TestEveryRepoDeckParamsAreRead' ./rules/
ok   github.com/adams-shaun/gorge/rules 0.909s
$ go test -run 'TestClone' ./effects/
ok   github.com/adams-shaun/gorge/effects (cached)
$ go test -run 'TestDescribe' ./view/
ok   github.com/adams-shaun/gorge/view 0.029s
$ go test -run 'TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' ./internal/testutil/
ok   github.com/adams-shaun/gorge/internal/testutil 0.002s
$ go test ./internal/archtest/
ok   github.com/adams-shaun/gorge/internal/archtest 3.498s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok   github.com/adams-shaun/gorge/cmd/botbench 0.627s
$ go run ./cmd/gentypes -check
(no output; exit 0)
$ gofmt -l effects/clone.go effects/clone_riders_test.go events/apply.go events/event.go rules/cast.go rules/clone_vesuva_entry_test.go rules/etb_clone_whitelist_census_test.go rules/layers.go rules/paramcensus_test.go rules/trigger_eligibility.go state/continuous.go view/describe.go
(no output)
$ git diff --check main
(no output)
```

## Issues

- `rules/cast.go` (`etbCloneWhitelist`): other ETB Clone bodies remain withheld, notably Mockingbird (`cmcLEY` needs a paid-X resolver), Flesh Duplicate (`IfNew Vanishing` keyword), Echoing Deeps (`ChoiceZone$ Graveyard`) and Callidus Assassin (additional rider grammar). Separately owned ETB-copy ticket must broaden those templates safely. Not a claim that *every* ETB-copy carrier is now supported. `IntoPlayTapped$` appears on **3** raw DB$ Clone lines; only Vesuva is newly admitted. Follow-up issue already tracked by the ETB-copy ticket; no new closing-register row added.
- `effects/clone.go` (`CopyFromChosenName$`): if a match has no `NameUniverse`, the named copy now emits a Note and does nothing rather than inventing a printed template. **1** raw DB$ Clone carrier (Killer Cosplay); a legacy no-universe match would need its historical chosen card face carried on a replayable event to support this route.
- `effects/clone.go` (`FaceDown$`, `KeepFacedown$`): no raw DB$ Clone corpus carriers at this pin. This implementation uses `TurnFaceDown`/`TurnFaceUp`, not a turn-face-up *choice* for manifested/cloaked cards (different existing ticket). A `FaceDown$ True` on a copy with `Duration$ UntilFacedown` would turn it down before its marker is registered; should such a combined corpus line appear, the marker lifetime needs an entry-time turn-down check.
