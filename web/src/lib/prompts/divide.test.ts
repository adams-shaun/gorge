import { describe, expect, it } from 'vitest';
import type { Decision, Option } from '../../protocol';
import { rendererFor } from './renderer';
import { canStepUp, canSubmitDivide, divisionAnswer, expectationOf, isDivideTarget, remaining, step, targetChoices, type Counts } from './divide';

const obj = (index: number, o: number, extra: Partial<Option> = {}) => ({ index, kind: 'creature', label: `c${o}`, obj: o, player: 0, ...extra }) as Option;
const target = (total: number, options: Option[], over: Partial<Decision> = {}) =>
  ({ seq: 5, player: 0, kind: 'target', prompt: '', min: 0, max: 4, source: 77, divide_total: total, options, ...over }) as unknown as Decision;
const fury = () => target(4, [obj(0, 11), obj(1, 12), obj(2, 13)]);

describe('divide prompt stepper', () => {
  it('routes only a target ask carrying a total', () => {
    expect(isDivideTarget(fury())).toBe(true);
    expect(rendererFor(fury())).toBe('divide');
    expect(rendererFor(target(0, [obj(0, 11)]))).toBe('target');
  });

  it('counts down the points left and stops at zero', () => {
    const d = fury();
    let c: Counts = {};
    expect(remaining(d, c)).toBe(4);
    c = step(d, c, 0, 1);
    c = step(d, c, 0, 1);
    c = step(d, c, 1, 1);
    expect(remaining(d, c)).toBe(1);
    c = step(d, c, 2, 1);
    expect(remaining(d, c)).toBe(0);
    expect(canStepUp(d, c, 0)).toBe(false);
    expect(step(d, c, 0, 1)).toBe(c);
  });

  it('never goes below zero, and a target back at 0 is not targeted', () => {
    const d = fury();
    let c = step(d, {}, 1, 1);
    expect(step(d, c, 0, -1)).toBe(c);
    c = step(d, c, 1, -1);
    expect(targetChoices(d, c)).toEqual([]);
  });

  it('submits only when every point is assigned, answering the targets in list order', () => {
    const d = fury();
    let c: Counts = {};
    c = step(d, c, 2, 1);
    c = step(d, c, 0, 1);
    expect(canSubmitDivide(d, c)).toBe(false);
    c = step(d, c, 2, 1);
    c = step(d, c, 2, 1);
    expect(canSubmitDivide(d, c)).toBe(true);
    expect(targetChoices(d, c)).toEqual([0, 2]);
  });

  it('will not include more targets than the ask allows', () => {
    const d = target(4, [obj(0, 11), obj(1, 12), obj(2, 13)], { max: 2 });
    let c = step(d, {}, 0, 1);
    c = step(d, c, 1, 1);
    expect(canStepUp(d, c, 2)).toBe(false);
    expect(canStepUp(d, c, 0)).toBe(true);
  });
});

describe('divide prompt follow-up matching', () => {
  const division = (options: Option[], min: number, over: Partial<Decision> = {}) =>
    ({ seq: 6, player: 0, kind: 'choose', prompt: '', min, max: min, repeatable: true, source: 77, options, ...over }) as unknown as Decision;
  const dopt = (index: number, o: number) => ({ index, kind: 'divide', label: `c${o}`, obj: o, player: 0 }) as Option;

  it('answers each target count-1 extra picks, matching by obj not position', () => {
    const d = fury();
    // 3 on c11, 1 on c13
    const c: Counts = { 0: 3, 2: 1 };
    const want = expectationOf(d, c);
    // the engine lists them in the other order
    const ask = division([dopt(0, 13), dopt(1, 11)], 2);
    expect(divisionAnswer(ask, want)).toEqual([1, 1]);
  });

  it('matches players by seat', () => {
    const d = target(3, [{ index: 0, kind: 'player', label: 'p', player: 1 } as Option, obj(1, 12)]);
    const want = expectationOf(d, { 0: 2, 1: 1 });
    const ask = division([{ index: 0, kind: 'divide', label: 'p', player: 1 } as Option, dopt(1, 12)], 1);
    expect(divisionAnswer(ask, want)).toEqual([0]);
  });

  it('refuses a different source, different targets, or another kind of ask', () => {
    const want = expectationOf(fury(), { 0: 3, 1: 1 });
    expect(divisionAnswer(division([dopt(0, 11), dopt(1, 12)], 2, { source: 99 }), want)).toBeNull();
    expect(divisionAnswer(division([dopt(0, 11), dopt(1, 14)], 2), want)).toBeNull();
    expect(divisionAnswer(division([dopt(0, 11)], 2), want)).toBeNull();
    expect(divisionAnswer(division([dopt(0, 11), dopt(1, 12)], 3), want)).toBeNull();
    expect(divisionAnswer({ ...fury() }, want)).toBeNull();
  });

  it('accepts any source when the ability target ask had none', () => {
    const d = target(4, [obj(0, 11), obj(1, 12)], { source: 0 });
    const want = expectationOf(d, { 0: 3, 1: 1 });
    expect(divisionAnswer(division([dopt(0, 11), dopt(1, 12)], 2, { source: 5 }), want)).toEqual([0, 0]);
  });
});
