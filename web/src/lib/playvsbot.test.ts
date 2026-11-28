import { readFileSync } from 'node:fs';
import { describe, expect, it, vi } from 'vitest';
import { MAX_DECK_TEXT_BYTES, deckTextTooLong, startPlayVsBot } from './playvsbot';
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
    await startPlayVsBot('constructed', '', '', 6, '', 'off');
    expect(JSON.parse(fetchMock.mock.calls[2][1].body)).toEqual({ format: 'constructed', mulligans: 6, paired_bot: 'off' });
  });

  it('posts a four-seat Commander pod body equal to the shared fixture', async () => {
    setBasePathForTests('');
    fetchMock.mockReset();
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({ table: 'g1', match: 1, seed: 7, seat: 0, token: 'tok', join: '/t/g1?seat=0&token=tok' }),
    });
    const fixture = JSON.parse(readFileSync(new URL('../../../server/gorged/testdata/web-create-pod.json', import.meta.url), 'utf8'));
    expect(fixture.seats).toBe(4);
    await startPlayVsBot('commander', 'h', 'b', 6, '', '', 4);
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual(fixture);
    await startPlayVsBot('commander', 'h', 'b', 6, 'bot', 'off', 3);
    expect(fetchMock.mock.calls[1][1].body).toBe(
      JSON.stringify({ format: 'commander', bot_policy: 'bot', mulligans: 6, paired_bot: 'off', seats: 3, humans: 1 }),
    );
    await startPlayVsBot('commander', 'h', 'b', 6, '', '', 2);
    expect(JSON.parse(fetchMock.mock.calls[2][1].body)).toEqual({ format: 'commander', human_deck: 'h', bot_deck: 'b', mulligans: 6 });
  });

  it('sends pasted text as decks[0].text and equals the shared fixture', async () => {
    setBasePathForTests('');
    fetchMock.mockReset();
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({ table: 'g1', match: 1, seed: 7, seat: 0, token: 'tok', join: '/t/g1?seat=0&token=tok' }),
    });
    const fixture = JSON.parse(readFileSync(new URL('../../../server/gorged/testdata/web-create-pod-text.json', import.meta.url), 'utf8'));
    expect(fixture.decks).toHaveLength(1);
    const text = 'Commander\n1 Ghoulcaller Gisa\n\n1 Sol Ring\n';
    await startPlayVsBot('commander', 'h', '', 6, '', '', 4, text);
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual(fixture);
    // At two seats the pasted list replaces the pool pick; the bot's pick stays.
    await startPlayVsBot('commander', 'h', 'b', 6, '', '', 2, text);
    expect(fetchMock.mock.calls[1][1].body).toBe(
      JSON.stringify({ format: 'commander', bot_deck: 'b', mulligans: 6, decks: [{ seat: 0, text: text.trim() }] }),
    );
  });

  it('keeps the request unchanged when the box is empty', async () => {
    setBasePathForTests('');
    fetchMock.mockReset();
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({ table: 'g1', match: 1, seed: 7, seat: 0, token: 'tok', join: '/t/g1?seat=0&token=tok' }),
    });
    for (const empty of ['', '  \n\t\n']) {
      fetchMock.mockClear();
      await startPlayVsBot('commander', 'h', 'b', 6, '', '', 2, empty);
      expect(fetchMock.mock.calls[0][1].body).toBe(
        JSON.stringify({ format: 'commander', human_deck: 'h', bot_deck: 'b', mulligans: 6 }),
      );
    }
  });

  it('deckTextTooLong is true past 65536 bytes', () => {
    expect(MAX_DECK_TEXT_BYTES).toBe(65536);
    expect(deckTextTooLong('a'.repeat(65536))).toBe(false);
    expect(deckTextTooLong('a'.repeat(65537))).toBe(true);
    // Bytes, not characters: 32769 two-byte characters are 65538 bytes.
    expect(deckTextTooLong('é'.repeat(32769))).toBe(true);
  });

  it('propagates a server rejection so the panel can render it', async () => {
    setBasePathForTests('');
    fetchMock.mockReset();
    fetchMock.mockResolvedValueOnce({ ok: false, status: 404, json: async () => ({ code: 'not_found', message: 'disabled' }) });
    await expect(startPlayVsBot('commander')).rejects.toMatchObject({ status: 404 });
  });

  it('sends substitute only with consent', async () => {
    setBasePathForTests('');
    fetchMock.mockReset();
    fetchMock.mockResolvedValue({
      ok: true,
      json: async () => ({ table: 'g1', match: 1, seed: 7, seat: 0, token: 'tok', join: '/t/g1?seat=0&token=tok' }),
    });
    const text = 'Commander\n1 Ghoulcaller Gisa\n\n1 Sol Ring';
    await startPlayVsBot('commander', '', '', 6, '', '', 2, text, true);
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({ format: 'commander', mulligans: 6, decks: [{ seat: 0, text, substitute: true }] });
    // Without consent the seat entry is exactly what the paste flow sent
    // before substitution existed: no substitute key at all.
    await startPlayVsBot('commander', '', '', 6, '', '', 2, text);
    const decks = JSON.parse(fetchMock.mock.calls[1][1].body).decks;
    expect(decks).toEqual([{ seat: 0, text }]);
    expect('substitute' in decks[0]).toBe(false);
  });
});
