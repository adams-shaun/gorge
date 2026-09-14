import { withBase } from './basepath';

/**
 * versioncheck is the stale-client detector (fb-3ab6d9da, defect 2): the demo
 * redeploys on every merge to `main`, and a browser tab that was already open
 * keeps the OLD embedded bundle until the player reloads — which is how a fix
 * deployed before a report "still" hadn't reached the reporter. There is no
 * mechanism on the wire for the server to tell a seated player their client
 * is stale (host/httpapi/static.go serves whatever is embedded and never
 * speaks versions), so the client asks: it records the module script src of
 * the index.html it was loaded from (the vite build hashes the bundle name
 * into that src — `/assets/index-<hash>.js` — so the src IS the version), and
 * polls `GET /` on a slow timer. When the served index.html names a different
 * script src, a non-intrusive banner says so. It NEVER auto-reloads: a reload
 * would drop a player's picked options mid-decision; the banner only tells.
 *
 * What is deliberately NOT here:
 *  - any auto-start. The poll exists only while the page wiring calls
 *    versionWatch.start() (App.svelte's onMount), so a unit test that imports
 *    this module — or any test suite — starts no network traffic. That is the
 *    test-environment guarantee, structural rather than remembered.
 *  - any engine or wire change: `GET /` is the public static index.html every
 *    deployment already serves, fetched through the same base path as every
 *    other same-origin request (basepath.ts).
 *
 * Poll errors are swallowed: the network being down is not a version change,
 * and a transient failure must never surface as a banner. A non-200 (a proxy
 * answering for `/`) is equally silent. The poll is issued with
 * `cache: 'no-store'`: host/httpapi/static.go serves index.html with no
 * Cache-Control directive, so under fetch's default cache mode a browser (or
 * a shared proxy) is allowed to satisfy the poll from its cached PRE-deploy
 * index — the old bundle src compared against the old bundle src, stale
 * forever and the banner never fires, which is exactly the class of silence
 * this module exists to close. no-store forces each cadence to observe the
 * DEPLOYED index; the cost is one uncached GET per minute, which is the point.
 * A page whose OWN script src is not a
 * built asset (the vite dev server's `/src/main.ts`) is never watched — in
 * dev the served index.html legitimately names a different bundle, and a dev
 * tab is not a stale deployment.
 */

/** POLL_MS is the slow cadence: a redeploy is worth a minute's latency, and this is one tiny GET per seated client per minute. */
export const POLL_MS = 60_000;

/**
 * parseBundleSrc extracts the module script src from served index.html text;
 * null when the page carries none (an error page, a proxy's answer). It walks
 * every <script ...> tag, admits the one carrying type="module", and takes
 * its src attribute — attribute ORDER on the tag does not matter (the built
 * tag reads type then src; other orders must match too).
 */
export function parseBundleSrc(html: string): string | null {
  for (const tag of html.matchAll(/<script\b[^>]*>/g)) {
    if (!/type="module"/.test(tag[0])) continue;
    const src = /\ssrc="([^"]+)"/.exec(tag[0]);
    if (src) return src[1];
  }
  return null;
}

/** loadedBundleSrc reads the page's own module script src — the bundle version this tab is running; null under SSR, in a test DOM, or when the page carries no module script. */
export function loadedBundleSrc(doc: Document | null | undefined): string | null {
  if (!doc) return null;
  return doc.querySelector('script[type="module"]')?.getAttribute('src') ?? null;
}

/** bundleStale compares the tab's recorded src with the served one; anything unreadable (either null) is never "stale" — a comparison this client cannot make must not show a banner. */
export function bundleStale(loaded: string | null, served: string | null): boolean {
  return loaded !== null && served !== null && loaded !== served;
}

/** isBuiltBundle reports whether a script src names a built asset — the vite build emits under /assets/. The dev server's /src/main.ts does not, and a dev tab is never watched. */
function isBuiltBundle(src: string | null): boolean {
  return src !== null && src.startsWith('/assets/');
}

export interface VersionEnv {
  fetch?: typeof fetch;
  setTimeout?: (fn: () => void, ms: number) => unknown;
  clearTimeout?: (t: unknown) => void;
  /** document to read the tab's own script src from; defaults to the real one, null in tests/SSR. */
  doc?: Document | null;
}

const defaultEnv = (env: VersionEnv): Required<Pick<VersionEnv, 'fetch' | 'setTimeout' | 'clearTimeout'>> => ({
  fetch: env.fetch ?? ((...a: Parameters<typeof fetch>) => fetch(...a)),
  setTimeout: env.setTimeout ?? ((fn, ms) => setTimeout(fn, ms)),
  clearTimeout: env.clearTimeout ?? ((t) => clearTimeout(t as ReturnType<typeof setTimeout>)),
});

export class VersionWatch {
  #stale = $state(false);
  #loaded: string | null = null;
  #timer: unknown = null;
  #started = false;
  /** the cancel from the env start() captured — stop() must cancel with the SAME clock the schedule came from, not the real one. */
  #cancel: (t: unknown) => void = (t) => clearTimeout(t as ReturnType<typeof setTimeout>);

  /** stale is the banner's state: the served bundle differs from the loaded one. */
  get stale(): boolean {
    return this.#stale;
  }

  /**
   * start begins polling. Idempotent (a second start is a no-op), and inert
   * unless the page was loaded from a BUILT bundle (isBuiltBundle above): a
   * dev tab is never watched. The first poll fires immediately — a tab opened
   * just before a deploy learns about it one poll early — then the timer
   * reschedules itself; once stale, polling stops (the banner is on and the
   * reload is the player's act).
   */
  start(env: VersionEnv = {}, pollMs = POLL_MS): void {
    if (this.#started) return;
    const doc = env.doc ?? (typeof document === 'undefined' ? null : document);
    const loaded = loadedBundleSrc(doc);
    if (!isBuiltBundle(loaded)) return;
    this.#started = true;
    this.#loaded = loaded;
    const { fetch: doFetch, setTimeout: schedule, clearTimeout: cancel } = defaultEnv(env);
    this.#cancel = cancel;
    const tick = () => {
      this.#timer = null;
      void this.pollOnce(doFetch).then(() => {
        if (this.#stale || !this.#started) return;
        this.#timer = schedule(tick, pollMs);
      });
    };
    tick();
  }

  /** stop cancels a pending poll and drops the watch (the page teardown edge). */
  stop(): void {
    if (this.#timer !== null) this.#cancel(this.#timer);
    this.#timer = null;
    this.#started = false;
  }

  /** pollOnce fetches the served index.html — uncached (see the module doc: a cached pre-deploy index would silence the watch forever) — and compares; every failure mode is silence (swallowed), because none of them is a version change. */
  async pollOnce(doFetch: typeof fetch): Promise<void> {
    try {
      const res = await doFetch(withBase('/'), { cache: 'no-store', headers: { Accept: 'text/html' } });
      if (!res.ok) return;
      const served = parseBundleSrc(await res.text());
      if (bundleStale(this.#loaded, served)) this.#stale = true;
    } catch {
      // The network being down is not a version change.
    }
  }
}

/** versionWatch is the page's single watch, started once by App.svelte's mount and stopped by its teardown. Importing this module never fetches. */
export const versionWatch = new VersionWatch();
