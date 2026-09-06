import { describe, expect, it } from 'vitest';
import { PHASE_GROUPS, stepLabel } from './phases';
import { STEPS, STOPPABLE_STEPS } from './autopilot';

describe('PHASE_GROUPS', () => {
  it('flattens back to STEPS exactly, in engine order', () => {
    const flat = PHASE_GROUPS.flatMap((g) => g.steps.map((s) => s.step));
    expect(flat).toEqual([...STEPS]);
  });

  it('is the five phases, named the way a player says them', () => {
    expect(PHASE_GROUPS.map((g) => g.key)).toEqual(['beginning', 'main1', 'combat', 'main2', 'ending']);
    expect(PHASE_GROUPS.map((g) => g.label)).toEqual(['Beginning', 'Main', 'Combat', 'Main 2', 'Ending']);
  });

  it('marks stoppable from STOPPABLE_STEPS, so untap and cleanup are the only inert cells', () => {
    const inert = PHASE_GROUPS.flatMap((g) => g.steps).filter((s) => !s.stoppable).map((s) => s.step);
    expect(inert).toEqual(['untap', 'cleanup']);
    for (const s of PHASE_GROUPS.flatMap((g) => g.steps)) {
      expect(s.stoppable).toBe(STOPPABLE_STEPS.includes(s.step));
    }
  });

  it('gives every step a spoken label that is not the wire name', () => {
    for (const step of STEPS) {
      expect(stepLabel(step)).not.toBe(step);
      expect(stepLabel(step).length).toBeGreaterThan(0);
    }
    // the finer combat steps stay legible inside the combat group
    const combat = PHASE_GROUPS.find((g) => g.key === 'combat');
    expect(combat?.steps.map((s) => s.label)).toEqual(['Begin', 'Attackers', 'Blockers', 'Damage', 'End']);
  });

  it('falls back to the wire name for a step it has never heard of', () => {
    expect(stepLabel('brand-new-step')).toBe('brand-new-step');
  });
});
