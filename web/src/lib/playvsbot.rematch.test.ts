import { describe, expect, it, vi } from 'vitest';
import { startRematch } from './playvsbot';
import { setBasePathForTests, withBase } from './basepath';

const fetchMock = vi.fn();
vi.stubGlobal('fetch', fetchMock);

describe('startRematch (fb-20260922T202722Z)', () => {
  it('POSTs the whole matchup — format, both exact deck ids, bot_policy and mulligans — and returns the base-relative join path', async () => {
    setBasePathForTests('/x');
    fetchMock.mockReset();
    fetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ table: 'g2', match: 1, seed: 99, seat: 0, token: 'tok2', join: '/t/g2?seat=0&token=tok2' }),
    });
    await expect(startRematch('constructed', 'a', 'b', 'bot', 3)).resolves.toBe(withBase('/t/g2?seat=0&token=tok2'));
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({
      format: 'constructed',
      human_deck: 'a',
      bot_deck: 'b',
      bot_policy: 'bot',
      mulligans: 3,
    });
  });

  it('sends mulligans 0 (a real value, the pre-game round disabled) rather than omitting it', async () => {
    setBasePathForTests('');
    fetchMock.mockReset();
    fetchMock.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ table: 'g2', match: 1, seed: 1, seat: 0, token: 't', join: '/t/g2' }),
    });
    await startRematch('commander', 'h', 'b', '', 0);
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({
      format: 'commander',
      human_deck: 'h',
      bot_deck: 'b',
      mulligans: 0,
    });
  });

  it('propagates a server rejection (a 404 when CreateGame is not armed) so the control can render it', async () => {
    setBasePathForTests('');
    fetchMock.mockReset();
    fetchMock.mockResolvedValueOnce({ ok: false, status: 404, json: async () => ({ code: 'not_found', message: 'disabled' }) });
    await expect(startRematch('constructed', 'a', 'b', 'bot', 6)).rejects.toMatchObject({ status: 404 });
  });
});
