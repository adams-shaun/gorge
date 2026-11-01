import { describe, expect, it } from 'vitest';
import { parseFrame } from './stream';

// The server emits a table_state frame for a series table between
// games. parseFrame is the only gate between the wire and the stores, so it
// must accept the frame rather than drop it, and no second, hand-maintained
// frame-type list may omit it.
describe('stream frame types', () => {
  it('accepts a table_state frame', () => {
    const data = JSON.stringify({ v: 1, t: 'table_state', seq: 1, table: 't1', body: { state: 'between_games' } });
    expect(parseFrame(data)?.t).toBe('table_state');
  });

  it('accepts every kind the draft-event stream carries', () => {
    for (const t of ['event_state', 'draft_pack', 'pod_progress', 'series_update']) {
      expect(parseFrame(JSON.stringify({ v: 1, t, seq: 0, body: {} }))?.t).toBe(t);
    }
  });

  it('still rejects an unknown kind and a wrong protocol version', () => {
    expect(parseFrame(JSON.stringify({ v: 1, t: 'not_a_frame', seq: 0, body: {} }))).toBeNull();
    expect(parseFrame(JSON.stringify({ v: 2, t: 'table_state', seq: 0, body: {} }))).toBeNull();
  });
});
