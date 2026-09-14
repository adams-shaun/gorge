import { mount } from 'svelte';
import type { Decision } from '../protocol';
import { parseRoute } from '../lib/router';
import { getSeat, initSeatContext } from '../lib/seat';
import { SeatPanelState } from '../lib/seatpanel.svelte';
import FeedbackButton from './FeedbackButton.svelte';

// Exercise the same route and seat-store context App supplies to Feedback,
// rather than passing hand-written context or seeding breadcrumbs directly.
history.replaceState(null, '', '/t/table-fixture?seat=2&token=fixture-token');
initSeatContext(location.search);
const route = parseRoute(location.pathname);
if (route.kind !== 'table') throw new Error('fixture route did not parse');
const ctx = getSeat();
if (ctx === null) throw new Error('fixture seat did not initialise');

(window as unknown as { feedbackForm: Record<string, string> | null }).feedbackForm = null;
window.fetch = async (input, init) => {
  if (String(input).includes('/api/feedback')) {
    const form = init?.body as FormData;
    const read = (name: string) => form.get(name);
    (window as unknown as { feedbackForm: Record<string, string> | null }).feedbackForm = {
      text: String(read('text')),
      table: String(read('table')),
      seat: String(read('seat')),
      client: String(read('client.json')),
    };
  }
  return new Response('{}', { status: 201, headers: { 'Content-Type': 'application/json' } });
};

// A real SeatPanelState click runs the production intent dispatcher, which
// records the breadcrumb before posting. No fixture writes clientBreadcrumbs.
const panel = new SeatPanelState(route.table, 1, ctx, null, null);
const decision: Decision = {
  seq: 41,
  player: ctx.seat,
  kind: 'target',
  prompt: 'Choose a target.',
  min: 1,
  max: 1,
  options: [{ index: 3, kind: 'permanent', label: 'Target permanent', obj: 7, player: ctx.seat }],
};
panel.adoptView(decision);
panel.click(3);

mount(FeedbackButton, { target: document.getElementById('app')!, props: { table: route.table, seat: ctx.seat } });
