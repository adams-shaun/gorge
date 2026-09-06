<script lang="ts">
  import { manaSymbols, type ManaSymbol } from '../lib/mana';

  /**
   * Renders a Forge-notation mana cost as pips; unrecognised notation renders
   * nothing rather than guessing.
   *
   * Colour is not decoration here. The design system's rule is that a
   * saturated colour means mana, card colour or seat identity, and mana is the
   * sport's native language: a player reads a red pip faster than the letter
   * R. So each pip wears its own mana identity, hybrids wear both halves
   * split on the diagonal the printed symbol uses, and generic costs stay
   * stone-coloured because generic mana has no colour to claim.
   *
   * The class is picked from the SYMBOL's own letters — a lexical mapping of
   * wire notation to the palette. Nothing here decides what a cost does.
   */
  let { cost }: { cost: string } = $props();

  const symbols = $derived(manaSymbols(cost));

  const COLOUR: Record<string, string> = { W: 'w', U: 'u', B: 'b', R: 'r', G: 'g', C: 'c' };

  /** kind maps a parsed symbol to its pip class: a single colour, a two-colour hybrid, or generic. */
  function kind(s: ManaSymbol): string {
    switch (s.kind) {
      case 'colour': return `p-${COLOUR[s.colour]}`;
      case 'colourless': return 'p-c';
      case 'snow':
      case 'variable':
      case 'generic':
      case 'unknown': return 'p-x';
      case 'twobrid': return `p-split p-x-${COLOUR[s.colour]}`;
      case 'phyrexian': return `p-split p-${COLOUR[s.colour]}-x`;
      case 'phyrexianHybrid': return `p-split p-${COLOUR[s.a]}-${COLOUR[s.b]}`;
      case 'hybrid': {
        // colourless halves have no gradient class; fall back to a defined stone disc rather than a half-empty pip.
        if (s.a === 'C' || s.b === 'C') return 'p-x';
        return `p-split p-${COLOUR[s.a]}-${COLOUR[s.b]}`;
      }
    }
  }
</script>

{#if symbols.length}
  <span class="mana-symbols">
    {#each symbols as s, i (i)}<span class="pip {kind(s)}" class:wide={s.text.length > 1}>{s.text}</span>{/each}
  </span>
{/if}

<style>
  .mana-symbols {
    display: inline-flex;
    gap: 2px;
    vertical-align: middle;
    flex: none;
  }
  /* A pip is a disc, the way the printed symbol is, and it holds its size
     against the surrounding type rather than scaling with it into
     illegibility: 11px is the floor at which a letter inside a 15px disc
     stays a letter. */
  .pip {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    box-sizing: border-box;
    min-width: 15px;
    height: 15px;
    padding: 0 3px;
    border-radius: 999px;
    font-family: var(--font-ui);
    font-size: var(--t-10);
    font-weight: 600;
    line-height: 1;
    letter-spacing: 0;
  }
  .pip.wide {
    letter-spacing: -0.04em;
  }

  /* Dark glyph on a light disc is how every printed mana symbol is drawn, and
     it is also the higher-contrast choice for W, U, G, C and generic. Black
     and red are the two whose tokens are dark enough that the glyph has to
     invert instead. */
  .p-w { background: var(--mana-w); color: var(--felt-sunk); }
  .p-u { background: var(--mana-u); color: var(--felt-sunk); }
  .p-g { background: var(--mana-g); color: var(--felt-sunk); }
  .p-c { background: var(--mana-c); color: var(--felt-sunk); }
  .p-b { background: var(--mana-b); color: var(--mana-w); }
  .p-r { background: var(--mana-r); color: var(--mana-w); }
  /* Generic and X are stone: no colour to claim, so none is given. */
  .p-x { background: var(--ink-faint); color: var(--felt-sunk); }

  /* A hybrid wears both halves, split on the printed symbol's diagonal. */
  .p-split { color: var(--felt-sunk); }
  .p-w-u { background: linear-gradient(135deg, var(--mana-w) 50%, var(--mana-u) 50%); }
  .p-w-b { background: linear-gradient(135deg, var(--mana-w) 50%, var(--mana-b) 50%); }
  .p-u-b { background: linear-gradient(135deg, var(--mana-u) 50%, var(--mana-b) 50%); }
  .p-u-r { background: linear-gradient(135deg, var(--mana-u) 50%, var(--mana-r) 50%); }
  .p-b-r { background: linear-gradient(135deg, var(--mana-b) 50%, var(--mana-r) 50%); }
  .p-b-g { background: linear-gradient(135deg, var(--mana-b) 50%, var(--mana-g) 50%); }
  .p-r-g { background: linear-gradient(135deg, var(--mana-r) 50%, var(--mana-g) 50%); }
  .p-r-w { background: linear-gradient(135deg, var(--mana-r) 50%, var(--mana-w) 50%); }
  .p-g-w { background: linear-gradient(135deg, var(--mana-g) 50%, var(--mana-w) 50%); }
  .p-g-u { background: linear-gradient(135deg, var(--mana-g) 50%, var(--mana-u) 50%); }
  /* Two-brid (2/R) and Phyrexian (W/P): the half with no mana identity of its
     own stays stone, so the coloured half still reads. */
  .p-x-w { background: linear-gradient(135deg, var(--ink-faint) 50%, var(--mana-w) 50%); }
  .p-x-u { background: linear-gradient(135deg, var(--ink-faint) 50%, var(--mana-u) 50%); }
  .p-x-b { background: linear-gradient(135deg, var(--ink-faint) 50%, var(--mana-b) 50%); color: var(--mana-w); }
  .p-x-r { background: linear-gradient(135deg, var(--ink-faint) 50%, var(--mana-r) 50%); }
  .p-x-g { background: linear-gradient(135deg, var(--ink-faint) 50%, var(--mana-g) 50%); }
  .p-w-x { background: linear-gradient(135deg, var(--mana-w) 50%, var(--ink-faint) 50%); }
  .p-u-x { background: linear-gradient(135deg, var(--mana-u) 50%, var(--ink-faint) 50%); }
  .p-b-x { background: linear-gradient(135deg, var(--mana-b) 50%, var(--ink-faint) 50%); color: var(--mana-w); }
  .p-r-x { background: linear-gradient(135deg, var(--mana-r) 50%, var(--ink-faint) 50%); }
  .p-g-x { background: linear-gradient(135deg, var(--mana-g) 50%, var(--ink-faint) 50%); }
  .p-x-x { background: var(--ink-faint); color: var(--felt-sunk); }
</style>
