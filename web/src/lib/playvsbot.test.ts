import { describe, expect, it, vi } from 'vitest';
import { startPlayVsBot } from './playvsbot';
import { setBasePathForTests, withBase } from './basepath';

const fetchMock = vi.fn();
vi.stubGlobal('fetch', fetchMock);

describe('startPlayVsBot', () => {
  it('returns the join path the server hands back, made base-relative', async () => {
    setBasePathForTests('/x');
    fetchMock.mockReset();
    fetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ table: 'g1', match: 1, seed: 7, seat: 0, token: 'tok', join: '/t/g1?seat=0&token=tok' }),
    });
    await expect(startPlayVsBot('constructed')).resolves.toBe(withBase('/t/g1?seat=0&token=tok'));
    expect(fetchMock).toHaveBeenCalledWith('/x/api/games', expect.objectContaining({
      body: JSON.stringify({ format: 'constructed', mulligans: 6 }),
    }));
  });

  it('POSTs the default allowance 6 and an explicit allowance otherwise', async () => {
    setBasePathForTests('');
    fetchMock.mockReset();
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({ table: 'g1', match: 1, seed: 7, seat: 0, token: 'tok', join: '/t/g1?seat=0&token=tok' }),
    });
    await startPlayVsBot('constructed');
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({ format: 'constructed', mulligans: 6 });
    await startPlayVsBot('constructed', '', '', 3);
    expect(JSON.parse(fetchMock.mock.calls[1][1].body)).toEqual({ format: 'constructed', mulligans: 3 });
    await startPlayVsBot('constructed', '', '', 0);
    expect(JSON.parse(fetchMock.mock.calls[2][1].body)).toEqual({ format: 'constructed', mulligans: 0 });
  });

  it('sends bot_policy only when the player picked one', async () => {
    setBasePathForTests('');
    fetchMock.mockReset();
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({ table: 'g1', match: 1, seed: 7, seat: 0, token: 'tok', join: '/t/g1?seat=0&token=tok' }),
    });
    await startPlayVsBot('commander', 'h', 'b', 6, 'bot');
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({ format: 'commander', human_deck: 'h', bot_deck: 'b', bot_policy: 'bot', mulligans: 6 });
    await startPlayVsBot('commander', '', '', 6, '');
    expect(JSON.parse(fetchMock.mock.calls[1][1].body)).toEqual({ format: 'commander', mulligans: 6 });
  });

  it('propagates a server rejection so the panel can render it', async () => {
    setBasePathForTests('');
    fetchMock.mockReset();
    fetchMock.mockResolvedValueOnce({ ok: false, status: 404, json: async () => ({ code: 'not_found', message: 'disabled' }) });
    await expect(startPlayVsBot('commander')).rejects.toMatchObject({ status: 404 });
  });
});
