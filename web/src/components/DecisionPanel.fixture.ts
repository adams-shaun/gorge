import { mount } from 'svelte';
import type { ReviewDecision } from '../protocol';
import '../app.css';
import DecisionPanelFixture from './DecisionPanel.fixture.svelte';

// The review decision panel's browser fixture: the REAL DecisionPanel over a
// stubbed GET .../decisions (window.fetch answers it from the index below),
// hosted by a wrapper that owns the cursor the way Table.svelte's DVR does.
// `?plain=1` serves the same index with every explain note stripped, the
// shape of a record made before explain existed.
//
// Six decisions asked at seqs 5, 15, ..., 55: 0 no note; 1 searched and
// overrode the bot; 2 heuristic; 3 root fallback; 4 searched, agreed; 5 a
// refusal fallback.
const opts = [
  { index: 0, label: 'Pass priority', kind: 'pass' },
  { index: 1, label: 'Cast Lightning Bolt', kind: 'cast' },
  { index: 2, label: 'Concede', kind: 'concede' },
];
const entry = (intent: number, over: Partial<ReviewDecision>): ReviewDecision => ({
  intent, ask_seq: 5 + intent * 10, made_seq: 6 + intent * 10, turn: 1, round: 1, seat: intent % 2, kind: 'priority',
  prompt: 'turn 1, main1', min: 1, max: 1, options: opts, options_total: 3, chosen: [0], made: 'priority:[0]',
  answer: 'Pass priority', explain: null, ...over,
});
const search = (rootValue: number) => ({
  engine: 'azmcts', sims: 64, root_value: rootValue, ms: 12,
  rows: [
    { choices: [1], visits: 40, prior: 0.5, q: 0.31, chosen: true },
    { choices: [0], visits: 24, prior: 0.5, q: -0.12, bot: true },
  ],
});
const index: ReviewDecision[] = [
  entry(0, {}),
  entry(1, { chosen: [1], answer: 'Cast Lightning Bolt', explain: { seat: 1, policy: 'az', source: 'policy', bot: [0], search: search(0.3) } }),
  entry(2, { explain: { seat: 0, source: 'heuristic', bot: [0] } }),
  entry(3, { explain: { seat: 1, source: 'fallback_root', reason: 'root refused' } }),
  entry(4, { explain: { seat: 0, policy: 'az', source: 'policy', bot: [0], search: { ...search(-0.2), rows: [{ choices: [0], visits: 60, prior: 0.6, q: -0.2, chosen: true, bot: true }] } } }),
  entry(5, { chosen: [2], explain: { seat: 1, source: 'refusal_fallback', rung: 2, bot: [0] } }),
];
const plain = new URLSearchParams(location.search).has('plain');
const served = plain ? index.map((e) => ({ ...e, explain: null })) : index;

const realFetch = window.fetch.bind(window);
window.fetch = (input: RequestInfo | URL, init?: RequestInit) => {
  const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
  if (url.includes('/matches/1/decisions')) {
    return Promise.resolve(new Response(JSON.stringify(served), { status: 200, headers: { 'Content-Type': 'application/json' } }));
  }
  return realFetch(input, init);
};

mount(DecisionPanelFixture, { target: document.querySelector('#panel')! });
