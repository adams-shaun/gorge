import { afterEach, describe, expect, it, vi } from 'vitest';
import { eventSeatFor, forgetEventSeat } from './eventseat';
import { cardNames, countNames, pickHistory, revealDecks } from './eventlog';
import { pct, record, standingRows } from './standings';
import { createRequest, defaultSeats, eventPolicies, inviteURL, maskToken, ratedBy17Lands, setNote, timerLabel, timerOffered, type SeatChoice } from './newevent';
import { gameDots, inPlay, seriesScore, seriesStatus } from './series';
import { pickSecondsLeft } from './pickclock';
import { createEvent, listEventPersonas, listEventSets } from './eventapi';
import { eventState, fakeFetch } from '../test/eventfixtures';
import type { BotPolicyInfo, EventSeries, EventSetInfo } from '../protocol';

afterEach(() => vi.unstubAllGlobals());

function memStorage(): Storage {
  const m = new Map<string, string>();
  return {
    get length() { return m.size; },
    clear: () => m.clear(),
    getItem: (k) => m.get(k) ?? null,
    key: (i) => [...m.keys()][i] ?? null,
    removeItem: (k) => void m.delete(k),
    setItem: (k, v) => void m.set(k, String(v)),
  };
}

describe('eventSeatFor', () => {
  it('reads ?seat=&token= from the invite link and remembers it for this event only', () => {
    const s = memStorage();
    expect(eventSeatFor('e1', '?seat=0&token=abc123', s)).toEqual({ seat: 0, token: 'abc123' });
    expect(eventSeatFor('e1', '', s)).toEqual({ seat: 0, token: 'abc123' });
    expect(eventSeatFor('e2', '', s)).toBeNull();
    forgetEventSeat('e1', s);
    expect(eventSeatFor('e1', '', s)).toBeNull();
  });
  it('rejects a malformed link like a table join URL does', () => {
    const s = memStorage();
    expect(eventSeatFor('e1', '?seat=x&token=abc', s)).toBeNull();
    expect(eventSeatFor('e1', '?seat=0&token=<script>', s)).toBeNull();
    expect(eventSeatFor('e1', '?token=abc', s)).toBeNull();
  });
});

describe('eventlog reveal', () => {
  // build.Deck's JSON tags are snake_case: DeckSaved's deck is {main, basics}, main omitted when empty.
  const log = [
    { kind: 'EventCreated', seed: 7, set: 'FDN' },
    { kind: 'PackOpened', round: 1, cards: [{ id: 'c1', name: 'Serra Angel' }, { id: 'c2', name: 'Lightning Bolt' }] },
    { kind: 'PackOpened', round: 1, seat: 1, cards: [{ id: 'c3', name: 'Lightning Bolt' }, { id: 'c4', name: 'Shock' }] },
    { kind: 'Picked', round: 1, pick_no: 1, id: 'c1' },
    { kind: 'Picked', round: 1, pick_no: 1, seat: 1, id: 'c3', auto: true },
    { kind: 'Picked', round: 1, pick_no: 2, id: 'c4' },
    { kind: 'DeckSaved', rev: 1, deck: { main: ['c1'], basics: [17, 0, 0, 0, 0] } },
    { kind: 'DeckSaved', rev: 2, deck: { main: ['c1', 'c4'], basics: [16, 0, 0, 0, 0] } },
    { kind: 'DeckSaved', seat: 1, rev: 1, deck: { main: ['c3', 'c2'], basics: [0, 0, 0, 20, 0] } },
  ];
  it('maps ids to names from the opened packs', () => {
    expect(cardNames(log).get('c4')).toBe('Shock');
  });
  it('reveals each seat\'s LAST saved deck', () => {
    const d = revealDecks(log);
    expect(d.map((x) => x.seat)).toEqual([0, 1]);
    expect(d[0]).toMatchObject({ rev: 2, cards: ['Serra Angel', 'Shock'], basics: [{ name: 'Plains', count: 16 }], total: 18 });
    expect(d[1].cards).toEqual(['Lightning Bolt', 'Lightning Bolt']);
    expect(countNames(d[1].cards)).toEqual([{ name: 'Lightning Bolt', count: 2 }]);
  });
  it('lists one seat\'s picks in order, flagging auto-picks', () => {
    expect(pickHistory(log, 0).map((p) => p.name)).toEqual(['Serra Angel', 'Shock']);
    expect(pickHistory(log, 1)).toEqual([{ round: 1, pickNo: 1, name: 'Lightning Bolt', auto: true }]);
  });
  it('a basics-only deck (main omitted) still reveals its lands', () => {
    expect(revealDecks([{ kind: 'DeckSaved', seat: 3, rev: 1, deck: { basics: [0, 0, 0, 0, 40] } }])).toEqual([
      { seat: 3, rev: 1, cards: [], basics: [{ name: 'Forest', count: 40 }], total: 40 },
    ]);
  });
  it('a redacted (pre-COMPLETE) log reveals nothing it hides', () => {
    expect(revealDecks([{ kind: 'DeckSaved', seat: 2, rev: 1, deck: { basics: [0, 0, 0, 0, 0] } }])).toEqual([]);
  });
});

describe('standings rows', () => {
  const series: EventSeries[] = [
    { round: 1, a: 0, b: 3, best_of: 1, state: 'ended', score: [1, 0], winner: 0, table: 't1' },
    { round: 1, a: 1, b: 2, best_of: 1, state: 'ended', score: [0, 0], winner: null, table: 't2' },
    { round: 2, a: 0, b: 2, best_of: 1, state: 'live', score: [0, 0], winner: null, table: 't3' },
  ];
  it('keeps the server order and formats the MTR columns', () => {
    const st = eventState({
      standings: [
        { seat: 0, points: 3, mw: 1, omw: 0.33, gw: 1, ogw: 0.333 },
        { seat: 1, points: 1, mw: 0.33, omw: 0.33, gw: 0.33, ogw: 0.33 },
        { seat: 2, points: 1, mw: 0.33, omw: 0.33, gw: 0.33, ogw: 0.33 },
        { seat: 3, points: 0, mw: 0.33, omw: 1, gw: 0.33, ogw: 1 },
      ],
    });
    const rows = standingRows(st.standings, st.seats, series, 0);
    expect(rows.map((r) => r.seat)).toEqual([0, 1, 2, 3]);
    expect(rows[0]).toMatchObject({ rank: 1, label: 'You', you: true, points: 3, record: '1-0', mw: '100%', omw: '33%', gw: '100%', ogw: '33%' });
    expect(rows[1].record).toBe('0-0-1');
    expect(rows[3]).toMatchObject({ label: 'Seat 4 · Bot 4 (bot)', record: '0-1', omw: '100%' });
  });
  it('pct and record helpers', () => {
    expect(pct(0.3333)).toBe('33%');
    expect(record(2, series)).toBe('0-0-1');
  });
});

const bots = (policies: string[], drafters: string[] = []): SeatChoice[] => [
  { kind: 'human', policy: '', drafter: '' },
  ...policies.map((policy, i): SeatChoice => ({ kind: 'bot', policy, drafter: drafters[i] ?? '' })),
];

describe('create request', () => {
  const p = (name: string, formats: string[]): BotPolicyInfo => ({
    name, label: name, description: '', tier: 'production', mean_ms: 0, p95_ms: 0, cost_scope: '', cost_note: '', search: false, formats,
  });
  it('offers only policies that play constructed', () => {
    expect(eventPolicies([p('a', ['constructed', 'commander']), p('b', ['commander'])]).map((x) => x.name)).toEqual(['a']);
  });
  it('seat 1 is the human; an empty policy keeps the server default', () => {
    expect(createRequest('FDN', bots(['lethal-pressure', '', 'bot']))).toEqual({
      set: 'FDN',
      seats: [{ kind: 'human' }, { kind: 'bot', policy: 'lethal-pressure' }, { kind: 'bot' }, { kind: 'bot', policy: 'bot' }],
    });
  });
});

describe('create request: seat kinds, best of, pick timer', () => {
  const pod = (...kinds: ('human' | 'bot')[]): SeatChoice[] => kinds.map((kind) => ({ kind, policy: kind === 'human' ? 'ignored' : '', drafter: '' }));
  it('the default pod is you plus three bots', () => {
    expect(defaultSeats().map((s) => s.kind)).toEqual(['human', 'bot', 'bot', 'bot']);
  });
  it('a human seat carries only its kind; best_of is sent only for 3', () => {
    expect(createRequest('FDN', pod('human', 'bot', 'human', 'bot'), { bestOf: 3 })).toEqual({
      set: 'FDN', best_of: 3, seats: [{ kind: 'human' }, { kind: 'bot' }, { kind: 'human' }, { kind: 'bot' }],
    });
    expect(createRequest('FDN', pod('human', 'bot', 'bot', 'bot'), { bestOf: 1 })).not.toHaveProperty('best_of');
  });
  it('pick_timer is sent only when chosen, and only for two or more humans', () => {
    expect(timerOffered(pod('human', 'bot', 'bot', 'bot'))).toBe(false);
    expect(timerOffered(pod('human', 'human', 'bot', 'bot'))).toBe(true);
    expect(createRequest('FDN', pod('human', 'human', 'bot', 'bot'))).not.toHaveProperty('pick_timer');
    expect(createRequest('FDN', pod('human', 'human', 'bot', 'bot'), { pickTimer: 'relaxed' }).pick_timer).toBe('relaxed');
    expect(createRequest('FDN', pod('human', 'bot', 'bot', 'bot'), { pickTimer: 'relaxed' })).not.toHaveProperty('pick_timer');
  });
  it('labels the stored timer mode; an invite link is origin + base + join, shown with its token masked', () => {
    expect(timerLabel('standard')).toBe('standard pick timer');
    expect(timerLabel('relaxed')).toBe('relaxed pick timer');
    expect(timerLabel('off')).toBe('no pick timer');
    expect(timerLabel(undefined)).toBe('no pick timer');
    const url = inviteURL('https://x.test', '/e/ev1?seat=1&token=abcDEF');
    expect(url).toBe('https://x.test/e/ev1?seat=1&token=abcDEF');
    expect(maskToken(url)).toBe('https://x.test/e/ev1?seat=1&token=••••••');
  });
});

describe('series rows and the pick clock', () => {
  const row = (over: Partial<EventSeries>): EventSeries => ({ round: 1, a: 0, b: 3, best_of: 3, state: 'live', score: [0, 0], winner: null, ...over });
  it('game dots read the per-game winners from one side (-1 is a draw)', () => {
    const s = row({ games: [3, 0, -1], score: [1, 1] });
    expect(gameDots(s, 0)).toEqual(['l', 'w', 'd']);
    expect(gameDots(s, 3)).toEqual(['w', 'l', 'd']);
    expect(gameDots(row({}), 0)).toEqual([]);
  });
  it('the status line names the live game, the between state and the outcome', () => {
    expect(seriesScore(row({ score: [2, 1] }))).toBe('2 – 1');
    expect(seriesScore(row({ state: 'pending' }))).toBe('vs');
    expect(seriesStatus(row({ game: 2 }))).toBe('live · game 2');
    expect(seriesStatus(row({ best_of: 1, attempt: 2 }))).toBe('live · attempt 2');
    expect(seriesStatus(row({ state: 'between', game: 3 }))).toBe('between games');
    expect(seriesStatus(row({ state: 'ended', winner: 0 }))).toBe('final');
    expect(seriesStatus(row({ state: 'ended', winner: null }))).toBe('draw');
    expect(inPlay(row({ state: 'between' }))).toBe(true);
    expect(inPlay(row({ state: 'ended' }))).toBe(false);
  });
  it('the pick clock counts the server\'s remaining_ms down from arrival, never below zero', () => {
    expect(pickSecondsLeft(70_000, 1000, 1000)).toBe(70);
    expect(pickSecondsLeft(70_000, 1000, 1500)).toBe(70);
    expect(pickSecondsLeft(70_000, 1000, 62_000)).toBe(9);
    expect(pickSecondsLeft(70_000, 1000, 71_000)).toBe(0);
    expect(pickSecondsLeft(70_000, 1000, 99_000)).toBe(0);
  });
});

describe('create request: personas, no seed', () => {
  it('names a bot\'s pickbot persona only when one was chosen', () => {
    expect(createRequest('BLB', bots(['', 'bot', ''], ['best23', '', 'rating-colour']))).toEqual({
      set: 'BLB',
      seats: [{ kind: 'human' }, { kind: 'bot', drafter: 'best23' }, { kind: 'bot', policy: 'bot' }, { kind: 'bot', drafter: 'rating-colour' }],
    });
  });
  it('never sends a seed (the server mints it; an unknown field is a 400)', async () => {
    const { calls } = fakeFetch({ 'POST /api/events': { body: { event: 'e9', invites: [], state: eventState() } } });
    await createEvent(createRequest('FDN', defaultSeats()));
    expect(calls).toHaveLength(1);
    const body = calls[0].body as Record<string, unknown>;
    expect(Object.keys(body).sort()).toEqual(['seats', 'set']);
    expect(JSON.stringify(body)).not.toContain('seed');
  });
});

describe('event discovery', () => {
  const sets: EventSetInfo[] = [
    { code: 'BLB', name: 'Bloomburrow', pack_size: 14, ratings: 'heuristic' },
    { code: 'FDN', name: 'Foundations', pack_size: 14, ratings: '17lands' },
  ];
  it('reads the server\'s set catalogue and persona list', async () => {
    const { calls } = fakeFetch({
      'GET /api/events/sets': { body: sets },
      'GET /api/events/personas': { body: { drafters: ['best23', 'rating-colour'], builders: ['best23', 'rating-colour'] } },
    });
    expect(await listEventSets()).toEqual(sets);
    expect((await listEventPersonas()).drafters).toEqual(['best23', 'rating-colour']);
    expect(calls.map((c) => `${c.method} ${c.path}`)).toEqual(['GET /api/events/sets', 'GET /api/events/personas']);
    expect(calls.every((c) => c.auth === null)).toBe(true);
  });
  it('credits 17Lands only for a 17lands-rated set', () => {
    expect(setNote(sets[1])).toBe('14-card packs · ratings: 17Lands');
    expect(setNote(sets[0])).toBe('14-card packs · ratings: card heuristic');
    expect(ratedBy17Lands(sets, 'FDN')).toBe(true);
    expect(ratedBy17Lands(sets, 'BLB')).toBe(false);
    // Unknown (catalogue not loaded, or unlisted): keep the credit.
    expect(ratedBy17Lands(null, 'BLB')).toBe(true);
    expect(ratedBy17Lands(sets, 'XYZ')).toBe(true);
  });
});
