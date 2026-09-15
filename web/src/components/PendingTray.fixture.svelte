<script lang="ts">
  import type { Decision, PendingView } from '../protocol';
  import { SeatPanelState } from '../lib/seatpanel.svelte';
  import { stuckDecision } from '../lib/prompt';
  import PendingTray from './PendingTray.svelte';

  /**
   * PendingTray fixture: the tray states the empty-answer safety-net test
   * drives in a real browser. `case` picks the seat's pending decision:
   *
   *  - 'empty' — no decision and an empty trigger queue: "Nothing waiting",
   *    exactly as before this fix.
   *  - 'stuck' — the live bug's wire decision (demo game g4, seat 0): the
   *    Squadron Hawk search's Min 0 / Max 0 KChoose with NO options, which no
   *    picker can render. The tray entry and its Continue are wired the way
   *    Table.svelte wires them — stuckDecision classifies the decision and
   *    the Continue calls the REAL SeatPanelState.continueEmpty, whose post
   *    goes through the real lib/api postIntent. window.fetch is captured so
   *    the posted intent is readable at window.__posted without a server.
   *  - 'stuck-unanswerable' — the same shape with a positive Min: there is
   *    no legal answer at all, so the tray names the decision but offers no
   *    Continue.
   */

  let { case: which = 'empty' }: { case?: 'empty' | 'stuck' | 'stuck-unanswerable' } = $props();

  const pending: PendingView[] = [];

  function initialDecision(): Decision | null {
    const emptyChoose: Decision = {
      seq: 846,
      player: 0,
      kind: 'choose',
      prompt: 'Search a library: choose up to 0 card(s)',
      min: 0,
      max: 0,
      options: [],
    };
    if (which === 'stuck') return emptyChoose;
    if (which === 'stuck-unanswerable') return { ...emptyChoose, prompt: 'Search a library: choose 1 card(s)', min: 1, max: 1 };
    return null;
  }
  const decision = initialDecision();

  const panel = new SeatPanelState('t1', 1, { seat: 0, token: 'tok' }, null, null);
  panel.skipEmpty = false;
  panel.adoptView(decision);

  const stuck = stuckDecision(decision);
  const onContinue = stuck?.answerable ? () => panel.continueEmpty() : null;

  // Capture the intent the seat posts so the test reads the exact wire body
  // and the post resolves OK without a server.
  const real = window.fetch.bind(window);
  window.fetch = async (input: Parameters<typeof fetch>[0], init?: Parameters<typeof fetch>[1]): Promise<Response> => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url;
    if (url.includes('/intent') && init?.body !== undefined) {
      (window as unknown as { __posted?: string }).__posted = String(init.body);
      return new Response('{}', { status: 200 });
    }
    return real(input, init);
  };
</script>

<PendingTray {pending} {stuck} {onContinue} />
