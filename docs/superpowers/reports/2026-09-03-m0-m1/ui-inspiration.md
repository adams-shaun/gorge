# UI inspiration for the `gorge` browser client

> The frames this report cites (`*.png`, 47 files) are third-party stills from
> YouTube gameplay and are deliberately NOT in this repository. They live at
> `/home/sadams/gorge-ui-frames/` on the author's machine. Internal design study only.


**Design study — internal use only.** Every frame in `frames/` is a still extracted from a
publicly-posted YouTube video, kept solely as design evidence for this report. These images are
third-party copyrighted material (Wizards of the Coast card art and client UI, plus each channel's
video). **Do not publish, redistribute, or ship any of these frames.** Delete them when the study
is closed.

Method summary at the bottom. 14 videos surveyed; 31 frames kept, 16 of them annotated (each `.annotated.png` sits beside its unmodified original).

---

## Executive summary

**Patterns worth stealing**

1. **Type-label every stack object** — MTGO and XMage both render a stack entry as *source art +
   a coloured band reading "Triggered Ability" / "Activated Ability" / "Instant" + that ability's
   own text*, so "the card" and "the card's trigger" are never confusable.
   `mtgo-c2-Xraxh2sWCE-0726-zoom.png`, `xmage-c2-iWmJ1K_mO2o-1008.annotated.png` (2–4).
2. **Bind the parameters into the stack text.** Forge prints the resolved values inside the
   trigger — `…[Player: Judge Beard, Gained Amount: 4]`, `…[Zone Change … Traveler's Amulet (90)]`.
   Two copies of the same template become distinguishable. `forge-c3-d9SAIG5M3As-0743.annotated.png` (2).
3. **A four-line "where are we" status block**: *Priority: X / Turn: N (Y) / Phase: Z / Stack: N to
   Resolve.* Forge puts it dead-centre at the bottom and it answers every "is it me?" question at
   once. `forge-c8-d9SAIG5M3As-0732.annotated.png` (7).
4. **Per-player × per-step stop toggles as a 12-cell colour strip.** Forge gives each seat a
   vertical `UP DR M1 BC DA DB FS CD EC M2 ET CL` column, green = stop, red = skip. It scales to
   4 players as 4 columns. `forge-c8-d9SAIG5M3As-0732.annotated.png` (3, 4).
5. **Print the hotkey on the button.** XMage shows `TO NEXT TURN / F4`, `SKIP STACK / F10`,
   `CANCEL SKIP / F3` as real buttons, so every yield level is discoverable *and* fast.
   `xmage-c1-iWmJ1K_mO2o-0700.annotated.png` (6).
6. **A visible auto-yield manager.** Forge lists every ability you have muted, in full text, with
   *Disable All* and *Remove Yield*; you can also set a yield by right-clicking the object on the
   stack. `forge-c7-d9SAIG5M3As-0716.annotated.png`, `forge-c3-d9SAIG5M3As-0708.png`.
7. **Optional triggers as two asymmetric buttons.** Arena asks *"(Golos, Tireless Pilgrim) Search
   your library for a land card?"* with a cool **Decline** above a warm **Take Action**, while the
   full trigger text stays readable on the stack.
   `arena-c3-Cngnftg2npw-0320.annotated.png` (1–3).
8. **Explicit two-list "Resolve first" ordering**, with an **Auto** escape hatch and OK disabled
   until the order is complete. `forge-c3-d9SAIG5M3As-0743.annotated.png` (4, 8).
9. **Arrows carry relationship type.** XMage: red arrow = attacks (terminating *on the defending
   player's panel*), blue arrow = blocks/targets. This is the cheapest possible answer to
   "who is attacking whom" in multiplayer. `xmage-c4-iWmJ1K_mO2o-0900.annotated.png` (1, 9, 3).
10. **Object identity tokens in the log.** XMage suffixes every object with a short id —
    `Thermo-Alchemist [436]` vs `[3f2]` — so a log line about one of two identical permanents is
    unambiguous. `xmage-c2-iWmJ1K_mO2o-1008.annotated.png` (9).
11. **The log as a rules transcript, not a diff.** MTGO writes *"eruntalon84 puts triggered ability
    from Dismal Backwater onto the stack (When Dismal Backwater enters the battlefield, you gain 1
    life.)"* — source, event, and full text in one line. `mtgo-c6-Xraxh2sWCE-0806.png`.
12. **A rolling "what just happened" card history.** SpellTable keeps a *Last Card* pane plus a
    scrolling history of recently-played cards, rendered readable, beside a 4-seat grid.
    `spelltable-c8-CFwRZwqoy3A-0640.annotated.png` (9, 10).
13. **Colour-code the clauses of a long ability** when calling it out to spectators — Game Knights
    tints the three thresholds of a modal trigger blue/green/red. `coverage-c9-biWuhZQBs0-1900.png`.

**Anti-patterns observed**

- **Full-screen modals that hide the board.** Forge's *Select Order* covers the entire battlefield —
  yet trigger order almost always depends on board state. (Contrast Forge's own Auto-Yields sheet,
  which takes only the bottom half.) `forge-c3-d9SAIG5M3As-0743.annotated.png` (9).
- **Two counters that disagree.** In the same Forge frame the tab reads `Stack (1)` while the status
  bar reads `Stack: 2 to Resolve.` — one authority per fact.
  `forge-c8-d9SAIG5M3As-0732.annotated.png` (1, 7).
- **Auto-pass with an irreversible fast path.** MTGO's F6 is described by its own tutorial as
  "you have a very tiny amount of time to turn that off… if you hit that, you're out of luck"
  (`-Xraxh2sWCE` @12:43). A yield that cannot be revoked before it fires is a bug surface.
- **Dead battlefield.** XMage packs permanents from the top-left and never re-centres or re-scales;
  ~22 % of the frame is empty wallpaper while cards elsewhere are cropped.
  `xmage-c1-iWmJ1K_mO2o-0700.annotated.png` (11).
- **No persistent log at all.** Arena has no always-visible game log; when a trigger resolves during
  an auto-pass the only record is the animation you already missed.

**The single biggest lesson about screen real estate:** *nobody gives the opponent's board equal
space, and nobody should.* MTGO splits the two battlefields ~1 : 2 in favour of the local seat
(17 % vs 34 % of the frame). But when a real 4-player game arrives, MTGO **stops compressing and
starts tiling**: four equal ~12.5 % quadrants, each with its own player strip, clock and command
zone, and it moves the phase bar from horizontal to a vertical left rail to buy the width
(`mtgo-c8-5qzIh4WgXEU-1140.annotated.png`). A 4-player layout is not a 2-player layout with more
rows — it is a different layout, and the chrome has to rotate to make room.

---

## Comparison table

| | **Arena** | **MTGO** | **Forge** | **XMage** | **Coverage (PT / Game Knights)** | **SpellTable** | **Cockatrice** |
|---|---|---|---|---|---|---|---|
| **C1** auto-pass / stops | Auto-pass default; hold Ctrl = Full Control; button text *is* the state (`Resolve`/`Pass To Blockers`/`End Turn`) `arena-c1-BN9KIcO0xIg-0305.annotated.png` | Per-step stop carets under a horizontal phase bar; F2/F6 yields `mtgo-c1-Xraxh2sWCE-0646.annotated.png` (3) | Per-player 12-cell green/red step column, always visible `forge-c8-d9SAIG5M3As-0732.annotated.png` (3,4) | Vertical step rail + labelled yield buttons with printed hotkeys `xmage-c1-iWmJ1K_mO2o-0700.annotated.png` (3,6) | n/a (no player agency) | n/a (webcam) | Left icon rail of steps, purely manual `cockatrice-c10-GgmJBWnWhK8-0600.png` |
| **C2** stack display | Right-edge fan of card slivers; top object expands to art + *Ability* band `arena-c2-Cngnftg2npw-0220.annotated.png` | Floating resizable **"The Stack"** window over the board `mtgo-c2-Xraxh2sWCE-0726-zoom.png` | `Stack (N)` tab + overlay list, top-of-stack expanded, rest greyed `forge-c8-d9SAIG5M3As-0732.annotated.png` (1,2) | Horizontal row of tiles bottom-right, all fully readable `xmage-c2-iWmJ1K_mO2o-1008.annotated.png` (1) | Commentator voice + card callout `coverage-c9-biWuhZQBs0-1900.png` | *Last Card* pane `spelltable-c8-CFwRZwqoy3A-0640.annotated.png` (9) | n/a (no engine) |
| **C3** stack interaction | Click/hover the strip; chevron expands; **Decline / Take Action** for "may" `arena-c3-Cngnftg2npw-0320.annotated.png` | Click objects in the window; F-key yields | Right-click object → **Auto-Yield / Zoom-Details**; *Select Order* two-list `forge-c3-d9SAIG5M3As-0708.png`, `forge-c3-d9SAIG5M3As-0743.annotated.png` | Priority granted per object by default; **SKIP STACK F10** opts out `xmage-c1-iWmJ1K_mO2o-0700.annotated.png` (6) | n/a | n/a | n/a |
| **C4** combat | Attackers glow; phase pips advance; `Pass To Blockers` `arena-c1-BN9KIcO0xIg-0305.annotated.png` | Attack/Block/Damage are separate stops on the phase bar `mtgo-c1-Xraxh2sWCE-0806-phasebar.png` | `DA` / `DB` / `FS` / `CD` cells in the step column | **Red arrow → defending player's panel**, blue arrow = block; sword overlay on attackers `xmage-c4-iWmJ1K_mO2o-0900.annotated.png` (1,9) | Overhead camera; red midline splits the halves `coverage-c9-jRiSRDwaEE4-6100.annotated.png` (9) | Overhead camera per seat | Manual |
| **C5** prompts | Centre text names the source in parentheses; board stays visible `arena-c3-Cngnftg2npw-0320.annotated.png` (1) | Top-left prompt box: plain-English step + `OK` `mtgo-c1-Xraxh2sWCE-0646.annotated.png` (1) | Bottom status bar + half-height sheets; full-screen only for ordering | Prompt band shows remaining cost as mana symbols + spell name + `Cancel` `xmage-c5-iWmJ1K_mO2o-1000.png` | n/a | n/a | Manual counters per colour `cockatrice-c10-GgmJBWnWhK8-0600.png` |
| **C6** feedback | Comet trails on zone change, pulse glows, no log `arena-c6-bfb52ycYwnY-0524.png` | Log narrates each event with full ability text `mtgo-c6-Xraxh2sWCE-0806.png` | Magenta ring on freshly-changed permanents `forge-c6-d9SAIG5M3As-0730.png` | Colour-coded log, `T11.DB:` phase prefixes, object ids `xmage-c2-iWmJ1K_mO2o-1008.annotated.png` (9) | Card callout blown to ~40 % of frame `coverage-c9-biWuhZQBs0-1900.png` | Card-recognition history strip `spelltable-c8-CFwRZwqoy3A-0640.annotated.png` (10) | Timestamped message log |
| **C7** triggers | Source card + separate **"Ability"** sub-panel with the trigger text `arena-c7-BN9KIcO0xIg-0826.annotated.png` (1,2) | `Triggered Ability` band inside the stack window `mtgo-c2-Xraxh2sWCE-0726-zoom.png` | **Auto-Yields manager** listing every muted ability `forge-c7-d9SAIG5M3As-0716.annotated.png` | `Triggered Ability` red band per tile; log line `Ability triggers: …` `xmage-c2-iWmJ1K_mO2o-1008.annotated.png` (2,3) | n/a | n/a | n/a |
| **C8** multiplayer | Brawl is 1v1 only; no 4-player evidence found | **2×2 quadrants**, per-seat strip + clock + named **Command Zone** `mtgo-c8-5qzIh4WgXEU-1140.annotated.png` | One player block + step column per seat, stacked `forge-c8-d9SAIG5M3As-0732.annotated.png` (5,6) | Stacked player panels, green = has priority `xmage-c1-iWmJ1K_mO2o-0700.annotated.png` (1,2) | 2×2 colour-coded life grid, centre turn marker `coverage-c8-biWuhZQBs0-3300.annotated.png` | 2×2 webcam grid, life in the outer corners `spelltable-c8-CFwRZwqoy3A-0640.annotated.png` | n/a |
| **C9** spectator | Tournament broadcast client (not sampled directly) | Replay with `▷ ▷▷ ▷▷▷` transport `mtgo-c9-5qzIh4WgXEU-1140-replay.png` | n/a | Watchers supported (`/pings`); no distinct view sampled | **Both hands as text lists**, persistent name/life/archetype bars `coverage-c9-jRiSRDwaEE4-6100.annotated.png` | Card history + per-seat life `spelltable-c8-CFwRZwqoy3A-0640.annotated.png` | n/a |
| **C10** real estate | Board ~36 %, stack 8 %, hand 7 %, **no log**, ~40 % dead | Boards 51 %, log 12 %, hand 7 %, phase bar 3 % | Boards 71 %, hand 7 %, chrome 21 %, ~0 % dead | Board 40 % (**22 % of it empty**), log 13 %, chrome 25 % | Board 71 %, hidden-info rail 18 % | 4 seats × 21 % + 14 % rail | Board ~45 %, rails ~40 % |

---

## Per-platform findings

### MTG Arena

#### C1 — auto-pass, Full Control, and the button that doubles as a state readout

![Arena, combat priority under Full Control](frames/arena-c1-BN9KIcO0xIg-0305.annotated.png)

1. **The action button** — a single button, bottom-right, whose *label is the state*. Across three
   frames it reads `Resolve` (orange, an object is about to resolve),
   `Pass` with sub-label `To Blockers` (blue, you are responding on someone else's step),
   and `End Turn` sub-label `To End` (orange, your turn, nothing pending). Warm = you own the
   initiative, cool = you are merely being offered a window.
   Compare `arena-c1-BN9KIcO0xIg-0220.png` and `arena-c1-BN9KIcO0xIg-0532.png`.
2. **The phase-pip strip** immediately above/left of the button — six small pips with a sword icon
   for combat; the current step is lit. It is *attached to the button*, not to the board.
3. Opponent life plate, top-centre.
4. Opponent battlefield band.
5. Own battlefield band — attackers here carry a warm underline glow.
6. Own hand, fanned and overlapping.
7. Opponent avatar + library/graveyard piles.

*Dynamic behaviour (`BN9KIcO0xIg`, 1:51–3:20).* Default Arena auto-passes: the presenter's whole
complaint at 0:36–1:02 is "I was going to stop you… the game auto-passed." Holding **Ctrl** flips
into Full Control; a small `Ctrl` chip appears next to the hand (visible at 2:17–2:35, frame
`arena-c1-BN9KIcO0xIg-0220.png`) and stays lit for as long as the key is held. In that mode the button
steps you through every priority window one click at a time — 2:48 `Pass To Attackers`,
3:02 `Pass To Blockers`, 3:15 `Pass To Damage`. Releasing Ctrl immediately resumes auto-pass.
The mechanism is a *modifier*, not a setting: fast by default, exhaustive on demand.

#### C2/C7 — the stack as a fan, and triggers as a sub-panel

![Arena stack: fanned slivers with the top object expanded](frames/arena-c2-Cngnftg2npw-0220.annotated.png)

1. The whole stack region, right edge. 2. Five objects behind the top one, drawn as overlapping
card slivers ~12 px wide — you can count the stack without reading it. 3. The top object,
expanded: art, name bar, then a distinct dark **`Ability`** band carrying the *specific triggered
text* ("When this land enters, exile target opponent's graveyard"). 4/5. Life plates.
6. Opponent battlefield. 7. Own **nonland row**. 8. Own **land row** — Arena splits each seat's
battlefield into a nonland row above a land row, the same grouping MTGO and Forge use.
9. History chevrons.

![Arena: a triggered ability with its source and its target](frames/arena-c7-BN9KIcO0xIg-0826.annotated.png)

1. `Banishing Light` — the source card. 2. The **`Ability`** sub-panel below it, holding the
trigger's own text. Arena never shows a bare card on the stack and expects you to infer which of
its abilities is resolving. 3. The expand chevron. 4–7. Life, boards, hand. 8. Graveyard/exile
piles. 9. Opponent avatar.

*Dynamic behaviour (`BN9KIcO0xIg`, 8:18–8:32).* At 8:21 a **thick cyan bezier arc** springs from
the stack object and lands on the targeted permanent, which simultaneously gains a green ground
glow (`arena-c2-BN9KIcO0xIg-0821.png`). The arc persists for as long as the object sits on the
stack — it is not a one-shot animation, it is the *state* of "this object targets that". On
resolution the targeted permanent flies off the battlefield trailing an orange comet, and the
button changes to `Next / To Combat`. An hourglass appears bottom-right while the opponent thinks.

#### C3/C5 — optional triggers are an explicit two-button choice (R2)

![Arena asking a "may" trigger](frames/arena-c3-Cngnftg2npw-0320.annotated.png)

1. The question, centred over the board: **"(Golos, Tireless Pilgrim) Search your library for a
   land card?"** — the *source is named in parentheses* before the question.
2. **`Decline`** — cool violet, positioned *above*.
3. **`Take Action`** — warm orange, *below*, i.e. nearest the thumb/cursor.
4. The stack strip still shows the trigger with its complete oracle text while you answer, so the
   question and its authority are on screen together.
5. Opponent life plate. 6. Own life plate with the available-mana pips beneath it.
7. Own land row. 8. Own nonland row — the board is fully visible behind the question; the prompt
is text over the board, never a modal that hides it.

This is the best "may" prompt in the survey: **two labelled buttons, never a timeout, never an
implicit decline**, and the reason for the question is legible three ways at once (the parenthesised
source, the stack entry, the highlighted permanent).

For casting, Arena lifts the card out of hand, enlarges it with a cyan outline and floats it over
the hand while you pick targets (`arena-c5-Cngnftg2npw-0140.png`); mana is auto-tapped by default
("I tapped autopay", 5:06) and can be undone by manual tapping (5:32).

#### C6 — feedback without a log

`arena-c6-bfb52ycYwnY-0524.png`: permanents and lands that currently have an available action carry
a **blue/cyan outline**; that is the only "why can't I click this" signal — illegal things simply
have no glow. Zone changes are shown by cards physically flying with a comet trail; life changes
animate on the plate. There is **no persistent log**, which is the platform's biggest information
failure: an event you did not watch happen leaves no trace you can read.

---

### Magic Online (MTGO)

#### C1 — the phase bar with per-step stop carets

![MTGO board, upkeep, prompt box and phase bar](frames/mtgo-c1-Xraxh2sWCE-0646.annotated.png)

1. **The prompt box**, top-left: *"Upkeep step. Cast instants and activate abilities."* plus an
   `OK` button and a `▲` collapse toggle. Plain English naming the exact step and what you may do.
2. **The player panel**: avatar with life overprinted at ~40 px, library/hand badges above, four
   zone-count chips below. In a real game a second identical panel sits beneath it.
3. **The phase bar**: `Turn 1 | Untap | Upkeep | Draw | Main | Begin Com… | Attack | Block | Damage
   | End Combat | Main | End | Cleanup`, current step boxed in amber.
4. Opponent battlefield band. 5. Own battlefield band — note the ~1 : 2 height split.
6. Own hand, full-size non-overlapping cards, left-aligned. 7. Chat/log rail. 8. Settings/chat/yield.

![MTGO stop carets under every step](frames/mtgo-c1-Xraxh2sWCE-0806-phasebar.png)

The small white ▲ carets under Untap, Upkeep, Draw, Main, Begin Com, Attack, Block, Damage and
End Combat are the **stops**: a caret means "give me priority here". They are set per step and
(in the full client) separately for your turn and your opponent's.

*Dynamic behaviour (`-Xraxh2sWCE`, 6:20–13:12).* The tutorial's argument is worth recording because
it is the core tension of our C1: at 7:58 he asks *"why don't you just turn on all the stops?"* and
answers at 8:12 — *"you're stopping at every single step"* and the clock kills you. The practical
compromise he lands on (10:00–10:41) is: keep both main phases, drop the draw step, keep upkeep only
when a card needs it. That is an argument for **per-step, per-turn-owner toggles that a player can
tune**, not for a smart default. F2 = pass one priority (11:17), F6 = yield the rest of the turn
(12:09), and F6 is explicitly dangerous (12:43, see anti-patterns). Auto-yields are cleared by
right-clicking (12:50).

#### C2/C7 — the floating "The Stack" window

![MTGO: a triggered ability on the stack](frames/mtgo-c2-Xraxh2sWCE-0726.annotated.png)
![MTGO stack window, zoomed](frames/mtgo-c2-Xraxh2sWCE-0726-zoom.png)

1. The window itself — titled **"The Stack"**, draggable, **resizable** (grip bottom-right), and it
   only exists while the stack is non-empty. It floats over the battlefield rather than owning
   permanent real estate. 2. The source's art and gold name bar. 3. The **`Triggered Ability`**
   band and the ability's exact text on a parchment field. 4. Prompt box. 5. Log. 6. Phase bar.

The object carries a cyan halo when it is the top of the stack. This is the clearest
"card vs. ability" distinction found anywhere in the survey.

#### C6 — the log as a rules transcript

`mtgo-c6-Xraxh2sWCE-0806.png`. Representative lines:

> `eruntalon84 puts triggered ability from Dismal Backwater onto the stack (When Dismal Backwater
> enters the battlefield, you gain 1 life.).`
> `Turn 2: eruntalon84.`

Card names are hyperlinked (hover to preview). Turn boundaries are headers. Tapped permanents are
drawn rotated 90°, the physical convention. Own permanents auto-group into a creature sub-row above
a land sub-row.

#### C8/C9 — 4-player Commander, and replay

![MTGO 4-player Commander](frames/mtgo-c8-5qzIh4WgXEU-1140.annotated.png)

1–4. **Four quadrants**, one per seat, each with its own background tint so a permanent's owner is
   readable from colour alone. 5, 6. **Named command zones** — `Gosk's Command Zone`,
   `lifeandllamas's Command Zone` — bordered boxes with a title bar, docked *inside* their owner's
   quadrant, commander face-up (`mtgo-c8-5qzIh4WgXEU-1140-cmdzone.png`).
7. The **card preview pane**, permanently docked top-left, showing the hovered card at full
   readable size. 8. **"Waiting for meh1936."** — the priority readout, in words.
9. The **phase list rotated vertical** to buy horizontal width for four boards.
10. The log. 11. Own hand. 12. Replay transport. 13. That quadrant's own clock.

Each seat's strip carries avatar, big life (38 / 31 / 33 / 21) and a column of round zone-count
chips; each quadrant carries its own chess clock in a corner.

The log is doing the multiplayer heavy lifting:

> `being attacked by: Sylvan Primordial, Maelstrom Wanderer`
> `lifeandllamas has been dealt 9 total damage by RoyalAl's commander.`
> `RoyalAl puts triggered ability from Sword of Fire and Ice onto the stack targeting Azusa, Lost
> but Seeking (Sword of Fire and Ice deals 2 damage to target creature or player and you draw a card.).`
> `Turn 5: meh1936.`

— i.e. **commander damage is tracked per commander per player and announced in prose**, and turn
headers name the active player.

![MTGO replay transport](frames/mtgo-c9-5qzIh4WgXEU-1140-replay.png)

The replay control is a small translucent floating bar with `▷` (step), `▷▷` (play) and `▷▷▷`
(fast-forward), plus a `QUIT REPLAY` button in the title bar. **There is no timeline scrubber and no
visible step-back** — this is the weakest DVR in the survey and the clearest gap for us to beat.

---

### Forge

#### C8/C1/C2 — the densest, best-organised player seat found

![Forge: full board, per-player step columns, status block](frames/forge-c8-d9SAIG5M3As-0732.annotated.png)

1. The stack overlay (tab `Stack (2)` is active). 2. **The top-of-stack entry, expanded and bright**,
   with source art, name, object id `(142)` and the bound parameters `([Artificer's Dragon])
   [Phase: Slobad]`; the other entry is collapsed to one truncated line and greyed. Emphasis by
   *contrast*, not by position.
3, 4. **The per-player step columns** — `UP DR M1 BC DA DB FS CD EC M2 ET CL`, one 12-cell strip per
   seat, **green = stop here, red = skip**. Compact, always visible, individually clickable, and
   trivially extensible to 4 seats.
5, 6. **Per-player info blocks**: avatar, life (32 / 9), then icon-labelled counts for hand,
   graveyard, library, poison and other counters.
7. **The status block** — the most valuable four lines in the survey:
   `Priority: Judge Beard` / `Turn: 12 (Slobad)` / `Phase: End step` / `Stack: 2 to Resolve.`
8. Opponent battlefield. 9. Own battlefield. 10. **Own hand as a vertical strip on the right edge**
   (an unusual but space-efficient choice — it does not steal height from the boards).
11, 12. `OK` and `End Turn`, at opposite bottom corners.

Cards render as full mini card faces with readable text, a large P/T badge and small coloured
keyword icons in the corner; tapped permanents rotate 90°. Tabs carry live counts —
**`Stack (N)`** tells you the stack depth even when you are looking at the Log tab.

#### C3 — trigger ordering (R1) and per-object yields

![Forge: Reorder simultaneous abilities](frames/forge-c3-d9SAIG5M3As-0743.annotated.png)

1. **"Select Order"** — the pool of simultaneous triggers awaiting ordering.
2. One row: source thumbnail + full trigger text + **the bound values**
   `[Player: Judge Beard, Gained Amount: 4]`. Two "whenever you gain life" triggers from different
   sources are told apart by thumbnail, effect and magnitude.
3. `>` and `>>` transfer buttons (move selected / move all).
4. **"Resolve first"** — the ordered destination list you build.
5. `<` / `<<` to take items back.
6. `OK`, **disabled until the order is complete**.
7. The instruction line, *"Reorder simultaneous abilities"*.
8. **`Auto`** — accept the engine's default order. Essential: most orderings are irrelevant.
9. The dimmed tab bar — **and nothing else**: the dialog is full-screen, which is the flaw.

`forge-c3-d9SAIG5M3As-0708.png` shows the complementary interaction: **right-clicking an object on
the stack** opens a two-item menu — **`Auto-Yield` [checkbox]** and **`Zoom/Details`**. You mute a
trigger from the object itself, at the moment it annoys you.

#### C7 — the auto-yield manager

![Forge Auto-Yields sheet](frames/forge-c7-d9SAIG5M3As-0716.annotated.png)

1–3. Every ability currently muted, listed **in full text**. 4. `Disable All Auto Yields`.
5. `OK`. 6. Title. 7. **`Remove Yield`**. 8. The stack overlay, still live.
9. **The battlefield, still visible in the top half** — this dialog takes only the bottom ~50 %.

Yields here are not invisible state: you can enumerate them, read them, and revoke them. That is
the direct antidote to "a trigger resolved before I noticed it existed".

#### C6 — change highlighting

`forge-c6-d9SAIG5M3As-0730.png`: permanents that just entered the battlefield wear a **pulsing
magenta ring**. The status block reads `Stack: Empty` when nothing is pending — the absence of a
stack is stated, not merely implied by an empty panel.

---

### XMage

#### C1 — step rail, yield buttons with printed hotkeys, and "waiting for whom"

![XMage: begin combat, waiting for the opponent](frames/xmage-c1-iWmJ1K_mO2o-0700.annotated.png)

1. The active player's panel — **green header and border = this seat holds priority**.
2. The non-active player's panel, grey. Each panel carries a **per-player chess clock**
   (`00:28:17`) and a 14-cell grid of every zone count and counter type (life, hand, library,
   graveyard, exile, poison, experience, energy, commander…). It stacks to 4 seats unchanged.
3. **The vertical step rail** — `UNTAP UPKEEP DRAW MAIN1 START-COMBAT ATTACK BLOCK DAMAGE
   END-COMBAT MAIN2 END PASS` as icon+label buttons. 4. The current step, lit red.
5. **"Begin Combat — Waiting for Nibiru"** — the step and the blocking player, in one sentence,
   centred under the board with the opponent's name in colour.
6. **The yield button bar** — `TO NEXT TURN F4`, `TO END STEP F5`, `TO MAIN STEP F7`,
   `TO YOUR TURN F9`, `SKIP STACK F10`, `TO PRIOR END F11`, `CANCEL SKIP F3`, `CONCEDE`.
   Every yield is both a button and a hotkey, with the key printed on it, **and there is an explicit
   cancel**. 7. The shared battlefield. 8. Inline hotkey help in the chat pane. 9. The game log.
10. Own hand. 11. **Dead battlefield** — see anti-patterns.

*Dynamic behaviour (`iWmJ1K_mO2o`, 7:45–10:32).* XMage's default is the opposite of Arena's: it
"makes you click on each item on the stack if you don't have a skip on" (7:45). Maximally safe,
maximally R3-compliant, and tiring — which is why `SKIP STACK F10` exists (7:57–8:05) and why the
presenter reaches for it whenever the stack gets long (10:30).

#### C2/C7 — the stack as a row of type-labelled tiles

![XMage: four objects on the stack, each type-labelled](frames/xmage-c2-iWmJ1K_mO2o-1008.annotated.png)

1. The stack panel, bottom-right — a **horizontal row of tiles growing rightwards**, bottom of stack
   leftmost, every object fully readable at once.
2. **`Triggered Ability`** band (red) on the first trigger. 3. The same on the second — two
   simultaneous copies of the same Thermo-Alchemist trigger. 4. **`Activated Ability`** band, a
   different object kind, same treatment. The leftmost tile carries a blue **`Instant`** band.
5. The prompt band. 6. The yield bar. 7. The step rail. 8. The priority-holder panel.
9. The log — note `Thermo-Alchemist [436]` vs `Thermo-Alchemist [3f2]`: **short object identity
   tokens** disambiguate two identical permanents in prose. 10. Dead battlefield.

The log format is `12:25 AM, T11.FCD:` — wall clock, **turn number and phase code** (`M1`, `DA`,
`DB`, `FCD`), then the event. Ability triggers are logged as
`Ability triggers: Kitsune Dawnblade [035] - When Kitsune Dawnblade enters the battlefield, you may
tap target creature. - targeting Cathodion [e74]` — source, full text, **and target**.

#### C4 — combat as coloured arrows

![XMage: declare blockers, arrows and the stack](frames/xmage-c4-iWmJ1K_mO2o-0900.annotated.png)

1. **The red attack arrow terminating on the defending player's panel** — the arrowhead lands
   *on the player*, which is exactly how a 4-player client should show "who is being attacked".
2. Blue arrows from blockers to attackers, and from a stack object to its target — **arrow colour
   encodes relationship kind** (red = attacks, blue = blocks/targets).
3. The stack tile with its `Triggered Ability` band, at the far end of a blue target arrow.
4. **The prompt band**, dark red because it is the opponent's turn: *"Play instants and activated
   abilities."* with the sub-line *"Nibiru's turn / Declare Blockers"*. 5. `Done`.
6. Yield bar. 7. Step rail (BLOCK lit). 8. Log. 9. Attackers, with sword overlays.
10. The defending player's panel.

Log for the same moment: `T11.DB: Attacker: Sensei Golden-Tail [3e2] (2/1) blocked by Cathodion
[e74] (3/3)` and `T11.DB: Attacker: Kitsune Dawnblade [035] (2/3) unblocked` — every attacker's
blocked/unblocked state written out with P/T.

#### C5 — mana payment

`xmage-c5-iWmJ1K_mO2o-1000.png`: the prompt band becomes **`Pay ②💧`** (remaining cost as rendered
mana symbols) with the sub-line naming the spell and its object id, `Forbidden Alchemy [a9c]`, and
a `Cancel` button. Payable lands are outlined; you click them one at a time. The cost counts down
as you pay.

---

### Coverage / spectator (Pro Tour, Game Knights)

#### C9 — the Pro Tour feature-match overlay

![Pro Tour finals overlay](frames/coverage-c9-jRiSRDwaEE4-6100.annotated.png)

1. **`CARDS IN HAND`** — the player's *entire hand, as a text list*, each row with its mana cost
   rendered as symbols on the right. Text, not card images: far denser, and the viewer already
   knows what the cards do.
2. A short list of that player's cards of current interest.
3, 4. Player facecams, top and bottom, with country flags.
5, 6. **Persistent identity bars**: name + pronouns, top and bottom, never moving.
7, 8. **Life shields** — large numerals in the horizontal centre of each identity bar.
9. The board, an overhead camera on the mat, ~71 % of the frame; a **red midline** separates the
   two halves and printed grey slots mark empty zones so the spatial frame is stable.
10, 11. **Match-score dots** (● ○ ○) — one tiny widget per player.
12. Archetype name (`SELESNYA LANDFALL`) and round (`FINALS`).

The whole design is: *board in the middle, everything the players can't see in a rail down one
side, identity and life in fixed bars that never move.*

#### C6/C9 — the card callout

`coverage-c9-biWuhZQBs0-1900.png`: when a card matters, the broadcast blows it to ~40 % of the frame
over a blurred live board, and — the transferable idea — **colour-codes the clauses of its ability**:
"total toughness 10 or greater, draw a card" in blue, "20 or greater, untap each creature" in green,
"40 or greater, each opponent loses half their life" in red. A wall of oracle text becomes parseable
at a glance. The 4-player life widget stays visible behind it.

#### C8 — the 4-player life widget

![Game Knights 4-player life tracker](frames/coverage-c8-biWuhZQBs0-3300.annotated.png)

1. The whole widget. 2–5. **Four quadrants, one per seat, each in that seat's assigned colour**
   (red / purple / green / blue) with life in large numerals. 6. **A turn marker at the centre
   point** where the four quadrants meet, pointing at the active player.

It occupies roughly 3 % of the frame and conveys four life totals, four seat identities and whose
turn it is. This is the model for a multi-table overview cell.

---

### SpellTable

![SpellTable 4-seat grid](frames/spelltable-c8-CFwRZwqoy3A-0640.annotated.png)

1–4. **A 2×2 grid of per-seat feeds**, each ~21 % of the frame. Each cell has a header strip with
the player's name and pronouns, a *"No commander(s) set / Click to add commander(s)"* affordance,
and a `•••` overflow menu.
5–8. **The life totals radiate to the four outer corners** — top-left cell's 40 is top-left, the
top-right cell's is top-right, and so on — so they never collide with board content.
9. **`Last Card`** — the most recently identified card, rendered full-size and readable.
10. **`History`** — a scrolling column of previously played cards as mini cards with name and cost.
11. The header strip.

The `Last Card` + `History` pair is the best "what just happened" affordance in the survey and maps
directly onto our spectator requirement: a rolling, readable, chronological strip of the objects
that recently mattered, beside a grid of seats.

### Cockatrice

`cockatrice-c10-GgmJBWnWhK8-0600.png`. No rules engine, so everything is manual and the layout
reflects it: a left icon rail of steps, a player panel with life plus **per-colour mana-pool
counters you click up and down**, a plain battlefield, hand as a bottom row, and a right rail with
a card-info pane (with **`Image` / `Description` / `Both` tabs** — the viewer chooses what the
preview shows) over a timestamped message log that records manual counter changes
(`Player 1 sets counter Life to 22 (-1)`). Useful mainly as a lower bound: with no engine, ~40 % of
the frame goes to manual bookkeeping chrome.

---

## Screen real estate (C10), measured

All figures are the region's bounding box as a percentage of the frame, measured from the cited
frame with a 5 % grid overlay. "Dead space" = background visible inside a region nominally reserved
for content.

### Arena — `arena-c7-BN9KIcO0xIg-0826.png` (1v1)

| Region | x % | y % | % of frame |
|---|---|---|---|
| Opponent battlefield band | 5–90 | 18–40 | **18.7** |
| Own battlefield band | 5–92 | 58–78 | **17.4** |
| Stack strip (right edge) | 72–99 | 15–63 | **13.0** |
| Own hand (fanned, 4 cards) | 36–70 | 76–100 | 8.2 |
| Graveyard / exile piles | 78–95 | 63–90 | 4.6 |
| Opponent avatar + piles | 0–14 | 8–32 | 3.4 |
| Life plates (both) | — | — | ~3.0 |
| Action button + phase pips | 60–82 | 72–90 | 2.9 |
| Log / chat | — | — | **0** |
| Dead space (centre band, board margins) | — | — | **~35** |

Hand cards overlap ~60 %; ~7 fit before the fan tightens. Zoom is by hovering — the card
inflates in place; there is no docked preview pane, because the stack strip doubles as one.
Arena spends its space on *animation room*: the empty centre band exists so cards can fly through it.

### MTGO — `mtgo-c1-Xraxh2sWCE-0646.png` (1v1)

| Region | x % | y % | % of frame |
|---|---|---|---|
| Own battlefield | 13–86 | 29.5–77 | **34.4** |
| Opponent battlefield | 13–86 | 5.5–29 | **17.0** |
| Chat / game log | 86–100 | 5–88 | **11.6** |
| Own hand | 13–49 | 82–100 | 6.5 |
| Player panel (per seat) | 0–12.5 | 22.5–49 | 3.0 |
| Phase bar + stop carets | 12.5–87 | 78–82.5 | 3.0 |
| Prompt box | 0–12.5 | 5–22 | 2.0 |
| Card preview | — | — | 0 (hover tooltip) |
| Dead space (right of hand, left column) | — | — | ~10 |

Hand cards do **not** overlap; ~10 fit at full size before scaling. The two battlefields are in a
deliberate **1 : 2** ratio. Permanents auto-group into creature and land sub-rows.

### Forge — `forge-c8-d9SAIG5M3As-0732.png` (2 players)

| Region | x % | y % | % of frame |
|---|---|---|---|
| Opponent battlefield | 7.4–100 | 6–48 | **39.4** |
| Own battlefield | 7.4–83 | 48–90 | **31.8** |
| Own hand (vertical strip, right) | 83–100 | 48–90 | 7.1 |
| Stack overlay (over opponent board) | 65–100 | 6–28 | 7.7 |
| Left rail: player blocks + step columns | 0–7.4 | 6–90 | 6.2 |
| Tab bar (`Game / Players / Log / Stack (N)`) | 0–100 | 0–5.5 | 5.5 |
| Status block + corner buttons | 0–100 | 90–100 | 10.0 |
| Dead space | — | — | **~2** |

The most efficient layout measured: **71 % of the frame is battlefield**, chrome is 21 %, and there
is almost no dead space because the two bands stretch to fill. Cost: the log is behind a tab, so
you cannot watch the board and the log at once.

### XMage — `xmage-c1-iWmJ1K_mO2o-0700.png` (2 players)

| Region | x % | y % | % of frame |
|---|---|---|---|
| Battlefield (shared) | 7–78.5 | 8–64 | **40.4** — *of which ~22 pp is empty wallpaper* |
| Game log / chat | 84–99.5 | 7.5–96 | **13.3** |
| Top menu bar | 0–100 | 0–7.5 | 7.4 |
| Prompt band + `Done` | 0–53.5 | 66–79 | 7.0 |
| Stack panel | 53.5–83.5 | 76–96 | 6.2 |
| Own hand | 10–42 | 82–96+ | 4.5 |
| Player rail (2 panels) | 0.3–6.7 | 8.5–64 | 3.5 |
| Yield button bar | 53.5–83.5 | 66–76 | 2.9 |
| Step rail | 80.3–84 | 8.5–65 | 2.0 |

XMage reserves the most *chrome* (25 %) and wastes the most *board*: permanents pack from the
top-left and never re-centre, so the right half of the battlefield is empty while cards on the left
overlap and clip.

### MTGO 4-player Commander — `mtgo-c8-5qzIh4WgXEU-1140.png`

| Region | x % | y % | % of frame |
|---|---|---|---|
| Four battlefield quadrants (total) | 15.5–84 | 6–79 | **49.9** (each ~**12.5**) |
| Log / event rail | 84.5–100 | 6–86 | **12.6** |
| Card preview pane | 1–15 | 7–29 | 3.1 |
| Vertical phase rail | 1–15 | 53–96 | 5.9 |
| Own hand | 15.5–48 | 79–97 | 5.8 |
| Command zone (each) | ~6 × 25 of its quadrant | | ~1.5 each |
| Priority readout ("Waiting for …") | 1–15 | 44–52 | 1.1 |
| Per-quadrant clock (each) | — | — | <0.5 each |

**The key numbers for us.** Going from 2 to 4 players, MTGO does not shrink the opponent's board —
it re-tiles: each seat gets ~12.5 %, roughly a third of what the local seat got in 1v1, and the
phase bar rotates from a 3 %-tall horizontal strip to a 6 % vertical rail to free the width. The
log's share *grows* (11.6 % → 12.6 %) because with four seats the log is the only thing that can
tell you what happened off-screen.

### SpellTable 4-seat — `spelltable-c8-CFwRZwqoy3A-0640.png`

| Region | % of frame |
|---|---|
| Four seat feeds | **85** (each **21.3**) |
| `Last Card` + `History` rail | **14** |
| Per-seat header strip | ~1 each |
| Dead space | ~0 |

### Pro Tour coverage — `coverage-c9-jRiSRDwaEE4-6100.png`

| Region | % of frame |
|---|---|
| Board | **70.9** |
| Hidden-info rail (hands, facecams, card lists) | **18.0** |
| Identity bars (name + life + archetype) | **11.0** |
| Dead space | ~0 |

### Synthesis

**Where they agree.** (a) Every client puts the local seat's hand along one edge and never lets it
exceed ~8 % of the frame. (b) Every client with a log gives it 12–13 %, a remarkably stable number,
and puts it in a right-hand rail. (c) Every client puts step/phase state on a *rail* — horizontal
when there is spare height (MTGO 1v1), vertical when there is not (MTGO 4p, XMage, Forge,
Cockatrice). (d) The opponent's board is always smaller than yours in 1v1.

**Where they differ.** The stack. Arena reserves 13 % permanently at the right edge and lives with
a fan; Forge reserves 0 % and overlays 7.7 % on demand behind a counted tab; MTGO reserves 0 % and
floats a resizable window; XMage reserves 6.2 % permanently at the bottom. **The float-on-demand
approaches all win on density and lose on ambient awareness** — which is precisely why Forge puts
the count in the tab label (`Stack (2)`) and in the status block (`Stack: 2 to Resolve.`).

**What this implies for a 4-player board.** Four seats at MTGO's ~12.5 % each is the realistic
ceiling; that is about 4 × 6 permanents at readable size, which matches typical Commander boards
only if permanents auto-group (creature row / land row) the way MTGO and Forge do. The chrome must
be *rotatable*: a design that only works with a horizontal phase bar will not survive the fourth
seat. And the log must get *more* space, not less — it is the only device that reports the three
boards you are not looking at.

**What this implies for a multi-table overview.** SpellTable's 4-up grid at 21 % per cell is the
upper bound for "you can still read the boards"; the Game Knights life widget at ~3 % is the lower
bound for "you can still read the state". Between them sits the answer: an overview cell should be
a **~3–6 % state widget** (four colour-coded life totals + a turn marker + a stack-depth badge),
not a shrunken board, and clicking it should promote that table to the ~70 %-board focused view
that the Pro Tour overlay demonstrates.

---

## Spectator / coverage findings (C9)

Consolidated, because this is a first-class requirement for us.

**What a viewer gets that a player does not.**
- **All hands, as text lists with mana costs** — Pro Tour, left rail, ~24 % of the rail's height per
  player (`coverage-c9-jRiSRDwaEE4-6100.annotated.png` ①). Text beats card images here: seven cards
  fit in the space two images would take, and a viewer who knows the format only needs the name.
- **Archetype labels** per player, in the identity bar, so a viewer who joined mid-game has context.
- **Card callouts** with **colour-coded ability clauses** (`coverage-c9-biWuhZQBs0-1900.png`).
- **A rolling history of recently-played cards**, rendered readable (SpellTable's `History` column,
  `spelltable-c8-CFwRZwqoy3A-0640.annotated.png` ⑩).

**Always-visible state.** Every coverage layout fixes name + life in bars that never move
(Pro Tour: top and bottom, 11 % of the frame combined). Match score is a 3-dot widget. MTGO's
4-player view keeps a per-quadrant clock. Game Knights' physical tracker proves four life totals +
a turn marker fit in ~3 % of a frame if you colour-code the seats.

**Multi-table.** No platform in this survey shows several *games* at once. SpellTable's 2×2 shows
several *seats of one game*; that is the closest analogue and its lessons still apply — per-cell
header with identity, state pushed to the cell's outer corner so it never collides with content,
a shared detail rail on the right.

**Replay / scrub.** Only MTGO has one: a floating `▷ / ▷▷ / ▷▷▷` transport with no timeline and no
visible step-back (`mtgo-c9-5qzIh4WgXEU-1140-replay.png`). Nothing in the survey offers
event-granular stepping, which is what our engine's event stream makes cheap and what would
differentiate us immediately.

---

## Recommendations for gorge's client

Every item cites a frame or a described behaviour.

### (a) The player seat

**Auto-pass model**
1. **Default to stopping, and make skipping explicit and revocable.** XMage's default (priority on
   every stack object) is R3-correct; its escape hatch is `SKIP STACK F10` plus a `CANCEL SKIP F3`
   — *ship the cancel with the skip*. `xmage-c1-iWmJ1K_mO2o-0700.annotated.png` ⑥.
   Do not copy MTGO's F6, which its own tutorial calls unrecoverable (`-Xraxh2sWCE` @12:43).
2. **Per-seat × per-step stop toggles as a 12-cell colour strip**, green = stop, red = skip, one
   column per player. `forge-c8-d9SAIG5M3As-0732.annotated.png` ③④. Ship MTGO's tuned default
   (both mains on, draw off, upkeep off) but expose every cell — the tutorial's 10:00–10:41
   argument is that the right set is personal.
3. **A held modifier for total control**, Arena-style, in addition to the toggles: hold a key,
   get every priority window, release and resume. `BN9KIcO0xIg` @1:51–3:20.
4. **A four-line status block, centred, always present**:
   `Priority: … / Turn: N (…) / Phase: … / Stack: N to resolve.`
   `forge-c8-d9SAIG5M3As-0732.annotated.png` ⑦. Add XMage's sentence form,
   *"Begin Combat — Waiting for Nibiru"* (⑤ of `xmage-c1-iWmJ1K_mO2o-0700`), when the wait is on someone else.
5. **The primary button's label is the state** — `Resolve` / `Pass → Blockers` / `End Turn` — with
   warm colour when you hold the initiative and cool when you are merely being offered a window.
   `arena-c1-BN9KIcO0xIg-0305.annotated.png` ①, and compare `arena-c1-BN9KIcO0xIg-0220.png` / `arena-c1-BN9KIcO0xIg-0532.png`.
6. **A visible auto-yield manager** listing every muted ability in full text, with per-row
   *Remove Yield* and a global *Disable All*; plus set-a-yield from the object's own context menu.
   `forge-c7-d9SAIG5M3As-0716.annotated.png`, `forge-c3-d9SAIG5M3As-0708.png`.

**Stack (R3)**
7. **Type-label every stack object** with a coloured band: `Spell` / `Triggered Ability` /
   `Activated Ability`, above the *specific* ability text, below the source's art and name.
   `mtgo-c2-Xraxh2sWCE-0726-zoom.png`, `xmage-c2-iWmJ1K_mO2o-1008.annotated.png` ②③④.
8. **Interpolate the bound parameters into the text** — `[Gained Amount: 4]`,
   `[Zone Change … Traveler's Amulet (90)]`, `- targeting Cathodion [e74]`.
   `forge-c3-d9SAIG5M3As-0743.annotated.png` ②; XMage log, `xmage-c2-iWmJ1K_mO2o-1008.annotated.png` ⑨.
9. **Give every object a short identity token** (`[436]`) and use it in the stack, the log and the
   prompts, so two copies of a card are never confused. XMage, ibid.
10. **Emphasise the top of stack by contrast, not position**: expand it, dim the rest.
    `forge-c8-d9SAIG5M3As-0732.annotated.png` ①②.
11. **Always show the depth even when the panel is closed** — a counted tab (`Stack (2)`) *and* the
    status line. `forge-c8-d9SAIG5M3As-0732.annotated.png` ①⑦. (And keep the two numbers in sync — Forge
    fails this in that very frame.)
12. **Draw targeting as a persistent arc from the stack object to the target**, cyan, with a ground
    glow on the target; it is state, not an animation. `arena-c2-BN9KIcO0xIg-0821.png`.
    Use XMage's convention that arrow **colour encodes relationship kind**.
    `xmage-c4-iWmJ1K_mO2o-0900.annotated.png` ①②.
13. **Show pending triggers before they hit the stack** in the same visual language as stack
    objects, in a distinct "pending" tray adjacent to the stack, so R3's "will hit the stack" case
    reads identically to the "is on the stack" case. *No platform in the survey does this* — it is
    a genuine gap and a differentiator.

**Triggers (R1, R2)**
14. **R1 — ordering**: a two-list *Select Order → Resolve first* transfer widget with source
    thumbnails, bound parameters, an **`Auto`** button, and OK disabled until complete.
    `forge-c3-d9SAIG5M3As-0743.annotated.png`. **But fix Forge's flaw: do not take the full screen.**
    Use the half-height sheet Forge itself uses for Auto-Yields (`forge-c7-d9SAIG5M3As-0716.annotated.png` ⑨)
    so the board stays readable — trigger order usually depends on it.
15. **R2 — optional triggers**: two labelled buttons, `Decline` (cool, secondary) and
    `Take Action` (warm, primary), never a timeout, with the question naming the source in
    parentheses and the full trigger text visible on the stack behind it.
    `arena-c3-Cngnftg2npw-0320.annotated.png` ①②③④.

**Combat**
16. **Red arrow from each attacker terminating on the defending player's panel** — the only
    affordance in the survey that scales to "which of three opponents".
    `xmage-c4-iWmJ1K_mO2o-0900.annotated.png` ①. Blue for blocks and targets.
17. **State the combat sub-step in the prompt** — *"Nibiru's turn / Declare Blockers"* — and log
    each attacker's resolution (`Attacker: … (2/1) blocked by … (3/3)` / `unblocked`). ibid ④, ⑧.

**Prompts**
18. **Prompts are text over the board, not modals**, and always name the source: Arena's
    `(Golos, Tireless Pilgrim) Search your library for a land card?`
    `arena-c3-Cngnftg2npw-0320.annotated.png` ①. Reserve half-height sheets for list interactions.
19. **Mana payment**: show the *remaining* cost as rendered mana symbols with the spell named,
    auto-tap by default, allow manual tapping, always offer `Cancel`.
    `xmage-c5-iWmJ1K_mO2o-1000.png`; Arena autopay/undo, `BN9KIcO0xIg` @5:06–5:33.

**Feedback**
20. **Ship a persistent, always-visible log as a rules transcript** — source, event, full ability
    text, target, turn/phase prefix, hyperlinked card names, ~12–13 % of the frame in a right rail.
    `mtgo-c6-Xraxh2sWCE-0806.png`, `xmage-c2-iWmJ1K_mO2o-1008.annotated.png` ⑨. This is Arena's single
    biggest omission.
21. **Ring freshly-changed permanents** for a few seconds. `forge-c6-d9SAIG5M3As-0730.png`.
22. **Group permanents by type into sub-rows** (nonlands above, lands below) so a board stays
    parseable as it grows. Arena, MTGO and Forge all do this
    (`arena-c3-Cngnftg2npw-0320.annotated.png` ⑦⑧, `mtgo-c1-Xraxh2sWCE-0646.annotated.png` ⑤,
    `forge-c8-d9SAIG5M3As-0732.annotated.png` ⑨); XMage does not, and suffers for it.

### (b) The focused spectator table view + DVR scrubber

23. **Board ~70 % of the frame; hidden information in one rail (~18 %); identity and life in fixed
    bars that never move (~11 %).** `coverage-c9-jRiSRDwaEE4-6100.annotated.png`.
24. **Reveal all four hands as text lists with mana-cost symbols**, not card images — four hands fit
    in a rail only as text. ibid ①.
25. **Per-seat identity bar**: name, life, commander, commander-damage received. Anchor life in the
    horizontal centre of the bar like the Pro Tour shields (ibid ⑦⑧); anchor per-seat widgets to
    the seat's *outer* corner so they never collide with board content (SpellTable,
    `spelltable-c8-CFwRZwqoy3A-0640.annotated.png` ⑤–⑧).
26. **A rolling "recently mattered" strip**: the last resolved object rendered large, then a
    scrolling history beneath it. `spelltable-c8-CFwRZwqoy3A-0640.annotated.png` ⑨⑩. This is the
    spectator's answer to "what just happened" and it is cheap for us — it is just the event stream.
27. **Colour-code the clauses of long abilities** in callouts. `coverage-c9-biWuhZQBs0-1900.png`.
28. **DVR: beat MTGO decisively.** MTGO offers only `▷ / ▷▷ / ▷▷▷` with no timeline and no step-back
    (`mtgo-c9-5qzIh4WgXEU-1140-replay.png`). Ship: a **timeline scrubber marked with turn
    boundaries** (the log already produces `Turn 5: meh1936.` headers — MTGO, XMage and Forge all
    emit them), **event-granular step forward/back**, a **live/paused indicator**, and a
    **"return to live"** button. Anchor each scrubber tick to a log event so the log and the board
    scrub together.

### (c) The multi-table overview

29. **Each table is a state widget, not a shrunken board.** The Game Knights tracker proves four
    life totals + seat colours + a turn marker fit in ~3 % of a frame and stay legible;
    a shrunken 4-player board at that size would not.
    `coverage-c8-biWuhZQBs0-3300.annotated.png`.
30. **Cell contents**: a 2×2 colour-coded life grid (one colour per seat, held consistent when the
    table is focused), a centre turn marker (ibid ⑥), the table's name/id, and a **stack-depth
    badge** borrowed from Forge's counted tab (`Stack (2)`,
    `forge-c8-d9SAIG5M3As-0732.annotated.png` ①) so a viewer can see *which table has something
    happening*.
31. **Grid, don't compress.** MTGO's 2→4 player transition is a re-tile, not a shrink
    (`mtgo-c8-5qzIh4WgXEU-1140.annotated.png`); SpellTable's 2×2 at 21 % per cell is the point where
    boards stop being readable (`spelltable-c8-CFwRZwqoy3A-0640.annotated.png`). So: below ~20 % per cell,
    show state widgets; above it, show boards; never show a board at 5 %.
32. **Clicking a cell promotes it to the focused view of (b)**, and the promoted view keeps the seat
    colours the overview used, so identity carries across.
33. **A shared right rail across the overview** carrying a merged, table-tagged event feed — the
    same 12–13 % the single-table log gets, since with N tables the log is even more the only thing
    that can tell you what happened off-screen.

---

## Sources appendix

| id | URL | channel | title | duration | timestamps pulled |
|---|---|---|---|---|---|
| `BN9KIcO0xIg` | https://www.youtube.com/watch?v=BN9KIcO0xIg | Jumpstart My MTG Heart | How to Use Full Control \| MTG Arena \| What I Wish I Knew #10 | 9:49 | 0:40, 2:12, 2:20, 2:30, 2:52, 3:05, 3:18, 5:02–5:12, 5:32, 8:10, 8:18–8:32 |
| `bfb52ycYwnY` | https://www.youtube.com/watch?v=bfb52ycYwnY | TheSkarTV | MTG Arena Shortcuts and What They Do \| Beginner Guide | 10:46 | 2:24, 3:36, 5:24, 5:44, 6:02, 6:28, 7:10, 7:30, 9:00 |
| `Cngnftg2npw` | https://www.youtube.com/watch?v=Cngnftg2npw | Deckshift | You Won't Believe How This Omnath Combo Wins… (No Commentary) | 14:54 | 1:40, 1:58, 2:02, 2:20, 3:20 |
| `STKeiAY4CBg` | https://www.youtube.com/watch?v=STKeiAY4CBg | eMpTyG | Beginners Guide to Brawl \| How To Play Commander on Arena | 8:32 | 1:20, 2:40, 3:20, 4:00 (Brawl is 1v1; no 4-player Arena evidence) |
| `-Xraxh2sWCE` | https://www.youtube.com/watch?v=-Xraxh2sWCE | The Mana Leek | Magic The Gathering Online Tutorial #3: Gameplay, Stops, and Hot Keys | 17:00 | 6:26, 6:46, 7:26, 8:06, 8:34, 10:20, 11:20, 12:12, 12:54 |
| `vfZydr8Hwe8` | https://www.youtube.com/watch?v=vfZydr8Hwe8 | HarryMTG | NEW Storm Deck Kills Turn 1 in Legacy! (Gameplay) | 9:31 | scanned by contact sheet only |
| `5qzIh4WgXEU` | https://www.youtube.com/watch?v=5qzIh4WgXEU | RoyalAl | Multiplayer Commander! Maelstrom Wanderer v Prime Speaker Zegana v Ghave v Borborygmos | 13:07 | 1:00, 5:00, 8:20, 10:40, 11:40, 12:40 |
| `iWmJ1K_mO2o` | https://www.youtube.com/watch?v=iWmJ1K_mO2o | XMage Draft Historical Society | XMage UI Tutorial | 20:51 | 5:20, 7:00, 7:20, 8:20, 8:40, 9:00, 9:20, 9:52–10:18, 10:00, 11:00, 11:40, 12:40 |
| `d9SAIG5M3As` | https://www.youtube.com/watch?v=d9SAIG5M3As | Judge Beard | The Best Single Player MTG Game? – MTG Forge Adventure | 22:59 | 6:38, 6:42, 7:04, 7:08, 7:16, 7:28, 7:30, 7:32, 7:43, 9:35, 10:00, 10:25, 10:50 |
| `mrnblrKWfOQ` | https://www.youtube.com/watch?v=mrnblrKWfOQ | MTG Phil | Playing Old School Magic 93/94 on Your Phone – Forge | 20:54 | contact-sheet scan (mobile Forge; `Select Order` dialog visible ~10:00) |
| `jRiSRDwaEE4` | https://www.youtube.com/watch?v=jRiSRDwaEE4 | Play MTG | Final \| Christoffer Larsen vs. Nathan Steuer \| Standard \| #PTSOS | 1:54:44 | 46:00, 60:00, 61:00 |
| `-biWuhZQBs0` | https://www.youtube.com/watch?v=-biWuhZQBs0 | The Command Zone | Tarkir: Dragonstorm w/ The Professor & AliasV \| Game Knights 77 | 1:00:57 | 19:00, 32:00, 33:00, 34:00 |
| `CFwRZwqoy3A` | https://www.youtube.com/watch?v=CFwRZwqoy3A | Tolarian Community College | A Guide To Playing Commander Online With Spelltable | 14:50 | 5:20, 5:40, 6:40 |
| `GgmJBWnWhK8` | https://www.youtube.com/watch?v=GgmJBWnWhK8 | Kage_Okami | How to Play Commander/EDH ONLINE with Cockatrice | 9:39 | 5:20, 6:00, 7:00 |

Auto-caption transcripts were pulled for `BN9KIcO0xIg`, `bfb52ycYwnY`, `-Xraxh2sWCE`, `iWmJ1K_mO2o`
and used to locate the timestamps above; quoted narration in this report comes from those captions.

---

## Method and limits

**Method.** Videos were located by `yt-dlp` metadata search, downloaded video-only at 480p
(720p/1080p for text-dense UIs: MTGO, XMage, Forge, coverage), scanned with 4×4 contact sheets at
one thumbnail per 20–60 s to locate moments, then sampled as full frames and short 1 fps bursts
around dynamic events. Auto-captions were grepped for *trigger / stack / stop / yield / full
control / pass / may / priority* to find teaching moments cheaply. Measurements in C10 were taken by
overlaying a 5 % coordinate grid on the cited frame and reading bounding boxes off it; figures are
accurate to roughly ±1 percentage point and describe *that frame*, not the client in general.
Annotations were drawn with Pillow; originals sit beside every `.annotated.png`. All video files
were deleted after extraction. This survey was done entirely in one session by one agent, in three
passes (search → capture → measure/annotate); no subagents were dispatched.

**What I could not see.**
- **No 4-player MTG Arena footage exists** — Arena has never shipped multiplayer; Brawl
  (`STKeiAY4CBg`) is 1v1. Arena's C8 row is genuinely n/a, not unsampled.
- **Arena's tournament/spectator broadcast client** was not sampled directly; the coverage evidence
  here is the paper Pro Tour overlay (`jRiSRDwaEE4`), which is a physical-table camera composite,
  not a digital spectator client.
- **Forge's and XMage's 4-player layouts were not observed.** Forge's per-seat step column and
  XMage's stacked player panels obviously extend to four seats, and I say so, but that extension is
  *inferred from the 2-player frames*, not seen. The only 4-player digital layout actually observed
  is MTGO's (`5qzIh4WgXEU`).
- **XMage's trigger-ordering dialog was not captured.** The two simultaneous triggers in
  `xmage-c2-iWmJ1K_mO2o-1008.annotated.png` are *identical*, so XMage stacked them without asking.
  XMage is known to have an ordering dialog; I have no frame of it. R1 evidence is Forge's only.
- **No platform was observed showing multiple concurrent games.** Recommendation (c) is therefore
  extrapolated from single-game multi-seat layouts (SpellTable, MTGO 4p) and from the Game Knights
  life widget, and is the least evidence-backed section of this report.
- **Untap.in and Tabletop Simulator were not sampled** (budget); Duels of the Planeswalkers was not
  sampled.
- **Inferred rather than observed**: MTGO's separate your-turn/opponent-turn stop rows (the tutorial
  describes them at 6:34–6:44; the frame shows only one caret row); Forge's greyed-vs-bright stack
  entries being a top-of-stack emphasis rather than a hover effect (consistent across
  `forge-c8-d9SAIG5M3As-0732` and `forge-c3-d9SAIG5M3As-0708`, but never seen changing); the meaning of MTGO's
  per-quadrant background tints (colour-coding by seat is the obvious reading, not a stated one).
