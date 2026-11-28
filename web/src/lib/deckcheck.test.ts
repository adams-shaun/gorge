import { readFileSync } from 'node:fs';
import { describe, expect, it, vi } from 'vitest';
import type { DeckReport } from '../protocol';
import { checkDeck, createGame, ApiError } from './api';
import { statusLine, substitutionNotice, unsupportedLine } from './deckcheck';
import { setBasePathForTests, withBase } from './basepath';

// The check panel's pure half, against the two fixtures the Go test
// TestWebDeckCheckFixturesDecode pins in the server's own shape: the wording
// comes only from the report, the request carries only text, format and
// consent, and a refused createGame keeps the server's report on the error.
const fetchMock = vi.fn();
vi.stubGlobal('fetch', fetchMock);

const checkFixture = JSON.parse(readFileSync(new URL('../../../server/host/httpapi/testdata/web-deck-check.json', import.meta.url), 'utf8')) as {
  format: 'commander';
  text: string;
  substitute: boolean;
};
const reportFixture = JSON.parse(readFileSync(new URL('../../../server/host/httpapi/testdata/web-deck-report.json', import.meta.url), 'utf8')) as DeckReport;

describe('deckcheck wording', () => {
  it('statusLine names the three states', () => {
    expect(statusLine(reportFixture)).toBe('Not legal');
    expect(statusLine({ ...reportFixture, legal: true, playable: true })).toBe('Legal and playable');
    expect(statusLine({ ...reportFixture, legal: true, playable: false })).toBe('Legal, but this server cannot play every card');
  });

  it('substitutionNotice sums the swapped copies', () => {
    // The fixture swaps two single copies, both for Swamp.
    expect(substitutionNotice(reportFixture)).toBe('This deck will play with 2 basic lands in place of the cards listed above. Games with it are not games of the deck as written.');
    expect(substitutionNotice({ ...reportFixture, substituted: [] })).toBe('');
  });

  it('unsupportedLine names the card, its copies and the primitives it needs', () => {
    expect(unsupportedLine(reportFixture.unsupported[0])).toBe('Leyline of the Hidden ×2 — needs trigger: a countered spell is missed');
    expect(unsupportedLine({ name: 'Vessel of Wildfire', count: 1, needs: ['cost: an exiled-zone override', 'static: split'] })).toBe('Vessel of Wildfire ×1 — needs cost: an exiled-zone override, static: split');
  });
});

describe('checkDeck', () => {
  it('checkDeck posts the shared fixture body', async () => {
    setBasePathForTests('');
    fetchMock.mockReset();
    fetchMock.mockResolvedValueOnce({ ok: true, json: async () => reportFixture });
    await expect(checkDeck(checkFixture.text, checkFixture.format, checkFixture.substitute)).resolves.toBe(reportFixture);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0][0]).toBe(withBase('/api/decks/check'));
    expect(fetchMock.mock.calls[0][1]).toMatchObject({ method: 'POST', headers: { 'Content-Type': 'application/json' } });
    const body = JSON.parse(fetchMock.mock.calls[0][1].body);
    expect(body).toEqual(JSON.parse(JSON.stringify(checkFixture)));
    // The server reads a fixed wire shape: format, text, substitute, in that order.
    expect(Object.keys(body)).toEqual(['format', 'text', 'substitute']);
  });

  it('a failed check throws the server code and message as ApiError', async () => {
    setBasePathForTests('');
    fetchMock.mockReset();
    fetchMock.mockResolvedValueOnce({ ok: false, status: 429, statusText: 'Too Many Requests', json: async () => ({ code: 'rate_limited', message: 'slow down' }) });
    const err = await checkDeck(checkFixture.text, checkFixture.format, false).catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.status).toBe(429);
    expect(err.code).toBe('rate_limited');
    expect(err.message).toBe('slow down');
  });
});

describe('createGame refusal', () => {
  it('createGame keeps the refused deck report on ApiError', async () => {
    setBasePathForTests('');
    fetchMock.mockReset();
    fetchMock.mockResolvedValueOnce({
      ok: false,
      status: 400,
      statusText: 'Bad Request',
      json: async () => ({ code: 'deck_refused', message: 'seat 0 deck: refused', deck: reportFixture }),
    });
    const err = await createGame({ format: 'commander', decks: [{ seat: 0, text: checkFixture.text, substitute: true }] }).catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.status).toBe(400);
    expect(err.code).toBe('deck_refused');
    expect(err.message).toBe('seat 0 deck: refused');
    expect(err.deck).toEqual(reportFixture);
  });
});
