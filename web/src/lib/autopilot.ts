import type { Decision, View } from '../protocol';

/**
 * autopilot is the "auto" decision logic for a seat: pass priority for the
 * player, stop where the player told it to, and auto-pass whenever there is
 * no action available. It is pure -- no I/O, no timers, no Svelte runes --
 * because a mis-firing autopasser loses games silently: it acts for the
 * player, at speed, where a wrong action is unrecoverable. The caller owns
 * the loop guard (track the answered seq) and the UI. This module only
 * answers one question: what should the seat do with this decision, right
 * now? Note the wire fact this module builds on: a priority decision's
 * options carry Kind ("pass", "cast", "ability", "concede", ...), so "no
 * action available" is answerable client-side -- an option list whose kinds
 * are only "pass" and "concede" means the player can do nothing.
 *
 * decide() can only ever return an index pointing at an option whose kind
 * is "pass". It is structurally incapable of returning a "concede": the
 * verdict's index is always taken from the pass-option ref found by the
 * shape check, never from a position or a default.
 */

export type TurnSide = 'yours' | 'opponents';

export interface Stops {
  yours: Set<string>;
  opponents: Set<string>;
}

export type AutoVerdict =
  | { act: 'pass'; index: number }
  | { act: 'stop'; reason: StopReason };

export type StopReason =
  | 'disabled'
  | 'not-priority'
  | 'unexpected-shape'
  | 'stop-set'
  | 'has-action-and-stack';

/** STEPS is the wire's twelve step names in engine order (state/ids.go). */
export const STEPS = [
  'untap', 'upkeep', 'draw', 'main1', 'begin-combat',
  'declare-attackers', 'declare-blockers', 'combat-damage', 'end-combat',
  'main2', 'end', 'cleanup',
] as const;

/**
 * STOPPABLE_STEPS is STEPS minus untap and cleanup, the two steps that
 * normally grant no priority. A stop there would be meaningless, so the UI
 * can only offer stops on these ten. Exported so the UI task cannot invent
 * its own list.
 */
export const STOPPABLE_STEPS: readonly string[] = STEPS.filter((s) => s !== 'untap' && s !== 'cleanup');

/** turnSide reports whether the given seat is the turn's active player. */
export function turnSide(view: View, seat: number): TurnSide {
  return view.active === seat ? 'yours' : 'opponents';
}

/**
 * actionable reports whether a priority decision offers the player a real
 * action: an option that is neither pass nor concede (cast, ability,
 * play_land, activate, ...). An option list whose kinds are only pass and
 * concede means the player can do nothing.
 */
export function actionable(decision: Decision): boolean {
  return decision.options.some((o) => o.kind !== 'pass' && o.kind !== 'concede');
}

export function decide(args: {
  decision: Decision;
  view: View;
  seat: number;
  stops: Stops;
  enabled: boolean;
}): AutoVerdict {
  const { decision, view, seat, stops, enabled } = args;

  // Evaluation order, first match wins. Every earlier branch is a stop
  // because acting on a decision it does not fully understand is exactly
  // how an autopasser loses a game silently.
  if (!enabled) return { act: 'stop', reason: 'disabled' };

  // Auto NEVER answers anything but a priority decision. Target, blockers,
  // attackers, mulligan, modes, trigger_order, trigger_optional and choose
  // always stop, whatever the settings say.
  if (decision.kind !== 'priority') return { act: 'stop', reason: 'not-priority' };

  // Plain single-pick shape only: min === max === 1 and exactly one pass
  // option. Anything else is not a window auto understands.
  if (decision.min !== 1 || decision.max !== 1) return { act: 'stop', reason: 'unexpected-shape' };
  const passOptions = decision.options.filter((o) => o.kind === 'pass');
  if (passOptions.length !== 1) return { act: 'stop', reason: 'unexpected-shape' };
  const pass = passOptions[0];

  // Nothing to do: pass regardless of stops. Safe precisely because the
  // player had no action, so auto is never choosing anything for them.
  if (!actionable(decision)) return { act: 'pass', index: pass.index };

  // A stop on the current step of the current turn side: the player wants
  // to look here.
  const side = turnSide(view, seat);
  if (stops[side].has(view.step)) return { act: 'stop', reason: 'stop-set' };

  // The player has an action and someone else controls an object on the
  // stack: auto-passing could let that object resolve unanswered.
  if (view.stack.some((s) => s.controller !== seat)) return { act: 'stop', reason: 'has-action-and-stack' };

  return { act: 'pass', index: pass.index };
}
