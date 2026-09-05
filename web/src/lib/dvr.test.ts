import { describe, expect, it } from 'vitest';
import { behindLive, dvrReducer, eventAt, initialDvr, turnOf, type DvrState } from './dvr';
import type { EventBody } from '../protocol';

const ev = (seq: number, kind = 'tap'): EventBody => ({ event: { seq, kind, player: 0 }, line: `${kind} ${seq}` });

function live(seqs: number[], head0 = 100): DvrState {
  let s = dvrReducer(initialDvr, { type: 'snapshot', match: 't1/1', head: head0, turnStarts: [0, 40, 90] });
  for (const q of seqs) s = dvrReducer(s, { type: 'event', body: ev(q) });
  return s;
}

describe('dvr reducer', () => {
  it('starts live at the snapshot head', () => {
    const s = live([]);
    expect(s).toMatchObject({ match: 't1/1', head: 100, cursor: 100, live: true, events: [], gap: false });
  });
  it('follows contiguous events while live', () => {
    const s = live([101, 102, 103]);
    expect(s.head).toBe(103);
    expect(s.cursor).toBe(103);
    expect(behindLive(s)).toBe(0);
    expect(eventAt(s, 102)?.line).toBe('tap 102');
  });
  it('flags a gap on an out-of-order event and ignores duplicates', () => {
    let s = live([101]);
    s = dvrReducer(s, { type: 'event', body: ev(101) });
    expect(s.gap).toBe(false);
    expect(s.events.length).toBe(1);
    s = dvrReducer(s, { type: 'event', body: ev(105) });
    expect(s.gap).toBe(true);
  });
  it('pauses, counts behind-live, steps, and returns to live', () => {
    let s = live([101, 102]);
    s = dvrReducer(s, { type: 'pause' });
    s = dvrReducer(s, { type: 'event', body: ev(103) });
    s = dvrReducer(s, { type: 'event', body: ev(104) });
    expect(s.cursor).toBe(102);
    expect(behindLive(s)).toBe(2);
    s = dvrReducer(s, { type: 'step', by: -1 });
    expect(s.cursor).toBe(101);
    s = dvrReducer(s, { type: 'step', by: 1 });
    s = dvrReducer(s, { type: 'step', by: 1 });
    s = dvrReducer(s, { type: 'step', by: 1 });
    s = dvrReducer(s, { type: 'step', by: 1 });
    expect(s.cursor).toBe(104); // clamped at head, still paused
    expect(s.live).toBe(false);
    s = dvrReducer(s, { type: 'live' });
    expect(s).toMatchObject({ cursor: 104, live: true });
  });
  it('stepping back from live pauses; cursor never goes below 0', () => {
    let s = live([101]);
    s = dvrReducer(s, { type: 'step', by: -1 });
    expect(s.live).toBe(false);
    expect(s.cursor).toBe(100);
    s = dvrReducer(s, { type: 'scrub', seq: -5 });
    expect(s.cursor).toBe(0);
    s = dvrReducer(s, { type: 'scrub', seq: 999 });
    expect(s.cursor).toBe(101);
  });
  it('scrubs to turn starts and reports the current turn', () => {
    let s = live([101]);
    s = dvrReducer(s, { type: 'scrub', seq: 40 });
    expect(s.cursor).toBe(40);
    expect(turnOf(s, 40)).toBe(1);
    expect(turnOf(s, 39)).toBe(0);
    expect(turnOf(s, 95)).toBe(2);
    expect(turnOf(s, -1)).toBe(-1);
  });
  it('backfills older events in front of the known ones', () => {
    let s = live([101, 102]);
    s = dvrReducer(s, { type: 'backfill', events: [ev(98), ev(99), ev(100), ev(101)] });
    expect(s.events.map((e) => e.event.seq)).toEqual([98, 99, 100, 101, 102]);
  });
  it('a snapshot for another match resets everything', () => {
    let s = live([101]);
    s = dvrReducer(s, { type: 'snapshot', match: 't1/2', head: 7, turnStarts: [0] });
    expect(s).toMatchObject({ match: 't1/2', head: 7, cursor: 7, live: true, events: [], gap: false });
    expect(dvrReducer(s, { type: 'reset' })).toEqual(initialDvr);
  });
  it('a snapshot for the same match while paused keeps the cursor', () => {
    let s = live([101, 102]);
    s = dvrReducer(s, { type: 'pause' });
    s = dvrReducer(s, { type: 'snapshot', match: 't1/1', head: 150, turnStarts: [0, 40, 90, 130] });
    expect(s.cursor).toBe(102);
    expect(s.live).toBe(false);
    expect(s.head).toBe(150);
    expect(s.events).toEqual([]); // the client re-fetches the range it needs
    expect(s.gap).toBe(false);
  });
  it('ignores a redelivered event at or below head right after a snapshot dropped events', () => {
    let s = dvrReducer(initialDvr, { type: 'snapshot', match: 't4/1', head: 5, turnStarts: [0] });
    const before = s;
    s = dvrReducer(s, { type: 'event', body: ev(5) });
    expect(s).toBe(before); // untouched: seq 5 is not newer than head, nothing to do
    expect(s.gap).toBe(false);
    s = dvrReducer(s, { type: 'event', body: ev(6) });
    expect(s.head).toBe(6);
    expect(s.events.map((e) => e.event.seq)).toEqual([6]);
    expect(s.gap).toBe(false);
  });
  it('step by -1 walks past the first known event down to the hard floor of 0', () => {
    let s = live([101, 102]);
    expect(s.events[0].event.seq).toBe(101); // firstSeq
    s = dvrReducer(s, { type: 'step', by: -1 });
    expect(s.cursor).toBe(101); // at firstSeq
    s = dvrReducer(s, { type: 'step', by: -1 });
    expect(s.cursor).toBe(100); // below firstSeq: no event recorded there
    expect(eventAt(s, s.cursor)).toBeUndefined();
    for (let i = 0; i < 200; i++) s = dvrReducer(s, { type: 'step', by: -1 });
    expect(s.cursor).toBe(0); // clamps at 0, never negative
  });
  it('a gap leaves head and events untouched', () => {
    let s = live([101, 102]);
    const head = s.head;
    const events = s.events;
    s = dvrReducer(s, { type: 'event', body: ev(110) });
    expect(s.gap).toBe(true);
    expect(s.head).toBe(head);
    expect(s.events).toEqual(events);
  });
  it('turnOf returns -1 when turnStarts is empty', () => {
    const s = dvrReducer(initialDvr, { type: 'snapshot', match: 't5/1', head: 10, turnStarts: [] });
    expect(turnOf(s, 5)).toBe(-1);
  });
});
