import { describe, expect, it } from 'vitest';
import { hiddenKinds, isHiddenKind, isStepKind, visibleLog, type LogLine } from './logfilter';
import { dvrReducer, initialDvr } from './dvr';

// Realistic mixed event list: the noise kinds among real game actions, plus a
// blank state-only event that must always be dropped regardless of the toggle.
const line = (kind: string, text = `${kind} line`): LogLine => ({ line: text, event: { kind } });

const MIX: LogLine[] = [
  line('priority'),
  line('draw'),
  line('decision_ask'),
  line('land_played', 'Ari plays a Mountain'),
  line('priority'),
  line('stack_push', 'Ari casts Lightning Bolt'),
  line('decision_made'),
  line('move_zone', 'Bolt moves to graveyard'),
  line('priority'),
  line('stack_resolve'),
  line('priority'),
];

describe('logfilter hiddenKinds', () => {
  it('hides exactly priority, decision_ask, decision_made and end_combat_reset', () => {
    expect([...hiddenKinds].sort()).toEqual(['decision_ask', 'decision_made', 'end_combat_reset', 'priority']);
  });
  it('isHiddenKind matches the set and nothing else', () => {
    for (const k of hiddenKinds) expect(isHiddenKind(k)).toBe(true);
    expect(isHiddenKind('land_played')).toBe(false);
    expect(isHiddenKind('draw')).toBe(false);
    expect(isHiddenKind('stack_push')).toBe(false);
    // never hides a kind that is not in the set
    expect(isHiddenKind('move_zone')).toBe(false);
  });
});

describe('logfilter end_combat_reset (lc1)', () => {
  it('end_combat_reset is engine noise: hidden by default, not part of the step set', () => {
    expect(isHiddenKind('end_combat_reset')).toBe(true);
    expect(isStepKind('end_combat_reset')).toBe(false);
    expect(isStepKind('step')).toBe(true);
  });

  it('visibleLog hides "Combat ends" by default, reveals it with revealAll, and the step toggle is independent', () => {
    const list: LogLine[] = [
      line('end_combat_reset', 'Combat ends'),
      line('land_played', 'Ari plays a Mountain'),
      line('step', 'Step: end-combat'),
      line('turn', 'Turn 3: Ari'),
    ];
    // default: Combat ends and the step line both hidden, real actions kept
    expect(visibleLog(list, false).map((l) => l.event.kind)).toEqual(['land_played', 'turn']);
    // revealSteps shows the phase boundary but NOT the end_combat_reset noise
    expect(visibleLog(list, false, true).map((l) => l.event.kind)).toEqual(['land_played', 'step', 'turn']);
    // revealAll shows the Combat ends line too
    expect(visibleLog(list, true).map((l) => l.event.kind)).toEqual(['end_combat_reset', 'land_played', 'step', 'turn']);
  });
});

describe('logfilter step lines (B4)', () => {
  it('step lines are hidden by default and are not part of the engine-noise set', () => {
    expect(isHiddenKind('step')).toBe(false);
    expect(isStepKind('step')).toBe(true);
    expect(isStepKind('turn')).toBe(false);
  });

  it('visibleLog drops step lines unless revealSteps, and the toggles are independent', () => {
    const list: LogLine[] = [
      line('step', 'Step: main-1'),
      line('land_played', 'Ari plays a Mountain'),
      line('step', 'Step: main-2'),
      line('priority', 'Ari has priority'),
      line('turn', 'Turn 3: Ari'),
    ];
    // default: steps and noise hidden, real actions and turn lines kept
    expect(visibleLog(list, false).map((l) => l.event.kind)).toEqual(['land_played', 'turn']);
    // revealSteps without revealing noise: steps come back, priority stays hidden
    expect(visibleLog(list, false, true).map((l) => l.event.kind)).toEqual(['step', 'land_played', 'step', 'turn']);
    // revealAll shows everything
    expect(visibleLog(list, true).map((l) => l.event.kind)).toEqual(['step', 'land_played', 'step', 'priority', 'turn']);
  });

  it('a step line is still a valid DVR scrub target — the filter never touches seq reachability', () => {
    const list: LogLine[] = [line('step', 'Step: main-1'), line('draw', 'Ari draws a card')];
    const hidden = visibleLog(list, false).map((l) => l.event.kind);
    expect(hidden).toEqual(['draw']);
    // the step seq is still reachable even though it is hidden from render
    let s = dvrReducer(initialDvr, { type: 'snapshot', match: 't1/1', head: 0, turnStarts: [0] });
    s = dvrReducer(s, { type: 'event', body: { event: { seq: 1, kind: 'step', player: 0 }, line: 'Step: main-1' } });
    s = dvrReducer(s, { type: 'scrub', seq: 1 });
    expect(s.cursor).toBe(1);
    expect(s.live).toBe(false);
  });
});

describe('visibleLog', () => {

  it('by default excludes exactly the three hidden kinds and keeps everything else', () => {
    const visible = visibleLog(MIX, false);
    const kinds = visible.map((l) => l.event.kind);
    expect(kinds).toEqual(['draw', 'land_played', 'stack_push', 'move_zone', 'stack_resolve']);
    expect(kinds).not.toContain('priority');
    expect(kinds).not.toContain('decision_ask');
    expect(kinds).not.toContain('decision_made');
  });

  it('the toggle reveals the full list, including the noise, except blank lines', () => {
    const revealed = visibleLog(MIX, true);
    expect(revealed.length).toBe(MIX.length);
    // sequence of kinds unchanged from the source
    expect(revealed.map((l) => l.event.kind)).toEqual(MIX.map((l) => l.event.kind));
  });

  it('always drops blank (state-only) lines, hidden or revealed', () => {
    const list = [...MIX, line('priority', ''), line('draw', ''), line('land_played', 'Mountain on table')];
    expect(visibleLog(list, false).map((l) => l.event.kind)).toEqual([
      'draw', 'land_played', 'stack_push', 'move_zone', 'stack_resolve', 'land_played',
    ]);
    // the two blank ones (priority, draw) are gone even when revealing all
    expect(visibleLog(list, true).map((l) => l.event.kind)).toEqual([
      // the MIX sequence unchanged, then the non-blank land_played
      ...MIX.map((l) => l.event.kind), 'land_played',
    ]);
  });

  it('a hidden line is still a valid DVR scrub target — the filter never touches seq reachability', () => {
    // The filter only selects for *rendering*; the DVR owns scrubbing over the
    // full contiguous seq range and never consults the filter, so a hidden
    // kind's seq must remain reachable by a scrub/step exactly like any other.
    const seqs = MIX.map((l, i) => i + 1); // 1..11, contiguous over the fixture
    const bySeq = (seq: number) => MIX[seq - 1];
    const hiddenSeqs = seqs.filter((s) => isHiddenKind(bySeq(s).event.kind));
    expect(hiddenSeqs).toEqual([1, 3, 5, 7, 9, 11]);

    // Drive the real DVR reducer: land a snapshot, append the whole mixed
    // burst, then scrub to each hidden seq and assert the cursor actually
    // lands there (clamped within head) — reachable despite being hidden.
    let s = dvrReducer(initialDvr, { type: 'snapshot', match: 't1/1', head: 0, turnStarts: [0] });
    for (const i of seqs) {
      const l = bySeq(i);
      s = dvrReducer(s, { type: 'event', body: { event: { seq: i, kind: l.event.kind, player: 0 }, line: l.line } });
    }
    expect(s.head).toBe(11);
    for (const h of hiddenSeqs) {
      s = dvrReducer(s, { type: 'scrub', seq: h });
      expect(s.cursor).toBe(h); // landed on the hidden seq
      expect(s.live).toBe(false);
    }
  });
});
