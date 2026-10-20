import { describe, expect, it, vi } from 'vitest';

const { streamUrls } = vi.hoisted(() => ({ streamUrls: [] as string[] }));
vi.mock('./stream', () => ({
  openStream: (url: string) => {
    streamUrls.push(url);
    return { session: null, onFrame: () => () => {}, close: () => {} };
  },
}));
vi.mock('./api', () => ({ subscribe: vi.fn(), unsubscribe: vi.fn() }));
vi.mock('./eventapi', () => ({ eventApi: { subscribe: vi.fn() } }));

describe('session stream URL', () => {
  it('carries the join URL seat token so a private table is in hello', async () => {
    vi.stubGlobal('location', { search: '?seat=0&token=seat-token' });
    try {
      await import('./session.svelte');
    } finally {
      vi.unstubAllGlobals();
    }
    expect(streamUrls[0]).toBe('/api/stream?token=seat-token');
  });
});
