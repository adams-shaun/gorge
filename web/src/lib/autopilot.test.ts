import { describe, expect, it } from 'vitest';
import { actionable, actionables, decide, emptyPriorityWindow, respondable, respondableFor, STEPS, STOPPABLE_STEPS, turnSide } from './autopilot';
import { applyPreset, defaultSettings, type PlaySettings, type StoppableStep, type StepStop } from './playsettings';
import type { CardView, Decision, Option, PlayerView, View } from '../protocol';

/** view builds a View with only the fields decide reads: active (whose turn), step, stack, and the battlefield data the targets-me lookup reads. */
const view = (
  active: number,
  step: string,
  stack: { id: number; controller: number; kind?: string; targets?: { obj?: number; player: number; is_player: boolean }[] }[] = [],
  battlefield: { seat: number; cards: number[] }[] = [],
): View =>
  ({
    active,
    step,
    stack,
    players: battlefield.map((b) => ({ seat: b.seat, battlefield: b.cards.map((id) => ({ id, controller: b.seat })) })),
  }) as unknown as View;

const opt = (kind: string, at: number): Option => ({ index: at, kind, label: kind, player: 0 });

const priority = (options: Option[], min = 1, max = 1, kind = 'priority'): Decision =>
  ({ seq: 1, player: 0, kind, prompt: 'p', min, max, options } as Decision);

/** stackEntry builds one StackView-shaped entry; kind defaults to the view's "spell" (view/view.go:755). */
const stackEntry = (
  id: number,
  controller: number,
  kind = 'spell',
  targets: { obj?: number; player: number; is_player: boolean }[] = [],
) => ({ id, controller, kind, targets });

/** handCard builds one minimal CardView for a hand entry; only the fields castableAfterTap reads are named. */
const handCard = (over: Partial<CardView>): CardView =>
  ({
    id: 1,
    name: 'Card',
    types: 'Instant',
    mana_cost: 'R',
    controller: 0,
    owner: 0,
    ...over,
  }) as CardView;

/** withHand returns the view with seat `seat`'s own hand, pool and availability attached (the viewer's own hidden zone). */
const withHand = (v: View, seat: number, over: Partial<PlayerView>): View => {
  const players = [...(v.players ?? [])];
  const at = players.findIndex((p) => p.seat === seat);
  const base = at >= 0 ? players[at] : { seat };
  const merged = {
    life: 20,
    lost: false,
    hand: [],
    battlefield: [],
    graveyard: [],
    exile: [],
    pool: {},
    command: [],
    commanders: [],
    commander_casts: [],
    ...base,
    ...over,
  } as PlayerView;
  if (at >= 0) players[at] = merged;
  else players.push(merged);
  return { ...v, players } as View;
};

const RESPONDABLE = [opt('pass', 0), opt('cast', 1), opt('concede', 2)];
const ONLY_MANA = [opt('activate', 0), opt('pass', 1), opt('concede', 2)];

const run = (decision: Decision, v: View, settings: PlaySettings = defaultSettings()) =>
  decide({ decision, view: v, seat: 0, settings });

/** runFfwd is decide() in the one-shot fast-forward mode: the stack rules are skipped, the step rules are not. */
const runFfwd = (decision: Decision, v: View, settings: PlaySettings = defaultSettings()) =>
  decide({ decision, view: v, seat: 0, settings, ffwd: true });

/** withSteps returns a copy of casual with one side's step rules replaced wholesale. */
const withSteps = (side: 'yours' | 'opponents', rules: Partial<Record<StoppableStep, StepStop>>): PlaySettings => {
  const s = applyPreset('casual');
  for (const [k, v] of Object.entries(rules)) s.steps[side][k as StoppableStep] = v as StepStop;
  return s;
};

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
  it('stops with disabled when autoPass is off, before any rule is consulted', () => {
    const s = withSteps('yours', {}); // casual, but every step rule off
    s.autoPass = false;
    expect(run(priority(RESPONDABLE), view(0, 'main1'), s)).toEqual({ act: 'stop', reason: 'disabled' });
    // ffwd outranks the master switch: the run proceeds to the step rules
    expect(runFfwd(priority(RESPONDABLE), view(0, 'draw'), s)).toEqual({ act: 'pass', index: 0 });
  });

  it('stops a non-priority decision (never auto-answers a non-priority ask), even with autoPass on', () => {
    const d = priority([opt('pass', 0), opt('concede', 1)], 1, 1, 'target');
    expect(run(d, view(0, 'main1'))).toEqual({ act: 'stop', reason: 'not-priority' });
    expect(runFfwd(d, view(0, 'draw'))).toEqual({ act: 'stop', reason: 'not-priority' });
  });

  it.each(['attackers', 'blockers', 'mulligan', 'trigger_order', 'trigger_optional', 'choose', 'modes'])(
    'stops every non-priority %s ask',
    (kind) => {
      // The decision carries a pass option, so a mutated decide that
      // answered non-priority decisions would pass here instead of stopping.
      const d = priority([opt('pass', 0), opt('concede', 1)], 1, 1, kind);
      expect(run(d, view(0, 'main1'))).toEqual({ act: 'stop', reason: 'not-priority' });
    },
  );

  it('stops when min or max is not 1 (unexpected-shape)', () => {
    expect(run(priority([opt('pass', 0), opt('concede', 1)], 0, 1), view(0, 'draw')))
      .toEqual({ act: 'stop', reason: 'unexpected-shape' });
    expect(run(priority([opt('pass', 0), opt('cast', 1), opt('concede', 2)], 1, 2), view(0, 'draw')))
      .toEqual({ act: 'stop', reason: 'unexpected-shape' });
  });

  it('stops when there are two pass options (unexpected-shape)', () => {
    const d = priority([opt('pass', 0), opt('pass', 1), opt('concede', 2)]);
    expect(run(d, view(0, 'draw'))).toEqual({ act: 'stop', reason: 'unexpected-shape' });
  });

  it('stops when there is no pass option (unexpected-shape)', () => {
    const d = priority([opt('cast', 0), opt('concede', 1)]);
    expect(run(d, view(0, 'draw'))).toEqual({ act: 'stop', reason: 'unexpected-shape' });
  });

  // --- stack rules: opponent objects (casual: if-respondable / targets-me-if-respondable) ---

  it('casual: an opponent spell on top with a cast available stops (if-respondable)', () => {
    const d = priority(RESPONDABLE);
    expect(run(d, view(0, 'draw', [stackEntry(9, 1, 'spell')]))).toEqual({ act: 'stop', reason: 'opponent-object' });
  });

  it('casual: an opponent spell on top with nothing to respond with passes (if-respondable)', () => {
    const d = priority(ONLY_MANA);
    expect(run(d, view(0, 'draw', [stackEntry(9, 1, 'spell')]))).toEqual({ act: 'pass', index: 1 });
  });

  // --- the if-respondable arms see the tap-then-cast hand (fb-20260914T114244Z):
  // the engine prices a cast against the FLOATING pool only, so a window where the
  // player holds a castable counterspell and the untapped lands to pay for it
  // offers nothing but mana taps — respondable()'s option-kind test read that
  // window as dead and silently ate the response the rule exists to protect.

  it('casual: the report shape — opponent spell on top, mana-only window, Mana Leak + 2 untapped Islands in view STOPS', () => {
    const d = priority(ONLY_MANA); // activate + pass + concede — the window as the engine offers it, pool empty
    const v = withHand(view(0, 'draw', [stackEntry(9, 1, 'spell')]), 0, {
      hand: [handCard({ name: 'Mana Leak', mana_cost: '1 U' })], // handCard defaults types to Instant
      pool: {},
      available: { U: 2 },
    });
    expect(run(d, v)).toEqual({ act: 'stop', reason: 'opponent-object' });
  });

  it('casual: the same shape with NO hand still passes (the empty-hand pin above stays valid)', () => {
    const d = priority(ONLY_MANA);
    const v = withHand(view(0, 'draw', [stackEntry(9, 1, 'spell')]), 0, { hand: [], available: { U: 2 } });
    expect(run(d, v)).toEqual({ act: 'pass', index: 1 });
  });

  it('casual: a SORCERY castable after tapping does NOT stop an opponent-spell window (the timing filter)', () => {
    const d = priority(ONLY_MANA);
    const v = withHand(view(0, 'draw', [stackEntry(9, 1, 'spell')]), 0, {
      hand: [handCard({ types: 'Sorcery', mana_cost: '1 U' })],
      available: { U: 2 },
    });
    expect(run(d, v)).toEqual({ act: 'pass', index: 1 });
  });

  it('casual: a FLASH creature castable after tapping stops an opponent-spell window', () => {
    const d = priority(ONLY_MANA);
    const v = withHand(view(0, 'draw', [stackEntry(9, 1, 'spell')]), 0, {
      hand: [handCard({ types: 'Creature Bear', mana_cost: '1 U', keywords: ['Flash'] })],
      available: { U: 2 },
    });
    expect(run(d, v)).toEqual({ act: 'stop', reason: 'opponent-object' });
  });

  it('casual: an instant that is NOT affordable after tapping still passes (the money half applies)', () => {
    const d = priority(ONLY_MANA);
    const v = withHand(view(0, 'draw', [stackEntry(9, 1, 'spell')]), 0, {
      hand: [handCard({ mana_cost: '4 U' })],
      available: { U: 2 },
    });
    expect(run(d, v)).toEqual({ act: 'pass', index: 1 });
  });

  it('casual: a targets-me opponent trigger + mana-only window + castable instant in hand STOPS (targets-me-if-respondable)', () => {
    const d = priority(ONLY_MANA);
    const v = withHand(view(0, 'draw', [stackEntry(9, 1, 'trigger', [{ player: 0, is_player: true }])]), 0, {
      hand: [handCard({ mana_cost: '1 U' })],
      available: { U: 2 },
    });
    expect(run(d, v)).toEqual({ act: 'stop', reason: 'opponent-object' });
  });

  it('casual: a targets-me opponent trigger + mana-only window + castable SORCERY in hand passes (timing, not money)', () => {
    const d = priority(ONLY_MANA);
    const v = withHand(view(0, 'draw', [stackEntry(9, 1, 'trigger', [{ player: 0, is_player: true }])]), 0, {
      hand: [handCard({ types: 'Sorcery', mana_cost: '1 U' })],
      available: { U: 2 },
    });
    expect(run(d, v)).toEqual({ act: 'pass', index: 1 });
  });

  it('casual: a trigger NOT targeting me passes even with a castable instant in hand (targets-me still gates the trigger arm)', () => {
    const d = priority(ONLY_MANA);
    const v = withHand(view(0, 'draw', [stackEntry(9, 1, 'trigger', [{ player: 1, is_player: true }])]), 0, {
      hand: [handCard({ mana_cost: '1 U' })],
      available: { U: 2 },
    });
    expect(run(d, v)).toEqual({ act: 'pass', index: 1 });
  });

  it("full-control's ownObjects if-respondable: an OWN object on top + mana-only window + castable instant STOPS (own-object)", () => {
    const d = priority(ONLY_MANA);
    const s = { ...applyPreset('full-control'), autoPass: true };
    const v = withHand(view(0, 'draw', [stackEntry(9, 0, 'spell')]), 0, {
      hand: [handCard({ mana_cost: '1 U' })],
      available: { U: 2 },
    });
    expect(run(d, v, s)).toEqual({ act: 'stop', reason: 'own-object' });
  });

  it("respondableFor is respondable OR the instant-speed hand scan, and never asks another seat's hand", () => {
    const d = priority(ONLY_MANA);
    const holding = withHand(view(0, 'draw'), 0, { hand: [handCard({ mana_cost: '1 U' })], available: { U: 2 } });
    expect(respondableFor(holding, 0, d)).toBe(true);
    expect(respondableFor(view(0, 'draw'), 0, d)).toBe(false);
    expect(respondableFor(holding, 1, d)).toBe(false); // another seat's hand is a hidden zone
    const withCast = priority([opt('pass', 0), opt('cast', 1), opt('concede', 2)]);
    expect(respondableFor(view(0, 'draw'), 0, withCast)).toBe(true);
    const sorceryOnly = withHand(view(0, 'draw'), 0, { hand: [handCard({ types: 'Sorcery', mana_cost: '1 U' })], available: { U: 2 } });
    expect(respondableFor(sorceryOnly, 0, d)).toBe(false);
  });

  it('casual: an opponent ability on top with a cast available stops (if-respondable)', () => {
    const d = priority(RESPONDABLE);
    expect(run(d, view(0, 'draw', [stackEntry(9, 1, 'ability')]))).toEqual({ act: 'stop', reason: 'opponent-object' });
  });

  it('casual: an opponent trigger targeting MY creature + respondable stops (targets-me-if-respondable)', () => {
    const d = priority(RESPONDABLE);
    // my creature (id 5) on my battlefield, targeted by the opponent's trigger
    const v = view(0, 'draw', [stackEntry(9, 1, 'trigger', [{ obj: 5, player: 0, is_player: false }])], [{ seat: 0, cards: [5] }]);
    expect(run(d, v)).toEqual({ act: 'stop', reason: 'opponent-object' });
  });

  it('casual: an opponent trigger targeting an opponent creature + respondable passes', () => {
    const d = priority(RESPONDABLE);
    // the trigger targets the opponent's own creature (id 5, controller 1)
    const v = view(0, 'draw', [stackEntry(9, 1, 'trigger', [{ obj: 5, player: 1, is_player: false }])], [{ seat: 1, cards: [5] }]);
    expect(run(d, v)).toEqual({ act: 'pass', index: 0 });
  });

  it('casual: an opponent trigger targeting ME (a player target) + respondable stops', () => {
    const d = priority(RESPONDABLE);
    const v = view(0, 'draw', [stackEntry(9, 1, 'trigger', [{ player: 0, is_player: true }])]);
    expect(run(d, v)).toEqual({ act: 'stop', reason: 'opponent-object' });
  });

  it('casual: an opponent trigger targeting the opponent passes even when respondable', () => {
    const d = priority(RESPONDABLE);
    const v = view(0, 'draw', [stackEntry(9, 1, 'trigger', [{ player: 1, is_player: true }])]);
    expect(run(d, v)).toEqual({ act: 'pass', index: 0 });
  });

  // --- targets-me counts MY objects ON THE STACK (prio7) ---

  it('casual: an opponent trigger targeting MY spell lower on the stack + respondable stops', () => {
    const d = priority(RESPONDABLE);
    // my spell (id 8, controller 0) under the opponent's trigger (id 9) that targets it
    const v = view(0, 'draw', [
      stackEntry(8, 0, 'spell'),
      stackEntry(9, 1, 'trigger', [{ obj: 8, player: 0, is_player: false }]),
    ]);
    expect(run(d, v)).toEqual({ act: 'stop', reason: 'opponent-object' });
  });

  it('casual: an opponent trigger targeting the OPPONENT’s own spell on the stack passes', () => {
    const d = priority(RESPONDABLE);
    const v = view(0, 'draw', [
      stackEntry(8, 1, 'spell'),
      stackEntry(9, 1, 'trigger', [{ obj: 8, player: 1, is_player: false }]),
    ]);
    expect(run(d, v)).toEqual({ act: 'pass', index: 0 });
  });

  it('casual: an opponent trigger targeting MY ability on the stack + respondable stops', () => {
    const d = priority(RESPONDABLE);
    const v = view(0, 'draw', [
      stackEntry(8, 0, 'ability'),
      stackEntry(9, 1, 'trigger', [{ obj: 8, player: 0, is_player: false }]),
    ]);
    expect(run(d, v)).toEqual({ act: 'stop', reason: 'opponent-object' });
  });

  it('casual: an opponent trigger targeting MY card in my graveyard + respondable stops (public-zone controller)', () => {
    const d = priority(RESPONDABLE);
    const v = view(
      0, 'draw',
      [stackEntry(9, 1, 'trigger', [{ obj: 5, player: 0, is_player: false }])],
      [{ seat: 0, cards: [] }],
    );
    (v.players as { seat: number; graveyard: { id: number; controller: number }[] }[])[0].graveyard = [{ id: 5, controller: 0 }];
    expect(run(d, v)).toEqual({ act: 'stop', reason: 'opponent-object' });
  });

  it('no-tells: the same opponent trigger targeting the opponent\u2019s own creature stops (always)', () => {
    const d = priority(RESPONDABLE);
    const s = applyPreset('no-tells');
    const v = view(0, 'draw', [stackEntry(9, 1, 'trigger', [{ obj: 5, player: 1, is_player: false }])], [{ seat: 1, cards: [5] }]);
    expect(run(d, v, s)).toEqual({ act: 'stop', reason: 'opponent-object' });
  });

  it('no-tells: an opponent spell + NOT respondable still stops (always)', () => {
    const d = priority(ONLY_MANA);
    expect(run(d, view(0, 'draw', [stackEntry(9, 1, 'spell')]), applyPreset('no-tells')))
      .toEqual({ act: 'stop', reason: 'opponent-object' });
  });

  it('no-tells: an opponent spell with an empty option side still stops even when only mana is offered', () => {
    const d = priority([opt('pass', 0), opt('concede', 1)]);
    expect(run(d, view(0, 'draw', [stackEntry(9, 1, 'spell')]), applyPreset('no-tells')))
      .toEqual({ act: 'stop', reason: 'opponent-object' });
  });

  it('ffwd passes through the stack rules: an opponent spell under no-tells still passes the one-shot run', () => {
    const d = priority(RESPONDABLE);
    const s = applyPreset('no-tells');
    expect(runFfwd(d, view(0, 'draw', [stackEntry(9, 1, 'spell')]), s)).toEqual({ act: 'pass', index: 0 });
  });

  it('only the TOP of the stack is classified: my own spell over the opponent\u2019s does not trigger the opponent rule (casual)', () => {
    const d = priority(RESPONDABLE);
    const v = view(0, 'draw', [stackEntry(8, 1, 'spell'), stackEntry(9, 0, 'spell')]);
    expect(run(d, v)).toEqual({ act: 'pass', index: 0 });
  });

  // --- stack rules: own objects ---

  it('casual: my own spell on top + respondable passes (ownObjects never)', () => {
    const d = priority(RESPONDABLE);
    expect(run(d, view(0, 'draw', [stackEntry(9, 0, 'spell')]))).toEqual({ act: 'pass', index: 0 });
  });

  it('full-control: my own spell on top + respondable stops (ownObjects if-respondable)', () => {
    const d = priority(RESPONDABLE);
    // full-control's autoPass is false (the preset is manual play); the rule
    // set is what is under test here, so the master switch is turned on over
    // it — the state a player is in when they run auto with full-control's
    // rules.
    const s = { ...applyPreset('full-control'), autoPass: true };
    expect(run(d, view(0, 'draw', [stackEntry(9, 0, 'spell')]), s)).toEqual({ act: 'stop', reason: 'own-object' });
  });

  it('own object on top with nothing to respond with passes (ownObjects if-respondable, not respondable)', () => {
    const d = priority(ONLY_MANA);
    // casual's rules with ownObjects turned on: the own-object rule needs a
    // respondable window, and a mana-only one is not (draw is 'off' in
    // casual, so no step rule interferes).
    const s = applyPreset('casual');
    s.ownObjects = 'if-respondable';
    expect(run(d, view(0, 'draw', [stackEntry(9, 0, 'ability')]), s)).toEqual({ act: 'pass', index: 1 });
  });

  // --- step rules ---

  it('casual: a smart step with only a mana activate passes (not actionable)', () => {
    const d = priority(ONLY_MANA);
    const s = withSteps('yours', { main1: 'smart' });
    expect(run(d, view(0, 'main1'), s)).toEqual({ act: 'pass', index: 1 });
  });

  it('casual: a smart step with a real action stops', () => {
    const d = priority(RESPONDABLE);
    const s = withSteps('yours', { main1: 'smart' });
    expect(run(d, view(0, 'main1'), s)).toEqual({ act: 'stop', reason: 'stop-set' });
  });

  it('a forced step stops even with nothing to do', () => {
    const d = priority(ONLY_MANA);
    const s = withSteps('opponents', { end: 'forced' });
    expect(run(d, view(1, 'end'), s)).toEqual({ act: 'stop', reason: 'stop-set' });
  });

  it('an off step falls through to a pass', () => {
    const d = priority(RESPONDABLE);
    const s = withSteps('yours', { main1: 'off' });
    expect(run(d, view(0, 'main1'), s)).toEqual({ act: 'pass', index: 0 });
  });

  it('a step rule on the wrong turn side does not apply', () => {
    const d = priority(RESPONDABLE);
    const s = withSteps('yours', { main1: 'forced' });
    // their turn: the YOURS rule is inert
    expect(run(d, view(1, 'main1'), s)).toEqual({ act: 'pass', index: 0 });
  });

  it('a step outside the ten stoppable ones (untap, cleanup) never stops', () => {
    const d = priority(RESPONDABLE);
    expect(run(d, view(0, 'untap'))).toEqual({ act: 'pass', index: 0 });
    expect(run(d, view(0, 'cleanup'))).toEqual({ act: 'pass', index: 0 });
  });

  it('the step rule applies to ffwd too (the c2f4db8f contract)', () => {
    const d = priority(RESPONDABLE);
    const s = withSteps('yours', { main1: 'smart' });
    expect(runFfwd(d, view(0, 'main1'), s)).toEqual({ act: 'stop', reason: 'stop-set' });
    const forced = withSteps('yours', { main1: 'forced' });
    expect(runFfwd(priority(ONLY_MANA), view(0, 'main1'), forced)).toEqual({ act: 'stop', reason: 'stop-set' });
  });

  it('full-control with ffwd still runs (ffwd outranks autoPass off) but a forced step stops it', () => {
    const s = applyPreset('full-control'); // all steps forced, autoPass false
    expect(runFfwd(priority(RESPONDABLE), view(0, 'main1'), s)).toEqual({ act: 'stop', reason: 'stop-set' });
  });

  // --- structural invariant ---

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
        for (const stackEntryVariant of ['none', 'mine', 'theirs', 'their-trigger-targeting-me'] as const) {
          const stack = stackEntryVariant === 'none' ? []
            : stackEntryVariant === 'mine' ? [stackEntry(9, 0, 'spell')]
            : stackEntryVariant === 'theirs' ? [stackEntry(9, 1, 'spell')]
            : [stackEntry(9, 1, 'trigger', [{ obj: 5, player: 0, is_player: false }])];
          const v = view(active, step, stack, [{ seat: 0, cards: [5] }]);
          for (const settings of [applyPreset('casual'), applyPreset('no-tells'), applyPreset('full-control')]) {
            for (const list of lists) {
              // Both modes must hold the structural invariant: a pass verdict
              // always points at a pass option. On the ffwd path the stack
              // rules are skipped, so the stack branch is reachable as a pass.
              for (const ffwd of [false, true]) {
                const d = priority(list);
                const out = ffwd
                  ? decide({ decision: d, view: v, seat: 0, settings, ffwd })
                  : decide({ decision: d, view: v, seat: 0, settings });
                if (out.act !== 'pass') continue;
                const o = d.options[out.index];
                expect(o, `pass index ${out.index} on ${JSON.stringify(list.map((x) => x.kind))} must be a pass option`).toBeDefined();
                expect(o.kind, `kind of option at pass index ${out.index} on ${JSON.stringify(list.map((x) => x.kind))}`).toBe('pass');
              }
            }
          }
        }
      }
    }
  });

  it('actionable is false exactly when every option kind is pass, concede or activate AND no hand card is castable after tapping', () => {
    // SEMANTICS CHANGED (fb-20260914T014141Z): the old pin — "false exactly when every
    // option kind is pass/concede/activate" — no longer holds. A mana-only window
    // whose hand holds a card the seat could cast after tapping (lib/castable,
    // the post-land Lava Spike window) is now actionable: the engine prices a
    // cast against the floating pool only, so the cast option does not exist
    // YET, and auto-passing the tap would eat exactly the window the player
    // wants. These pins keep the view hand-less (the empty hand fails closed),
    // so every line below is the OLD semantics on the kind test alone.
    const noHand = view(0, 'main1');
    expect(actionable(priority([opt('pass', 0), opt('concede', 1)]), noHand, 0)).toBe(false);
    expect(actionable(priority([opt('pass', 0)]), noHand, 0)).toBe(false);
    expect(actionable(priority([opt('activate', 0), opt('pass', 1), opt('concede', 2)]), noHand, 0)).toBe(false);
    expect(actionable(priority([opt('pass', 0), opt('cast', 1), opt('concede', 2)]), noHand, 0)).toBe(true);
    expect(actionable(priority([opt('ability', 0)]), noHand, 0)).toBe(true);
    expect(actionable(priority([opt('play_land', 0), opt('activate', 1), opt('pass', 2), opt('concede', 3)]), noHand, 0)).toBe(true);
  });

  it('actionable on a mana-only window: a hand card castable after tapping makes it TRUE, an uncastable hand does not', () => {
    const d = priority(ONLY_MANA);
    // Post-land Lava Spike: pool empty, one untapped Mountain (Available {R}), {R} spell in hand.
    const stop = withHand(view(0, 'main1'), 0, { hand: [handCard({ mana_cost: 'R' })], available: { R: 1 } });
    expect(actionable(d, stop, 0)).toBe(true);
    // Same window, wrong colour in play: the hand is dead mana-wise, still not actionable.
    const pass = withHand(view(0, 'main1'), 0, { hand: [handCard({ mana_cost: '2 U' })], available: { R: 1 } });
    expect(actionable(d, pass, 0)).toBe(false);
    // A castable-from-the-pool hand carries the cast option already (kind test);
    // the helper must not make the window MORE than actionable — same verdict either way.
    const floating = withHand(view(0, 'main1'), 0, { hand: [handCard({ mana_cost: 'R' })], pool: { R: 1 } });
    const castable = priority([opt('pass', 0), opt('cast', 1), opt('concede', 2)]);
    expect(actionable(castable, floating, 0)).toBe(true);
  });

  it('respondable is true exactly when a cast or ability option is offered (not play_land, not activate)', () => {
    expect(respondable(priority([opt('pass', 0), opt('concede', 1)]))).toBe(false);
    expect(respondable(priority([opt('activate', 0), opt('pass', 1), opt('concede', 2)]))).toBe(false);
    expect(respondable(priority([opt('play_land', 0), opt('activate', 1), opt('pass', 2), opt('concede', 3)]))).toBe(false);
    expect(respondable(priority([opt('pass', 0), opt('cast', 1), opt('concede', 2)]))).toBe(true);
    expect(respondable(priority([opt('ability', 0)]))).toBe(true);
  });

  it('a mana-only window with a dead-mana hand is not actionable and emptyPriorityWindow returns the pass index', () => {
    const d = priority(ONLY_MANA);
    const v = withHand(view(0, 'main1'), 0, { hand: [handCard({ mana_cost: '4 U' })], available: { R: 1 } });
    expect(actionable(d, v, 0)).toBe(false);
    expect(emptyPriorityWindow(d, v, 0)).toBe(1);
  });

  it('a mana-only window with a castable-after-tap hand is NOT an empty priority window', () => {
    const d = priority(ONLY_MANA);
    const v = withHand(view(0, 'main1'), 0, { hand: [handCard({ mana_cost: 'R' })], available: { R: 1 } });
    expect(emptyPriorityWindow(d, v, 0)).toBe(null);
  });

  // --- the post-land window (fb-20260914T014141Z): smart stops now catch the tap-then-cast shape ---

  it('casual decide(): a post-land mana-only window with a castable-after-tap card in hand STOPS (stop-set)', () => {
    const d = priority(ONLY_MANA); // activate + pass + concede — exactly the window after the turn-1 Mountain
    const v = withHand(view(0, 'main1'), 0, { hand: [handCard({ name: 'Lava Spike', mana_cost: 'R' })], available: { R: 1 } });
    const s = withSteps('yours', { main1: 'smart' });
    expect(run(d, v, s)).toEqual({ act: 'stop', reason: 'stop-set' });
  });

  it('casual decide(): decision-offered Tundra and any-source taps stop the own-main window, while dead mana still passes', () => {
    const tap = (obj: number) => priority([
      { ...opt('activate', 0), obj }, opt('pass', 1), opt('concede', 2),
    ]);
    const s = withSteps('yours', { main1: 'smart' });
    const tundra = withHand(view(0, 'main1'), 0, {
      hand: [handCard({ mana_cost: 'W' })], pool: {}, available: { C: 1 },
      battlefield: [handCard({ id: 7, name: 'Tundra', types: 'Land Plains Island', produces: { colour: [1, 1, 0, 0, 0, 0], any: false } })],
    });
    expect(run(tap(7), tundra, s)).toEqual({ act: 'stop', reason: 'stop-set' });

    const any = withHand(view(0, 'main1'), 0, {
      hand: [handCard({ mana_cost: 'W' })], pool: {},
      battlefield: [handCard({ id: 7, name: 'Any land', types: 'Land', produces: { colour: [0, 0, 0, 0, 0, 1], any: true } })],
    });
    expect(run(tap(7), any, s)).toEqual({ act: 'stop', reason: 'stop-set' });

    const dead = withHand(view(0, 'main1'), 0, {
      hand: [handCard({ mana_cost: 'U' })], pool: {},
      battlefield: [handCard({ id: 7, name: 'Mountain', types: 'Land', produces: { colour: [0, 0, 0, 1, 0, 0], any: false } })],
    });
    expect(run(tap(7), dead, s)).toEqual({ act: 'pass', index: 1 });
  });

  it('casual decide(): a payable tap ability stops, but sacrifice and summon-sick abilities remain invisible', () => {
    const d = priority([{ ...opt('activate', 0), obj: 7 }, opt('pass', 1), opt('concede', 2)]);
    const s = withSteps('yours', { main1: 'smart' });
    const source = handCard({ id: 7, name: 'Rock', types: 'Artifact', produces: { colour: [0, 0, 0, 0, 0, 1], any: false } });
    const ability = handCard({ id: 8, name: 'Ability Rock', types: 'Artifact', ability_costs: ['1 T'] } as unknown as Partial<CardView>);
    const payable = withHand(view(0, 'main1'), 0, { pool: {}, battlefield: [source, ability] });
    expect(run(d, payable, s)).toEqual({ act: 'stop', reason: 'stop-set' });

    // The projected cost is effective: a printed 2 T reduced by a live
    // ReduceCost$ 1 | Type$ Ability static arrives as 1 T and must stop too.
    const reduced = withHand(view(0, 'main1'), 0, { pool: {}, battlefield: [source, handCard({ id: 8, name: 'Printed 2 T ability', ability_costs: ['1 T'] } as unknown as Partial<CardView>)] });
    expect(run(d, reduced, s)).toEqual({ act: 'stop', reason: 'stop-set' });

    const sacrifice = withHand(view(0, 'main1'), 0, { pool: {}, battlefield: [source, handCard({ id: 8, ability_costs: ['1 T Sac<1/Artifact>'] } as unknown as Partial<CardView>)] });
    expect(run(d, sacrifice, s)).toEqual({ act: 'pass', index: 1 });
    const sick = withHand(view(0, 'main1'), 0, { pool: {}, battlefield: [source, handCard({ id: 8, types: 'Creature', summon_sick: true, ability_costs: ['1 T'] } as unknown as Partial<CardView>)] });
    expect(run(d, sick, s)).toEqual({ act: 'pass', index: 1 });

    const hasty = withHand(view(0, 'main1'), 0, { pool: {}, battlefield: [source, handCard({ id: 8, types: 'Creature', summon_sick: true, keywords: ['Haste'], ability_costs: ['1 T'] } as unknown as Partial<CardView>)] });
    expect(run(d, hasty, s)).toEqual({ act: 'stop', reason: 'stop-set' });
  });

  it('casual decide(): the same mana-only window with only uncastable cards in hand passes', () => {
    const d = priority(ONLY_MANA);
    const v = withHand(view(0, 'main1'), 0, { hand: [handCard({ mana_cost: 'X R' })], available: { R: 1 } });
    const s = withSteps('yours', { main1: 'smart' });
    expect(run(d, v, s)).toEqual({ act: 'pass', index: 1 });
  });

  it('the castable-after-tap stop does not fire on another seat\u2019s hand (the helper fails closed) and never depends on turnSide', () => {
    const d = priority(ONLY_MANA);
    const handless = view(0, 'main1'); // the hand is the viewer\u2019s hidden zone; a view without it fails closed
    const s = withSteps('yours', { main1: 'smart' });
    expect(run(d, handless, s)).toEqual({ act: 'pass', index: 1 });
    // The stop is about the SEAT's own mana and hand, not whose turn it is:
    // actionable() is TRUE for the same shape on the opponent's turn (the
    // helper is turn-side-blind), but casual has no opponents-side main-phase
    // rule at all, so decide() still passes — the stop fires only where the
    // caller's step rules consult it.
    const mine = withHand(view(1, 'main1'), 0, { hand: [handCard({ mana_cost: 'R' })], available: { R: 1 } });
    expect(actionable(d, mine, 0)).toBe(true);
    expect(run(d, mine, s)).toEqual({ act: 'pass', index: 1 });
  });

  it('an opponent spell on top does not stop a window that only offers a land drop (not respondable)', () => {
    const d = priority([opt('activate', 0), opt('play_land', 1), opt('pass', 2), opt('concede', 3)]);
    expect(respondable(d)).toBe(false);
    expect(run(d, view(0, 'draw', [stackEntry(9, 1, 'spell')]))).toEqual({ act: 'pass', index: 2 });
  });

  // ---- prio6: always-yield per ability, and the Resolve All baseline ----
  // A yield key is `${controller}:${name}:${text}` — the same three facts
  // the stack tile renders (lib/yields.ts). The entry below is shaped like
  // the real wire StackView for an ability: name is the SOURCE's face name,
  // text the ability's own text (view/view.go's abilityName/abilityText).
  const namedEntry = (
    id: number,
    controller: number,
    name: string,
    text: string,
    kind = 'trigger',
    targets: { obj?: number; player: number; is_player: boolean }[] = [],
  ) => ({ id, controller, kind, name, text, targets });
  // artist targets me (a player target on seat 0), so every one of casual's
  // opponent-rule shapes genuinely stops for it before the yield is granted.
  const artist = namedEntry(9, 1, 'Blood Artist', 'Whenever a creature dies, each opponent loses 1 life', 'trigger', [{ player: 0, is_player: true }]);
  // other is a spell (kind spell), so casual's opponentSpell (if-respondable) stops for it.
  const other = namedEntry(10, 1, 'Soul Warden', 'Whenever a creature enters, its controller gains 1 life', 'spell');
  const artistKey = '1:Blood Artist:Whenever a creature dies, each opponent loses 1 life';

  it('a yielded key skips the opponent-object stop for THAT entry, but not for another entry', () => {
    const d = priority(RESPONDABLE);
    // Without the yield, casual stops (opponent spell, if-respondable).
    expect(run(d, view(0, 'draw', [artist]))).toEqual({ act: 'stop', reason: 'opponent-object' });
    // With the artist's key yielded, the opponent-object rule is skipped
    // for it and the window passes. A DIFFERENT key changes nothing.
    expect(decide({ decision: d, view: view(0, 'draw', [artist]), seat: 0, settings: defaultSettings(), yields: new Set([artistKey]) }))
      .toEqual({ act: 'pass', index: 0 });
    expect(decide({ decision: d, view: view(0, 'draw', [other]), seat: 0, settings: defaultSettings(), yields: new Set([artistKey]) }))
      .toEqual({ act: 'stop', reason: 'opponent-object' });
  });

  it('a yield overrides every opponent rule shape — always, if-respondable, targets-me — but not a step stop', () => {
    const d = priority(RESPONDABLE);
    const yields = new Set([artistKey]);
    for (const rule of ['always', 'if-respondable', 'targets-me-if-respondable'] as const) {
      const s = defaultSettings();
      s.opponentTrigger = rule;
      expect(decide({ decision: d, view: view(0, 'draw', [artist]), seat: 0, settings: s, yields }))
        .toEqual({ act: 'pass', index: 0 });
    }
    // The step rules still apply: a yield is per ability, never per step.
    const stopped = withSteps('yours', { draw: 'forced' });
    expect(decide({ decision: d, view: view(0, 'draw', [artist]), seat: 0, settings: stopped, yields }))
      .toEqual({ act: 'stop', reason: 'stop-set' });
  });

  it('a yield does not make the own-object rule skip: the key names the controller, so an own entry never matches', () => {
    const d = priority(RESPONDABLE);
    const mine = namedEntry(9, 0, 'Blood Artist', 'Whenever a creature dies, each opponent loses 1 life');
    const s = defaultSettings();
    s.ownObjects = 'if-respondable';
    expect(decide({ decision: d, view: view(0, 'draw', [mine]), seat: 0, settings: s, yields: new Set([artistKey]) }))
      .toEqual({ act: 'stop', reason: 'own-object' });
  });

  it('the Resolve All baseline passes through arm-time opponent objects but stops on a NEW opponent object', () => {
    const d = priority(RESPONDABLE);
    const settings = defaultSettings();
    // The arm-time entry (id 9) is on the baseline: the run plays through it.
    expect(decide({ decision: d, view: view(0, 'draw', [artist]), seat: 0, settings, baselineStack: new Set([9]) }))
      .toEqual({ act: 'pass', index: 0 });
    // A NEW opponent object (id 10) stops per the settings — the same stop
    // a plain End Turn would take.
    expect(decide({ decision: d, view: view(0, 'draw', [artist, other]), seat: 0, settings, baselineStack: new Set([9]) }))
      .toEqual({ act: 'stop', reason: 'opponent-object' });
    // And when the settings say never to stop for opponent spells, the new
    // object does not stop either — the run honours the player's own rules
    // for what arrived after the press.
    settings.opponentSpell = 'never';
    expect(decide({ decision: d, view: view(0, 'draw', [artist, other]), seat: 0, settings, baselineStack: new Set([9]) }))
      .toEqual({ act: 'pass', index: 0 });
  });

  it('Resolve All skips the own-object rule for both arm-time and NEW own objects, but not a step stop', () => {
    const d = priority(RESPONDABLE);
    const mine = namedEntry(9, 0, 'Blood Artist', 'Whenever a creature dies, each opponent loses 1 life');
    const newMine = namedEntry(10, 0, 'Young Pyromancer', 'Whenever you cast an instant or sorcery spell, create a token');
    const s = defaultSettings();
    s.ownObjects = 'if-respondable';
    expect(decide({ decision: d, view: view(0, 'draw', [mine]), seat: 0, settings: s, baselineStack: new Set([9]) }))
      .toEqual({ act: 'pass', index: 0 });
    // A seat-owned trigger pushed during the run is not one of Resolve All's
    // stop conditions, even under Full Control and when a response is offered.
    expect(decide({ decision: d, view: view(0, 'draw', [mine, newMine]), seat: 0, settings: s, baselineStack: new Set([9]) }))
      .toEqual({ act: 'pass', index: 0 });
    const stopped = withSteps('yours', { draw: 'forced' });
    expect(decide({ decision: d, view: view(0, 'draw', [artist]), seat: 0, settings: stopped, baselineStack: new Set([9]) }))
      .toEqual({ act: 'stop', reason: 'stop-set' });
  });
});

describe('actionables — the labels behind actionable() (fb-20260916T225211Z)', () => {
  // The Deadly Rollick window: one real cast the smart step rule stopped for,
  // and the note had to be able to name it.
  it('names each action-kind option by its wire label, and actionable is its emptiness test', () => {
    const d = priority([
      { index: 0, kind: 'cast', label: 'Cast Deadly Rollick (alternative cost)', player: 0 },
      opt('pass', 1),
      opt('concede', 2),
      opt('activate', 3),
    ]);
    // The mana tap is offered but is not a play: only the cast is named.
    expect(actionables(view(0, 'main1'), 0, d)).toEqual(['Cast Deadly Rollick (alternative cost)']);
    expect(actionable(d, view(0, 'main1'), 0)).toBe(true);
  });

  it('on a mana-only window it names the castable-after-tap card; a dead hand is empty and not actionable', () => {
    const d = priority(ONLY_MANA);
    const v = withHand(view(0, 'main1'), 0, { hand: [handCard({ id: 7, name: 'Lava Spike', mana_cost: 'R' })], available: { R: 1 } });
    expect(actionables(v, 0, d)).toEqual(['Cast Lava Spike (after tapping)']);
    expect(actionable(d, v, 0)).toBe(true);

    const dead = withHand(view(0, 'main1'), 0, { hand: [handCard({ id: 7, name: 'Lava Spike', mana_cost: '4 U U' })], available: { R: 1 } });
    expect(actionables(dead, 0, d)).toEqual([]);
    expect(actionable(d, dead, 0)).toBe(false);
  });

  it('names a payable battlefield activation behind the tap when the hand is dead', () => {
    // The same working shape the stop-side test above uses: an untapped
    // source the decision's activate offer joins, and a projected '1 T'
    // activation on another card.
    const d = priority([{ ...opt('activate', 0), obj: 7 }, opt('pass', 1), opt('concede', 2)]);
    const source = handCard({ id: 7, name: 'Rock', types: 'Artifact', produces: { colour: [0, 0, 0, 0, 0, 1], any: false } });
    const ability = handCard({ id: 8, name: 'Ability Rock', types: 'Artifact', ability_costs: ['1 T'] } as unknown as Partial<CardView>);
    const v = withHand(view(0, 'main1'), 0, { hand: [], pool: {}, battlefield: [source, ability] });
    expect(actionables(v, 0, d)).toEqual(['Activate Ability Rock (after tapping)']);
    expect(actionable(d, v, 0)).toBe(true);

    // A cost this projection cannot price (a sacrifice component) names
    // nothing and leaves the window not actionable — the fail-closed arm.
    const sacrifice = withHand(view(0, 'main1'), 0, { hand: [], pool: {}, battlefield: [source, handCard({ id: 8, name: 'Sac Rock', ability_costs: ['1 T Sac<1/Artifact>'] } as unknown as Partial<CardView>)] });
    expect(actionables(sacrifice, 0, d)).toEqual([]);
    expect(actionable(d, sacrifice, 0)).toBe(false);
  });
});
