# Mutate (CR 702.140): a merged permanent on both seats

Date: 2026-11-27.
Status: **design decision.** gorge's side shipped long ago (this doc's
"reference" section is the live code, not a plan); dirac's side was parked
behind exactly one question — a new `dirac.State` field or a side pool — and
is split into three implementation tickets at the end. No engine code changes
here.

## The decision

**A side pool, not a new fixed `State` field.** The kernel's merged-permanent
pile lives in one cold region (`newCold`, gated on a `genPrep.anyMerge`
table bit), shaped exactly like the meld link region (`dirac/face_meld.go`:
`meldRec{res, partner objWord}`, `maxMeldLinks = 16`, `coldMeld`), and the
permanently-merged cards are **parked objects** in `kop.ZoneNone` — the same
representation meld already uses for its partner (`face_meld.go`: "the
PARTNER object parks in `kop.ZoneNone`, on no zone list"). Three reasons,
all enforced by tests:

1. `perm` is 72 pinned bytes that `ObjHash` hashes whole
   (`dirac/state.go`, the `atkVer`/`spare` comments), and
   `TestNoNewFixedStateFields` (`dirac/layout_pins_test.go`, `testdata/layout.tsv`)
   refuses a new fixed field outright. The pile is per-object data, so it
   cannot be a `perm` field.
2. A per-object fixed array outside `perm` would still grow `State` past its
   64 KiB bar (`maxStateBytes`, `dirac/kernel_test.go:50-60`) — a
   per-object-slot array would be paid in every table, including the many
   that never print Mutate. The cold-region pattern keeps it out of every
   non-Mutate game's footprint (`coldMeld` is 0-length unless a program is
   `ExtMeld`; `dirac/face_meld.go:37-45`).
3. The hard rule (`AGENTS.md` "dirac.State holds no references… the
   transition loop allocates nothing") is satisfied by a pointer-free
   `uint16`-keyed region the same way `coldMeld` satisfies it.

**A new action kind, not a new payment decision.** Mutate's cost is a
*substitution*, not an added cost component, so it needs no new
`decision.Question` kind in the pay walk. It is the third member of the
bestow/sneak family: its own action kind `AkMutate` beside `AkBestow`
(`dirac/types.go:80`, `kw_bestow.go`'s documented extension), priced like
bestow's `CardDef.BestowCost` and answered by the existing hand-cast
machinery.

## The rules, and what the corpus actually prints

CR 702.140 (`MagicCompRules-20260807`) with CR 730 "Merging with Permanents":

- 702.140a: "Mutate [cost]" = pay [cost] rather than the mana cost; the
  spell becomes a mutating creature spell and **targets a non-Human creature
  with the same owner as this spell** (the modern, post-Ikoria wording — every
  corpus card uses it, below).
- 702.140b: an illegal target as it begins resolving un-mutates the spell; it
  resolves as an ordinary creature spell.
- 702.140c / 730.2: a legal merge does NOT enter the battlefield; the spell's
  controller chooses **top or under** (the modern rules have no mana-value
  comparison). The result is one object, the same object as before (effects
  and counters stay; CR 730.2c).
- 702.140e / 730.2a: the permanent has **all abilities of each component**
  and only the **topmost component's other characteristics** (a copiable
  effect with the merge's timestamp, CR 613.2).
- 730.3: it leaves the battlefield as ONE permanent and each component is
  put into the destination zone separately.

Corpus shape (measured 2026-11-27 over `.cards/cardsfolder`): **34 files
carry a raw `K:Mutate:<mana>` line**, matching the manifest census lines
that are the live counts (`dirac/manifest/KwID/KwMutate`:
`census: kw Mutate all=34 std=0 mbc=1 sole=2`, `status: active`;
`dirac/manifest/TrigEvent/TrigMutates`: `census: trig Mutates all=32
std=0 mbc=1 sole=3`, `status: reserved`; computed by
`dirac/kgen/censusstd.go`, seeded by `dirac/manifest/seed.go`). The ticket's
"38 carry / 33 sole" is a wave-2 re-measurement claim on `b1479e02a4`'s pin;
the manifest rows are what a lowering must actually move. Every corpus card is the
modern variant (`.cards/cardsfolder/c/cloudpiercer.txt`: `K:Mutate:3 R` +
`T:Mode$ Mutates | ValidCard$ Card.Self | …`), no Ikoria-style mana-value
form exists in the corpus, and the trigger mode is uniformly `Mode$ Mutates`
(29+ cards). The cost parameter never contains a nested colon
(`engine/rules/mutate.go:30-32` says "no corpus mutate cost itself contains a
colon — they are all plain mana symbols and hybrids, measured over all 34 raw
K:Mutate lines").

## gorge's reference implementation (source of truth for the kernel)

| Concern | Code |
|---|---|
| cast mode | `engine/rules/mutate.go` — `mutateCost` (:33, reads Forge's `K:Mutate:<cost>` param via `f.KeywordParam("Mutate")`, withholds X/unknown), `mutateTargetSA` (:58, a hand-built `cards.SA{API:"Mutate", ValidTgts: "Creature.nonHuman+YouOwn"}` standing in for the face's missing SP — the `bestowedAttachSA` precedent), `mutatePlaceAsk` (:71, a real two-option `KChoose` asked ONCE during the cast, answer folded onto `pendingCast.mutateTop`), `mutateVictim` (:105, CR 608.2b recheck at resolution start), `resolveMutate` (:126, emits `events.Resolve` + `events.Mutate` and returns WITHOUT `moveResolvedOffStack`) |
| offer | `legal_walk_alt.go:242-252,349-351` — `offerCastable(p, id, mc, spellScope("mutated"), false)` + `Option{Mode:"mutated"}` in the hand and graveyard walks; `cast_begin.go:491` `case castModeMutated:` substitutes `mutateCost` for the printed cost |
| placement provenance | the choice rides the pay-time `CastFlags` bit `state.FlagMutatedTop` (plus `FlagMutated`), so a log-only replay rebuilds it (CR 702.140b) |
| the pile | `state.Object.MergedCards []MergedCard` (`object.go:890`, type at :2187: `{Obj, Card, FaceIdx}` — the card is duplicated onto the pile value so scans need no second lookup and a lost parked object still leaves its abilities live) and `TimesMutated int32` (:1831-1843, CR 702.140f, read by `Count$TimesMutated`) |
| the fold | `events.Mutate` (kind at `event.go:599-611`, descriptor `kindinfo.go:163 {Name:"mutate", Trigger: TriggerNone}` — trig:Mutates matches through the past-the-mask fail-open path); `foldMutate` (`engine/events/apply_copy.go:18-88`): survivor's `Card/FaceIdx` always describe the TOP card; "top" demotes the old top into a freshly minted `ZCeased` parked object (with Commander-designation retargeting, :59-68), "under" parks the caster; `TimesMutated += Amount` |
| split | `events/apply_zone.go:719-731` — the Move fold: `MergedCards = nil`, `TimesMutated = 0`, and each parked component recurses `Move(g, mc.Obj, ZCeased, to)` with `SplitFrom/SplitDep` stamps (CR 400.7, 702.140e) |
| ability union | `engine/state/pile.go` — "the ONE flat view of a permanent's rules text that spans the top face and every card merged beneath it": `PileFaceAt/PileFaceCount`, `PileAbilityCount/At/Index`, `PileStaticCount/At`, `PileSVars`; top face first, then `MergedCards` in pile order, so pre-mutate flat ability indexes (and the wire's `Option.Ability`) stay byte-compatible. Consumed by `legal_granted.go`, `mana_activation.go`, `potential_plan.go`, `legal_walk_battlefield.go`, `statics.go`. |
| characteristics | `engine/rules/chars` reads `o.Face()` — always the TOP card, exactly 730.2a; the pile-aware reads are the keyword abilities (702.140e: the layer walk's base list spans the pile, `chars.PrintedKeywords`) and layer-4 type memoization keyed on `MergedCards` (`engine/rules/layer4types.go`, because under-card statics may change types via `PileStaticAt`) |
| triggers | `engine/rules/trigmatch/mutate.go:11-46` (`registerTrigMatcher(mutatesMatches, "Mutates")`; matches on `ev.Kind == events.Mutate` against `ev.Obj` = the surviving pile, `ValidCard$` aware); under-card triggers are minted by `events.MergedTriggerPush` (`event.go:612+`, `apply_trigger.go:595-603` packs `(pile index, trigger index)` into `Amount`); `trigger_match.go:2289+` `triggerFacesWithMerged` appends under-card faces at `faceIdx 2+i`; `trigger_eligibility.go:585-594` keeps a `Mutates`-only face's mask narrow |
| view/wire | the split cards are the *same ObjIDs* the pile's parked components always were (`apply_zone.go` Moves them from `ZCeased` in place), so client references survive the merge and the split; transcript lines `describe.go:346` ("mutated") and :818 (a pile's under-card trigger); spellbench maps `"mutated"` (priority.go:21) and the mutated-pile ability index (priority.go:75,91); manabrew `mb.ModeAlternative` + `Cost:"mutate"` (`server/manabrew/prompt_priority.go:69`) |
| payment | none: the mutate cost *replaces* the cost before payment; `cost.Cost` gains no component, and the spell's identity rides the CastFlags mode bits |

## Dirac data model

The kernel already models "one battlefield object backed by two cards" for
meld: the result's `perm.card` is the melded face, the partner parks in
`kop.ZoneNone`, and one `coldMeld` record links them
(`dirac/face_meld.go:9-19`; the mirror on gorge's side is
`engine/state/meld.go`). Mutate generalizes that to an ordered pile, with the
**top component kept in `perm.card`** (so every existing characteristic read
— `cardAt`/`typesAt`/`isCreature`/PT in `dirac/chars.go` — sees the top card
for free, exactly as gorge's `Face()` reads only the top) and the under
components as parked objects:

```go
// dirac/kw_mutate.go (new), model: dirac/face_meld.go
type mergeEntry struct{ obj objWord; card CardID; face uint8 } // 5 B, pad 6
const mergeCap = 4  // components under the top; corpus max is 2
type mergeRec struct {
    top  objWord
    n    uint8          // components under the top, <= mergeCap
    tm   uint8          // CR 702.140f TimesMutated (Count$TimesMutated)
    _    [2]uint8       // explicit padding: ObjHash-style byte hygiene
    und  [mergeCap]mergeEntry
} // 24 B
var coldMerge = newCold[mergeRec]("merge", func(g *genPrep) int {
    if g.anyMerge { return maxMerged /* 16, like maxMeldLinks */ }
    return 0
})
```

- **Capacity is a declared limit** (`mergeCap = 4`, `maxMerged = 16`
  concurrent merged permanents): a merge that would overflow makes the
  mutating cast an *illegal option* in the kernel, and the oracle counts it
  as a capacity stall (AGENTS.md: "capacity stalls are declared limits and
  always counted"). The corpus's 34 mutate cards realistically reach 2–3
  components; the sentinel/oracle sweep reports the real observed max
  (a `DebugMeldStats`-style counter, `face_meld.go:53-61`, is the model) and
  the ticket must record it.
- **New ordinals, all through the manifest** (AGENTS.md "a dirac op, counter
  kind, ExtOp, keyword, trigger event… a `dirac/manifest` entry with
  `value: next`", `go run ./dirac/cmd/kmanifest assign`): one `ExtOp`
  (`ExtMergePermanent`, the merge effect op, mirror of `ExtMeld`), one
  `EventKind` (the kernel's mutate event, mirror of gorge's
  `events.Mutate` — it drives TrigMutates matching, the transcript and the
  lockstep divergence report). The atoms a pile read needs are already
  reserved and unused: `kop.KwMutate = 34`, `kop.TrigMutates = 56`,
  `kop.ExTimesMutated = 80` (`dirac/kop/manifest_gen.go:1952,2710,977`) and
  `keyMutate = 3` (`dirac/manifest_gen.go:119`). A new action kind
  (`AkMutate`) joins the `Ak*` iota list and `scriptPlayKinds`
  (`dirac/types.go:80,93`).
- **Merge = the fold, not the op, for the pile mutation.** gorge folds the
  pile in `foldMutate` from one event; the kernel mirror fires from the new
  EventKind: top → park the old top's card as a fresh parked object (mint,
  `ZoneNone`), `perm.card =` the mutating card, `und = [demoted] + und`;
  under → park the spell's object, `und = und + [spell]`. Commander
  designation follows the card across the swap (gorge `apply_copy.go:59-68`
  retargets `Players[p].Commanders`; the kernel retargets `player.cmd` /
  `cmdPartner`, `dirac/state.go:482-485`, `zone_cmdpartner.go`) — **required
  by the ticket's own motivating deck** (Everquill Phoenix is a Commander).
- **Split** hangs on the leave path in `dirac/zones.go`, beside the meld
  split (`face_meld.go:171-174` `meldLeave`, which runs before the result's
  own placement): when a merged permanent leaves for zone z, each parked
  component moves `ZoneNone → z` ahead of the top, in pile order, silently
  (gorge's `Move` does the same from `apply_zone.go:719-731`; the components
  are new objects — CR 400.7 — so their `ver` bumps, and gorge's
  `SplitFrom/SplitDep` stamps get kernel mirrors only if a lowered card reads
  them; none does today).
- **Ability union**: every kernel scan that reads an object's card for
  abilities must, when the record is non-empty, also walk `und[]` and run the
  parked object's card programs: the static collectors (`dirac/static_*.go`,
  the `collect*`/`active*` passes), the trigger scan, the keyword reads
  (`dirac/filter_withkeyword.go`'s `printedKeyword` — the comment at
  `dirac/kgen/filter_words_relations.go:163-168` states the fail-closed
  `Mutate` default is exact **only because** the K: line refuses the face;
  accepting K:Mutate makes it a live obligation), and the activated-ability
  offers. Name/CManaCost/colours/P-T stay top-only (730.2a) and need no
  change: they already read `perm.card`.

## The cast and the placement ask

Mirror bestow's extension (`dirac/kw_bestow.go:15-33`, which is itself the
documented AkSneak/AkCastCasualty pattern):

- `AkMutate` action kind; the build is a plain hand cast priced at the new
  `CardDef.MutateCost` (lowered from the `K:Mutate:<cost>` parameter —
  `parseManaCost`, refuse X/hybrid/`<` like Overload's arm at
  `dirac/kgen/lower.go:2078-2086`), and the stack entry carries a new
  `modeMutated` bit beside `modeBestowed` (permanent spells have no modal
  bits, `kw_bestow.go:22-24` — bestow and mutate collide on one card, so
  kgen refuses the pair by name).
- Its slot-0 target is the synthesized mutate target, the kernel analogue of
  gorge's `mutateTargetSA()`: a `Creature` + not-Human + same-owner filter
  over battlefield creatures of the caster (the target machinery and
  filter-words already cover "same owner" and type tests; the non-Human
  test is the one new filter read).
- The **top/under choice** is asked at cast time like gorge's
  `mutatePlaceAsk` (not at resolution), as a two-option ask carried as a new
  `dirac/manifest/ask` entry (the `ra*` registry, e.g. `raDredge`), and it
  rides the stack entry's mode/flag byte so replay rebuilds it — gorge's
  equivalent is `FlagMutatedTop` on `CastFlags`.
- Resolution: the kernel's resolve path diverts a mutated permanent spell
  into `ExtMergePermanent` instead of the BTB move, with the CR 702.140b
  un-mutate fallback (illegal target → ordinary creature entry) reusing the
  existing illegal-target un-attach shape gorge uses
  (`engine/rules/stack.go:602-610`: `AlterAttribute AttrMutating` clears
  `FlagMutated/FlagMutatedTop` and the spell falls through to the ordinary
  permanent tail — in the kernel the mode bit clears).
- Payment itself is untouched: like evoke/dash/bestow, the alternative cost
  substitutes before the pay walk (`keywordAltCost`'s kernel counterpart),
  and the offer is gated the way `altGate*` rows gate gorge's family.

## Trigger, TimesMutated, support flip

- kgen: `K:Mutate` stops falling through the keyword switch's default refusal
  (today: `lower.go`'s `lc.refuseNode(node, "keyword %q"…)` arms; the
  expected refusal is pinned in
  `dirac/kgen/lower_destalt_exile_test.go:45` — `want: {`keyword "Mutate"`,
  "trigger mode Mutates"}` for Illuna — which that test's own comment says
  the mutate-lowering commit MUST update, "Do not simply delete these
  expectations": the exile-origin DestinationAlternative$ arm then becomes
  reachable and must lower as a prelude ask or refuse by name).
- `T:Mode$ Mutates` lowers to a `TrigEvent/TrigMutates` match on the new
  kernel mutate event (mirror of `engine/rules/trigmatch/mutate.go`),
  registered beside the other trig matchers in the `dirac/trig_*.go` family.
  `TrigMutates` is `status: reserved` today, so the manifest guard
  (`dirac/kgen/manifest_guard.go:12-16`: a card carrying a reserved code is
  refused, class `manifest:reserved:<Family>/<Name>`) flips to active in the
  same commit as the kernel matcher — never before it.
- `Count$TimesMutated` lowers to `kop.ExTimesMutated` (reserved, unused) and
  the kernel reads `mergeRec.tm`; the field zeroes with the pile on split.
- Acceptance flips are mechanical: `KGEN_REPIN=1`/`DIRAC_UPDATE_SUPPORT=1`
  re-pin the corpus pins (`dirac/kgen/ratchet_test.go`,
  `testdata/acceptance.tsv` + `build_acceptance.tsv`) and rewrite
  `dirac/docs/support.md` (`TestSupportDocIsCurrent`); the 33 sole-mutate
  cards must then lower and hold lockstep, while cards carried by other
  unsupported features stay refused for their other reason.

## Decided together with Meld (brief's instruction)

Meld (CR 701.42) is NOT a stack: its result is a dedicated third face with
its own name/PT/abilities (`state.MeldResultFace`, `face_meld.go`), while a
mutated pile is an ordered component list with top-only characteristics. They
share the parked-object + cold-link representation, and nothing else; a
card that is both a meld half and a mutate card is refused at kgen
(the corpus has none). **No unification is proposed** — meld's one-record
region and mutate's pile region coexist; if a third merge mechanic arrives
it inherits the parked-object pattern, not a new abstraction now.

## Staging (tickets)

| # | Ticket | Delivers | Done means |
|---|---|---|---|
| 1 | `dirac-kernel-merge-pile` | `coldMerge` region + `genPrep.anyMerge`; `ExtMergePermanent` + kernel EventKind via `kmanifest assign`; merge fold incl. commander retarget; split on leave; `tm`; synthetic-table unit tests; `TestStateSize`/`TestColdArraysHoldNoPointers`/layout pins green | kernel unit tests pass; **kgen still refuses Mutate** (gate holds: nothing plays a merged card) |
| 2 | `dirac-kernel-mutate-cast` | `AkMutate` + `modeMutated`; `CardDef.MutateCost`; synthesized target + filter; `raMutatePlace` ask + replay flag; resolve diversion + CR 702.140b un-mutate; TrigMutates firing + matcher; `ExTimesMutated` kernel read | synthetic + focused pair tests; gorge unchanged |
| 3 | `dirac-kgen-mutate-support` | lower `K:Mutate` + `Mode$ Mutates`; update `lower_destalt_exile_test.go` per its comment; `KGEN_REPIN`/`DIRAC_UPDATE_SUPPORT` flips; support.md rows for the 33 sole cards; lockstep pairs + repro recordings (JSON + manifest row, never a Go test fn) on the split-pool focus files with `GORGE_POOLS` | **the ticket's Done means**: `make test-one PKG=./dirac/kgen RUN='^TestSupportDocIsCurrent$'` passes with this row's cards lowered; corpus acceptance green |

Risks: the ability-union scans touch the hot collector passes (keep them
count/at-style over the record, like gorge's `pile.go` comment insists);
`ObjHash` sees `perm.card` change on merge-over (walk caches must treat
merge as a card change — it already re-hashes per byte, so verify, don't
cache across it); the mutator-debug switch pattern (`DebugMeldKeep`) gives
ticket 2 a red-test lever for the split.
