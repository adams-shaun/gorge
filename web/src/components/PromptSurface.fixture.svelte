<script lang="ts">
  import type { Decision, PlayerView, SeatInfo, View } from '../protocol';
  import { SeatPanelState } from '../lib/seatpanel.svelte';
  import HotButtonStrip from './HotButtonStrip.svelte';
  import SeatPanel from './SeatPanel.svelte';

  /**
   * PromptSurface fixture: the surfaces the prompt-surface tests drive in
   * a real browser. `case` picks the decision:
   *
   *  - 'initiative' — a target decision (no pass option): the REQUIRED prompt.
   *    The strip (with its ACTIONS drop) and the board prompt surface mount
   *    exactly as Table mounts them for this tone, so the test can prove the
   *    prompt is answerable with the dropdown never opened.
   *  - 'arrange' — a KArrange PURE REORDER (Min == Max == 5): the full arrange
   *    flow on the shape that keeps every card (the popup opens with all five
   *    kept in offered order; only reordering moves them).
   *  - 'scry' — a KArrange scry (Min 0, Max 5, destination "bottom"): the
   *    click-to-place flow, where the pool row exists and keeps cards one
   *    click at a time. window.fetch is captured so the intent the seat posts
   *    is readable at window.__posted without a server.
   *  - 'seqswap' — the stale-edits regression (r2 review): arrange decision A
   *    (seq 7) is on screen with its popup openable, and the [data-swap-decision]
   *    button replaces the view's decision with arrange decision B (seq 9,
   *    different cards) while the panel stays mounted — the two-tab / rapid
   *    external answer path, where the old ask's popup edits must never be
   *    presented as (or submitted for) the new ask.
   */

  let { case: which = 'initiative' }: { case?: 'initiative' | 'arrange' | 'scry' | 'seqswap' } = $props();

  const seats: SeatInfo[] = [
    { name: 'Ari', deck: 'burn', colour: '#e5484d' },
    { name: 'Bo', deck: 'stomp', colour: '#22c55e' },
  ];
  const player = (seat: number): PlayerView => ({
    seat, name: seats[seat].name, life: 20, lost: false, library_size: 40, hand_size: 0,
    graveyard_size: 0, hand: [], battlefield: [], graveyard: [], exile: [], pool: {},
    command: [], commanders: [], commander_casts: [],
  });
  const base: View = {
    viewer: 0, visibility: 'seat', turn: 2, round: 1, step: 'main1', phase: 'main1',
    active: 0, priority: 0, over: false, draw: false, winner: null,
    players: [player(0), player(1)], stack: [], pending: [],
  };

  const arrange: Decision = {
    seq: 7, player: 0, kind: 'arrange', min: 5, max: 5,
    prompt: 'Rearrange the top 5 card(s); the first card you pick goes on top',
    source: 9,
    options: [
      { index: 0, kind: 'bottom', label: 'Brazen Borrower', obj: 11, player: 0 },
      { index: 1, kind: 'bottom', label: 'Fabled Pass', obj: 12, player: 0 },
      { index: 2, kind: 'bottom', label: 'Gitaxian Probe', obj: 13, player: 0 },
      { index: 3, kind: 'bottom', label: 'Spell Pierce', obj: 14, player: 0 },
      { index: 4, kind: 'bottom', label: 'Unholy Heat', obj: 15, player: 0 },
    ],
  };
  const scry: Decision = {
    ...arrange, min: 0,
    prompt: 'Scry 5: pick the cards to keep on top, in order; the rest go to the bottom of your library',
  };
  // Decision B for the seqswap case: a different seq, prompt and option set,
  // so anything decision A left behind is detectable.
  const arrangeB: Decision = {
    seq: 9, player: 0, kind: 'arrange', min: 3, max: 3,
    prompt: 'Rearrange the top 3 card(s); the first card you pick goes on top',
    source: 12,
    options: [
      { index: 0, kind: 'bottom', label: 'Absorb Vis', obj: 21, player: 0 },
      { index: 1, kind: 'bottom', label: 'Batterskull', obj: 22, player: 0 },
      { index: 2, kind: 'bottom', label: 'Counterspell', obj: 23, player: 0 },
    ],
  };
  const target: Decision = {
    seq: 5, player: 0, kind: 'target', min: 1, max: 1,
    prompt: 'Choose a target', source: 9,
    options: [{ index: 0, kind: 'target', label: 'Target Bo', obj: 3, player: 0 }],
  };
  const decision: Decision = which === 'arrange' ? arrange : which === 'scry' ? scry : which === 'seqswap' ? arrange : target;
  // The seqswap case's view is state: the swap button replaces its decision
  // under the mounted panel, exactly as the next SSE view would.
  let view = $state<View>({ ...base, decision });
  const swapTo: Decision = arrangeB;

  const ctx = { seat: 0, token: 'tok' };
  const seatState = new SeatPanelState('t1', 1, ctx, null, null);
  seatState.skipEmpty = false;
  seatState.adoptView(decision);

  // Capture the intent the seat posts, so the tests can read the choices the
  // flows produced and the posts resolve OK without a server.
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

<div class="boardlike">
  <HotButtonStrip {view} {seats} state={seatState} {ctx} table="t1" match={1} />
  <SeatPanel {view} {seats} {ctx} table="t1" match={1} state={seatState} />
  {#if which === 'seqswap'}
    <button type="button" data-swap-decision onclick={() => (view.decision = swapTo)}>swap the ask</button>
  {/if}
</div>

<style>
  /* The one thing the fixture needs from the board: enough room for the two
     surfaces to coexist. The panel positions itself absolutely at the top
     centre, as on the real board. */
  .boardlike {
    position: relative;
    width: 900px;
    height: 600px;
    background: var(--felt);
  }
</style>
