# phase.rs and ManaBrew

We read both engines' source in November 2026 to plan a follower for each. These are the findings
that matter to the follower, written for the engines' own authors. Each section says what we read,
what fits the protocol well, and the hurdles we expect. All of it comes from reading the code and
building it. We haven't yet run a scenario through either engine, so treat the rest as a plan.

If you work on either engine and something here is wrong, or you'd like to own the follower
yourselves, please [open an issue](https://github.com/adams-shaun/gorge-compliance/issues).

## phase.rs

[phase.rs](https://github.com/phase-rs/phase) parses Oracle text from MTGJSON into its own card data.
It is the only engine here that doesn't share an encoding with gorge, Forge or XMage, so it is the
follower we expect to add the most independent evidence.

**What we read.** Commit `c53dd8a` (v0.122.0). Its release build of `phase-engine` compiles cleanly on
the pinned toolchain (`nightly-2026-04-19`, from `rust-toolchain.toml`). From scratch that took 12
minutes on our machine.

### What fits well

- **A debug API that places cards raw.** `create_debug_cards` with `run_etb: false`, plus debug
  actions for tapped state, counters, summoning sickness, attachments, life, tokens and face state,
  covers the [setup contract](protocol.md#setup) without engine changes. The test helper
  `add_real_card` runs the real entry pipeline, so enters-tapped and enters-with-counters apply; a
  follower must not use it for setup.
- **One entry point for actions.** Every action goes through `apply(state, actor, GameAction)`, and the
  engine lists the legal candidates for each prompt (`AiDecisionContract::issue`). A scripted answer can
  be checked against that list before it's submitted, which is exactly what a strict driver needs.
- **Stable identity.** An `ObjectId` survives zone changes, and `incarnation` marks a new object under
  CR 400.7. The follower's ref table binds gorge refs to `(ObjectId, incarnation)`.
- **An honest support flag.** Each face has parser gaps (`card_face_gaps`) and a list of known
  misparses (`confirmed_misparse`). The follower's preflight refuses a card if any face has either.

### Card support

From phase's published coverage data (card-data hash `d0ffa159dcde97c7`, engine `b14f974`, MTGJSON
5.3.0+20261110):

| Pool | Supported |
|---|---|
| FDN | 494 of 517 (95.6%) |
| FDN commons (Pauper-legal) | 196 of 196 |
| Pauper-legal cards | 10,405 of 11,123 (93.5%) |
| Standard | 5,129 of 5,725 (89.6%) |

21 of FDN's 23 unsupported cards parse without a gap but carry a known misparse, so a preflight that
checks only parser gaps would let them through. "Supported" means no *known* gap; the follower is how
we'd learn about the unknown ones.

### Hurdles

- **Matching abilities.** phase indexes abilities by its own parse of the Oracle text, and gorge by the
  Forge script. The follower matches on [`neutral.ability`](protocol.md#neutral): class, then ordinal
  among abilities of that class, then the text as a check.
- **Names.** phase names tokens by CR 111.4 ("Goblin"), and Forge's scripts by "Goblin Token". The
  comparator anonymises tokens, so this only matters for a token's `name` field. Split, Room and
  multi-face cards, and accented names, need mapping to gorge's spellings.
- **Prompts gorge may not record.** phase asks some choices that gorge and Forge resolve internally:
  replacement-effect order, combat damage order among several blockers, Phyrexian payment per pip, and
  the bottom order after scry. Until gorge records them, they become strict misses (`decision_shape`).
  We'll add recording where the counts justify it.
- **phase-only conveniences.** The batch-resolve prompts (`ResolveAll*`) are never started by a
  follower, and cards with real-time `PhysicalContest` prompts are refused.
- **Speed (CR 702.179).** We haven't found a setter for a player's speed, so items that set it will be
  harness rows for now.
- **Stack size.** The engine needs at least a 16 MiB stack (`RUST_MIN_STACK`). The follower runs it on
  a thread with 64 MiB and catches panics per row.
- **Pace of change.** phase made 15 releases between 2026-10-12 and 2026-11-10, and its `GameAction`
  and `WaitingFor` enums grow often. The follower pins one engine commit together with the card data
  built from it, and matches those enums exhaustively, so a new variant is a compile error rather than
  a silent gap. The card-data metadata named commit `b14f974` while the repository's head was
  `c53dd8a`, so the pin has to name both.

### Plan

The follower lives in gorge-compliance under `followers/phase/`. It depends only on `phase-engine`'s
public API, so it could move into phase's workspace unchanged if you want it. It carries no card fixes:
its point is to report phase as it is.

| Milestone | Scope | Done when |
|---|---|---|
| P0 | Toolchain, pin, preflight | The preflight reproduces the numbers above |
| P1 | Setup only | The `"setup"` snapshot agrees on at least 95% of FDN-commons scenarios |
| P2 | Cast, activate, resolve, pass; targets, modes, X, yes/no | Every FDN row agrees or has an explained class |
| P3 | Windows: combat, `pass_to`, attach | `decision_shape` under 20% of sent windows |
| P4 | Whole-game tapes | Tapes run to the lowering's cut |

phase's contributor guide (`docs/AI-CONTRIBUTOR.md`) is a procedure for single-card fixes, with
strict rules for outside pull requests. A follower is a different kind of contribution, so we'll start
with an issue rather than a PR. Related prior work:
[kevinbrar/mtg-rl-sim](https://github.com/kevinbrar/mtg-rl-sim) drives phase from Python for
reinforcement learning, and solved several of the same mapping problems.

## ManaBrew

[ManaBrew](https://github.com/witchesofthehill/manabrew) is a Rust port of Forge's engine that reads
Forge's card scripts. It is in the Forge family, so it doesn't add an independent reading of the cards.
What it adds is a second implementation of Forge's rules engine over the same scripts: a check on
Forge, on our Forge driver, and on ManaBrew itself. (gorge also serves the [ManaBrew wire protocol](../../api/manabrew.md)
for ManaBrew clients; that is unrelated to the follower.)

**What we read.** Commit `dadee73`. `manabrew-engine` builds on stable Rust 1.98 in under a minute.
The card scripts come from its Forge submodule, `witchesofthehill/forge@be9e0c8`.

### What fits well

- **The same card scripts as gorge.** The `abilities` script lines and gorge's mode labels match
  ManaBrew's parsed scripts directly, so a ManaBrew follower needs no `neutral` matching.
- **A headless game is already possible.** The (now deprecated) parity runner builds a game without the
  starting-player roll or mulligans, places cards and runs turns, which is the shape a follower needs.
- **Card data from any Forge tree.** Scripts are read from an archive built from a `res/` directory, so
  the follower can build it from gorge's pinned Forge scripts (`FORGE_REF`) rather than ManaBrew's own
  submodule. Only 4 FDN scripts differ between the two, which makes FDN a safe place to start.

### Card support

ManaBrew doesn't mark cards as unsupported, so we scanned the scripts statically for vocabulary the Rust
code doesn't know: effect types, trigger modes, static and replacement modes, and parameter keys.

| Pool | No red flag |
|---|---|
| FDN | 479 of 517 (92.6%) |
| All of Forge's cardsfolder | 30,138 of 34,442 (87.5%) |

The flagged FDN cards mostly use a parameter key that appears nowhere in the Rust source. Across the
whole cardsfolder, the unknown effect types are `FlipCoin` (84 cards), `PutSticker` (38), `Empower`
(35), `Recruit` (10), `Assimilate` (8) and `HealDamage` (3).

`FlipCoin` is worth a look on its own. Forge renamed the effect from `FlipACoin` on 2026-04-02, and
ManaBrew's own Forge submodule has the new name, but the Rust code only knows the old one. So all 84
coin-flip cards currently do nothing in ManaBrew itself, not just in our follower.

A static scan only shows that the vocabulary exists, not that the behaviour is right.

### Hurdles

- **Silent fallbacks.** Where ManaBrew meets something it doesn't know, it carries on: an unknown effect
  does nothing, an unknown trigger mode drops the trigger, an unknown static or replacement is inert, an
  unsupported trigger condition counts as true, and an unknown count is 0. 74 of the 83 `PlayerAgent`
  methods have defaults that pick the first, minimum, false or true answer. In a follower, each of
  those would look like agreement or like a rules difference. So the follower:
    - refuses any card with flagged vocabulary in the preflight;
    - implements all 83 agent methods, failing on any prompt it can't answer from the script;
    - fails a row on any engine diagnostic, on an illegal action the engine downgraded to a pass, and on
      any random draw it didn't script.
- **Engine hooks.** Three small engine changes, each useful beyond gorge:
    1. a hook that lets the agent change state from inside the priority loop (to add pool mana and to
       apply `tapped_after_untap`), like the existing restore-request hook;
    2. a way to place a card in a zone with given flags and no entry pipeline, since `move_card` applies
       entry replacements and adds loyalty;
    3. a sink for the engine's diagnostic messages ("Unimplemented effect API type" and the like), which
       today go to stderr. It would have caught the `FlipCoin` rename.
- **Trigger order.** The engine orders simultaneous triggers itself, in APNAP order. Where gorge chose an
  order, the item can't be matched until there is a hook for it.
- **Licence.** ManaBrew is AGPL. The follower lives in a fork and talks to the harness through files,
  like Forge's GPL driver today. Its output is never committed.

### Plan

| Milestone | Scope | Done when |
|---|---|---|
| M0 | A probe: build, card archive from `FORGE_REF`, place a board and cast one spell | Open questions answered (back-face setup, combat flags, cost callbacks) |
| M1 | Hooks 2 and 3, setup, snapshots, preflight | `"setup"` agrees on at least 95% of FDN-commons items that pass the preflight |
| M2 | Hook 1, cast, activate, resolve, the scripted agent | FDN rows agree or have an explained class; Forge-vs-ManaBrew differences triaged |
| M3 | Combat, `pass_to`, windows | A commons sweep beside Forge and XMage |
| M4 | Whole-game tapes | Tapes run past Forge's turn cap |

ManaBrew comes after phase.rs in our order of work, because phase.rs adds a new family. We'd like to
offer the three hooks upstream as generic agent and diagnostic hooks, following ManaBrew's contribution
rules. Since ManaBrew's parity harness is deprecated, a nightly gorge-vs-ManaBrew run could also take
over its job.
