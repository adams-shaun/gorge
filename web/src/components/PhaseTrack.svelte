<script lang="ts">
  import type { SeatInfo, View } from '../protocol';
  import { STEPS, type Stops, type TurnSide } from '../lib/autopilot';
  import { PHASE_GROUPS } from '../lib/phases';
  import { seatColour } from '../lib/colours';

  /**
   * PhaseTrack is the board's clock: whose turn it is, which round of the
   * table, and where in the twelve steps the game is standing right now —
   * plus the seat's stops, set by clicking the step you want to be handed
   * the game back at.
   *
   * The headline number is view.round, the round-trip counter projected by
   * the server (one full pass of the surviving seats), NOT view.turn, which
   * is the engine's per-player-turn counter and increments once per seat's
   * turn. The transcript still says "Turn N" because that is what the
   * engine actually did; the board's clock agrees with a viewer that a
   * four-seat table is on round 2 when the starting player takes their
   * second turn.
   *
   * It decides nothing about the rules. The step order and the stoppable
   * set come from lib/autopilot via lib/phases (the client has exactly one
   * step vocabulary), the current step is view.step verbatim, and a stop is
   * a callback to the seat's own state — this component never reads or
   * writes storage and never posts.
   *
   * The shard's perimeter says one thing only: the active seat's identity
   * colour. It is full-strength when that seat is yours and recedes when it
   * is not; the current-step fill keeps the existing initiative grammar.
   */
  let { view, seats, seat = null, stops = null, onToggle = null }: {
    view: View;
    seats: SeatInfo[];
    /** seat is the viewer's own seat, or null for a spectator / a finished replay. */
    seat?: number | null;
    /** stops is the viewer's stop set. Null means nobody owns stops here, and every cell is display-only. */
    stops?: Stops | null;
    /** onToggle is called with the step and the turn side the stop belongs to. */
    onToggle?: ((step: string, side: TurnSide) => void) | null;
  } = $props();

  // Only a seated viewer with a stop set can set stops. A spectator and a
  // frozen replay get the same track with nothing focusable on it, rather
  // than buttons that quietly do nothing.
  const settable = $derived(seat !== null && stops !== null && onToggle !== null);
  const yours = $derived(seat !== null && view.active === seat);
  const side = $derived<TurnSide>(yours ? 'yours' : 'opponents');
  const other = $derived<TurnSide>(yours ? 'opponents' : 'yours');
  // The track names whose turn it is. The seat list is the first word (it
  // carries the authoritative registered name when the table knows one), but
  // on the live table route `seats` can be EMPTY while the view still carries
  // each player's own name (the one-shot seed in Table.svelte races the
  // async tables.lookup — see the ui15 report). Falling straight through to
  // `Seat ${view.active}` then prints a 0-based index beside the 1-based
  // `Player N` names everywhere else, so the wire's player name is the
  // fallback, exactly as IdentityBar does.
  const activeName = $derived(
    seats[view.active]?.name ?? view.players.find((p) => p.seat === view.active)?.name ?? `Seat ${view.active}`,
  );
  const activeColour = $derived(seatColour(view.active, seats));
  // STEPS is a const tuple, so its own indexOf only accepts the twelve
  // literals; view.step is whatever the wire said. Widened once here so an
  // unknown step is -1 rather than a type error.
  const ORDER: readonly string[] = STEPS;
  const at = $derived(ORDER.indexOf(view.step));
  // The set on screen is the one a click here would edit: the side whose
  // turn it is. The other side's stops are a different intention and are
  // deliberately not drawn on the same cells.
  const shown = $derived<ReadonlySet<string>>(stops === null ? new Set<string>() : stops[side]);

  /** position tells a cell whether it is behind, on, or ahead of the game. -1 for a step the wire named but STEPS does not know. */
  function position(step: string): 'past' | 'now' | 'future' {
    const i = ORDER.indexOf(step);
    if (at < 0 || i < 0) return 'future';
    if (i < at) return 'past';
    return i === at ? 'now' : 'future';
  }
</script>

<div
  class="phase-track"
  class:yours
  data-phase-track
  style={`--seat:${activeColour}`}
  aria-label="Round and phase"
>
  <div class="head">
    <span class="whose" data-whose>{yours ? 'Your turn' : `${activeName}’s turn`}</span>
    <span class="turn"><span class="tk">Round</span><span class="tn">{view.round}</span></span>
    {#if settable}
      <span class="hint" data-hint>
        Click a step to stop there on {yours ? 'your' : 'this'} turn · Shift-click to set it on the other side
      </span>
    {/if}
  </div>

  <div class="groups">
    {#each PHASE_GROUPS as g (g.key)}
      <!-- Each group takes width in proportion to the steps it holds, so
           every cell is the same width and the track reads as one twelve-step
           timeline cut into five labelled sections — not five equal boxes,
           one of which happens to contain five steps. -->
      <div class="group" data-group={g.key} style={`flex-grow:${g.steps.length}`}>
        <!-- A one-step phase is named by its own cell; repeating the phase
             name above it says the same word twice. The empty caption keeps
             the cells on one baseline. -->
        {#if g.steps.length > 1}
          <span class="glabel">{g.label}</span>
        {:else}
          <span class="glabel" aria-hidden="true">&nbsp;</span>
        {/if}
        <div class="cells">
          {#each g.steps as c (c.step)}
            {@const pos = position(c.step)}
            {#if c.stoppable && settable}
              {@const on = shown.has(c.step)}
              <button
                class="cell"
                class:past={pos === 'past'}
                class:now={pos === 'now'}
                type="button"
                data-step={c.step}
                aria-pressed={on}
                aria-current={pos === 'now' ? 'step' : undefined}
                title={`${c.label} — ${on ? 'stop set' : 'no stop'} on ${yours ? 'your' : 'this'} turn. Shift-click for the other side.`}
                onclick={(e) => onToggle?.(c.step, e.shiftKey ? other : side)}
              >
                <span class="label">{c.label}</span>
                <span class="mark" aria-hidden="true"></span>
              </button>
            {:else}
              <span
                class="cell"
                class:inert={!c.stoppable}
                class:past={pos === 'past'}
                class:now={pos === 'now'}
                data-step={c.step}
                data-inert={!c.stoppable ? '' : undefined}
                aria-current={pos === 'now' ? 'step' : undefined}
              >
                <span class="label">{c.label}</span>
              </span>
            {/if}
          {/each}
        </div>
      </div>
    {/each}
  </div>
</div>

<style>
  /*
   * The instrument register: cool, flat, hairline-divided. The track is a
   * compact shard floating over felt, not a page-wide band or row of buttons — the phase groups are separated by rules
   * rather than by gaps, so `beginning`, `combat` and `ending` read as units
   * at a glance and the twelve cells never read as twelve equal boxes.
   */
  .phase-track {
    display: flex;
    flex-direction: column;
    width: 100%;
    background: color-mix(in srgb, var(--instrument) 94%, transparent);
    color: var(--ink-inst);
    border: 1px solid color-mix(in srgb, var(--seat) 34%, var(--edge-inst));
    border-radius: var(--radius);
    backdrop-filter: blur(8px);
    overflow: hidden;
    /* The one state colour on the track: warm when the initiative is yours. */
    --now: var(--offered);
  }
  .phase-track.yours {
    --now: var(--initiative);
    border: 2px solid var(--seat);
  }

  .head {
    display: flex;
    align-items: baseline;
    gap: var(--sp-3);
    padding: var(--sp-1) var(--sp-3);
    border-bottom: 1px solid var(--edge-inst);
    min-width: 0;
  }
  /* Whose turn it is, in that seat's identity colour — the same colour the
     identity bar and the life grid already use for them, so nobody has to
     work out which one they are. */
  .whose {
    font-size: var(--t-14);
    font-weight: 600;
    color: var(--seat);
    white-space: nowrap;
  }
  .turn {
    display: inline-flex;
    align-items: baseline;
    gap: 0.35em;
    white-space: nowrap;
  }
  .tk {
    font-size: 0.6875rem;
    color: var(--ink-faint);
  }
  .tn {
    font-family: var(--font-data);
    font-variant-numeric: tabular-nums;
    font-size: var(--t-14);
    color: var(--ink);
  }
  /* The modifier is stated, not hidden: an undiscoverable shortcut is not a
     feature. It is the first thing dropped when the board is narrow. */
  .hint {
    margin-left: auto;
    font-size: 0.6875rem;
    color: var(--ink-faint);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }

  /* The track scrolls inside itself on a narrow board rather than pushing
     the page sideways. */
  .groups {
    display: flex;
    align-items: stretch;
    overflow-x: auto;
    scrollbar-width: thin;
  }
  .group {
    display: flex;
    flex-direction: column;
    gap: 1px;
    padding: 2px var(--sp-1) var(--sp-1);
    border-right: 1px solid var(--edge-inst);
    flex: 1 1 0;
    min-width: 0;
  }
  .group:last-child {
    border-right: 0;
  }
  .glabel {
    font-size: 0.6875rem;
    line-height: 1.2;
    color: var(--ink-faint);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    white-space: nowrap;
  }
  .cells {
    display: flex;
    gap: 1px;
  }

  .cell {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    flex: 1 1 auto;
    min-width: 0;
    padding: 2px var(--sp-1) 6px;
    background: var(--instrument-raised);
    border: 0;
    border-bottom: 2px solid transparent;
    border-radius: var(--radius);
    font-family: var(--font-ui);
    font-size: var(--t-12);
    line-height: 1.25;
    color: var(--ink-dim);
    white-space: nowrap;
  }
  button.cell {
    cursor: pointer;
  }
  button.cell:hover {
    background: color-mix(in srgb, var(--ink) 8%, var(--instrument-raised));
    color: var(--ink);
  }
  /* Passed steps recede; steps still to come are neutral. */
  .cell.past {
    background: transparent;
    color: var(--ink-faint);
  }
  /* The current step is unmistakable: it is the only filled cell on the
     track, and its fill is the state colour. */
  .cell.now {
    background: var(--now);
    color: var(--felt-sunk);
    font-weight: 600;
  }
  .cell.now .label {
    color: var(--felt-sunk);
  }
  /* untap and cleanup grant no priority, so a stop there would mean nothing.
     They are drawn, because the track is the engine's twelve steps and
     hiding two of them would be a lie, but they are visibly inert. */
  .cell.inert {
    background: transparent;
    color: var(--ink-faint);
    opacity: 0.55;
    font-size: 0.6875rem;
  }

  /* A stop is a rule under the cell — the same "something is marked here"
     grammar the rail and the identity bars use, not a decorative badge. */
  .cell[aria-pressed='true'] {
    border-bottom-color: var(--initiative);
    color: var(--ink);
  }
  .mark {
    position: absolute;
    left: 50%;
    bottom: 4px;
    width: 0;
    height: 0;
    transform: translateX(-50%);
  }
  .cell[aria-pressed='true'] .mark {
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: var(--initiative);
  }
  .cell.now[aria-pressed='true'] .mark {
    background: var(--felt-sunk);
  }

  .label {
    overflow: hidden;
    text-overflow: ellipsis;
  }

  /* Four seats on a laptop: the hint is the first thing to go, then the
     group labels tighten. The track itself never wraps and never pushes the
     page sideways. */
  @media (max-width: 60rem) {
    .hint {
      display: none;
    }
    .cell {
      padding-left: var(--sp-1);
      padding-right: var(--sp-1);
    }
  }
</style>
