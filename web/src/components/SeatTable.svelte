<script lang="ts">
  import type { CardView, PlayerView, SeatInfo, View } from '../protocol';
  import { visibleHand } from '../lib/board';
  import { seatColour } from '../lib/colours';
  import { lossCauses, seatRows, stateLabel, type SeatState } from '../lib/seattable';
  import { zonesFor, type ZoneName } from '../lib/zones';
  import PileModal from './PileModal.svelte';

  type PileZone = ZoneName | 'hand';
  type OpenPile = { seat: number; zone: PileZone; trigger: HTMLButtonElement };

  /**
   * SeatTable is the rail's single per-seat summary. Each row preserves the
   * established seat-colour, active, priority and loss language while putting
   * all five public counts in one compact icon strip. Hand, graveyard and
   * exile become buttons only when this view actually carries cards for them;
   * their lists open in PileModal rather than expanding the rail.
   */
  let { view, seats = [], focus = null, onFocus, events = [] }: {
    view: View;
    seats?: SeatInfo[];
    focus?: number | null;
    onFocus: (seat: number) => void;
    events?: { event: { kind: string; player: number; text?: string } }[];
  } = $props();

  const rows = $derived(seatRows(view, seats, lossCauses(events)));
  let openPile = $state<OpenPile | null>(null);
  const modalPlayer = $derived(openPile === null ? null : (view.players.find((p) => p.seat === openPile?.seat) ?? null));
  const modalCards = $derived(openPile === null || modalPlayer === null ? [] : cardsFor(modalPlayer, openPile.zone));
  function possessive(name: string): string {
    return name === 'You' ? 'Your' : `${name}'s`;
  }
  const modalTitle = $derived(openPile === null ? '' : `${possessive(rows.find((r) => r.seat === openPile?.seat)?.name ?? `Seat ${openPile.seat}`)} ${openPile.zone}`);

  function describe(name: string, deck: string | null, state: SeatState, lostReason: string | null): string {
    const parts = [name];
    if (deck) parts.push(deck);
    if (state === 'lost') parts.push(lostReason ? `eliminated — ${lostReason}` : 'eliminated');
    else {
      const word = stateLabel(state);
      if (word) parts.push(word);
    }
    return parts.join(' — ');
  }

  function cardsFor(player: PlayerView, zone: PileZone): CardView[] {
    if (zone === 'hand') return visibleHand(player) ?? [];
    return zonesFor(player).find((summary) => summary.zone === zone)?.cards ?? [];
  }

  function showPile(seat: number, zone: PileZone, event: MouseEvent): void {
    openPile = { seat, zone, trigger: event.currentTarget as HTMLButtonElement };
  }

  function pileLabel(name: string, zone: PileZone, count: number): string {
    return `View ${name}'s ${zone} (${count} ${count === 1 ? 'card' : 'cards'})`;
  }
</script>

<section class="seats" data-seat-table>
  <table>
    <caption class="sr-only">Seats: life, hand, library, graveyard and exile</caption>
    <tbody>
      {#each rows as r (r.seat)}
        {@const player = view.players.find((p) => p.seat === r.seat)}
        {@const handCards = player ? cardsFor(player, 'hand') : []}
        {@const graveyardCards = player ? cardsFor(player, 'graveyard') : []}
        {@const exileCards = player ? cardsFor(player, 'exile') : []}
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
              title={`${describe(r.name, r.deck, r.state, r.lostReason)} — press to focus this seat`}
              aria-label={describe(r.name, r.deck, r.state, r.lostReason)}
              onclick={() => onFocus(r.seat)}
            >
              <span class="name" class:priority={r.priority}>{r.name}</span>
            </button>
            {#if r.lost}
              <p class="eliminated" data-eliminated>
                <span class="eliminated__tag">Eliminated</span>{#if r.lostReason}<span class="eliminated__cause"> — {r.lostReason}</span>{/if}
              </p>
            {/if}
          </th>

          <td data-stat="life" aria-label={`Life: ${r.life}`}>
            <span class="stat life">
              <svg data-icon="heart" viewBox="0 0 16 16" aria-hidden="true"><path d="M8 14S2 10.2 2 5.6C2 2.4 6 1.4 8 4c2-2.6 6-1.6 6 1.6C14 10.2 8 14 8 14Z"/></svg>
              <span>{r.life}</span>
            </span>
          </td>
          <td data-stat="hand" data-hand-hidden={r.handVisible ? undefined : ''} aria-label={`Hand: ${r.hand}`}>
            {#if r.hand > 0 && handCards.length > 0}
              <button type="button" class="pile" data-pile="hand" aria-label={pileLabel(r.name, 'hand', r.hand)} onclick={(e) => showPile(r.seat, 'hand', e)}>
                <svg data-icon="hand" viewBox="0 0 16 16" aria-hidden="true"><path d="M3 8V4.5a1 1 0 0 1 2 0V7 3.5a1 1 0 0 1 2 0V7 3a1 1 0 0 1 2 0v4-3a1 1 0 0 1 2 0v4.2l.7-.7a1.2 1.2 0 0 1 1.7 1.7L11 12.6A4 4 0 0 1 8 14H7a4 4 0 0 1-4-4V8Z"/></svg>
                <span>{r.hand}</span><svg class="caret" viewBox="0 0 8 12" aria-hidden="true"><path d="m2 2 4 4-4 4"/></svg>
              </button>
            {:else}
              <span class="count"><svg data-icon="hand" viewBox="0 0 16 16" aria-hidden="true"><path d="M3 8V4.5a1 1 0 0 1 2 0V7 3.5a1 1 0 0 1 2 0V7 3a1 1 0 0 1 2 0v4-3a1 1 0 0 1 2 0v4.2l.7-.7a1.2 1.2 0 0 1 1.7 1.7L11 12.6A4 4 0 0 1 8 14H7a4 4 0 0 1-4-4V8Z"/></svg><span>{r.hand}</span></span>
            {/if}
          </td>
          <td data-stat="library" aria-label={`Library: ${r.library}`}>
            <span class="count"><svg data-icon="book" viewBox="0 0 16 16" aria-hidden="true"><path d="M2 3.2C4 2.6 6 3 8 4v9c-2-1-4-1.4-6-.8v-9Zm12 0c-2-.6-4-.2-6 .8v9c2-1 4-1.4 6-.8v-9Z"/></svg><span>{r.library}</span></span>
          </td>
          <td data-stat="graveyard" aria-label={`Graveyard: ${r.graveyard}`}>
            {#if r.graveyard > 0 && graveyardCards.length > 0}
              <button type="button" class="pile" data-pile="graveyard" aria-label={pileLabel(r.name, 'graveyard', r.graveyard)} onclick={(e) => showPile(r.seat, 'graveyard', e)}>
                <svg data-icon="skull" viewBox="0 0 16 16" aria-hidden="true"><path d="M3 7a5 5 0 1 1 10 0c0 2-1 3-2 3.8V14H5v-3.2C4 10 3 9 3 7Zm3-1.5a1 1 0 1 0 0 2 1 1 0 0 0 0-2Zm4 0a1 1 0 1 0 0 2 1 1 0 0 0 0-2ZM7 9l1-1 1 1-1 1-1-1Z"/></svg>
                <span>{r.graveyard}</span><svg class="caret" viewBox="0 0 8 12" aria-hidden="true"><path d="m2 2 4 4-4 4"/></svg>
              </button>
            {:else}
              <span class="count"><svg data-icon="skull" viewBox="0 0 16 16" aria-hidden="true"><path d="M3 7a5 5 0 1 1 10 0c0 2-1 3-2 3.8V14H5v-3.2C4 10 3 9 3 7Zm3-1.5a1 1 0 1 0 0 2 1 1 0 0 0 0-2Zm4 0a1 1 0 1 0 0 2 1 1 0 0 0 0-2ZM7 9l1-1 1 1-1 1-1-1Z"/></svg><span>{r.graveyard}</span></span>
            {/if}
          </td>
          <td data-stat="exile" aria-label={`Exile: ${r.exile}`}>
            {#if r.exile > 0 && exileCards.length > 0}
              <button type="button" class="pile" data-pile="exile" aria-label={pileLabel(r.name, 'exile', r.exile)} onclick={(e) => showPile(r.seat, 'exile', e)}>
                <svg data-icon="exile" viewBox="0 0 16 16" aria-hidden="true"><path d="m3 3 10 10M13 3 3 13"/></svg>
                <span>{r.exile}</span><svg class="caret" viewBox="0 0 8 12" aria-hidden="true"><path d="m2 2 4 4-4 4"/></svg>
              </button>
            {:else}
              <span class="count"><svg data-icon="exile" viewBox="0 0 16 16" aria-hidden="true"><path d="m3 3 10 10M13 3 3 13"/></svg><span>{r.exile}</span></span>
            {/if}
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
</section>

<PileModal
  open={openPile !== null}
  title={modalTitle}
  cards={modalCards}
  returnFocus={openPile?.trigger ?? null}
  onClose={() => (openPile = null)}
/>

<style>
  table { width: 100%; border-collapse: collapse; table-layout: fixed; }
  .sr-only { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
  th.who { width: auto; font-weight: 400; padding: 0; }
  td { padding: 1px 0; color: var(--ink-inst); white-space: nowrap; }
  td:nth-child(2) { width: 2.45rem; }
  td:nth-child(3), td:nth-child(4), td:nth-child(5), td:nth-child(6) { width: 2.25rem; }
  .pick { display: flex; align-items: center; width: 100%; background: none; border: 0; border-left: 3px solid var(--seat); padding: 1px var(--sp-2); font-size: var(--t-12); line-height: 1.6; color: var(--ink-inst); text-align: left; cursor: pointer; }
  tr.active .pick { border-left-width: 6px; padding-left: calc(var(--sp-2) - 3px); }
  tr.selected .pick, .pick:hover { background: var(--instrument-raised); }
  tr.selected .pick { color: var(--ink); }
  .name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .name.priority { color: var(--initiative); text-decoration: underline 2px dotted; text-underline-offset: 0.16em; }
  .stat, .count, .pile { display: flex; align-items: center; justify-content: flex-end; gap: 0.16rem; font-family: var(--font-data); font-size: var(--t-11); font-variant-numeric: tabular-nums; color: var(--ink-dim); }
  .life { padding-right: var(--sp-1); font-size: var(--t-14); font-weight: 600; color: var(--ink-inst); }
  .stat svg, .count svg, .pile svg { width: 0.72rem; height: 0.72rem; flex: none; fill: currentColor; stroke: currentColor; stroke-width: 1.6; stroke-linecap: round; stroke-linejoin: round; }
  .caret { width: 0.36rem; fill: none; }
  .pile { width: 100%; border: 0; padding: 0; background: none; cursor: pointer; }
  .pile:hover { color: var(--ink-inst); }
  [data-hand-hidden] { color: var(--ink-faint); }
  tr.lost .name, tr.lost .life { text-decoration: line-through; color: var(--ink-faint); }
  .eliminated { margin: 1px 0 0; padding-left: var(--sp-2); font-size: var(--t-10); color: var(--danger); line-height: 1.3; }
  .eliminated__tag { font-weight: 700; text-transform: uppercase; letter-spacing: 0.02em; }
  .eliminated__cause { color: color-mix(in srgb, var(--danger) 82%, var(--ink-inst)); }
</style>
