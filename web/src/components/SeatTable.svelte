<script lang="ts">
  import type { SeatInfo, View } from '../protocol';
  import { seatColour } from '../lib/colours';
  import { lossCauses, seatRows, stateLabel, type SeatState } from '../lib/seattable';

  /**
   * SeatTable is the rail's one table across seats (U4). Rows are seats;
   * columns are the facts every seat has, so the reader compares DOWN a
   * column — which is the whole reason four separate panels were the wrong
   * shape: they made comparing two life totals a scroll.
   *
   * Columns are life, hand, library and graveyard: the four numbers that are
   * both always present and always worth comparing. Exile is not a column —
   * it is usually zero and, when it is not, the reader wants the cards rather
   * than the count, which is what the detail pane below the table is for.
   *
   * State is drawn in the vocabulary IdentityBar already uses on the felt, so
   * it is learned once: the seat's colour is a left rule, the rule GROWS on
   * the active seat, priority is the initiative-coloured dot, and a seat that
   * has lost is struck through. No pill, no second colour, no shouting; the
   * words themselves live in each row's accessible name.
   *
   * The seat cell is a button because the table also drives the detail pane:
   * pressing a row focuses that seat's hand and zones beneath.
   */
  let { view, seats = [], focus = null, onFocus, events = [] }: {
    view: View; seats?: SeatInfo[]; focus?: number | null; onFocus: (seat: number) => void;
    /** the DVR's own event list (EventBody-shaped), read only for player_lost causes (seattable.ts's lossCauses); optional so every existing caller keeps working with no cause shown. */
    events?: { event: { kind: string; player: number; text?: string } }[];
  } = $props();

  const rows = $derived(seatRows(view, seats, lossCauses(events)));

  // The row says its state with a rule and a dot; the words go here, where a
  // screen reader and a hover both find them, so the table stays quiet.
  function describe(name: string, deck: string | null, state: SeatState, lostReason: string | null): string {
    const parts = [name];
    if (deck) parts.push(deck);
    if (state === 'lost') {
      parts.push(lostReason ? `eliminated — ${lostReason}` : 'eliminated');
    } else {
      const word = stateLabel(state);
      if (word) parts.push(word);
    }
    return parts.join(' — ');
  }
</script>

<section class="seats" data-seat-table>
  <table>
    <caption class="sr-only">Seats: life, hand, library and graveyard</caption>
    <thead>
      <tr>
        <th scope="col" class="who">Seat</th>
        <th scope="col">Life</th>
        <th scope="col">Hand</th>
        <th scope="col"><abbr title="Library">Lib</abbr></th>
        <th scope="col">Grave</th>
      </tr>
    </thead>
    <tbody>
      {#each rows as r (r.seat)}
        <tr
          data-seat-row={r.seat}
          data-state={r.state}
          class:lost={r.lost}
          class:active={r.active}
          class:selected={focus === r.seat}
          style={`--seat:${r.colour || seatColour(r.seat, seats)}`}
        >
          <th scope="row" class="who">
            <button
              type="button"
              class="pick"
              aria-pressed={focus === r.seat}
              title={`${describe(r.name, r.deck, r.state, r.lostReason)} — press to show this seat's hand and zones`}
              aria-label={describe(r.name, r.deck, r.state, r.lostReason)}
              onclick={() => onFocus(r.seat)}
            >
              {#if r.priority}<span class="dot" aria-hidden="true"></span>{/if}
              <span class="name">{r.name}</span>
            </button>
            {#if r.lost}
              <p class="eliminated" data-eliminated>
                <span class="eliminated__tag">Eliminated</span>{#if r.lostReason}<span class="eliminated__cause"> — {r.lostReason}</span>{/if}
              </p>
            {/if}
          </th>
          <td class="life">{r.life}</td>
          <td class="data num" data-hand-hidden={r.handVisible ? undefined : ''}>{r.hand}</td>
          <td class="data num">{r.library}</td>
          <td class="data num">{r.graveyard}</td>
        </tr>
      {/each}
    </tbody>
  </table>
</section>

<style>
  /*
   * The instrument register: hairlines, no boxes, figures right-aligned in
   * their column (design system, "Layout"). One rule under the header row is
   * the only border — a grid of cell borders would read as a spreadsheet, and
   * four rows do not need to be told apart by lines.
   */
  table {
    width: 100%;
    border-collapse: collapse;
    table-layout: fixed;
  }
  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }
  thead th {
    font-size: var(--t-10);
    font-weight: 400;
    color: var(--ink-faint);
    text-align: right;
    padding: 0 var(--sp-1) var(--sp-1) 0;
    border-bottom: 1px solid var(--edge-inst);
  }
  thead th.who {
    text-align: left;
    padding-right: 0;
  }
  /* Abbreviated only where the full word would cost the seat name its width;
     the title carries the word for anyone who needs it. */
  abbr {
    text-decoration: none;
  }
  th.who {
    width: auto;
    font-weight: 400;
    padding: 0;
  }
  td {
    padding: 1px 0;
    text-align: right;
    color: var(--ink-inst);
    white-space: nowrap;
  }
  /* Fixed layout, and the figure columns are sized to their widest real
     value rather than to a round number: the rail is ~250px of content and
     every pixel a column does not need belongs to the seat name, which is
     the one cell whose content is unbounded. Life is three characters wide
     because life can go negative before the state-based action lands. */
  thead th:nth-child(2) {
    width: 1.9rem;
  }
  thead th:nth-child(3) {
    width: 1.7rem;
  }
  thead th:nth-child(4) {
    width: 1.9rem;
  }
  thead th:nth-child(5) {
    width: 2.2rem;
  }
  .pick {
    display: flex;
    align-items: center;
    gap: 0.35em;
    width: 100%;
    background: none;
    border: 0;
    border-left: 3px solid var(--seat);
    padding: 1px var(--sp-2);
    font-size: var(--t-12);
    line-height: 1.6;
    color: var(--ink-inst);
    text-align: left;
    cursor: pointer;
  }
  /* Active is the rule growing, exactly as it is on the felt — not a second
     colour competing with the seat's own. */
  tr.active .pick {
    border-left-width: 6px;
    padding-left: calc(var(--sp-2) - 3px);
  }
  tr.selected .pick,
  .pick:hover {
    background: var(--instrument-raised);
  }
  tr.selected .pick {
    color: var(--ink);
  }
  /* Priority is the initiative, and the initiative has its own colour in this
     palette; repeating the seat hue would say "seat" twice. */
  .dot {
    width: 0.4em;
    height: 0.4em;
    border-radius: 999px;
    background: var(--initiative);
    flex: none;
  }
  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  /* Life is the one place type is a visual element rather than a label
     (design system): interface face, semibold, tabular — a scoreboard column
     read down rather than a value read across. */
  .life {
    font-size: var(--t-14);
    font-weight: 600;
    font-variant-numeric: tabular-nums;
    padding-right: var(--sp-1);
  }
  .num {
    font-size: var(--t-11);
    color: var(--ink-dim);
    padding-right: var(--sp-1);
  }
  /* A hand whose cards this viewer may not see still publishes its size. The
     figure is true, so it is shown — dimmed, because it is the one number in
     the row the reader cannot open. */
  [data-hand-hidden] {
    color: var(--ink-faint);
  }
  tr.lost .name,
  tr.lost .life {
    text-decoration: line-through;
    color: var(--ink-faint);
  }
  /* A strikethrough alone reads as "unremarkable" at a glance -- exactly the
     complaint (survey: "their health doesn't reflect 0", i.e. a dead seat
     was easy to miss). Forcing life to 0 would state something false (a
     commander-damage or empty-library loss can happen at any life total), so
     the fix is a loud WORD instead: danger-coloured, bold, naming the cause
     the engine actually gave (PlayerLost's own Text) when this client has
     seen it. */
  .eliminated {
    margin: 1px 0 0;
    padding-left: var(--sp-2);
    font-size: var(--t-10);
    color: var(--danger);
    line-height: 1.3;
  }
  .eliminated__tag {
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.02em;
  }
  .eliminated__cause {
    color: color-mix(in srgb, var(--danger) 82%, var(--ink-inst));
  }
</style>
