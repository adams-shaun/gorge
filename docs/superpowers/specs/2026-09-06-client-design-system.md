# gorge client — design plan

## Subject, audience, job

Legacy Magic played on a **transparent rules engine**. The player acts on their
own seat and must stay aware of the whole table. The engine's distinguishing
property, from `interface-comparison.md`, is that it *explains itself*: the
client holds no rules knowledge, every frame is self-sufficient, pending
triggers are observable before they hit the stack (which no surveyed platform
offers), and the whole match is a hash-chained log you can scrub.

Arena hides the rules to feel like magic. **gorge shows its working.** That is
the product, so it is the design.

## Thesis: felt and instrument

Two registers on one screen, deliberately unlike each other.

**The felt** — the board. Warm, dark, tactile. Card art dominant, chrome almost
absent. This is where the game is played.

**The instrument** — the rails. Cooler, flatter, denser. Hairline structure,
tabular figures, mono for data. This is where the engine explains what it did.

The seam between them is the identity. Most Magic clients are uniformly cool
blue-grey; a warm table felt against a cool instrument rail is specific to a
product whose selling point is that the machine is visible.

## Colour

Functional only. **Saturation means something**: mana, card colour, or seat
identity. Nothing decorative is saturated.

| Token | Value | Role |
|---|---|---|
| `--felt` | `#17150F` | board ground — warm near-black, green-brown cast |
| `--felt-raised` | `#1F1C15` | the player's own board band |
| `--instrument` | `#14161A` | rail ground — cooler, bluer, flat |
| `--edge-felt` | `#2A2721` | hairline on felt |
| `--edge-inst` | `#23262C` | hairline on instrument |
| `--ink` | `#E8E4DA` | text on felt — warm off-white |
| `--ink-inst` | `#C9CFD6` | text on instrument — cooler |
| `--ink-dim` | `#8A8579` | secondary |

Mana keeps its conventional identities, because colour is the sport's native
language and a player reads it instantly: W `#F5EEDB`, U `#4A8FD4`,
B `#5B4E6B`, R `#D0473E`, G `#4A9E63`, C `#9A958C`.

**Seat identity stays a distinct hue ramp, not deck colours.** Deriving seat
colour from deck colour identity is tempting and wrong: two blue decks at a
four-player table become indistinguishable, and seat identity must never be
ambiguous. The existing `SEAT_COLOURS` ramp is kept but desaturated to sit
inside this palette instead of the raw Radix values.

## Type

**IBM Plex Sans** and **IBM Plex Mono** — one family, two registers. Chosen
because Plex was drawn for technical material and carries a slight mechanical
squareness that suits an instrument, and because its tabular figures are real.
Not `system-ui` (the current default), not Inter.

- Interface and card text: Plex Sans.
- Data: Plex Mono — the transcript, chain heads, sequence numbers, counts.
  **Mono is for values, never for decorative small labels.**
- Life totals are the one place type is a visual element: large Plex Sans with
  tabular figures, read like a scoreboard.

Scale: 12 / 14 / 16 / 20 / 28 / 40. Sentence case throughout. No tracked-out
all-caps labels.

## Layout

Survey-measured proportions (board ~70%, hidden-info rail ~18%, identity ~11%),
adapted to "player seat primary, full-table awareness".

```
┌───────────────────────────────────────────┬─────────────┐
│  opponents — compact board strips         │   STACK     │
│  seat 2        seat 3        seat 4       │   top card  │
│                                           │   expanded, │
├───────────────────────────────────────────┤   rest dim  │
│                                           ├─────────────┤
│  YOUR BOARD                               │   PENDING   │
│    nonlands ─────────────                 │   about to  │
│    lands    ─────────────                 │   hit stack │
│                                           ├─────────────┤
├───────────────────────────────────────────┤   LOG       │
│  hand, fanned            status block     │   rules     │
└───────────────────────────────────────────┤   transcript│
                                            └─────────────┘
```

**The pending tray sits directly above the transcript and directly below the
stack, because it is literally what is about to become stack.** Spatial
adjacency encodes the mechanic rather than decorating it. Permanents group into
nonland and land sub-rows (survey #22) so a board stays parseable as it grows.

Alignment: board content centred within each seat's band; rail content left
aligned with figures right-aligned in their column.

## Principles

1. **Never make the player guess.** A derived number carries its provenance one
   hover away — base power, each counter, each effect and its source.
2. **Two registers, never blended.** Felt is warm and quiet; instrument is cool
   and dense.
3. **Colour is functional.** Saturated means mana, card colour, or seat.
4. **Spend the boldness on the pending tray.** It is the one thing no rival
   has. It gets the emphasis and the motion; everything else stays quiet.
5. **Motion answers actions.** The only unprompted motion is a permanent's ring
   when it changes (#21) and the pending → stack transition. No entrance
   animations, no hover transitions on everything.

## Reviewed against the generic defaults

- Not cream/serif/terracotta.
- Dark, but **two** distinct grounds with a functional five-colour palette, not
  one acid accent on tinted black. Dark is forced by the subject: card art is
  the hero and every surveyed client is dark.
- Hairlines mark the instrument register only, not applied everywhere.
- No identical rounded cards with a shared soft shadow; rails are flat panels
  divided by hairlines.
- No all-caps eyebrows, no middle-dot meta strings, no arrows appended to
  buttons, no mono for decorative labels.
