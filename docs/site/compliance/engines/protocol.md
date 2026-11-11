# The follower protocol

This is the contract between the harness and an engine's runner: the command line, the card check,
the request and result schemas, the snapshot vocabulary, and how two snapshots are compared. It is
written from the code in [gorge-compliance](https://github.com/adams-shaun/gorge-compliance) and
gorge as of 2026-11-11. Where this page and the code differ, the code is right; please tell us.

The normative sources, for when you need the last detail:

| Topic | Source |
|---|---|
| The card check and the `neutral` object | `cmd/oraclediff/follower_export.go`, file comment |
| Scenario and step types | `oracle/compliance/oraclegen/gen.go` |
| gorge's runner (the authoritative decoder) | gorge `engine/rules/oracle_run.go` |
| Decision records and snapshots | gorge `engine/rules/oracle_snapshot.go` |
| The comparator | `oracle/compliance/oraclediff/compare.go` |
| The classifier | `oracle/xengine/classify/classify.go` |
| A complete follower | the Forge driver, `forge-oracle/` in [our Forge fork](https://github.com/adams-shaun/forge) |

## The runner

A follower is one program, run in two ways:

```text
RUNNER IN.jsonl OUT.jsonl        replay every request in IN, write one result row per line to OUT
RUNNER --preflight NAMES.txt     print the cards the engine lacks
```

The rules for a replay run:

- **One row per request line, in input order.** Flush after each row, so a crash keeps the rows
  already written.
- **One bad line never stops the batch.** A line you can't parse, a panic, an exception or a timeout
  becomes that line's `harness` row, and you go on to the next line.
- **Exit 0 when the batch ran,** whatever the rows say. Exit non-zero (the Forge runner uses 2) only
  when the runner itself couldn't start: a missing build, a bad card database, a pin mismatch. A
  non-zero exit fails the whole chunk.
- **Be deterministic.** Seed any randomness from the request (Forge seeds from a CRC32 of the item
  id). A rerun should give the same rows apart from `ms`.
- **Each line is independent.** Build a fresh game for every request. Nothing carries over between
  lines.

The sweep calls the runner as `$XENGINE_<NAME>_RUNNER` if set, else the registry's `Runner`. It runs
under `scripts/heavy-lock.sh` with `--mem $XENGINE_<NAME>_MEM` (default 12G). A runner that starts a
JVM also waits for one of the box's JVM slots; set `JVM: true` in the registry for that.

## The card check (preflight)

`oraclediff follower-export` asks the runner once per run, before it writes anything:

```text
RUNNER --preflight NAMES.txt
```

**NAMES.txt** holds one card name per line, UTF-8, sorted, with no duplicates. It lists every card any
item of the run uses: each seat's `battlefield`, `tapped`, `back_face`, `hand`, `graveyard`, `exile`,
`library`, `library_top`, `counters` and `attached` cards, and the card refs in the steps (`card`,
`targets`, `attackers`, `blocks`, `attached_to`) with the `pN:` owner and `#k` ordinal stripped.
Tokens and player refs aren't cards and are never asked about. Names are in gorge's spelling, which is
Forge's (see [card names](#card-names)).

**The answer** is one line on stdout per card the engine *lacks*:

```text
NAME<TAB>REASON
```

NAME is exactly as NAMES.txt gave it. REASON is free text with no tab or newline, such as
`not in card-data` or `unsupported keyword Foretell`. A supported card gets no line. Blank lines are
ignored, and stderr is passed through.

**Exit 0 means the answer is complete.** Any other status fails the whole export, and so does a line
without a tab or a NAME that wasn't asked about. A partial answer would send items the engine can't
place, so it is never accepted.

**Refusals.** An item that uses a refused card is not sent. It is written to
`<dir of -out>/<NAME>-prep.jsonl` instead:

```json
{"id":"Burst Lightning/cast-resolve/v1","reason":"toy_lacks_card","detail":"Burst Lightning: no card data"}
```

`xengine diff` reads that file and gives those items the class `not_sent`. The file is rewritten on
every run, even when empty.

Refuse generously. A refused card costs you a few items. A card your engine "supports" but gets wrong
in a way you already know about costs a person an afternoon reading a false `diverge`. If your engine
can tell that a card parsed with a gap, or falls back to a default for something it doesn't
understand, refuse it here.

## Requests

Each line of the request file is one JSON object:

```json
{"id": "...", "scenario_sha": "...", "item": {...}, "gorge_decisions": [...], "abilities": {...}, "neutral": {...}}
```

| Field | Meaning |
|---|---|
| `id` | The item id. Echo it in your row |
| `scenario_sha` | A hash of the item before any rewrite. The same item has the same sha for every follower |
| `item` | The scenario: setup, steps and payment. See below |
| `gorge_decisions` | Every choice gorge made while replaying the item. You answer your engine's prompts from these |
| `abilities` | For each step with an `ability_index`, the Forge script line of that ability. Useful if your engine reads Forge scripts; otherwise use `neutral` |
| `neutral` | Engine-neutral identities of the abilities, tokens and modes the item names. See [neutral](#neutral) |

### Kinds of item

All items use one schema. They differ in size and in where they start:

| Kind | id | What it is |
|---|---|---|
| Single-card scenario | `<Card>/<template>/v1` | From `oraclediff gen`: a small board, 0 to 4 steps. Templates: `cast-resolve`, `play-land`, `activate…`, `combat`, `trigger`, `static`, `static.cost`, `counter-spell` |
| Window | `xe:<SET>:<chunk>:<game>:w<turn>` | One turn of a gorge game. The position at that turn's first main-phase priority is rebased to turn 1, with the active player relabelled `p0`. The steps are that turn's actions |
| Tape | `xe:<SET>:<chunk>:<game>:tape` | A whole game from turn 1: each seat's opening hand and entire library, then every action until the first thing the harness can't express. Most tapes end at the first shuffle |
| Tape prefix | `xe:…:tape:t<N>` | A tape cut at the end of turn N (`xengine play -tape-turns`) |

### The item

| Field | Read it? | Meaning |
|---|---|---|
| `id`, `name` | Echo | Your row repeats both |
| `card`, `template` | Optional | The card under test and the generator template; empty for windows and tapes |
| `turn` | Yes | Absent or 1 for every generated item. A hand-written scenario may set it higher |
| `players` | Yes | Absent means two seats. Generated items are always 1v1 |
| `setup` | Yes | `{"p0": Seat, "p1": Seat}`, below |
| `steps` | Yes | The actions, below |
| `payment` | Yes | Parallel to `steps`: the exact pool mana gorge spent on step i, as WUBRGC letters (a multiset). `""` means none recorded |
| `ignore` | No | Fields the comparator skips, such as `library_top` after a shuffle. You don't act on it |
| `compare` | No | Extra fields the comparator checks. Generated items carry none |
| `setup_answers`, step `answers`, `expect`, `observe` | No | Inputs to gorge's own runner. Use `gorge_decisions` instead |
| `xmage_*` | No | XMage driver hints |
| `cr`, `why` | No | Prose |

### Setup

Each seat:

| Key | Type | Meaning |
|---|---|---|
| `battlefield` | names | Placed directly onto the battlefield. **No ETB triggers, no entry replacements** (enters tapped, enters with counters), **no summoning sickness** |
| `tapped` | names | Tap every battlefield placement of each name |
| `tapped_after_untap` | refs | Tap that one object after turn 1's untap step, at the first priority, with no trigger. Refs are seat-relative: `"Forest"`, `"Grizzly Bears#2"`, `"token:Soldier"` |
| `back_face` | names | Battlefield cards placed on their back face. Each also appears in `battlefield` |
| `hand` | names | Placed directly |
| `graveyard` | names | Placed in this order, oldest first |
| `exile` | names | Placed directly |
| `library_top` | names | The top of the library, first is top, in this exact order. Windows and tapes list the **whole** library here |
| `library` | names | Library cards in no particular order. gorge shuffles them into its deal, so an item that names `library` ignores library order |
| `counters` | `{name: {KIND: n}}` | Counters **added** to what the card already enters with (its loyalty, say), on every placement of the name. Kinds in gorge spelling: `P1P1`, `M1M1`, `LOYALTY`, `CHARGE`, `LORE`, … |
| `tokens` | `[{script, counters}]` | Tokens minted after the seat's cards, in order, silently: no create or ETB trigger. `script` is a Forge tokenscript stem such as `r_1_1_goblin`; `neutral.token[stem]` gives its characteristics. `counters` is the token's complete counter set |
| `attached` | `[{card, to}]` | Auras placed already attached, after every seat and token is placed, with no trigger. `card` is seat-relative; `to` is a full ref |
| `life` | int | Starting life. Default 20 |
| `speed` | 0–4 | CR 702.179 speed |

`command`, `sideboard` and `mana` exist in the type but aren't used by generated items; refuse them if
you see them.

**Library filler.** When a seat names fewer than 40 cards, the rest of its library is `Wastes`:
`max(0, 40 - named)` of them, placed under `library_top` and above any `library` cards. Windows and
tapes name all 40 cards, so they have no filler.

**The starting point.** The game starts at turn 1 with `p0` active. `p0` skips its turn-1 draw. The
`"setup"` checkpoint is taken at the first priority in `p0`'s first main phase, even if the stack is not
empty then. Setup must leave no "this turn" history: no spells cast, no lands played, no cards entered
this turn. No mulligans and no starting-player roll happen.

### Refs and object identity

Steps and decisions name objects by ref:

| Ref | Means |
|---|---|
| `pN` | Seat N, the player |
| `pN:Name` | The first card named Name that seat N **owns** |
| `pN:Name#k` | The k-th such card (k from 2) |
| `pN:token:Sub#k` | The k-th battlefield token **controlled** by seat N whose name contains Sub (case-insensitive) |

**Create cards in gorge's order**, because a ref's `#k` is creation order. Per seat: `battlefield`,
`hand`, `graveyard`, `library`, `exile`, `library_top`, the Wastes filler, then `tokens`. Each placement
binds the next ordinal of its name.

A bound ref follows its card through every zone. A ref to an object created during play (a token, a
copy) means the k-th live object, in creation order, owned by seat N with that name. A name matches
the card's current name or any of its face names.

### Steps

| Op | Fields | What to do | Checkpoint |
|---|---|---|---|
| `play` | `seat`, `card` | Play the land | When the land is on the battlefield |
| `cast` | `seat`, `card`, `mana`?, `targets`?, `cast_mode`? | Add `mana` (WUBRGC letters) to the pool, then cast the spell, paying **from the pool only** | When priority returns to the caster with the spell on the stack |
| `activate` | `seat`, `card`, `ability_index`, `mana`?, `targets`? | As `cast`, for an activated ability. A mana ability completes at once | After activation; for a mana ability, immediately |
| `pass` | `seat` | The named seat passes priority **exactly once**. If another seat holds priority, that's a harness error | At the next priority, whoever holds it |
| `pass_to` | `seat`, `decision`? or `step` + `active`? | Pass until the named decision (`priority`, `attackers` or `blockers`) is pending, or, after at least one pass, until the named step with the named active seat. `decision: "priority"` completes at once with no pass | When reached |
| `attack` | `seat`, `attackers`, `defender` | Declare those attackers, all at the one player `defender` (`"pN"`) | At the first priority after the declaration |
| `block` | `seat`, `blocks` | Declare blocks, given as `[blocker, attacker]` pairs | At the first priority after |
| `resolve` | `seat` | Pass until the stack is empty and the active player holds priority. Never stop at a priority in the cleanup step | When reached |
| `attach` | `card`, `attached_to` | Attach directly, with no trigger. Windows use it to place Equipment before their first step | Immediately |
| `move` | `card`, `to` | Move the card directly to `library`, `hand`, `battlefield`, `graveyard` or `exile`, then let one empty round of priority pass so state-based actions and triggers happen | After that round |
| `mana` | `seat`, `mana` | Add the letters to the pool | Immediately |

gorge's runner also knows `life`, which no generated item uses.

The checkpoint after step `i` is named `"step i (op)"`, for example `"step 2 (cast)"`. You may label a
step with a different op (see [rewrites](#forge-shaped-rewrites)): the comparator pairs checkpoints by
index.

**`cast_mode`** says how a spell is cast. `""` is the ordinary way. Others name an alternative or
optional cost: `kicked`, `kicked1`, `kicked2`, `kickedboth`, `multikicked`, `buyback`, `entwined`,
`offspring`, `replicated`, `optionalcost`, `plot`, `adventure_alt`, `omen_alt`, `modal_spell`,
`warped`, `flashback`, and more as gorge supports them. If your driver can't act on a mode, list the
ones it can in the registry's `CastModes`. A cast with any other mode then becomes `harness` instead of
a false `diverge`.

**Mana in windows and tapes.** gorge's players pay their own costs, and the lowering turns every mana
ability gorge used into its own `activate` step with an `ability_index`, placed before the cast it pays
for. So your driver must activate mana abilities as standalone steps that complete immediately, and
then pay the cast from the floating pool. A producer of any colour gets a colour answer in
`gorge_decisions`.

**`payment[i]`** is the exact multiset of pool mana gorge spent on step `i`. Spend those units first.
With two or more colours in the pool, a different split is a real difference, but only if the registry
says your driver uses `payment` (`ConsumesPayment: true`). Otherwise such a difference is classed
`payment_choice`, not `diverge`.

#### Forge-shaped rewrites

Requests from `follower-export` carry three rewrites made for Forge, which asks for a combat
declaration before the step's first priority:

- An `attack` or `block` step that declares nothing becomes `pass_to` with `decision: "priority"`, a
  no-op checkpoint.
- The `pass` just before a non-empty declaration becomes `pass_to` with `decision: "attackers"` or
  `"blockers"`.
- gorge's own `attackers` and `blockers` decisions are dropped for those steps.

Step indices never move, so `payment`, `abilities`, `neutral` and the decisions stay aligned. An engine
that gives a priority before the declaration, as the CR does, can still follow the rewritten steps:
`pass_to attackers` simply passes until the declaration is due.

### Decisions

`gorge_decisions` lists every choice gorge's runner made, in order. Key fields:

| Field | Meaning |
|---|---|
| `step` | The step it belongs to. `-1` means during setup |
| `seat` | Who was asked |
| `kind` | Normalised kind: `target`, `yesno`, `mode`, `choose_n`, `order`, `attackers`, `blockers` |
| `gorge_kind` | gorge's raw kind: `choose`, `target`, `arrange`, `modes`, `trigger_optional`, `trigger_order`, … |
| `picks` | The labels gorge picked |
| `pick_refs` | The picked objects or players, as refs |
| `object_picks` | The picked objects, in submission order |
| `pick_kinds` | The kind of each pick: `permanent`, `player`, `mana`, `color`, `yes`, `discard`, `trigger`, `sacrifice`, `search`, `x`, `pay_G`, … |
| `pick_idx` | The picks' positions among gorge's options. These are gorge's option lists, not the printed card's |
| `resume` | Context, for example `cast_modes`, `damage_split`, `look_ack` |
| `min`, `max` | How many could be picked |

Priority, mulligan and starting-player choices are never recorded.

How to answer each kind of prompt your engine poses:

- **Targets.** Take, from this step's unused `target` decisions in order, the refs your prompt can
  legally target, up to its maximum. An explicit empty answer to an "up to" prompt means no target.
  Divided damage is a `choose_n` with `resume: "damage_split"`, whose picks repeat a target once per
  point.
- **Yes or no.** `kind: "yesno"`, or a `choose_n` with one yes/no-shaped pick (pick kinds `yes`, `no`,
  `opening_yes`, `trigger_cost_pay`, `decline`). An empty answer declines.
- **Modes.** `kind: "mode"`. The label is the mode's text. For a Charm-style spell,
  `neutral.mode` also gives the mode's position on the card.
- **Choosing objects** (search, discard, sacrifice, choose a card). Match by ref, then by name. One
  gorge decision can answer several prompts in an engine that asks one card at a time.
- **Trigger order.** `kind: "order"` with pick kind `trigger`: `picks[0]` goes on the stack first, so
  it resolves last.
- **Scry, surveil and similar** (`gorge_kind: "arrange"`). The picks stay on top in pick order; the
  rest go to the destination the pick kind names.
- **X.** A `choose_n` with pick kind `x`, or a label `X = n`.
- **Colour, card name, number, type, copy targets.** By label.

If your engine asks something gorge recorded no answer for, don't guess. Set the row's `strict_miss`
(see [result rows](#result-rows)). If a decision is never used, report it as `leftover`. Decisions
gorge made while paying (pick kinds `pay_*`, `activate` inside a payment) and `look_ack` never count as
leftovers.

### `neutral`

gorge identifies abilities by their index in its compiled card, which follows the Forge script. An
engine that doesn't read Forge scripts needs another key. `neutral` provides one. Every map is always
present, and a field that can't be derived is left out, never guessed.

**`neutral.ability`**, keyed by step index, for every step with an `ability_index`:

| Field | Meaning |
|---|---|
| `class` | `mana`, `activated`, `loyalty` or `intrinsic_mana` (a basic land's CR 305.6 ability) |
| `ordinal` | 0-based position among the card face's abilities **of the same class**, in script order. Absent for an ability made from a keyword line |
| `keyword` | For an ability made from a keyword line, the keyword and its cost, for example `"Equip:1"`. Its position in the Oracle text isn't known, so it has no ordinal |
| `cost` | Normalised: mana symbols braced and run together (`{2}{W}`), then `{T}` or `{Q}`, loyalty as `+N` or `-N`, other parts in Forge's spelling with `~` for the card's name, joined with `, ` |
| `text` | The ability's description, with `~`. Absent for `intrinsic_mana` |
| `produces` | Mana abilities only: the colours one activation can add, in WUBRGC order, or `any`. Absent where the colour is chosen or depends on something else |

Real examples from an FDN sweep:

```json
{"class": "intrinsic_mana", "ordinal": 0, "cost": "{T}", "produces": "W"}
{"class": "activated", "ordinal": 0, "cost": "{T}, AddCounter<1/PAGE>", "text": "Scry 1."}
{"class": "activated", "keyword": "Equip:1", "cost": "{1}", "text": "Equip 1"}
```

Match the class and ordinal first, then check the text. Forge script order follows the Oracle text for
ordinary cards, so the ordinal is usually enough; a text mismatch tells you it isn't.

**`neutral.token`**, keyed by tokenscript stem, for every setup token:

```json
{"r_1_1_goblin": {"name": "Goblin Token", "types": ["Creature"], "subtypes": ["Goblin"], "colours": "R", "power": 1, "toughness": 1}}
```

Fields: `name` (the script's), `supertypes`, `types`, `subtypes`, `colours` (WUBRG letters; `""` is
colourless), `power` and `toughness` (numeric only), `keywords`, and `text`. Mint the token by these
characteristics. The comparator anonymises tokens, so the name you give it barely matters.

**`neutral.mode`**, keyed by the decision's index in `gorge_decisions`:

```json
{"1": {"step": 15, "picks": [{"ordinal": 0, "text": "Target opponent discards two cards."}]}}
```

It covers Charm-style "choose one" spells: `ordinal` is the mode's 0-based position on the card. Other
choices are left out: gorge offers only the modes that are legal at that moment, so its own index isn't
the printed position.

In one FDN chunk of 574 items, every one of 3,107 ability steps got a class and cost. 3,078 got an
ordinal; the other 29 were Equip and Crew abilities, which got `keyword`. All 64 setup tokens and all 7
modal decisions were filled.

## Result rows

Write one JSON object per request:

```json
{"id": "xe:FDN:0:0:w2", "name": "...", "engine": "toy", "strict": true, "ms": 8, "snapshots": [...]}
```

| Field | Meaning |
|---|---|
| `id`, `name` | From the request |
| `engine` | Your registry name. Required when the registry says `StampsEngine` |
| `strict` | `true` when every prompt was answered from the script |
| `ms` | Wall time for the row. Not compared |
| `snapshots` | One at `"setup"`, then one after each step. See below |
| `harness` | Set when the driver couldn't go on. Start it with `harness: ` |
| `strict_miss` | The first prompt the script had no answer for, for example `Missing target answer at step 3` |
| `leftover` | gorge decisions your engine never asked for |
| anything else | Ignored. Forge adds `forge_ref`, `request_sha` and `notes`, which are useful for debugging |

**Stopping early.** A row with `harness` set keeps every snapshot taken before the stop. That matters:
if gorge already differs within the stretch you did replay, the item is a real `diverge`, not a
`harness`. So take snapshots as you go and keep them when you fail.

**Lacking a card at run time.** If your engine finds out during a replay that it lacks a card, put
your registry's `LacksCardMsg` and the card's name in the harness message, for example
`harness: toy lacks card: Llanowar Elves`. The item is then classed as a missing card, not a harness
gap. (The class is still called `xmage_lacks`, from when XMage was the only follower.) Better still,
refuse the card in the preflight.

**Turn caps.** If your driver refuses to play past some turn, give the registry the start of that
message as `TurnCapMsg`. A tape that stops with it is classed `turn_cap`.

## Snapshots

A snapshot is what a player can observe at one checkpoint, in gorge's vocabulary:

```text
checkpoint   "setup" | "step <i> (<op>)"
turn         int
step         untap upkeep draw main1 begin-combat declare-attackers declare-blockers
             combat-damage end-combat main2 end cleanup
active       seat
priority     seat                                   (not compared)
over         bool
players[]    seat, life, counters {KIND: n}?, hand[] (sorted), graveyard[] (oldest first),
             exile[] (sorted; cards the seat owns), command[] (not compared), library_count,
             library_top[] (top first, at most 5), pool ("WUBRGC" letters)
permanents[] name ("" when face down), controller, owner, token, tapped, face_down,
             pt ("P/T", creatures only), damage, counters {KIND: n}?, types[] (card types,
             supertypes and subtypes, sorted), colors ("WUBRG" letters, "" colourless),
             keywords[] (sorted), attached_to (the host's current name, or "pN"),
             attacking, blocking
stack[]      top first: kind ("spell" | "ability"), source (the card's name), controller
```

Both damage steps are named `combat-damage`. Counter kinds use gorge's names: `P1P1`, `M1M1`,
`LOYALTY`, `CHARGE`, `LORE`, or another kind's name in capitals without spaces.

A real snapshot of a permanent:

```json
{"name": "Mountain", "controller": 0, "owner": 0, "token": false, "tapped": true, "face_down": false,
 "damage": 0, "types": ["Basic", "Land", "Mountain"], "colors": "", "keywords": [],
 "attacking": false, "blocking": false}
```

`gorge.jsonl` in every play directory has gorge's own snapshots for each item, in exactly this shape.
gorge's also carry `ref` on permanents and an `offered` list of legal actions; you don't need either.

### Card names

Use gorge's spellings, which are Forge's: the printed name of face 0 for a card outside the
battlefield, the current face's name on the battlefield, and the first half for a split or Room card.
If your card database spells a name differently (accents, punctuation, a face name), map it on the way
in and back on the way out, as the XMage driver does.

## How snapshots are compared

The comparator renders each snapshot as an ordered list of fields and reports the **first** difference,
with its checkpoint, field name and both values. In order:

1. `turn`, `step`, `active`, `over`, and the number of players.
2. Per seat: `pN.life`, `pN.counters`, `pN.hand` (as a sorted list), `pN.graveyard` (order matters),
   `pN.exile` (sorted), `pN.library_count`, `pN.library_top` (order matters), `pN.pool` (letters
   re-sorted to WUBRGC).
3. `permanents`, as a multiset. Each permanent becomes one line of controller, owner, name, types,
   colours, P/T, tapped, face down, damage, counters, attachment and combat state. **Tokens are
   anonymised to `token`** and matched by characteristics. Keywords are compared only when an item asks
   for them.
4. `stack`, top first, as `kind:source@pN`.

Not compared: `priority`, `winner`, `command`, permanent `ref`, damage outside the battlefield, and
`offered` unless an item asks. When both sides say the game is over, only the turn, step, active seat
and life totals are compared.

**Checkpoint pairing.** Your snapshot list must be as long as gorge's, and checkpoint i must have the
same label or the same step index. A different length is a harness error, unless your row stopped
early with `harness` set.

**`ignore`.** An item's `ignore` entries match fields by suffix: `library_top` skips both seats'.
`<field>@N` skips it from checkpoint N on, where 0 is `"setup"`. Items that shuffle carry these.

## What you are never sent

The harness only sends positions it can describe exactly, and gorge's own runner must reproduce each
item before it goes out. A window is refused when its starting position holds something setup can't
place, for example:

- a non-empty stack, extra turns, day and night, the monarch or the initiative;
- this-turn history: lands played, spells cast, life gained or lost, cards that entered this turn;
- a non-empty mana pool, or player counters such as poison or energy;
- face-down, phased-out, copied, mutated or melded permanents, marked damage, or combat state;
- continuous effects from a spell or ability that already resolved;
- tokens that are copies, or that lack a tokenscript.

Tapes stop at the first thing the harness can't express, most often a shuffle or another random
choice (`unsupported_kind:rng`). Attacks are always at one player: never at planeswalkers or battles,
and never split.

## Registry fields

Each follower is one entry in `oracle/xengine/follower/follower.go`. A zero field is the behaviour of a
driver without that quirk.

| Field | Meaning |
|---|---|
| `Name` | The id: `-engine`, your rows' `engine`, the env prefix (`XENGINE_<NAME>_…`) |
| `Display` | The name messages print |
| `Runner` | The runner, relative to the repository root |
| `Prep` | `PrepFollowerExport` for a new engine |
| `StampsEngine` | Refuse a row whose `engine` isn't this one |
| `ConsumesPayment` | The driver spends `payment[i]` exactly, so a pool split after a cast is a real difference |
| `TurnCapMsg` | The start of the driver's turn-cap harness message (class `turn_cap`) |
| `CastModes` | The `cast_mode` values the driver can act on. `nil` means all |
| `SkipsDamageStep` | The engine skips the combat damage step when no creature is left in combat |
| `StopsInCleanup` | The engine gives priority in the cleanup step after an end-the-turn effect |
| `TrailingMisses` | The engine asks a cleanup discard or optional trigger after the item's last step; such a miss alone isn't `decision_shape` |
| `TapsWithCast` | The driver taps mana and casts within one priority out of turn |
| `TriggerOrderByName` | The driver orders triggers by text or source name, so it can't tell two from same-named objects apart |
| `LacksCardMsg` | The text in a harness message that names a card the engine lacks |
| `JVM` | The runner starts a JVM and must wait for a JVM slot |

Only set a quirk your driver has, and only once you've seen it in results. Each one tells the
classifier to stop calling a kind of difference `diverge`.
