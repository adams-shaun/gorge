# Seat-box "pill" visual requirements — fb-20260923T015756Z-c1c7d6d8

Status: **scoping**. No production code changes accompany this document. It
converts the reporter's report and sketch into explicit, testable visual
requirements for a follow-up implementation ticket (filed alongside as
`.ds4/new-tickets/seat-box-pills.md`).

## 1. The reporter's words (verbatim, unedited)

From `feedback/20260923T015756Z-c1c7d6d8/report.json`:

```
player boxes ugly

[ NAME | (HEALTH PILL) |  [BOOK] (LIB PILL) | [SKULL] (GY PILL) | [X-ICON] (EXILE PILL)
```

Received 2026-09-23T01:57:56Z, table `g2`, seat `0`, Chrome 151 on X11. The
report carries a replay snapshot (`match.json`, `log.json`, `view.json`,
`client.json`); the seat's own captured view is a **2-seat table** where seat 0
is `You` (life 20, library 49, hand 7, graveyard 2, exile 0) and seat 1 is a
redacted `Bot` (`hand: null`, graveyard/exile empty). So the rail under
complaint is two rows, and the Bot row is the redacted-count shape (counts, no
carets).

Behaviour at the report point: seat 0's hand/graveyard/exile lists are all
present, so **every actionable pile in the reporter's own row is a button**
(`data-pile`), and the counts are shown for all four zones.

## 2. What is NOT ambiguous

The reporter gave a concrete ordering, and the current component already
implements that ordering. Verified at this checkout against
`web/src/components/SeatTable.svelte`:

- name in `.who` → life in `[data-stat='life']` → `.zone-line` holding
  `hand`, `library`, `graveyard`, `exile` in that order.

So the complaint is **not** a missing icon/count sequence and **not** a
reordering request. Two things differ between the sketch and the current
render, and they are the whole substance of the report:

### 2.1 The sketch draws the counts as PILLS; the current render does not

This client already has exactly one "pill" recipe, in
`web/src/components/OracleText.svelte` (` .ic`): a rounded outline badge
(`border: 1px solid`, `border-radius: 999px`, `min-width: 12px`,
`height: 12px`, `padding: 0 2px`, `--font-ui`, `--t-10`, `font-weight: 600`).
`TableCell.svelte` also uses `border-radius: 999px` for its status dot.

`SeatTable.svelte`'s `.stat, .count, .pile` rule currently has **no** border,
background or radius — the counts are bare glyph+number runs, painted
`--ink-dim` (`--t-10`, `--font-data`). There is no pill chrome anywhere in the
seat row. The one rounded thing in the component is the `2px` tone ring on a
`.pile[data-tone]` (`fb-20260916T225802Z`), which is the card-tile register and
is deliberately a *ring*, not a pill.

### 2.2 The sketch omits HAND

The sketch names five things — NAME, HEALTH, LIB, GY, EXILE — and **hand is
not among them**. The current row renders hand first in `.zone-line`. Either:

- the reporter treats hand as a non-pill because the hand is also rendered
  elsewhere (the HandFan is the reading surface), or
- the reporter dropped it from the sketch by accident.

This cannot be resolved from the capture and must not be guessed.

## 3. Testable requirements (for the follow-up implementation ticket)

These are written so each maps to one assertion in
`web/src/components/SeatTable.svelte.test.ts` (a JSDOM `render()` assertion) or
to one geometry assertion in the existing `SeatTable — the rail floor` block
(playwright, `SeatTable.geometry.html`). Every value below is an existing
token, not an invented one.

R-1. **Each count gains a pill container.** The wrapper for each of life,
library, graveyard and exile (see the hand open question, §4) renders a pill:
`border-radius: 999px`, a 1px outline, and the count/icon inside it.
   - Assert: the rendered HTML for a count cell carries the pill class; the
     class rule in `<style>` sets `border-radius: 999px` and a `border`.
   - This is the only requirement that answers the word "pill".

R-2. **A pill's box is the OracleText `.ic` geometry**, not a new scale:
`min-width: 12px`, `height: 12px`, `padding: 0 2px`, font-size `--t-10`.
   - Assert: geometry (playwright) — a rendered pill's bounding box height is
     12px ± 1 and its min-width is at least 12px. Do not assert the exact
     width, which grows with the digit count.

R-3. **The icon stays adjacent to its own count inside the pill.** The sketch
draws `[BOOK] (LIB PILL)` — icon then its value together. This is already the
DOM order in every count cell (svg then `<span>`), so the requirement is that
the pill wraps *both*, rather than the count alone.
   - Assert: the element carrying `border-radius: 999px` contains the cell's
     `data-icon` svg as a descendant.

R-4. **The one-line contract is preserved.** Since `fb-20260917T232028Z` a
live seat row is one text-line tall and the rail's floor is the counts'
content; `SeatTable — the rail floor` pins `FLOOR_PX = 176` and asserts no
horizontal overflow of any rail child. Pills must not raise the row's height
or widen the counts past the floor.
   - Assert: the existing "fits every rail section horizontally at the 11rem
     floor" test still passes unchanged, and a new assertion pins the live-row
     height at the current measured value under the pill chrome.

R-5. **Actionability and tone rings are unchanged.** Hand/graveyard/exile stay
buttons when and only when their card lists are present; library stays a plain
count; the `data-tone` ring still appears for a pending decision. The pill is
chrome *around* the existing element, not a replacement for it.
   - Assert: the existing three compact-summary tests (five-cell order, one
     line, redacted-hides-caret) still pass unchanged; `data-pile` presence is
     unaffected.

R-6. **Colour/contrast:** the pill outline uses an existing ink token
(`--ink-dim` for the outline keeps the current quiet register). No new colour
is introduced.
   - Assert: the pill rule references a `var(--…)` token, and no raw hex
     colour is added to `SeatTable.svelte`'s `<style>`.

## 4. Open questions — the reporter's input is still required

The brief asked for three answers; the capture answers none of them. The
implementation ticket must not start until these are settled, because each
changes the diff:

1. **Hand** (§2.2): does the reporter want hand to get a pill too (making four
   pills), or to stay a bare count (three pills, matching the sketch
   literally)? *This changes R-1's scope and the row's width budget.*
2. **Pill fill:** outline-only (the OracleText `.ic` recipe) or a filled
   background? The sketch's parentheses read as an outline, but the reporter
   may have meant a filled chip. *Outline-only is assumed by R-6; a fill needs
   a token decision.*
3. **Does the reporter's complaint include anything the sketch does not
   show** — spacing, icon size, alignment? "Ugly" may name a property the
   sketch silently normalises. If the reporter can supply a marked-up
   screenshot, it supersedes this section.

Until 1–3 are answered, the requirements above are the *maximum* that can be
derived without guessing, and R-1/R-3 should be implemented as outline pills
covering life + library + graveyard + exile, leaving hand unpilled.

## 5. Verified current-state anchors

- `web/src/components/SeatTable.svelte` — `.who`/`.pick`/`.name`; life
  `[data-stat='life']`; `.zone-line` with the four `[data-stat]` cells;
  `.stat, .count, .pile` styling; `.pile[data-tone]` ring.
- `web/src/components/SeatTable.svelte.test.ts` — the file the follow-up
  extends (JSDOM order/actionability assertions + playwright geometry block).
- `web/src/components/SeatTable.geometry.html` / `.geometry.ts` — the rail
  fixture at `--rail-w`, `FLOOR_PX = 176`.
- `web/src/app.css` — `--ink-dim`, `--ink-faint`, `--instrument-raised`,
  `--initiative`, `--offered`, `--danger`, `--sp-1/2`, `--t-10`, `--radius`,
  `--font-ui`, `--font-data`.
- `web/src/components/OracleText.svelte` `.ic` — the client's existing pill
  recipe (the reference for R-2).
- `web/src/components/TableCell.svelte` `.state-live::before` — the other
  `border-radius: 999px` use.
- History: `bc190fe1` (one-line summary), `bb0e3ce7` (seat counts + pile
  modals).
