import { STEPS, STOPPABLE_STEPS } from './autopilot';

/**
 * phases groups the wire's twelve step names into the five phases a player
 * actually talks about, and gives each step the words they say out loud
 * ("Attackers", not "declare-attackers"). It is the ONLY place a step name
 * is written as a literal on the client: the display list is built from
 * autopilot's STEPS, so a step that exists on the wire and is missing here
 * is a build-time-visible hole rather than a silently absent cell, and
 * `stoppable` is read from STOPPABLE_STEPS rather than re-decided.
 *
 * No rules knowledge: this is vocabulary and order, nothing else.
 */

export interface StepCell {
  /** step is the wire name, the value that appears in view.step and in a Stops set. */
  readonly step: string;
  /** label is what a player calls it. */
  readonly label: string;
  /** stoppable is false for untap and cleanup, the two steps that grant no priority. */
  readonly stoppable: boolean;
}

export interface PhaseGroup {
  /** key is the protocol Phase name this group corresponds to. */
  readonly key: string;
  /** label names the group the way a player says it. */
  readonly label: string;
  readonly steps: readonly StepCell[];
}

/** STEP_LABELS is every wire step's spoken name. Keyed by every member of STEPS; see the test. */
const STEP_LABELS: Record<string, string> = {
  'untap': 'Untap',
  'upkeep': 'Upkeep',
  'draw': 'Draw',
  'main1': 'Main',
  'begin-combat': 'Begin',
  'declare-attackers': 'Attackers',
  'declare-blockers': 'Blockers',
  'combat-damage': 'Damage',
  'end-combat': 'End',
  'main2': 'Main 2',
  'end': 'End Step',
  'cleanup': 'Cleanup',
};

/** GROUP_OF maps a step to the phase group that owns it, and GROUP_LABELS names those groups. */
const GROUP_OF: Record<string, string> = {
  'untap': 'beginning',
  'upkeep': 'beginning',
  'draw': 'beginning',
  'main1': 'main1',
  'begin-combat': 'combat',
  'declare-attackers': 'combat',
  'declare-blockers': 'combat',
  'combat-damage': 'combat',
  'end-combat': 'combat',
  'main2': 'main2',
  'end': 'ending',
  'cleanup': 'ending',
};

const GROUP_LABELS: Record<string, string> = {
  beginning: 'Beginning',
  main1: 'Main',
  combat: 'Combat',
  main2: 'Main 2',
  ending: 'Ending',
};

/** stepLabel is the spoken name of a wire step, falling back to the wire name so an unknown step still renders. */
export function stepLabel(step: string): string {
  return STEP_LABELS[step] ?? step;
}

/**
 * PHASE_GROUPS is STEPS in engine order, cut into its phases. Built by a
 * single left-to-right pass so the cell order can never diverge from the
 * order the engine walks; a step with no group entry lands in a trailing
 * group of its own rather than disappearing.
 */
export const PHASE_GROUPS: readonly PhaseGroup[] = (() => {
  const out: { key: string; label: string; steps: StepCell[] }[] = [];
  for (const step of STEPS) {
    const key = GROUP_OF[step] ?? step;
    const cell: StepCell = { step, label: stepLabel(step), stoppable: STOPPABLE_STEPS.includes(step) };
    const last = out[out.length - 1];
    if (last !== undefined && last.key === key) last.steps.push(cell);
    else out.push({ key, label: GROUP_LABELS[key] ?? stepLabel(step), steps: [cell] });
  }
  return out;
})();
