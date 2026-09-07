# Keyword use, not just deck presence (FL-110)

```sh
go run ./cmd/keywordbench -corpus -games 10 -seed 110000
go run ./cmd/keywordbench -decks mono-red-prowess,mono-white-equipment -games 10 -seed 110000
```

`-games` is **per deck**. Each deck plays two-seat mirror matches, with one
`seat.NewBot(seed)` shared by the seats as in mtgsim. Every deck uses seeds
`seed` through `seed+games-1`; adding a deck does not change another's seeds.
Commander files use Commander, command zones and 40 life; constructed files use
20 life. No mulligans. Matches cap at 200 turns / 40,000 intents. Completed
games must replay; stalls are named, retain explicitly partial counts, and
make the command fail. `-games 0` measures presence/corpus only.

## What the columns prove

- `distinct_deck_cards`: distinct card names, not copies, deduplicated across
  decks in TOTAL; printed keywords on any face. Regeneration is separately
  `api:Regenerate`, not a Forge keyword.
- `battlefield_objects`: unique object+metric pairs seen on a printed face on
  the battlefield, per game, across both seats. Reentry does not inflate it.
  Not exposure time; excludes tokens and continuously granted keywords.
- `offered` / `selected`: option occurrences at live decisions, not unique
  opportunities. Only `ability` and special-mode `cast` options are counted;
  **not** optional-trigger/Miracle asks. One offer can recur many times.
- `activated`: actual `AbilityPush` events, attributed to that ability's
  `Keyword` tag (or a Regenerate API in its Sub chain), **not every keyword on
  its source**. `triggered`: actual `TriggerPush`, attributed to its trigger's
  keyword tag. This counts stack pushes, not resolutions, successful effects,
  prevented deaths, or good decisions. Optional triggers declined before the
  push are not counted. Regeneration counts an activation of an ability
  carrying that API, not proof that a regeneration shield saved anything.
- `special_cast`: `CastInfo` flags for Kicker, Surge, Flashback and Miracle.
  These are casts, **not** activations/triggers.
- Unselected-option comments show overlapping observed contexts and which
  option kind was chosen instead. `no_printed_creature` is not a Derived()
  census and must not be treated as a policy score or a causal verdict.

A clone of post-genesis state consumes the actual game's event log through
`events.Apply`; attribution reads the face **at event time**, before applying
the next event, rather than a possibly transformed final face. The collector
never changes the played game or policy.

**Zero is not a coverage verdict.** Flying, Trample, protection, etc. do not
activate or trigger. Enchant casts, ETB replacements, Delve payments and Flash
timing are not instrumented here. Their zero event columns mean **not measured**.
This tool is not a general keyword-fidelity or tactical-benefit oracle.

## Corpus evidence and selection

Measured at engine base `bb61a7b`, `.cards/cards.lock` commit
`95f04e8a04c8925fa97cb226fc3341cabcc90a53`. This checkout's lock and compiled
registry contain **33,669 cards**, not the stale 33,667 in AGENTS.md. Cache
SHA-256: `c8066fb00936ff84fee81b0529ac465bbaecf9ec12e9cd3aa92296a3e72b138d`.

The command uses **`/usr/bin/grep -rhI '^K:' .cards/cardsfolder`**: 18,243 raw
K lines, including repeats and alternate faces. The compiled walker counts
face keyword occurrences and distinct registry cards, excluding token scripts
and dynamic grants/SVars in both populations. Here the raw line and compiled
occurrence counts agree; distinct-card reach differs. Rank is compiled reach
multiplied by registered implementation (0/1), then reach and name. Registration
is not full semantic fidelity. Selected high-reach rows and all targeted rows:

| Keyword | Raw K lines (`/usr/bin/grep`) | Compiled K occurrences | Compiled cards | Registered |
|---|---:|---:|---:|---|
| Flying | 3299 | 3299 | 3274 | yes |
| Enchant | 1264 | 1264 | 1263 | yes |
| Trample | 1036 | 1036 | 1024 | yes |
| Vigilance | 747 | 747 | 743 | yes |
| Haste | 682 | 682 | 669 | yes |
| Flash | 628 | 628 | 627 | yes |
| Equip | 646 | 646 | 622 | yes |
| etbCounter | 475 | 475 | 475 | yes |
| Reach | 443 | 443 | 435 | yes |
| ETBReplacement | 405 | 405 | 399 | yes |
| First Strike | 392 | 392 | 383 | yes |
| Lifelink | 387 | 387 | 382 | yes |
| Deathtouch | 347 | 347 | 340 | yes |
| Kicker | 239 | 239 | 239 | yes |
| Flashback | 212 | 212 | 212 | yes |
| Prowess | 94 | 94 | 90 | yes |
| Living Weapon | 19 | 19 | 19 | yes |
| Menace | 414 | 414 | 408 | no |
| Defender | 309 | 309 | 309 | no |
| Cycling | 306 | 306 | 306 | no |
| Chapter | 236 | 236 | 236 | no |
| Ward | 215 | 215 | 208 | no |
| Crew | 192 | 192 | 192 | no |
| Double Strike | 129 | 129 | 124 | no |
| Hexproof | 110 | 110 | 94 | no |

Equip, Kicker and Flashback are the highest-reach registered keywords with
explicit activation / alternate-payment instrumentation here. Prowess supports
a real spells archetype; Living Weapon fits equipment and had only one baseline
trigger. Evergreen static/timing/replacement mechanics above them are deliberately
not advertised as covered by this activation counter. Storm, Evolve, Exalted
and Undying already fire in the baseline; no additional deck solely for them.
Miracle/Surge have zero measured special casts and remain follow-ups rather than
an excuse for a keyword pile. Cycling, Ward, vehicles, Bogles, etc. need engine
work first. Rejected candidates include Soul-Scar Mage (`repl:DamageDone`),
Thraben Inspector (`api:Investigate`), Gladecover Scout (`kw:Hexproof`), and
Hyena Umbra (`kw:Umbra armor`); none was added to the ratchet.

## Baseline: the original 17 decks

Reproduce after adding files by passing this fixed pool:

```sh
go run ./cmd/keywordbench -corpus -games 10 -seed 110000 -decks death-n-taxes,dimir-tempo,eldrazi-stompy,foundations-calling-all-angels,foundations-keen-engineering,foundations-reign-of-dragons,foundations-tramplesaurus-rex,foundations-wretched-ranks,mono-black-aggro,mono-blue-tempo,mono-green-stompy,mono-red-goblins,the-epic-storm,tron,ur-delver,uw-control,uw-tempo
```

**170 completed, replay-verified games**, seeds 110000–110009 per deck, 340
seat-games. Presence is over the original **423 distinct deck cards**, not
170 samples of the deck list. A = activation pushes, T = trigger pushes,
C = special casts; `—` = not an activation/trigger/cast measured by this tool.

| Keyword/API | Distinct in-deck cards | Measured event use |
|---|---:|---:|
| Regenerate (API) | 1 | 14 A |
| Deathtouch | 4 | — |
| Delve | 3 | — |
| Devoid | 1 | — |
| ETBReplacement | 6 | — |
| Enchant | 1 | — |
| Equip | 6 | 21 A |
| Evolve | 1 | 37 T |
| Exalted | 3 | 17 T |
| First Strike | 4 | — |
| Flash | 3 | — |
| Flashback | 5 | 19 C |
| Flying | 63 | — |
| Haste | 10 | — |
| Indestructible | 2 | — |
| Kicker | 5 | 25 C |
| Lifelink | 5 | — |
| Living Weapon | 1 | 1 T |
| Miracle | 3 | 0 C |
| Protection from black | 0 | — |
| Protection from blue | 1 | — |
| Protection from green | 0 | — |
| Protection from red | 0 | — |
| Protection from white | 1 | — |
| Prowess | 1 | 87 T |
| Reach | 1 | — |
| Storm | 2 | 22 T |
| Surge | 1 | 0 C |
| Trample | 7 | — |
| Undying | 3 | 32 T |
| Vigilance | 7 | — |
| etbCounter | 7 | — |

**Surprise:** Experiment One is not inert in these games: 22 battlefield objects,
15 regeneration option occurrences, 14 selected and 14 actual activation
pushes, all from mono-green-stompy. The remaining offer lost to a cast. This
does not establish whether any shield prevented destruction, and does not
contradict a different golden seed never activating it.

## Added archetypes and actual use

`mono-red-prowess.json`: cheap spell-growing threats, burn, looting and reusable
flashback spells; Burst Lightning is a late-game kicker outlet. Not a tier-list
claim. `mono-white-equipment.json`: equipment tutors, cost reduction, equipped
creature payoffs, cheap equipment and Living Weapon bearers with removal and
protection. Both are coherent casual 60-card constructed lists.

**20 additional completed replay-verified games**, 10 per new deck at the same
seeds; 40 seat-games. These are new-deck-only counts, not the baseline plus new:

| New deck / keyword | Distinct deck cards | Battlefield objects | Offered | Selected | Actual event use |
|---|---:|---:|---:|---:|---:|
| equipment / Equip | 4 | 103 | 1107 | 16 | 16 A |
| equipment / Living Weapon | 1 | 33 | n/a | n/a | 33 T |
| prowess / Prowess | 2 | 45 | n/a | n/a | 33 T |
| prowess / Flashback | 4 | n/a | 22 | 22 | 22 C |
| prowess / Kicker | 1 | n/a | 0 | 0 | 0 C |

Prowess was already active (87 baseline triggers); this adds a second printed
Prowess card and another spells context, not a claim to have activated an inert
mechanic. Living Weapon's 33 pushes versus one baseline push is the clearest
sampling improvement. No unsupported distinct cards: **0 of 436** after adding
the decks. `legacyDeckNames`, the ratchet and golden heads are untouched;
`TestHeads` passes. The new lists remain explicit bench choices, not automatic
members of the frozen acceptance or default `make sim` rotation.

## Evidence for the next policy task (no policy edits here)

- **Equip works, but gets low priority.** Of 1091 unselected option occurrences
  on the new deck, the selected kind was cast 710, mana activation 319, land 36,
  another ability 23, pass 3. Overlapping contexts: source already attached
  106, no printed own creature 153, outside main 0. These are not 1091 missed
  tactical opportunities. `botpolicy/policy.go:Decide` prioritizes mana/lands/
  casts before `chooseAbility`; `botpolicy/ability.go:equipNoOp` deliberately
  refuses re-equips and creatureless boards. A follow-up needs per-ability
  value/target/cost facts before changing that order, not simply activating more.
- **Living Weapon and Prowess really trigger** (33 each); these are automatic
  engine triggers, not evidence of learned spell timing. Prowess-aware sequencing
  belongs in `botpolicy/cast.go:chooseCast` and `policy.go:Decide`; trigger
  ordering is `botpolicy/trigger.go:chooseTriggerOrder`.
- **Flashback really casts**: all 22 offered occurrences were selected. No
  refusal was observed. This is not proof that all four flashback cards were
  used, or that sacrificing a land for a flashback was beneficial. The mana
  planner still prices printed cost rather than Flashback cost:
  `botpolicy/tap.go:poolPays`, `tapWants`, `bestUnpayable`.
- **Kicker remains inert in the new prowess games:** zero kicked options and
  zero kicked `CastInfo` events, not a claim of policy rejecting an offer.
  `rules/legal.go:legalActions` requires floating mana for base+kicker;
  `botpolicy/tap.go:poolPays` only budgets printed cost. Burst Lightning needs
  five mana kicked in a low-curve deck. A mode-aware mana plan is the next task;
  changing only `chooseCast` cannot choose an option that was never offered.
  Baseline Kicker's 25 casts proves this is context-dependent, not globally
  unimplemented. Keep this failed target visible rather than claim coverage.
- **Regeneration needs tactical evidence next**, not a blind activation mandate:
  `policy.go:Decide` gates abilities on main phase, and `ability.go:abilityScore`
  has no impending-destruction/shield facts. The 14 measured activations alone
  cannot establish benefit or responsiveness. No botpolicy files or frozen
  `LegacyDecide` were changed.
