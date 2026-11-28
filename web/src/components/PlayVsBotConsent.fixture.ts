import { mount } from 'svelte';
import PlayVsBot from './PlayVsBot.svelte';

// The real panel over a fetch the test drives: every POST (a deck check or a
// game start) is recorded and held until the test answers it, so replies can
// land late or out of order. The panel's GETs answer empty at once.
export interface HeldPost {
  to: 'check' | 'start';
  body: Record<string, unknown>;
  reply: (status: number, payload: unknown) => void;
}

const posts: HeldPost[] = [];
(window as unknown as { posts: HeldPost[] }).posts = posts;

// A plain object rather than a Response: its json() settles in microtasks, so
// one macrotask after reply the panel has applied (or ignored) the answer.
const answer = (status: number, payload: unknown) =>
  ({ ok: status < 300, status, statusText: String(status), json: async () => payload }) as Response;

window.fetch = async (input, init) => {
  const url = String(input);
  if (init?.method === 'POST') {
    const to = url.endsWith('/api/decks/check') ? 'check' : 'start';
    return new Promise<Response>((resolve) => {
      posts.push({ to, body: JSON.parse(String(init.body)), reply: (status, payload) => resolve(answer(status, payload)) });
    });
  }
  if (url.endsWith('/api/decks')) return answer(200, []);
  if (url.endsWith('/api/bot-policies')) return answer(200, { policies: [], default: '' });
  return answer(200, {});
};

mount(PlayVsBot, { target: document.getElementById('app')! });
