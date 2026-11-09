# Activated abilities on emblems (and other command-zone Effects)

Date: 2026-11-09.
Status: **design only.** No engine code changes. Operator parked the ticket on
2026-10-27 as "1 Standard card"; this note is the design that ticket asked for.
Proposed tickets are at the end; none are filed.

## Problem

CR 114.4: abilities of emblems function in the command zone. gorge has no
command-zone activation path. Today `DB$ Effect | Name$ Emblem — … | Abilities$ X`
resolves, registers whatever `StaticAbilities$`/`Triggers$` it carries, and
drops `Abilities$` (the paramcensus lists it as `param:api:Effect.Abilities`,
unsupported; Karn, Living Legacy is the DMU row in
`engine/rules/testdata/paramcensus-sets/DMU.tsv`). With only `Abilities$`,
`effect.go` falls to the `!registered` Note ("registers a continuous effect …").
So Karn's −7 pays 7 loyalty and gives an emblem that does nothing.

## What the corpus actually asks for

`grep` over `.cards/cardsfolder` for `$ Effect` lines carrying `Abilities$`
(not `StaticAbilities$`) gives **12 lines in 12 cards**. They are not one
problem; there are four shapes:

| Shape | Cards | What the ability does |
|---|---|---|
| **E. Emblem with an activated ability** (CR 114) | Karn, Living Legacy (DMU) and its `rebalanced/a-` Arena copy; Ob Nixilis of the Black Oath; 1996 World Champion | `Cost$ tapXType<1/Artifact>` → 1 damage to any target; `1 B Sac<1/Creature>` → gain X, draw X; `Discard<1/Hand>` → put a named card from library on top (`ActivationPhases$ Upkeep`, `PlayerTurn$ True`, `IsPresent$ … PresentZone$ Library`) |
| **P. Pay-to-avoid** (not a printed activated ability; Forge's `ST$ ChangeZone`, `Origin$ Command`) | Quenchable Fire, Frogkin Kidnapper, Glass Asp, Nafs Asp, Sabertooth Cobra | "…unless that player pays {N} before that step": the payer is `Activator$ Player.IsRemembered`; paying moves the Effect to exile, which (by Forge's design) ends the Effect's one-off delayed trigger. Frogkin Kidnapper is a variant: the owner pays {3} to move a remembered exiled card to hand, with no delayed trigger |
| **I. Instant-speed Effect-held ability** | Guardian Angel | `AB$ PreventDamage`, `Cost$ 1`, `InstantSpeed$`, bound to a remembered permanent/player, `ForgetOnMoved$ Battlefield` ends it |
| **M. Mana / play grants** | Channel (`AB$ Mana`, `PayLife<1>`); Discord, Lord of Disharmony (`ST$ Play`, `Cost$ 0`, `ValidSA$ Spell.CouldCastTiming+NamedSpell`) | mana ability usable "any time you could activate a mana ability"; a may-cast grant |

Only shape E is a rules-level emblem. Karn is the only Standard card (per the
ticket); I did not check the legality of the others. P, I and M reuse
Forge's trick of parking an `Effect` card in the command zone; they are
worth designing for so E's data model does not have to be redone, but they are
**separate rollouts** (below).

## What already exists (verified in this tree)

- `state.ZCommand` is a real zone with real `Object`s. The dungeon is the
  precedent: `foldDungeonCreate` (`engine/events/apply_misc.go`) does
  `g.AddObject(def, p)` from a `g.Tokens[key]` card script and `Move`s it to
  `ZCommand`; `foldDungeonRemove` moves it to `ZCeased`. Commanders live there
  too.
- The Ring emblem is the opposite precedent: no object. `foldRingEmblemPush`
  mints a *stack ability* with `Source = 0`, rebuilt from event text alone so a
  log-only replay agrees (CR 701.54c). It carries triggers only; it cannot be
  activated.
- Every *other* Effect product (statics, delayed triggers, `Stackable$ False`
  dedup via `ContinuousNamed`) lives in the continuous/delayed registries, keyed
  by controller and the creating card's id (`DelayedRegister{Obj: c.Source}`),
  with emblem lifetime handled by "emblems are never `|SB` source-bounded"
  (`effect.go`, `isEmblemEffectName`).
- Activation is anchored by `decision.Option{Obj, Ability}` (printed face
  index via `o.PileAbilityAt`), or `{Obj, SVar}` for a *granted* ability
  (`beginGrantedActivation`), or `{GainedSource,…}`. `beginActivationPaid`
  resolves the source object's face and SVar table, so a source with a real
  `Face()` needs no new anchor kind.
- `ActivationZone$ Command` is parsed but unusable: there are **three** zone
  tables that agree it is not a zone — `abilityZoneMaskTab` and
  `abilityZoneOK`'s switch (`engine/rules/legal_activation.go`) and
  `activationZoneMaskTab` (`engine/effects/activation_params.go`). Mask 0 means
  "never offered" (CR 602.1b comment). `walk_face_facts.go` caches a per-face
  `abZones` mask from the same read, so the offer walks skip faces whose mask
  lacks the scanned zone.
- `dirac/` models emblems as table rows (`kop.Emblem`, `g.emb[p][i]`), not
  objects. `kop.Emblem` carries triggered or static abilities
  (`dirac/kop/statics.go`, `Emblem.Abilities`) but has no activated-ability
  slot and no option kind to activate one. A dirac "supported" card must play
  identically to gorge; Karn is not supported there today (not in
  `dirac/docs/support.md`; I did not run the support test), so a gorge-only
  feature changes no supported card.

## Options considered

### A. Emblem is an `Object` in the command zone (recommended)

`EmblemCreate` event mints an object owned and controlled by the player, in
`ZCommand`, whose `Card` is synthesised from the creating Effect: one face,
name from `Name$` (CR 114.3: no types, no colour, no cost), `Abilities` parsed
from the `Abilities$` SVar names, `SVars` shared with the creating face (so
`SubAbility$ DBDraw`, `X` resolve — Ob Nixilis needs both, and
`X:Sacrificed$CardPower`).

- Ability, cost, targeting, stack, resolution, replay, activation limits,
  `Activator$`, `Defined$ Self` ("this emblem deals 1 damage") all run through
  the existing object-anchored path. The damage source is the emblem itself,
  colourless with no types, which is what CR 114 says and what protection /
  prevention checks should see.
- Human clients get what they need for free: the option's `Obj` is in
  `view.Command`, so a UI can show the emblem and attach the "Activate" row to
  it. (An option whose `Obj` appears in no zone list would not render.)
- It generalises: Ring emblem triggers, `Stackable$ False` dedup, and shapes
  I/P can move onto the same object later.
- Cost: every `Zone(ZCommand, …)` reader now sees a non-commander object that
  is not a dungeon. I audited the readers in `engine/rules` and `engine/view`
  (table below, including the ones that only name, scan or classify); one is a
  real bug if not guarded, the rest are benign or need a one-line skip.

### B. Registry entry, no object (Karn registered like a continuous effect)

Add `Game.EmblemAbilities` rows `{Owner, HostObj, SVar}`; offer them with the
existing granted-ability anchor `{Obj: host, SVar}`.

- No `ZCommand` readers change.
- But the source is the **creating card**, which has usually died or left the
  battlefield (Karn's −7 can take him to 0 loyalty, and he dies to state-based
  actions). "This emblem deals damage" becomes damage
  from Karn: wrong colour/types for prevention and protection, and
  `Sacrificed$CardPower`/LKI plumbing keyed to a dead host. Per-source state
  (ActivationLimit counts are keyed by object id) is shared across two emblems
  from the same host. A human UI has no object to hang the option on.
  `Stackable$ False` plus per-seat duplicates need a synthetic identity
  anyway.
- Rejected: it saves the audit but reintroduces an identity problem the object
  model solves.

### C. Forge's model (every Effect is a command-zone card)

That is A made universal. Right end state, too big for one change: statics,
triggers and delayed registrations for hundreds of Effect cards are in the
continuous registries and their goldens. Do E first, keep the other products
where they are, and revisit C only if P/I/M want it.

**Recommendation: A, scoped to shape E first.**

## Design (A)

### Data

- `cards.Card` gets an `Emblem bool` (or `Face.Emblem`), set only on
  synthesised cards. This is the one flag every command-zone reader tests, so
  the "is this a commander?" question has one home. `cards` is the lowest layer,
  so the flag and an exported wrapper over `parseSA` (currently unexported,
  `engine/cards/parse.go:210`) belong there; `events` may then build the card in
  `Apply`.
- `events.Kind` append-only: add `EmblemCreate`. Fields (no `Event` struct
  change): `Player` owner, `Obj` creating source (the host whose face owns the
  SVar table; `svarTableOwner` rules decide which face, as for Animate),
  `Text` = `Name$` + `\x00` + comma-joined `Abilities$` names + `\x00` +
  the creating `Effect` SVar name (the `Stackable$` identity, below). `Apply` mints
  the object (`AddObject` + `Move` library→command, the dungeon sequence). The
  card is built inside `Apply`, from event text and the host's face, so
  a log-only replay reproduces it (the Ring emblem rule). Retirement, only when
  a card needs it (shape I), would be `EmblemRemove`, mirroring `DungeonRemove`
  (`ZCommand`→`ZCeased`). Karn's emblem is permanent: no removal event.
- `Stackable$ False` (Karn, Ob Nixilis, 1996 World Champion all set it):
  `effect.go` dedups through `ContinuousNamed`, a continuous-registry query. An
  emblem that registers no continuous row (Karn's only product is the object)
  would never be found there, so the dedup needs a new question, asked in one
  place. It must **not** key on the name: CR 114.3 says most emblems have no
  name, and a name key would make two different nameless emblems from one
  player suppress each other. Key it on the creating effect's identity — the
  host card's face plus the creating `Effect` SVar (both already in the
  `EmblemCreate` event: `Obj` and the SVar name, stored on the minted object)
  — so "does this player already own an emblem object minted by this same
  Effect of this same card?" is the test. `Name$` is display only. (Shape E's
  four cards all carry `Name$`, so a name key would happen to work for them,
  but the object design is meant to generalise.) The event `Text` therefore
  carries the creating SVar name as a third `\x00`-separated field.

### Activation

1. Add `Command` to the zone tables. This is also the moment to **collapse the
   three tables to one** (the AGENTS.md rule "dispatch on a word: the existing
   StrCodes table"): `abilityZoneMask`, `abilityZoneOK` and
   `activationZoneMask` should read one table; the new bit is `1 << ZCommand`.
2. There is **no reusable per-ability offer helper today.** The whole gate
   sequence for a printed activated ability (zone mask, `Activator$` via
   `activatorAllows`, phases/`PlayerTurn$`, loyalty, `ActivationLimit$`,
   cost feasibility via `offerCastable`/`autoPayOffer`, targets, `CheckSVar$`,
   `IsPresent$`, Adapt, Monstrosity, …) is inline in `battlefieldWalk`
   (`engine/rules/legal_walk_battlefield.go:158`, a ~750-line function; the
   printed-ability block is roughly lines 250-500). Only the cost feasibility
   checks are extracted. Ticket 2 therefore **first extracts that printed-ability
   gate sequence into a named helper inside `legal_walk_battlefield.go`**
   (AGENTS.md: "a branch in a 300+ line func → a named helper"), a pure
   move with no behaviour change, proven by the existing walk tests and
   `TestHeads` not moving. That is a deliberate, minimal hot-file edit; the
   alternative (re-implementing ~150 lines of gates in a new file) is a pasted
   block and is rejected. Then a new file
   `engine/rules/legal_walk_command.go` holds `commandAbilitiesWalk`, which
   enumerates `Zone(ZCommand, p)` objects with `Card.Emblem`, for abilities
   whose mask has the command bit, and calls that helper. It is wired into the
   walk dispatch in `engine/rules/legal.go` (the `w.commandZoneWalk()` …
   `w.battlefieldWalk()` sequence, ~line 179), next to `w.commandZoneWalk()`.
   Everything battlefield-specific must be audited when reusing the helper: summoning
   sickness is irrelevant (the emblem has no `{T}`); `tapXType` is tap *other*
   permanents (Karn, CR 302.6 does not apply to them); `Sac<1/Creature>` and
   `Discard<1/Hand>` are ordinary payments; `ActivationPhases$`/`PlayerTurn$`
   are already in `compileActivation`.
3. `walk_face_facts` `abZones` automatically includes the command bit once
   the face's mask does, so the quiet-seat skip (`quiet_facts.go`) keeps
   working; a command-zone emblem face must report "has command-zone
   abilities" so the skip does not hide the offer.
4. `beginActivation` needs no new anchor: `opt.Obj` is the emblem object and
   `opt.Ability` indexes its single face.
5. Resolution: `AB$ DealDamage` with the emblem as damage source. Confirm the
   damage path accepts a non-permanent source in `ZCommand` the way it accepts
   one on the stack (Gut Shot); that is the one effect-side unknown.
6. Mana: Karn's cost has no mana, Ob Nixilis's is `1 B`; mana payment is
   unchanged. Channel (shape M) is *not* covered: it is a mana ability, and
   mana abilities are enumerated by `mana_activation.go` and the autopay planner
   (I did not trace whether either looks at the command zone; `rules/mana.go`'s
   only command-zone logic is commander tax). That is a planner change and out
   of scope here.

### `Zone(ZCommand)` reader audit (what the emblem object touches)

| Site | Risk | Required |
|---|---|---|
| `legal_walk_alt.go:293` command-zone cast loop | **Bug if unguarded**: in a Commander game an emblem face with no types is iterated as a castable commander (`f.SpellAbility()` nil → `castTargetsAvailable`, `RawBaseCost`) | skip `Card.Emblem` |
| `window_diagnostics.go:353` `classifyCommandZone` | would record a "withheld cast" reason for an emblem | skip `Card.Emblem` |
| `repl_zoneskip.go:289,332` replacement visit, `untap.go:20` | iterate objects and read their faces' replacements/statics; an emblem face carries none | none, verify with a test; the dungeon already exercises this |
| `quiet_proof.go:688` | `len(Zone(ZCommand,p)) > 0` turns the quiet fast-path off for the whole game | accept for emblem games, or refine to "has activatable command abilities"; measure before refining |
| `view/view.go:665` | the emblem appears in the seat's `Command` list | check `describe.go` can name a face with no types; view test; no wire type change |
| `count_zone_fold.go`, `filter_word.go` `Zone$ Command` counts | "cards in command zone" counts would include the emblem | count `Card.Emblem` out (CR 114.5: neither a card nor a permanent) |
| triggers: `trigger_zoneskip.go` (`ZCommand` slot) | emblem triggers (future) | out of scope for E |
| `oracle_snapshot.go:302` `Command: r.zoneNames(ZCommand…)` | the emblem's name appears in the oracle snapshot's `Command` list; reachable if an emblem game is replayed through the oracle harness | none for gorge (it is a real command-zone object, CR 114.1); the oracle/dirac comparison must expect it, which ticket 4 owns |
| `prio_memo.go:266` priority-memo sensitivity scan | iterates command-zone objects through `prioObjSensitive`; an emblem with activated abilities may mark the memo sensitive | benign: that only costs memo hits; verify the emblem's activation is not memoised away (ticket 2 test) |
| `statics_costscan.go:81` cost-static scan | reads statics of command-zone objects; an emblem face has none | benign |
| `potential_plan.go:1330` origin label | labels a plan's origin `command_zone`; only reached for a cast/activation option from `ZCommand` | benign: correct label for an emblem activation |
| `replacement_match.go:56` `commandReplZoneAdmits` | only consulted for replacements whose source is in `ZCommand`; an emblem face carries no replacements | benign |
| `mana.go:52` commander tax | returns tax only for `Player.Commanders` ids; an emblem is never one | benign |

### Determinism, ratchets, dirac

- Every field is derived from event text and the host face's SVar table:
  no map ranges reach events; `Abilities$` names are comma-split in written
  order.
- Ratchets are already per card, so there is no sequencing constraint here.
  `knownUnsupportedParams` is `map[string][]string` loaded one JSON file per
  card (`loadPerCardLabels`, `engine/rules/paramcensus_test.go`; the
  `testdata/paramcensus/` directory is absent in this tree), and the set
  baseline is one `<card>\t<labels>` row per card. The only live
  `param:api:Effect.Abilities` entry is Karn's row,
  `engine/rules/testdata/paramcensus-sets/DMU.tsv:1`
  (`param:api:ChooseNumber.ChooseAnyNumber,param:api:Effect.Abilities`);
  A-Karn, Ob Nixilis, 1996 World Champion and the P/I/M cards have no baseline
  row. The ticket that makes the emblem real deletes exactly
  `param:api:Effect.Abilities` from Karn's row and leaves
  `ChooseNumber.ChooseAnyNumber`. No ratchet refactor, no residual entry, and
  the P/I/M cards are unaffected whenever shape E lands.
- Karn's −1 needs `ChooseNumber.ChooseAnyNumber`, the other unsupported param
  in the DMU census row. It is independent of the emblem; the emblem work makes
  the −7 work, not the whole card.
- dirac: Karn stays unsupported there. A dirac port is a separate ticket:
  `kop.Emblem` gains an activated-ability slot plus an option kind, and the
  lockstep evidence games would need a Karn/Ob Nixilis deck. Do not start it
  until the gorge side lands and the `Event` kind is fixed.

## Shapes P, I, M (later)

- **P (pay-to-avoid, 5 cards).** These need a *non-emblem* Effect object that
  carries `Remembered` (the object already has `Remembered`; the Ring event
  sets it) with `Activator$ Player.IsRemembered` resolved against it, and whose
  departure cancels the one-off delayed trigger. The alternative —
  resolving the trigger with an `UnlessCost` ask — is simpler but a rules
  approximation (the payer would have the untapped mana of the *trigger's*
  upkeep, not of "before that step") and AGENTS.md's Known-approximations table
  is delete-only, so a new approximation is not an option without the operator.
  So P waits for A, then adds `EmblemRemove`/`EffectRemove` and a trigger
  liveness link. (The ruling that paying is allowed at any time before the step
  is from memory; pin it against the actual Gatherer rulings in that ticket.)
- **I (Guardian Angel).** Same object plus a lifetime ending on the remembered
  permanent leaving (`ForgetOnMoved$ Battlefield`, existing `|FM=` machinery)
  and `PreventDamage` shield registered per activation.
- **M (Channel, Discord).** Not an emblem problem. Channel is a mana-planner
  change; Discord is a may-play grant (the Effect-delivered grant registry
  already exists, `ContinuousEffect.Chosen`) with a `ValidSA$` filter. Handle
  them under their own designs.

## Proposed tickets (not filed)

1. **cards/events/state: emblem object + `EmblemCreate`.** `Card.Emblem`, an
   exported SA parser, event Kind appended, fold builds the object; unit tests
   for fold + replay equality + clone; reader guards in the audit table
   (`legal_walk_alt.go`, `window_diagnostics.go`, command-zone counts). No
   card behaviour change yet. Test: a Commander-format game with an emblem
   object in the command zone offers no "cast" row for it (fails without the
   `legal_walk_alt.go` guard).
2. **rules: `ActivationZone$ Command` offer path.** One zone table; the new
   `legal_walk_command.go`; Karn's `KarnPing` offered, paid (tap an untapped
   artifact), targeted (any target), resolved (1 damage, emblem as source), and
   not offered with no untapped artifact / not offered from the opponent's
   seat. Ob Nixilis (`Sacrificed$CardPower`) and A-Karn follow for free.
   Depends on 1. Includes the gate-sequence extraction from `battlefieldWalk`
   described under Activation step 2 (a pure move, first commit of the ticket)
   and the dispatch line in `legal.go`.
3. **effects: `Effect` resolution mints the emblem for `Name$ Emblem…` +
   `Abilities$`.** Stackable dedup (keyed on the creating effect, not the
   name), A-Karn, hook-up of ticket 2. Deletes `param:api:Effect.Abilities`
   from Karn's `DMU.tsv` row (the only ratchet change). Could merge into 2.
4. **dirac: activated emblem ability** (`kop.Emblem` slot + option), after 1–3
   land. Independent.
5. **Shape P** (5 cards) design-then-ticket, after 1–3.
6. **Shape I** (Guardian Angel), after P (reuses the removal event).
7. **Channel** (mana-planner command-zone abilities) and **Discord** (may-play
   grant): independent designs.

Tickets 1–3 touch `cards`, `events`, `rules`, `effects` and `view`; treat each
as high complexity. The risk is concentrated in ticket 1's reader guards, which
is why they are a table above.

## Open questions for the operator

- Is shape P in scope (5 cards)? If not, the object design still holds and
  shape E leaves the ratchet on its own; the P cards simply stay unsupported.
- Is a quiet-fast-path regression (any game with an emblem leaves it) acceptable?
  A game with a dungeon already has a non-empty command zone, so this only
  extends an existing cost to emblem games.
