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

  it('a NULL pool renders nothing — an absent pool is not an error', () => {
    // The retired redaction sent a literal JSON null for a non-owning seat's
    // pool; the pool is now public (CR 106.4a/106.4b) and always a non-nil
    // object, so a null pool is no longer reachable from the wire. The
    // component still defends an absent/undefined prop (the generated
    // protocol.ts types it a plain Record, so the type checker cannot catch
    // a hand-built null), and an absent group draws nothing.
    expect(drawn(render(ManaPool, { props: { pool: null } }).html)).toBe('');
  });

  it('an absent pool renders nothing', () => {
    expect(drawn(render(ManaPool, { props: { pool: undefined } }).html)).toBe('');
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

  // --- available mana (task mp1): the public, battlefield-derived half ---

  it('available mana renders its own group, distinct from the floating pool', () => {
    const { html } = render(ManaPool, { props: { pool: {}, available: { G: 2, W: 1 } } });
    expect(html).toContain('data-mana-available');
    expect(html).toContain('data-avail="G"');
    expect(html).toContain('data-avail="W"');
    // Only nonzero available symbols appear, in WUBRGC order.
    expect(orderAvail(html)).toEqual(['W', 'G']);
    // no floating pool is rendered when it is empty
    expect(html).not.toContain('data-mana-pool');
    expect(html).toContain('aria-label="Available by tapping: 1 white, 2 green"');
  });

  it('available renders even when the floating pool is null (a hidden/spectator pool)', () => {
    const { html } = render(ManaPool, { props: { pool: null, available: { B: 1 } } });
    expect(html).toContain('data-mana-available');
    expect(html).toContain('data-avail="B"');
    expect(html).not.toContain('data-mana-pool');
  });

  it('an absent available is not an error and draws nothing', () => {
    expect(drawn(render(ManaPool, { props: { pool: {} } }).html)).toBe('');
  });

  it('both groups together are separated and never mistaken for one pile', () => {
    const { html } = render(ManaPool, { props: { pool: { R: 3 }, available: { U: 2 } } });
    expect(html).toContain('data-mana-available');
    expect(html).toContain('data-mana-pool');
    expect(html).toContain('data-mana-sep');
    expect(html).toContain('aria-label="Available by tapping: 2 blue"');
    expect(html).toContain('aria-label="Mana pool: 3 red"');
  });

  // The persistent legibility cue: the two groups are distinguishable by fill
  // (hollow vs solid), but that is subtle and colour-blind-invisible, so each
  // group also leads with a persistent text tag. This must be a real element
  // in the static markup, not a hover-only tooltip.
  it('each group leads with a persistent text tag naming what it is', () => {
    const { html } = render(ManaPool, { props: { pool: { R: 3 }, available: { U: 2 } } });
    expect(html).toContain('data-mana-tag="tap"');
    expect(html).toContain('data-mana-tag="pool"');
    // The tag is the first child inside each group, before its chips.
    const availIdx = html.indexOf('data-mana-available');
    const tapIdx = html.indexOf('data-mana-tag="tap"');
    const availChip = html.indexOf('data-avail="U"');
    expect(tapIdx).toBeGreaterThan(availIdx);
    expect(availChip).toBeGreaterThan(tapIdx);
  });
});

const orderAvail = (html: string): string[] => [...html.matchAll(/data-avail="([WUBRGC])"/g)].map((m) => m[1]);
