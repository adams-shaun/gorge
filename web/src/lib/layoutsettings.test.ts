import { describe, expect, it } from 'vitest';
import {
  bumpedScale,
  clampScale,
  defaultLayout,
  LAYOUT_KEY,
  LAYOUT_ZONES,
  loadLayout,
  saveLayout,
  SCALE_MAX,
  SCALE_MIN,
  withAlign,
  withHandPeek,
  withScale,
  withSteppers,
  type LayoutSettings,
} from './layoutsettings';

/** memStorage is a minimal in-memory Storage; throwing=true makes every method throw (private-mode browsers do). Same helper playsettings.test.ts uses. */
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

describe('defaults', () => {
  it('every zone at 100%, left-packed rows, centred hand, hover peek', () => {
    const d = defaultLayout();
    expect(d.version).toBe(1);
    expect(d.scale).toEqual({ creatures: 1, others: 1, lands: 1, command: 1, hand: 1 });
    expect(d.align).toEqual({ creatures: 'left', others: 'left', lands: 'left', command: 'left', hand: 'center' });
    expect(d.handPeek).toBe('hover');
    expect(d.steppersOnBoard).toBe(false); // fb-20260917T004304Z: hidden by default
  });

  it('covers exactly the five zones: the three battlefield rows, the seat\'s command rim zone, then hand', () => {
    // fb-20260917T232202Z: the command zone is a first-class LayoutZone —
    // panel order is battlefield rows top-to-bottom, the seat's own rim
    // zone, then hand.
    expect(LAYOUT_ZONES).toEqual(['creatures', 'others', 'lands', 'command', 'hand']);
  });
});

describe('the command zone\'s own keys (fb-20260917T232202Z)', () => {
  it('a pre-change saved v1 blob (no command keys) loads WITHOUT falling back to all-defaults', () => {
    // The exact blob shape the PRE-COMMAND client saved: version 1, the four
    // legacy zones' scale/align, handPeek, steppersOnBoard — and nothing for
    // the command zone. validate treats the command keys as OPTIONAL (the
    // steppersOnBoard precedent, fb-20260917T004304Z): the missing keys load
    // as the shipped command defaults (scale 1, align left) and every
    // player-chosen value below survives the deploy — a blob wipe to
    // all-defaults here would lose the player their sizes.
    const st = memStorage();
    const preCommand = {
      version: 1,
      scale: { creatures: 1.2, others: 0.8, lands: 1.1, hand: 0.9 },
      align: { creatures: 'center', others: 'right', lands: 'left', hand: 'left' },
      handPeek: 'always',
      steppersOnBoard: true,
    };
    st.setItem(LAYOUT_KEY, JSON.stringify(preCommand));
    expect(loadLayout(st)).toEqual({
      ...preCommand,
      scale: { ...preCommand.scale, command: 1 },
      align: { ...preCommand.align, command: 'left' },
    });
  });

  it('the loaded pre-change blob can then be edited per zone and round-trips the command keys', () => {
    const st = memStorage();
    const preCommand = {
      version: 1,
      scale: { creatures: 1.2, others: 0.8, lands: 1.1, hand: 0.9 },
      align: { creatures: 'center', others: 'right', lands: 'left', hand: 'left' },
      handPeek: 'always',
    };
    st.setItem(LAYOUT_KEY, JSON.stringify(preCommand));
    const loaded = loadLayout(st);
    expect(withScale(loaded, 'command', 1.4).scale.command).toBe(1.4);
    expect(loaded.scale.creatures).toBe(1.2); // the legacy zones untouched
    saveLayout(st, withAlign(loaded, 'command', 'right'));
    const reloaded = loadLayout(st);
    expect(reloaded.align.command).toBe('right');
    expect(reloaded.align.creatures).toBe('center');
  });

  it('a present-but-invalid command scale corrupts the whole blob to defaults (same strictness as every field)', () => {
    const st = memStorage();
    st.setItem(LAYOUT_KEY, JSON.stringify({ ...defaultLayout(), scale: { ...defaultLayout().scale, command: 5 } }));
    expect(loadLayout(st)).toEqual(defaultLayout());
    st.setItem(LAYOUT_KEY, JSON.stringify({ ...defaultLayout(), scale: { ...defaultLayout().scale, command: 'big' } }));
    expect(loadLayout(st)).toEqual(defaultLayout());
  });

  it('a present-but-unknown command align word corrupts the whole blob to defaults', () => {
    const st = memStorage();
    st.setItem(LAYOUT_KEY, JSON.stringify({ ...defaultLayout(), align: { ...defaultLayout().align, command: 'middle' } }));
    expect(loadLayout(st)).toEqual(defaultLayout());
  });

  it('the default command align is left, not the hand\'s centre', () => {
    // The command tiles sit at the creatures row's FRONT today (CZ2, genesis
    // order); their shipped packing must keep them there.
    expect(defaultLayout().align.command).toBe('left');
    expect(defaultLayout().scale.command).toBe(1);
  });
});

/** withScale / withAlign / withHandPeek / withSteppers */
describe('withScale / withAlign / withHandPeek / withSteppers', () => {
  it('returns fresh objects and touches only the named zone', () => {
    const d = defaultLayout();
    const s = withScale(d, 'creatures', 1.3);
    expect(s.scale.creatures).toBe(1.3);
    expect(s.scale.lands).toBe(1);
    expect(s).not.toBe(d);
    expect(s.scale).not.toBe(d.scale);
    expect(d.scale.creatures).toBe(1); // the original untouched

    const a = withAlign(s, 'lands', 'right');
    expect(a.align.lands).toBe('right');
    expect(a.align.creatures).toBe('left');
    expect(a.scale).toEqual(s.scale);

    const p = withHandPeek(a, 'always');
    expect(p.handPeek).toBe('always');
    expect(p.align).toEqual(a.align);

    const t = withSteppers(p, false);
    expect(t.steppersOnBoard).toBe(false);
    expect(t.handPeek).toBe('always');
    expect(t.scale).toEqual(p.scale);
    const on = withSteppers(t, true);
    expect(on.steppersOnBoard).toBe(true);
    expect(p.steppersOnBoard).toBe(false); // the original untouched (fb-20260917T004304Z default)
  });

  it('clamps an out-of-range scale instead of storing it', () => {
    expect(clampScale(0.2)).toBe(SCALE_MIN);
    expect(clampScale(9)).toBe(SCALE_MAX);
    expect(clampScale(1.25)).toBe(1.25);
    expect(clampScale(Number.NaN)).toBe(1);
    expect(withScale(defaultLayout(), 'hand', 99).scale.hand).toBe(SCALE_MAX);
  });
});

describe('bumpedScale', () => {
  it('steps on a clean one-decimal grid (no binary-float drift)', () => {
    let s = 1;
    for (let i = 0; i < 6; i++) s = bumpedScale(s, 0.1);
    expect(s).toBe(1.6); // 1.1 + 0.1 thrice in raw floats is 1.4000000000000001
    expect(bumpedScale(0.7, -0.1)).toBe(0.6);
  });

  it('clamps at the bounds and stops there', () => {
    expect(bumpedScale(SCALE_MAX, 0.1)).toBe(SCALE_MAX);
    expect(bumpedScale(SCALE_MIN, -0.1)).toBe(SCALE_MIN);
  });
});

describe('persistence', () => {
  it('loadLayout with an absent key returns the defaults', () => {
    expect(loadLayout(memStorage())).toEqual(defaultLayout());
    expect(loadLayout(null)).toEqual(defaultLayout());
  });

  it('loadLayout with a saved round-trip returns what was saved, under the ONE global key', () => {
    const st = memStorage();
    const s = withScale(withHandPeek(defaultLayout(), 'never'), 'others', 0.8);
    saveLayout(st, s);
    expect(st.getItem(LAYOUT_KEY)).not.toBeNull();
    expect(loadLayout(st)).toEqual(s);
  });

  it.each(['not json at all', '{"version":1,', '[]', '"a string"', 'null', '123'])('corrupt value %j falls back to defaults', (raw) => {
    const st = memStorage();
    st.setItem(LAYOUT_KEY, raw);
    expect(loadLayout(st)).toEqual(defaultLayout());
  });

  it('a wrong-version value falls back to defaults', () => {
    const st = memStorage();
    st.setItem(LAYOUT_KEY, JSON.stringify({ ...defaultLayout(), version: 2 }));
    expect(loadLayout(st)).toEqual(defaultLayout());
  });

  it('a missing or wrong-typed field falls back to defaults (no partial merge)', () => {
    const st = memStorage();
    const broken = { ...defaultLayout(), scale: { creatures: 1, others: 1, lands: 1 } };
    st.setItem(LAYOUT_KEY, JSON.stringify(broken));
    expect(loadLayout(st)).toEqual(defaultLayout());
    const missing = { ...defaultLayout() } as Partial<LayoutSettings>;
    delete missing.handPeek;
    st.setItem(LAYOUT_KEY, JSON.stringify(missing));
    expect(loadLayout(st)).toEqual(defaultLayout());
  });

  it('an out-of-range or non-finite scale falls back to defaults', () => {
    const st = memStorage();
    // Written RAW (not through withScale, which clamps): the corrupt-blob
    // path — a value out of range must fall back to defaults wholesale.
    st.setItem(LAYOUT_KEY, JSON.stringify({ ...defaultLayout(), scale: { creatures: 1, others: 1, lands: 5, hand: 1 } }));
    expect(loadLayout(st)).toEqual(defaultLayout());
    st.setItem(LAYOUT_KEY, JSON.stringify({ ...defaultLayout(), scale: { creatures: 'big', others: 1, lands: 1, hand: 1 } }));
    expect(loadLayout(st)).toEqual(defaultLayout());
    st.setItem(LAYOUT_KEY, JSON.stringify({ ...defaultLayout(), scale: { creatures: 1, others: 1, lands: 1, hand: Number.NaN } }));
    expect(loadLayout(st)).toEqual(defaultLayout());
  });

  it('an unknown align or handPeek word falls back to defaults', () => {
    const st = memStorage();
    // Raw literals (the bad words are deliberately not valid ZoneAlign/HandPeek values).
    st.setItem(LAYOUT_KEY, JSON.stringify({ ...defaultLayout(), align: { ...defaultLayout().align, hand: 'middle' } }));
    expect(loadLayout(st)).toEqual(defaultLayout());
    st.setItem(LAYOUT_KEY, JSON.stringify({ ...defaultLayout(), handPeek: 'sometimes' }));
    expect(loadLayout(st)).toEqual(defaultLayout());
  });

  it('a saved pre-toggle v1 blob (no steppersOnBoard key) loads with the field defaulting to false (fb-20260917T004304Z flip)', () => {
    // The exact blob the PRE-TOGGLE client saved: version 1, scale/align/
    // handPeek, no steppersOnBoard key. validate treats the field as
    // OPTIONAL so every existing player's saved sizes/alignments/peek
    // survive the deploy — but the missing field now loads as HIDDEN:
    // the toggle was opt-out-with-default-ON for one day, so nobody could
    // have opted IN and no saved blob loses a choice it made (the missing
    // path must still preserve the rest of the blob, hence optional, not
    // corrupt). The command zone's keys (fb-20260917T232202Z, optional for
    // the same reason) load as their shipped defaults alongside.
    const st = memStorage();
    const preToggle = {
      version: 1,
      scale: { creatures: 1.2, others: 0.8, lands: 1.1, hand: 0.9 },
      align: { creatures: 'center', others: 'right', lands: 'left', hand: 'left' },
      handPeek: 'always',
    };
    st.setItem(LAYOUT_KEY, JSON.stringify(preToggle));
    expect(loadLayout(st)).toEqual({
      ...preToggle,
      scale: { ...preToggle.scale, command: 1 },
      align: { ...preToggle.align, command: 'left' },
      steppersOnBoard: false,
    });
  });

  it('a present-but-non-boolean steppersOnBoard is still corrupt (falls back to ALL defaults)', () => {
    const st = memStorage();
    st.setItem(LAYOUT_KEY, JSON.stringify({ ...defaultLayout(), steppersOnBoard: 'yes' }));
    expect(loadLayout(st)).toEqual(defaultLayout());
    st.setItem(LAYOUT_KEY, JSON.stringify({ ...defaultLayout(), steppersOnBoard: 1 }));
    expect(loadLayout(st)).toEqual(defaultLayout());
  });

  it('a throwing storage yields defaults on load and is swallowed on save', () => {
    const st = memStorage(true);
    expect(loadLayout(st)).toEqual(defaultLayout());
    expect(() => saveLayout(st, defaultLayout())).not.toThrow();
  });

  it('saveLayout with null storage is a no-op, not a throw', () => {
    expect(() => saveLayout(null, defaultLayout())).not.toThrow();
  });

  it('the playsettings key is deliberately NOT read (separate models, separate keys)', () => {
    const st = memStorage();
    st.setItem('gorge.playsettings.v1', JSON.stringify({ version: 1 }));
    expect(loadLayout(st)).toEqual(defaultLayout());
  });
});
