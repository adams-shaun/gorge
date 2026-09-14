import { describe, expect, it, vi } from 'vitest';
import { VersionWatch, bundleStale, loadedBundleSrc, parseBundleSrc, versionWatch } from './versioncheck.svelte';
import { setBasePathForTests } from './basepath';

// The stale-client detector (fb-3ab6d9da defect 2): a tab loaded from an old
// embedded bundle is told when the served index.html names a different one.
// What these tests hold down: the parsing/comparison half, and the watch's
// poll loop under an injected fetch and a manual timer — which also proves
// the loop never fires network traffic unless the wiring starts it (importing
// the module, the thing every test in this suite does, starts nothing).

/** A realistic built index.html, as vite emits it and host/httpapi/static.go serves it. */
const BUILT_HTML = (hash: string): string => `<!doctype html>
<html lang="en"><head><meta charset="UTF-8" /><title>gorge</title></head>
<body><div id="app"></div>
<script type="module" crossorigin src="/assets/index-${hash}.js"></script>
</body></html>`;

describe('parseBundleSrc', () => {
  it('extracts the module script src from a built index.html', () => {
    expect(parseBundleSrc(BUILT_HTML('DrGghxIf'))).toBe('/assets/index-DrGghxIf.js');
  });

  it('matches when crossorigin or attribute order differ', () => {
    const html = '<script src="/assets/index-X.js" type="module"></script>';
    expect(parseBundleSrc(html)).toBe('/assets/index-X.js');
  });

  it('returns null for a page with no module script (an error page, a proxy answer)', () => {
    expect(parseBundleSrc('<html><body>503</body></html>')).toBeNull();
    expect(parseBundleSrc('')).toBeNull();
  });
});

/**
 * docWith is a minimal stub of the Document the watch reads its own script
 * src from: querySelector for the module script, returning a getAttribute
 * stub. The vitest environment is node — no DOMParser, and none is needed:
 * loadedBundleSrc's whole contract is one querySelector.
 */
const docWith = (src: string | null): Document =>
  ({
    querySelector: (sel: string) =>
      sel === 'script[type="module"]' && src !== null ? { getAttribute: () => src } : null,
  }) as unknown as Document;

describe('loadedBundleSrc', () => {
  it("reads the page's own module script src", () => {
    expect(loadedBundleSrc(docWith('/assets/index-abc123.js'))).toBe('/assets/index-abc123.js');
  });

  it('is null without a document (tests, SSR) and for a page without a module script', () => {
    expect(loadedBundleSrc(null)).toBeNull();
    expect(loadedBundleSrc(undefined)).toBeNull();
    expect(loadedBundleSrc(docWith(null))).toBeNull();
  });
});

describe('bundleStale', () => {
  it('is true exactly when both srcs are readable and differ', () => {
    expect(bundleStale('/assets/index-old.js', '/assets/index-new.js')).toBe(true);
    expect(bundleStale('/assets/index-same.js', '/assets/index-same.js')).toBe(false);
    expect(bundleStale(null, '/assets/index-new.js')).toBe(false);
    expect(bundleStale('/assets/index-old.js', null)).toBe(false);
    expect(bundleStale(null, null)).toBe(false);
  });
});

interface PendingTimer {
  at: number;
  fn: () => void;
}

/** manualEnv is a watch environment with a manual clock (setTimeout collects; the test fires) and a counting fetch. */
function manualEnv(html: () => string | undefined, base = '') {
  setBasePathForTests(base);
  const timers: PendingTimer[] = [];
  let clock = 0;
  const fetches: string[] = [];
  const env = {
    fetch: vi.fn(async (input: RequestInfo | URL) => {
      fetches.push(String(input));
      const body = html();
      if (body === undefined) throw new Error('network down');
      return new Response(body, { status: 200, headers: { 'Content-Type': 'text/html' } });
    }),
    setTimeout: (fn: () => void, ms: number) => {
      timers.push({ at: clock + ms, fn });
      return timers.length;
    },
    clearTimeout: (t: unknown) => {
      const i = (t as number) - 1;
      if (i >= 0 && i < timers.length) timers.splice(i, 1);
    },
    doc: docWith('/assets/index-old.js'),
  };
  // fireNext fires ONE scheduled cadence tick and then drains every
  // microtask the tick scheduled (a setImmediate callback runs only after the
  // microtask queue is fully empty), so the watch's .then reschedule has
  // landed before the next assertion. A plain double await would race that
  // reschedule on the fast rejection path — the earlier version of this
  // helper spun forever there and OOMed the worker.
  const fireNext = async () => {
    if (!timers.length) throw new Error('fireNext: no scheduled tick');
    clock = timers[0].at;
    timers.shift()!.fn();
    await new Promise((r) => setImmediate(r));
  };
  return { env, fetches, fireNext, timers };
}

describe('VersionWatch', () => {
  it('the module\'s shared watch starts nothing: importing this suite never fetches and never arms the banner', () => {
    expect(versionWatch.stale).toBe(false);
  });

  it('a dev tab (script src not under /assets/) is never watched: start() is inert', async () => {
    const { env, fetches } = manualEnv(() => undefined);
    env.doc = docWith('/src/main.ts');
    const w = new VersionWatch();
    w.start(env);
    await new Promise((r) => setImmediate(r));
    expect(fetches).toEqual([]);
    expect(w.stale).toBe(false);
  });

  it('start() polls once immediately, then keeps the slow cadence; an unchanged hash stays silent', async () => {
    const { env, fetches, fireNext, timers } = manualEnv(() => BUILT_HTML('old'));
    const w = new VersionWatch();
    w.start(env);
    await new Promise((r) => setImmediate(r));
    expect(fetches).toEqual(['/']); // the first poll, at the served root
    expect(w.stale).toBe(false); // unchanged hash: silent
    expect(timers).toHaveLength(1); // and the cadence is armed

    await fireNext();
    expect(fetches).toHaveLength(2); // one more poll per cadence tick
    expect(w.stale).toBe(false);
    expect(timers).toHaveLength(1); // and still armed
    w.stop();
  });

  it('a changed hash flips the banner on and stops polling — the player reloaded is the next act', async () => {
    let served = BUILT_HTML('old');
    const { env, fetches, fireNext, timers } = manualEnv(() => served);
    const w = new VersionWatch();
    w.start(env);
    await new Promise((r) => setImmediate(r));
    expect(fetches).toHaveLength(1);
    expect(w.stale).toBe(false);

    served = BUILT_HTML('new'); // a redeploy happened
    await fireNext(); // the scheduled poll sees the new bundle
    expect(w.stale).toBe(true);
    expect(timers).toHaveLength(0); // nothing was rescheduled once stale

    const after = fetches.length;
    await new Promise((r) => setImmediate(r));
    expect(fetches).toHaveLength(after); // polling stopped once stale
    expect(w.stale).toBe(true);
  });

  it('a poll error is swallowed — the network being down is not a version change', async () => {
    const { env, fetches, timers, fireNext } = manualEnv(() => undefined); // fetch throws
    const w = new VersionWatch();
    w.start(env);
    await new Promise((r) => setImmediate(r));
    expect(fetches).toHaveLength(1);
    expect(w.stale).toBe(false);

    await fireNext();
    expect(fetches).toHaveLength(2); // the loop survived the failure and rescheduled
    expect(w.stale).toBe(false);
    expect(timers).toHaveLength(1);
    w.stop();
  });

  it('a non-200 answer is silent too', async () => {
    const { env, fetches } = manualEnv(() => BUILT_HTML('new'));
    env.fetch = vi.fn(async (input: RequestInfo | URL) => {
      fetches.push(String(input)); // the overridden fetch records too, or the assertion below is vacuous
      return new Response('nope', { status: 503 });
    });
    const w = new VersionWatch();
    w.start(env);
    await new Promise((r) => setImmediate(r));
    expect(fetches).toHaveLength(1);
    expect(w.stale).toBe(false);
  });

  it('start() is idempotent and stop() cancels a pending poll', async () => {
    const { env, fetches, timers } = manualEnv(() => BUILT_HTML('old'));
    const w = new VersionWatch();
    w.start(env);
    w.start(env);
    await new Promise((r) => setImmediate(r));
    expect(fetches).toHaveLength(1); // the second start added no poll
    expect(timers).toHaveLength(1);

    w.stop();
    expect(timers).toHaveLength(0); // the cadence died with the page
  });

  it('the poll rides the base path when the page is mounted under a prefix', async () => {
    const { env, fetches } = manualEnv(() => BUILT_HTML('old'), '/gorge');
    const w = new VersionWatch();
    w.start(env);
    await new Promise((r) => setImmediate(r));
    expect(fetches).toEqual(['/gorge/']);
    w.stop();
  });
});
