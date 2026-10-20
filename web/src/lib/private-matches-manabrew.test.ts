import { describe, expect, it, vi } from 'vitest';
import type { MatchInfo } from '../protocol';
import { ManaBrewMatch } from './manabrew/source.svelte';
import { runs } from './manabrew/testdata/fixture';

// A private play-vs-bot table's match list is fenced behind the seat claim,
// so the ManaBrew source must list matches with its token too.
describe('private table ManaBrew match list', () => {
  it('lists the table matches with the seat claim', async () => {
    const info: MatchInfo = { table: 'g1', match: 1, seed: 1, seats: [], state: 'live', winner: null, events: 10, turns: 1, bot_policy: 'bot' };
    const fetchMatches = vi.fn(async () => [info]);
    const transport = {
      fetchState: async () => runs.find((r) => r.run === 'cast')!.records[0].manabrew,
      send: async () => {},
      openStream: () => ({ close: () => {} }),
    };
    const ctx = { seat: 0, token: 'g1-token' };
    const m = new ManaBrewMatch('g1', ctx, { transport, fetchMatches, compat: false });

    await expect(m.start()).resolves.toEqual({ ok: true });
    expect(fetchMatches).toHaveBeenCalledWith('g1', ctx);
  });
});
