<script lang="ts">
  import type { DvrState } from '../lib/dvr';
  import { visibleLog } from '../lib/logfilter';
  import type { LogSeatIdentity } from '../lib/logcolour';
  import { parseLogLine, cardColourVar, type CardColourResolver } from '../lib/logrender';
  import ManaSymbols from './ManaSymbols.svelte';

  /**
   * Transcript is the rules log: one line per event, the cursor's line
   * highlighted and scrolled into view, clicking a line scrubs to it. Lines
   * with no text (state-only events) are skipped. The three engine-noise
   * kinds (priority, decision_ask, decision_made) are hidden by default so
   * land plays, casts and triggers surface; the toggle reveals them. Step
   * lines ("Step: main-1") are hidden by default behind their own toggle,
   * independent of the noise toggle.
   *
   * Task 3 ("each log should note the player name in colour"): `identities`
   * is the caller's already-resolved seat name/colour list (Table.svelte
   * builds it the same way SeatTable does, off the same view+seats), and
   * every line is run through logcolour.ts's colourSegments so a seat's own
   * name is coloured wherever it reads as that PLAYER rather than as part of
   * a card's name — see logcolour.ts for how the two are told apart.
   * Optional so a caller with no seats yet (or every existing test) renders
   * every line as plain text, unchanged.
   *
   * Readable log (ui9). Beyond that task-3 colouring, each line is also run
   * through logrender.ts, which re-renders the server-described line from
   * its own structure rather than as prose:
   *   - mana symbols (`{R}{G}{W/U}`) are drawn as pips (ManaSymbols),
   *   - a card/permanent/spell reference `<Name> #<id>` is coloured by the
   *     card's mana-colour identity (B2) with its `#<id>` suppressed from
   *     the visible text and kept as a hover title (B3) — the id is
   *     load-bearing when two copies of the same card are in play, so it is
   *     reachable, not dropped;
   *   - a faceless ability reference ("an ability #id") gets an italic
   *     ability treatment (B2).
   * `cardColour` is the caller's card-name -> colour-key resolver, built
   * from the current view's cards (buildCardColour). It also carries the
   * view's exact card-name keys, so logrender.ts resolves a card reference
   * by longest exact match against them rather than by word shape (a comma
   * in `Jace, the Mind Sculptor` cannot split the name). It is optional, so
   * a caller with no cards (or a test) renders card names uncoloured by the
   * word-shape fallback.
   */
  let { dvr, onSeek, identities = [], cardColour = null }: {
    dvr: DvrState;
    onSeek: (seq: number) => void;
    identities?: LogSeatIdentity[];
    cardColour?: CardColourResolver | null;
  } = $props();

  let container: HTMLDivElement | undefined;
  let revealAll = $state(false);
  let revealSteps = $state(false);
  const lines = $derived(visibleLog(dvr.events, revealAll, revealSteps));

  $effect(() => {
    // The cursor is always a valid DVR target, but if it landed on a hidden
    // line there is no rendered row to bring into view, so the log simply
    // stays put; revealing all brings that row back and the scroll resumes.
    const seq = dvr.cursor;
    container?.querySelector<HTMLElement>(`[data-seq="${seq}"]`)?.scrollIntoView({ block: 'nearest' });
  });
</script>

<div class="transcript" bind:this={container}>
  <div class="bar">
    <button
      type="button"
      class="toggle"
      aria-pressed={revealSteps}
      title='Phase/step lines ("Step: main-1") are clock noise; they are hidden unless this is on. Independent of the engine-noise toggle.'
      onclick={() => (revealSteps = !revealSteps)}
    >
      {revealSteps ? 'Hide step lines' : 'Show step lines'}
    </button>
    <button
      type="button"
      class="toggle"
      aria-pressed={revealAll}
      title="Priority, decision asks and decision answers are engine bookkeeping; they are hidden unless this is on."
      onclick={() => (revealAll = !revealAll)}
    >
      {revealAll ? 'Hide engine noise' : 'Show engine noise'}
    </button>
  </div>
  {#each lines as e (e.event.seq)}
    <button
      type="button"
      class="line"
      class:current={e.event.seq === dvr.cursor}
      data-seq={e.event.seq}
      onclick={() => onSeek(e.event.seq)}
    >
      <span class="seq">{e.event.seq}</span>
      <span class="text">{#each parseLogLine(e.line, { identities, cardColour }) as p, i (i)}
        {#if p.kind === 'text'}{p.text}{:else if p.kind === 'mana'}<ManaSymbols cost={p.token} />{:else if p.kind === 'seat'}<span class="who" style:color={p.colour}>{p.text}</span>{:else if p.kind === 'card'}<span class="obj card" style:color={cardColourVar(p.colour) ?? undefined} title="{p.name} #{p.id}">{p.name}</span>{:else if p.kind === 'ability'}<span class="obj ability" title="an ability #{p.id}">{p.name}</span>{/if}
      {/each}</span>
    </button>
  {/each}
</div>

<style>
  .transcript {
    display: flex;
    flex-direction: column;
    height: 100%;
  }
  /* The bar used to carry a sentence explaining the default. A permanent
     sentence is a permanent line of a 160px band whose whole job is log
     lines, and the button already says what it does; the explanation moved
     to the button's own title, where it is one hover away and costs nothing
     when it is not wanted. Two toggles sit here now: the engine-noise one
     and the step-lines one, independent. */
  .bar {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: var(--sp-3);
    padding: var(--sp-1) var(--sp-3);
    border-bottom: 1px solid var(--edge-inst);
    font-size: .72rem;
    flex: none;
  }
  .toggle {
    background: none;
    border: 1px solid var(--edge-inst);
    border-radius: 4px;
    color: var(--ink-inst);
    font: inherit;
    padding: 2px var(--sp-3);
    cursor: pointer;
    flex: none;
  }
  .toggle:hover {
    border-color: var(--ink-dim);
    color: var(--ink);
  }
  .line {
    display: flex;
    gap: var(--sp-3);
    text-align: left;
    background: none;
    border: none;
    color: var(--ink-dim);
    font: inherit;
    padding: 1px var(--sp-3);
    cursor: pointer;
    width: 100%;
  }
  .line:hover {
    color: var(--ink-inst);
  }
  /* The cursor line is where the DVR is pointing: marked by a rule in the
     initiative colour, the same device the pending tray uses, rather than by
     a filled band that would fight the log's density. */
  .line.current {
    color: var(--ink);
    background: var(--instrument-raised);
    box-shadow: inset 2px 0 0 var(--initiative);
  }
  .seq {
    color: var(--ink-faint);
    font-variant-numeric: tabular-nums;
    width: 3.5em;
    text-align: right;
    flex: none;
  }
  .text {
    overflow-wrap: anywhere;
  }
  /* The seat's own identity colour (Task 3), inline with the sentence rather
     than a separate column — the same "learned once" register the rest of
     the rail uses colour for. */
  .who {
    font-weight: 600;
  }
  /* A card name (B2/B3) is coloured by its mana identity, the same palette a
     card frame wears, and the "#<id>" is off the visible text and on the
     hover title instead. */
  .obj.card {
    font-weight: 600;
  }
  /* An ability reference (the faceless "an ability #id" shape) gets its own
     treatment — italic, not the card weight — because on the stack an
     ability is not a card and should not read as one. */
  .obj.ability {
    font-style: italic;
    color: var(--ink-inst);
  }
</style>
