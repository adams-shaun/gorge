import { describe, expect, it } from 'vitest';
import { render } from 'svelte/server';
import ManaPool from './ManaPool.svelte';

// The repo's component-test pattern is deterministic SSR via svelte/server.
// The pool shapes below are the ones view/visibility.go's poolView emits: a
// map keyed by W/U/B/R/G/C, only the symbols with mana in them.

const order = (html: string): string[] => [...html.matchAll(/data-mana="([WUBRGC])"/g)].map((m) => m[1]);
// SSR emits hydration markers around every block; "renders nothing" means no
// element, not a literally empty string.
const drawn = (html: string): string => html.replace(/<!--[^]*?-->/g, '').trim();

describe('ManaPool', () => {
  it('an empty pool renders nothing at all — not an empty row', () => {
    expect(drawn(render(ManaPool, { props: { pool: {} } }).html)).toBe('');
  });

  it('a pool whose every symbol is zero renders nothing', () => {
    expect(drawn(render(ManaPool, { props: { pool: { R: 0, G: 0 } } }).html)).toBe('');
  });

  it('one symbol renders its colour token and its count in the data face', () => {
    const { html } = render(ManaPool, { props: { pool: { R: 2 } } });
    expect(html).toContain('data-mana-pool');
    expect(html).toContain('data-mana="R"');
    expect(html).toContain('var(--mana-r)');
    expect(html).toContain('>2<');
    expect(html).not.toContain('data-mana="G"');
  });

  // Object key order is not a rendering order. A readout that reshuffles
  // itself between frames cannot be read at a glance.
  it('renders in WUBRGC order however the wire ordered the keys', () => {
    const { html } = render(ManaPool, { props: { pool: { C: 1, G: 1, R: 1, B: 1, U: 1, W: 1 } } });
    expect(order(html)).toEqual(['W', 'U', 'B', 'R', 'G', 'C']);

    const { html: two } = render(ManaPool, { props: { pool: { G: 3, U: 1 } } });
    expect(order(two)).toEqual(['U', 'G']);
  });

  it('only nonzero symbols appear', () => {
    const { html } = render(ManaPool, { props: { pool: { W: 0, U: 1, B: 0, R: 3, G: 0, C: 0 } } });
    expect(order(html)).toEqual(['U', 'R']);
  });

  it('the pool is named for a screen reader, since its only visual label is colour', () => {
    const { html } = render(ManaPool, { props: { pool: { U: 1, R: 2 } } });
    expect(html).toContain('aria-label="Mana pool: 1 blue, 2 red"');
  });

  it('a symbol outside the wire\'s six is ignored rather than guessed at', () => {
    const { html } = render(ManaPool, { props: { pool: { S: 4 } } });
    expect(drawn(html)).toBe('');
  });
});
