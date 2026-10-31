# A3 Forge AI hints: parsed as card facts, read by an opt-in cast rule

Date: 2026-10-20.
Outcome: **implemented and armed only on the experimental `forge-heuristic`
policy.** The default `bot` is unchanged and no win rate is claimed.

## Census

Measured over the linked `.cards/cardsfolder` corpus on 2026-10-20. File
counts are `rg -l -F`; value counts are `rg -F` line counts. They differ
because a file may carry more than one `SVar:PlayMain1`/`AI:RemoveDeck` line.

| pattern | files | value counts |
|---|---:|---|
| `AI:RemoveDeck:` | 4699 | `All` 2501, `Random` 2383, `NonCommander` 70 |
| `AI:RemoveDeck:Random` | 2382 | 2383 lines |
| `AI:RemoveDeckAll` | 0 | 0 |
| `AI:RemoveDeckCard` | 0 | 0 |
| `SVar:PlayMain1` | 1524 | `TRUE` 1448, `ALWAYS` 54, `OPPONENTCREATURES` 11, `True` 10, `Random` 1, `FALSE` 1 |

The two `AI:RemoveDeck*` spellings the ticket names beyond the base key are
absent from this pin. They are parsed anyway because they are part of Forge's
closed vocabulary and cost nothing at load time; a future corpus can add them
without another parser change.

## What was built

**The facts.** `engine/cards` now carries the hints on the printed face:

- `cards.Face.RemoveDeck` is the canonical value of `AI:RemoveDeck:<value>`
  (`All`, `Random`, `NonCommander`); `AI:RemoveDeckAll` and
  `AI:RemoveDeckCard` canonicalise to `All` and `Card`. Empty means no hint.
  `AI:` was previously ignored wholesale as deck-builder metadata.
- `cards.Face.PlayMain1Hint()` reads the existing `Face.SVars["PlayMain1"]`;
  it deliberately does **not** add a parallel parse path. Forge's
  `castPermanentInMain1` treats `ALWAYS` as unconditional,
  `OPPONENTCREATURES` as the opposing-creature condition, and every other
  value except `FALSE`/empty (including `TRUE` and `Random`) as the
  own-creature condition. The spellbench IR reader
  (`bots/spellbench/builtins/tactical_ir.go`) now reads `PlayMain1Hint()`
  instead of the raw SVar, so it agrees with the canonical set; the one
  corpus card that changes the tool is Webweaver Changeling
  (`SVar:PlayMain1:Random`), whose `playMain1` flips from false to true.
- `view.CardView` exposes the facts as `play_main1,omitempty` and
  `remove_deck,omitempty`. They are printed facts of a visible card, never
  hidden information.
- `botpolicy.Card` carries `PlayMain1 string` and `RemoveDeck bool`. Both
  adapter halves fill them from the same face: `BoardFromView` from the
  projected `CardView`, and `BoardFromGameInto`/`BoardFromInc` from
  `cards.Face` directly. The board-table parity reference in
  `board_table_equiv_test.go` carries the same two fields, so the existing
  whole-game parity tests hold the adapters together.

The cache grew a face field, so `cacheVersion` is 8 and the image schema is 2;
`FaceEqual`, the flat face codec and the compiler fingerprint census were
updated with it. A v7 cache or image is recompiled rather than served with
every `RemoveDeck` empty.

**The opt-in rules.** `bots/botpolicy/aihints.go` defines
`AIHints{Enabled bool}` (zero value = off) and two hooks in the existing cast
scorer:

- `aiHintsVetoCast` refuses a card whose `RemoveDeck` fact is set. Forge's
  hint means "the AI does not know how to play this card"; the opt-in policy
  therefore never chooses it as a cast, even when its body would otherwise
  outrank the alternative. The default policy still ranks it normally. The
  veto is applied at every place the cast scorer enumerates a candidate:
  `chooseCast`, C11's `castEntries` subset table, and the tap gate's three
  folds (`tapWants`, `bestUnpayable`, `AnyCastableNow`), so the armed policy
  never floats mana toward a card it would refuse to cast.
- `aiHintsCastBonus` adds 40 to a `PlayMain1` card in the caster's own first
  main phase when its condition holds: `ALWAYS` unconditional,
  `OPPONENTCREATURES` when an opponent controls a creature, and
  `TRUE`/unknown when the caster does. "First main" is the active player's
  (`FirstMain && MyTurn`); the opponent's first main earns nothing, which
  matters for the 21 instants carrying the hint. It is a score bonus, not a
  hard rule, so a C9 threshold can still hold the card. No second-main
  penalty is added; the hint only pulls the card forward.

`heuristic.Bot.EnableAIHints`/`AIHints()` carry the configuration into
`Board.AIHints`, exactly as A1/A2's `Removal`/`Targets` do.
`forgeheuristic.New` arms it and the registry Description names it; the
default `bot` never calls the constructor.

## Firing evidence

Focused, capped tests in `bots/botpolicy/aihints_test.go`:

- `TestAIHintsPlayMain1CastsPrecombat`: a 2/2 base score is 38; a C9
  threshold of 39 holds it with the rule off, and the +40 `ALWAYS` bonus
  clears it with the rule on. The same armed board outside the first main
  still passes, and the opponent's first main (FirstMain with MyTurn false)
  earns nothing, so the rule moves WHEN the card is cast, not whether it is
  ever cast.
- `TestAIHintsPlayMain1Conditions` pins all three conditions and the empty
  mode.
- `TestAIHintsRemoveDeckNeverCast`: the 3-power `RemoveDeck` creature
  outranks a 1/1 with the rule off and is vetoed with it on; when it is the
  only castable option the scorer passes.
- `TestAIHintsRemoveDeckNeverTapTarget`: with the rule off, an unpayable
  `RemoveDeck` card makes the tap gate want a tap (`chooseTap` picks the
  source, `AnyCastableNow` is true); armed, all three read false/-1 because
  the card is vetoed.

`bots/heuristic/adapter_aihints_test.go` projects a real game with
`view.Project` and then lifts it with `BoardFromView`, so it holds the
projector hunk (not a hand-built `CardView`) and the game half to the same
values. `engine/cards/aihints_test.go` pins parsing, canonicalisation and the
`PlayMain1` mapping.

## The `RemoveDeck` wording

The ticket's acceptance line says a `RemoveDeck` card is "never chosen as a
removal target of a deck-wide-bad shape". In gorge, deck-choice options do not
carry card facts, and the cast scorer is the only path this ticket arms; the
implemented reading is therefore **"never chosen as a cast by the armed cast
scorer"**. This is the same deck-wide-bad shape Forge's hint names: the AI
does not know how to play the card, so the policy declines to cast it. If a
later ticket needs a deck-choice veto, that is a separate fact path and is not
claimed here.

## Scope and non-claims

- No per-card `SpecialCardAi` classes, no A1/A4/A5 work, no change to the
  default `bot`.
- No botbench arm or strength measurement. The A2 result already showed the
  constructed corpus rarely poses these asks; the rule is opt-in so the
  orchestrator can measure it separately.
- The hint is advisory metadata, not a rules restriction. It never changes
  target legality or payment; it only changes the opt-in cast ranking.

## Reproduce

```sh
systemd-run --user --scope -q -p MemoryMax=2G -p CPUQuota=200% \
  env GOMAXPROCS=2 GOMEMLIMIT=1536MiB \
  go test -timeout 2m -run 'TestAIHints' ./bots/botpolicy
systemd-run --user --scope -q -p MemoryMax=2G -p CPUQuota=200% \
  env GOMAXPROCS=2 GOMEMLIMIT=1536MiB \
  go test -timeout 2m -run 'TestAdapters|TestBoardFromView' ./bots/heuristic
systemd-run --user --scope -q -p MemoryMax=2G -p CPUQuota=200% \
  env GOMAXPROCS=2 GOMEMLIMIT=1536MiB \
  go test -timeout 2m ./engine/cards ./engine/view
```
