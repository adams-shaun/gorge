import type { Decision, View } from '../protocol';
import type { OpponentObjectRule, OpponentTriggerRule, PlaySettings, StoppableStep } from './playsettings';
import { stackYieldKey } from './yields';

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
 * are only "pass", "concede" and "activate" means the player can do
 * nothing that matters (the engine offers a mana tap for every available
 * source at every priority window; see actionable()).
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
  | 'opponent-object'
  | 'own-object'
  | 'stop-set';

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
 * action: an option that is not pass, concede or activate (cast, ability,
 * play_land, ...). An option list whose kinds are only pass, concede and
 * activate means the player can do nothing that matters: the engine offers
 * an "activate" (tap for mana) option for every available mana source at
 * every priority window (rules/legal.go's availableManaAbilities loop), so
 * counting those taps as actions would make almost every window
 * "actionable" and defeat both the empty-window skip and the
 * smart step rule. Tapping mana with nothing to spend it on is
 * not a play.
 */
export function actionable(decision: Decision): boolean {
  return decision.options.some((o) => o.kind !== 'pass' && o.kind !== 'concede' && o.kind !== 'activate');
}

/**
 * respondable reports whether the player could respond to something on the
 * stack: a cast or ability option (a spell or non-mana ability at speed).
 * It deliberately excludes play_land (a land drop is never a response —
 * lands are sorcery-speed and cannot interact with a resolving spell) and
 * activate (see actionable above — every mana source offers a tap at every
 * window). Used by decide()'s stack rules so an opponent object on the
 * stack only stops a player who can actually answer it, not one who can
 * merely tap a land.
 */
export function respondable(decision: Decision): boolean {
  return decision.options.some((o) => o.kind === 'cast' || o.kind === 'ability');
}

/**
 * emptyPriorityWindow reports the one window shape the panel skips even when
 * auto is OFF: a plain single-pick priority window with nothing actionable
 * on it. "Actionable" is actionable()'s test — pass, concede and activate
 * do not count (a mana tap is offered at every window and is not a play),
 * so this covers both the only-pass-and-concede shape and the mana-only
 * shape (activate + pass + concede). There is nothing to decide there --
 * the player's only non-suicidal answer is the pass, so stopping to collect
 * it is a click that carries no information. This is deliberately the SAME shape test decide()
 * applies before its own !actionable branch (single-pick, exactly one pass
 * option), factored out rather than restated, so the manual-mode skip can
 * never come to a different conclusion than auto would.
 *
 * It returns the pass option's index, or null when the window is not that
 * shape. Like decide(), it is structurally incapable of pointing at a
 * concede: the index always comes from the pass option that was found.
 */
export function emptyPriorityWindow(decision: Decision): number | null {
  if (decision.kind !== 'priority') return null;
  if (decision.min !== 1 || decision.max !== 1) return null;
  const passOptions = decision.options.filter((o) => o.kind === 'pass');
  if (passOptions.length !== 1) return null;
  if (actionable(decision)) return null;
  return passOptions[0].index;
}

/**
 * targetsMe reports whether any target of the given stack entry is this seat
 * (a player target) or an object this seat controls. The controller lookup
 * reads every public zone the View exposes plus the stack:
 *
 * - every CardView[] zone on view.players[*] — battlefield, graveyard,
 *   exile, and hand (present only for the viewer's own seat, a CR 400.2
 *   hidden zone) — found structurally by walking the player object's array
 *   fields and admitting only entries shaped like a CardView (numeric id +
 *   numeric controller), so a zone the view gains later is covered without
 *   this function changing, and a non-card array field can never contribute
 *   a bogus id. view/view.go's cardView() sets Controller for every zone,
 *   so graveyard/exile targets ("target card in a graveyard") count;
 * - view.stack, where a spell or ability object's controller is the seat
 *   that cast/activated it — a counterspell at MY spell on the stack must
 *   count as "targeting me", not just one at my creature.
 *
 * Every entry is the object's CURRENT controller, so an object that changed
 * zones or controllers resolves to whoever holds it now. The function stays
 * pure: it only reads the view it is given.
 */
function targetsMe(view: View, seat: number, top: { targets: { obj?: number; player: number; is_player: boolean }[] }): boolean {
  const controllers = new Map<number, number>();
  for (const p of view.players ?? []) {
    for (const zone of Object.values(p)) {
      if (!Array.isArray(zone)) continue;
      for (const c of zone) {
        if (c !== null && typeof c === 'object' && typeof c.id === 'number' && typeof c.controller === 'number') {
          controllers.set(c.id, c.controller);
        }
      }
    }
  }
  for (const s of view.stack ?? []) controllers.set(s.id, s.controller);
  return (top.targets ?? []).some((t) =>
    t.is_player ? t.player === seat : t.obj !== undefined && controllers.get(t.obj) === seat
  );
}

/**
 * opponentRuleFor maps a StackView.Kind (view/view.go sets exactly three:
 * "spell" for a spell object, "trigger" for one minted by TriggerPush,
 * "ability" for any other ability object) to the settings' matching rule.
 * A kind the view does not define today is treated as if-respondable — the
 * old guard's answer for any opponent object — so a future kind fails
 * toward stopping, never toward silently passing.
 */
function opponentRuleFor(settings: PlaySettings, kind: string): OpponentObjectRule | OpponentTriggerRule {
  switch (kind) {
    case 'spell': return settings.opponentSpell;
    case 'ability': return settings.opponentAbility;
    case 'trigger': return settings.opponentTrigger;
    default: return 'if-respondable';
  }
}

export function decide(args: {
  decision: Decision;
  view: View;
  seat: number;
  /** settings carries every rule; playsettings.ts holds the values (presets), this module only consumes them. */
  settings: PlaySettings;
  /**
   * ffwd marks the one-shot fast-forward run (absent/false = persistent
   * Auto). Pressing FFWD is itself the player's explicit "I have no more
   * actions to take", so the stack rules below are skipped on this path:
   * the pass IS the consent that guard otherwise has to assume for an
   * unattended autopasser. The step rules still apply — the c2f4db8f
   * contract: a set stop stops a fast-forward too — and the caller's pass
   * cap still bounds the run.
   */
  ffwd?: boolean;
  /**
   * yields is the game-scoped set of "always pass for this ability" keys
   * (prio6, lib/yields.ts): when the TOP stack entry's key is in the set,
   * the opponent-object rule is skipped for it — the player has already
   * said, for this game, that they never stop for that ability. The step
   * rules still apply: a yield is per ability, never per step.
   */
  yields?: ReadonlySet<string>;
  /**
   * baselineStack is the Resolve All run's arm-time stack (prio6): the ids
   * of the objects that were already on the stack when the run started.
   * Resolve All's whole point is to play through the stack AS IT STANDS, so
   * an opponent object that was present at arm time does not stop the run
   * (the rule checks are skipped for it); a NEW opponent object — an id not
   * in the baseline — stops per the caller's settings, exactly as a plain
   * End Turn would. Resolve All never stops for the seat's own objects,
   * whether they were present at arm time or were added while resolving;
   * only a NEW opponent object is subject to a stack stop rule.
   */
  baselineStack?: ReadonlySet<number>;
}): AutoVerdict {
  const { decision, view, seat, settings, ffwd = false, yields = null, baselineStack = null } = args;

  // Safety first: auto NEVER answers anything but a plain single-pick
  // priority decision with exactly one pass option. Target, blockers,
  // attackers, mulligan, modes, trigger_order, trigger_optional and choose
  // always stop, whatever the settings say — acting on a decision it does
  // not fully understand is exactly how an autopasser loses a game silently.
  if (decision.kind !== 'priority') return { act: 'stop', reason: 'not-priority' };
  if (decision.min !== 1 || decision.max !== 1) return { act: 'stop', reason: 'unexpected-shape' };
  const passOptions = decision.options.filter((o) => o.kind === 'pass');
  if (passOptions.length !== 1) return { act: 'stop', reason: 'unexpected-shape' };
  const pass = passOptions[0];

  // 1. Master switch. FFWD outranks it: a one-shot run is explicit consent
  // even when persistent auto is off.
  if (!settings.autoPass && !ffwd) return { act: 'stop', reason: 'disabled' };

  // 2. Stack rules, on the TOP of the stack only (the object that resolves
  // next). ffwd passes through all of them (pressing FFWD is consent).
  // Resolve All's baseline passes through them too, but ONLY for the
  // objects that were on the stack when the run was armed: the run is the
  // player's "resolve what is already there". A yield skips the
  // opponent-object rule for its one key, whatever run is asking.
  const top = view.stack.length > 0 ? view.stack[view.stack.length - 1] : null;
  if (top !== null && !ffwd && !(baselineStack?.has(top.id) ?? false)) {
    if (top.controller !== seat) {
      if (yields?.has(stackYieldKey(top))) {
        // yielded: the player always passes for this ability this game.
      } else {
        const rule = opponentRuleFor(settings, top.kind);
        if (rule === 'always') return { act: 'stop', reason: 'opponent-object' };
        if (rule === 'if-respondable' && respondable(decision)) return { act: 'stop', reason: 'opponent-object' };
        if (rule === 'targets-me-if-respondable' && respondable(decision) && targetsMe(view, seat, top)) {
          return { act: 'stop', reason: 'opponent-object' };
        }
        // 'never' (and a rule the arms above did not meet) falls through.
      }
    } else if (baselineStack === null && settings.ownObjects === 'if-respondable' && respondable(decision)) {
      // Resolve All only stops for NEW opponent objects. An own trigger or
      // ability pushed while it runs is part of resolving the stack, even
      // under Full Control; persistent Auto still honours ownObjects.
      return { act: 'stop', reason: 'own-object' };
    }
  }

  // 3. Step rule for the current turn side and step — this applies to ffwd
  // too (the c2f4db8f contract: a set stop stops a fast-forward). 'forced'
  // stops whenever priority is posed, even with nothing to do; 'smart' stops
  // only when the window offers a real action (actionable(), so a mana-only
  // window still passes); 'off' — or a step outside the ten stoppable ones —
  // falls through.
  const side = turnSide(view, seat);
  const stepRule = settings.steps[side][view.step as StoppableStep] ?? 'off';
  if (stepRule === 'forced' || (stepRule === 'smart' && actionable(decision))) {
    return { act: 'stop', reason: 'stop-set' };
  }

  // 4. Pass.
  return { act: 'pass', index: pass.index };
}
