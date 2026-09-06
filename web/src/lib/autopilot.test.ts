import { describe, expect, it } from 'vitest';
import { actionable, decide, STEPS, STOPPABLE_STEPS, turnSide, type Stops } from './autopilot';
import type { Decision, Option, View } from '../protocol';

const DEFAULT: Stops = {
  yours: new Set(['main1', 'declare-attackers', 'main2']),
  opponents: new Set(['declare-attackers', 'declare-blockers']),
};
const stops = (yours: string[], opponents: string[]): Stops => ({ yours: new Set(yours), opponents: new Set(opponents) });

/** view builds a View with only the fields decide reads: active (whose turn), step, stack (controller lists). */
const view = (active: number, step: string, stack: { id: number; controller: number }[] = []): View =>
  ({ active, step, stack } as unknown as View);

const opt = (kind: string, at: number): Option => ({ index: at, kind, label: kind, player: 0 });

const priority = (options: Option[], min = 1, max = 1, kind = 'priority'): Decision =>
  ({ seq: 1, player: 0, kind, prompt: 'p', min, max, options } as Decision);

const run = (decision: Decision, v: View, st = DEFAULT) =>
  decide({ decision, view: v, seat: 0, stops: st, enabled: true });

describe('turnSide', () => {
  it('is yours on the seat\u2019s own turn and opponents otherwise', () => {
    expect(turnSide(view(0, 'main1'), 0)).toBe('yours');
    expect(turnSide(view(1, 'main1'), 0)).toBe('opponents');
    expect(turnSide(view(2, 'main1'), 0)).toBe('opponents');
  });
});

describe('STEPS / STOPPABLE_STEPS', () => {
  it('STEPS is the twelve wire step names in engine order', () => {
    expect(STEPS).toEqual([
      'untap', 'upkeep', 'draw', 'main1', 'begin-combat',
      'declare-attackers', 'declare-blockers', 'combat-damage', 'end-combat',
      'main2', 'end', 'cleanup',
    ]);
  });
  it('STOPPABLE_STEPS is the ten steps that grant priority (untap and cleanup excluded)', () => {
    expect(STOPPABLE_STEPS).toEqual([
      'upkeep', 'draw', 'main1', 'begin-combat',
      'declare-attackers', 'declare-blockers', 'combat-damage', 'end-combat',
      'main2', 'end',
    ]);
    expect(STOPPABLE_STEPS).toHaveLength(10);
  });
});

describe('decide', () => {
  it('stops with disabled when autopilot is off, before anything else', () => {
    const d = priority([opt('pass', 0), opt('concede', 1)]);
    const out = decide({ decision: d, view: view(0, 'main1'), seat: 0, stops: DEFAULT, enabled: false });
    expect(out).toEqual({ act: 'stop', reason: 'disabled' });
  });

  it.each(['target', 'attackers', 'blockers', 'mulligan', 'trigger_order', 'trigger_optional', 'choose', 'modes'])(
    'stops a %s decision (never auto-answers a non-priority ask)',
    (kind) => {
      // The decision carries a pass option, so a mutated decide that
      // answered non-priority decisions would pass here instead of stopping.
      const d = priority([opt('pass', 0), opt('concede', 1)], 1, 1, kind);
      expect(run(d, view(0, 'main1'))).toEqual({ act: 'stop', reason: 'not-priority' });
    },
  );

  it('stops when min or max is not 1 (unexpected-shape)', () => {
    const s = stops([], []);
    expect(run(priority([opt('pass', 0), opt('concede', 1)], 0, 1), view(0, 'draw'), s))
      .toEqual({ act: 'stop', reason: 'unexpected-shape' });
    expect(run(priority([opt('pass', 0), opt('cast', 1), opt('concede', 2)], 1, 2), view(0, 'draw'), s))
      .toEqual({ act: 'stop', reason: 'unexpected-shape' });
  });

  it('stops when there are two pass options (unexpected-shape)', () => {
    const d = priority([opt('pass', 0), opt('pass', 1), opt('concede', 2)]);
    expect(run(d, view(0, 'draw'), stops([], []))).toEqual({ act: 'stop', reason: 'unexpected-shape' });
  });

  it('stops when there is no pass option (unexpected-shape)', () => {
    const d = priority([opt('cast', 0), opt('concede', 1)]);
    expect(run(d, view(0, 'draw'), stops([], []))).toEqual({ act: 'stop', reason: 'unexpected-shape' });
  });

  it('passes an option list of only pass+concede even with a stop set (nothing to do is always safe)', () => {
    const d = priority([opt('pass', 0), opt('concede', 1)]);
    const out = run(d, view(0, 'main1')); // main1 is in the default yours stops
    expect(out).toEqual({ act: 'pass', index: 0 });
    if (out.act === 'pass') expect(d.options[out.index].kind).toBe('pass');
  });

  it('passes when a cast is available, the stack is empty and no stop is set', () => {
    const d = priority([opt('pass', 0), opt('cast', 1), opt('concede', 2)]);
    const out = run(d, view(0, 'draw')); // draw is not in the default yours stops
    expect(out).toEqual({ act: 'pass', index: 0 });
    if (out.act === 'pass') expect(d.options[out.index].kind).toBe('pass');
  });

  it('stops when a cast is available and another player controls an object on the stack', () => {
    const d = priority([opt('pass', 0), opt('cast', 1), opt('concede', 2)]);
    expect(run(d, view(0, 'draw', [{ id: 9, controller: 1 }]))).toEqual({ act: 'stop', reason: 'has-action-and-stack' });
  });

  it('passes when a cast is available and only the seat\u2019s own object is on the stack', () => {
    const d = priority([opt('pass', 0), opt('cast', 1), opt('concede', 2)]);
    const out = run(d, view(0, 'draw', [{ id: 9, controller: 0 }]));
    expect(out).toEqual({ act: 'pass', index: 0 });
    if (out.act === 'pass') expect(d.options[out.index].kind).toBe('pass');
  });

  it('stops on a stop set for the current step of the current turn side', () => {
    const d = () => priority([opt('pass', 0), opt('cast', 1), opt('concede', 2)]);
    // my turn, my main1 is stopped
    expect(run(d(), view(0, 'main1'))).toEqual({ act: 'stop', reason: 'stop-set' });
    // their turn, their declare-blockers is stopped
    const st = stops([], ['declare-blockers']);
    const out = decide({ decision: d(), view: view(1, 'declare-blockers'), seat: 0, stops: st, enabled: true });
    expect(out).toEqual({ act: 'stop', reason: 'stop-set' });
  });

  it('ignores a stop whose turn side does not match the current turn', () => {
    const d = priority([opt('pass', 0), opt('cast', 1), opt('concede', 2)]);
    // their turn, but the stop is in YOURS only: keep passing
    const out1 = decide({ decision: d, view: view(1, 'main1'), seat: 0, stops: stops(['main1'], []), enabled: true });
    expect(out1).toEqual({ act: 'pass', index: 0 });
    // my turn, but the stop is in OPPONENTS only: keep passing
    const out2 = run(d, view(0, 'declare-blockers'), stops([], ['declare-blockers']));
    expect(out2).toEqual({ act: 'pass', index: 0 });
  });

  it('passes a storable decision whose option kinds are not one of the four known ones, as long as none is pass or concede', () => {
    // "activate"/"play_land" are real priority-window kinds (legal.go);
    // actionable() treats any non-pass/non-concede option as an action, so
    // the shape-invariant (pass index points at a pass option) still holds.
    const d = priority([opt('pass', 0), opt('play_land', 1), opt('activate', 2), opt('concede', 3)]);
    const out = run(d, view(0, 'draw'));
    expect(out).toEqual({ act: 'pass', index: 0 });
    if (out.act === 'pass') expect(d.options[out.index].kind).toBe('pass');
  });

  it('property: every pass verdict points at a pass option, over generated option-list permutations', () => {
    const KINDS = ['pass', 'concede', 'cast', 'ability'];
    const lists: Option[][] = [];
    const gen = (prefix: Option[], depth: number) => {
      if (depth === 5) return;
      for (const k of KINDS) {
        const at = prefix.length;
        const next = [...prefix, { index: at, kind: k, label: k, player: 0 } as Option];
        lists.push(next);
        gen(next, depth + 1);
      }
    };
    gen([], 1);
    // Two-pass lists (shape must stop them, but assert the invariant anyway).
    for (const n of [2, 3, 4]) {
      const list: Option[] = [];
      for (let i = 0; i < n; i++) list.push({ index: i, kind: i < 2 ? 'pass' : 'cast', label: String(i), player: 0 });
      lists.push(list);
    }
    for (const active of [0, 1]) {
      for (const step of ['main1', 'draw', 'declare-blockers', 'cleanup']) {
        for (const stackCtl of [-1, 0, 1]) {
          const v = view(active, step, stackCtl === -1 ? [] : [{ id: 9, controller: stackCtl }]);
          for (const st of [stops(['main1'], ['declare-blockers']), stops([], [])]) {
            for (const list of lists) {
              const d = priority(list);
              const out = decide({ decision: d, view: v, seat: 0, stops: st, enabled: true });
              if (out.act !== 'pass') continue;
              const o = d.options[out.index];
              expect(o, `pass index ${out.index} on ${JSON.stringify(list.map((x) => x.kind))} must be a pass option`).toBeDefined();
              expect(o.kind, `kind of option at pass index ${out.index} on ${JSON.stringify(list.map((x) => x.kind))}`).toBe('pass');
            }
          }
        }
      }
    }
  });

  it('actionable is false exactly when every option kind is pass or concede', () => {
    expect(actionable(priority([opt('pass', 0), opt('concede', 1)]))).toBe(false);
    expect(actionable(priority([opt('pass', 0)]))).toBe(false);
    expect(actionable(priority([opt('pass', 0), opt('cast', 1), opt('concede', 2)]))).toBe(true);
    expect(actionable(priority([opt('ability', 0)]))).toBe(true);
  });
});
