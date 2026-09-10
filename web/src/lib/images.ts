import { withBase } from './basepath';

export interface ImageSource {
  fetch: typeof fetch;
  now: () => number;
  setTimeout: (fn: () => void, ms: number) => void;
  storage: Storage | null;
}

const SPACING = 100;
const OFFLINE_FOR = 60_000;
const KEY = 'gorge.img.';

type Scryfall = { image_uris?: { normal?: string }; card_faces?: { image_uris?: { normal?: string } }[] };

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
    const res = await env.fetch(withBase(`/art/named?exact=${encodeURIComponent(name)}`), { headers: { Accept: 'application/json' } });
    if (res.status === 404) return null;
    if (!res.ok) throw new Error(`art ${res.status}`);
    const j = (await res.json()) as Scryfall;
    const path = j.image_uris?.normal ?? j.card_faces?.[0]?.image_uris?.normal ?? null;
    return path ? withBase(path) : null;
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

function safeStorage(): Storage | null {
  try { return typeof localStorage === 'undefined' ? null : localStorage; } catch { return null; }
}

export const images = createImages();
