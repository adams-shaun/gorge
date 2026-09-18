/**
 * layoutsettings is the pure settings model behind the board's appearance
 * options: per-zone card size, per-zone alignment and the hand's peek
 * behaviour. It is deliberately SEPARATE from playsettings.ts — that model's
 * types, validation and preset-equality logic are all about auto-pass
 * semantics, and mixing geometry into it would break its preset relabelling
 * (withChange relabels `custom` by deep-equality against the three presets).
 * fb-20260916T182801Z brief: settings are a property of the player, not the
 * table, so persistence is ONE global localStorage key, `gorge.layoutsettings.v1`.
 *
 * Pure TypeScript — no Svelte runes, no I/O except the two storage functions —
 * following the playsettings.ts house pattern: a version field, strict
 * validate-with-fail-to-defaults (corrupt means defaults, never a partial
 * merge), and a swallowed throw on save. The reactive shell the components
 * read lives in layoutsettings.svelte.ts; this module stays testable.
 *
 * The zones are the ones that can actually take the settings (brief decision
 * 1: stacks + hand first; quadrant geometry is a later job):
 *
 *   - creatures / others / lands — the three battlefield card rows every
 *     quadrant renders (groupBattlefield), the zones a board player actually
 *     scans;
 *   - hand — the seated player's own HandFan.
 *
 * Scale is a multiplier on the gameplay card width (--play-card-w), 1 = the
 * shipped size, clamped so a 4-seat board never becomes unreadable. Align is
 * the row's main-axis packing (justify-content), left = the shipped default.
 * handPeek is brief decision 2's three readings of "slide up": hover (the
 * shipped peek — half-clipped, rises on hover/focus), always (full cards,
 * never clipped) and never (no slide-up at all; the hover inspector still
 * reads a card, and keyboard focus still raises it for accessibility).
 *
 * command (fb-20260917T232202Z) is the seat's COMMAND ZONE — the commander
 * tiles CommandArea draws at the front of the creatures row. It is a rim
 * zone of the seat's own, not a battlefield row, so it sits between lands
 * and hand in the panel order; on the board its tiles are one pack at the
 * creatures row's front (a flex sub-row inside that row), which is why it
 * can carry its own scale + align while the creatures row carries its own.
 *
 * steppersOnBoard (fb-20260916T200925Z) gates the ON-BOARD − / + size
 * steppers (ZoneStepper mounts in Quadrant and HandFan): the player asked
 * for a show/hide toggle in the Game Options panel, so the board can carry
 * nothing but cards when it is off. The panel's OWN per-zone steppers are
 * NOT gated by it — they already live in options. Default false
 * (fb-20260917T004304Z: the same player asked the toggle default to HIDDEN
 * — "the board carries nothing but cards" out of the box; a player who
 * explicitly toggled ON keeps the boolean in their saved blob and is
 * untouched). The field is OPTIONAL in validate (missing → false) so every
 * saved pre-toggle v1 blob still loads with its scale/align/handPeek
 * intact; a present-but-non-boolean value still corrupts the whole blob to
 * defaults.
 */

export type LayoutZone = 'creatures' | 'others' | 'lands' | 'command' | 'hand';
export type ZoneAlign = 'left' | 'center' | 'right';
export type HandPeek = 'hover' | 'always' | 'never';

/** The five zones, in panel order (battlefield rows top-to-bottom, the seat's own command rim zone, then hand). */
export const LAYOUT_ZONES: readonly LayoutZone[] = ['creatures', 'others', 'lands', 'command', 'hand'];

export const ZONE_ALIGNS: readonly ZoneAlign[] = ['left', 'center', 'right'];
export const HAND_PEEKS: readonly HandPeek[] = ['hover', 'always', 'never'];

/** ZONE_LABELS is the panel's plain word for each zone (data-facing keys stay ASCII). */
export const ZONE_LABELS: Record<LayoutZone, string> = {
  creatures: 'Creatures',
  others: 'Other permanents',
  lands: 'Lands',
  command: 'Command zone',
  hand: 'Hand',
};

export const ALIGN_LABELS: Record<ZoneAlign, string> = {
  left: 'Left',
  center: 'Centre',
  right: 'Right',
};

export const HAND_PEEK_LABELS: Record<HandPeek, string> = {
  hover: 'Hover',
  always: 'Always',
  never: 'Never',
};

/** Card-scale bounds: 60% (a crammed 4-seat board stays readable) to 160%. */
export const SCALE_MIN = 0.6;
export const SCALE_MAX = 1.6;
/** The step the − / + steppers move by, in scale units (10% of the shipped size). */
export const SCALE_STEP = 0.1;

export interface LayoutSettings {
  version: 1;
  /** per-zone card-scale multiplier, clamped to [SCALE_MIN, SCALE_MAX] */
  scale: Record<LayoutZone, number>;
  /** per-zone main-axis packing of the zone's container */
  align: Record<LayoutZone, ZoneAlign>;
  /** how the seated player's own hand presents (HandFan) */
  handPeek: HandPeek;
  /** the on-board − / + size steppers (Quadrant rows + HandFan) are shown */
  steppersOnBoard: boolean;
}

/**
 * defaultLayout is the shipped board: every zone at 100%, left-packed, hover
 * peek, on-board size steppers HIDDEN (fb-20260917T004304Z default flip).
 */
export function defaultLayout(): LayoutSettings {
  return {
    version: 1,
    scale: { creatures: 1, others: 1, lands: 1, command: 1, hand: 1 },
    align: { creatures: 'left', others: 'left', lands: 'left', command: 'left', hand: 'center' },
    handPeek: 'hover',
    steppersOnBoard: false,
  };
}

/** clampScale clips any number onto the legal range (NaN fails to the default). */
export function clampScale(n: number): number {
  if (typeof n !== 'number' || !Number.isFinite(n)) return 1;
  return Math.min(SCALE_MAX, Math.max(SCALE_MIN, n));
}

/**
 * bumpedScale moves a scale by delta and snaps to the step grid, so repeated
 * − / + presses never accumulate binary-float drift (0.1 is not exact in
 * binary) and the stored value stays a clean one-decimal number.
 */
export function bumpedScale(cur: number, delta: number): number {
  const snapped = Math.round((cur + delta) * 10) / 10;
  return Math.min(SCALE_MAX, Math.max(SCALE_MIN, snapped));
}

/** withScale returns a fresh layout with one zone's scale set (clamped). */
export function withScale(s: LayoutSettings, zone: LayoutZone, scale: number): LayoutSettings {
  return { ...s, scale: { ...s.scale, [zone]: clampScale(scale) } };
}

/** withAlign returns a fresh layout with one zone's alignment set. */
export function withAlign(s: LayoutSettings, zone: LayoutZone, align: ZoneAlign): LayoutSettings {
  return { ...s, align: { ...s.align, [zone]: align } };
}

/** withHandPeek returns a fresh layout with the hand's peek mode set. */
export function withHandPeek(s: LayoutSettings, peek: HandPeek): LayoutSettings {
  return { ...s, handPeek: peek };
}

/** withSteppers returns a fresh layout with the on-board steppers shown (true) or hidden (false). */
export function withSteppers(s: LayoutSettings, on: boolean): LayoutSettings {
  return { ...s, steppersOnBoard: on };
}

/** cloneLayout deep-copies a layout (the storage-free write paths return fresh objects; this guards the record fields). */
function cloneLayout(s: LayoutSettings): LayoutSettings {
  return { version: 1, scale: { ...s.scale }, align: { ...s.align }, handPeek: s.handPeek, steppersOnBoard: s.steppersOnBoard };
}

/**
 * Storage key: ONE global key, deliberately not per table (see module doc).
 */
export const LAYOUT_KEY = 'gorge.layoutsettings.v1';

function isOneOf<T extends string>(v: unknown, allowed: readonly T[]): v is T {
  return typeof v === 'string' && (allowed as readonly string[]).includes(v);
}

function isScale(v: unknown): v is number {
  return typeof v === 'number' && Number.isFinite(v) && v >= SCALE_MIN && v <= SCALE_MAX;
}

/**
 * validate returns a LayoutSettings from a parsed value, or null when the
 * value is corrupt: wrong version, a missing or wrong-typed field, a scale
 * outside the legal range, an unknown zone/align/peek word. Corrupt means
 * defaults, not a partial merge — a half-understood layout blob should not
 * half-apply (same rule as playsettings.validate).
 *
 * steppersOnBoard and the COMMAND ZONE are the deliberate exceptions to
 * "missing field means corrupt": both shipped AFTER the blob's first
 * version, so blobs saved by an earlier client have no such keys. A missing
 * steppersOnBoard loads as false (fb-20260917T004304Z: the player asked the
 * steppers to default hidden). A missing command scale/align loads as the
 * shipped command defaults (scale 1, align left) — fb-20260917T232202Z, the
 * player asked for the command zone to have its own sizing and placement,
 * and the four legacy zones' saved sizes must survive that deploy intact,
 * NOT fall back to all-defaults. Both stay STRICT about what is present: a
 * steppersOnBoard that is not a boolean, a command scale outside the legal
 * range or a command align that is not a known word still corrupt the whole
 * blob to defaults (same rule as every other field). Version stays 1 — the
 * blob's shape only ever grew, its meaning never changed.
 */
function validate(v: unknown): LayoutSettings | null {
  if (typeof v !== 'object' || v === null || Array.isArray(v)) return null;
  if ((v as Record<string, unknown>).version !== 1) return null;
  const o = v as Record<string, unknown>;
  if (typeof o.scale !== 'object' || o.scale === null || Array.isArray(o.scale)) return null;
  if (typeof o.align !== 'object' || o.align === null || Array.isArray(o.align)) return null;
  /**
   * zones shipped after the v1 blob format (fb-20260917T232202Z): a key
   * these zones lack loads as the zone's shipped default instead of
   * corrupting the blob. Every other zone key is required, as it always was.
   */
  const POST_V1_ZONES: Record<LayoutZone, { scale: number; align: ZoneAlign } | undefined> = {
    creatures: undefined,
    others: undefined,
    lands: undefined,
    command: { scale: 1, align: 'left' },
    hand: undefined,
  };
  const scale = {} as Record<LayoutZone, number>;
  for (const z of LAYOUT_ZONES) {
    const fallback = POST_V1_ZONES[z];
    const raw = (o.scale as Record<string, unknown>)[z];
    if (raw === undefined && fallback !== undefined) {
      scale[z] = fallback.scale;
      continue;
    }
    if (!isScale(raw)) return null;
    scale[z] = (o.scale as Record<string, number>)[z];
  }
  const align = {} as Record<LayoutZone, ZoneAlign>;
  for (const z of LAYOUT_ZONES) {
    const fallback = POST_V1_ZONES[z];
    const raw = (o.align as Record<string, unknown>)[z];
    if (raw === undefined && fallback !== undefined) {
      align[z] = fallback.align;
      continue;
    }
    if (!isOneOf(raw, ZONE_ALIGNS)) return null;
    align[z] = (o.align as Record<string, ZoneAlign>)[z];
  }
  if (!isOneOf(o.handPeek, HAND_PEEKS)) return null;
  const steppersOnBoard = o.steppersOnBoard === undefined ? false : o.steppersOnBoard;
  if (typeof steppersOnBoard !== 'boolean') return null;
  return cloneLayout({ version: 1, scale, align, handPeek: o.handPeek, steppersOnBoard });
}

/**
 * loadLayout reads the global key. Absent, corrupt, wrong-version or a
 * throwing storage all yield the defaults; a throw must never escape —
 * private-mode browsers throw on getItem too. Same pattern as
 * playsettings.loadSettings.
 */
export function loadLayout(storage: Storage | null): LayoutSettings {
  try {
    const raw = storage?.getItem(LAYOUT_KEY) ?? null;
    if (raw === null) return defaultLayout();
    const validated = validate(JSON.parse(raw));
    return validated ?? defaultLayout();
  } catch {
    return defaultLayout();
  }
}

/**
 * saveLayout writes the global key. A throw (private mode, quota) is
 * swallowed: the caller's in-memory copy is the source of truth until a
 * later save lands. Same pattern as playsettings.saveSettings.
 */
export function saveLayout(storage: Storage | null, s: LayoutSettings): void {
  try {
    storage?.setItem(LAYOUT_KEY, JSON.stringify(s));
  } catch {
    /* private mode or quota: keep the in-memory copy */
  }
}
