import { withBase } from './basepath';
import { safeStorage } from './storage';

export interface ImageSource {
  fetch: typeof fetch;
  now: () => number;
  setTimeout: (fn: () => void, ms: number) => void;
  storage: Storage | null;
}

const SPACING = 100;
const OFFLINE_FOR = 60_000;
// REQUEST_TIMEOUT bounds one /art/named fetch. gorged's art proxy serialises
// every uncached lookup behind a pacing semaphore whose 429 backoff sleeps
// HOLD the semaphore (cmd/gorged/art.go), so under Scryfall rate limiting a
// fetch can block for minutes — measured: 3 hand-card lookups at ~8.9s each
// and 4 more that never completed in a whole smoke test. A browser holds one
// of its ~6 per-host HTTP/1.1 connections for every such fetch, so unbounded
// art lookups starve the page's own state channel (view/events/pending) and
// the client wedges on "waiting for the game to advance" while the SSE
// stream — exempt from that pool — keeps arriving (task
// fb-20260916T225456Z, seated-smoke R-E4-1 timeout). Aborting after 10s
// frees the socket; the catch path below takes its existing 60s offline
// rest (nothing is stored, so the name re-resolves later) while the
// server's prewarm warms the cache meanwhile.
const REQUEST_TIMEOUT = 10_000;
// Version the browser cache independently of the server's art blobs. Existing
// browsers may hold a valid immutable URL whose bytes were selected by older
// face-matching logic; changing this namespace makes them resolve the name
// again instead of returning that stale URL before lookup() can run.
// v3 (task fb-20260914T113850Z-682e875e): a name resolved to null while the
// server's art cache was cold (each deploy used to wipe it) is stored
// permanently by toStorage and never re-asked. The server now keeps a
// durable, prewarmed cache, so bumping the namespace lets such a stored null
// re-resolve once against the warm server; every miss that re-resolves to a
// miss costs one same-origin request.
// Exported so every test fixture that seeds image-storage keys derives its
// keys from this one constant — a future bump cannot silently strand a
// fixture on the old namespace again (CardDetail.geometry.ts did exactly
// that on this bump).
export const IMAGE_KEY = 'gorge.img.v3.';
const KEY = IMAGE_KEY;

type Scryfall = {
  image_uris?: { normal?: string };
  card_faces?: { name?: string; image_uris?: { normal?: string } }[];
};

/** createImages resolves exact card names to card art served from this app's own origin (art.go proxies and caches Scryfall server-side) with memory + localStorage caches, request spacing and an offline backoff. */
export function createImages(src: Partial<ImageSource> = {}) {
  const env: ImageSource = {
    fetch: src.fetch ?? ((...a) => fetch(...a)),
    now: src.now ?? (() => Date.now()),
    setTimeout: src.setTimeout ?? ((fn, ms) => void setTimeout(fn, ms)),
    storage: src.storage === undefined ? safeStorage() : src.storage,
  };
  const memo = new Map<string, string | null>();
  const pending = new Map<string, Promise<string | null>>();
  const queue: (() => void)[] = [];
  let offlineUntil = 0;

  const offline = () => env.now() < offlineUntil;

  function fromStorage(name: string): string | null | undefined {
    // Deliberately do not fall back to an older namespace: those entries can
    // point at still-served immutable blobs containing incorrectly chosen art.
    try { const v = env.storage?.getItem(KEY + name); return v === null || v === undefined ? undefined : v || null; } catch { return undefined; }
  }
  function toStorage(name: string, url: string | null) {
    try { env.storage?.setItem(KEY + name, url ?? ''); } catch { /* quota or private mode */ }
  }

  // Scryfall asks for <=10 req/s. A request that finds nothing else in flight
  // and nothing queued fires straight away; one that arrives while another is
  // outstanding joins a queue drained one item every SPACING ms, so bursts
  // stay paced without adding artificial delay to ordinary sequential use.
  function drain() {
    const next = queue.shift();
    if (next) next();
    if (queue.length) env.setTimeout(drain, SPACING);
  }

  async function lookup(name: string): Promise<string | null> {
    // Same-origin, not Scryfall directly: cmd/gorged's /art/named proxies and
    // caches Scryfall on the server (art.go), so every viewer's browser only
    // ever talks to this app's own origin, and a card fetched once by any
    // viewer is served from disk for every viewer after. The response shape
    // mirrors Scryfall's own /cards/named (name + image_uris.normal) on
    // purpose, so this parsing needs no change from when it read Scryfall
    // directly — only the request's base URL moved.
    // REQUEST_TIMEOUT (see the constant's comment) bounds the whole lookup —
    // body read included — because every millisecond of it holds a browser
    // connection. The abort is armed through env.setTimeout so the fake-clock
    // tests drive it; aborting an already-settled controller is a no-op.
    const abort = new AbortController();
    let settled = false;
    env.setTimeout(() => { if (!settled) abort.abort(); }, REQUEST_TIMEOUT);
    try {
      const res = await env.fetch(withBase(`/art/named?exact=${encodeURIComponent(name)}`), { headers: { Accept: 'application/json' }, signal: abort.signal });
      if (res.status === 404) return null;
      if (!res.ok) throw new Error(`art ${res.status}`);
      const j = (await res.json()) as Scryfall;
      // A multi-faced card (Delver of Secrets / Insectile Aberration) puts no
      // top-level image_uris in the response and lists card_faces[0] = the
      // FRONT face for either name — so the front-face-only fallback resolved
      // a back-face name to the front art forever (task
      // fb-20260914T033246Z-3f1cc033, defect 3). Pick the face whose printed
      // name is the requested one; fall back to the front face, then to the
      // top-level image (single-faced cards).
      const face = j.card_faces?.find((f) => f.name === name);
      const path =
        j.image_uris?.normal ??
        face?.image_uris?.normal ??
        j.card_faces?.[0]?.image_uris?.normal ??
        null;
      return path ? withBase(path) : null;
    } finally {
      settled = true;
    }
  }

  function url(name: string): Promise<string | null> {
    if (memo.has(name)) return Promise.resolve(memo.get(name)!);
    const stored = fromStorage(name);
    if (stored !== undefined) { memo.set(name, stored); return Promise.resolve(stored); }
    if (offline()) return Promise.resolve(null);
    const inflight = pending.get(name);
    if (inflight) return inflight;

    const idle = queue.length === 0 && pending.size === 0;
    const p = new Promise<string | null>((resolve) => {
      const run = () => {
        // A queued lookup can sit for a while before its turn; re-check here
        // too, so a source that went offline after this was queued doesn't
        // still fire the request once its slot comes up.
        if (offline()) { resolve(null); return; }
        lookup(name).then((u) => { memo.set(name, u); toStorage(name, u); resolve(u); })
          .catch(() => { offlineUntil = env.now() + OFFLINE_FOR; resolve(null); });
      };
      if (idle) run();
      else { queue.push(run); if (queue.length === 1) env.setTimeout(drain, SPACING); }
    }).finally(() => pending.delete(name));
    pending.set(name, p);
    return p;
  }

  return { url, offline };
}

export const images = createImages();
