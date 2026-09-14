import type { PlaySettings } from './playsettings';

/** A bounded FIFO used for client diagnostics. values() returns oldest first. */
export class RingBuffer<T> {
  #items: T[] = [];

  constructor(readonly capacity: number) {}

  push(item: T): void {
    this.#items.push(item);
    if (this.#items.length > this.capacity) this.#items.splice(0, this.#items.length - this.capacity);
  }

  values(): T[] {
    return [...this.#items];
  }
}

export interface BreadcrumbAction {
  at: string;
  type: string;
  detail?: Record<string, unknown>;
}

interface BreadcrumbState {
  view_sequence: number | null;
  intent_index: number | null;
  settings: PlaySettings | null;
  active_yields: string[];
}

/**
 * ClientBreadcrumbs is the one client-side diagnostic collector. Network
 * dispatchers and the autopilot loop record here; FeedbackButton only takes a
 * snapshot, so opening the feedback form cannot change game behaviour.
 */
export class ClientBreadcrumbs {
  #actions = new RingBuffer<BreadcrumbAction>(50);
  #errors = new RingBuffer<BreadcrumbAction>(20);
  #state: BreadcrumbState = { view_sequence: null, intent_index: null, settings: null, active_yields: [] };

  record(type: string, detail?: Record<string, unknown>): void {
    this.#actions.push({ at: new Date().toISOString(), type, ...(detail === undefined ? {} : { detail }) });
  }

  consoleError(args: unknown[]): void {
    const message = args.map(stringifyConsoleArg).join(' ').slice(0, 2_000);
    this.#errors.push({ at: new Date().toISOString(), type: 'console_error', detail: { message } });
  }

  setView(viewSequence: number | null, intentIndex: number | null): void {
    this.#state.view_sequence = viewSequence;
    this.#state.intent_index = intentIndex;
  }

  setPlay(settings: PlaySettings, yields: Iterable<string>): void {
    // Settings are a small data-only object. Copying keeps later UI mutations
    // from rewriting the history a report says was in effect.
    this.#state.settings = JSON.parse(JSON.stringify(settings)) as PlaySettings;
    this.#state.active_yields = [...yields];
  }

  snapshot(doc: Document | null = typeof document === 'undefined' ? null : document): Record<string, unknown> {
    return {
      version: 1,
      view: { sequence: this.#state.view_sequence, intent_index: this.#state.intent_index },
      play: { settings: this.#state.settings, active_yields: [...this.#state.active_yields] },
      actions: this.#actions.values(),
      bundle_build_id: bundleBuildID(doc),
      viewport: viewport(),
      console_errors: this.#errors.values(),
    };
  }
}

function stringifyConsoleArg(value: unknown): string {
  if (value instanceof Error) return value.stack ?? value.message;
  if (typeof value === 'string') return value;
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function bundleBuildID(doc: Document | null): string | null {
  const src = doc?.querySelector('script[type="module"]')?.getAttribute('src') ?? null;
  return src;
}

function viewport(): { width: number | null; height: number | null } {
  if (typeof window === 'undefined') return { width: null, height: null };
  return { width: window.innerWidth, height: window.innerHeight };
}

/** The app-wide collector; importing it starts nothing and sends nothing. */
export const clientBreadcrumbs = new ClientBreadcrumbs();

/** installConsoleBreadcrumbs is called once by App on mount. */
export function installConsoleBreadcrumbs(): () => void {
  if (typeof window === 'undefined') return () => {};
  const original = console.error;
  console.error = (...args: unknown[]) => {
    clientBreadcrumbs.consoleError(args);
    original.apply(console, args);
  };
  const onError = (event: ErrorEvent) => clientBreadcrumbs.consoleError([event.error ?? event.message]);
  const onRejection = (event: PromiseRejectionEvent) => clientBreadcrumbs.consoleError([event.reason]);
  window.addEventListener('error', onError);
  window.addEventListener('unhandledrejection', onRejection);
  return () => {
    console.error = original;
    window.removeEventListener('error', onError);
    window.removeEventListener('unhandledrejection', onRejection);
  };
}
