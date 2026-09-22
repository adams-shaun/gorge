# AGENTS.md — gorge

Pure-Go Magic rules engine.

## What this is

The rules engine that will replace `mtgplay`'s XMage bridge. Card behaviour is
compiled from Forge card scripts into an IR; the engine implements the
primitives that IR references. See
`docs/superpowers/specs/2026-09-03-mtgcore-go-engine-design.md`.

## Hard rules

- **Never commit Forge card scripts.** They are GPL-3.0; gorge is Apache-2.0.
  `forgec fetch` pulls the card corpus and token scripts into `.cards/`,
  pinned to a commit SHA (`FORGE_REF` in the Makefile), which is gitignored.
  `cards/boundary_test.go` fails the build if any are tracked.
- **No cgo, no third-party deps** in the card pipeline and rules core.
  `wazero` arrives with the plugin tier in M3.
- **All state mutation goes through `events.Apply`.** If you are writing to a
  `state.Game` field outside `events`, you are introducing a replay bug.
  `events.Kind` is append-only, and every kind M2r and M2d added was appended
  after all earlier kinds so ordinals, the hash chain and golden replays stay
  untouched: M2r appended `CastInfo`, `Choose`, `TokenCreate`, `StackCopy`,
  `Attach` and `AbilityPush`; M2d appended `ModeChosen` for mid-resolution
  modal answers.
- **No nondeterminism.** No wall clock, no ambient randomness, no `map` range
  where iteration order can reach an event.
- **This engine never imports anything from the mtgbld/mtgserve application.**

## Build / run / test

```sh
make fetch-cards          # one-time; ~25 MB, pinned commit from Card-Forge/forge
make compile-cards        # parse into the IR cache
make report               # card coverage against implemented primitives
make sim                  # build mtgsim and play 20 verified 4-seat games
make test lint
make conformance          # the focused CR conformance audit
```

`make conformance` is the focused CR 601/733 audit. I-2 (mandatory-target
feasibility), I-7 (targets before payment), and the CR 733.1 illegal-cast
reversal are fixed and asserted in the ordinary suite; the historical
`requireCR601Audit` guard and `GORGE_CR_CONFORMANCE=1` switch were removed. The
Makefile target remains explicit so reviewers can run the focused audit without
running the full suite.

## Reproduce a feedback report

A player-submitted bug report that named a table carries a replayable
snapshot (`match.json`, `log.json`, the seat's `view.json`, captured by the
server's feedback capture). Turning it into a failing test is one command:

```sh
go run ./cmd/repro <feedback-dir>              # replay, verify the head, print the board
                                             # at the report point (turn, step, life,
                                             # battlefield, stack, last ~20 log lines)
                                             # (-at 0 also works flags-after-dir:
                                             #  `repro <dir> -at 0` is parsed same as
                                             #  `repro -at 0 <dir>`)
go run ./cmd/repro -at N <feedback-dir>        # replay to intent N instead
                                             # (-list prints the intent timeline to find N)
go run ./cmd/repro -omniscient <feedback-dir>  # show every hand in the summary
go run ./cmd/repro -emit-test <pkg> <feedback-dir>
                                             # copy the snapshot into
                                             # <pkg>/testdata/feedback/<id>/ and write a
                                             # failing test skeleton (feedback.EngineAt) as
                                             # the target package's EXTERNAL test package
                                             # (package <name>_test: feedback imports the
                                             # engine tier, so an internal-package skeleton
                                             # would be an import cycle for an engine-side
                                             # target); replace the TODO with the assertion

A committed fixture's `match.json` does not embed token script text — Forge
scripts are GPL-3.0 and must never be committed — so `-emit-test` and the
fixture generator strip `match.json`'s `tokens`/`tokens_unread` and sync the
exact text into the gitignored `cmd/repro/testdata/.tokens/<id>/` keyed by
the fixture id. `feedback.Load` resolves token scripts from that sync
directory first (exact historical text) and falls back to the live corpus
(`.cards/` at the current `FORGE_REF`) when it is missing (a fresh clone), so
a fixture still replays without ever re-embedding the text into committed
files.

A capture also records the name-card universe MODE (`name_universe`) and the
exact sorted label list that match offered (`name_universe_names`), because
a universe-backed match poses `NameCard` asks the legacy path never poses and
a replay rebuilt without the mode refuses the recorded name intent. The
committed fixture keeps only the mode bit -- the list is ~24k entries -- and
`feedback.config` re-derives it from the live corpus, so a `FORGE_REF` move
can renumber a recorded name choice and shows up as the same `DIVERGED`.
```

Exit 0 is a verified replay: the rebuilt event stream matches the recording
event for event and the final head equals log.json's `head`. A non-zero exit
prints `DIVERGED` naming the first mismatching event — **a corpus change since
the report was filed is a real, expected cause** (the snapshot replays against
the corpus as it was at the report's pin), and so is an engine change; read the
first diverging event before assuming either. In a test, load the same
snapshot with `feedback.EngineAt(t, dir, n)` (`internal/testutil/feedback`;
`n < 0` = every recorded intent) and assert at the engine it returns.

The committed fixture `cmd/repro/testdata/feedback/20260914T120000Z-fb01/` is
a real capture produced by `host.SnapshotForFeedback`; regenerate it with
`REPRO_REGEN_FIXTURE=1 go test ./cmd/repro -run TestGenerateCommittedFixture`
after a corpus pin bump (`FORGE_REF`), or the fixture's own gate reports
DIVERGED exactly as a stale report would.

## Status

**M2r closed the coverage ratchet.** `rules/acceptance_test.go`'s
`knownUnsupported` (Ruling P12/D2-a) names exactly the cards the measured
deck set does not fully support, and
`TestEveryRepoDeckIsFullySupported` asserts the measured gap set equals the
table in both directions (Ruling R-20): a card the build newly cannot fully
support fails and is named together with the primitives it is missing, and a
table entry the build now supports is stale and fails too. Measured
2026-09-17 at the avengers-assemble Commander import, the ratchet stands at
**3 of 791** -- every one of the 791 distinct cards across the repo deck
files (`internal/testutil/decks/*.json`: 14 60-card constructed decks plus
10 Commander decks) is fully supported and plays except the three the table
holds (Avengers Quinjet's crew, Captain Marvel's mirror-counter trigger,
Speed's pay-on-trigger -- the Marvel Super Heroes Commander import's own
remaining backlog). The 12 pinned Legacy decks (`legacyDeckNames`)
round-robin across 2/4/6/8 seats
(`TestRepoDecksPlayAtEverySeatCount`), replay byte-identically
(`TestRepoDeckGamesReplayExactly`), and `TestHeads` pins the chain heads as
goldens in `rules/heads_test.go`:

| seats | 2 | 4 | 6 | 8 |
|---|---|---|---|---|
| chain head | `bc7420d9e4c7d3d2` | `e7cffb892a152493` | `76f187361778b000` | `7c9dbf608ada58b3` |

`TestEveryRepoDeckParamsAreRead` (`rules/paramcensus_test.go`) is the
companion ratchet over the same decks' parameters: measured at the same
date, its `knownUnsupportedParams` table holds 25 repo-deck cards carrying
an unread param or unmodelled cost token (19 of them from the
avengers-assemble import; the desc-only `ValidTgtsDesc` family the new
deck's cards carry several of).

`make sim` plays 20 verified 4-seat games from the same seed set, every one
replaying byte-identically (20/20 `replay OK`).

Measured at the corpus pin `master @
95f04e8a04c8925fa97cb226fc3341cabcc90a53` (`FORGE_REF` in the Makefile):
`make report` prints `cards: 33667  playable: 24727 (73.4%)` with `tokens:
839` (re-measured 2026-09-17; the jump from the 2026-09-14 figure of
21108/62.7% is the parameter-read registrations merged since -- the
param-census task wave -- and the pw1 figure 20635/61.3% predated api:Untap,
api:ManaReflected, stat:ManaConvert, stat:UntapOtherPlayer and
kw:Cumulative upkeep registering. `repl:Untap` is out of scope and remains
unsupported. The registered primitive set is measured by `make report`.
M1's 37/8/8/8/1 = 62 was the count before M2r registered the keyword,
trigger, static and mid-game primitives the ratchet's card work needed.)

A seat's clients must be able to answer every `decision.Kind`, and the set is
closed: the M1 kinds (`priority`, `target`, `attackers`, `blockers`,
`trigger_order`, `trigger_optional`), M2r's `choose` (an {X} value, delve
exiles, cost sacrifices, "as it enters" name/type/number, miracle-style
yes/no), the two M2d closures -- `mulligan`, the London keep/mulligan
and bottoming round `Config.Mulligans` runs between the deal and turn 1, and
`modes`, the modal pick and unless-pay ask a mid-resolution answer serves
(the `ModeChosen` event carries the answer into the log) -- and the three
kinds the later card work added: `commander_zone` (a commander's OWNER's
CR 903.9 command-zone replacement choice), `replacement` (the CR 616.1
order choice over competing replacement effects) and `arrange` (the ordered
subset ask `Scry`/`Surveil`/`RearrangeTopOfLibrary` share, Ruling J0).
Concede (M2d-3) is not a kind: it is a `concede` option on every priority
decision that emits the existing `PlayerLost` with Text "conceded". The
engine-side defaults that still stand in for a choice the engine cannot yet
ask are listed under **Known approximations** below.

Acceptance commands:

```sh
go test ./rules/ -run 'TestEveryRepoDeck|TestRepoDecks|TestRepoDeckGames' -v
make sim
```

## Known approximations

Each row is one place a registered primitive defaults instead of asking, or
behaves more narrowly than the card text, with the stand-in's location and
the milestone that removes it.

**This table is a CLOSING REGISTER, frozen at this commit. No row may ever be
added, and no row may be grown.** It started at 13 rows as an honest audit of
where "supported" was not the whole truth; it then became a place every
landed ticket wrote a paragraph, reached 264 KB on 2026-09-22 across ~100
rows, and was loaded into every agent's context on every turn. Every
remaining row now has a P1 ticket whose only job is to delete it.

So:

- **A landing ticket DELETES its row** and lowers the
  `knownApproximationRows` constant in `internal/testutil/agentsdoc_test.go`
  by the number of rows it deleted. `TestKnownApproximationsOnlyShrinks`
  fails the build on a row added, a row grown past 600 bytes, or a count
  above the constant.
- **A deviation a ticket cannot close goes in the COMMIT MESSAGE and the
  ticket report — never here.** If it needs tracking, the implementer says so
  plainly in the report and the operator files a ticket for it.
- **A reviewer treats any added or grown row as a MAJOR finding.**

Behaviour that is a deliberate contract rather than debt — the R-9 no-ask
host degradation contract, regeneration, the exact round number — is in
[`docs/superpowers/specs/2026-09-22-engine-contracts.md`](docs/superpowers/specs/2026-09-22-engine-contracts.md),
not here.

| Stand-in | Where | Removed by |
|---|---|---|
| PLAYER shroud (`Affected$ You` + `AddKeyword$ Shroud`, True Believer, Ivory Mask) is unread: players have no ObjID and no keyword machinery, so a player with granted shroud stays targetable by everything. Permanent shroud is real; this is only the player half. | `rules/stack.go` (`candidatesFor`, `legalTargets`) | M4 (player keyword machinery) |
| `AB$ ManaReflected` reached standalone, outside the mana-activation path, falls back to the deterministic first candidate (the R-9 no-ask contract). The ManaProduction collector and colour identity deliberately do NOT see reflected mana (membership added only at `availableManaAbilities`), so a bot never taps a reflected source for its coloured pips. | `effects/mana_reflected.go` (`effManaReflected`, `ManaReflectedCandidates`), `rules/mana_activation.go` (`availableManaAbilities`, `resolveManaEffect`) | M4 (tap-to-pay for reflected sources; a bot-quality collector) |
| `stat:ManaConvert` is battlefield S: statics only: the Effect-granted family (`SVar:...Mode$ ManaConvert`) and EffectZone$ Command statics never reach the payment path; `Optional$ True` (Jodah) is unread; an unevaluable `ValidCard$` fails closed and never grants. Not applied to the mid-resolution unless-pay arm. Pool-only: no tap-to-pay. | `rules/mana_convert.go`, `rules/mana.go` (`resolveMana`), `rules/stack.go` (`payManaConv`, `costPayable`) | M4 (Effect-granted statics, unless-pay conversion, tap-to-pay) |
| `api:FlipCoin` unread, each loud when present: `ForEachPlayer$` (one flip instead of one per opponent) and the per-flip memory `RememberResult$`/`RememberNumber$`/`RememberLoser$`. `Defined$ FlippedHeads`/`FlippedTails` resolve to the EMPTY set (deliberately fail closed). `FlipUntilYouLose$` abandons the loop if a win branch suspends. | `effects/flipcoin.go` (`effFlipCoin`, `rememberFlags`), `effects/context.go` (`definedSpec`) | M4 (flip-memory parameters, `ForEachPlayer$`, until-you-lose resume) |
| "As this enters, choose ..." is asked at cast/play time, so the choice is recorded and visible a resolution early; the mid-resolution machinery exists (M2d-2) but these asks have not migrated. | `rules/cast.go` (`etbAsk`, `etbAnswer`) | M4 |
| `NameCard` asks over the embedder-supplied compiled corpus (`state.Game.NameUniverse`), filtered by the SA's own `ValidCards$`; the sidecar saves its sorted name snapshot so replay is corpus-update-stable. A no-universe replay retains the exact pre-feature paths (mid-resolution top-library name; ETB visible-object filter with empty `ValidCards$` defaulting to nonland). `ChooseFromList$`/`AtRandom$` are unread, so those forms offer the whole universe; `ValidDescription$` is prompt text, with a narrow description-only fallback. | `effects/namecard.go`, `effects/cardflow.go` (`effNameCard`), `rules/cast.go` (`legacyETBNameOptions`), `host/persist.go` | M4 (ChooseFromList$/AtRandom$ forms) |
| `effMana`'s no-host fallback resolves `Produced$ Any`/`Combo Any` to colourless, and raw `Combo <colours>` to its full amount in every listed colour rather than asking. Direct raw `Chosen`/`ComboChosen` fails closed; activation asks for its colour choices, while the bot-side production collector counts `Any` as one colourless unit for its tap gate. The remaining Special selectors stay loud-fail: `EnchantedManaCost`, `DoubleManaInPool`, `EachColoredManaSymbol_Milled`, `EachColorAmong_ExiledWith`. | `effects/misc.go` (`effMana`), `rules/mana_activation.go`, `cards/mana_production.go` (`ProducedCounts`) | M4 (mana-choice decisions) |
| `RestrictValid$` is carried into the mana pool; only dotted `Spell.<filter>`/`Activated.<filter>` alternatives supported by the matcher are enforced. Unsupported alternatives fail closed, including bare `Spell`/`Activated` (13 raw lines), `CostContainsX`, `CumulativeUpkeep`, `CantCast*`, `Static.*` and `nonSpell`; 41 raw `RestrictValid$` lines across 39 corpus files contain at least one such term, so mixed alternatives can lose only the unsupported branch. | `effects/misc.go` (`effMana`), `rules/stack.go` (`restrictValidMatches`) | M4 (the remaining restriction grammar) |
| CopySpellAbility's `MayChooseTarget$ True` is a real one-shot election: the flag rides the StackCopy event from the CREATING copy SA (so an external copier grants it too), the ask takes the copied spell's own resolved target bounds and per-controller `Option.Group` through the shared cast readers, every inherited target is offered first (even when no longer legal -- declining lets the copy fizzle per CR 608.2b), and the TargetsChosen fold consumes it. Remaining approximation: the ask is still ONE flat option list over the copy's targets, so a copied spell whose distinct target DECLARATIONS each carry their own legal set loses the per-declaration attribution, and the `MaxTotalTargetPower$` budget and affordability prune the cast ask applies are not re-run here. | `rules/stack.go` (`AskCopyTargets`, `resolveTop`), `effects/copy.go`, `events/apply.go` | M4 (per-declaration copy target groups; the copy reprice/budget prunes) |
| `Discard` modes `Random`, `Defined`, `LookYouChoose`, `YouChoose` and `RevealTgtChoose` still take the front card deterministically. `Mode$ Hand` with `Optional$ True` discards the whole hand plus a Note (a real may-discard election is M4). Multi-target `TgtChoose` abandons targets 2..n. Unread: the Condition*, UnlessCost, UnlessPayer, AILogic, Mandatory and TargetMin/Max/Unique families. | `effects/cardflow.go` (`effDiscard`) | M4 |
| `RevealAllValid$` is unread: the reveal-all-matching family (`Break Expectations`, `Mind Spike`, `Thought Rattle`) still shows the FIRST card of the hand rather than every matching card, so the caster's chained pick chooses among one card. The plain hand-reveal pick itself is real (task infernaltutor1): a hand `Reveal`/`RevealHand` whose eligible pool exceeds its count (or carries `AnyNumber$`) asks the pool's owner which cards, an `Optional$` reveal asks its yes/no first, a DECLINED yes/no reveals nothing and poses no pick (fx45), and a walk over several `Defined$` players attributes EACH answer to its own target (the per-target cursors `RevealPickTarget` and `RevealOptTarget`), so an answered pick suppresses the Optional$ gate only for its own player and every later player is asked independently. | `effects/cardflow.go` (`effReveal`) | M4 (`RevealAllValid$`) |
| `effCharm`'s inner `Resolve` has no `Suspended()` guard and does not re-check `Suspended()` between modes, so a future asking sub there would ask N times. | `effects/misc.go` (`effCharm`), `effects/registry.go` (`Resolve`) | M4 (add the `Suspended()` guards) |
| `TargetUnique$ True` is read only at the shared mid-resolution target pre-ask: ask kinds outside that tail -- a Charm mode election, a ward pay window, a dig/scry/arrange ask -- parked between two riders lose the accumulator and the resumed ask over-offers (the conservative direction). Placeask/announcement asks enforce distinctness only via Decision.Validate's duplicate rejection. | `effects/targets_ask.go` (`poseTargetsAsk`), `effects/target_unique.go`, `effects/registry.go` (`Ctx.TargetsUnique`), `rules/resolution.go` (`resumePoint.targetsUnique`) | M4 (the remaining ask kinds riding the accumulator) |
| `Sacrifice` still does not read `ValidCard$` (Expert-Level Safe's `Defined$ You` plus `ValidCard$ Card.Self`, whose controller hands over the first zone-order permanent rather than "this artifact"). The no-host stand-in (R-9) keeps the deterministic first-`Amount$`-eligible pick under a secret Note. An unmodelled condition count body fails OPEN by convention. | `effects/zone.go` (`effSacrifice`, `sacrificeAmount`), `rules/resolution.go` (`"sacrifice"` resume arms) | M4 (`ValidCard$`; the general condition grammar) |
| A sacrifice cost's `CARDNAME` is the source object id and source-less filters fail closed, but this does not make every CARDNAME-bearing cost playable: Exile/Discard/Return and other cost verbs remain unsupported. Mana-ability sacrifice costs offer a real choice when multiple candidates exist. | `effects/filter.go` (`MatchesObjectCtx`), `rules/cast.go` (`castable`, `sacAsk`), `rules/mana_activation.go` | M4 (remaining cost grammar) |
| UnlessCost$ mana payments open a one-source-at-a-time CR 601.2g mana-ability window after the payer chooses pay (the ward/attack-prop discipline); each source's single tap offers its production ALTERNATIVES as separate window options (a dual land's {U} or {R}), and the pay option is offered only when the pool plus one alternative per source can reach the exact amount and colours AND every supported component (Sac/Discard/Reveal, SubCounter, RevealChosen, Draw) is satisfiable -- InstantSpeed$ sources stay withheld and a `RestrictValid$`-governed or Indeterminate-Amount$ ability is not counted. A resolvable SVar body on a Counter or CopySpellAbility unless-cost is folded to a concrete generic amount (Mausoleum Wanderer's `Sacrificed$CardPower`); every other API's SVar, and an unfolded `X`/`XX`, `PayEnergy<N>`, `Return<...>`, `LifeTotalHalfUp`, `DefinedCost_*` or `Y`/`Z`, stays a HARD DECLINE the ask never offers, and exotic selector bindings remain unresolved. | `effects/unless.go` (`poseUnlessAsk`, `UnlessCostResolved`), `rules/unless_payment.go` (`UnlessCostPayable`, `UnlessCostPayableFromCtx`, `askUnlessMana`), `rules/mana_available.go` (`windowManaUnits`), `rules/resolution.go` (`unless_pay` arm, `ParseUnlessCost`) | M4 (X-cost grammar, remaining selector bindings, the other APIs' SVar folds) |
| The other `TargetType$` token qualifiers are unread, so those restrictions widen to the whole kind set: `singleTarget`/`numTargets` target-count arithmetic, `YouDontCtrl`, `Spell.nonCreature`, `Spell.Colorless`, `Spell.Legendary`. The kind split degrades to "activated" for an ability object whose source or face is gone or flipped, so a `Triggered`-only counter misses those. | `rules/stack.go` (`legalTargetCandidates`, `stackObjKind`, `stackTargetKindTokens`), `effects/misc.go` (`effCounter`) | M4 (the remaining TargetType$ qualifiers) |
| `ValidTgts$ Spell` with no `TargetType$` and no `TgtZone$` still searches the battlefield: `targetZones` adds the stack only when `TargetType$` names a stack object. A latent footgun, not a live deviation -- recorded so a future script omitting `TargetType$` is not silently inert. | `rules/stack.go` (`targetZones`) | M4 |
| A spell on the stack is offered with `Option.Kind` `"permanent"`, inherited from the battlefield branch it was cloned from. Cosmetic today (nothing in botpolicy, view or seat reads it) but wrong on the wire; correcting it moves the option lists and all four chain heads, so it is deferred to a change already regenerating them. | `rules/stack.go` (`legalTargetCandidates`) | M4 |
| `Effect` registers real continuous effects only for `CantTarget`, `CantRegenerate`, the cost-modifier modes, and `Triggers$ BecomeMonarch` (Palace Jailer). Every other `StaticAbilities$`/`Triggers$` mode stays a bare Note. An absent `Duration$` now correctly expires at cleanup for every source kind, and `UntilYourNextTurn` ends as that turn begins rather than a full turn too long; explicit exotic `Duration$` forms remain incomplete. | `effects/misc.go` (`effEffect`, `effectUntilEOT`, `IsNextTurnDuration`, `IsUntilYourNextTurn`), `rules/layers.go` (`restrictionApplies`, `AddContinuous`, `EndOfTurnCleanup`), `state/continuous.go` | M4 (remaining Triggers$ modes, full Duration$ grammar, the other Effect-delivered grant keywords) |
| `DelayedTrigger` implements `Mode$ Phase` (including its `ValidPlayer$` filter), event-matched `SpellCast` and `ChangesZone`, and `BecomeMonarch`; the latter recognizes only exact `ValidPlayer$ Player.OpponentOf Remembered` (Palace Jailer). `ChangesController`, `DamageDone`, and `AttackersDeclared` still record a Note; event-mode `ValidPlayer$` except that narrow monarch shape and `RememberObjects$ Targeted` (vs `RememberedLKI`) capture are unmodelled. A registration whose source, SVar or controller is gone is skipped but never collected, so it is re-scanned forever. | `effects/misc.go` (`effDelayedTrigger`), `events/apply.go` (`DelayedRegister`, `DelayedPush`), `rules/trigger_match.go` (`checkDelayedTriggers`) | the remaining modes plus `ValidPlayer$`/`Targeted` capture |
| The CR 601.2c cast-offer census is deliberately NARROW: `castTargetsAvailable` withholds a cast only for the unconditional one-target shape (ValidTgts$ with no TargetMin/TargetMax, Choices$ or Announce$, not Charm). Every other shape is offered and relies on the post-push `askTarget` backstop, so a `TargetMin$ 1` spell with no legal target stays castable. Offering-then-fizzling is deliberate. | `rules/legal.go` (`castTargetsAvailable`), `rules/stack.go` (`legalTargetCandidates`) | M4 (target counts evaluated with announced choices in hand) |
| (alltargeted1) `AllTargeted$` binds `Ctx.Targets` -- the resolving SA's own chosen targets -- rather than Forge's union over the whole root/sub-ability chain. The engine defers a sub-ability's targeting to resolution, so a cost reduction that must count a SUB-ability target reads 0 at every point the head is read. | `effects/count.go` (`refTargets`), `rules/legal.go` (`ownReduceCost`), `rules/cast.go` (`repriceForTargets`, `pendingCast.ownReduce`) | M4 (sub-ability target pre-asking at activation; `CollectEvidence` cost parsing) |
| `Count$Compare`: the 2 exotic shapes with no parseable threshold fail closed, and an unmodelled inner head in a branch degrades to 0 per the evaluator's convention. | `effects/count.go` (`evalCompare`, `evalCountOperand`, `maxCountDepth`) | M4 at the earliest |
| Fixed `Choices$` and `VoteCard$` ballots now pose a private per-voter `KChoose`, reveal votes only after the whole ballot, and resume through the decision-scoped vote continuation. A no-host fallback still takes the first option (R-9); `VoteMessage$`, `Secretly$` and `UpTo$` remain unread. | `effects/misc.go` (`effVote`, `effCardVote`), `effects/vote.go`, `rules/resolution.go` (the `vote` resume arm) | M4 (the remaining unread vote parameters) |
| The three planechase verbs are registered no-ops with a Note: `api:Planeswalk` and `api:ChaosEnsues` record "no planar deck" because this build has no planar deck or planar zone (`Valid$ Plane` scans nothing). `Planeswalk`'s `Optional$ True` election is real -- a yes/no ask, the deterministic decline on a no-host (R-9), and the `SubAbility$` chain completes either way, with only a `yes` recording the no-op Note. `DontPlaneswalkAway$`, `ChaosEnsues$`'s `Defined$`/`Remembered$` riders and `T:Mode$ ChaosEnsues` stay unread or unregistered and silently inert. | `effects/planechase.go` (`effBlankLine`, `effPlaneswalk`, `effChaosEnsues`) | M4 (a planar-deck tier) |
| Forge's `Mandatory$` on a hidden-library search is deliberately unread: the CR 701.23b/701.23d fail-to-find split is driven only by whether the ChangeType filter states a quality, so a real card-quality grammar is still owed. | `effects/zone.go` (`effSearchLibrary`), `effects/filter.go` (`SearchStatesQuality`) | M4 (a real card-quality grammar) |
| (searchmay1) `ShuffleNonMandatory$` poses no may-shuffle confirm on the fail-to-find shape (nothing moved keeps the mandatory shuffle silently, where Forge would ask), and the lines where the flag rides an object-target `Origin$ Graveyard` `ChangeZone` are inert because that path shuffles nothing at all. | `effects/zone.go` (`searchShuffleTail`, `shuffleLibrary`), `rules/resolution.go` (`search_mayshuffle` arm) | M4 (a confirm on the fail-to-find shape; the object-path shuffle) |
| No LIMITED-look grammar: the chooser still sees and picks over the FULL eligible option list. Search decisions stay private to the chooser — `view.project` attaches a decision only to `Decision.Player`, so even `Omniscient` spectators receive none. | `effects/zone.go` (`effSearchLibrary`, `effHiddenPick`, `applyLibrarySearch`), `view/view.go` (`project`) | M4 (limited-look grammar; spectator search decisions) |
| Hidden-library `ChangeZone` ignores `GainControl$`, `ExileFaceDown$` and `Imprint$`: a fetched permanent keeps its existing controller, an exiled card stays face up, and an imprint reference is not retained. | `effects/zone.go` (`applyLibrarySearch`) | M4 (control, face-down exile and imprint state) |
| A hidden-library `ChangeZone` whose `DefinedPlayer$` resolves to several libraries asks only the FIRST in deterministic resolution order; there is no persisted per-player cursor, so re-asking after a suspension would re-ask player one rather than continue to player two. | `effects/zone.go` (`effSearchLibrary`, `searchPlayers`) | M4 (persisted per-player search continuations) |
| `ChangeType$ "EACH <A> & <B>"`: on the NON-library object paths the union matcher makes candidates reachable but the pick follows the ordinary count, not one-per-type; a per-type `ChangeNum$` above 1 keeps the flat-count path behind a loud Note; a quantity-only EACH spec keeps the mandatory-find reading; overlapping sub-specs offer a card in both Groups, and picking it in one blocks the other. | `effects/filter.go` (`eachAlternatives`), `effects/zone.go` (`effSearchLibrary`) | M4 (per-type pick structure on object paths; per-type ChangeNum counts) |
| Hidden-origin searches support `Origin$ Sideboard` — exact (`Burning Wish`) and compound no-selector (`Origin$ Sideboard,Exile`, Karn's -2) — plus library and `OriginAlternative$` unions; a compound origin WITH an object/fetch selector (Eladamri, Korvecdal's `Library,Hand`) and the non-search mixed-Hand shapes keep the object path (the rows below). | `effects/zone.go` (`effChangeZone`, `effSearchLibrary`) | M4 (remaining mixed-origin chooser grammar) |
| An explicit multi-zone `ChangeZone Origin$` including `Hand` has no origin-aware chooser: the exact-hand and exact-library walkers cannot build one private option list across hand, graveyard, library and battlefield, so a source-default picker emits one replay-visible Note and moves nothing loudly. | `effects/zone.go` (`mixedOriginIncludesHand`, `effChangeZone`) | M4 (origin-aware mixed hidden-zone chooser) |
| `RearrangeTopOfLibrary`: the R-9 no-host stand-in keeps the existing order (pile A = the offered options in offered order), narrower than "in any order", and a multi-player `Defined$`/`ValidTgts$` rearrange asks only the first library in scan order. | `effects/cardflow.go` (`effRearrangeTopOfLibrary`), `rules/arrange.go` (`handleArrange`) | M4 (per-library asks) |
| `Scry` sends its unchosen pile B to the BOTTOM in the OFFERED order, not the player's (KArrange orders only pile A; `handleArrange` scans `d.Options` by index) — observably wrong, since those cards are later drawn. A multi-library `Defined$`/`ValidTgts$` Scry asks only the FIRST library: `effLookAndArrange` returns out of the whole Defined loop on the first ask. | `rules/arrange.go` (`handleArrange`), `effects/cardflow.go` (`effLookAndArrange`) | M4 (a player-chosen pile-B order, shared with Surveil) |
| `Surveil` puts its unchosen pile B into the GRAVEYARD in the OFFERED order, with no player choice over that pile's order (CR 701.25; a Surveil has no bottom pile). | `rules/arrange.go` (`handleArrange`), `effects/cardflow.go` (`effLookAndArrange`) | M4 (a player-chosen graveyard order) |
| `Optional$ True` on `Scry` is not read — the scry always happens; `Min 0` lets the player keep nothing on top, which is not the same as declining to look at all. No `Surveil` line carries `Optional$`. | `effects/cardflow.go` (`effLookAndArrange`) | M4 (optional scry) |
| `T:Mode$ Scry` and `R:Event$ Scry` do not fire: registering `api:Scry`/`api:Surveil` does NOT register the "whenever you scry" trigger or the scry replacement, and nothing in this build may claim it does. | `effects/cardflow.go` (`effScry`, `effSurveil`) — no `trig:Scry`/`repl:Scry` registered in `rules/` | M4 (scry triggers and replacements) |
| (dig1) An `Optional$ True` window with exactly `ChangeNum$` eligible cards still plays "you may take" as a mandatory take; the remainder is not asked (untaken cards stay ON TOP in existing order, `RestRandomOrder$` unread); the primary `LibraryPosition$`, `Choser$` and `DigNum$ X` are unread; a multi-player `Defined$` Dig asks only the first library and the rest keep the first-eligible stand-in. | `effects/cardflow.go` (`effDig`), `effects/zone.go` (`effHiddenPick`, `effSearchLibrary`), `effects/play.go` (`effPlay`), `rules/resolution.go` (the "dig" arm), `botpolicy/policy.go` | M4 (per-library asks; remainder ordering; `Optional$` on the no-choice path; the primary `LibraryPosition$`; `Choser$`; `DigNum$ X`) |
| (diguntil1) `RevealRandomOrder$` is unread (randomness is forbidden here): the revealed pile returns in existing library order. Withheld loudly, one Note each, core move still running: non-literal `Amount$`, `DigZone$`, `NoMoveFound$`, `FoundLibraryPosition$`, `Shuffle$`, `Imprint*$`, `NoneFound*$`. A found Aura's CR 303.4f attach takes the first eligible bearer, not a real choice. | `effects/cardflow.go` (`effDigUntil`, `digUntilParamValue`, `auraEntryBearer`) | M4 (the `RevealRandomOrder$` pile order; the withheld params; a real bearer choice for the Aura attach) |
| (cascade1) The exiled bottom pile returns in existing library order, not random. `Triggers$ ExileEffect` is unread, so an Effect-delivered cascade grant without `ForgetOnCast$` lasts while its source stays and EVERY qualifying spell that turn cascades. The free-cast election has no casting-legal-timing check, and the mana-value comparison reads the printed face (an {X} spell compares X=0). | `rules/cascade.go` (`queueCascadeTriggers`, `cascadeInstances`), `effects/cascade.go` (`effCascade`, `effCascadeBottom`), `effects/misc.go` (effEffect's AddKeyword arm) | M4 (random order is M4-or-never; `Triggers$ ExileEffect`; a timing check on the free cast) |
| (ft1) Bot target selection is effect-blind except for a known literal damage amount: `effectDamage` trusts the supplied amount without inspecting the API string (a consumer-contract discrepancy, unreachable in play). Destroy, counter and non-damage effects are unranked, and "spare mana" is read only from the floating pool, never from untapped sources. | `botpolicy/target.go` (`effectDamage`, `mayKillMe`, `hasSpareMana`, `effectRanker`, `chooseTargets`) | richer outcome modelling / Board mana-source facts |
| (fx20) `MatchesPlayerSpec` still fails closed on `EnchantedBy`, `EnchantedController`, `Chosen`, `descended`, `IsRemembered`, `TriggeredDefendingPlayer`, `counters_` and compound/space forms. `Active`, its `NonActive` complement, `life<OP><N>` and `isMonarch` evaluate. | `effects/filter.go` (`MatchesPlayerSpec`), `rules/trigger_match.go` (`spellCastMatches`, `damageMatches`, `phaseMatches`), `rules/statics.go` (`actorMatches`), `rules/layers.go` (`restrictionActorMatches`) | M4 (the remaining player-spec grammar and player-state machinery) |
| `ValidLKI$` on a replacement fails closed for the may-play provenance it gates: `Spell.MayPlaySource`, `Warp`, `Mayhem` and `ManaFromArtifact` have no per-cast provenance, so a conditional graveyard-cast replacement (Glimpse the Cosmos) is never admitted -- conservative, but the positive route is unproven. Same predicates fail closed at the layer `Affected$` match and `Count$ThisTurnCast_`. | `rules/replacement.go` (`replacementMatches`), `rules/layers.go` (`matchesWithTypes`, `Affected$` statics), `rules/stack.go` (`spellsCastThisTurnMatching`, `Count$ThisTurnCast_`), `rules/cast_provenance.go` (`castSaAdmits`) | the may-play cast-provenance predicates (MayPlaySource/Warp/Mayhem/ManaFromArtifact) |
| A competition of all-`Updated` replacements applies them in deterministic scan order with no CR 616.1 order choice -- correct where they commute, wrong where they do not. A non-commuting life competition met while another decision is outstanding, or affecting a player who has left the game, also applies in scan order, since a second ask would overwrite the first. | `rules/replacement.go` (`composeUpdatedReplacements`, `poseLifeReplacementChoice`) | M4 (order choice for every competing shape) |
| A bot or a host that cannot answer takes option 0 of the `KReplacement` order decision through `botpolicy`'s clamp fallback; there is no policy arm of its own. Deterministic and non-wedging. | `botpolicy/policy.go` (`Decide`'s clamp fallback) | M4 (a real replacement-order policy) |
| The CR 704.5j legend rule keeps the first duplicate in battlefield scan order and bins the rest, with no controller choice: an SBA is not a decision a seat answers, so the survivor is picked by scan position rather than by its controller. Correctly scoped per controller. | `rules/sba.go` (`legendCasualties`) | M4 (a controller choice for SBA-driven sacrifices, shared with the 704.5m/704.5q families) |
| Monocolour hybrid (Forge's `2B`: two generic or one black, CR 107.4e) degrades to a single generic pip -- under-priced by one generic, and payable by a colour it should also accept. Deliberately not widened: it needs a third `Cost` shape (a generic ALTERNATIVE, not a colour pair) alongside `Hybrid []ManaPair`. Two-colour hybrid and Phyrexian are real. | `rules/mana.go` (`ParseCost`'s two-character hybrid branch) | M4 (a generic-or-colour hybrid alternative alongside `Hybrid []ManaPair`) |
| When `CharmNum$` selects several DISTINCT independently targeted modes, the stack object still has one undivided target list: casting uses the first selected target-bearing mode's declaration and every selected mode receives that shared list at resolution. The repeated-mode slice (`CanRepeatModes$ True`) is closed -- later instances pose their own ask. | `rules/cast.go` (`castModeAsk`, `modalTargetSA`), `rules/stack.go` (`resolveTop`), `effects/misc.go` (`effCharm`) | per-mode target groups (the distinct-mode half) |
| `non<X>` negation is generic over type words and the five colours; an `<X>` that is neither still fails closed (the safe direction -- `!hasType` on an unknown word would silently widen the filter). Three shapes remain unmatched: `nonColorless` (a colour-identity test), `nonChosenCard` and `nonCopiedSpell`. | `effects/filter.go` (`nonPredicate`), `effects/typewords.go` (`predicateTypeWords`) | M4 (a colourless/identity test; the Chosen/Copied stack-object predicates) |
| (pc1) A positive predicate word that is neither a type word nor `Colorless`/`MultiColor` fails closed and is reported by `UnknownPredicates`. The remaining families need object or game context this path lacks: `ExiledWithSource`, `wasDealtDamageThisTurn`, `IsImprinted`, `NotDefinedTargeted`, `DefenderCtrl`, `Opponent`. `DB$ Seek` is unregistered, so its carriers stay inert. | `effects/filter.go` (`wordPredicate`, `wordMatches`, `MatchesObjectCtx`, `UnknownPredicates`) | M4 (object/game-context predicates: LKI, zone-history beyond ThisTurnEntered, imprint) |
| Layer-4 type grants reach only the layer walk. Filters outside it -- target offer and legality, cost sites, `Count$Valid`, CantTarget specs -- read the printed face plus Changeling's CDA, so a creature a static made a Goblin is counted by a lord but not targetable by a Goblin-killer. Do NOT add a callable resolver field to `SpecContext`/`Ctx`: it poisons escape analysis. | `rules/layers.go` (`staticEffects`, `typeCharacteristics`, `matchesWithTypes`), `effects/filter.go` (`hasType`, `hasTypeCtx`, `typePredicate`) | M4 (layer-4 types in the ordinary filter grammar: target offer, cost sites, Count$Valid) |
| (setname1) Layer-3 `SetName$` names reach the layer walk and every SpecContext rules builds; a filter call rules did not build -- `effects.MatchesSpecFrom` from inside a resolving effect, a bare `*state.Game` -- reads the printed name, and the table is battlefield-only. The rename table is a plain `[]effects.ObjectName` on `SpecContext` and is refreshed from `emit`, gated on a genesis-time pool probe: do NOT make `specCtxSVars` call a function for it (it stops inlining, its `Resolve` closure escapes and every hot-path context allocates), and never put a rules pointer on `state.Game`. The Attached-replacement election itself asks name then creature type only for a `NameCard` body; every other `Attached` replacement leaves the event untouched. | `rules/setname.go`, `rules/statics.go` (`specCtxSVars`), `effects/filter.go` (`hasEffectiveName`, `sharesName`), `rules/replacement.go` (`applyAttachedReplacement`) | M4 (names in the context-free filter callers; the other `Attached` bodies) |
| Four trigger modes carry limits. **AttackersDeclared** fires once per defender, not per declare step, so a split attack over-fires a batch attack trigger. **Cycled** matches only a printed-`K:Cycling` face's cost-discard; layer-granted cycling misses and another card's cost false-fires. **CounterAdded** models only the crossing gate. **AttackerBlocked** misses `AddTrigger$`-granted instances. | `rules/trigger_match.go` (`attackersDeclaredOneTargetMatches`, `cycledMatches`, `counterAddedMatches`, `attackerBlockedCandidates`, `checkAttackerBlockedTriggers`) | M4 (per-declare-step batch latching, layer-granted cycling provenance, the batch-amount CounterAdded shape) |
| `Phase$ First Strike Damage` maps to the engine's single combat-damage step -- gorge never models two damage steps in one combat. When a first striker IS present, Forge fires the trigger in its own early step and regular damage later, while this build fires it once, at the single damage step. Dormant groundwork: no corpus line carries the phase name. | `state/phase.go` (`forgePhaseNames`) | M4 (a two-step combat-damage turn, or the mapping gated on first-strike presence) |
| (pw1) Printed `K:Compleated` is read through pay-time `CastInfo` provenance: each life paid for its Phyrexian symbols lowers entry loyalty by that amount. Missing: combat damage to a walker (defenders are players only), `[-X]` dynamic costs (one-generic fallback, read only for gating), walker TOKENS (TokenCreate skips `Move`, so one enters at 0 loyalty and the SBA kills it), transform walkers, Effect-delivered `NumLoyaltyAct`. A `Loyalty:X` face is EXEMPT from the zero-loyalty SBA. | `events/apply.go` (`Move`, `CastInfo`), `rules/sba.go` (`planeswalkerZeroLoyalty`), `rules/mana.go` (`ParseCost`, `addCounterCost`), `rules/legal.go` (`isLoyaltyAbility`, `loyaltyAbilityLimit`), `rules/cast.go` | M4 (walker combat damage, [-X] cost grammar, walker tokens, transform walkers, Effect-delivered NumLoyaltyAct) |
| (rv2b) Three fail-closed damage remainders: `DamageSource$` specs it cannot model (`Imprinted`, `Any`, `Valid <card-spec>`, `Spawner>...`, `ChosenCard`) keep the resolving source instead of redirecting; the exotic DamageAll `ValidPlayers$` lines deal no player damage; and exotic `<Ref>$<Property>` count heads evaluate to zero (CastTotalManaSpent, LifeTotal, CardCounters.ALL/AGE, CardNumColors). | `effects/damage.go` (`newDamageRider`, `validPlayers`), `effects/context.go` (`definedSpec`), `effects/count.go` (`evalRefProperty`) | M4 (the DamageSource$ lines, the ValidPlayers$ lines, the exotic count heads) |
| A mulligan declaration's REDRAW resolves immediately after that seat's declaration, interleaved with the pass's remaining keep/mulligan asks, rather than after every un-kept seat has declared (CR 103.4/103.5 has all mulligans happen simultaneously). Decision order and per-seat observable state are already correct; only the private redraw events' interleaving differs. | `rules/mulligan.go` (`handleMulligan`) | M4-or-later (redraws resolved together after each pass's declarations) |
| The starting player is uniformly random but the toss winner never CHOOSES (CR 103.1's second half): the build gives them no decision, so every playable game silently hands the toss winner the first turn. A start-of-game choice decision (new `decision.Kind`, a bot-policy arm, a host conveyance) is deliberately out of scope. | `rules/engine.go` (`New`) | M4-or-later (a start-of-game choice decision: toss-winner-chooses) |
| `K:Affinity:Affinity` discounts nothing (Sojourner's Enforcermite): "Affinity" is not a type word in the filter vocabulary, so the count fails closed to 0 and the spell always costs full price -- deterministic and conservative. Note the expansion's join separator is load-bearing: a dot-less spec joins the controller qualifier with `.`, a spec already carrying a dot joins with `+`. | `cards/keywords.go` (`case "Affinity"`), `effects/filter.go` (`MatchesObjectCtx`) | M4 (only if "Affinity" ever becomes a meaningful countable type, which no CR rule suggests) |
| (eqcm1) Three Equip remainders: a `Targeted$`-referenced equip reduction (belt_of_giant_strength, dragonfire_blade) resolves to 0 -- `ownReduceCost`'s Ctx has no target bindings; blackblade_reforged's `Creature.YouCtrl+Worthy` fails closed, so its equip is never offered; and `AlternateCost$` rider fields are dropped from the minted SA (rules reads no such activation param). | `cards/keywords.go` (`case "Equip"`), `rules/legal.go` (`ownReduceCost`) | M4 (target-anchored ReduceCost; the `Worthy` predicate; activation AlternateCost) |
| (cloak1) Turn-face-up (CR 708.6) is not implemented: a manifested or cloaked card stays face down forever. Loud-unimplemented: `ManifestDread` (a different API), `RememberManifested$`, `Defined$`-object manifests, the `Choices$` asking forms; for Cloak, the from-hand chooser and `Defined$ ValidLibrary`. Ordinary filters read the printed face, missing a manifested Elf. | `effects/zone.go` (`effManifest`, `effCloak`), `rules/layers.go` (`typeCharacteristics`, `faceDownPrintedHides`), `effects/filter.go` (`faceDown`) | M4 (turn-face-up CR 708.6 for both -- the Morph/Megamorph/Disguise ticket owns the shared path; ManifestDread; RememberManifested$) |
| (tokrepl1) Token-replacement stand-ins: an `Optional$ True` R: line takes the deterministic decline with no ask; `TokenScript$ Chosen`, `Type$ ReplaceController` and an unpriceable `Amount$` each emit a loud Note and leave the event verbatim; NO CR 616.1 order choice (scan order); `effToken`'s riders land on the FIRST mint only, so replacement extras get none. | `rules/replacement.go` (`replacementEvent`, `continueCreateTokenReplacements`, `applyTokenReplacementToPlan`, `tokenReplaceCount`) | M4 (asks for the Optional$/ValidChoices bodies, ReplaceController, rider propagation to extras, the CR 616.1 order choice, cost-token provenance) |
| (copyperm-grants) `api:CopyPermanent` now grants named `AddTriggers$`/`AddSVars$`/`AddAbilities$` from the resolving source table, supports a named `AttachedTo$` destination and the exact Zndrsplt `Choices$`/`Chooser$` shape. `WithDifferentNames$` remains loud-unimplemented, while `ImprintTokens$ True` still records no copies for delayed exile/sacrifice (Kharasha Foothills, Shredder, Shadow Master); `TokenRemembered$` is real. | `effects/copypermanent.go` (`effCopyPermanent`, unread-parameter Note block) | M4 (`WithDifferentNames$`, `ImprintTokens$`) |
| Replicate copy target choice now honors `MayChooseTarget$` through the shared copy target ask (see the CopySpellAbility row). The count ask's max is conservative: priced over the RAW composed cost (a ReduceCost modifier is unseen) and pool-only, so a Convoke contribution announced after the count cannot extend it. A bound of 0 degrades to a plain cast with a loud Note. | `rules/stack.go` (`AskCopyTargets`), `rules/cast.go` (`replicateAsk`, `nonManaCastable`) | M4 (modifier-aware and convoke-extended count bounds) |
| (multikicker1) No bot policy arm for the multikick count ask: a bot takes option 0 = "No multikick" through botpolicy's KChoose default arm, so it never pays a kick. A plain-Kicker cast's count rides the trailing event ONLY when the face carries a `Count$TimesKicked` SVar (`faceWantsTimesKicked`) — deliberate; a carrier reading the count without that SVar reads 0. | `rules/cast.go` (`multikickAsk`, `faceWantsTimesKicked`), `botpolicy/policy.go` | M4 (a bot-quality multikick policy) |
| (ft1) `withForetell`/`withoutForetell` remain unread, so granted-foretell visibility is not widened into ordinary filters. Cosmos Charger's any-turn timing grant and the Effect-delivered `MayPlay` static also remain unimplemented; the {2} action itself stays gated only on "during your turn" (deliberately no instant/sorcery-speed check). | `rules/legal.go`, `rules/cast.go`, `rules/altcast.go` (`foretellCastAvailable`), `effects/filter.go`, `rules/mayplay.go` | M4 (the remaining foretell predicates, timing grant and Effect-delivered MayPlay) |
| (ct1) A `Type$` category this build cannot enumerate (Basic Land, Card, Land, Planeswalker, CreatureInTargetedDeck, Nonbasic Land, Shared) keeps a loud Note plus a nonsensical CREATURE-type fallback. The siblings `effChooseColor` (first-WUBRG "W") and `effChooseNumber` (0) never ask mid-resolution. Zero or one offerable type takes the fallback with no ask. | `effects/choose.go` (`effChooseType`, `effChooseColor`, `effChooseNumber`), `rules/resolution.go` (the "choosetype" arm) | M4 (a non-creature option enumeration; the sibling ChooseColor/ChooseNumber asks) |
| (abcopy) Effect-GRANTED spell-cast copy triggers are invisible (the printed-face scan). The ValidStack exclusion anchors on the wrapper's whole printed-ability family, not its id — an id-only exclusion livelocks — so a second instance or copy is never offered and its controller never declines. The ValidStack copy arm is SINGLE-TARGET: only the first match in stack-arena order. | `effects/copy.go`, `effects/context.go` (`validStackTokens`), `rules/stack.go`, `rules/resolution.go` | none on the role path; M4 for the Effect-granted and plural-copy halves |
| (combatrestriction1) Only whitelisted static shapes are enforced; an unwhitelisted parameter means the static is SKIPPED, not enforced blanket — the deliberate permissive direction. `MustAttack$ ChosenPlayer`/`RememberedPlayer` name their defender for real (Territorial Hellkite's ChoosePlayer riders, via `attackRequirementSet`); every OTHER `MustAttack$` player reference stays unread and fails closed, and a NAMED duty whose defender no offered pair reaches releases the creature entirely (CR 508.1d: attacking anyone else obeys zero requirements too), so it is not marked Required. Unread: CantAttack's UnlessDefender$/Condition$/CheckSVar$ family, CantSacrifice's `ForCost$ True`, a `Target$` list's walker half, `stat:MustBlock`. | `rules/layers.go` (`SacrificeBlocked`, `attackBlocked`), `rules/combat.go` (`mustAttackRequired`, `attackRequirements`, `attackDutyDischargeable`), `effects/choose_control.go` (`effChoosePlayer`), `effects/misc.go` (`effEffect`) | M4 (the conditional parameter grammar; ForCost$ True cost provenance; stat:MustBlock; the walker Target$ half) |
| (attackprop1) The priced attack prop is mana-only; skipped permissively are non-mana cost tokens, the Phyrexian `{W/P}`, the `RememberingAttacker$` per-attacker price and a fail-closed `Condition$`. Tap window and budget share ONE narrow membership — one free-cost ability with deterministic production — so restricted, choice-shaped or paid sources cannot pay. | `rules/attack_cost.go` (`attackUnlessPrice`, `attackManaSources`, `attackBudget`, `attackPayWindow`), `rules/combat.go` (`handleAttackers`) | M4 (non-mana attack costs; {W/P}; paid/multi-ability/indeterminate tap sources; a per-attacker-variable price) |
| (blockprop1) Face `stat:CantBlockUnless` enforces only mana-priceable printed statics per (blocker, attacker), with `Attacker$` scopes and an absent `ValidCard$` matching every blocker (Awesome Presence); `PayLife<1>` (Heat Wave), `tapXType<...>` (Hollow Warrior), and Effect/Animate-delivered props are permissively skipped. | `rules/attack_cost.go` (`blockPairCharge`, `blockPayWindow`), `rules/combat.go` (`askBlockers`, `validateBlockers`, `handleBlockers`) | M4 (non-mana costs, per-blocker-variable prices, and delivered static forms) |
| (each1) Fails closed with no Note and no damage: damager specs naming unknown predicates, `ControlledBy TargetedController`, and a recipient `ValidTgts$` with an empty eligible set. An unresolvable `NumDmg` degrades to 0. Unread: `TargetUnique$` (a future multi-target ask needs a distinctness rule), the desc params, `RememberDamaged$`. The referent arm reads only `YouCtrl`. | `effects/damage.go` (`effEachDamage`, `eachDamagerTargets`) | M4 (the unknown damager predicates; a distinctness rule for multi-target asks) |
| (bestow1) Three exotic bestow costs are withheld and unoffered rather than degraded (the replicate convention): an `{X}` bestow cost, `CollectEvidence<6>`, and the `5 U U:GainControl` shape — `bestowCost` withholds on `Cost.Unknown` or `Cost.X`. A bestowed cast still fires "cast a CREATURE spell" triggers and would not fire "cast an Aura spell" ones (CR 702.114c). | `rules/bestow.go` (`bestowCost`), `rules/cast.go` (cost-mode map), `rules/legal.go` (the two offer blocks) | M4 (the three exotic costs; the creature-spell/Aura-spell trigger nuance) |
| (ap1) AddPhase loud-degrades (one Note, grant still applied) on an unresolvable `ExtraPhase$`/`AfterPhase$`/`FollowedBy$`/`NumPhases$`/DelTrig value or a multi-step set. Deliberate: a grant whose splice point has passed is dropped silently at TurnChange; a splice mid-range of another active extra phase is untested; a multi-completion takes the LAST entry's resume point. `FirstAttack$` unread. | `effects/addphase.go` (`effAddPhase`, `parseExtraPhaseValue`), `rules/turn.go` (`extraPhaseBoundary`) | M4 (a real per-turn attack history for the unread `FirstAttack$` gate) |
| (staticgoad1) A `Goad$ True` static delivered by `AddStaticAbilities$` or `StaticAbilities$` is invisible: `activeStatics` walks printed `Face.Statics` only and never expands those grants, so those creatures are never goaded. A statically goaded creature is also invisible to the `Creature.IsGoaded` predicate, which reads only the event-backed `state.Object.Goads`. | `rules/combat.go` (`staticGoaders`, `hasActiveGoad`, `goadedBy`) | M4 (expanding the grants in `activeStatics`; the `IsGoaded` predicate reading the static route) |
| `Cost$ Mandatory PayLife<X>` replacement bodies run for free and never pay, on two blockers: `DB$ StoreSVar` is unregistered (the body hits the unimplemented-API fallback and moves nothing), and a replacement is off the mandatory-settle path, whose window has no channel for the X-announcement and `xPaid` binding the cost needs. `mandatorySettleShape` deliberately arms only Sac/Exile. | `rules/cumulative.go` (`mandatorySettleShape`), `rules/replacement.go` (`runReplaceWith`), `rules/mana.go` (`Cost.LifeX`) | M4 (register `StoreSVar`; an X-announcement and `xPaid` binding for a replacement body) |
| (mtsp1) `TriggersWhenSpent$` fires only on `Mode$ SpellCast`: `Mode$ SpellAbilityCast` is deliberately unsupported, and ability activations, the unless-pay arm and every non-spell payment fire nothing. A carrier pairing the rider with `RestrictValid$`/`AddsNoCounter$` loses the rider's provenance to the restriction encoding under one loud Note. | `effects/misc.go` (`effMana`), `rules/cast.go` (`fireManaSpentTriggers`), `rules/stack.go` (`emitRestrictedManaSpend`) | M4 (ability-activation and `SpellAbilityCast` riders) |
| (castfilter1/2) `Count$CastTotalManaSpent <Type>` sees only Snow/Treasure/Cave/Desert: the other positive ManaAdd producers (cumulative-upkeep AddMana, ManaReflected) stay UNTAGGED and never count, and an unknown `<Type>` arg keeps the fail-closed 0. The `TriggeredCard$CastTotalManaSpent` ref-head still evaluates to zero. | `effects/count.go` (`evalCountBody`'s `CastTotalManaSpent` case), `effects/misc.go` (`effMana`'s tagging), `rules/mana.go` (`takeUnit`) | M4 (the `TriggeredCard$` ref-head and the untagged producers) |
| (addcounter1/2) Competing AddCounter replacements compose in deterministic scan order with NO CR 616.1 order choice. A counter `events.Move` folds in as an entry characteristic emits no `CounterChange`, so no replacement sees it. While a replacement body is in flight `emit` bypasses `applyReplacements`, so a counter placed from inside one is unchecked against `PutCounterBlocked`. | `rules/replacement.go` (`applyAddCounterReplacements`, `applyReplacementsDispatch`), `rules/layers.go` (`PutCounterBlocked`), `rules/engine.go` (`emit`) | M4 (the CR 616.1 order choice) |
| (api:Clone) The ETB-copy route (a `ReplaceWith$` body of `DB$ Clone`) is NOT implemented, so Vesuva's copy never fires. `Choices$` takes the deterministic first-eligible object under a Note (R-9). `GainThisAbility$ True` appends the become object's WHOLE ability list and SVar table. `UntilFacedown`/`UntilTargetedUntaps` approximate to until-the-object-leaves; the modifier-param list is unread. | `effects/clone.go` (`effClone`, `cloneDuration`, `cloneUnreadModifiers`), `rules/layers.go` (`settleExpiredClones`) | M4 (the ETB-copy route; a real `Choices$` ask; the unread modifier list; `UntilFacedown`/`UntilTargetedUntaps`) |
| (mutate1) The CR 702.140b over/under placement ask has no dedicated bot arm: a bot takes option 0 = "On top" through the clamp fallback. A mutate pile is one live object plus parked shells, so a second mutate (or its death) re-parks top cards and `g.Objs` grows. Owning-face SVar routing is closed at four named sites only. | `rules/mutate.go` (`mutatePlaceAsk`), `rules/trigger_queue.go` (`faceOwningTrigger`), `botpolicy/policy.go` | M4 (the placement-ask bot arm) |
| (battle1) Attacking a battle (CR 310.7/310.11) is not modelled at all: defenders are players only, no combat damage removes a defense counter, the "defeated" exile-and-cast arm never runs, and a battle at zero defense goes to the graveyard. The protector ask has no bot arm (option 0 = lowest-seat living opponent), is never re-chosen if that player leaves, and only `Siege` gets it. | `rules/sba.go` (`battleZeroDefense`), `rules/replacement.go` (`applySiegeProtector`), `rules/turn.go` (the `chooseSiege` arm) | M4 (attacking a battle: the CR 310.7 defender, combat damage, the CR 310.11 defeated exile-and-cast; a real protector policy) |
| (choosesource1) The Effect one-shot ender is bound to the replacement-body path only: the `Origin$ Command`, `Destination$ Exile` self-exile idiom reached from an Effect's `Triggers$` body, an ordinary spell or ability, or a resumed replacement body (whose Ctx has no frame) ends nothing, so the effect persists for its whole duration. Colour-source and shadow predicates fail closed. | `effects/choose_control.go` (`effChooseSource`), `effects/zone.go` (`effChangeZone`'s EffectFrame arm), `rules/replacement.go` (`seedEffectReplCtx`) | M4 (the one-shot ender on the Triggers$/resume paths; the colour-source/shadow predicates; emblems) |
| (chosencopy1) `CanBeTargetedByTriggeredSpellAbility` is the target-SPEC half only — the protection/shroud/hexproof/CantTarget half is not modelled (the filter tier cannot reach rules' legality registry), so a protected creature still enters the choice pool. `UnlessCost$` SVar folds apply to Counter and CopySpellAbility when the shared count evaluator resolves the body; other APIs retain the strict grammar. Unresolved X/Y and the remaining dynamic cost grammar still hard-decline. | `effects/filter.go` (`triggeredSpellTargetSA`), `effects/unless.go` (`UnlessCostResolved`), `effects/copy.go` (`copyDefinedTargets`) | M4 (full canBeTargetedBy legality through the filter tier; the remaining unless-cost grammar) |
| (manaexpend1) `trig:ManaExpend` counts pool mana only: convoke, delve, free casts and ability activations do not count, so a convoke-assisted cast under-counts (the CR 702.10 ruling is unsettled; the pool-delta reading was kept deliberately). `Player$ You` is the only selector read — any other value, and an unresolvable or non-positive `Amount$`, fails closed. | `rules/cast.go` (`manaExpendAdd`, `manaExpendTotal`), `rules/trigmatch_cast.go` (`manaExpendMatches`), `rules/engine.go` (`manaExpended`) | M4 (count convoke contributions once a ruling settles it) |
| (maxpower1) The `MaxTotalTargetPower$` prune does not model `TargetMax$` bounding how many negative-power candidates one selection may take, and `askCrossModeCharmTargets` (the cross-mode TargetUnique Charm ask) does not read the cap at all — corpus-unreachable today, since neither carrier is a Charm mode. | `rules/stack.go` (`totalPowerCappedCandidates`, `askCrossModeCharmTargets`), `rules/cast.go` (`targetAsk`) | M4 (a TargetMax$-aware offset bound; the cross-mode Charm ask if a carrier appears) |
| (devthr1) Six repo-deck `Count$` bodies still degrade to the unresolvable verdict and are held in `knownUnmodelledCountHeads`: `MaxOppDamageThisTurn`, `YourStartingLife`, `ResolvedThisTurn`, `CardNumAttacksThisTurn`, `NonCombatDamageThisTurn`, `ChosenNumber` (the last being the documented unbound-context verdict). The ratchet is bidirectional per body. | `effects/count.go` (`evalCountBody`), `rules/count_head_ratchet_test.go` (`knownUnmodelledCountHeads`) | M4 (the six remaining table bodies) |
| (kw:MayFlashSac) The keyword is implemented rules-side, not as a `cards/keywords.go` expansion: its meaning is a casting OPTION, which `expandKeywords`' own doc lists as the family rules reads directly, and the sibling `kw:MayFlashCost` landed the same way. The rider ("if you CAST it any time a sorcery couldn't have been cast") rides a pay-time `FlagMayFlashSac` CastInfo, and `state.CastProvenanceFlags` strips that bit from a stack copy, since a copy is put on the stack and never cast (CR 707.10). Deliberately NOT changed with it: the sibling entry-hook bits `FlagEvoked`/`FlagDashed`/`FlagWarped` are still inherited by a copy, because they are conditioned on an alternative COST having been paid — a cast-time choice the copy rules do carry for the comparable kicked case. | `rules/mayflashsac.go`, `rules/statics.go` (`spellTimingOK`), `rules/altcast.go` (`altCostEnter`), `state/object.go` (`CastProvenanceFlags`), `events/apply.go` (`StackCopy`) | M4 (a copy ruling for the evoke/dash/warp entry hooks) |
| (kw:Flanking) The layer walk binds the KEYWORDS-so-far list (`matchesWithChars` -> `SpecContext.ExtraKeywords`), so an `Affected$ ...+with<Keyword>` lord sees a keyword an earlier layer-6 effect granted (Cavalry Master over a Sidewinder Sliver); CR 613.6 dependency reordering is NOT modelled, so a grant whose effect is OLDER than the one it depends on still misses the instance (the conservative direction). `derivedScalarFrom`'s layer-7 P/T walk binds no keyword list at all, so a P/T lord gated on a GRANTED keyword still misses it. Flanking instance counts come from that derived list, so both narrowings carry into CR 702.25b. | `rules/layers.go` (`matchesWithChars`, `derivedWith`, `derivedScalarFrom`), `effects/filter.go` (`keywordPredicates`, `SpecContext.ExtraKeywords`), `rules/trigmatch_combat.go` (`flankingInstances`, `queueGrantedFlanking`) | M4 (CR 613.6 dependency ordering; a keyword-aware layer-7 P/T walk) |
| `kw:Infect` (CR 702.90) is real for combat, non-combat and cost damage, and a granted infect reads CR 113.7a's last known characteristics when its source left the battlefield before the ability resolved (the same capture carries deathtouch; `Wither` is unimplemented engine-wide and so has no such gap). The two rules-side cost sites still read infect -- and lifelink -- off the LIVE source with no LKI consult, the pre-existing shape they share. Two infect carriers stay blocked on unrelated unregistered primitives and are not this row's debt: Grafted Exoskeleton on `trig:Unattached` and Ichor Rats on `api:Poison`. | `rules/infect.go`, `effects/damage.go` (`newDamageRider`), `rules/engine.go` (`damageKeywordLKI`), `rules/cast.go` (`payDamageCost`), `rules/resolution.go` (`payUnlessDamageCost`) | M4 (LKI at the two cost sites) |

| `api:ExchangeLifeVariant` fails closed when life is prevented or transformed, and when a competing life-replacement choice parks the event; the characteristic setter is not installed until an unchanged `LifeChange` is actually applied. Ordinary replacement choices therefore leave the exchange incomplete rather than risking mismatched life and P/T. | `effects/life.go` (`effExchangeLifeVariant`), `rules/stack.go` (`EmitLifeChange`) | M4 (transactional exchange continuation) |

## Trigger-relative filter arguments (pg2)

`ControlledBy <ref>` and `OwnedBy <ref>` recognise exactly `TriggeredTarget`,
`TriggeredDefendingPlayer`, `TriggeredPlayer` and `TriggeredCard`; an absent
binding fails closed (also under `!`); `Targeted*` and `Spawner>...` chains
stay unknown. `effects.TriggerContext` carries event roles separately from the
resolving ability's source, targets and Remembered, survives suspension,
cloning and stack copying, and is rebuilt by replay without new events. Not
implemented: `TargetingPlayer$` (Magus of the Abyss asks the trigger
controller), LKI owner/control snapshots for a referent that changes before
resolution (live owner/controller is read), carrying these bindings into a
registered continuous effect, and the Self/Other target-source normalisation
(Flickerwisp's `Permanent.Other`).

## Cast-provenance filter arguments (castprov1/2/3 + wascastfrom)

The `Card.wasCast*` family is a rules-side split, not a filter predicate:
`castProvenanceAdmits` (`rules/cast_provenance.go`) strips the tokens and reads
the log (latest PutOnStack cast wins; a copy was never cast; a never-cast card
reads false). Hand family: `wasCastFromYourHandByYou`, `wasCastByYou`,
`wasCastFromYourHand`. Origin-zone family: `wasCastFromExile`,
`wasCastFromYourGraveyard` and `...ByYou` (identical here: a cast's origin zone
is always the caster's), `wasCastFromTheirHand`. Bare `wasCastFromGraveyard`
is the effects-side CastFlags predicate (flashback/harmonize/escape) instead.
Wired at the trigger match walks, `Count$ThisTurnCast_<spec>`,
`Count$wasCastFromExile`, the target walks (offer and CR 608.2b recheck) and
the CantBeCast walk (`castOriginAdmitsAtZone` reads the pending cast's
origin). Still open: the tokens are not evaluated by effects' own
ConditionPresent/ConditionDefined evaluator (such a gate runs its sub
unconditionally), there is no `Count$wasCastFromYourGraveyard` head, and the
may-play provenance predicates (`MayPlaySource`/`CastSa`) stay fail-closed
(see the ValidLKI row).

## Host behaviour notes (embedder observer hooks, D15)

`OnBurst` errors crash the match like a persist failure (D15): the table
halts and the chain does not continue. `OnMatchEnd` errors are discarded
because the outcome is already recorded and an error cannot un-record it, so
an embedder that persists through `OnMatchEnd` must handle its own
persistence failures inside the callback.

## Seat-privacy boundary

A seat credential names **both its table and seat**. Every current
seat-authorised HTTP route (`view`, `events`, `pending`, `intent`, `undo`)
goes through `host/httpapi`'s `claimForTable`; a mismatched or legacy unbound
claim is 403 and must never reach registry state. `cmd/gorged` mints startup
and vs-bot tokens with that pair, so after deployment old tokens stop working;
the web client returns a rejected stale join to the lobby, where it obtains a
fresh game claim. Stream session ids are random 128-bit values issued only in
`hello`; `host.Session.serial`, not the public id, preserves fan-out order.

## Running a gorged server while you work

Two orchestrator sessions share this box, and one of them serves a live demo
that the operator redeploys by hand (`make deploy-demo`; a redeploy aborts
every in-flight vs-bot game, so nothing does it automatically). So ports are
allocated, not first-come:

| range | who |
|---|---|
| 8080-8081 | the demo. **Never bind these**, and never run `make deploy-demo` |
| 8082-8089 | the bot-policy / botbench workstream |
| 8090-8099 | task agents on the engine side — pick one of these |

Put persistence under `/tmp/gorge-<something-unique>`, never the repo-root
default `gorged-data`: several servers sharing one persistence directory
silently corrupt each other, and a resumed directory written by an older binary
comes back with the fields that binary lacked set to their zero values
(`Format`'s zero is `constructed`, a real value, so `-format` is ignored with
nothing in the output to say so).

**Stopping a server: never `pkill -f` or a bare `pgrep -f`.** The pattern is
matched against every process's `/proc/<pid>/cmdline` including your own shell's,
and it has killed a session here. Find the server by its listening socket
(`ss -lptn`), confirm the pid, then signal that pid. Note that `/proc/<pid>/comm`
is the BINARY's name -- if you built `gorged-after`, its `comm` is
`gorged-after`, not `gorged` -- so match on the port rather than the name.

`scripts/fleet.sh ports` prints the current allocation, and `scripts/fleet.sh
port` prints a free one in your range.

## Working in a task worktree

**Every change is made in its own worktree, and `main`'s checkout is left
clean.** This is not only for dispatched seats -- it applies to an operator
session's own hand work too, including a one-line fix. Merge into `main`, then
remove the worktree immediately: a stale worktree is what makes the next
agent's `git grep` and `find` return duplicate hits from a sibling checkout.

The reason is that several sessions share the one `/home/sadams/projects/gorge`
checkout, and a working tree has exactly ONE HEAD, so anything done to it
happens to everyone:

- `git checkout -b` in the main checkout to commit your own work switches the
  shared tree. A peer switching back to `main` then reverts your edited files
  out from under you mid-task.
- `git reset` and branch switches are worse than dirty files: they move HEAD
  for the other session too, so a peer's next commit lands on a base it never
  chose.
- A peer mid-refactor leaves the shared tree not compiling. That has produced
  19 build failures in `go test ./...` belonging to nobody in the room. **In a
  shared checkout a red suite is not evidence about your own change**: check
  `git status` for untracked or modified files you did not write, and their
  mtimes -- a file seconds old is a peer mid-edit, not a defect to fix, and not
  yours to fix. (Red that is genuinely committed on `main` is still yours to
  bisect and fix in the same turn.)

So, in the main checkout: never `git checkout`, `git switch`, `git reset` or
bare `git stash`, and never `git checkout` inside another worktree either (it
discards that seat's uncommitted work). Stage explicit paths and never
`git add -A`, which sweeps up a peer's in-flight files.

Task worktrees are created with `scripts/agent-worktree.sh <id> [base] [--web]`,
never with a bare `git worktree add`. A worktree carries only tracked files, and
this repo needs one untracked thing to test honestly: the `.cards` corpus. Without
it `internal/testutil`'s `CorpusRegistry` calls `t.Skip`, so every
corpus-dependent test SKIPS instead of running, the package still prints `ok` --
in about 2ms -- and the run reads green to you, to the review pre-filter and to
the gate while having executed almost nothing. If you find yourself in a worktree
with no `.cards`, stop and say so rather than reporting a green suite.

Which agent seats exist, what they cost and when to escalate between them is
recorded in `docs/superpowers/agent-seats.md`.
