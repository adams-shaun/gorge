<script lang="ts">
  import { oracleSegments } from '../lib/oracletext';
  import ManaSymbols from './ManaSymbols.svelte';

  /**
   * OracleText renders one Scryfall-style oracle text — the printed card's
   * rules text — as interleaved text runs and inline pips. It renders no
   * element of its own: its output is meant to sit INSIDE a host paragraph
   * (CardDetail's `.card-detail__oracle`) whose `white-space: pre-wrap`,
   * border and padding styling it inherits, so the tokenizer's verbatim
   * whitespace contract is honoured by emitting the text runs as raw
   * interpolated text rather than wrapped spans.
   *
   * Mana tokens draw ManaSymbols' own pips at the `inline` size — the same
   * classifier and classes the transcript and the feed use, smaller because
   * these pips live inside running 12px text. The non-mana icons {T}/{Q}/{E}
   * are drawn as outlined discs: an icon, not mana, and the one distinction
   * the design system's restraint allows. Anything the classifier does not
   * know ({TK}, {A}, {∞}) stays its literal braced text.
   */
  let { text }: { text: string } = $props();

  const segments = $derived(oracleSegments(text));
</script>

{#each segments as seg, i (i)}{#if seg.kind === 'text'}{seg.text}{:else if seg.kind === 'mana'}<ManaSymbols cost={seg.symbol.text} size="inline" />{:else if seg.kind === 'tap'}<span class="ic" title="tap">T</span>{:else if seg.kind === 'untap'}<span class="ic" title="untap">Q</span>{:else}<span class="ic" title="energy">E</span>{/if}{/each}

<style>
  /* The non-mana icons: an outlined disc, deliberately NOT the filled stone
     disc the unknown mana pip wears — {T} is not mana and must not read as
     one. Same 12px geometry as the inline mana pip so a line mixing both
     sits on one rhythm. */
  .ic {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    box-sizing: border-box;
    min-width: 12px;
    height: 12px;
    padding: 0 2px;
    border: 1px solid var(--ink-dim);
    border-radius: 999px;
    color: var(--ink-dim);
    font-family: var(--font-ui);
    font-size: var(--t-10);
    font-weight: 600;
    line-height: 1;
    letter-spacing: 0;
    vertical-align: middle;
  }
</style>
