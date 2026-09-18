import {
  bumpedScale,
  defaultLayout,
  loadLayout,
  saveLayout,
  withAlign,
  withHandPeek,
  withScale,
  withSteppers,
  type HandPeek,
  type LayoutSettings,
  type LayoutZone,
  type ZoneAlign,
} from './layoutsettings';
import { safeStorage } from './storage';

/**
 * layoutsettings.svelte.ts is the reactive shell over the pure
 * layoutsettings model: ONE store instance the whole client reads, so a
 * change made in the Game Options panel re-renders every zone carrying that
 * setting (Quadrant rows, HandFan) without any event plumbing. Same shape as
 * session.svelte.ts's Session: a class with $state fields, exported as a
 * module singleton. The pure model (layoutsettings.ts) stays rune-free and
 * does the persistence; this file only wires reactivity and the dotted
 * outline's flash timing.
 *
 * The FLASH timing is the brief's decision 4: the dotted outline shows while
 * a zone's size control is hovered/focused (the parent reads the stepper's
 * hover state directly) and, after any adjustment — from the board stepper
 * OR the panel — for FLASH_MS afterwards, so the player sees which zones the
 * change landed on even when the control was elsewhere. Timers are injected
 * so tests are deterministic (HoverCard's TimerEnv pattern).
 */

/** How long the dotted outline stays after an adjustment lands. */
export const FLASH_MS = 1600;

export interface LayoutStoreEnv {
  storage: Storage | null;
  setTimeout: (fn: () => void, ms: number) => unknown;
  clearTimeout: (id: unknown) => void;
}

const NO_FLASH: Record<LayoutZone, boolean> = { creatures: false, others: false, lands: false, command: false, hand: false };

export class LayoutStore {
  #env: LayoutStoreEnv;
  /** settings is the live layout; every consumer reads it reactively. */
  settings = $state<LayoutSettings>(defaultLayout());
  /** flash is the post-adjustment dotted-outline state, per zone. */
  flash = $state<Record<LayoutZone, boolean>>({ ...NO_FLASH });
  #timers: Partial<Record<LayoutZone, unknown>> = {};

  constructor(env: Partial<LayoutStoreEnv> = {}) {
    this.#env = {
      storage: env.storage !== undefined ? env.storage : safeStorage(),
      setTimeout: env.setTimeout ?? ((fn, ms) => setTimeout(fn, ms)),
      clearTimeout: env.clearTimeout ?? ((id) => clearTimeout(id as Parameters<typeof clearTimeout>[0])),
    };
    this.settings = loadLayout(this.#env.storage);
  }

  /** scale is one zone's card-size multiplier. */
  scale(zone: LayoutZone): number {
    return this.settings.scale[zone];
  }

  /** align is one zone's main-axis packing. */
  align(zone: LayoutZone): ZoneAlign {
    return this.settings.align[zone];
  }

  /** peek is the hand's peek mode. */
  get peek(): HandPeek {
    return this.settings.handPeek;
  }

  /** steppersOnBoard is whether the on-board − / + size steppers are shown (fb-20260916T200925Z). */
  get steppersOnBoard(): boolean {
    return this.settings.steppersOnBoard;
  }

  /**
   * bump moves one zone's scale by delta (step grid, clamped), persists, and
   * pulses the dotted outline on that zone.
   */
  bump(zone: LayoutZone, delta: number): void {
    this.settings = withScale(this.settings, zone, bumpedScale(this.settings.scale[zone], delta));
    this.save();
    this.pulse(zone);
  }

  /** setAlign sets one zone's alignment, persists, and pulses the outline. */
  setAlign(zone: LayoutZone, align: ZoneAlign): void {
    this.settings = withAlign(this.settings, zone, align);
    this.save();
    this.pulse(zone);
  }

  /** setHandPeek sets the hand's peek mode and persists (no pulse: nothing geometric changed size-wise the outline needs to point at). */
  setHandPeek(peek: HandPeek): void {
    this.settings = withHandPeek(this.settings, peek);
    this.save();
  }

  /**
   * setSteppersOnBoard shows (true) or hides (false) the on-board − / + size
   * steppers and persists (no pulse: hiding a control changes no zone's
   * geometry the dotted outline would point at).
   */
  setSteppersOnBoard(on: boolean): void {
    this.settings = withSteppers(this.settings, on);
    this.save();
  }

  /** reset returns every zone to the shipped layout and persists. */
  reset(): void {
    this.settings = defaultLayout();
    this.save();
  }

  /**
   * pulse shows the dotted outline on a zone for FLASH_MS, restarting the
   * window if an adjustment lands while one is already running (so a burst
   * of + presses keeps the outline up for FLASH_MS after the LAST press,
   * rather than flickering).
   */
  pulse(zone: LayoutZone): void {
    const pending = this.#timers[zone];
    if (pending !== undefined) this.#env.clearTimeout(pending);
    this.flash = { ...this.flash, [zone]: true };
    this.#timers[zone] = this.#env.setTimeout(() => {
      this.#timers[zone] = undefined;
      this.flash = { ...this.flash, [zone]: false };
    }, FLASH_MS);
  }

  /** testing helper: clear every running flash timer without waiting. */
  #clearTimers(): void {
    for (const z of Object.keys(this.#timers) as LayoutZone[]) {
      const t = this.#timers[z];
      if (t !== undefined) this.#env.clearTimeout(t);
      this.#timers[z] = undefined;
      this.flash = { ...this.flash, [z]: false };
    }
  }

  /** testing helper: tear the store back to defaults (tests construct their own instances; this serves a shared one). */
  dispose(): void {
    this.#clearTimers();
  }

  private save(): void {
    saveLayout(this.#env.storage, this.settings);
  }
}

/** layoutStore is the one store instance every component reads. */
export const layoutStore = new LayoutStore();
