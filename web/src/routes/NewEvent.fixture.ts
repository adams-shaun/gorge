import { mount, tick } from 'svelte';
import NewEvent from './NewEvent.svelte';

// The create screen against a recording fetch double: the three discovery
// GETs answer from fixtures, and POST /api/events records its body and
// answers with one invite per human seat the body asked for — the shape
// eventhost's Create replies with. navigateTo is the real router's, so a
// navigation shows as a changed location.pathname. ?format=sealed or
// ?format=prerelease selects that Mode before the browser test looks.
type W = { createBodies: unknown[] };
const initialFormat = new URLSearchParams(location.search).get('format');
(window as unknown as W).createBodies = [];
history.replaceState(null, '', '/events/new');
window.fetch = async (input, init) => {
  const url = String(input);
  const json = (body: unknown) => new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } });
  if (url.endsWith('/api/bot-policies')) return json({ default: 'bot', policies: [] });
  if (url.endsWith('/api/events/sets')) return json([{ code: 'FDN', name: 'Foundations', pack_size: 14, ratings: 'heuristic' }]);
  if (url.endsWith('/api/events/personas')) return json({ drafters: ['rating-colour'], builders: ['rating-colour'] });
  if (url.endsWith('/api/events') && init?.method === 'POST') {
    const body = JSON.parse(String(init.body)) as { seats: { kind: string }[] };
    (window as unknown as W).createBodies.push(body);
    const invites = body.seats
      .map((s, seat) => ({ s, seat }))
      .filter(({ s }) => s.kind === 'human')
      .map(({ seat }) => ({ seat, token: `tok${seat}secret`, join: `/e/ev7?seat=${seat}&token=tok${seat}secret` }));
    return json({ event: 'ev7', invites, state: {} });
  }
  return new Response('{}', { status: 404 });
};

mount(NewEvent, { target: document.getElementById('app')! });

if (initialFormat === 'sealed' || initialFormat === 'prerelease') {
  document.querySelector<HTMLButtonElement>(`[data-testid="format-${initialFormat}"]`)?.click();
  await tick();
}
