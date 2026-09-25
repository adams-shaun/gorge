# pn21: known-card projection for search sampling (2026-09-24)

## TL;DR

- `searchprobe.ProjectKnownCards(History)` / `KnownCardTracker` derive, from
  the observation contract alone (redacted frames + the seat's own semantic
  answers), which cards the deciding seat KNOWS are in each hand and each
  library, with known library top/bottom positions.
- Every claim is checked against the true engine after every observed frame
  of 15 real-deck games (both seats' views, 15,080 frames; decks that scry,
  surveil, Ponder, Brainstorm and bounce): 0 false claims.
- `SampleOptions.KnownCards` (off by default) rejects any world that
  contradicts the projection. On full-replay worlds it never fires (the
  bench fixture and four effect fixtures accept byte-identical worlds on/off).
- Stretch done: `SampleOptions.Redeal` (off by default) is a fallback that
  runs only when the sampler starves. It redeals the hidden cards the seat
  does not know, uniformly, and pins every known card. On a 6-game hindsight
  smoke with identical settings, `no_world` fell from **196/216 to 6/216**
  multi-candidate decisions. At turn 13+ it fell from 125/125 to 0/125.
- With the flag off, all existing goldens (`TestSampleRealDeckGolden`,
  `TestTeacherChoiceRealDeckGolden`) are unchanged. New result fields are
  `omitempty`.

## What is tracked

References are observer refs (`Identity.ID`). Processing happens in event
order within each frame.

| Source | Knowledge |
|---|---|
| observed move naming the card (`Obj != 0`) into a hand/library | card in its owner's hand / library; into a library it is appended at the bottom (events.Move semantics), extending the known bottom suffix |
| actor `KArrange` decision (Scry/Surveil/Rearrange kinds `""`, `top`, `bottom`, `graveyard`) | options are the top k of the actor's library, top first |
| actor's answer + the next frame's first actor `LibraryOrder` | the new order: pile A in answer order, then the old known cards below the window. Pile B is ordered only when it is a single card. |
| unseen draw with a known top | the top card moves to the drawer's known hand |
| non-secret `"revealed … as a cost"` note | card stays in its owner's hand (cost reveals are from hand) |
| end-of-frame board | public/stack cards are in no hidden zone; the actor's hand is exactly the board hand |

## How knowledge is invalidated

- `Shuffle` clears that player's known positions. Membership survives.
- An unexplained `LibraryOrder` also clears that player's positions.
- An unseen (`Obj == 0`) move out of a hand forgets every hand claim, for
  every seat. The event's Player is not trusted as the zone owner.
- An unseen non-draw move out of a library forgets every library claim.
- An unseen move into a library forgets every known bottom.
- An unseen draw with no known top forgets that library's position-less
  members. It also forgets the known bottom unless the library provably held
  more cards than the bottom list.
- A known card named anywhere else is dropped from its old zone.
- Zone sizes are tracked through events and resynced from the board each
  frame. An unexplained size change, or more claims than cards, clears that
  zone.

## Deliberately conservative (card stays UNKNOWN)

- Public reveal notes that do not state a zone: RevealHand / Thoughtseize-style
  hand reveals, and revealed library windows. The same note shape is used for
  hand reveals, library reveals and mills.
- The private "looks at the top of the library" note, Dig and Hideaway
  arranges (`dig_bottom`, `hideaway_bottom`, `hand`, `exile` kinds), and the
  view's `library_top` grant.
- The order of an arranged rest pile longer than one card. `History.Answers`
  records `Choices` but not `Intent.Rest`.
- Sideboards.

## Redeal fallback (stretch)

`RedealBase{Engine, Observer}` is the engine the seat is deciding in, plus its
collector at the same boundary. When `Sample` would return no worlds, it
builds `Worlds` worlds as follows:

- Each world is `Engine.CloneHypothetical(seed)`. This new rules API copies
  the engine with a fresh, recorded generator, so a world never inherits the
  real game's future randomness.
- Per seat, the unknown hidden cards are dealt uniformly into the hand's free
  slots and the library's free positions. Position-less known library cards
  are dealt into the free library positions too.
- Known top/bottom positions and known hand cards are pinned.
- Deals start from a canonical (name, object) order. Opponents' hands are
  rebuilt in canonical order, so neither which same-named copy is hidden nor
  the true hand order survives.
- All changes are Secret `MoveZone`/`LibraryOrder` events through
  `events.Emit` on the clone's own log. The live engine is never touched
  (tested: head and length unchanged after sampling and rollouts).
- It fails closed, and names the reason in `SampleResult.RedealRefused`, when:
  - the base is not at the observed boundary (re-captured board and decision
    must match the last frame);
  - the projection does not hold in the base;
  - the seat-derivable pool (deck list − cards seen outside hidden zones −
    known hidden cards) differs from the base's actual unknown multiset (this
    is how a composition leak is excluded);
  - the observed decision or stack refers to an unpinned hidden card;
  - a redealt world captures a different board.
- Two of these checks compare against the real engine. They only fire when the
  seat's own accounting is wrong, and cost a missing world, not a wrong one.
- Wiring: `searchseat.Options.Redeal` and `cmd/hindsight -redeal`, both off
  by default. Hindsight's JSONL gains `sampler_redealt` and
  `sampler_redeal_refused`, both `omitempty`. The report adds a redeal line
  only under `-redeal`.

### Smoke

Both runs used the same command, differing only in `-redeal`:

`hindsight -losses 4 -wins 2 -workers 3 -reliability-games 0
-omniscient-games 0 -max-rollouts 32 -block 16`

Default pairs; 2,172 recorded decisions, 216 with ≥2 candidates.

| run | no_world | ok | by turn (no_world/decisions) 1-3 · 4-6 · 7-12 · 13+ | clear |
|---|---:|---:|---|---:|
| `/mnt/sata/gorge-training/pn21/smoke-off/` | 196 | 20 | 0/13 · 9/12 · 62/66 · 125/125 | 0 |
| `/mnt/sata/gorge-training/pn21/smoke-on/` | 6 | 210 | 0/13 · 0/12 · 6/66 · 0/125 | 13 |

- 190 decisions used redealt worlds.
- The 6 remaining `no_world` were all refused with "observed frame refers to
  an unpinned hidden card". The decision or stack named an opponent hidden
  card the projection does not pin, e.g. a reveal-hand choice.

Caveat for whoever consumes this: a redealt world is uniform over the unknown
cards. It ignores the behavioural evidence rejection sampling would weigh
(e.g. "the opponent did not cast X when able"). So for most late decisions,
labels come from a coarser belief than the sampler's.

## Tests

`internal/searchprobe/known_test.go`

- Hand-built scenarios:
  - scry positions at the decision and after the answer
  - a shuffle wipes positions but keeps members, and an unseen draw then drops
    members
  - a draw of the known top moves it to the hand
  - an unseen draw of an opponent's known tucked card, with the bottom
    surviving in a larger library
  - a revealed hand card stays known, and is dropped on an unseen put-back or
    a cast
  - a bounce followed by a discard
  - an unmodelled size change forgets the zone
- `TestKnownCardsAreTrueInRealGames`: the truth-soundness sweep described in
  the TL;DR.
- `TestKnownCardsFromEngineEffects`: real engine scry, surveil, rearrange,
  bounce, tuck and search, with truth checked at every frame.
- `TestKnownCardsProjectionIgnoresHiddenState` (no-leak): two engines differ
  only in where the opponent's never-revealed card sits. They give identical
  histories and identical, non-trivial projections.
- `TestSampleKnownCardsConstraintHonouredAndInvisibleOnReplays` and
  `TestSampleKnownCardsOnRealDeckFixture`: with the constraint on, every world
  honours every claim, there are 0 violations, and the worlds are identical to
  flag-off.
- `TestKnownCardsHoldsDetectsViolations`: tampered worlds are caught.

`internal/searchprobe/redeal_test.go`

- On a starved bench fixture, the redeal:
  - pins every known card;
  - keeps each seat's multiset and zone sizes;
  - leaves the actor's hand untouched;
  - captures an identical board;
  - produces distinct deals;
  - plays hypothetically;
  - is deterministic;
  - leaves the live engine unchanged.
- The rearrange and bounce fixtures keep their pins.
- A redeal is identical across bases that differ only in hidden arrangement.

`rules/chance_test.go`

- `TestCloneHypotheticalReseedsWithoutTouchingSource`.

Gate:
- `go vet` on `./internal/searchprobe ./internal/searchseat ./internal/hindsight
  ./cmd/hindsight ./cmd/botbench ./view ./rules`
- `go test -count=1` on the same packages plus `./rules ./internal/archtest
  ./internal/testutil`: all ok, 0 SKIPs
- `go build ./...`
