<script lang="ts">
  import { untrack } from 'svelte';
  import type { CardView, Decision } from '../protocol';
  import { SeatPanelState } from '../lib/seatpanel.svelte';
  import { resolveCardFollowUp } from '../lib/cardoptions';
  import CardTile from './CardTile.svelte';

  /**
   * SeatPanelFollowUp.fixture.svelte is the panel-path twin of
   * CardMenu.fixture.ts's fb-e079def5 stage-1 → stage-2 test AND the mounted
   * proof of the L2 race finding for fb-20260923T050205Z. Its point is the
   * ARM and the DECODE together:
   *
   *  - the captured treasure activate option is posted through the REAL
   *    SeatPanelState.click (the path SeatPanel.svelte's own option buttons
   *    use), so the panel's post arms followUpExpected;
   *  - the $effect below is Table.svelte's decode effect body, VERBATIM: it
   *    tracks the arriving decision AND panel.followUpRevision, reads the
   *    armed expectation untracked, decodes with resolveCardFollowUp and
   *    mounts the stage-2 tile open.
   *
   * Running the effect reactively (not a manual decode call) is what lets a
   * test deliver the follow-up decision BEFORE the post promise resolves —
   * the SSE-first ordering the L2 finding named — and still see the wheel
   * open, because arming bumps followUpRevision and retriggers the effect.
   *
   * The stage-two mount is deferred one macrotask, exactly as
   * CardMenu.fixture.ts defers it: synchronously it would happen inside the
   * click's bubble and the portaled radial's window-level close-on-click
   * would shut the freshly opened wheel again — an artifact the real
   * frame-driven flow never has.
   */

  // A real SeatPanelState posts through the production api dispatcher; stub
  // window.fetch so the accepted intent lands without a server (the same trick
  // FeedbackButton.fixture.ts uses). The staging decision is adopted directly,
  // so /pending is never read.
  window.fetch = async () =>
    new Response('{}', { status: 200, headers: { 'Content-Type': 'application/json' } });

  const ctx = { seat: 0, token: 'fixture-token' };
  const panel = new SeatPanelState('fixture-treasure', 1, ctx, null, null);

  // The captured priority decision (capture seq 688): two identical Treasure
  // activations (obj 205, 207), pass, concede. min == max == 1, so a click IS
  // the answer and posts.
  const priority: Decision = {
    seq: 688, player: 0, kind: 'priority', prompt: 'You have priority.', min: 1, max: 1,
    options: [
      { index: 0, kind: 'activate', label: 'Activate Treasure Token for mana', obj: 205, player: 0 },
      { index: 1, kind: 'activate', label: 'Activate Treasure Token for mana', obj: 207, player: 0 },
      { index: 2, kind: 'pass', label: 'Pass priority', player: 0 },
      { index: 3, kind: 'concede', label: 'Concede', player: 0 },
    ],
  };
  panel.adoptView(priority);

  // The captured follow-up colour ask (capture seq 693): five Kind "mana"
  // options all on obj 207 — the 2-6 all-mana shape resolveCardFollowUp opens.
  const choose: Decision = {
    seq: 693, player: 0, kind: 'choose', prompt: 'Add 1 mana of any one color — choose the colour', min: 1, max: 1,
    source: 207,
    options: [
      { index: 0, kind: 'mana', label: 'Add W', obj: 207, player: 0 },
      { index: 1, kind: 'mana', label: 'Add U', obj: 207, player: 0 },
      { index: 2, kind: 'mana', label: 'Add B', obj: 207, player: 0 },
      { index: 3, kind: 'mana', label: 'Add R', obj: 207, player: 0 },
      { index: 4, kind: 'mana', label: 'Add G', obj: 207, player: 0 },
    ],
  };

  const card = (id: number, name: string, types: string): CardView => ({
    id, name, types, mana_cost: '',
    printing: { name }, token: '', tapped: false, power: 0, toughness: 0,
    damage: 0, attacking: false, controller: 0, owner: 0, summon_sick: false,
  });

  // The follow-up decision is DELIVERED separately from the arm, so a test
  // can choose the order: the race test sets it BEFORE the click (SSE first),
  // the ordinary test after (the network round trip order).
  let followUp = $state<Decision | null>(null);
  let autoOpen = $state<{ seq: number; obj: number } | null>(null);
  let stageTwoMounted = $state(false);

  // Table.svelte's decode effect, reproduced verbatim. Tracking
  // followUpRevision is what retriggers after the POST arms the expectation
  // when the follow-up decision already arrived; untracking the expectation
  // keeps the panel's derived UI graph out of this effect's dependencies (the
  // dfa4bb5c crash).
  $effect(() => {
    const d = followUp;
    const followUpRevision = panel.followUpRevision;
    void followUpRevision;
    const expected = untrack(() => panel.followUpExpected);
    if (d === null || expected === null || d.seq === expected.seq) return;
    autoOpen = resolveCardFollowUp(expected, d);
    panel.followUpExpected = null;
    if (autoOpen !== null) {
      stageTwoMounted = false;
      setTimeout(() => (stageTwoMounted = true), 0);
    }
  });

  // window.* casts (not a `declare global`) keep svelte-check happy: a global
  // interface declared inside a component <script> is module-scoped, so it
  // never augments Window for the test file.
  const w = window as unknown as {
    __click: (index: number) => void;
    __deliverFollowUp: () => void;
    __armState: () => { seq: number; obj: number } | null;
    __autoOpen: () => { seq: number; obj: number } | null;
    __revision: () => number;
  };
  w.__click = (index) => panel.click(index);
  w.__deliverFollowUp = () => (followUp = choose);
  w.__armState = () => panel.followUpExpected;
  w.__autoOpen = () => autoOpen;
  w.__revision = () => panel.followUpRevision;
</script>

<div id="panel">
  <!-- SeatPanel.svelte's own option-button shape: a plain click goes straight
       to logic.click(opt.index), no follow-up flag, no help from the tile. -->
  {#each priority.options as opt (opt.index)}
    <button type="button" id={`panel-option-${opt.index}`} onclick={() => panel.click(opt.index)}>
      {opt.label}
    </button>
  {/each}
</div>
<div id="followup" style="width: 220px; height: 320px; margin: 260px">
  {#if stageTwoMounted && autoOpen !== null}
    <CardTile
      card={card(207, 'Treasure Token', 'Artifact Token')}
      tileOptions={{
        list: choose.options,
        pickedOrder: [],
        tone: 'initiative',
        autoOpen: true,
        post: () => {},
      }}
    />
  {/if}
</div>
