<script lang="ts">
  import type { PlayerView, SeatInfo } from '../protocol';
  import { zonesFor, countsFor, type ZoneName } from '../lib/zones';
  import { visibleHand } from '../lib/board';
  import { seatColour } from '../lib/colours';

  /** The zones this viewer can disclose a card list for: the two card-list
   *  zones plus hand (hand is a zone — CR 400.1 — and it gets the same
   *  name-disclosure treatment when this viewer may see the cards). */
  type DisclosureZone = ZoneName | 'hand';

  /**
   * ZoneViewer is the rail's per-seat zone browse. It shows EVERY seat's
   * zones (ui10, bug 3) — one collapsible panel per seat, so a viewer can
   * inspect any player's zones at any time, not only the focused seat's.
   * The rail is already dense, so each seat starts COLLAPSED as a one-line
   * summary — the seat name plus its four counts — and expands on demand.
   * The expanded panel is the established zone-row pattern: a row per zone —
   * hand, graveyard, exile, then the library count — each a count in mono
   * with an expand affordance that discloses the card names, most-recently-
   * added first. Library is count-only because the wire carries only
   * `library_size`, never the cards (a list is never invented for it).
   *
   * HAND IS A ZONE here (CR 400.1 lists hand among the seven), so it is one
   * of the rows, inside the seat group alongside graveyard, exile and
   * library — where the demo showed it floating above the group (ui10,
   * bug 2) rather than in it. Hand keeps the client-wide redaction rule
   * (lib/board.ts `visibleHand`): this viewer's own seat, and every seat on
   * an omniscient table, gets its revealed cards disclosed on expand; a
   * seat-scoped public view of someone else's hand ships the array as JSON
   * `null`, so that seat's hand is a COUNT only — never an empty list, never
   * an apology. The count (`hand_size`) is always on the wire and always
   * true; only the cards are withheld.
   *
   * Rows stay quiet until asked: a zero-count zone renders greyed with no
   * expander, and a zone whose cards were redacted (the array null while its
   * `_size` field is not) shows its count without a false expander. This
   * component renders fields already on the view and decides nothing about
   * the game.
   */
  let { players, seats = [], startOpen = false }: {
    players: PlayerView[];
    seats?: SeatInfo[];
    /** open every seat's panel and every zone disclosure on first render — the harness for the expanded-state assertions */
    startOpen?: boolean;
  } = $props();

  // Per-seat disclosure state, keyed by seat number. openSeat[seat] is
  // whether that seat's whole zone group is expanded; openZone[seat][zone]
  // is whether a card-list zone's names are disclosed within an expanded
  // group. startOpen seeds every seat/zone open, and — because each panel is
  // mounted per seat and the rail re-renders a live view — it is read as the
  // initial condition each time a seat first appears, not tracked forever.
  const openSeat = $state<Record<number, boolean>>({});
  const openZone = $state<Record<number, Partial<Record<DisclosureZone, boolean>>>>({});

  function seatExpanded(seat: number): boolean {
    return openSeat[seat] ?? startOpen;
  }
  function zoneExpanded(seat: number, z: DisclosureZone): boolean {
    return openZone[seat]?.[z] ?? startOpen;
  }
  function toggleSeat(seat: number): void {
    openSeat[seat] = !seatExpanded(seat);
  }
  function toggleZone(seat: number, z: DisclosureZone): void {
    const cur = openZone[seat] ?? {};
    openZone[seat] = { ...cur, [z]: !zoneExpanded(seat, z) };
  }

  function listId(seat: number, z: string): string {
    return `zone-list-${seat}-${z}`;
  }
  function panelId(seat: number): string {
    return `zones-${seat}`;
  }
</script>

{#each players as p (p.seat)}
  {@const counts = countsFor(p)}
  {@const hand = visibleHand(p)}
  {@const zones = zonesFor(p)}
  <section class="zones" style:border-left-color={seatColour(p.seat, seats)} data-seat={p.seat}>
    <button
      type="button"
      class="zone-row zone-toggle seat-toggle"
      aria-expanded={seatExpanded(p.seat)}
      aria-controls={panelId(p.seat)}
      onclick={() => toggleSeat(p.seat)}>
      <span class="zone-name">{p.name}'s zones</span>
      <span class="count">
        {counts.hand} hand &middot; {counts.graveyard} gy &middot; {counts.exile} ex &middot; {counts.library} lib
      </span>
      <span class="affordance" aria-hidden="true">{seatExpanded(p.seat) ? '▾' : '▸'}</span>
    </button>

    {#if seatExpanded(p.seat)}
      <div id={panelId(p.seat)} class="zone-panel">
        {#if hand === null}
          <!-- A hand this viewer may not see is a COUNT, not a sentence (B2):
               exactly the library/graveyard row shape — zone name, mono count —
               with no fabricated prose claiming the owner as subject. -->
          <div class="zone-row" data-hand-count>
            <span class="zone-name">hand</span>
            <span class="count">{p.hand_size}</span>
          </div>
        {:else if hand.length === 0}
          <div class="zone-row empty">
            <span class="zone-name">hand</span>
            <span class="count">0</span>
          </div>
        {:else}
          <button
            type="button"
            class="zone-row zone-toggle"
            aria-expanded={zoneExpanded(p.seat, 'hand')}
            aria-controls={listId(p.seat, 'hand')}
            onclick={() => toggleZone(p.seat, 'hand')}>
            <span class="zone-name">hand</span>
            <span class="count">{p.hand_size}</span>
            <span class="affordance" aria-hidden="true">{zoneExpanded(p.seat, 'hand') ? '▾' : '▸'}</span>
          </button>
          {#if zoneExpanded(p.seat, 'hand')}
            <ul id={listId(p.seat, 'hand')} class="zone-cards" data-zone="hand">
              {#each hand as c (c.id)}
                <li data-obj={c.id}>
                  <span class="card-name">{c.name}</span>
                  <span class="types">{c.types}</span>
                </li>
              {/each}
            </ul>
          {/if}
        {/if}

        {#each zones as z (z.zone)}
          {#if z.count === 0}
            <div class="zone-row empty"><span class="zone-name">{z.zone}</span><span class="count">0</span></div>
          {:else if z.cards.length === 0}
            <div class="zone-row"><span class="zone-name">{z.zone}</span><span class="count">{z.count}</span></div>
          {:else}
            <button
              type="button"
              class="zone-row zone-toggle"
              aria-expanded={zoneExpanded(p.seat, z.zone)}
              aria-controls={listId(p.seat, z.zone)}
              onclick={() => toggleZone(p.seat, z.zone)}>
              <span class="zone-name">{z.zone}</span>
              <span class="count">{z.count}</span>
              <span class="affordance" aria-hidden="true">{zoneExpanded(p.seat, z.zone) ? '▾' : '▸'}</span>
            </button>
            {#if zoneExpanded(p.seat, z.zone)}
              <ul id={listId(p.seat, z.zone)} class="zone-cards" data-zone={z.zone}>
                {#each z.cards as c (c.id)}
                  <li data-obj={c.id}>
                    <span class="card-name">{c.name}</span>
                    <span class="types">{c.types}</span>
                  </li>
                {/each}
              </ul>
            {/if}
          {/if}
        {/each}

        <div class="zone-row">
          <span class="zone-name">library</span>
          <span class="count">{p.library_size}</span>
        </div>
      </div>
    {/if}
  </section>
{/each}

<style>
  /* The instrument register, like the rest of the rail: hairlines and flat
     raised panels, never felt tokens. The seat colour is the only saturated
     thing, and it stays at the edge where the hand used to put it. */
  .zones {
    margin-bottom: 0.75rem;
    border-left: 3px solid transparent;
    padding-left: 0.5rem;
  }
  /* The seat header is the whole collapsed expression: name + the four
     counts on one line, with an expand affordance. It doubles as the toggle,
     so a collapsed seat is not a dead headline — it answers a click (quiet
     until asked). The counts group against the right edge, with the
     affordance immediately after them. */
  .seat-toggle {
    cursor: pointer;
    gap: 0.5rem;
  }
  .seat-toggle:hover .affordance {
    color: var(--ink-inst);
  }
  .seat-toggle .count {
    margin-left: auto;
  }
  .seat-toggle .affordance {
    flex: none;
    margin-left: 0.25rem;
  }
  .zone-panel {
    display: flex;
    flex-direction: column;
  }
  .zone-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    width: 100%;
    padding: 0.15rem 0;
    border: 0;
    background: none;
    font-size: var(--t-12);
    color: var(--ink-inst);
    text-align: left;
    cursor: pointer;
  }
  .zone-toggle:hover .affordance {
    color: var(--ink-inst);
  }
  .zone-row.empty {
    opacity: 0.45;
    cursor: default;
  }
  .zone-name {
    text-transform: capitalize;
  }
  /* Counts are values: mono, like every other figure on the rail. */
  .count {
    font-family: var(--font-data);
    font-variant-numeric: tabular-nums;
    color: var(--ink-faint);
  }
  /* In a zone row that is a toggle, the affordance owns the right edge; in
     one that is not (library, a redacted or zero hand) the count sits alone. */
  .zone-toggle:not(.seat-toggle) .affordance {
    margin-left: auto;
  }
  .zone-cards {
    margin: 0 0 0.35rem;
    padding: 0.25rem 0.5rem;
    list-style: none;
    background: var(--instrument-raised);
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    max-height: 10rem;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }
  .zone-cards li {
    display: flex;
    justify-content: space-between;
    gap: 0.5rem;
    font-size: var(--t-12);
    line-height: 1.35;
  }
  .card-name {
    color: var(--ink-inst);
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .types {
    color: var(--ink-faint);
    font-size: 0.6875rem;
    white-space: nowrap;
    flex: none;
    margin-left: auto;
  }
</style>
