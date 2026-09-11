/**
 * oracle resolves an exact card name to its printed facts — oracle text,
 * mana cost, type line, printed power/toughness — from an EXTERNAL catalog.
 *
 * gorge itself compiles no card text onto the wire from Forge's GPL-3.0
 * scripts; the text comes from a catalog chosen by the EMBEDDING page and
 * named by its <meta name="gorge-cards" content="…"> tag, exactly the way
 * basepath.ts reads <meta name="gorge-base"> — read ONCE at startup, needs
 * no script tag (strict CSP), no build-time config.
 *
 * The tag's PRESENCE opts in, its content names the catalog: a URL names
 * that catalog; EMPTY content names the SAME-ORIGIN catalog (gorged itself
 * serves one at /cards/named — see cmd/gorged/art.go — and injects the tag
 * into the served index.html). Presence is the bit because normalize("/")
 * is "" and "" is indistinguishable from "no tag" in the string alone — the
 * empty base must still issue a request. No tag at all stays the
 * no-request default: an embedding that ships no catalog (the local
 * fixture, a bare mtgserve) resolves null immediately, which is the honest
 * answer, not an error.
 */

export interface OracleCard {
  name: string;
  mana_cost?: string;
  type_line?: string;
  oracle_text?: string;
  power?: string;
  toughness?: string;
}

export interface OracleSource {
  fetch: typeof fetch;
  now: () => number;
  setTimeout: (fn: () => void, ms: number) => void;
  storage: Storage | null;
}

const SPACING = 100;
const OFFLINE_FOR = 60_000;
const KEY = 'gorge.oracle.';

// Read once at module load, like basepath. A meta tag is the source of
// choice because it is inert — nothing else on the page reads it. null is
// "the page ships no catalog" (no tag); a string — INCLUDING the empty
// string, from a tag with empty content — is the request prefix, so ""
// means same-origin.
let base: string | null = detect();

function normalize(raw: string): string {
  return raw.replace(/\/+$/, '');
}

function detect(): string | null {
  // vitest runs without a served page (so without a DOM); there is no meta
  // tag to read and no catalog — which is also the correct default.
  if (typeof document === 'undefined') return null;
  const el = document.querySelector('meta[name="gorge-cards"]');
  // PRESENCE is the opt-in bit, even for content="": an empty content
  // normalises to "" and must still be a catalog (the same-origin one),
  // never silently collapse into the no-catalog guard below.
  if (!el) return null;
  return normalize(el?.getAttribute('content') ?? '');
}

/** setOracleBaseForTests is the test hook (the vitest page has no served meta tag); production code never calls it.
 *  null is "no catalog at all" (identical to no meta tag); "" is a SAME-ORIGIN catalog — the exact values detect() can produce. */
export function setOracleBaseForTests(b: string | null): void {
  base = b === null ? null : normalize(b);
}

/**
 * createOracle resolves exact card names to catalog entries with the same
 * discipline as images.ts: memory + localStorage caches, a 100ms request
 * spacing queue, and a 60s offline backoff on any failure other than a 404
 * (a 404 is a KNOWN miss — cached as null, not an error).
 */
export function createOracle(src: Partial<OracleSource> = {}) {
  const env: OracleSource = {
    fetch: src.fetch ?? ((...a) => fetch(...a)),
    now: src.now ?? (() => Date.now()),
    setTimeout: src.setTimeout ?? ((fn, ms) => void setTimeout(fn, ms)),
    storage: src.storage === undefined ? safeStorage() : src.storage,
  };
  const memo = new Map<string, OracleCard | null>();
  const pending = new Map<string, Promise<OracleCard | null>>();
  const queue: (() => void)[] = [];
  let offlineUntil = 0;

  const offline = () => env.now() < offlineUntil;

  function fromStorage(name: string): OracleCard | null | undefined {
    try {
      const v = env.storage?.getItem(KEY + name);
      if (v === null || v === undefined) return undefined;
      if (v === '') return null;
      return JSON.parse(v) as OracleCard;
    } catch {
      return undefined;
    }
  }
  function toStorage(name: string, card: OracleCard | null) {
    try {
      env.storage?.setItem(KEY + name, card ? JSON.stringify(card) : '');
    } catch {
      /* quota, private mode, or a storage that throws — the cache is best-effort */
    }
  }

  // One lookup every SPACING ms at most; a call that finds nothing else in
  // flight and nothing queued fires straight away, so ordinary sequential
  // use gets no artificial delay.
  function drain() {
    const next = queue.shift();
    if (next) next();
    if (queue.length) env.setTimeout(drain, SPACING);
  }

  async function lookup(name: string): Promise<OracleCard | null> {
    const res = await env.fetch(`${base}/cards/named?exact=${encodeURIComponent(name)}`, { headers: { Accept: 'application/json' } });
    if (res.status === 404) return null;
    if (!res.ok) throw new Error(`oracle ${res.status}`);
    const j = (await res.json()) as OracleCard;
    // Normalise to exactly the six published fields; a catalog may send more
    // (art URLs, set data) and this cache neither needs nor stores them.
    return {
      name: j.name ?? name,
      mana_cost: j.mana_cost,
      type_line: j.type_line,
      oracle_text: j.oracle_text,
      power: j.power,
      toughness: j.toughness,
    };
  }

  function text(name: string): Promise<OracleCard | null> {
    // No catalog (no meta tag at all): the honest answer is "we have no
    // text", and the invariant is that not a single request is issued for
    // it. A tag WITH empty content is a catalog (same-origin) and falls
    // through to a real request.
    if (base === null) return Promise.resolve(null);
    if (memo.has(name)) return Promise.resolve(memo.get(name)!);
    const stored = fromStorage(name);
    if (stored !== undefined) {
      memo.set(name, stored);
      return Promise.resolve(stored);
    }
    if (offline()) return Promise.resolve(null);
    const inflight = pending.get(name);
    if (inflight) return inflight;

    const idle = queue.length === 0 && pending.size === 0;
    const p = new Promise<OracleCard | null>((resolve) => {
      const run = () => {
        // A queued lookup can sit a while before its turn; re-check offline
        // here too, so a source that went offline after this was queued
        // doesn't still fire once its slot comes up.
        if (offline()) {
          resolve(null);
          return;
        }
        lookup(name)
          .then((c) => {
            memo.set(name, c);
            toStorage(name, c);
            resolve(c);
          })
          .catch(() => {
            offlineUntil = env.now() + OFFLINE_FOR;
            resolve(null);
          });
      };
      if (idle) run();
      else {
        queue.push(run);
        if (queue.length === 1) env.setTimeout(drain, SPACING);
      }
    }).finally(() => pending.delete(name));
    pending.set(name, p);
    return p;
  }

  return { text, offline };
}

function safeStorage(): Storage | null {
  try {
    return typeof localStorage === 'undefined' ? null : localStorage;
  } catch {
    return null;
  }
}

export const oracle = createOracle();
