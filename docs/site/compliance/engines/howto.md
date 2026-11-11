# How-to guides

Task-sized guides for building a follower. They assume you've done the [quickstart](quickstart.md) and
keep [the protocol](protocol.md) open beside them.

## Grow the runner in milestones

Each stage below is useful on its own, and anything a stage can't do yet becomes a `harness` row
rather than a false finding. So you can run real sweeps from the first stage on.

| Stage | Build | Corpus | Done when |
|---|---|---|---|
| 0. Card check | `--preflight` | `oracle/xengine/pools/fdn-commons.txt` | It refuses what your engine knows it can't play, and nothing else |
| 1. Setup | Place the board, reach the setup checkpoint, write one snapshot; `harness` for every step | `oraclediff gen -level A` on FDN | The `"setup"` snapshot agrees on nearly every item |
| 2. Cast and resolve | `cast`, `activate`, `resolve`, `pass`; targets, modes, X, yes/no, choose-n from the script | The same, then all FDN | Every row agrees, or carries a reason you understand |
| 3. Windows | `pass_to`, `attack`, `block`, `attach`; combat prompts | `xengine play -pool … -windows` | `decision_shape` is a small share of sent windows |
| 4. Tapes | Whole games from turn 1: real turn structure, draws, cleanup | The same play directories | Tapes run to the point where the lowering cut them |

Widen the corpus only when the current one is quiet: FDN commons (`fdn-commons.txt`, 196 cards), then
all of FDN (`fdn.txt`, 517), then other sets with `-set`.

## Place a board without entry effects

Setup is the part most engines lack a public API for. The contract (the [setup table](protocol.md#setup))
is that placed permanents arrive **as if they had always been there**: no ETB triggers, no "enters
tapped" or "enters with counters" replacements, no summoning sickness, no "this turn" history.

What usually works:

- **A debug or editor API**, if your engine has one for a deck builder or test harness. Check that it
  really skips the entry pipeline. A function that "adds a card to the battlefield" by moving it there
  usually runs replacements.
- **A small placement hook in the engine**: put card X in zone Z for controller C with these flags, and
  touch nothing else. In the Forge driver this is about 500 lines because Forge has no such hook. In an
  engine you control it's a few dozen lines, and worth sending upstream.
- **Counters are added on top.** A planeswalker placed with `{"LOYALTY": 1}` ends up with its printed
  loyalty plus one. Tokens are the exception: their `counters` are the complete set.

Then clear any per-turn bookkeeping the placement left behind, set turn 1 with `p0` active, and run the
engine forward from the untap step to `p0`'s first main-phase priority. Apply `tapped_after_untap` there,
and take the `"setup"` snapshot.

Test it against `gorge.jsonl` first: compare only checkpoint 0 of each item until it agrees.

## Drive priority exactly as gorge does

Most early disagreements are about priority, not cards. gorge follows the CR:

- **One `pass` step is one priority pass** by the named seat. Your engine must not auto-pass for either
  player, and must not stop anywhere else. If a different seat holds priority when a `pass` arrives, the
  sequences have drifted. Stop with a harness row rather than guessing.
- **Priority is given in every step the CR gives it**: upkeep, draw, beginning of combat, each combat
  step, end step, and both main phases. An engine that skips a step, like Forge skipping combat damage
  when no creature is left in combat, needs a registry quirk so the classifier knows.
- **`resolve` passes until the stack is empty** and the active player holds priority, never stopping at
  a cleanup priority.
- **The checkpoint after a cast** is when priority comes back to the caster with the spell on the
  stack, not when it resolves.

If your engine gives priority somewhere gorge doesn't, report it as a harness row with a clear message
the first time it happens, and tell us. It may be a gorge bug.

## Answer decisions strictly

Answer every prompt from `gorge_decisions`, and never let the engine's AI or a default fill a gap.

- **A prompt with one legal answer** can be answered automatically. Note it in your row's `notes`.
- **A prompt with several answers and no recorded decision** is a strict miss. Set `strict_miss` to
  `Missing <what> answer at step <i>` and stop. The item is classed `decision_shape`.
- **A recorded decision you never used** goes in `leftover`.
- **Don't add a "loose" retry** in the first version. The Forge driver re-runs a missed item with
  Forge's AI answering, but loose rows are much harder to interpret.

A silent default is worse than a miss: it turns into a `diverge` that someone has to read.

## Match an ability to gorge's index

`activate` steps carry gorge's `ability_index`, an index into gorge's compiled card.

- **If your engine reads Forge scripts** (ManaBrew does), `abilities[step]` is the exact script line.
  Match it against your parsed abilities.
- **Otherwise, use `neutral.ability[step]`.** Find the abilities of the same `class` on the face, take
  the one at `ordinal`, and check its `cost` and `text` against yours. If they don't match, stop with a
  harness row such as `harness: ability_unmatched at step 3`. Don't fall back to "the first one".
- **Keyword abilities** (Equip, Crew, …) have `keyword` instead of an ordinal: match by keyword and
  cost.
- **Basic land mana** is `intrinsic_mana`, with `produces` naming the colour.

## Pay from the pool

The harness never asks your engine to pick lands to tap. Every mana source gorge used is its own
`activate` step before the spell. So:

1. Activate mana abilities as their own steps. Each completes at once, with no stack.
2. When the spell or ability is cast, pay its cost from the floating pool only, spending the units in
   `payment[i]` first.
3. If the engine's own auto-payment would tap anything, turn it off for the run.

If your driver spends `payment[i]` exactly, set `ConsumesPayment: true`. Then a pool split that differs
after a cast is reported as a real difference.

## Write snapshots gorge can compare

A checklist, in the order mistakes usually show up:

- Step names are gorge's (`main1`, `begin-combat`, `combat-damage`, …), and both damage steps are
  `combat-damage`.
- Colours are WUBRG letters in that order, and colourless is `""`. The pool is WUBRGC letters.
- `hand` and `exile` are sorted, `graveyard` keeps its order with the oldest card first, and
  `library_top` is the top five, top first.
- `types` holds card types, supertypes and subtypes together, sorted.
- `pt` is only set for creatures, after every continuous effect has applied. Recompute layers after
  any setup change.
- `attached_to` is the host's current name, or `pN` for a player.
- A stack entry's `source` is the card's name, not an id.
- Counter kinds use gorge's names (`P1P1`, not `+1/+1`).
- Card names use gorge's spellings: map yours on the way out.

## Debug one item

Every finding gets a directory under `<diff out>/finds/<class>/<id>/`:

| File | What it is |
|---|---|
| `REPRO.md` | The commands to re-export, re-run and re-diff just this item |
| `input.jsonl` | The item as one line, cut after the step that differed, ready for `follower-export` |
| `scenario.json` | The same item, pretty-printed for reading |
| `gorge.json` | gorge's result: snapshots, decisions and a transcript of the game |
| `<engine>.json` | Your row |
| `verdict.json` | The class, checkpoint, field and both values |

`REPRO.md` ends with an `xengine diff … -only <id>` command, which classifies one re-run row without
re-running the whole chunk. To see why gorge did something, read the `transcript` in `gorge.json`.

## Handle shuffles and randomness

You don't need to make your engine's shuffles match gorge's:

- A tape stops at the first random choice: a shuffle, a coin flip, a random discard.
- A window whose turn shuffles a library carries `ignore: ["library_top"]`, so library order isn't
  compared.
- A hand or library difference after a recorded random draw is classed `rng`, not `diverge`.

Do seed your own randomness from the request, so reruns are identical. If your engine would consult
randomness for something gorge didn't (a coin flip gorge's script doesn't have, say), stop with a harness
row. Cards whose effects need random outcomes are good candidates to refuse in the preflight.

## Pin your engine and card data

A difference in card data looks exactly like a rules difference, so both sides pin:

- gorge-compliance pins a gorge commit (`GORGE_PIN`), and gorge pins the Forge card scripts it compiles
  (`FORGE_REF` in gorge's Makefile).
- Pin your engine to a commit, and your card data to the revision built from it. Your setup script
  should refuse to run if they don't match.
- If your engine reads Forge scripts, build its card data from gorge's `FORGE_REF`, not from whatever
  Forge version your engine ships with. The Forge runner checks that its card-script trees are
  byte-identical to `FORGE_REF`'s before every run (`scripts/forge-oracle-setup.sh --check-pin`).

Bump on purpose, never to "latest" automatically, and re-run the same corpus after a bump to see what
it changed.

## Run your engine beside Forge and XMage

The sweep takes a list of followers:

```sh
XENGINE_ROOT=/tmp/xe XENGINE_FOLLOWERS='forge xmage toy' \
  XENGINE_POOL=oracle/xengine/pools/fdn-commons.txt \
  scripts/xengine-sweep.sh --sets FDN --chunks 4 --games 50
```

| Variable | Meaning |
|---|---|
| `XENGINE_FOLLOWERS` | Space-separated registry names. Default `forge xmage` |
| `XENGINE_ROOT` | Output root, outside the repository |
| `XENGINE_POOL` | A card list to draw decks from (passed to `play -pool`) |
| `XENGINE_POOL_FILTER` | Card names to leave out of the pool |
| `XENGINE_<NAME>_RUNNER` | Override your runner's path |
| `XENGINE_<NAME>_MEM` | Memory for your runner's heavy lock. Default 12G |
| `XENGINE_<NAME>_WAIT` | Seconds to wait for the heavy lock. Default 20000 |

Sweeps are resumable: re-running the same command skips finished chunks. Each chunk's results are in
`<root>/<SET>/<chunk>/diff-<name>/`, and `<root>/SUMMARY.md` has each follower's class histogram.

Today the three-way comparison (`xengine adjudicate`) runs only when both Forge and XMage are followers,
and it doesn't include a third engine yet. Your engine's own diff against gorge runs either way.

## Draw decks from a card pool

`xengine play -pool FILE` draws both decks from a list of card names, one per line, with `#` comments.
Each deck is 23 different spells and 17 basics in two colours. The pool goes through the same filter as a
set: gorge must fully support each card. The pools shipped in `oracle/xengine/pools/` are:

| Pool | Cards | Source |
|---|---|---|
| `fdn.txt` | 517 | The FDN manifest |
| `fdn-commons.txt` | 196 | FDN cards that are Pauper-legal, per MTGJSON |

Make your own with `xengine pool -set <SET> [-legal <format> -mtgjson <SET>.json.gz] -out FILE`, or by
hand. `-pool-filter FILE` leaves the named cards out, which is handy for cards your engine refuses:
fewer items then end up `not_sent`. The run writes `pool.json` with the pool's sha256, so results
record exactly which list they came from.

## Report a finding

A `diverge` is a lead, not a verdict. Before anyone calls it a bug in either engine:

1. Read the printed card and the Comprehensive Rules, and decide what should happen.
2. Check whether other engines agree with you or with gorge.
3. Rule out your driver: re-run the item from its `REPRO.md` and read your engine's own log.

If gorge is wrong, open an issue on
[gorge-compliance](https://github.com/adams-shaun/gorge-compliance/issues) with the item id, the repro
directory's `verdict.json`, and the rule. If your engine is wrong, it's yours to fix. If you do, we'd
like a link, because we [keep a list](https://github.com/adams-shaun/gorge-compliance/blob/main/docs/upstream-bugs.md)
of engine bugs the harness found.

## Keep the runner robust

- Catch panics or exceptions per row and turn them into harness rows.
- Cap each row's work. The Forge driver stops after 4,000 loop iterations without progress.
- Give a deep-recursion engine a big stack. phase.rs, for example, needs at least 16 MiB.
- If a crash can corrupt process-wide state, run rows in a worker process and restart it after a crash.
