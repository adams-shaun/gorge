import { describe, expect, it } from 'vitest';
import { createImages } from './images';

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
  it('asks this app\'s own /art/named, never Scryfall directly', async () => {
    const { env, calls } = fakeEnv({ 'Goblin Guide': { image_uris: { normal: '/art/blob/abc.jpg' } } });
    await createImages(env).url('Goblin Guide');
    expect(calls).toHaveLength(1);
    expect(calls[0]).toMatch(/^\/art\/named\?exact=/);
    expect(calls[0]).not.toContain('scryfall');
  });
  it('resolves the normal image, caches in memory and storage, and treats 404 as a known miss', async () => {
    const { env, calls, store } = fakeEnv({ 'Goblin Guide': { image_uris: { normal: 'https://img/gg.jpg' } } });
    const im = createImages(env);
    expect(await im.url('Goblin Guide')).toBe('https://img/gg.jpg');
    expect(await im.url('Goblin Guide')).toBe('https://img/gg.jpg');
    expect(calls.length).toBe(1);
    expect(store.get('gorge.img.v2.Goblin Guide')).toBe('https://img/gg.jpg');
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
    expect(store.get('gorge.img.v2.Insectile Aberration')).toBe('/art/blob/new-back.jpg');
    expect(store.get('gorge.img.Insectile Aberration')).toBe('/art/blob/old-front.jpg');
  });
  it('uses the front face of a double-faced card when no face name matches', async () => {
    const { env } = fakeEnv({ 'Delver of Secrets': { card_faces: [{ image_uris: { normal: 'https://img/front.jpg' } }, { image_uris: { normal: 'https://img/back.jpg' } }] } });
    expect(await createImages(env).url('Delver of Secrets')).toBe('https://img/front.jpg');
  });
  it('resolves a back-face name to the back face of a double-faced card', async () => {
    // task fb-20260914T033246Z-3f1cc033 defect 3: Scryfall lists BOTH faces
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
