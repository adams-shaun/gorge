import { describe, expect, it } from 'vitest';
import {
  applyPreset,
  defaultSettings,
  loadSettings,
  PRESETS,
  saveSettings,
  withChange,
  type PlaySettings,
  type StoppableStep,
  type StepStop,
} from './playsettings';
import { STOPPABLE_STEPS } from './autopilot';

/** memStorage is a minimal in-memory Storage; throwing=true makes every method throw (private-mode browsers do). */
const memStorage = (throwing = false): Storage => {
  const map = new Map<string, string>();
  const guard = <T>(fn: () => T): T => {
    if (throwing) throw new DOMException('denied', 'SecurityError');
    return fn();
  };
  return {
    length: 0,
    clear: () => guard(() => map.clear()),
    getItem: (k: string) => guard(() => map.get(k) ?? null),
    key: (i: number) => guard(() => [...map.keys()][i] ?? null),
    removeItem: (k: string) => guard(() => void map.delete(k)),
    setItem: (k: string, v: string) => guard(() => void map.set(k, v)),
  } as unknown as Storage;
};

describe('presets', () => {
  it('defaults are casual, field for field', () => {
    expect(defaultSettings()).toEqual(PRESETS.casual);
    expect(defaultSettings().preset).toBe('casual');
  });

  it('PRESETS carries exactly the three named presets (custom is a label, not a preset)', () => {
    expect(Object.keys(PRESETS).sort()).toEqual(['casual', 'full-control', 'no-tells']);
  });

  it('casual matches the settings table exactly', () => {
    const s = PRESETS.casual;
    expect(s.version).toBe(1);
    expect(s.autoPass).toBe(true);
    expect(s.opponentSpell).toBe('if-respondable');
    expect(s.opponentAbility).toBe('if-respondable');
    expect(s.opponentTrigger).toBe('targets-me-if-respondable');
    expect(s.ownObjects).toBe('never');
    expect(s.passAfterAct).toBe(true);
    expect(s.autoOrderIdenticalTriggers).toBe(true);
    expect(s.pacing).toEqual({ stepMs: 200, resolveMs: 400 });
    expect(s.logAutoPasses).toBe(true);
    for (const step of STOPPABLE_STEPS) {
      const expectedYours: StepStop = step === 'main1' || step === 'main2' ? 'smart' : 'off';
      const expectedOpponents: StepStop = step === 'declare-attackers' || step === 'end' ? 'smart' : 'off';
      expect(s.steps.yours[step as StoppableStep]).toBe(expectedYours);
      expect(s.steps.opponents[step as StoppableStep]).toBe(expectedOpponents);
    }
  });

  it('no-tells flips every opponent rule to always and keeps casual\u2019s steps', () => {
    const s = applyPreset('no-tells');
    expect(s.preset).toBe('no-tells');
    expect(s.autoPass).toBe(true);
    expect(s.opponentSpell).toBe('always');
    expect(s.opponentAbility).toBe('always');
    expect(s.opponentTrigger).toBe('always');
    expect(s.ownObjects).toBe('never');
    expect(s.passAfterAct).toBe(true);
    expect(s.pacing).toEqual({ stepMs: 200, resolveMs: 400 });
    expect(s.steps).toEqual(PRESETS.casual.steps);
    expect(s.steps).not.toBe(PRESETS.casual.steps);
  });

  it('full-control is manual play with everything forced', () => {
    const s = applyPreset('full-control');
    expect(s.preset).toBe('full-control');
    expect(s.autoPass).toBe(false);
    expect(s.opponentSpell).toBe('always');
    expect(s.opponentAbility).toBe('always');
    expect(s.opponentTrigger).toBe('always');
    expect(s.ownObjects).toBe('if-respondable');
    expect(s.passAfterAct).toBe(false);
    expect(s.autoOrderIdenticalTriggers).toBe(false);
    expect(s.pacing).toEqual({ stepMs: 0, resolveMs: 0 });
    expect(s.logAutoPasses).toBe(true);
    for (const step of STOPPABLE_STEPS) {
      expect(s.steps.yours[step as StoppableStep]).toBe('forced');
      expect(s.steps.opponents[step as StoppableStep]).toBe('forced');
    }
  });

  it('applyPreset returns an independent copy (mutating it does not touch the preset)', () => {
    const s = applyPreset('casual');
    s.steps.yours.main1 = 'off';
    s.pacing.stepMs = 999;
    s.opponentTrigger = 'never';
    expect(PRESETS.casual.steps.yours.main1).toBe('smart');
    expect(PRESETS.casual.pacing.stepMs).toBe(200);
    expect(PRESETS.casual.opponentTrigger).toBe('targets-me-if-respondable');
  });
});

describe('withChange', () => {
  it('marks an edited copy custom', () => {
    const next = withChange(defaultSettings(), { opponentTrigger: 'always' });
    expect(next.preset).toBe('custom');
    expect(next.opponentTrigger).toBe('always');
    // the original is untouched
    expect(defaultSettings().opponentTrigger).toBe('targets-me-if-respondable');
  });

  it('restores a preset\u2019s name when the edited copy deep-equals it', () => {
    const edited = withChange(defaultSettings(), { opponentTrigger: 'always', opponentSpell: 'always', opponentAbility: 'always' });
    expect(edited).toEqual(PRESETS['no-tells']);
    expect(edited.preset).toBe('no-tells');
    const back = withChange(edited, {
      opponentTrigger: 'targets-me-if-respondable',
      opponentSpell: 'if-respondable',
      opponentAbility: 'if-respondable',
    });
    expect(back).toEqual(PRESETS.casual);
    expect(back.preset).toBe('casual');
  });

  it('merges nested step and pacing patches field-wise', () => {
    const next = withChange(defaultSettings(), { steps: { yours: { main1: 'forced' } } } as Partial<PlaySettings>);
    expect(next.steps.yours.main1).toBe('forced');
    expect(next.steps.yours.main2).toBe('smart'); // untouched siblings survive
    expect(next.steps.opponents).toEqual(PRESETS.casual.steps.opponents);
    const paced = withChange(defaultSettings(), { pacing: { stepMs: 0 } } as Partial<PlaySettings>);
    expect(paced.pacing).toEqual({ stepMs: 0, resolveMs: 400 });
    expect(paced.preset).toBe('custom');
  });
});

describe('persistence', () => {
  it('loadSettings with an absent key returns the defaults', () => {
    expect(loadSettings(memStorage())).toEqual(defaultSettings());
    expect(loadSettings(null)).toEqual(defaultSettings());
  });

  it('loadSettings with a saved round-trip returns what was saved', () => {
    const st = memStorage();
    const s = applyPreset('no-tells');
    saveSettings(st, s);
    expect(loadSettings(st)).toEqual(s);
  });

  it.each(['not json at all', '{"version":1,', '[]', '"a string"', 'null'])('corrupt value %j falls back to defaults', (raw) => {
    const st = memStorage();
    st.setItem('gorge.playsettings.v1', raw);
    expect(loadSettings(st)).toEqual(defaultSettings());
  });

  it('a wrong-version value falls back to defaults', () => {
    const st = memStorage();
    st.setItem('gorge.playsettings.v1', JSON.stringify({ ...PRESETS.casual, version: 2 }));
    expect(loadSettings(st)).toEqual(defaultSettings());
  });

  it('a missing or wrong-typed field falls back to defaults (no partial merge)', () => {
    const st = memStorage();
    const broken = { ...PRESETS.casual, opponentSpell: 'sometimes' };
    st.setItem('gorge.playsettings.v1', JSON.stringify(broken));
    expect(loadSettings(st)).toEqual(defaultSettings());
    const missing = { ...PRESETS.casual } as Partial<PlaySettings>;
    delete missing.passAfterAct;
    st.setItem('gorge.playsettings.v1', JSON.stringify(missing));
    expect(loadSettings(st)).toEqual(defaultSettings());
  });

  it('a throwing storage yields defaults on load and is swallowed on save', () => {
    const st = memStorage(true);
    expect(loadSettings(st)).toEqual(defaultSettings());
    expect(() => saveSettings(st, applyPreset('full-control'))).not.toThrow();
  });

  it('saveSettings with null storage is a no-op, not a throw', () => {
    expect(() => saveSettings(null, applyPreset('casual'))).not.toThrow();
  });

  it('the legacy per-table keys are deliberately NOT imported (they encode the old mana-tap-era defaults)', () => {
    const st = memStorage();
    st.setItem('gorge.stop.t1.0', JSON.stringify({ yours: ['main1'], opponents: [] }));
    st.setItem('gorge.actpass.t1.0', '1');
    expect(loadSettings(st)).toEqual(defaultSettings());
  });
});
