import { describe, expect, it } from 'vitest';
import type { Decision, Intent } from '../../protocol';
import { followupOf, nextAnswer, panelDecision, startWalk, substepDecision } from './answer';
import type { HumanExtension, NativeOp, Sbv2Request, ViewPayload } from './wire';

const option = (index: number, kind: string, label = kind) => ({ index, kind, label, player: 0 });

function payload(ops: NativeOp[], options = [option(0, 'a'), option(1, 'b'), option(2, 'c')], extra: Partial<Decision> = {}): ViewPayload {
  return {
    version: 1,
    native_index: 40,
    view: {} as ViewPayload['view'],
    decision: { seq: 40, player: 0, kind: 'choose', prompt: 'p', min: 0, max: 3, options, ...extra } as Decision,
    ops,
  };
}

const intent = (choices: number[], rest?: number[]): Intent => ({ seq: 7, player: 0, choices, ...(rest ? { rest } : {}) });

describe('nextAnswer', () => {
  it('walks a pick one choice per substep, then finishes', () => {
    const first = payload([{ op: 'choose', option: 0 }, { op: 'choose', option: 1 }, { op: 'choose', option: 2 }, { op: 'finish', option: -1 }]);
    const w = startWalk(intent([2, 0]), first);
    expect(nextAnswer(w, first)).toEqual({ candidate: 2 });
    const second = payload([{ op: 'choose', option: 0 }, { op: 'choose', option: 1 }, { op: 'finish', option: -1 }]);
    expect(nextAnswer(w, second)).toEqual({ candidate: 0 });
    expect(nextAnswer(w, payload([{ op: 'choose', option: 1 }, { op: 'finish', option: -1 }]))).toEqual({ candidate: 1 });
  });

  it('never finishes a pick early', () => {
    const p = payload([{ op: 'choose', option: 1 }, { op: 'finish', option: -1 }]);
    expect(nextAnswer(startWalk(intent([2]), p), p)).toBeNull();
  });

  it('answers each declaration unit: its option, or declines', () => {
    const attacker = (unit: number, option: number): ViewPayload => payload([{ op: 'choose', option, unit }, { op: 'none', option: -1, unit }]);
    const w = startWalk(intent([1]), attacker(5, 0));
    expect(nextAnswer(w, attacker(5, 0))).toEqual({ candidate: 1 });
    expect(nextAnswer(w, attacker(6, 1))).toEqual({ candidate: 0 });
  });

  it('partitions an arrangement by Choices, then orders both piles', () => {
    const dest = (option: number): ViewPayload => payload([{ op: 'dest', option, list: 'top' }, { op: 'dest', option, list: 'bottom' }]);
    const w = startWalk(intent([2, 0], [1]), dest(0));
    expect(nextAnswer(w, dest(0))).toEqual({ candidate: 0 });
    expect(nextAnswer(w, dest(1))).toEqual({ candidate: 1 });
    const order = payload([{ op: 'list', option: 0, list: 'choices', position: 0 }, { op: 'list', option: 2, list: 'choices', position: 0 }]);
    expect(nextAnswer(w, order)).toEqual({ candidate: 1 });
  });

  it('reads a destination inside Choices from the op over its name', () => {
    const p = payload([{ op: 'dest', option: 0, list: 'graveyard', inside: true }, { op: 'dest', option: 0, list: 'top', inside: false }]);
    expect(nextAnswer(startWalk(intent([0]), p), p)).toEqual({ candidate: 0 });
  });

  it('orders an unchosen pile as offered when the Intent has no Rest', () => {
    const p = payload([{ op: 'list', option: 1, list: 'rest', position: 0 }, { op: 'list', option: 2, list: 'rest', position: 0 }]);
    expect(nextAnswer(startWalk(intent([0]), p), p)).toEqual({ candidate: 0 });
  });

  it('leaves an order only a later decision holds to the player', () => {
    const p = payload([{ op: 'list', option: 0, list: 'followup:dig_bottom' }, { op: 'list', option: 1, list: 'followup:dig_bottom' }]);
    expect(nextAnswer(startWalk(intent([2]), p), p)).toBeNull();
  });

  it('orders a follow-up by an Intent on that decision', () => {
    const order = (position: number): NativeOp[] => [{ op: 'list', option: 0, list: 'followup:dig_bottom', position }, { op: 'list', option: 1, list: 'followup:dig_bottom', position }];
    const p = { ...payload(order(0)), followups: { dig_bottom: { seq: 40, kind: 'arrange' } as Decision } };
    expect(followupOf(p)).toBe('dig_bottom');
    expect(followupOf(payload([{ op: 'choose', option: 0 }]))).toBeNull();
    const w = startWalk(intent([1, 0]), p, 'dig_bottom');
    expect(nextAnswer(w, p)).toEqual({ candidate: 1 });
    expect(nextAnswer(w, { ...p, ops: order(1) })).toEqual({ candidate: 0 });
  });

  it('takes a substep with one candidate as forced', () => {
    const p = payload([{ op: 'dest', option: -1, list: 'bottom' }]);
    expect(nextAnswer(startWalk(intent([]), p), p)).toEqual({ candidate: 0 });
  });

  it('pays a cast by its action, on the cast and on an optional-cost follow-up', () => {
    const plan = { id: 'pl', activations: [] } as unknown as NonNullable<NativeOp['payment']>['plan'];
    const p = payload([{ op: 'choose', option: 0 }, { op: 'payment', option: -1, payment: { action_id: 'act', plan } }]);
    const w = startWalk({ seq: 7, player: 0, choices: [], payment: { action_id: 'act', plan } }, p);
    expect(nextAnswer(w, p)).toEqual({ candidate: 1 });
  });

  it('asks a covered cast first, then the variant', () => {
    const covered = payload([{ op: 'choose', option: 2 }, { op: 'cast', option: -1, covers: [1, 0] }]);
    const w = startWalk(intent([1]), covered);
    expect(nextAnswer(w, covered)).toEqual({ candidate: 1 });
    expect(nextAnswer(w, payload([{ op: 'choose', option: 0 }, { op: 'choose', option: 1 }]))).toEqual({ candidate: 1 });
  });

  it('answers a name past the candidate window by its label', () => {
    const p = payload([{ op: 'choose', option: 0 }], [option(0, 'name', 'Abu Jafar'), option(1, 'name', 'Zur the Enchanter')]);
    expect(nextAnswer(startWalk(intent([0]), p), p)).toEqual({ candidate: 0 });
    expect(nextAnswer(startWalk(intent([1]), p), p)).toEqual({ name: 'Zur the Enchanter' });
  });
});

describe('panelDecision', () => {
  const plan = { id: 'first', activations: [] } as unknown as NonNullable<NativeOp['payment']>['plan'];
  const other = { id: 'second', activations: [] } as unknown as NonNullable<NativeOp['payment']>['plan'];
  const options = [option(0, 'activate'), option(1, 'ability', 'Equip'), option(2, 'ability', 'Equip (alternate)'), option(3, 'pass'), option(4, 'concede')];
  const actions = [{ id: 'act', cast: { object: 1, face: 0, origin: 'hand' }, label: 'Cast', plans: [plan, other] }, { id: 'unposed', cast: { object: 2, face: 0, origin: 'hand' }, label: 'Cast', plans: [other] }];
  const p = payload([{ op: 'choose', option: 3 }, { op: 'choose', option: 1 }, { op: 'payment', option: -1, payment: { action_id: 'act', plan } }], options, { kind: 'priority', payment_actions: actions } as Partial<Decision>);

  it('numbers a decision by its request, so an undo never reads as stale', () => {
    expect(panelDecision(p, 9).seq).toBe(9);
  });

  it('keeps taps, concede and what a candidate can do, at their own indices', () => {
    expect(panelDecision(p, 9).options.map((o) => o.index)).toEqual([0, 1, 3, 4]);
  });

  it('offers each cast the one plan its candidate pays with', () => {
    expect(panelDecision(p, 9).payment_actions).toEqual([{ ...actions[0], plans: [plan] }]);
  });

  it('leaves another kind of decision whole: a substep names only part of it', () => {
    const q = payload([{ op: 'choose', option: 0 }]);
    expect(panelDecision(q, 9).options).toHaveLength(3);
  });
});

describe('substepDecision', () => {
  it('labels candidates from the labels, and from the semantic where those are empty', () => {
    const p = payload([{ op: 'list', option: 0, list: 'followup:dig_bottom' }, { op: 'dest', option: -1, list: 'bottom' }]);
    const human: HumanExtension = { log: [], labels: { 0: 'Island', 1: '' }, can_undo: false, native_only: false };
    const r: Sbv2Request = {
      request_id: '12',
      decision: {
        candidates: [
          { candidate_id: 0, semantic: { kind: 'order_pick', item: { object: { card_name: 'Island' } } } },
          { candidate_id: 1, semantic: { kind: 'arrange_card', card: { card_name: 'Mountain' }, destination: 'bottom' } },
        ],
        extensions: { x_gorge_view_v1: p, x_gorge_human_v1: human },
      },
    };
    const d = substepDecision(r, human, 12);
    expect(d.options.map((o) => [o.index, o.label])).toEqual([[0, 'Island'], [1, 'Mountain to bottom']]);
    expect([d.seq, d.min, d.max]).toEqual([12, 1, 1]);
  });
});
