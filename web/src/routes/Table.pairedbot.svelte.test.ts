import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'svelte/server';
import type { PlayerView, View } from '../protocol';
import { initSeatContext } from '../lib/seat';

// Where the paired bot's panel goes. MatchState and session are mocked as in
// Table.svelte.test.ts so the SSR render is deterministic; the table's offer
// is seeded into the route's status holder, since no effect runs on the server.
const { fake } = vi.hoisted(() => {
  const shared = { view: null as View | null };
  class FakeMatch {
    match: number | null = 1;
    view: View | null = shared.view;
    seats = [];
    dvr = { match: 't1/1', head: 0, cursor: 0, live: true, events: [], turnStarts: [0], gap: false };
    decision = null;
    halted: string | null = null;
    loadError: string | null = null;
    dispatch() {}
    apply() {}
    loadFinished() {}
  }
  return { fake: { shared, FakeMatch } };
});
vi.mock('../lib/match.svelte', () => ({ MatchState: fake.FakeMatch }));
vi.mock('../lib/session.svelte', () => ({
  session: { stream: { onFrame: () => () => {} }, focus: async () => {}, unfocus: async () => {} },
}));

import Table from './Table.svelte';
import { layoutStore } from '../lib/layouts.svelte';
import { pairedStatus } from '../lib/pairedstatus.svelte';
import type { PartnerPlacement, RailSide } from '../lib/layoutprofile';

const player = (seat: number): PlayerView => ({
  seat, name: `P${seat}`, life: 20, lost: false, spells_cast_this_turn: 0, life_gained_this_turn: 0, life_lost_this_turn: 0, tokens_created_this_turn: 0, library_size: 30, hand_size: 7, graveyard_size: 0,
  completed_dungeons: 0,
  hand: [], battlefield: [], graveyard: [], exile: [], pool: {}, command: [], commanders: [], commander_casts: [],
});
const view = (): View => ({
  viewer: 0, visibility: 'seat', turn: 3, round: 3, step: 'main', phase: 'main1', active: 0, priority: 0,
  over: false, draw: false, winner: null, players: [player(0), player(1)], stack: [], pending: [],
});

function renderAt(rail: RailSide, partner: PartnerPlacement): string {
  layoutStore.edit((p) => {
    p.panels.rail = rail;
    p.panels.partner = partner;
  });
  return render(Table, { props: { table: 't1' } }).html;
}

/** Where the panel landed: its own rail (and that rail's side), or the main rail. */
function placed(html: string): { rail: 'partner' | 'main' | 'none'; side: string | null } {
  const panel = html.indexOf('data-paired-panel');
  const main = html.indexOf('data-rail-side=');
  const own = /<aside class="partner[^"]*"[^>]*data-partner-rail="(left|right)"/.exec(html);
  if (panel < 0) return { rail: 'none', side: own?.[1] ?? null };
  if (own && panel > own.index && panel < main) return { rail: 'partner', side: own[1] };
  return { rail: panel > main ? 'main' : 'none', side: own?.[1] ?? null };
}

describe('Table.svelte paired bot rail', () => {
  beforeEach(() => {
    initSeatContext('?seat=0&token=tok');
    fake.shared.view = view();
    pairedStatus.set('t1', { offered: true, policy: 'az-closed' });
  });

  it('by default puts an offered paired bot in a rail of its own, opposite the main rail', () => {
    let html = renderAt('right', 'opposite');
    expect(placed(html)).toEqual({ rail: 'partner', side: 'left' });
    expect(html).toMatch(/<main class="table rail-right partner-column partner-left[\s"]/);
    expect(html).toContain('aria-label="Paired bot width: drag, or use arrow keys; Home resets"');

    html = renderAt('left', 'opposite');
    expect(placed(html)).toEqual({ rail: 'partner', side: 'right' });
    expect(html).toMatch(/<main class="table rail-left partner-column partner-right[\s"]/);

    // The width is its own variable beside the main rail's.
    expect(html).toMatch(/--rail-width:[\d.]+%;--partner-width:20%/);
  });

  it('keeps the panel in the main rail when the layout says "In the rail"', () => {
    for (const rail of ['right', 'left'] as const) {
      const html = renderAt(rail, 'same');
      expect(placed(html), rail).toEqual({ rail: 'main', side: null });
      expect(html).toMatch(new RegExp(`<main class="table rail-${rail}[\\s"]`));
    }
  });

  it('tucks a hidden paired bot into an edge drawer that takes no column', () => {
    const html = renderAt('right', 'hidden');
    expect(placed(html)).toEqual({ rail: 'partner', side: 'left' });
    expect(html).toContain('data-partner-mode="drawer"');
    expect(html).toMatch(/<main class="table rail-right partner-drawer partner-left[\s"]/);
    expect(html).not.toContain('Paired bot width');
  });

  it('with no paired bot offered there is no partner rail, whatever the setting', () => {
    for (const partner of ['opposite', 'same', 'hidden'] as const) {
      pairedStatus.set('t1', null);
      let html = renderAt('right', partner);
      expect(placed(html), partner).toEqual({ rail: 'none', side: null });
      expect(html).toMatch(/<main class="table rail-right[\s"]/);
      expect(html).not.toMatch(/<main class="[^"]*partner-/);

      // A server that says why not shows the reason where the panel always was.
      pairedStatus.set('t1', { offered: false, reason: 'This table has no paired bot.' });
      html = renderAt('right', partner);
      expect(placed(html), partner).toEqual({ rail: 'main', side: null });
      expect(html).not.toContain('data-partner-rail');
    }
  });

  it('a spectator gets no paired bot and no partner rail', () => {
    initSeatContext('');
    const html = renderAt('right', 'opposite');
    expect(placed(html)).toEqual({ rail: 'none', side: null });
    expect(html).not.toContain('data-partner-rail');
  });
});
