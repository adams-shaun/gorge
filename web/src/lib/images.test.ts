import { describe, expect, it } from 'vitest';
import { createImages, IMAGE_KEY } from './images';

function fakeEnv(responses: Record<string, unknown | Error>) {
  const calls: string[] = [];
  let clock = 0;
  const timers: { at: number; fn: () => void }[] = [];
  const store = new Map<string, string>();
  const storage = { getItem: (k: string) => store.get(k) ?? null, setItem: (k: string, v: string) => void store.set(k, v) } as unknown as Storage;
  const env = {
    fetch: (async (url: string) => {
      calls.push(url);
      const name = decodeURIComponent(new URL(url, 'http://localhost').searchParams.get('exact')!);
      const r = responses[name];
      if (r instanceof Error) throw r;
      if (r === undefined) return new Response('{}', { status: 404 });
      return new Response(JSON.stringify(r), { status: 200 });
    }) as unknown as typeof fetch,
    now: () => clock,
    setTimeout: (fn: () => void, ms: number) => void timers.push({ at: clock + ms, fn }),
    storage,
  };
  const tick = (ms: number) => { clock += ms; for (const t of timers.splice(0)) if (t.at <= clock) t.fn(); else timers.push(t); };
  return { env, calls, tick, store };
}

describe('images', () => {
  it('asks nothing for a face-down card, which has no name', async () => {
    const { env, calls } = fakeEnv({});
    await expect(createImages(env).url('')).resolves.toBeNull();
    expect(calls).toHaveLength(0);
  });

  it('asks this app\'s own /art/named, never Scryfall directly', async () => {
    const { env, calls } = fakeEnv({ 'Goblin Guide': { image_uris: { normal: '/art/blob/abc.jpg' } } });
    await createImages(env).url('Goblin Guide');
    expect(calls).toHaveLength(1);
    expect(calls[0]).toMatch(/^\/art\/named\?exact=/);
    expect(calls[0]).not.toContain('scryfall');
  });
  it('resolves a token through /art/token under a namespaced key, leaving a same-named card entry alone', async () => {
    const { env, calls, store } = fakeEnv({ 'Goblin Token': { image_uris: { normal: '/art/blob/tok.jpg' } } });
    const im = createImages(env);
    expect(await im.url('Goblin Token', { isToken: true, types: 'Creature Goblin', pt: '1/1', printedPt: '1/1' })).toBe('/art/blob/tok.jpg');
    expect(calls[0]).toMatch(/^\/art\/token\?exact=Goblin%20Token&types=Creature%20Goblin&pt=1%2F1&ppt=1%2F1$/);
    // The entry lives under the token namespace, keyed by name+types+PRINTED
    // P/T, so a real card named exactly "Goblin Token" keeps its own storage
    // slot.
    expect(store.get(`${IMAGE_KEY}token\u001fGoblin Token\u001fCreature Goblin\u001f1/1`)).toBe('/art/blob/tok.jpg');
    expect(store.get(`${IMAGE_KEY}Goblin Token`)).toBeUndefined();
    // A different PRINTED P/T under the same display name resolves again, not
    // from cache.
    expect(await im.url('Goblin Token', { isToken: true, types: 'Creature Goblin', pt: '3/3', printedPt: '3/3' })).toBe('/art/blob/tok.jpg');
    expect(calls).toHaveLength(2);
  });
  it('keys a token on its PRINTED P/T, so a pumped sibling shares the entry', async () => {
    const { env, calls, store } = fakeEnv({ 'Soldier Token': { image_uris: { normal: '/art/blob/soldier.jpg' } } });
    const im = createImages(env);
    // Un-pumped: the current P/T equals the printed 1/1.
    expect(await im.url('Soldier Token', { isToken: true, types: 'Creature Soldier', pt: '1/1', printedPt: '1/1' })).toBe('/art/blob/soldier.jpg');
    // Pumped to 2/2 by a +1/+1 counter: the printed P/T is unchanged, so this
    // is the SAME entry — no second request, no second storage slot.
    expect(await im.url('Soldier Token', { isToken: true, types: 'Creature Soldier', pt: '2/2', printedPt: '1/1' })).toBe('/art/blob/soldier.jpg');
    expect(calls).toHaveLength(1);
    expect(store.get(`${IMAGE_KEY}token\u001fSoldier Token\u001fCreature Soldier\u001f1/1`)).toBe('/art/blob/soldier.jpg');
    expect(store.get(`${IMAGE_KEY}token\u001fSoldier Token\u001fCreature Soldier\u001f2/2`)).toBeUndefined();
  });
  it('resolves the normal image, caches in memory and storage, and treats 404 as a known miss', async () => {
    const { env, calls, store } = fakeEnv({ 'Goblin Guide': { image_uris: { normal: 'https://img/gg.jpg' } } });
    const im = createImages(env);
    expect(await im.url('Goblin Guide')).toBe('https://img/gg.jpg');
    expect(await im.url('Goblin Guide')).toBe('https://img/gg.jpg');
    expect(calls.length).toBe(1);
    expect(store.get(`${IMAGE_KEY}Goblin Guide`)).toBe('https://img/gg.jpg');
    expect(await im.url('Nonexistent')).toBeNull();
    expect(await im.url('Nonexistent')).toBeNull();
    expect(calls.length).toBe(2);
  });
  it('ignores a legacy cached blob URL and resolves into the versioned browser cache', async () => {
    const { env, calls, store } = fakeEnv({
      'Insectile Aberration': {
        card_faces: [
          { name: 'Delver of Secrets', image_uris: { normal: '/art/blob/new-front.jpg' } },
          { name: 'Insectile Aberration', image_uris: { normal: '/art/blob/new-back.jpg' } },
        ],
      },
    });
    // A browser that visited before the face picker was fixed has the old
    // server blob URL under the unversioned namespace. That blob remains
    // valid and immutable, so only a browser-side namespace rotation can
    // prevent url() from returning it before /art/named is consulted.
    store.set('gorge.img.Insectile Aberration', '/art/blob/old-front.jpg');

    expect(await createImages(env).url('Insectile Aberration')).toBe('/art/blob/new-back.jpg');
    expect(calls).toHaveLength(1);
    expect(calls[0]).toMatch(/^\/art\/named\?exact=Insectile%20Aberration$/);
    expect(store.get(`${IMAGE_KEY}Insectile Aberration`)).toBe('/art/blob/new-back.jpg');
    expect(store.get('gorge.img.Insectile Aberration')).toBe('/art/blob/old-front.jpg');
  });
  it('uses the front face of a double-faced card when no face name matches', async () => {
    const { env } = fakeEnv({ 'Delver of Secrets': { card_faces: [{ image_uris: { normal: 'https://img/front.jpg' } }, { image_uris: { normal: 'https://img/back.jpg' } }] } });
    expect(await createImages(env).url('Delver of Secrets')).toBe('https://img/front.jpg');
  });
  it('resolves a back-face name to the back face of a double-faced card', async () => {
    // Scryfall lists BOTH faces
    // for either name of a transform card and leaves the top-level
    // image_uris empty, so the old front-face-only fallback resolved
    // "Insectile Aberration" to Delver of Secrets' art forever and a
    // transformed Delver never displayed its back side. The face whose
    // printed name is the requested one must win.
    const { env } = fakeEnv({
      'Insectile Aberration': {
        card_faces: [
          { name: 'Delver of Secrets', image_uris: { normal: 'https://img/front.jpg' } },
          { name: 'Insectile Aberration', image_uris: { normal: 'https://img/back.jpg' } },
        ],
      },
    });
    expect(await createImages(env).url('Insectile Aberration')).toBe('https://img/back.jpg');
  });
  it('prefers a top-level image over the face list when one is present', async () => {
    const { env } = fakeEnv({
      'Insectile Aberration': {
        image_uris: { normal: 'https://img/top.jpg' },
        card_faces: [
          { name: 'Delver of Secrets', image_uris: { normal: 'https://img/front.jpg' } },
          { name: 'Insectile Aberration', image_uris: { normal: 'https://img/back.jpg' } },
        ],
      },
    });
    expect(await createImages(env).url('Insectile Aberration')).toBe('https://img/top.jpg');
  });
  it('spaces requests at least 100ms apart', async () => {
    const { env, calls, tick } = fakeEnv({ A: { image_uris: { normal: 'a' } }, B: { image_uris: { normal: 'b' } }, C: { image_uris: { normal: 'c' } } });
    const im = createImages(env);
    const all = Promise.all([im.url('A'), im.url('B'), im.url('C')]);
    await Promise.resolve();
    expect(calls.length).toBe(1);
    tick(100); await Promise.resolve(); await Promise.resolve();
    expect(calls.length).toBe(2);
    tick(100); await Promise.resolve(); await Promise.resolve();
    expect(calls.length).toBe(3);
    expect(await all).toEqual(['a', 'b', 'c']);
  });
  it('goes offline on a network error and recovers after 60s', async () => {
    const { env, calls, tick } = fakeEnv({ A: new Error('net down') });
    const im = createImages(env);
    expect(await im.url('A')).toBeNull();
    expect(im.offline()).toBe(true);
    expect(await im.url('B')).toBeNull();
    expect(calls.length).toBe(1);
    tick(60_000);
    expect(im.offline()).toBe(false);
  });
  it('retries a 503 pending name on its own, never going offline, and keeps resolving other names', async () => {
    // cmd/gorged answers 503 when its Scryfall fetch outlives the wait
    // budget (errArtPending): a cold cache does this for a burst of names.
    // It used to take the 60s offline rest and blank every later card.
    const { env, calls, tick, store } = fakeEnv({ B: { image_uris: { normal: '/art/blob/b.jpg' } } });
    let aTries = 0;
    const base = env.fetch;
    env.fetch = (async (url: string, init?: RequestInit) => {
      if (url.includes('exact=A')) {
        calls.push(url);
        aTries++;
        if (aTries <= 2) return new Response('pending', { status: 503 });
        return new Response(JSON.stringify({ image_uris: { normal: '/art/blob/a.jpg' } }), { status: 200 });
      }
      return base(url, init);
    }) as unknown as typeof fetch;
    const im = createImages(env);
    const a = im.url('A');
    await Promise.resolve(); await Promise.resolve();
    expect(im.offline()).toBe(false);
    const b = im.url('B');
    for (let i = 0; i < 4; i++) { tick(100); for (let k = 0; k < 6; k++) await Promise.resolve(); }
    expect(await b).toBe('/art/blob/b.jpg');
    for (let i = 0; i < 3; i++) { tick(3_000); for (let k = 0; k < 6; k++) await Promise.resolve(); }
    expect(await a).toBe('/art/blob/a.jpg');
    expect(aTries).toBe(3);
    expect(im.offline()).toBe(false);
    expect(store.get(IMAGE_KEY + 'A')).toBe('/art/blob/a.jpg');
  });
  it('aborts a lookup still in flight after the 10s timeout, resolves null, and never stores the miss', async () => {
    // The production failure this pins: gorged's
    // /art/named proxy queues behind the art cache's pacing semaphore while
    // the prewarm's Scryfall 429 backoff sleeps HOLD it (cmd/gorged/art.go),
    // so a fetch can block for minutes — and every millisecond of it holds
    // one of the browser's ~6 per-host connections, starving the page's own
    // state channel (view/events/pending) while the SSE stream keeps
    // arriving. The lookup must abort on the injectable clock instead.
    let clock = 0;
    const timers: { at: number; fn: () => void }[] = [];
    const calls: string[] = [];
    const signals: AbortSignal[] = [];
    const store = new Map<string, string>();
    const storage = { getItem: (k: string) => store.get(k) ?? null, setItem: (k: string, v: string) => void store.set(k, v) } as unknown as Storage;
    const env = {
      fetch: (async (url: string, init?: RequestInit) => {
        calls.push(url);
        signals.push(init?.signal ?? null as unknown as AbortSignal);
        return new Promise<Response>((_, reject) => {
          init?.signal?.addEventListener('abort', () => reject(new Error('aborted')));
        });
      }) as unknown as typeof fetch,
      now: () => clock,
      setTimeout: (fn: () => void, ms: number) => void timers.push({ at: clock + ms, fn }),
      storage,
    };
    const tick = (ms: number) => { clock += ms; for (const t of timers.splice(0)) if (t.at <= clock) t.fn(); else timers.push(t); };
    const im = createImages(env);
    const p = im.url('City of Traitors');
    await Promise.resolve(); await Promise.resolve();
    expect(calls.length).toBe(1);
    expect(signals[0]).toBeInstanceOf(AbortSignal);
    tick(9_999);
    let settled: string | null | 'pending' = 'pending';
    void p.then((v) => { settled = v; });
    await Promise.resolve();
    expect(settled).toBe('pending'); // a second before the timeout the lookup is still in flight
    tick(1); // the 10s boundary arms the abort
    expect(await p).toBeNull();
    expect(signals[0].aborted).toBe(true);
    expect(im.offline()).toBe(true); // the existing congestion response applies
    expect([...store.keys()]).toEqual([]); // a timeout is a failure, never a stored known-miss
  });
  it('works without storage', async () => {
    const { env } = fakeEnv({ A: { image_uris: { normal: 'a' } } });
    expect(await createImages({ ...env, storage: null }).url('A')).toBe('a');
  });
  it('re-checks offline before dispatching an already-queued lookup', async () => {
    const { env, calls, tick } = fakeEnv({
      A: new Error('net down'),
      B: { image_uris: { normal: 'b' } },
      C: { image_uris: { normal: 'c' } },
    });
    const im = createImages(env);
    const all = Promise.all([im.url('A'), im.url('B'), im.url('C')]);
    await Promise.resolve(); await Promise.resolve(); await Promise.resolve();
    expect(calls.length).toBe(1);
    expect(im.offline()).toBe(true);
    tick(100); await Promise.resolve(); await Promise.resolve();
    tick(100); await Promise.resolve(); await Promise.resolve();
    expect(calls.length).toBe(1);
    expect(await all).toEqual([null, null, null]);
  });
});
