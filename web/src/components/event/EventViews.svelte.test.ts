import { afterEach, describe, expect, it, vi } from 'vitest';
import { render } from 'svelte/server';
import DraftView from './DraftView.svelte';
import BuildView from './BuildView.svelte';
import HubView from './HubView.svelte';
import Standings from './Standings.svelte';
import LobbyView from './LobbyView.svelte';
import PhaseRail from './PhaseRail.svelte';
import { EventClient } from '../../lib/draftevent.svelte';
import { DeckBuilder } from '../../lib/deckbuild.svelte';
import { standingRows } from '../../lib/standings';
import { card, draftPack, eventPool, eventState } from '../../test/eventfixtures';
import PodStatus from './PodStatus.svelte';
import type { EventSeries, Frame } from '../../protocol';

// SSR via svelte/server, the repo's component-test convention: no DOM, no
// effects, nothing reaches the network. The interactive behaviour (select,
// pick, stale refetch, drag/click moves, steppers, Auto, Ready) is pinned on
// the state classes in lib/draftevent.svelte.test.ts and
// lib/deckbuild.svelte.test.ts; these tests pin what each view RENDERS from
// that state.
vi.mock('../../lib/images', () => ({ images: { url: () => new Promise<string | null>(() => {}), offline: () => false } }));

afterEach(() => vi.unstubAllGlobals());

const frame = (t: string, body: unknown): Frame => ({ v: 1, t, seq: 0, body });

function drafting() {
  const c = new EventClient('e1', { seat: 0, token: 'tok0' });
  c.handleFrame(frame('event_state', eventState()));
  c.handleFrame(frame('draft_pack', draftPack()));
  return c;
}

describe('DraftView', () => {
  it('renders the pick header, the pack, the pool by mana value and the 17Lands credit', () => {
    const { body } = render(DraftView, { props: { client: drafting() } });
    expect(body).toContain('Pack 1 · Pick 3');
    expect(body).toContain('3 cards');
    expect(body).toContain('opened by Seat 3 · Bot 3 (bot)'); // pick 3 of a left pass came two seats round
    expect(body).toContain('passing left ←');
    expect(body).toContain('aria-label="Shivan Dragon, rare"');
    expect(body).toMatch(/data-testid="take"[^>]*disabled/);
    expect(body).toContain('Take card');
    expect(body).toContain('Your picks · 2');
    expect(body).toContain('Serra Angel');
    expect(body).toMatch(/Pick ratings: data from <a[^>]*>17Lands<\/a>,\s*<a[^>]*>CC BY 4.0<\/a>/);
  });

  it('a selected card enables Take, names it, and shows in the inspector', () => {
    const c = drafting();
    c.select('c11');
    const { body } = render(DraftView, { props: { client: c } });
    expect(body).toContain('Take Shivan Dragon');
    expect(body).not.toMatch(/data-testid="take"[^>]*disabled/);
    expect(body).toMatch(/aria-selected="true"[^>]*data-id="c11"|data-id="c11"[^>]*aria-selected="true"/);
    expect(body).toContain('MV 6');
  });

  it('pack 2 passes right', () => {
    const c = new EventClient('e1', { seat: 0, token: 't' });
    c.handleFrame(frame('event_state', eventState()));
    c.handleFrame(frame('draft_pack', draftPack({ round: 2, pick_no: 2 })));
    const { body } = render(DraftView, { props: { client: c } });
    expect(body).toContain('passing right →');
    expect(body).toContain('opened by Seat 2');
  });

  it('shows the stale-pick notice the client set', () => {
    const c = drafting();
    c.notice = 'That pack moved on before your pick landed — here is the pack you hold now.';
    const { body } = render(DraftView, { props: { client: c } });
    expect(body).toContain('That pack moved on');
  });

  it('waiting for a pack, with the pod queue counts from pod_progress', () => {
    const c = drafting();
    c.handleFrame(frame('draft_pack', draftPack({ waiting: true, pack: [] })));
    c.handleFrame(frame('pod_progress', { event: 'e1', round: 1, picks: [4, 3, 3, 3], queued: [0, 1, 2, 0] }));
    const { body } = render(DraftView, { props: { client: c } });
    expect(body).toContain('Waiting for the next pack');
    expect(body).toContain('picks 3 · queued 2');
    expect(body).toContain('picks 4 · queued 0');
  });
  it('lays the pool out by the wire\'s mana value and colours, and asks the network for nothing', () => {
    const fetch = vi.fn(() => Promise.reject(new Error('no fetch expected')));
    vi.stubGlobal('fetch', fetch);
    const c = new EventClient('e1', { seat: 0, token: 't' });
    c.handleFrame(frame('event_state', eventState()));
    c.handleFrame(frame('draft_pack', draftPack({ pool: [card('c1', 'Serra Angel'), card('c2', 'Lightning Bolt'), card('c3', 'Evolving Wilds')] })));
    c.select('c11');
    const { body } = render(DraftView, { props: { client: c } });
    // Serra Angel in the 5 column, Bolt in 0–1, Wilds under Land; no "?" column.
    expect(body).toMatch(/>5<\/span>[\s\S]*?data-id="c1"/);
    expect(body).toMatch(/>0–1<\/span>[\s\S]*?data-id="c2"/);
    expect(body).toMatch(/>Land<\/span>[\s\S]*?data-id="c3"/);
    expect(body).not.toMatch(/>\?<\/span>/);
    expect(body).toContain('ev-tint-W');
    expect(body).toContain('Creature Dragon'); // the inspector's type line is the wire's
    expect(fetch).not.toHaveBeenCalled();
  });

  it('a card the server sent no facts for is the only thing in "?"', () => {
    const c = new EventClient('e1', { seat: 0, token: 't' });
    c.handleFrame(frame('event_state', eventState()));
    c.handleFrame(frame('draft_pack', draftPack({ pool: [card('c1', 'Serra Angel'), card('c9', 'Mystery Card')] })));
    const { body } = render(DraftView, { props: { client: c } });
    expect(body).toMatch(/>\?<\/span>[\s\S]*?data-id="c9"/);
  });

  it('a heuristic-rated set carries no 17Lands credit', () => {
    const { body } = render(DraftView, { props: { client: drafting(), rated: false } });
    expect(body).not.toContain('17Lands');
  });
});

describe('DraftView pick timer', () => {
  afterEach(() => vi.useRealTimers());

  function timed(remaining: number) {
    const c = new EventClient('e1', { seat: 0, token: 'tok0' });
    c.handleFrame(frame('event_state', eventState({ pick_timer: 'standard' })));
    c.handleFrame(frame('draft_pack', draftPack({ time_limit_ms: 70_000, remaining_ms: remaining })));
    return c;
  }

  it('without a timer the header says "no pick timer"', () => {
    const { body } = render(DraftView, { props: { client: drafting() } });
    expect(body).toContain('no pick timer');
    expect(body).not.toContain('data-testid="pick-clock"');
  });

  it('counts down from remaining_ms as of the pack\'s arrival, warns under 10 s and shows "auto-picking…" at 0', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-10-13T12:00:00Z'));
    const c = timed(42_500);
    let { body } = render(DraftView, { props: { client: c } });
    expect(body).toMatch(/data-testid="pick-clock"[^>]*>\s*43s/);
    expect(body).not.toMatch(/class="clock[^"]*warn/);
    expect(body).not.toContain('no pick timer');

    vi.advanceTimersByTime(34_000); // 8.5 s left
    ({ body } = render(DraftView, { props: { client: c } }));
    expect(body).toMatch(/data-testid="pick-clock"[^>]*>\s*9s/);
    expect(body).toMatch(/class="clock[^"]*warn/);

    vi.advanceTimersByTime(9_000);
    ({ body } = render(DraftView, { props: { client: c } }));
    expect(body).toContain('auto-picking…');

    // The server's next pack restarts the count from its own remaining_ms.
    c.handleFrame(frame('draft_pack', draftPack({ pick_no: 4, time_limit_ms: 65_000, remaining_ms: 65_000 })));
    ({ body } = render(DraftView, { props: { client: c } }));
    expect(body).toMatch(/data-testid="pick-clock"[^>]*>\s*65s/);
  });
});

describe('PodStatus', () => {
  it('shows picks alone until a seat\'s queue count arrives — no placeholder dash', () => {
    const { body } = render(PodStatus, { props: { seats: eventState().seats, you: 0, progress: null } });
    expect(body).toContain('picks 2');
    expect(body).not.toContain('queued');
    expect(body).not.toContain('—');
  });
});

describe('BuildView', () => {
  function builder() {
    const b = new DeckBuilder('e1', 'tok0');
    b.adopt(eventPool());
    return b;
  }

  it('renders the two zones, the chips, the steppers and the server\'s deck check verbatim', () => {
    const b = builder();
    b.check = { valid: false, total: 19, error: 'build: deck has 19 cards, needs at least 40', auto_lands: [0, 0, 0, 10, 11] };
    const { body } = render(BuildView, { props: { builder: b, ev: eventState({ phase: 'BUILD' }) } });
    expect(body).toContain('data-testid="zone-deck"');
    expect(body).toContain('data-testid="zone-side"');
    expect(body).toContain('Colourless');
    expect(body).toMatch(/data-testid="deck-count"[^>]*>\s*19/);
    expect(body).toContain('build: deck has 19 cards, needs at least 40');
    expect(body).toMatch(/data-testid="basic-R"[^>]*>8</);
    expect(body).toMatch(/data-testid="basic-G"[^>]*>9</);
    expect(body).toContain('Auto would add: 10 Mountain, 11 Forest');
    expect(body).toMatch(/data-testid="ready"[^>]*disabled/);
    // Deck cards are draggable buttons in the deck zone; pool cards in the side zone.
    expect(body).toMatch(/draggable="true"[^>]*data-id="c2"|data-id="c2"[^>]*draggable="true"/);
    expect(body).toContain('Pick ratings and the suggested deck: data from');
  });

  it('a valid server check enables Ready', () => {
    const b = builder();
    b.check = { valid: true, total: 40, auto_lands: [0, 0, 0, 19, 19] };
    const { body } = render(BuildView, { props: { builder: b, ev: eventState({ phase: 'BUILD' }) } });
    expect(body).toContain('The server accepts this deck.');
    expect(body).not.toMatch(/data-testid="ready"[^>]*disabled/);
  });

  it('the colour filter narrows the pool zone only', () => {
    const b = builder();
    b.setFilter('W');
    const { body } = render(BuildView, { props: { builder: b, ev: eventState({ phase: 'BUILD' }) } });
    expect(body).toContain('Serra Angel'); // white, in the pool
    expect(body).not.toContain('Giant Growth'); // green, in the pool: filtered out
    expect(body).toContain('Lightning Bolt'); // in the deck: never filtered
  });

  it('curve, pips and the spell/land split come from the wire facts', () => {
    const b = builder();
    b.move('c1', 'deck'); // Serra Angel joins Bolt and Elves
    const { body } = render(BuildView, { props: { builder: b, ev: eventState({ phase: 'BUILD' }) } });
    expect(body).toContain('3 spells');
    expect(body).not.toMatch(/>\?<\/span>/);
  });

  it('a heuristic-rated set carries no 17Lands credit', () => {
    const { body } = render(BuildView, { props: { builder: builder(), ev: eventState({ phase: 'BUILD' }), rated: false } });
    expect(body).not.toContain('17Lands');
  });

  it('a save refusal is shown as the server worded it', () => {
    const b = builder();
    b.error = 'eventhost: build: deck has 19 cards, needs at least 40';
    const { body } = render(BuildView, { props: { builder: b, ev: eventState({ phase: 'BUILD' }) } });
    expect(body).toMatch(/data-testid="build-error"[^>]*>eventhost: build: deck has 19 cards/);
  });
});

describe('HubView and Standings', () => {
  const s = (over: Partial<EventSeries>): EventSeries => ({ round: 1, a: 0, b: 3, best_of: 1, state: 'pending', score: [0, 0], winner: null, ...over });

  it('the viewer\'s live game links to its seat-tokenised join path; other games are spectate links', () => {
    const ev = eventState({
      phase: 'ROUND1',
      round: 1,
      series: [
        s({ state: 'live', table: 'e1-r1-s03-g1-a1', attempt: 1, join: '/t/e1-r1-s03-g1-a1?seat=0&token=gt' }),
        s({ a: 1, b: 2, state: 'live', table: 'e1-r1-s12-g1-a1', attempt: 1 }),
        s({ round: 2, a: 0, b: 2 }),
      ],
    });
    const { body } = render(HubView, { props: { ev } });
    expect(body).toMatch(/href="\/t\/e1-r1-s03-g1-a1\?seat=0&amp;token=gt"[^>]*data-testid="join-game"|data-testid="join-game"[^>]*href="\/t\/e1-r1-s03-g1-a1\?seat=0&amp;token=gt"/);
    expect(body).toContain('href="/t/e1-r1-s12-g1-a1"');
    expect(body).toContain('Watch');
    expect(body).toContain('Round 1 · playing');
    expect(body).toContain('Round 2 · upcoming');
    expect(body.match(/data-testid="join-game"/g)).toHaveLength(1);
  });

  it('a best-of-3 row shows the series score, one dot per played game and the live game number', () => {
    const ev = eventState({
      phase: 'ROUND1',
      round: 1,
      best_of: 3,
      series: [
        s({ best_of: 3, state: 'live', table: 'e1-r1-s03-g1-a1', attempt: 1, game: 3, games: [3, 0], score: [1, 1], join: '/t/e1-r1-s03-g1-a1?seat=0&token=gt&event=e1' }),
        s({ a: 1, b: 2, best_of: 3, state: 'ended', table: 'e1-r1-s12-g1-a1', games: [1, 1], score: [2, 0], winner: 1 }),
      ],
    });
    const { body } = render(HubView, { props: { ev } });
    expect(body).toContain('1 – 1');
    expect(body).toContain('2 – 0');
    expect(body).toContain('live · game 3');
    const dots = [...body.matchAll(/<i class="dot (\w) /g)].map((m) => m[1]);
    // the viewer's row from the viewer's side (lost game 1, won game 2), the other row from its left seat's
    expect(dots).toEqual(['l', 'w', 'w', 'w']);
    expect(body.match(/data-testid="game-dots"/g)).toHaveLength(2);
  });

  it('a between-games series says so, counts as playing, and offers the viewer its deck for the next game', () => {
    const ev = eventState({
      phase: 'ROUND1',
      round: 1,
      best_of: 3,
      series: [
        s({ best_of: 3, state: 'between', table: 'e1-r1-s03-g1-a1', game: 2, games: [3], score: [0, 1], chooser: 0, waiting: [0] }),
        s({ a: 1, b: 2, best_of: 3, state: 'between', table: 'e1-r1-s12-g1-a1', game: 2, games: [-1], score: [0, 0], waiting: [] }),
      ],
    });
    const { body } = render(HubView, { props: { ev } });
    expect(body.match(/between games/g)).toHaveLength(2);
    expect(body).toContain('Round 1 · playing');
    expect(body).toMatch(/href="#between-games"[^>]*data-testid="your-deck"[^>]*>your deck for game 2/);
    expect(body.match(/data-testid="your-deck"/g)).toHaveLength(1);
    expect(body).toContain('href="/t/e1-r1-s12-g1-a1"'); // the other series stays watchable
    expect(body).toContain('<i class="dot d ');
  });

  it('the viewer\'s own between-games series carries the between-games panel inline; others do not', () => {
    const ev = eventState({
      phase: 'ROUND1',
      round: 1,
      best_of: 3,
      series: [
        s({ best_of: 3, state: 'between', table: 'e1-r1-s03-g1-a1', game: 2, games: [3], score: [0, 1], chooser: 0, waiting: [0, 3] }),
        s({ a: 1, b: 2, best_of: 3, state: 'between', table: 'e1-r1-s12-g1-a1', game: 2, games: [1], score: [1, 0], chooser: 2, waiting: [] }),
      ],
    });
    const { body } = render(HubView, { props: { ev, token: 'evtok0' } });
    expect(body.match(/data-testid="between-games"/g)).toHaveLength(1);
    expect(body).toContain('id="between-games"'); // the row's "your deck" link anchors here
    expect(body).toContain('You will choose play or draw');
    expect(body).toContain('Game 2 of 3 — you lost game 1');
    expect(body).not.toContain('evtok0');
    // A spectator (no seat token) gets no panel.
    expect(render(HubView, { props: { ev: { ...ev, you: null } } }).body).not.toContain('data-testid="between-games"');
  });

  it('a best-of-1 row has no game dots', () => {
    const ev = eventState({ phase: 'ROUND1', round: 1, series: [s({ state: 'ended', score: [1, 0], winner: 0, games: [0] })] });
    const { body } = render(HubView, { props: { ev } });
    expect(body).not.toContain('data-testid="game-dots"');
  });

  it('standings render the server rows in order with the MTR tiebreak columns', () => {
    const ev = eventState({
      phase: 'ROUND2',
      standings: [
        { seat: 2, points: 3, mw: 1, omw: 0.5, gw: 1, ogw: 0.5 },
        { seat: 0, points: 3, mw: 1, omw: 0.33, gw: 0.67, ogw: 0.4 },
        { seat: 1, points: 0, mw: 0.33, omw: 1, gw: 0.33, ogw: 0.8 },
        { seat: 3, points: 0, mw: 0.33, omw: 1, gw: 0.33, ogw: 0.7 },
      ],
      series: [s({ state: 'ended', score: [1, 0], winner: 0 }), s({ a: 1, b: 2, state: 'ended', score: [0, 1], winner: 2 })],
    });
    const { body } = render(Standings, { props: { rows: standingRows(ev.standings, ev.seats, ev.series, 0) } });
    for (const h of ['Pts', 'W-L', 'OMW%', 'GW%', 'OGW%']) expect(body).toContain(`>${h}<`);
    const order = [...body.matchAll(/data-seat="(\d)"/g)].map((m) => Number(m[1]));
    expect(order).toEqual([2, 0, 1, 3]); // the server's order, not re-ranked
    for (const l of ['Seat 3 · Bot 3 (bot)', '<b>You</b>', 'Seat 2 · Bot 2 (lethal-pressure)', 'Seat 4 · Bot 4 (bot)']) expect(body).toContain(l);
    expect(body).toContain('67%');
    expect(body).toContain('MTR Appendix C');
  });

  it('standings with no rows (the server sends none before the first result) say so, with no table', () => {
    const { body } = render(Standings, { props: { rows: [] } });
    expect(body).toContain('No results yet.');
    expect(body).toContain('data-testid="standings-empty"');
    expect(body).not.toContain('<table');
    expect(body).not.toContain('data-seat=');
  });

  it('a hub before any result (standings null or []) shows the empty state, not zero rows', () => {
    for (const standings of [[], null]) {
      const ev = eventState({ phase: 'ROUND1', round: 1, standings: standings as unknown as [] });
      const { body } = render(HubView, { props: { ev } });
      expect(body).toContain('No results yet.');
      expect(body).not.toContain('data-seat=');
    }
  });

  it('COMPLETE shows final standings and the revealed decks from the log', () => {
    const ev = eventState({ phase: 'COMPLETE', round: 3, standings: [{ seat: 0, points: 9, mw: 1, omw: 0.4, gw: 1, ogw: 0.4 }] });
    const log = [
      { kind: 'PackOpened', round: 1, cards: [{ id: 'c1', name: 'Serra Angel' }] },
      { kind: 'Picked', round: 1, pick_no: 1, id: 'c1' },
      { kind: 'DeckSaved', rev: 1, deck: { main: ['c1'], basics: [39, 0, 0, 0, 0] } },
    ];
    const { body } = render(HubView, { props: { ev, log } });
    expect(body).toContain('Event complete');
    expect(body).toContain('Final standings');
    expect(body).toContain('data-testid="revealed-deck"');
    expect(body).toContain('40 cards');
    expect(body).toContain('Serra Angel');
    expect(body).toContain('Plains');
  });
});

describe('LobbyView and PhaseRail', () => {
  it('offers the name form to an unjoined human seat', () => {
    const ev = eventState({ phase: 'LOBBY', seats: eventState().seats.map((x) => (x.seat === 0 ? { ...x, name: '', joined: false } : x)) });
    const { body } = render(LobbyView, { props: { ev, canJoin: true, onjoin: () => {} } });
    expect(body).toContain('data-testid="join-form"');
    expect(body).toContain('Join the pod');
    expect(body).toContain('bot · plays lethal-pressure');
    expect(body).toContain('best of 1 · no pick timer');
  });

  it('the lobby summary names best of 3 and the timer mode', () => {
    const ev = eventState({ phase: 'LOBBY', best_of: 3, pick_timer: 'relaxed' });
    const { body } = render(LobbyView, { props: { ev, canJoin: false, onjoin: () => {} } });
    expect(body).toContain('best of 3 · relaxed pick timer');
  });

  it('marks the current phase', () => {
    const { body } = render(PhaseRail, { props: { phase: 'ROUND2' } });
    expect(body).toMatch(/class="ph[^"]*now[^"]*"[^>]*aria-current="step"[^>]*>.*Round 2/s);
    expect(body).toMatch(/class="ph[^"]*done[^"]*"[^>]*>.*Draft/s);
  });
});
