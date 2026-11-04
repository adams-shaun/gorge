import { mount } from 'svelte';
import '../app.css';
import Fixture from './PairedBotPanel.fixture.svelte';
import { botPick } from '../lib/botpick.svelte';

// The paired bot panel over stubbed routes: seq 1 answers only after
// __releaseFirst(), so a test can move the decision while it thinks; seq 3 is
// refused; seq 4 is out of date on the first ask; seq 5 fails in the server's
// words; seq 6 never answers, as on a busy server. Every request is counted.
// ?shared=1 is a table with another human. window.__pick reads the pick the
// panel marks on the decision.
const shared = new URLSearchParams(location.search).get('shared') === '1';
const probe = { asked: [] as number[], posts: 0 };
let releaseFirst = () => {};
const first = new Promise<void>((resolve) => (releaseFirst = resolve));
const w = window as unknown as { __probe: typeof probe; __releaseFirst: () => void; __pick: () => unknown };
w.__probe = probe;
w.__releaseFirst = () => releaseFirst();
w.__pick = () => (botPick.current === null ? null : { ...botPick.current });

const json = (body: unknown) => new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } });
const problem = (code: string, message: string) => new Response(JSON.stringify({ code, message }), { status: 409, headers: { 'Content-Type': 'application/json' } });
const realFetch = window.fetch.bind(window);
window.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
  const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
  if (init?.method === 'POST') probe.posts++;
  if (url.includes('/paired-bot')) return json({ offered: true, policy: 'az-closed', ...(shared ? { shared: true } : {}) });
  const m = url.match(/\/advice\?seq=(\d+)/);
  if (!m) return realFetch(input, init);
  const seq = Number(m[1]);
  probe.asked.push(seq);
  const base = { table: 't1', match: 1, ask_seq: seq, seat: 0, policy: 'az-closed' };
  if (seq === 3) return json({ ...base, state: 'refused', reason: 'Refused in plain words.' });
  if (seq === 4 && probe.asked.filter((s) => s === 4).length === 1) return problem('advice_stale', 'The decision has moved on.');
  if (seq === 5) return problem('conflict', 'host: match 1: seat 0 is not a human seat');
  if (seq === 1) await first;
  if (seq === 6) await new Promise(() => {});
  return json({
    ...base,
    state: 'ready',
    explain: {
      seat: 0, source: 'policy', bot: [0],
      search: { engine: 'azmcts', sims: 100, root_value: 0.2, rows: [
        { choices: [1], visits: 60, prior: 0.5, q: 0.3, chosen: true },
        { choices: [0], visits: 40, prior: 0.5, q: -0.1, bot: true },
      ] },
    },
  });
};

for (const k of ['gorge.pairedbot.v1', 'gorge.pairedbot.shared.v1', 'gorge.pairedbot.mark.v1']) localStorage.removeItem(k);
mount(Fixture, { target: document.querySelector('#panel')! });
