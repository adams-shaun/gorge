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
      const name = decodeURIComponent(new URL(url).searchParams.get('exact')!);
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
  it('resolves the normal image, caches in memory and storage, and treats 404 as a known miss', async () => {
    const { env, calls, store } = fakeEnv({ 'Goblin Guide': { image_uris: { normal: 'https://img/gg.jpg' } } });
    const im = createImages(env);
    expect(await im.url('Goblin Guide')).toBe('https://img/gg.jpg');
    expect(await im.url('Goblin Guide')).toBe('https://img/gg.jpg');
    expect(calls.length).toBe(1);
    expect(store.get('gorge.img.Goblin Guide')).toBe('https://img/gg.jpg');
    expect(await im.url('Nonexistent')).toBeNull();
    expect(await im.url('Nonexistent')).toBeNull();
    expect(calls.length).toBe(2);
  });
  it('uses the front face of a double-faced card', async () => {
    const { env } = fakeEnv({ 'Delver of Secrets': { card_faces: [{ image_uris: { normal: 'https://img/front.jpg' } }, { image_uris: { normal: 'https://img/back.jpg' } }] } });
    expect(await createImages(env).url('Delver of Secrets')).toBe('https://img/front.jpg');
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
