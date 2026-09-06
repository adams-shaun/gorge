import { describe, expect, it, vi } from 'vitest';
import { createOracle, setOracleBaseForTests } from './oracle';

// fakeEnv is a deterministic catalog server: named cards resolve from
// `responses` (an Error rejects, the number 500 makes the server fail, any
// unknown name is a 404), everything else is an injected clock and storage.
function fakeEnv(responses: Record<string, unknown | Error | 500>) {
  const calls: string[] = [];
  let clock = 0;
  const timers: { at: number; fn: () => void }[] = [];
  const store = new Map<string, string>();
  const storage = { getItem: (k: string) => store.get(k) ?? null, setItem: (k: string, v: string) => void store.set(k, v) } as unknown as Storage;
  const env = {
    fetch: (async (url: string) => {
      calls.push(String(url));
      const name = decodeURIComponent(new URL(url).searchParams.get('exact')!);
      const r = responses[name];
      if (r instanceof Error) throw r;
      if (r === 500) return new Response('{"error":"boom"}', { status: 500 });
      if (r === undefined) return new Response('{}', { status: 404 });
      return new Response(JSON.stringify(r), { status: 200 });
    }) as unknown as typeof fetch,
    now: () => clock,
    setTimeout: (fn: () => void, ms: number) => void timers.push({ at: clock + ms, fn }),
    storage,
  };
  const tick = (ms: number) => {
    clock += ms;
    for (const t of timers.splice(0)) if (t.at <= clock) t.fn();
    else timers.push(t);
  };
  return { env, calls, tick, store };
}

const GOBLIN = {
  name: 'Goblin Guide', mana_cost: '1 R', type_line: 'Creature — Goblin Scout',
  oracle_text: 'Haste', power: '2', toughness: '2',
};

describe('createOracle', () => {
  // The module-level base is read once at startup; each test names its own
  // catalog (or none) so ordering can never leak between tests.
  it('with no catalog meta tag, text() resolves null and never issues a request', async () => {
    setOracleBaseForTests('');
    const { env, calls } = fakeEnv({ 'Goblin Guide': GOBLIN });
    const o = createOracle(env);
    expect(await o.text('Goblin Guide')).toBeNull();
    expect(await o.text('Anything At All')).toBeNull();
    expect(calls.length).toBe(0);
    expect(o.offline()).toBe(false);
  });

  it('resolves text from the catalog, caches in memory and storage, and 404s cache as a known null', async () => {
    setOracleBaseForTests('https://cat/');
    const { env, calls, store } = fakeEnv({ 'Goblin Guide': GOBLIN });
    const o = createOracle(env);
    expect(calls).toEqual([]);
    const card = await o.text('Goblin Guide');
    expect(card?.oracle_text).toBe('Haste');
    expect(card?.power).toBe('2');
    expect(calls).toEqual(['https://cat/cards/named?exact=Goblin%20Guide']);
    // second call is served from the cache: no second request
    expect(await o.text('Goblin Guide')).toEqual(card);
    expect(calls.length).toBe(1);
    expect(store.get('gorge.oracle.Goblin Guide')).toContain('"oracle_text":"Haste"');

    // an unknown name is a 404: null, cached as null, still one request
    expect(await o.text('Nobody Home')).toBeNull();
    expect(await o.text('Nobody Home')).toBeNull();
    expect(calls.length).toBe(2);
    expect(store.get('gorge.oracle.Nobody Home')).toBe('');
  });

  it('a 404 is a cached null, not an error and not a retry', async () => {
    setOracleBaseForTests('https://cat');
    const { env, calls } = fakeEnv({});
    const o = createOracle(env);
    expect(await o.text('Missing')).toBeNull();
    expect(await o.text('Missing')).toBeNull();
    expect(calls.length).toBe(1);
    expect(o.offline()).toBe(false); // a known miss does NOT trip the backoff
  });

  it('any other failure trips the offline backoff: no further requests inside the window', async () => {
    setOracleBaseForTests('https://cat');
    const { env, calls, tick } = fakeEnv({ 'Goblin Guide': 500 });
    const o = createOracle(env);
    expect(await o.text('Goblin Guide')).toBeNull();
    expect(o.offline()).toBe(true);
    // inside the 60s window every ask resolves null without touching the network
    expect(await o.text('Goblin Guide')).toBeNull();
    expect(await o.text('Some Other Card')).toBeNull();
    expect(calls.length).toBe(1);
    tick(60_000);
    expect(o.offline()).toBe(false);
    // after the window the catalog is asked again
    expect(await o.text('Goblin Guide')).toBeNull();
    expect(calls.length).toBe(2);
  });

  it('a rejected fetch (network error) backs off the same way', async () => {
    setOracleBaseForTests('https://cat');
    const { env, calls, tick } = fakeEnv({ A: new Error('net down') });
    const o = createOracle(env);
    expect(await o.text('A')).toBeNull();
    expect(o.offline()).toBe(true);
    expect(await o.text('B')).toBeNull();
    expect(calls.length).toBe(1);
    tick(60_000);
    expect(o.offline()).toBe(false);
  });

  it('spaces requests at least 100ms apart', async () => {
    setOracleBaseForTests('https://cat');
    const { env, calls, tick } = fakeEnv({
      A: { name: 'A' }, B: { name: 'B' }, C: { name: 'C' },
    });
    const o = createOracle(env);
    const all = Promise.all([o.text('A'), o.text('B'), o.text('C')]);
    await Promise.resolve();
    expect(calls.length).toBe(1);
    tick(100); await Promise.resolve(); await Promise.resolve();
    expect(calls.length).toBe(2);
    tick(100); await Promise.resolve(); await Promise.resolve();
    expect(calls.length).toBe(3);
    expect((await all).map((c) => c?.name)).toEqual(['A', 'B', 'C']);
  });

  it('re-checks offline before dispatching an already-queued lookup', async () => {
    setOracleBaseForTests('https://cat');
    const { env, calls, tick } = fakeEnv({
      A: new Error('net down'), B: { name: 'B' }, C: { name: 'C' },
    });
    const o = createOracle(env);
    const all = Promise.all([o.text('A'), o.text('B'), o.text('C')]);
    await Promise.resolve(); await Promise.resolve(); await Promise.resolve();
    expect(calls.length).toBe(1);
    expect(o.offline()).toBe(true);
    tick(100); await Promise.resolve(); await Promise.resolve();
    tick(100); await Promise.resolve(); await Promise.resolve();
    expect(calls.length).toBe(1);
    expect((await all).map((c) => c ?? null)).toEqual([null, null, null]);
  });

  it('a storage that throws is ignored, not fatal', async () => {
    setOracleBaseForTests('https://cat');
    const { env } = fakeEnv({ 'Goblin Guide': GOBLIN });
    const throwing = {
      getItem: () => { throw new Error('denied'); },
      setItem: () => { throw new Error('denied'); },
    } as unknown as Storage;
    const o = createOracle({ ...env, storage: throwing });
    const card = await o.text('Goblin Guide');
    expect(card?.name).toBe('Goblin Guide');
    expect(await o.text('Goblin Guide')).toEqual(card);
  });
});

// The base URL is read ONCE at module load, from the served page's meta tag.
// The default vitest page has no doc and no tag — which is the no-catalog
// default — so this test re-imports the module behind a stubbed document and
// lives LAST: after resetModules the earlier static imports are untouched.
describe('catalog meta tag', () => {
  it('reads the base once, at import, from <meta name="gorge-cards">', async () => {
    vi.stubGlobal('document', {
      querySelector: (sel: string) =>
        sel === 'meta[name="gorge-cards"]' ? { getAttribute: () => 'https://cat/' } : null,
    });
    vi.resetModules();
    const mod = await import('./oracle');
    const calls: string[] = [];
    const o = mod.createOracle({
      fetch: (async (url: string) => {
        calls.push(String(url));
        return new Response('{}', { status: 404 });
      }) as unknown as typeof fetch,
      now: () => 0,
      setTimeout: () => {},
      storage: null,
    });
    expect(await o.text('Llanowar Elves')).toBeNull();
    expect(calls).toEqual(['https://cat/cards/named?exact=Llanowar%20Elves']);
    vi.unstubAllGlobals();
  });
});
