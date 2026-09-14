import { describe, expect, it } from 'vitest';
import { actionable, decide, emptyPriorityWindow, respondable, STEPS, STOPPABLE_STEPS, turnSide } from './autopilot';
import { applyPreset, defaultSettings, type PlaySettings, type StoppableStep, type StepStop } from './playsettings';
import type { Decision, Option, View } from '../protocol';

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

  it('actionable is false exactly when every option kind is pass, concede or activate', () => {
    expect(actionable(priority([opt('pass', 0), opt('concede', 1)]))).toBe(false);
    expect(actionable(priority([opt('pass', 0)]))).toBe(false);
    expect(actionable(priority([opt('activate', 0), opt('pass', 1), opt('concede', 2)]))).toBe(false);
    expect(actionable(priority([opt('pass', 0), opt('cast', 1), opt('concede', 2)]))).toBe(true);
    expect(actionable(priority([opt('ability', 0)]))).toBe(true);
    expect(actionable(priority([opt('play_land', 0), opt('activate', 1), opt('pass', 2), opt('concede', 3)]))).toBe(true);
  });

  it('respondable is true exactly when a cast or ability option is offered (not play_land, not activate)', () => {
    expect(respondable(priority([opt('pass', 0), opt('concede', 1)]))).toBe(false);
    expect(respondable(priority([opt('activate', 0), opt('pass', 1), opt('concede', 2)]))).toBe(false);
    expect(respondable(priority([opt('play_land', 0), opt('activate', 1), opt('pass', 2), opt('concede', 3)]))).toBe(false);
    expect(respondable(priority([opt('pass', 0), opt('cast', 1), opt('concede', 2)]))).toBe(true);
    expect(respondable(priority([opt('ability', 0)]))).toBe(true);
  });

  it('a mana-only window (activate + pass + concede) is not actionable and emptyPriorityWindow returns the pass index', () => {
    const d = priority(ONLY_MANA);
    expect(actionable(d)).toBe(false);
    expect(emptyPriorityWindow(d)).toBe(1);
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

  it('the Resolve All baseline skips the own-object rule for arm-time objects too, but not a step stop', () => {
    const d = priority(RESPONDABLE);
    const mine = namedEntry(9, 0, 'Blood Artist', 'Whenever a creature dies, each opponent loses 1 life');
    const s = defaultSettings();
    s.ownObjects = 'if-respondable';
    expect(decide({ decision: d, view: view(0, 'draw', [mine]), seat: 0, settings: s, baselineStack: new Set([9]) }))
      .toEqual({ act: 'pass', index: 0 });
    const stopped = withSteps('yours', { draw: 'forced' });
    expect(decide({ decision: d, view: view(0, 'draw', [artist]), seat: 0, settings: stopped, baselineStack: new Set([9]) }))
      .toEqual({ act: 'stop', reason: 'stop-set' });
  });
});
